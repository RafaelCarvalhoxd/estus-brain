package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rafael/estus-vault/backend/internal/domain"
)

// fakeBills keeps bills in memory, which is all Materialize needs.
type fakeBills struct {
	bills []domain.Bill
	// paidExpense captures the exact domain.Transaction Pay handed the
	// repository, so tests can assert on it — a fake that discarded this
	// argument would let a wrong competence month, amount or IsRecurring
	// sail through the whole suite unnoticed.
	paidExpense *domain.Transaction
}

func (f *fakeBills) Create(_ context.Context, b domain.Bill) (domain.Bill, error) {
	if b.ID == "" {
		b.ID = "generated-" + b.DueDate.Format("2006-01")
	}
	f.bills = append(f.bills, b)
	return b, nil
}

func (f *fakeBills) ListByMonth(_ context.Context, ym domain.YearMonth, _ *domain.BillDirection) ([]domain.Bill, error) {
	var out []domain.Bill
	for _, b := range f.bills {
		if b.DueDate.Year() == ym.Year && int(b.DueDate.Month()) == ym.Month {
			out = append(out, b)
		}
	}
	return out, nil
}

func (f *fakeBills) LatestPerSeries(context.Context) ([]domain.Bill, error) {
	latest := map[string]domain.Bill{}
	for _, b := range f.bills {
		if b.SeriesID == nil {
			continue
		}
		if cur, ok := latest[*b.SeriesID]; !ok || b.DueDate.After(cur.DueDate) {
			latest[*b.SeriesID] = b
		}
	}
	var out []domain.Bill
	for _, b := range latest {
		out = append(out, b)
	}
	return out, nil
}

func (f *fakeBills) List(context.Context, *domain.BillDirection, bool) ([]domain.Bill, error) {
	return f.bills, nil
}
func (f *fakeBills) MarkPaid(context.Context, string, time.Time) (domain.Bill, error) {
	return domain.Bill{}, nil
}

func (f *fakeBills) Get(_ context.Context, id string) (domain.Bill, error) {
	for _, b := range f.bills {
		if b.ID == id {
			return b, nil
		}
	}
	return domain.Bill{}, domain.ErrNotFound
}

// Pay records the expense it was handed (see paidExpense) and marks the
// matching bill paid, so a test can inspect both what the service decided
// to persist and that it used the right bill.
func (f *fakeBills) Pay(_ context.Context, id string, paidAt time.Time, expense domain.Transaction) (domain.Bill, error) {
	f.paidExpense = &expense
	for i := range f.bills {
		if f.bills[i].ID == id {
			f.bills[i].PaidAt = &paidAt
			f.bills[i].TransactionID = &expense.ID
			return f.bills[i], nil
		}
	}
	return domain.Bill{}, domain.ErrNotFound
}

func (f *fakeBills) Unpay(context.Context, string) (domain.Bill, error) {
	return domain.Bill{}, nil
}

func (f *fakeBills) SetSeriesEnded(_ context.Context, id string, ended bool) (domain.Bill, error) {
	for i := range f.bills {
		if f.bills[i].ID == id {
			f.bills[i].SeriesEnded = ended
			return f.bills[i], nil
		}
	}
	return domain.Bill{}, domain.ErrNotFound
}

// fakeCategories and fakeCards stand in for *postgres.CategoryRepo and
// *postgres.CreditCardRepo (via categoryStore/cardStore) so
// BillService.Pay's category and credit-card lookups can be tested without
// a database.
type fakeCategories struct{ known map[string]domain.Category }

func (f fakeCategories) Get(_ context.Context, id string) (domain.Category, error) {
	if c, ok := f.known[id]; ok {
		return c, nil
	}
	return domain.Category{}, domain.ErrNotFound
}

type fakeCards struct{ known map[string]domain.CreditCard }

func (f fakeCards) Get(_ context.Context, id string) (domain.CreditCard, error) {
	if c, ok := f.known[id]; ok {
		return c, nil
	}
	return domain.CreditCard{}, domain.ErrNotFound
}

