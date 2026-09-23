package domain

import (
	"testing"
	"time"
)

func date(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// As duas tabelas de "Casos conferidos" da spec, literais.
func TestInvoiceMonthFollowsTheInvoiceDueDate(t *testing.T) {
	closes25due15 := CreditCard{ClosingDay: 25, DueDay: 15}
	closes10due20 := CreditCard{ClosingDay: 10, DueDay: 20}
	cases := []struct {
		name     string
		card     CreditCard
		purchase time.Time
		want     YearMonth
	}{
		{"fecha 25 vence 15: compra antes do fechamento", closes25due15,
			date(2026, time.September, 5), YearMonth{2026, 10}},
		{"fecha 25 vence 15: compra depois do fechamento", closes25due15,
			date(2026, time.September, 28), YearMonth{2026, 11}},
		{"fecha 10 vence 20: compra antes do fechamento", closes10due20,
			date(2026, time.September, 5), YearMonth{2026, 9}},
		{"fecha 10 vence 20: compra depois do fechamento", closes10due20,
			date(2026, time.September, 15), YearMonth{2026, 10}},
		{"vira o ano", closes10due20,
			date(2026, time.December, 15), YearMonth{2027, 1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := InvoiceMonth(tc.purchase, tc.card); got != tc.want {
				t.Errorf("InvoiceMonth = %v, want %v", got, tc.want)
			}
		})
	}
}

// A compra no cartão aparece no mês em que foi feita; só a fatura vai
// para o mês do vencimento.
func TestCompetenceMonthIsThePurchaseMonth(t *testing.T) {
	if got := CompetenceMonth(date(2026, time.September, 28)); got != (YearMonth{2026, 9}) {
		t.Errorf("CompetenceMonth = %v, want setembro", got)
	}
}

func TestNewInstallmentPurchase_SplitsRemainderIntoLastInstallment(t *testing.T) {
	card := CreditCard{ID: "card-1", ClosingDay: 10, DueDay: 20}
	// 8 de maio é antes do fechamento: a fatura fecha 10/05 e vence 20/05,
	// então a primeira parcela cai em maio e as seguintes andam um mês.
	txns := NewInstallmentPurchase("Notebook Dell", 302400, "cat-compras", card, date(2026, time.May, 8), 12)

	if len(txns) != 12 {
		t.Fatalf("got %d installments, want 12", len(txns))
	}

	var sum Cents
	for i, tx := range txns {
		sum += tx.AmountCents
		if tx.InstallmentNumber != i+1 {
			t.Errorf("installment %d has InstallmentNumber %d", i, tx.InstallmentNumber)
		}
		if tx.InstallmentTotal != 12 {
			t.Errorf("installment %d has InstallmentTotal %d, want 12", i, tx.InstallmentTotal)
		}
		if tx.CompetenceMonth != (YearMonth{2026, 5}).Add(i) {
			t.Errorf("installment %d competence = %v, want %v", i, tx.CompetenceMonth, (YearMonth{2026, 5}).Add(i))
		}
		if tx.CreditCardID == nil || *tx.CreditCardID != "card-1" {
			t.Errorf("installment %d card = %v", i, tx.CreditCardID)
		}
	}
	if sum != 302400 {
		t.Errorf("installments sum to %d cents, want 302400 (no cents lost or invented)", sum)
	}
	// 302400 / 12 divides evenly, but the invariant that matters is the sum —
	// verify a case that doesn't divide evenly too.
	txns2 := NewInstallmentPurchase("Presente", 1000, "cat", card, date(2026, time.January, 1), 3)
	sum = 0
	for _, tx := range txns2 {
		sum += tx.AmountCents
	}
	if sum != 1000 {
		t.Errorf("uneven split sum = %d, want 1000", sum)
	}
	if txns2[2].AmountCents != 334 { // 333, 333, 334
		t.Errorf("last installment = %d, want remainder folded in (334)", txns2[2].AmountCents)
	}
}

func TestSingleInstallmentHasNoGroupID(t *testing.T) {
	card := CreditCard{ID: "card-1", ClosingDay: 10, DueDay: 20}
	txns := NewInstallmentPurchase("Uber", 3890, "cat", card, date(2026, time.August, 26), 1)
	if len(txns) != 1 {
		t.Fatalf("got %d transactions, want 1", len(txns))
	}
	if txns[0].InstallmentGroupID != nil {
		t.Errorf("a non-installment purchase should not have an InstallmentGroupID")
	}
	// A compra aparece em agosto, quando foi feita. 26/08 é depois do
	// fechamento: fecha 10/09, vence 20/09, então a fatura é de setembro.
	if txns[0].CompetenceMonth != (YearMonth{2026, 8}) {
		t.Errorf("competence = %v, want agosto", txns[0].CompetenceMonth)
	}
	if txns[0].InvoiceMonth == nil || *txns[0].InvoiceMonth != (YearMonth{2026, 9}) {
		t.Errorf("invoice = %v, want setembro", txns[0].InvoiceMonth)
	}
}
