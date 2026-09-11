package domain

import (
	"testing"
	"time"
)

func date(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func TestCompetenceMonth(t *testing.T) {
	cases := []struct {
		name   string
		date   time.Time
		method PaymentMethod
		want   YearMonth
	}{
		{"debit counts same month", date(2026, time.September, 5), PaymentDebit, YearMonth{2026, 9}},
		{"pix counts same month", date(2026, time.September, 1), PaymentPix, YearMonth{2026, 9}},
		{"credit rolls to next month", date(2026, time.August, 26), PaymentCredit, YearMonth{2026, 9}},
		{"credit on last day of month still rolls one month", date(2026, time.August, 31), PaymentCredit, YearMonth{2026, 9}},
		{"credit across year boundary", date(2026, time.December, 15), PaymentCredit, YearMonth{2027, 1}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := CompetenceMonth(c.date, c.method)
			if got != c.want {
				t.Errorf("CompetenceMonth(%v, %v) = %v, want %v", c.date, c.method, got, c.want)
			}
		})
	}
}

func TestNewInstallmentPurchase_SplitsRemainderIntoLastInstallment(t *testing.T) {
	txns := NewInstallmentPurchase("Notebook Dell", 302400, "cat-compras", "card-1", date(2026, time.May, 8), 12)

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
		if tx.CompetenceMonth != (YearMonth{2026, 6}).Add(i) {
			t.Errorf("installment %d competence = %v, want %v", i, tx.CompetenceMonth, (YearMonth{2026, 6}).Add(i))
		}
	}
	if sum != 302400 {
		t.Errorf("installments sum to %d cents, want 302400 (no cents lost or invented)", sum)
	}
	// 302400 / 12 divides evenly, but the invariant that matters is the sum —
	// verify a case that doesn't divide evenly too.
	txns2 := NewInstallmentPurchase("Presente", 1000, "cat", "card", date(2026, time.January, 1), 3)
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
	txns := NewInstallmentPurchase("Uber", 3890, "cat", "card", date(2026, time.August, 26), 1)
	if len(txns) != 1 {
		t.Fatalf("got %d transactions, want 1", len(txns))
	}
	if txns[0].InstallmentGroupID != nil {
		t.Errorf("a non-installment purchase should not have an InstallmentGroupID")
	}
}
