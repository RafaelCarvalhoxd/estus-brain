package service

import (
	"context"
	"fmt"
	"time"

	"github.com/rafael/estus-vault/backend/internal/domain"
	"github.com/rafael/estus-vault/backend/internal/store/postgres"
)

// billStore is the part of *postgres.BillRepo the service uses, so the
// materialization rules can be tested without a database.
type billStore interface {
	Create(ctx context.Context, b domain.Bill) (domain.Bill, error)
	Get(ctx context.Context, id string) (domain.Bill, error)
	List(ctx context.Context, direction *domain.BillDirection, onlyOpen bool) ([]domain.Bill, error)
	ListByMonth(ctx context.Context, ym domain.YearMonth, direction *domain.BillDirection) ([]domain.Bill, error)
	LatestPerSeries(ctx context.Context) ([]domain.Bill, error)
	MarkPaid(ctx context.Context, id string, paidAt time.Time) (domain.Bill, error)
	Pay(ctx context.Context, id string, paidAt time.Time, expense domain.Transaction) (domain.Bill, error)
	Unpay(ctx context.Context, id string) (domain.Bill, error)
	Update(ctx context.Context, b domain.Bill) (domain.Bill, error)
	Delete(ctx context.Context, id string) error
	ReceivedTotalForMonth(ctx context.Context, ym domain.YearMonth) (domain.Cents, error)
	OpenTotals(ctx context.Context) (domain.Cents, domain.Cents, int, error)
}

// categoryStore is the part of *postgres.CategoryRepo the service uses, so
// Pay's category-existence check can be tested without a database.
type categoryStore interface {
	Get(ctx context.Context, id string) (domain.Category, error)
}

// cardStore is the part of *postgres.CreditCardRepo the service uses, so
// Pay's credit-card competence-month math can be tested without a database.
type cardStore interface {
	Get(ctx context.Context, id string) (domain.CreditCard, error)
}

type BillService struct {
	bills      billStore
	categories categoryStore
	cards      cardStore
}

func NewBillService(repo *postgres.BillRepo, categories *postgres.CategoryRepo, cards *postgres.CreditCardRepo) *BillService {
	return &BillService{bills: repo, categories: categories, cards: cards}
}

type NewBillInput struct {
	Description string
	AmountCents domain.Cents
	DueDate     time.Time
	Direction   domain.BillDirection
	CategoryID  *string
	// SeriesID is the series this bill belongs to, if the caller supplied
	// one. The service never invents a series id of its own — see the note
	// in Create.
	SeriesID *string
	// Recurring currently only round-trips: no wire input sets SeriesID yet,
	// so this flag doesn't do anything on its own. It exists so a future
	// task (once the form has category and payment-method fields) has
	// somewhere to read "the caller wants this bill to start a series" from.
	Recurring       bool
	AmountEstimated bool
	PaymentMethod   *domain.PaymentMethod
}

// BillSummary is the "em aberto" snapshot the /contas page's stat tiles are
// built from.
type BillSummary struct {
	PayableOpenCents    domain.Cents
	ReceivableOpenCents domain.Cents
	OverdueCount        int
}

func (s *BillService) Create(ctx context.Context, in NewBillInput) (domain.Bill, error) {
	// The service never mints a series id: it only ever writes SeriesID when
	// the caller explicitly supplied one. Generating one here for
	// in.Recurring==true would make every recurring payable bill require a
	// category and payment method (domain.Bill.Validate) before either the
	// assistant tool or the Contas form has a field to collect the payment
	// method — that arrives with the task that adds those fields, which is
	// also the task that should decide where series-id generation happens.
	bill := domain.Bill{
		Description:     in.Description,
		AmountCents:     in.AmountCents,
		DueDate:         in.DueDate,
		Direction:       in.Direction,
		CategoryID:      in.CategoryID,
		SeriesID:        in.SeriesID,
		AmountEstimated: in.AmountEstimated,
		PaymentMethod:   in.PaymentMethod,
	}
	if err := bill.Validate(); err != nil {
		return domain.Bill{}, err
	}
	if in.CategoryID != nil {
		if _, err := s.categories.Get(ctx, *in.CategoryID); err != nil {
			return domain.Bill{}, fmt.Errorf("category %s: %w", *in.CategoryID, err)
		}
	}
	created, err := s.bills.Create(ctx, bill)
	if err != nil {
		return domain.Bill{}, fmt.Errorf("save bill: %w", err)
	}
	return created, nil
}