// serviceWithPay wires a BillService whose categories and credit cards are
// fakes too, which serviceWith's nil fields don't allow — Pay needs both.
func serviceWithPay(f *fakeBills, cats fakeCategories, cards fakeCards) *BillService {
	s := NewBillService(nil, nil, nil)
	s.bills = f
	s.categories = cats
	s.cards = cards
	return s
}

// Update echoes back whatever bill the service handed it, unchanged. That is
// what makes TestServiceNeverInventsASeriesID meaningful: it lets the test
// see exactly what BillService.Update passed through, rather than a stub
// value that would hide a bug either way.
func (f *fakeBills) Update(_ context.Context, b domain.Bill) (domain.Bill, error) {
	return b, nil
}
func (f *fakeBills) Delete(context.Context, string) error { return nil }
func (f *fakeBills) ReceivedTotalForMonth(context.Context, domain.YearMonth) (domain.Cents, error) {
	return 0, nil
}
func (f *fakeBills) OpenTotals(context.Context) (domain.Cents, domain.Cents, int, error) {
	return 0, 0, 0, nil
}

func seriesBill(id string, y int, m time.Month, d int, cents domain.Cents) domain.Bill {
	series, method, cat := "series-1", domain.PaymentPix, "cat-1"
	return domain.Bill{
		ID: id, Description: "Luz", AmountCents: cents,
		DueDate:   time.Date(y, m, d, 0, 0, 0, 0, time.UTC),
		Direction: domain.BillPayable, CategoryID: &cat,
		SeriesID: &series, PaymentMethod: &method,
	}
}

func serviceWith(f *fakeBills) *BillService {
	s := NewBillService(nil, nil, nil)
	s.bills = f
	return s
}

func TestMaterializeIsIdempotent(t *testing.T) {
	f := &fakeBills{bills: []domain.Bill{seriesBill("b1", 2026, time.September, 10, 18000)}}
	s := serviceWith(f)
	ctx := context.Background()

	if _, err := s.Materialize(ctx, domain.YearMonth{Year: 2026, Month: 10}); err != nil {
		t.Fatalf("materialize: %v", err)
	}
	if _, err := s.Materialize(ctx, domain.YearMonth{Year: 2026, Month: 10}); err != nil {
		t.Fatalf("materialize de novo: %v", err)
	}
	october, _ := f.ListByMonth(ctx, domain.YearMonth{Year: 2026, Month: 10}, nil)
	if len(october) != 1 {
		t.Fatalf("outubro tem %d contas, want 1", len(october))
	}
}

// Chegar em dezembro partindo de setembro cria outubro a partir de setembro,
// novembro a partir de outubro e dezembro a partir de novembro. Copiar
// sempre de setembro perderia a correção de valor feita em outubro.
func TestMaterializeChainsMonthsCarryingCorrectionsForward(t *testing.T) {
	f := &fakeBills{bills: []domain.Bill{seriesBill("b1", 2026, time.September, 10, 18000)}}
	s := serviceWith(f)
	ctx := context.Background()

	if _, err := s.Materialize(ctx, domain.YearMonth{Year: 2026, Month: 10}); err != nil {
		t.Fatalf("materialize outubro: %v", err)
	}
	// O dono corrige o valor de outubro quando a conta chega.
	for i := range f.bills {
		if f.bills[i].DueDate.Month() == time.October {
			f.bills[i].AmountCents = 25000
			f.bills[i].AmountEstimated = false
		}
	}
	if _, err := s.Materialize(ctx, domain.YearMonth{Year: 2026, Month: 12}); err != nil {
		t.Fatalf("materialize dezembro: %v", err)
	}

	for _, month := range []time.Month{time.November, time.December} {
		got, _ := f.ListByMonth(ctx, domain.YearMonth{Year: 2026, Month: int(month)}, nil)
		if len(got) != 1 {
			t.Fatalf("%s tem %d contas, want 1", month, len(got))
		}
		if got[0].AmountCents != 25000 {
			t.Errorf("%s herdou %d, want 25000 (a correção de outubro)", month, got[0].AmountCents)
		}
	}
}

