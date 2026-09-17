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

// Paying a bill has to record the expense in the same database transaction:
// a bill marked paid with no matching transaction is exactly the balance bug
// this phase exists to fix.
func TestBillRepoPayWritesTheBillAndTheExpenseTogether(t *testing.T) {
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
	transactions := NewTransactionRepo(db)

	cat, err := categories.Create(ctx, domain.Category{Name: "Categoria de teste (pagamento)", Nature: "essencial", Color: "#654321"})
	if err != nil {
		t.Fatalf("create category: %v", err)
	}
	// Cleanup order matters: bills.transaction_id references transactions(id)
	// with no cascade, so the bill row must go before the transaction row, and
	// both must go before the category they reference. Deferred statements run
	// LIFO, so declaring the transaction cleanup before the bill cleanup makes
	// the bill delete run first. Every cleanup error is asserted on, not
	// dropped — a silently failed delete here is exactly the kind of leak
	// that already put orphan rows in the owner's real database this session.
	defer func() {
		if err := categories.Delete(ctx, cat.ID); err != nil {
			t.Errorf("cleanup: delete category %s: %v", cat.ID, err)
		}
	}()

	method := domain.PaymentPix
	bill, err := repo.Create(ctx, domain.Bill{
		Description: "Conta de teste (pagamento)", AmountCents: 5000,
		DueDate:   time.Date(2026, time.September, 10, 0, 0, 0, 0, time.UTC),
		Direction: domain.BillPayable, CategoryID: &cat.ID, PaymentMethod: &method,
	})
	if err != nil {
		t.Fatalf("create bill: %v", err)
	}

	expense := domain.Transaction{
		ID: domain.NewID(), Description: "Conta de teste (pagamento)", AmountCents: 5000,
		CategoryID: cat.ID, PaymentMethod: domain.PaymentPix,
		PurchaseDate:     time.Date(2026, time.September, 10, 0, 0, 0, 0, time.UTC),
		CompetenceMonth:  domain.YearMonth{Year: 2026, Month: 9},
		InstallmentTotal: 1, IsRecurring: false,
	}
	defer func() {
		if err := transactions.Delete(ctx, expense.ID); err != nil {
			t.Errorf("cleanup: delete transaction %s: %v", expense.ID, err)
		}
	}()
	defer func() {
		if err := repo.Delete(ctx, bill.ID); err != nil {
			t.Errorf("cleanup: delete bill %s: %v", bill.ID, err)
		}
	}()

	paid, err := repo.Pay(ctx, bill.ID, time.Now(), expense)
	if err != nil {
		t.Fatalf("pay: %v", err)
	}
	if paid.PaidAt == nil {
		t.Error("a conta não ficou paga")
	}
	if paid.TransactionID == nil || *paid.TransactionID != expense.ID {
		t.Errorf("bill.TransactionID = %v, want %s", paid.TransactionID, expense.ID)
	}

	txns, err := transactions.ListByCompetenceMonth(ctx, domain.YearMonth{Year: 2026, Month: 9})
	if err != nil {
		t.Fatalf("list transactions: %v", err)
	}
	found := false
	for _, row := range txns {
		if row.ID == expense.ID {
			found = true
		}
	}
	if !found {
		t.Error("o lançamento não foi criado junto com o pagamento")
	}

	// Paying an already-paid bill must not create a second expense.
	if _, err := repo.Pay(ctx, bill.ID, time.Now(), expense); err != domain.ErrNotFound {
		t.Errorf("Pay on an already-paid bill = %v, want domain.ErrNotFound", err)
	}
}