func (s *BillService) List(ctx context.Context, direction *domain.BillDirection, onlyOpen bool) ([]domain.Bill, error) {
	bills, err := s.bills.List(ctx, direction, onlyOpen)
	if err != nil {
		return nil, fmt.Errorf("list bills: %w", err)
	}
	return bills, nil
}

func (s *BillService) MarkPaid(ctx context.Context, id string, paidAt time.Time) (domain.Bill, error) {
	bill, err := s.bills.MarkPaid(ctx, id, paidAt)
	if err != nil {
		return domain.Bill{}, err
	}
	return bill, nil
}

// PaymentInput is what the owner confirms when settling a bill: the amount
// that actually left the account, and how.
type PaymentInput struct {
	PaidAt        time.Time
	AmountCents   domain.Cents
	CategoryID    string
	PaymentMethod domain.PaymentMethod
	CreditCardID  *string
}

// Pay settles a payable bill and records the expense behind it, in one
// database transaction: either the bill and the expense both land or
// neither does. A receivable is settled with MarkPaid instead: the
// transactions ledger is an expense ledger, and a credit there would be a
// negative every aggregate would have to special-case.
func (s *BillService) Pay(ctx context.Context, id string, in PaymentInput) (domain.Bill, domain.Transaction, error) {
	bill, err := s.bills.Get(ctx, id)
	if err != nil {
		return domain.Bill{}, domain.Transaction{}, err
	}
	if bill.Direction != domain.BillPayable {
		return domain.Bill{}, domain.Transaction{}, fmt.Errorf("%w: only a payable bill records an expense", domain.ErrValidation)
	}
	if in.AmountCents <= 0 {
		return domain.Bill{}, domain.Transaction{}, fmt.Errorf("%w: amount must be positive", domain.ErrValidation)
	}
	if !in.PaymentMethod.Valid() {
		return domain.Bill{}, domain.Transaction{}, fmt.Errorf("%w: invalid payment method %q", domain.ErrValidation, in.PaymentMethod)
	}
	if in.PaymentMethod == domain.PaymentCredit && (in.CreditCardID == nil || *in.CreditCardID == "") {
		return domain.Bill{}, domain.Transaction{}, domain.ErrCreditCardMissing
	}
	if _, err := s.categories.Get(ctx, in.CategoryID); err != nil {
		return domain.Bill{}, domain.Transaction{}, fmt.Errorf("category %s: %w", in.CategoryID, err)
	}

	var card *domain.CreditCard
	if in.PaymentMethod == domain.PaymentCredit {
		c, err := s.cards.Get(ctx, *in.CreditCardID)
		if err != nil {
			return domain.Bill{}, domain.Transaction{}, fmt.Errorf("credit card %s: %w", *in.CreditCardID, err)
		}
		card = &c
	}

	expense := domain.Transaction{
		ID:               domain.NewID(),
		Description:      bill.Description,
		AmountCents:      in.AmountCents,
		CategoryID:       in.CategoryID,
		PaymentMethod:    in.PaymentMethod,
		PurchaseDate:     in.PaidAt,
		CreditCardID:     in.CreditCardID,
		CompetenceMonth:  domain.CompetenceMonth(in.PaidAt, in.PaymentMethod, card),
		InstallmentTotal: 1,
		IsRecurring:      bill.Recurring(),
	}
	paid, err := s.bills.Pay(ctx, id, in.PaidAt, expense)
	if err != nil {
		return domain.Bill{}, domain.Transaction{}, err
	}
	return paid, expense, nil
}