func TestMaterializeDoesNotInventThePastBeforeTheSeriesStarted(t *testing.T) {
	f := &fakeBills{bills: []domain.Bill{seriesBill("b1", 2026, time.September, 10, 18000)}}
	s := serviceWith(f)
	ctx := context.Background()

	n, err := s.Materialize(ctx, domain.YearMonth{Year: 2026, Month: 7})
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}
	if n != 0 {
		t.Errorf("criou %d contas para um mês anterior à série, want 0", n)
	}
}

func TestMaterializeStopsAtTheSafetyCap(t *testing.T) {
	f := &fakeBills{bills: []domain.Bill{seriesBill("b1", 2026, time.September, 10, 18000)}}
	s := serviceWith(f)

	n, err := s.Materialize(context.Background(), domain.YearMonth{Year: 2040, Month: 1})
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}
	if n != 24 {
		t.Errorf("criou %d contas, want 24 (o teto)", n)
	}
}

func TestMaterializeIgnoresOneOffBills(t *testing.T) {
	one := seriesBill("b1", 2026, time.September, 10, 18000)
	one.SeriesID = nil
	f := &fakeBills{bills: []domain.Bill{one}}
	s := serviceWith(f)

	n, err := s.Materialize(context.Background(), domain.YearMonth{Year: 2026, Month: 12})
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}
	if n != 0 {
		t.Errorf("criou %d contas a partir de uma conta avulsa, want 0", n)
	}
}

// TestMaterializeGivesEachSeriesItsOwnCap builds two series that each need 15
// new occurrences to reach the target month — comfortably under
// materializeCap (24) on their own, but 30 combined, which is over a cap
// shared across all series. If the cap is shared, whichever series
// LatestPerSeries hands out second is starved and never reaches the target
// month; the owner would see an incomplete month with no signal why.
func TestMaterializeGivesEachSeriesItsOwnCap(t *testing.T) {
	seriesA := seriesBill("a1", 2026, time.September, 10, 18000)
	seriesB := seriesBill("b1", 2026, time.September, 10, 9000)
	seriesBID := "series-2"
	seriesB.SeriesID = &seriesBID

	f := &fakeBills{bills: []domain.Bill{seriesA, seriesB}}
	s := serviceWith(f)
	ctx := context.Background()

	target := domain.YearMonth{Year: 2026, Month: 9}.Add(15) // December 2027
	if _, err := s.Materialize(ctx, target); err != nil {
		t.Fatalf("materialize: %v", err)
	}

	for _, series := range []string{"series-1", "series-2"} {
		var latest time.Time
		count := 0
		for _, b := range f.bills {
			if b.SeriesID != nil && *b.SeriesID == series {
				count++
				if b.DueDate.After(latest) {
					latest = b.DueDate
				}
			}
		}
		got := domain.YearMonthOf(latest)
		if got.Before(target) {
			t.Errorf("%s only reached %v (want %v); has %d occurrences — starved by a shared cap", series, got, target, count)
		}
	}
}

// TestMaterializeSkipsASeriesWhoseLatestOccurrenceIsEnded guards the fix for
// the critical defect in the original spec: "deleting the last occurrence of
// a series ends the series" is unimplementable once Materialize runs on
// every month view (a fresh occurrence would just reappear on the very next
// GET). SeriesEnded, set only via EndSeries, is the explicit replacement.
func TestMaterializeSkipsASeriesWhoseLatestOccurrenceIsEnded(t *testing.T) {
	ended := seriesBill("b1", 2026, time.September, 10, 18000)
	ended.SeriesEnded = true
	f := &fakeBills{bills: []domain.Bill{ended}}
	s := serviceWith(f)
	ctx := context.Background()

	n, err := s.Materialize(ctx, domain.YearMonth{Year: 2026, Month: 11})
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}
	if n != 0 {
		t.Errorf("criou %d contas para uma série encerrada, want 0", n)
	}
	november, _ := f.ListByMonth(ctx, domain.YearMonth{Year: 2026, Month: 11}, nil)
	if len(november) != 0 {
		t.Errorf("novembro tem %d contas, want 0 (série encerrada em setembro)", len(november))
	}
}

