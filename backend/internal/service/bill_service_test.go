package service

import (
	"context"
	"testing"
	"time"

	"github.com/rafael/estus-vault/backend/internal/domain"
)

// fakeBills keeps bills in memory, which is all Materialize needs.
type fakeBills struct {
	bills []domain.Bill
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

func (f *fakeBills) Pay(context.Context, string, time.Time, domain.Transaction) (domain.Bill, error) {
	return domain.Bill{}, nil
}

func (f *fakeBills) Unpay(context.Context, string) (domain.Bill, error) {
	return domain.Bill{}, nil
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

// TestServiceNeverInventsASeriesID guards the ruling behind Create and
// Update: the service passes the caller's SeriesID through verbatim and
// never mints one of its own. An earlier version of this code did mint a
// series id on the service side and that caused two critical bugs before it
// was removed — this test is here so nobody re-adds it by accident. Minting
// belongs to a later task, once the form has the fields (category, payment
// method) that make a payable recurring bill valid.
func TestServiceNeverInventsASeriesID(t *testing.T) {
	f := &fakeBills{}
	s := serviceWith(f)
	ctx := context.Background()

	t.Run("update keeps the caller's SeriesID unchanged", func(t *testing.T) {
		series := "series-1"
		updated, err := s.Update(ctx, "b1", NewBillInput{
			Description: "Luz",
			AmountCents: 25000,
			DueDate:     time.Date(2026, time.October, 10, 0, 0, 0, 0, time.UTC),
			Direction:   domain.BillReceivable,
			SeriesID:    &series,
		})
		if err != nil {
			t.Fatalf("update: %v", err)
		}
		if updated.SeriesID == nil || *updated.SeriesID != series {
			t.Fatalf("SeriesID = %v, want unchanged %q", updated.SeriesID, series)
		}
	})

	t.Run("update with Recurring but no SeriesID leaves SeriesID nil", func(t *testing.T) {
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

	t.Run("create with Recurring but no SeriesID leaves SeriesID nil", func(t *testing.T) {
		created, err := s.Create(ctx, NewBillInput{
			Description: "Luz",
			AmountCents: 18000,
			DueDate:     time.Date(2026, time.September, 10, 0, 0, 0, 0, time.UTC),
			Direction:   domain.BillReceivable,
			Recurring:   true,
		})
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if created.SeriesID != nil {
			t.Fatalf("SeriesID = %v, want nil (Recurring alone must not invent a series id)", created.SeriesID)
		}
	})
}
