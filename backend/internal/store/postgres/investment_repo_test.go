package postgres

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/rafael/estus-vault/backend/internal/domain"
)

func TestInvestmentRepo(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	db, err := Connect(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer db.Close()
	repo := NewInvestmentRepo(db)

	// A month far from any real data, so the totals below are only ours.
	ym := domain.YearMonth{Year: 2091, Month: 3}
	day := func(d int) time.Time { return time.Date(2091, time.March, d, 0, 0, 0, 0, time.UTC) }

	inv, err := repo.Create(ctx, domain.Investment{
		Name: "CDB teste", Kind: domain.InvestmentCDB, RateBP: 100, RatePeriod: domain.RatePerMonth,
	}, domain.InvestmentContribution{AmountCents: 100_000, Date: day(5)})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer repo.Delete(ctx, inv.ID)

	second, err := repo.AddContribution(ctx, domain.InvestmentContribution{InvestmentID: inv.ID, AmountCents: 25_000, Date: day(20)})
	if err != nil {
		t.Fatalf("add contribution: %v", err)
	}
	if _, err := repo.AddContribution(ctx, domain.InvestmentContribution{
		InvestmentID: "00000000-0000-0000-0000-000000000000", AmountCents: 1, Date: day(1),
	}); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("contribution to missing investment = %v, want ErrNotFound", err)
	}

	total, err := repo.MonthTotal(ctx, ym)
	if err != nil {
		t.Fatalf("month total: %v", err)
	}
	if total != 125_000 {
		t.Errorf("month total = %d, want 125000", total)
	}
	if next, _ := repo.MonthTotal(ctx, ym.Add(1)); next != 0 {
		t.Errorf("next month total = %d, want 0", next)
	}

	list, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var found *domain.Investment
	for i := range list {
		if list[i].ID == inv.ID {
			found = &list[i]
		}
	}
	if found == nil || len(found.Contributions) != 2 || found.InvestedCents() != 125_000 {
		t.Fatalf("listed investment = %+v", found)
	}

	inv.Name, inv.RateBP, inv.RatePeriod = "CDB renomeado", 1200, domain.RatePerYear
	if err := repo.Update(ctx, inv); err != nil {
		t.Fatalf("update: %v", err)
	}
	if err := repo.Update(ctx, domain.Investment{ID: "nao-e-uuid", Name: "x", Kind: domain.InvestmentCDB, RatePeriod: domain.RatePerYear}); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("update with bad id = %v, want ErrNotFound", err)
	}

	if err := repo.DeleteContribution(ctx, inv.ID, second.ID); err != nil {
		t.Fatalf("delete contribution: %v", err)
	}
	first := found.Contributions[0].ID
	if err := repo.DeleteContribution(ctx, inv.ID, first); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("deleting the last contribution = %v, want ErrConflict", err)
	}

	if err := repo.Delete(ctx, inv.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if total, _ := repo.MonthTotal(ctx, ym); total != 0 {
		t.Errorf("month total after delete = %d, want 0", total)
	}
}