// Unpay reverses Pay: the bill goes back to pending and the expense it
// created is removed, in the same commit.
func (s *BillService) Unpay(ctx context.Context, id string) (domain.Bill, error) {
	return s.bills.Unpay(ctx, id)
}

func (s *BillService) Update(ctx context.Context, id string, in NewBillInput) (domain.Bill, error) {
	// Same rule as Create: never invent a series id here. An edit that only
	// corrects this month's amount must not silently split the bill into a
	// new series — it must keep whatever SeriesID the caller passed in
	// (typically the occurrence's own, unchanged) or none at all.
	bill := domain.Bill{
		ID:              id,
		Description:     in.Description,
		AmountCents:     in.AmountCents,
		DueDate:         in.DueDate,
		Direction:       in.Direction,
		CategoryID:      in.CategoryID,
		SeriesID:        in.SeriesID,
		AmountEstimated: in.AmountEstimated,
		PaymentMethod:   in.PaymentMethod,
	}
	if err := bill.Validate(); err != nil {
		return domain.Bill{}, err
	}
	if in.CategoryID != nil {
		if _, err := s.categories.Get(ctx, *in.CategoryID); err != nil {
			return domain.Bill{}, fmt.Errorf("category %s: %w", *in.CategoryID, err)
		}
	}
	updated, err := s.bills.Update(ctx, bill)
	if err != nil {
		return domain.Bill{}, err
	}
	return updated, nil
}

func (s *BillService) Delete(ctx context.Context, id string) error {
	return s.bills.Delete(ctx, id)
}

func (s *BillService) Summary(ctx context.Context) (BillSummary, error) {
	payable, receivable, overdue, err := s.bills.OpenTotals(ctx)
	if err != nil {
		return BillSummary{}, fmt.Errorf("bill summary: %w", err)
	}
	return BillSummary{PayableOpenCents: payable, ReceivableOpenCents: receivable, OverdueCount: overdue}, nil
}

func (s *BillService) ReceivedTotal(ctx context.Context, ym domain.YearMonth) (domain.Cents, error) {
	return s.bills.ReceivedTotalForMonth(ctx, ym)
}

// materializeCap is how many months one call may create PER SERIES. It is
// per series, not shared, because an incomplete month is worse than a
// larger single write: with the 5-10 fixed expenses a person actually has,
// a laptop off for months would otherwise starve whichever series
// LatestPerSeries happens to hand out last, and the owner has no way to
// tell an incomplete month apart from a complete one. Opening a month years
// out still bounds the write per series instead of creating unboundedly
// many rows for any one series.
const materializeCap = 24

// Materialize creates the missing occurrences of every active series up to
// and including ym, oldest first, each copied from the one before it, and
// returns the total it created across all series. It is idempotent: a month
// a series already has an occurrence in is left alone. Each series gets its
// own budget of materializeCap new occurrences — see the cap's doc comment
// for why the budget is not shared across series.
//
// This is a write driven by a read, on purpose: the app runs on a laptop
// that is off for days at a time, so a scheduler on the first of the month
// would need catch-up logic for every month it slept through.
func (s *BillService) Materialize(ctx context.Context, ym domain.YearMonth) (int, error) {
	latest, err := s.bills.LatestPerSeries(ctx)
	if err != nil {
		return 0, err
	}
	total := 0
	for _, last := range latest {
		current := last
		createdForSeries := 0
		for createdForSeries < materializeCap {
			currentMonth := domain.YearMonthOf(current.DueDate)
			if !currentMonth.Before(ym) {
				break
			}
			next := current.NextOccurrence(current.AmountEstimated)
			saved, err := s.bills.Create(ctx, next)
			if err != nil {
				return total, err
			}
			total++
			createdForSeries++
			current = saved
		}
	}
	return total, nil
}
