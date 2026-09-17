package service

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/rafael/estus-vault/backend/internal/domain"
	"github.com/rafael/estus-vault/backend/internal/store/postgres"
)

// TestEndSeriesFromANonLatestOccurrenceStopsMaterialize guards Finding 2 of
// the whole-branch review against the real database, not just the in-memory
// fake: SetSeriesEnded used to stamp only the CLICKED row, but Materialize
// only ever reads LatestPerSeries. Ending the series from an occurrence that
// is not the latest (the owner navigates ahead, comes back, and clicks
// "Encerrar repetição" on an older month) used to leave the actual latest
// occurrence unmarked, so the series kept growing every month forever while
// the clicked row lied on screen, showing "· Repetição encerrada".
//
// This test is run against the real BillRepo on purpose: the bug lived
// entirely in the SQL (`where id = $1` instead of a series-wide update), so
// a test against bill_service_test.go's in-memory fakeBills cannot exercise
// it — the fake would have to already encode the fix to be useful, which
// proves nothing about the actual repository.
func TestEndSeriesFromANonLatestOccurrenceStopsMaterialize(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	db, err := postgres.Connect(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer db.Close()
	if err := db.Migrate(ctx, "../../migrations"); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	bills := postgres.NewBillRepo(db)
	categories := postgres.NewCategoryRepo(db)
	cards := postgres.NewCreditCardRepo(db)
	svc := NewBillService(bills, categories, cards)

	cat, err := categories.Create(ctx, domain.Category{
		Name: "Categoria de teste (encerrar série não-mais-recente)", Nature: "essencial", Color: "#0f0f0f",
	})
	if err != nil {
		t.Fatalf("create category: %v", err)
	}
	defer func() {
		if err := categories.Delete(ctx, cat.ID); err != nil {
			t.Errorf("cleanup: delete category %s: %v", cat.ID, err)
		}
	}()

	series := domain.NewID()
	method := domain.PaymentPix
	september, err := bills.Create(ctx, domain.Bill{
		Description: "Netflix (teste)", AmountCents: 5000,
		DueDate: time.Date(2026, time.September, 10, 0, 0, 0, 0, time.UTC),
		Direction: domain.BillPayable, CategoryID: &cat.ID,
		SeriesID: &series, PaymentMethod: &method,
	})
	if err != nil {
		t.Fatalf("create september: %v", err)
	}
	defer func() {
		if err := bills.Delete(ctx, september.ID); err != nil && err != domain.ErrNotFound {
			t.Errorf("cleanup: delete september bill %s: %v", september.ID, err)
		}
	}()

	// The owner navigates ahead to October (materializing it), then goes
	// back to September.
	created, err := svc.Materialize(ctx, domain.YearMonth{Year: 2026, Month: 10})
	if err != nil {
		t.Fatalf("materialize october: %v", err)
	}
	if created != 1 {
		t.Fatalf("materialize criou %d contas, want 1 (outubro)", created)
	}
	octoberList, err := bills.ListByMonth(ctx, domain.YearMonth{Year: 2026, Month: 10}, nil)
	if err != nil {
		t.Fatalf("list october: %v", err)
	}
	if len(octoberList) != 1 {
		t.Fatalf("outubro tem %d contas, want 1", len(octoberList))
	}
	october := octoberList[0]
	defer func() {
		if err := bills.Delete(ctx, october.ID); err != nil && err != domain.ErrNotFound {
			t.Errorf("cleanup: delete october bill %s: %v", october.ID, err)
		}
	}()

	// From the September row — NOT the latest occurrence — the owner clicks
	// "Encerrar repetição".
	if _, err := svc.EndSeries(ctx, september.ID); err != nil {
		t.Fatalf("end series from september: %v", err)
	}

	// The whole point of the fix: opening a much later month must not
	// materialize anything more, because the series was actually ended.
	n, err := svc.Materialize(ctx, domain.YearMonth{Year: 2026, Month: 12})
	if err != nil {
		t.Fatalf("materialize december: %v", err)
	}
	if n != 0 {
		t.Errorf("materialize criou %d contas depois de encerrar a série a partir de uma ocorrência não-mais-recente; want 0", n)
	}
	novemberOrLater, err := bills.ListByMonth(ctx, domain.YearMonth{Year: 2026, Month: 11}, nil)
	if err != nil {
		t.Fatalf("list november: %v", err)
	}
	if len(novemberOrLater) != 0 {
		t.Errorf("novembro tem %d contas, want 0 (a série deveria estar encerrada, incluindo na ocorrência mais recente, outubro)", len(novemberOrLater))
	}

	// The latest occurrence itself (October) must show the flag too — that
	// is what Materialize actually reads, and what keeps the label
	// consistent across the whole series on screen.
	refreshedOctober, err := bills.Get(ctx, october.ID)
	if err != nil {
		t.Fatalf("get october: %v", err)
	}
	if !refreshedOctober.SeriesEnded {
		t.Error("outubro (a ocorrência mais recente) não ficou com series_ended=true depois de encerrar a série a partir de setembro")
	}
}
