package assistant

import (
	"errors"
	"testing"

	"github.com/rafael/estus-vault/backend/internal/domain"
	"github.com/rafael/estus-vault/backend/internal/service"
	"github.com/rafael/estus-vault/backend/internal/store/postgres"
)

func TestPurchaseCents(t *testing.T) {
	cases := []struct {
		amount       float64
		per          bool
		installments int
		want         domain.Cents
	}{
		{480, false, 12, 48000},
		{40, true, 12, 48000},
		{40, true, 0, 4000},
		{19.9, false, 1, 1990},
	}
	for _, c := range cases {
		if got := purchaseCents(c.amount, c.per, c.installments); got != c.want {
			t.Errorf("purchaseCents(%v, %v, %d) = %d, want %d", c.amount, c.per, c.installments, got, c.want)
		}
	}
}

func TestCategoryInputApplyKeepsUnsentFields(t *testing.T) {
	c := domain.Category{Name: "Pets", Kind: domain.KindExpense, Nature: domain.NatureDiscretionary, Color: "#111111"}
	if err := (categoryInput{Kind: "Receita"}).apply(&c); err != nil {
		t.Fatal(err)
	}
	if c.Kind != domain.KindIncome || c.Nature != domain.NatureDiscretionary || c.Color != "#111111" {
		t.Fatalf("category = %+v", c)
	}
	if err := (categoryInput{Kind: "outro"}).apply(&c); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("bad kind err = %v", err)
	}
}

func TestFinanceAndBillToolsRegister(t *testing.T) {
	r := New(Deps{
		Categories: &postgres.CategoryRepo{}, Cards: &postgres.CreditCardRepo{},
		Transactions: &service.TransactionService{}, TransactionLog: &postgres.TransactionRepo{},
		Dashboard: &service.DashboardService{}, Bills: &service.BillService{},
		CardSpending: &service.CardSpendingService{},
	})
	destructive := map[string]bool{
		"finance_delete_category": true, "finance_delete_credit_card": true, "finance_delete_transaction": true, "bills_delete": true,
	}
	for _, name := range []string{
		"finance_create_transaction", "finance_update_transaction", "finance_delete_transaction", "finance_month_summary",
		"finance_create_category", "finance_update_category", "finance_delete_category",
		"finance_create_credit_card", "finance_update_credit_card", "finance_delete_credit_card", "finance_card_invoices",
		"bills_list", "bills_summary", "bills_create", "bills_update", "bills_pay", "bills_mark_received",
		"bills_unpay", "bills_end_series", "bills_resume_series", "bills_delete",
	} {
		tool, ok := r.byName[name]
		if !ok {
			t.Errorf("%s not registered", name)
			continue
		}
		if tool.Destructive != destructive[name] {
			t.Errorf("%s destructive = %v", name, tool.Destructive)
		}
	}
	if _, ok := r.byName["bills_mark_paid"]; ok {
		t.Error("bills_mark_paid should be gone: bills_pay and bills_mark_received replace it")
	}
}
