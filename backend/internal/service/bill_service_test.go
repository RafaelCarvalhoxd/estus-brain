package service

import (
	"strings"
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
// MarkPaid mutates the matching bill in place, like Pay and SetSeriesEnded
// do, so TestMarkPaidRefusesAPayableBill and its receivable counterpart can
// tell a real settlement from BillService's guard never even reaching here.
func (f *fakeBills) MarkPaid(_ context.Context, id string, paidAt time.Time) (domain.Bill, error) {
	for i := range f.bills {
		if f.bills[i].ID == id {
			f.bills[i].PaidAt = &paidAt
			return f.bills[i], nil
		}
	}
	return domain.Bill{}, domain.ErrNotFound
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

// SetSeriesEnded mirrors the fixed BillRepo: it flips SeriesEnded on EVERY
// occurrence of id's series, not just the row named by id — the same
// series-wide update Finding 2 put in the real SQL, so a fake that stayed
// row-scoped would let a service-level test believe Materialize is fixed
// when only the postgres layer's bug was.
func (f *fakeBills) SetSeriesEnded(_ context.Context, id string, ended bool) (domain.Bill, error) {
	var series *string
	for _, b := range f.bills {
		if b.ID == id {
			series = b.SeriesID
			break
		}
	}
	if series == nil {
		return domain.Bill{}, domain.ErrNotFound
	}
	var out domain.Bill
	found := false
	for i := range f.bills {
		if f.bills[i].SeriesID != nil && *f.bills[i].SeriesID == *series {
			f.bills[i].SeriesEnded = ended
			if f.bills[i].ID == id {
				out = f.bills[i]
				found = true
			}
		}
	}
	if !found {
		return domain.Bill{}, domain.ErrNotFound
	}
	return out, nil
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
// Delete actually removes the row, so TestDeleteEndsTheSeriesWhenDeletingTheLatestActiveOccurrence
// can materialize afterwards and see whether the (now former) latest
// occurrence's SeriesEnded stuck on what remains of the series.
func (f *fakeBills) SyncInvoices(context.Context, domain.YearMonth) error { return nil }

func (f *fakeBills) DeleteUnpaidAfter(_ context.Context, id string) (int, error) {
	var ref *domain.Bill
	for i := range f.bills {
		if f.bills[i].ID == id {
			ref = &f.bills[i]
		}
	}
	if ref == nil || ref.SeriesID == nil {
		return 0, nil
	}
	series, due := *ref.SeriesID, ref.DueDate
	kept := f.bills[:0]
	deleted := 0
	for _, b := range f.bills {
		if b.SeriesID != nil && *b.SeriesID == series && b.DueDate.After(due) && b.PaidAt == nil {
			deleted++
			continue
		}
		kept = append(kept, b)
	}
	f.bills = kept
	return deleted, nil
}

func (f *fakeBills) Delete(_ context.Context, id string) error {
	for i, b := range f.bills {
		if b.ID == id {
			f.bills = append(f.bills[:i], f.bills[i+1:]...)
			return nil
		}
	}
	return domain.ErrNotFound
}
func (f *fakeBills) ReceivedTotalForMonth(context.Context, domain.YearMonth) (domain.Cents, error) {
	return 0, nil
}
func (f *fakeBills) OpenTotals(context.Context, domain.YearMonth, time.Time) (domain.Cents, domain.Cents, int, error) {
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

	if _, err := s.EndSeries(context.Background(), "b1", false); !errors.Is(err, domain.ErrValidation) {
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

// TestDeleteEndsTheSeriesWhenDeletingTheLatestActiveOccurrence guards
// Finding 3: deleting the latest occurrence of an active series used to
// undo itself, because ListByMonth's Materialize call on the next view
// recreated the exact row just deleted (copied from the occurrence before
// it), silently discarding whatever correction the owner had made to it.
// Ending the series as part of the same Delete is what makes the delete
// stick.
func TestDeleteEndsTheSeriesWhenDeletingTheLatestActiveOccurrence(t *testing.T) {
	september := seriesBill("b1", 2026, time.September, 10, 18000)
	f := &fakeBills{bills: []domain.Bill{september}}
	s := serviceWith(f)
	ctx := context.Background()

	// October is materialized (and, per the branch's real bug, corrected):
	// the owner changes the power bill from 18000 to 21200 once it arrives.
	if _, err := s.Materialize(ctx, domain.YearMonth{Year: 2026, Month: 10}); err != nil {
		t.Fatalf("materialize outubro: %v", err)
	}
	var octoberID string
	for i := range f.bills {
		if f.bills[i].DueDate.Month() == time.October {
			f.bills[i].AmountCents = 21200
			f.bills[i].AmountEstimated = false
			octoberID = f.bills[i].ID
		}
	}
	if octoberID == "" {
		t.Fatal("outubro não foi materializado")
	}

	if err := s.Delete(ctx, octoberID); err != nil {
		t.Fatalf("delete outubro: %v", err)
	}
	for _, b := range f.bills {
		if b.ID == octoberID {
			t.Fatal("outubro ainda existe depois do delete")
		}
	}

	// Reopening a later month must not recreate October, nor grow past it:
	// deleting the series' latest occurrence must have ended the series.
	n, err := s.Materialize(ctx, domain.YearMonth{Year: 2026, Month: 12})
	if err != nil {
		t.Fatalf("materialize dezembro: %v", err)
	}
	if n != 0 {
		t.Errorf("materialize criou %d contas depois de apagar a última ocorrência de uma série ativa; want 0 (a série deveria ter sido encerrada)", n)
	}
}

// TestDeleteOfANonLatestOccurrenceDoesNotEndTheSeries guards the other half:
// deleting a PAST occurrence (not the series' latest) must stay a plain
// delete — the series is still growing, and nothing should stop it just
// because an old row was removed.
func TestDeleteOfANonLatestOccurrenceDoesNotEndTheSeries(t *testing.T) {
	september := seriesBill("b1", 2026, time.September, 10, 18000)
	f := &fakeBills{bills: []domain.Bill{september}}
	s := serviceWith(f)
	ctx := context.Background()

	if _, err := s.Materialize(ctx, domain.YearMonth{Year: 2026, Month: 10}); err != nil {
		t.Fatalf("materialize outubro: %v", err)
	}
	if err := s.Delete(ctx, "b1"); err != nil {
		t.Fatalf("delete setembro (não é a última ocorrência): %v", err)
	}

	n, err := s.Materialize(ctx, domain.YearMonth{Year: 2026, Month: 11})
	if err != nil {
		t.Fatalf("materialize novembro: %v", err)
	}
	if n != 1 {
		t.Errorf("materialize criou %d contas, want 1 (novembro); a série não deveria ter sido encerrada por apagar setembro", n)
	}
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

// TestServiceCreateRejectsAnUnknownCreditCard guards the card-existence check
// added alongside category's: a recurring bill saved with credito needs a
// real card, the same way it needs a real category, so a stale or
// hand-crafted id from the client can't slip through and only fail later,
// silently, the first time the bill is paid.
func TestServiceCreateRejectsAnUnknownCreditCard(t *testing.T) {
	f := &fakeBills{}
	cats := fakeCategories{known: map[string]domain.Category{"cat-1": {ID: "cat-1"}}}
	s := serviceWithPay(f, cats, fakeCards{})
	ctx := context.Background()

	method := domain.PaymentCredit
	cat := "cat-1"
	card := "no-such-card"
	_, err := s.Create(ctx, NewBillInput{
		Description:   "Netflix",
		AmountCents:   4000,
		DueDate:       time.Date(2026, time.September, 10, 0, 0, 0, 0, time.UTC),
		Direction:     domain.BillPayable,
		CategoryID:    &cat,
		PaymentMethod: &method,
		CreditCardID:  &card,
		Recurring:     true,
	})
	if err == nil {
		t.Fatal("create com cartão inexistente = nil, want erro")
	}
}

// TestMarkPaidRefusesAPayableBill guards Finding 1 of the whole-branch
// review: MarkPaid used to have no direction check at all, so
// POST /api/bills/{id}/paid on a payable bill (and the assistant's
// bills_mark_paid tool, which called MarkPaid directly) could stamp paid_at
// on a payable bill and leave it with no expense behind it — silently
// reproducing the exact balance bug this branch exists to fix, just through
// a different door than Pay.
func TestMarkPaidRefusesAPayableBill(t *testing.T) {
	f := &fakeBills{bills: []domain.Bill{{ID: "b1", Description: "Luz", Direction: domain.BillPayable}}}
	s := serviceWith(f)

	_, err := s.MarkPaid(context.Background(), "b1", time.Now())
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("MarkPaid numa conta a pagar = %v, want domain.ErrValidation", err)
	}
	if f.bills[0].PaidAt != nil {
		t.Error("a conta a pagar não deveria ter sido marcada como paga")
	}
}

// TestMarkPaidStillWorksOnAReceivable is the mirror: the guard added for
// Finding 1 must not break the one case MarkPaid exists for — settling money
// coming in, which has no expense to record.
func TestMarkPaidStillWorksOnAReceivable(t *testing.T) {
	f := &fakeBills{bills: []domain.Bill{{ID: "b1", Description: "Freela", Direction: domain.BillReceivable}}}
	s := serviceWith(f)

	paid, err := s.MarkPaid(context.Background(), "b1", time.Now())
	if err != nil {
		t.Fatalf("MarkPaid numa conta a receber: %v", err)
	}
	if paid.PaidAt == nil {
		t.Error("a conta a receber deveria ter sido marcada como paga")
	}
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
	// The expense counts in the month it was paid; the card invoice it lands
	// on is the month that invoice is due.
	wantMonth := domain.YearMonth{Year: 2026, Month: 9}
	if expense.CompetenceMonth != wantMonth {
		t.Errorf("expense.CompetenceMonth = %v, want %v (o mês do pagamento)", expense.CompetenceMonth, wantMonth)
	}
	if want := (domain.YearMonth{Year: 2026, Month: 10}); expense.InvoiceMonth == nil || *expense.InvoiceMonth != want {
		t.Errorf("expense.InvoiceMonth = %v, want %v (o mês de vencimento da fatura)", expense.InvoiceMonth, want)
	}
	if !expense.IsRecurring {
		t.Error("expense.IsRecurring deveria ser true: a conta pertence a uma série")
	}
	if f.paidExpense == nil || f.paidExpense.CompetenceMonth != wantMonth {
		t.Error("o repositório não recebeu o mesmo lançamento devolvido pelo serviço")
	}
}

func TestEndSeriesCanDeleteTheFollowingUnpaidOccurrences(t *testing.T) {
	sep := seriesBill("sep", 2026, time.September, 10, 18000)
	oct := seriesBill("oct", 2026, time.October, 10, 18000)
	nov := seriesBill("nov", 2026, time.November, 10, 18000)
	dec := seriesBill("dec", 2026, time.December, 10, 18000)
	paidAt := time.Date(2026, time.September, 20, 0, 0, 0, 0, time.UTC)
	dec.PaidAt = &paidAt
	f := &fakeBills{bills: []domain.Bill{sep, oct, nov, dec}}
	s := serviceWith(f)
	ctx := context.Background()

	if _, err := s.EndSeries(ctx, "oct", true); err != nil {
		t.Fatalf("end series: %v", err)
	}
	var ids []string
	for _, b := range f.bills {
		ids = append(ids, b.ID)
		if !b.SeriesEnded {
			t.Errorf("%s: series not ended", b.ID)
		}
	}
	// November goes; December is paid, so it stays.
	if want := "sep,oct,dec"; strings.Join(ids, ",") != want {
		t.Fatalf("bills = %v, want %s", ids, want)
	}
}

func TestEndSeriesKeepsTheFollowingOccurrencesByDefault(t *testing.T) {
	f := &fakeBills{bills: []domain.Bill{
		seriesBill("sep", 2026, time.September, 10, 18000),
		seriesBill("oct", 2026, time.October, 10, 18000),
	}}
	if _, err := serviceWith(f).EndSeries(context.Background(), "sep", false); err != nil {
		t.Fatalf("end series: %v", err)
	}
	if len(f.bills) != 2 {
		t.Fatalf("got %d bills, want 2", len(f.bills))
	}
}

func TestCardInvoiceIsMarkedPaidWithoutAnExpense(t *testing.T) {
	card := "card-1"
	invoice := domain.Bill{
		ID: "inv", Description: "Fatura Nubank", AmountCents: 50000,
		DueDate:   time.Date(2026, time.October, 10, 0, 0, 0, 0, time.UTC),
		Direction: domain.BillPayable, InvoiceCardID: &card,
	}
	f := &fakeBills{bills: []domain.Bill{invoice}}
	s := serviceWith(f)
	ctx := context.Background()
	paidAt := time.Date(2026, time.October, 9, 0, 0, 0, 0, time.UTC)

	if _, _, err := s.Pay(ctx, "inv", PaymentInput{PaidAt: paidAt, AmountCents: 50000, CategoryID: "cat-1", PaymentMethod: domain.PaymentPix}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("Pay on an invoice: err = %v, want ErrValidation", err)
	}
	paid, err := s.MarkPaid(ctx, "inv", paidAt)
	if err != nil {
		t.Fatalf("MarkPaid: %v", err)
	}
	if paid.PaidAt == nil || paid.TransactionID != nil {
		t.Fatalf("paid = %+v, want paid with no transaction", paid)
	}
	if err := s.Delete(ctx, "inv"); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("Delete on an invoice: err = %v, want ErrValidation", err)
	}
}
