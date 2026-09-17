package postgres

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/rafael/estus-vault/backend/internal/domain"
)

// TestTransactionRepoDeleteRefusesATransactionABillPointsAt guards Finding 4
// of the whole-branch review: bills.transaction_id references
// transactions(id) with no `on delete` clause (NO ACTION), so deleting the
// expense a settled bill points at raises Postgres 23503. Before this fix
// that surfaced as a raw, untranslated 500; the owner clicking delete on
// what looked like a duplicate in Lançamentos saw the spinner stop and the
// row silently stay, forever, with no explanation. Delete must translate the
// violation into domain.ErrConflict so the HTTP and frontend layers can show
// a sentence that points at the actual fix (undo the payment in Contas).
func TestTransactionRepoDeleteRefusesATransactionABillPointsAt(t *testing.T) {
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

	bills := NewBillRepo(db)
	categories := NewCategoryRepo(db)
	transactions := NewTransactionRepo(db)

	cat, err := categories.Create(ctx, domain.Category{
		Name: "Categoria de teste (excluir lançamento de conta)", Nature: "essencial", Color: "#ff00aa",
	})
	if err != nil {
		t.Fatalf("create category: %v", err)
	}
	defer func() {
		if err := categories.Delete(ctx, cat.ID); err != nil {
			t.Errorf("cleanup: delete category %s: %v", cat.ID, err)
		}
	}()

	method := domain.PaymentPix
	bill, err := bills.Create(ctx, domain.Bill{
		Description: "Conta de teste (excluir lançamento)", AmountCents: 3000,
		DueDate:   time.Date(2026, time.September, 10, 0, 0, 0, 0, time.UTC),
		Direction: domain.BillPayable, CategoryID: &cat.ID, PaymentMethod: &method,
	})
	if err != nil {
		t.Fatalf("create bill: %v", err)
	}

	expense := domain.Transaction{
		ID: domain.NewID(), Description: "Conta de teste (excluir lançamento)", AmountCents: 3000,
		CategoryID: cat.ID, PaymentMethod: domain.PaymentPix,
		PurchaseDate:     time.Date(2026, time.September, 10, 0, 0, 0, 0, time.UTC),
		CompetenceMonth:  domain.YearMonth{Year: 2026, Month: 9},
		InstallmentTotal: 1, IsRecurring: false,
	}
	// Cleanup order: the bill (which points at the transaction) must go
	// before the transaction, and the transaction before the category.
	defer func() {
		if err := transactions.Delete(ctx, expense.ID); err != nil {
			t.Errorf("cleanup: delete transaction %s: %v", expense.ID, err)
		}
	}()
	defer func() {
		if err := bills.Delete(ctx, bill.ID); err != nil {
			t.Errorf("cleanup: delete bill %s: %v", bill.ID, err)
		}
	}()

	if _, err := bills.Pay(ctx, bill.ID, time.Now(), expense); err != nil {
		t.Fatalf("pay: %v", err)
	}

	err = transactions.Delete(ctx, expense.ID)
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("delete transaction pointed at by a bill = %v, want domain.ErrConflict", err)
	}
}
