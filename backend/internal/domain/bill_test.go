package domain

import (
	"testing"
	"time"
)

func TestBillStatus(t *testing.T) {
	now := date(2026, time.September, 11)
	paidAt := date(2026, time.September, 5)

	cases := []struct {
		name string
		bill Bill
		want string
	}{
		{
			name: "payable paid is pago",
			bill: Bill{Direction: BillPayable, DueDate: date(2026, time.September, 1), PaidAt: &paidAt},
			want: "pago",
		},
		{
			name: "receivable paid is recebido",
			bill: Bill{Direction: BillReceivable, DueDate: date(2026, time.September, 1), PaidAt: &paidAt},
			want: "recebido",
		},
		{
			name: "unpaid past due date is atrasado",
			bill: Bill{Direction: BillPayable, DueDate: date(2026, time.September, 1)},
			want: "atrasado",
		},
		{
			name: "unpaid due today is pendente",
			bill: Bill{Direction: BillPayable, DueDate: date(2026, time.September, 11)},
			want: "pendente",
		},
		{
			name: "unpaid due in the future is pendente",
			bill: Bill{Direction: BillReceivable, DueDate: date(2026, time.September, 30)},
			want: "pendente",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := c.bill.Status(now)
			if got != c.want {
				t.Errorf("Status() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestBillValidate(t *testing.T) {
	cases := []struct {
		name    string
		bill    Bill
		wantErr bool
	}{
		{"valid payable", Bill{Description: "Aluguel", AmountCents: 150000, Direction: BillPayable}, false},
		{"valid receivable", Bill{Description: "Freela", AmountCents: 50000, Direction: BillReceivable}, false},
		{"empty description", Bill{Description: "", AmountCents: 100, Direction: BillPayable}, true},
		{"zero amount", Bill{Description: "Aluguel", AmountCents: 0, Direction: BillPayable}, true},
		{"negative amount", Bill{Description: "Aluguel", AmountCents: -100, Direction: BillPayable}, true},
		{"invalid direction", Bill{Description: "Aluguel", AmountCents: 100, Direction: "outro"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.bill.Validate()
			if (err != nil) != c.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, c.wantErr)
			}
		})
	}
}
