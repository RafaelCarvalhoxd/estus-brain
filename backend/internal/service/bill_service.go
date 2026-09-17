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
	SetSeriesEnded(ctx context.Context, id string, ended bool) (domain.Bill, error)
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
	// one. Create honors it verbatim when present; Update ignores it
	// entirely — see the notes on each.
	SeriesID *string
	// Recurring tells Create "start a new series for this bill" when no
	// SeriesID was supplied. It has no effect on Update: series membership
	// only ever changes at creation.
	Recurring bool
	// AmountEstimated says THIS occurrence's amount is not yet confirmed.
	// Create seeds a new series' first occurrence from AmountVaries (there is
	// no previous occurrence to carry it from); Update takes it verbatim from
	// the request, so an edit that supplies a corrected amount and simply
	// omits this field — as the Contas edit row does — clears it, the same
	// way paying a bill does.
	AmountEstimated bool
	// AmountVaries says THIS SERIES' amount changes every month — see the
	// field of the same name on domain.Bill for why it must stay distinct
	// from AmountEstimated. Both Create and Update take it from the request:
	// unlike SeriesID/PaidAt/TransactionID/SeriesEnded, it is an ordinary
	// editable property, not one only a dedicated endpoint may change — the
	// caller (frontend) is responsible for resending the bill's current
	// value on every edit, the same way it already does for CategoryID and
	// PaymentMethod.
	AmountVaries  bool
	PaymentMethod *domain.PaymentMethod
}

// BillSummary is the "em aberto" snapshot the /contas page's stat tiles are
// built from.
type BillSummary struct {
	PayableOpenCents    domain.Cents
	ReceivableOpenCents domain.Cents
	OverdueCount        int
}

func (s *BillService) Create(ctx context.Context, in NewBillInput) (domain.Bill, error) {
	// Create is the only place that mints a series id, and only when the
	// caller marked the bill recurring and didn't already hand one in (the
	// assistant tool, unlike the Contas form, may one day want to attach a
	// new bill to an existing series by passing SeriesID directly). A
	// recurring PAYABLE bill with no category or payment method still fails
	// bill.Validate below with a 422 — the Contas form supplies both, which
	// is what makes minting safe to turn on here.
	seriesID := in.SeriesID
	if in.Recurring && seriesID == nil {
		fresh := domain.NewID()
		seriesID = &fresh
	}
	bill := domain.Bill{
		Description:     in.Description,
		AmountCents:     in.AmountCents,
		DueDate:         in.DueDate,
		Direction:       in.Direction,
		CategoryID:      in.CategoryID,
		SeriesID:        seriesID,
		AmountEstimated: in.AmountEstimated,
		AmountVaries:    in.AmountVaries,
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

// ListByMonth is how the Contas screen reads a month: it materializes ym
// first (creating whatever occurrences a recurring series is missing up to
// and including ym) and only then lists it, so opening a month is what
// makes its recurring bills exist. Materialize is idempotent, so calling it
// on every view — including navigating back to a month already seen — never
// creates a duplicate.
func (s *BillService) ListByMonth(ctx context.Context, ym domain.YearMonth, direction *domain.BillDirection) ([]domain.Bill, error) {
	if _, err := s.Materialize(ctx, ym); err != nil {
		return nil, fmt.Errorf("materialize %v before listing: %w", ym, err)
	}
	bills, err := s.bills.ListByMonth(ctx, ym, direction)
	if err != nil {
		return nil, fmt.Errorf("list bills by month: %w", err)
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
	// Update never mints a series id, and — unlike Create — it never even
	// reads in.SeriesID: the wire has no series_id field, so an edit form
	// that resubmits "recurring: true" carries no SeriesID at all, and
	// trusting that absence would silently pull the bill out of its series
	// on every save. Series membership, whether it's paid, which transaction
	// it settled into, and whether the series has been ended are loaded from
	// the bill as it stands in the database and carried forward untouched;
	// the client cannot alter any of the four, by accident or otherwise —
	// ending or resuming a series is EndSeries/ResumeSeries's job alone.
	current, err := s.bills.Get(ctx, id)
	if err != nil {
		return domain.Bill{}, err
	}
	bill := domain.Bill{
		ID:              id,
		Description:     in.Description,
		AmountCents:     in.AmountCents,
		DueDate:         in.DueDate,
		Direction:       in.Direction,
		CategoryID:      in.CategoryID,
		SeriesID:        current.SeriesID,
		PaidAt:          current.PaidAt,
		TransactionID:   current.TransactionID,
		SeriesEnded:     current.SeriesEnded,
		AmountEstimated: in.AmountEstimated,
		AmountVaries:    in.AmountVaries,
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

// EndSeries stops a series from growing new occurrences: Materialize skips
// any series whose latest occurrence has SeriesEnded set. It touches no
// history — every occurrence already materialized is untouched, and
// deleting one is a plain delete again, not a signal Materialize would
// undo the next time the month is opened.
func (s *BillService) EndSeries(ctx context.Context, id string) (domain.Bill, error) {
	bill, err := s.bills.Get(ctx, id)
	if err != nil {
		return domain.Bill{}, err
	}
	if !bill.Recurring() {
		return domain.Bill{}, fmt.Errorf("%w: only a recurring bill's series can be ended", domain.ErrValidation)
	}
	return s.bills.SetSeriesEnded(ctx, id, true)
}

// ResumeSeries undoes EndSeries: the next materialization picks the series
// back up from wherever its latest occurrence left off.
func (s *BillService) ResumeSeries(ctx context.Context, id string) (domain.Bill, error) {
	bill, err := s.bills.Get(ctx, id)
	if err != nil {
		return domain.Bill{}, err
	}
	if !bill.Recurring() {
		return domain.Bill{}, fmt.Errorf("%w: only a recurring bill's series can be resumed", domain.ErrValidation)
	}
	return s.bills.SetSeriesEnded(ctx, id, false)
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
// a series already has an occurrence in is left alone. A series whose latest
// occurrence has SeriesEnded set is skipped entirely — that is how the owner
// cancels a recurring bill, instead of the deleted-last-occurrence rule this
// replaced (which Materialize itself made unworkable: deleting the latest
// occurrence and reopening the same month would just recreate it). Each
// series gets its own budget of materializeCap new occurrences — see the
// cap's doc comment for why the budget is not shared across series.
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
		if last.SeriesEnded {
			continue
		}
		current := last
		createdForSeries := 0
		for createdForSeries < materializeCap {
			currentMonth := domain.YearMonthOf(current.DueDate)
			if !currentMonth.Before(ym) {
				break
			}
			// The new occurrence's AmountEstimated is seeded from the
			// SERIES' own AmountVaries, not the outgoing occurrence's own
			// AmountEstimated — that would already be false once an
			// occurrence is paid or corrected, which says nothing about
			// whether the series itself still varies month to month.
			next := current.NextOccurrence(current.AmountVaries)
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