// Undoing a payment must delete the exact expense Pay created, not merely
// wipe the only column that names it. A naive
// "update bills set transaction_id = null ... returning transaction_id"
// always returns null — RETURNING reflects the row AFTER the update — so a
// bug there leaves the expense orphaned in the ledger forever (the foreign
// key is NO ACTION: nothing else in the schema can clean it up), still
// counted in "Saídas" for a payment the owner just undid.
func TestBillRepoUnpayRemovesTheBillAndTheExpenseTogether(t *testing.T) {
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
	transactions := NewTransactionRepo(db)

	cat, err := categories.Create(ctx, domain.Category{Name: "Categoria de teste (desfazer pagamento)", Nature: "essencial", Color: "#123abc"})
	if err != nil {
		t.Fatalf("create category: %v", err)
	}
	defer func() {
		if err := categories.Delete(ctx, cat.ID); err != nil {
			t.Errorf("cleanup: delete category %s: %v", cat.ID, err)
		}
	}()

	method := domain.PaymentPix
	bill, err := repo.Create(ctx, domain.Bill{
		Description: "Conta de teste (desfazer pagamento)", AmountCents: 7000,
		DueDate:   time.Date(2026, time.September, 12, 0, 0, 0, 0, time.UTC),
		Direction: domain.BillPayable, CategoryID: &cat.ID, PaymentMethod: &method,
	})
	if err != nil {
		t.Fatalf("create bill: %v", err)
	}

	expense := domain.Transaction{
		ID: domain.NewID(), Description: "Conta de teste (desfazer pagamento)", AmountCents: 7000,
		CategoryID: cat.ID, PaymentMethod: domain.PaymentPix,
		PurchaseDate:     time.Date(2026, time.September, 12, 0, 0, 0, 0, time.UTC),
		CompetenceMonth:  domain.YearMonth{Year: 2026, Month: 9},
		InstallmentTotal: 1, IsRecurring: false,
	}
	// Safety-net cleanup for if the test fails before reaching (or because
	// of) Unpay itself; a successful Unpay already removes the transaction
	// and repo.Delete below removes the bill, so both calls are expected to
	// return domain.ErrNotFound on the happy path — anything else is a real
	// leak and must be reported, not swallowed.
	defer func() {
		if err := transactions.Delete(ctx, expense.ID); err != nil && err != domain.ErrNotFound {
			t.Errorf("cleanup: delete transaction %s: %v", expense.ID, err)
		}
	}()
	defer func() {
		if err := repo.Delete(ctx, bill.ID); err != nil && err != domain.ErrNotFound {
			t.Errorf("cleanup: delete bill %s: %v", bill.ID, err)
		}
	}()

	if _, err := repo.Pay(ctx, bill.ID, time.Now(), expense); err != nil {
		t.Fatalf("pay: %v", err)
	}

	unpaid, err := repo.Unpay(ctx, bill.ID)
	if err != nil {
		t.Fatalf("unpay: %v", err)
	}
	if unpaid.PaidAt != nil {
		t.Error("a conta deveria voltar a ficar pendente depois do unpay")
	}
	if unpaid.TransactionID != nil {
		t.Errorf("bill.TransactionID = %v, want nil depois do unpay", unpaid.TransactionID)
	}

	txns, err := transactions.ListByCompetenceMonth(ctx, domain.YearMonth{Year: 2026, Month: 9})
	if err != nil {
		t.Fatalf("list transactions: %v", err)
	}
	for _, row := range txns {
		if row.ID == expense.ID {
			t.Error("o lançamento deveria ter sido apagado pelo unpay, mas ainda existe")
		}
	}

	// The bill itself must still exist (unpaid, not deleted) so the deferred
	// repo.Delete above is expected to succeed, not hit ErrNotFound.
	stillThere, err := repo.Get(ctx, bill.ID)
	if err != nil {
		t.Fatalf("get bill after unpay: %v", err)
	}
	if stillThere.PaidAt != nil {
		t.Error("Get depois do unpay ainda mostra a conta como paga")
	}
}

// Pay must be one commit: if the expense insert fails for any reason — here,
// a category that doesn't exist, which the foreign key on
// transactions.category_id rejects — the bill must stay unpaid. An
// implementation that runs the bill update and the expense insert as two
// separate commits instead of one shared transaction would let the bill
// update stick regardless of what happens to the second one; this is the
// case the atomicity guarantee this task exists to add is actually for.
func TestBillRepoPayRollsBillBackWhenTheExpenseFailsToInsert(t *testing.T) {
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
	transactions := NewTransactionRepo(db)

	cat, err := categories.Create(ctx, domain.Category{Name: "Categoria de teste (rollback pagamento)", Nature: "essencial", Color: "#abcdef"})
	if err != nil {
		t.Fatalf("create category: %v", err)
	}
	defer func() {
		if err := categories.Delete(ctx, cat.ID); err != nil {
			t.Errorf("cleanup: delete category %s: %v", cat.ID, err)
		}
	}()

	method := domain.PaymentPix
	bill, err := repo.Create(ctx, domain.Bill{
		Description: "Conta de teste (rollback pagamento)", AmountCents: 4000,
		DueDate:   time.Date(2026, time.September, 14, 0, 0, 0, 0, time.UTC),
		Direction: domain.BillPayable, CategoryID: &cat.ID, PaymentMethod: &method,
	})
	if err != nil {
		t.Fatalf("create bill: %v", err)
	}
	defer func() {
		if err := repo.Delete(ctx, bill.ID); err != nil {
			t.Errorf("cleanup: delete bill %s: %v", bill.ID, err)
		}
	}()

	expense := domain.Transaction{
		ID: domain.NewID(), Description: "Conta de teste (rollback pagamento)", AmountCents: 4000,
		// A category id that doesn't exist: the insert must fail on the
		// transactions.category_id foreign key.
		CategoryID: "00000000-0000-0000-0000-000000000000", PaymentMethod: domain.PaymentPix,
		PurchaseDate:     time.Date(2026, time.September, 14, 0, 0, 0, 0, time.UTC),
		CompetenceMonth:  domain.YearMonth{Year: 2026, Month: 9},
		InstallmentTotal: 1, IsRecurring: false,
	}
	// Safety net only: the insert is expected to fail, so this row should
	// never exist. If it does, that itself is a defect worth failing loudly
	// on rather than silently cleaning up.
	defer func() {
		if err := transactions.Delete(ctx, expense.ID); err != nil && err != domain.ErrNotFound {
			t.Errorf("cleanup: delete transaction %s: %v", expense.ID, err)
		} else if err == nil {
			t.Error("o lançamento com categoria inexistente foi inserido mesmo assim — Pay não é atômico")
		}
	}()

	if _, err := repo.Pay(ctx, bill.ID, time.Now(), expense); err == nil {
		t.Fatal("pay com categoria inexistente deveria falhar")
	}

	got, err := repo.Get(ctx, bill.ID)
	if err != nil {
		t.Fatalf("get bill: %v", err)
	}
	if got.PaidAt != nil {
		t.Error("a conta ficou marcada como paga mesmo com o lançamento falhando ao ser inserido — Pay não é atômico")
	}
	if got.TransactionID != nil {
		t.Errorf("bill.TransactionID = %v, want nil quando o pagamento falha", got.TransactionID)
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
