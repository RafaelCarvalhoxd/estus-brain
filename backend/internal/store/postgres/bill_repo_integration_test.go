package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/rafael/estus-vault/backend/internal/domain"
)

func TestBillRepo_Integration(t *testing.T) {
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

	repo := NewBillRepo(db)

	payable, err := repo.Create(ctx, domain.Bill{
		Description: "Aluguel",
		AmountCents: 150000,
		DueDate:     time.Date(2026, time.September, 5, 0, 0, 0, 0, time.UTC),
		Direction:   domain.BillPayable,
	})
	if err != nil {
		t.Fatalf("create payable: %v", err)
	}
	defer func() {
		db.Pool.Exec(ctx, `delete from bills where id = $1`, payable.ID)
	}()

	receivable, err := repo.Create(ctx, domain.Bill{
		Description: "Freela",
		AmountCents: 80000,
		DueDate:     time.Date(2026, time.September, 20, 0, 0, 0, 0, time.UTC),
		Direction:   domain.BillReceivable,
	})
	if err != nil {
		t.Fatalf("create receivable: %v", err)
	}
	defer func() {
		db.Pool.Exec(ctx, `delete from bills where id = $1`, receivable.ID)
	}()

	all, err := repo.List(ctx, nil, false)
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(all) < 2 {
		t.Fatalf("expected at least 2 bills, got %d", len(all))
	}

	payableOnly, err := repo.List(ctx, &[]domain.BillDirection{domain.BillPayable}[0], false)
	if err != nil {
		t.Fatalf("list payable: %v", err)
	}
	for _, b := range payableOnly {
		if b.Direction != domain.BillPayable {
			t.Errorf("list filtered by payable returned direction %q", b.Direction)
		}
	}

	paid, err := repo.MarkPaid(ctx, payable.ID, time.Now())
	if err != nil {
		t.Fatalf("mark paid: %v", err)
	}
	if paid.PaidAt == nil {
		t.Fatalf("expected PaidAt to be set")
	}

	openOnly, err := repo.List(ctx, nil, true)
	if err != nil {
		t.Fatalf("list open: %v", err)
	}
	for _, b := range openOnly {
		if b.ID == payable.ID {
			t.Errorf("marked-paid bill should not appear in onlyOpen list")
		}
	}

	_, receivableOpenCents, _, err := repo.OpenTotals(ctx)
	if err != nil {
		t.Fatalf("open totals: %v", err)
	}
	if receivableOpenCents < 80000 {
		t.Errorf("expected receivable open totals to include the 80000 test bill, got %d", receivableOpenCents)
	}

	_, err = repo.MarkPaid(ctx, "00000000-0000-0000-0000-000000000000", time.Now())
	if err != domain.ErrNotFound {
		t.Errorf("MarkPaid on unknown id = %v, want domain.ErrNotFound", err)
	}
}

func TestBillRepo_SeriesColumnsAndMonthQueries(t *testing.T) {
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
	if err := db.Migrate(ctx, "../../../migrations"); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	repo := NewBillRepo(db)
	categories := NewCategoryRepo(db)

	cat, err := categories.Create(ctx, domain.Category{Name: "Categoria de teste (contas)", Nature: "essencial", Color: "#123456"})
	if err != nil {
		t.Fatalf("create category: %v", err)
	}
	defer categories.Delete(ctx, cat.ID)

	series, method := "11111111-1111-1111-1111-111111111111", domain.PaymentPix
	created, err := repo.Create(ctx, domain.Bill{
		Description: "Aluguel de teste", AmountCents: 200000,
		DueDate:   time.Date(2026, time.September, 10, 0, 0, 0, 0, time.UTC),
		Direction: domain.BillPayable, CategoryID: &cat.ID,
		SeriesID: &series, AmountEstimated: true, PaymentMethod: &method,
	})
	if err != nil {
		t.Fatalf("create bill: %v", err)
	}
	defer repo.Delete(ctx, created.ID)

	if created.SeriesID == nil || *created.SeriesID != series {
		t.Errorf("series_id = %v, want %s", created.SeriesID, series)
	}
	if !created.AmountEstimated {
		t.Error("amount_estimated não voltou true")
	}
	if created.PaymentMethod == nil || *created.PaymentMethod != domain.PaymentPix {
		t.Errorf("payment_method = %v, want pix", created.PaymentMethod)
	}

	fetched, err := repo.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("get bill: %v", err)
	}
	if fetched.SeriesID == nil || *fetched.SeriesID != series {
		t.Errorf("Get series_id = %v, want %s", fetched.SeriesID, series)
	}

	if _, err := repo.Get(ctx, "00000000-0000-0000-0000-000000000000"); err != domain.ErrNotFound {
		t.Errorf("Get on unknown id = %v, want domain.ErrNotFound", err)
	}

	inMonth, err := repo.ListByMonth(ctx, domain.YearMonth{Year: 2026, Month: 9}, nil)
	if err != nil {
		t.Fatalf("list by month: %v", err)
	}
	if !containsBill(inMonth, created.ID) {
		t.Error("a conta não apareceu no mês do vencimento dela")
	}

	other, err := repo.ListByMonth(ctx, domain.YearMonth{Year: 2026, Month: 10}, nil)
	if err != nil {
		t.Fatalf("list by month: %v", err)
	}
	if containsBill(other, created.ID) {
		t.Error("a conta apareceu num mês que não é o do vencimento")
	}

	latest, err := repo.LatestPerSeries(ctx)
	if err != nil {
		t.Fatalf("latest per series: %v", err)
	}
	if !containsBill(latest, created.ID) {
		t.Error("a única ocorrência da série deveria ser a mais recente dela")
	}
}

func containsBill(bills []domain.Bill, id string) bool {
	for _, b := range bills {
		if b.ID == id {
			return true
		}
	}
	return false
}
