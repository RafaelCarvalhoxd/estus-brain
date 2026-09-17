package domain

import (
	"errors"
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

func billOn(y int, m time.Month, d int) Bill {
	series := "series-1"
	method := PaymentPix
	cat := "cat-1"
	return Bill{
		ID: "bill-1", Description: "Aluguel", AmountCents: 200000,
		DueDate:   time.Date(y, m, d, 0, 0, 0, 0, time.UTC),
		Direction: BillPayable, CategoryID: &cat,
		SeriesID: &series, PaymentMethod: &method,
	}
}

func TestNextOccurrenceKeepsTheDueDay(t *testing.T) {
	next := billOn(2026, time.September, 10).NextOccurrence(false)
	want := time.Date(2026, time.October, 10, 0, 0, 0, 0, time.UTC)
	if !next.DueDate.Equal(want) {
		t.Errorf("vencimento = %s, want %s", next.DueDate.Format("2006-01-02"), want.Format("2006-01-02"))
	}
}

// Dia 31 num mês de 30 cai no último dia — nunca escorrega para o mês
// seguinte, o que mudaria a competência do lançamento em silêncio.
func TestNextOccurrenceClampsToTheLastDayOfAShorterMonth(t *testing.T) {
	cases := []struct {
		name string
		from Bill
		want time.Time
	}{
		{"31 de outubro vira 30 de novembro", billOn(2026, time.October, 31), time.Date(2026, time.November, 30, 0, 0, 0, 0, time.UTC)},
		{"31 de janeiro vira 28 de fevereiro", billOn(2027, time.January, 31), time.Date(2027, time.February, 28, 0, 0, 0, 0, time.UTC)},
		{"31 de dezembro vira 31 de janeiro", billOn(2026, time.December, 31), time.Date(2027, time.January, 31, 0, 0, 0, 0, time.UTC)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.from.NextOccurrence(false).DueDate; !got.Equal(tc.want) {
				t.Errorf("vencimento = %s, want %s", got.Format("2006-01-02"), tc.want.Format("2006-01-02"))
			}
		})
	}
}

func TestNextOccurrenceCarriesTheSeriesAndClearsTheSettlement(t *testing.T) {
	paid := time.Date(2026, time.September, 9, 0, 0, 0, 0, time.UTC)
	from := billOn(2026, time.September, 10)
	from.PaidAt = &paid
	next := from.NextOccurrence(false)

	if next.SeriesID == nil || *next.SeriesID != *from.SeriesID {
		t.Errorf("série = %v, want %v", next.SeriesID, from.SeriesID)
	}
	if next.ID == from.ID {
		t.Error("a ocorrência nova não pode reusar o id da anterior")
	}
	if next.PaidAt != nil {
		t.Error("a ocorrência nova nasce pendente, não paga")
	}
	if next.AmountCents != from.AmountCents || next.Description != from.Description {
		t.Errorf("valor/descrição não vieram junto: %+v", next)
	}
	if next.CategoryID == nil || *next.CategoryID != *from.CategoryID {
		t.Error("categoria não veio junto")
	}
	if next.PaymentMethod == nil || *next.PaymentMethod != *from.PaymentMethod {
		t.Error("forma de pagamento não veio junto")
	}
}

func TestNextOccurrenceMarksTheAmountAsEstimatedWhenAsked(t *testing.T) {
	if got := billOn(2026, time.September, 10).NextOccurrence(true); !got.AmountEstimated {
		t.Error("a série de valor variável deve nascer com o valor estimado")
	}
	if got := billOn(2026, time.September, 10).NextOccurrence(false); got.AmountEstimated {
		t.Error("a série de valor fixo não deve nascer estimada")
	}
}

func TestRecurringIsHavingASeries(t *testing.T) {
	if !billOn(2026, time.September, 10).Recurring() {
		t.Error("conta com série deve ser recorrente")
	}
	one := billOn(2026, time.September, 10)
	one.SeriesID = nil
	if one.Recurring() {
		t.Error("conta sem série não é recorrente")
	}
}

// Só a conta a pagar recorrente vira lançamento, e lançamento exige os dois.
func TestValidateRequiresCategoryAndMethodOnAPayableSeries(t *testing.T) {
	noCategory := billOn(2026, time.September, 10)
	noCategory.CategoryID = nil
	if err := noCategory.Validate(); !errors.Is(err, ErrValidation) {
		t.Errorf("série a pagar sem categoria = %v, want ErrValidation", err)
	}

	noMethod := billOn(2026, time.September, 10)
	noMethod.PaymentMethod = nil
	if err := noMethod.Validate(); !errors.Is(err, ErrValidation) {
		t.Errorf("série a pagar sem forma de pagamento = %v, want ErrValidation", err)
	}

	receivable := billOn(2026, time.September, 10)
	receivable.Direction = BillReceivable
	receivable.CategoryID, receivable.PaymentMethod = nil, nil
	if err := receivable.Validate(); err != nil {
		t.Errorf("série a receber sem categoria/forma = %v, want nil", err)
	}

	oneOff := billOn(2026, time.September, 10)
	oneOff.SeriesID, oneOff.CategoryID, oneOff.PaymentMethod = nil, nil, nil
	if err := oneOff.Validate(); err != nil {
		t.Errorf("conta avulsa sem categoria/forma = %v, want nil", err)
	}
}