// TestMaterializeResumesAfterASeriesIsUnended guards the mirror of the test
// above: clearing SeriesEnded (ResumeSeries) must let the series grow new
// occurrences again from wherever it left off.
func TestMaterializeResumesAfterASeriesIsUnended(t *testing.T) {
	ended := seriesBill("b1", 2026, time.September, 10, 18000)
	ended.SeriesEnded = true
	f := &fakeBills{bills: []domain.Bill{ended}}
	s := serviceWith(f)
	ctx := context.Background()

	if _, err := s.Materialize(ctx, domain.YearMonth{Year: 2026, Month: 11}); err != nil {
		t.Fatalf("materialize (ended): %v", err)
	}
	if _, err := s.ResumeSeries(ctx, "b1"); err != nil {
		t.Fatalf("resume series: %v", err)
	}
	n, err := s.Materialize(ctx, domain.YearMonth{Year: 2026, Month: 11})
	if err != nil {
		t.Fatalf("materialize (resumed): %v", err)
	}
	if n != 2 {
		t.Fatalf("criou %d contas ao retomar, want 2 (outubro e novembro)", n)
	}
	for _, month := range []time.Month{time.October, time.November} {
		got, _ := f.ListByMonth(ctx, domain.YearMonth{Year: 2026, Month: int(month)}, nil)
		if len(got) != 1 {
			t.Errorf("%s tem %d contas, want 1 depois de retomar a série", month, len(got))
		}
	}
}

// TestEndSeriesRefusesAOneOffBill and its ResumeSeries mirror guard that
// these dedicated endpoints only ever act on an actual series — a one-off
// bill has no series to end or resume.
func TestEndSeriesRefusesAOneOffBill(t *testing.T) {
	f := &fakeBills{bills: []domain.Bill{{ID: "b1"}}}
	s := serviceWith(f)

	if _, err := s.EndSeries(context.Background(), "b1"); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("EndSeries numa conta avulsa = %v, want domain.ErrValidation", err)
	}
	if _, err := s.ResumeSeries(context.Background(), "b1"); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("ResumeSeries numa conta avulsa = %v, want domain.ErrValidation", err)
	}
}

// TestMaterializeSeedsNextOccurrenceEstimatedFromAmountVaries guards
// Important 1: a series whose amount varies must keep marking new
// occurrences as estimated even after one of them was paid (which clears
// only that occurrence's own AmountEstimated, never the series' AmountVaries)
// — collapsing the two into one column made the marker say the opposite of
// the truth the very next month.
func TestMaterializeSeedsNextOccurrenceEstimatedFromAmountVaries(t *testing.T) {
	paidAt := time.Date(2026, time.September, 12, 0, 0, 0, 0, time.UTC)
	paidSeptember := seriesBill("b1", 2026, time.September, 10, 18000)
	paidSeptember.AmountVaries = true
	// Mirrors what BillRepo.Pay does to the row: it clears AmountEstimated
	// and nothing else — AmountVaries is untouched.
	paidSeptember.AmountEstimated = false
	paidSeptember.PaidAt = &paidAt

	f := &fakeBills{bills: []domain.Bill{paidSeptember}}
	s := serviceWith(f)
	ctx := context.Background()

	if _, err := s.Materialize(ctx, domain.YearMonth{Year: 2026, Month: 10}); err != nil {
		t.Fatalf("materialize: %v", err)
	}
	october, _ := f.ListByMonth(ctx, domain.YearMonth{Year: 2026, Month: 10}, nil)
	if len(october) != 1 {
		t.Fatalf("outubro tem %d contas, want 1", len(october))
	}
	if !october[0].AmountEstimated {
		t.Error("outubro deveria nascer estimada: a série varia (amount_varies), mesmo setembro já paga não devendo importar")
	}
	if !october[0].AmountVaries {
		t.Error("amount_varies deveria continuar true em outubro")
	}
}

// TestListByMonthMaterializesBeforeListing guards the behaviour the whole
// point of Task 5 depends on: without ListByMonth calling Materialize first,
// "repetir todo mês" repeats nothing. Deleting the Materialize call, or
// moving it after the list read, must fail this test.
func TestListByMonthMaterializesBeforeListing(t *testing.T) {
	f := &fakeBills{bills: []domain.Bill{seriesBill("b1", 2026, time.September, 10, 18000)}}
	s := serviceWith(f)

	got, err := s.ListByMonth(context.Background(), domain.YearMonth{Year: 2026, Month: 10}, nil)
	if err != nil {
		t.Fatalf("list by month: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("outubro tem %d contas, want 1 (Materialize deveria ter rodado antes de listar)", len(got))
	}
}

// TestListDoesNotMaterialize guards the other half: the plain List path (no
// month), which the home dashboard uses to show upcoming bills, must never
// write anything — browsing the home page is not something that should
// mutate the bills table.
func TestListDoesNotMaterialize(t *testing.T) {
	f := &fakeBills{bills: []domain.Bill{seriesBill("b1", 2026, time.September, 10, 18000)}}
	s := serviceWith(f)

	if _, err := s.List(context.Background(), nil, false); err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(f.bills) != 1 {
		t.Fatalf("List gravou %d contas a mais; want nenhuma (List não deve materializar)", len(f.bills)-1)
	}
}

// TestServiceNeverInventsASeriesID now guards the Update side only: Update
// loads the bill as it stands in the database (via Get) and carries its
// SeriesID forward untouched, regardless of what NewBillInput says — the
// wire has no series_id field, so trusting in.SeriesID (always nil from an
// HTTP request) would silently un-series a bill on every edit, and trusting
// in.Recurring would let an edit mint a fresh series and split it in two.
// An earlier version of this code minted a series id from the service and
// that caused two critical bugs before it was removed; Create is now the
// one place allowed to mint (see TestServiceCreateMintsASeriesIDWhenRecurring
// below) — Update must never do either.
func TestServiceNeverInventsASeriesID(t *testing.T) {
	ctx := context.Background()

	t.Run("update keeps the bill's own SeriesID unchanged, ignoring the request's", func(t *testing.T) {
		series := "series-1"
		other := "some-other-series"
		f := &fakeBills{bills: []domain.Bill{{ID: "b1", SeriesID: &series}}}
		s := serviceWith(f)

		updated, err := s.Update(ctx, "b1", NewBillInput{
			Description: "Luz",
			AmountCents: 25000,
			DueDate:     time.Date(2026, time.October, 10, 0, 0, 0, 0, time.UTC),
			Direction:   domain.BillReceivable,
			// A request that names a DIFFERENT series must still be ignored:
			// series membership is never taken from the wire.
			SeriesID: &other,
		})
		if err != nil {
			t.Fatalf("update: %v", err)
		}
		if updated.SeriesID == nil || *updated.SeriesID != series {
			t.Fatalf("SeriesID = %v, want unchanged %q (the bill's own, not the request's)", updated.SeriesID, series)
		}
	})

	t.Run("update with Recurring but no prior SeriesID leaves SeriesID nil", func(t *testing.T) {
		f := &fakeBills{bills: []domain.Bill{{ID: "b2"}}}
		s := serviceWith(f)

		updated, err := s.Update(ctx, "b2", NewBillInput{
			Description: "Internet",
			AmountCents: 12000,
			DueDate:     time.Date(2026, time.October, 5, 0, 0, 0, 0, time.UTC),
			Direction:   domain.BillReceivable,
			Recurring:   true,
		})
		if err != nil {
			t.Fatalf("update: %v", err)
		}
		if updated.SeriesID != nil {
			t.Fatalf("SeriesID = %v, want nil (Recurring alone must not make Update mint a series id)", updated.SeriesID)
		}
	})

	t.Run("update carries PaidAt and TransactionID forward from the database", func(t *testing.T) {
		paidAt := time.Date(2026, time.September, 10, 0, 0, 0, 0, time.UTC)
		txnID := "txn-1"
		f := &fakeBills{bills: []domain.Bill{{ID: "b3", PaidAt: &paidAt, TransactionID: &txnID}}}
		s := serviceWith(f)

		updated, err := s.Update(ctx, "b3", NewBillInput{
			Description: "Aluguel",
			AmountCents: 150000,
			DueDate:     time.Date(2026, time.October, 5, 0, 0, 0, 0, time.UTC),
			Direction:   domain.BillPayable,
		})
		if err != nil {
			t.Fatalf("update: %v", err)
		}
		if updated.PaidAt == nil || !updated.PaidAt.Equal(paidAt) {
			t.Errorf("PaidAt = %v, want unchanged %v", updated.PaidAt, paidAt)
		}
		if updated.TransactionID == nil || *updated.TransactionID != txnID {
			t.Errorf("TransactionID = %v, want unchanged %q", updated.TransactionID, txnID)
		}
	})
}

// TestServiceCreateMintsASeriesIDWhenRecurring guards the other half of the
// same ruling: Create (and only Create) mints a fresh series id when the
// caller marks a bill recurring and doesn't already supply one. This is safe
// now because the Contas form supplies category and payment method, so
// domain.Bill.Validate's requirement for a recurring payable bill is met.
func TestServiceCreateMintsASeriesIDWhenRecurring(t *testing.T) {
	f := &fakeBills{}
	cats := fakeCategories{known: map[string]domain.Category{"cat-1": {ID: "cat-1"}}}
	s := serviceWithPay(f, cats, fakeCards{})
	ctx := context.Background()

	method := domain.PaymentPix
	cat := "cat-1"
	created, err := s.Create(ctx, NewBillInput{
		Description:   "Luz",
		AmountCents:   18000,
		DueDate:       time.Date(2026, time.September, 10, 0, 0, 0, 0, time.UTC),
		Direction:     domain.BillPayable,
		CategoryID:    &cat,
		PaymentMethod: &method,
		Recurring:     true,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.SeriesID == nil || *created.SeriesID == "" {
		t.Fatal("SeriesID = nil, want a freshly minted series id")
	}

	t.Run("create without Recurring leaves SeriesID nil", func(t *testing.T) {
		created, err := s.Create(ctx, NewBillInput{
			Description: "Compra avulsa",
			AmountCents: 5000,
			DueDate:     time.Date(2026, time.September, 10, 0, 0, 0, 0, time.UTC),
			Direction:   domain.BillReceivable,
		})
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if created.SeriesID != nil {
			t.Fatalf("SeriesID = %v, want nil (a one-off bill must not get a series)", created.SeriesID)
		}
	})

	t.Run("create honors a caller-supplied SeriesID instead of minting a new one", func(t *testing.T) {
		series := "series-existing"
		created, err := s.Create(ctx, NewBillInput{
			Description:   "Luz",
			AmountCents:   18000,
			DueDate:       time.Date(2026, time.October, 10, 0, 0, 0, 0, time.UTC),
			Direction:     domain.BillPayable,
			CategoryID:    &cat,
			PaymentMethod: &method,
			SeriesID:      &series,
			Recurring:     true,
		})
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if created.SeriesID == nil || *created.SeriesID != series {
			t.Fatalf("SeriesID = %v, want the caller's own %q, not a fresh one", created.SeriesID, series)
		}
	})
}

// TestServicePayRefusesAReceivableBill guards the rule that only a payable
// bill settlement records an expense: the transactions ledger is an expense
// ledger, and a credit there would be a negative every aggregate would have
// to special-case.
func TestServicePayRefusesAReceivableBill(t *testing.T) {
	f := &fakeBills{bills: []domain.Bill{{ID: "b1", Description: "Freela", Direction: domain.BillReceivable}}}
	s := serviceWithPay(f, fakeCategories{}, fakeCards{})

	_, _, err := s.Pay(context.Background(), "b1", PaymentInput{
		PaidAt: time.Now(), AmountCents: 1000, CategoryID: "cat-1", PaymentMethod: domain.PaymentPix,
	})
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("Pay numa conta a receber = %v, want domain.ErrValidation", err)
	}
}

// TestServicePayRefusesCreditWithoutACard guards the same rule enforced
// elsewhere for regular purchases: a credit expense with no card to book an
// invoice against is not a payment method, it's a missing field.
func TestServicePayRefusesCreditWithoutACard(t *testing.T) {
	f := &fakeBills{bills: []domain.Bill{{ID: "b1", Description: "Luz", Direction: domain.BillPayable}}}
	cats := fakeCategories{known: map[string]domain.Category{"cat-1": {ID: "cat-1"}}}
	s := serviceWithPay(f, cats, fakeCards{})

	_, _, err := s.Pay(context.Background(), "b1", PaymentInput{
		PaidAt: time.Now(), AmountCents: 1000, CategoryID: "cat-1", PaymentMethod: domain.PaymentCredit,
	})
	if !errors.Is(err, domain.ErrCreditCardMissing) {
		t.Fatalf("Pay a crédito sem cartão = %v, want domain.ErrCreditCardMissing", err)
	}
}

// TestServicePayBuildsTheExpenseFromThePaymentInput is the test a fakeBills
// that discards its Pay argument would let sail through: it pins down that
// the expense uses the CONFIRMED amount and category (not the bill's own),
// that a credit payment's competence month is the invoice's due month (not
// the calendar month the owner clicked "pago" in), and that IsRecurring
// mirrors the bill's series membership.
func TestServicePayBuildsTheExpenseFromThePaymentInput(t *testing.T) {
	series := "series-1"
	bill := domain.Bill{
		ID: "b1", Description: "Cartão de crédito (teste)", AmountCents: 9999,
		Direction: domain.BillPayable, SeriesID: &series,
	}
	f := &fakeBills{bills: []domain.Bill{bill}}
	cats := fakeCategories{known: map[string]domain.Category{"cat-1": {ID: "cat-1"}}}
	// Closing on the 25th, due on the 15th: a purchase made ON the closing
	// day (September 25) still closes in September (buying on the closing
	// day is one more day of grace), and a due day at or before the closing
	// day belongs to the NEXT month — so the invoice is due in October, not
	// September. If CompetenceMonth ever used the payment month instead of
	// this card math, this test would catch it.
	card := domain.CreditCard{ID: "card-1", Name: "Cartão de teste", ClosingDay: 25, DueDay: 15}
	cards := fakeCards{known: map[string]domain.CreditCard{"card-1": card}}
	s := serviceWithPay(f, cats, cards)

	paidAt := time.Date(2026, time.September, 25, 0, 0, 0, 0, time.UTC)
	cardID := "card-1"
	_, expense, err := s.Pay(context.Background(), "b1", PaymentInput{
		PaidAt: paidAt,
		// Deliberately different from bill.AmountCents (9999), to prove Pay
		// uses the confirmed amount, not the bill's original estimate.
		AmountCents:   4321,
		CategoryID:    "cat-1",
		PaymentMethod: domain.PaymentCredit,
		CreditCardID:  &cardID,
	})
	if err != nil {
		t.Fatalf("pay: %v", err)
	}
	if expense.AmountCents != 4321 {
		t.Errorf("expense.AmountCents = %d, want 4321 (o valor confirmado, não o valor original da conta)", expense.AmountCents)
	}
	if expense.CategoryID != "cat-1" {
		t.Errorf("expense.CategoryID = %q, want cat-1", expense.CategoryID)
	}
	wantMonth := domain.YearMonth{Year: 2026, Month: 10}
	if expense.CompetenceMonth != wantMonth {
		t.Errorf("expense.CompetenceMonth = %v, want %v (o mês de vencimento da fatura, não o mês do pagamento)", expense.CompetenceMonth, wantMonth)
	}
	if !expense.IsRecurring {
		t.Error("expense.IsRecurring deveria ser true: a conta pertence a uma série")
	}
	if f.paidExpense == nil || f.paidExpense.CompetenceMonth != wantMonth {
		t.Error("o repositório não recebeu o mesmo lançamento devolvido pelo serviço")
	}
}
