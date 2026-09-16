package domain

import (
	"errors"
	"testing"
	"time"
)

// onDay, not day: internal/domain's habit_test.go already defines a day()
// that parses a string, and two helpers with one name cannot share a package.
func onDay(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func TestClosingDateFor(t *testing.T) {
	card := CreditCard{ClosingDay: 10, DueDay: 20}
	cases := []struct {
		name     string
		purchase time.Time
		want     time.Time
	}{
		{"antes do fechamento", onDay(2026, time.September, 5), onDay(2026, time.September, 10)},
		{"no dia do fechamento fecha hoje", onDay(2026, time.September, 10), onDay(2026, time.September, 10)},
		{"depois do fechamento", onDay(2026, time.September, 15), onDay(2026, time.October, 10)},
		{"vira o ano", onDay(2026, time.December, 28), onDay(2027, time.January, 10)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := card.ClosingDateFor(tc.purchase); !got.Equal(tc.want) {
				t.Errorf("ClosingDateFor(%s) = %s, want %s",
					tc.purchase.Format("2006-01-02"), got.Format("2006-01-02"), tc.want.Format("2006-01-02"))
			}
		})
	}
}

func TestDueDateFor(t *testing.T) {
	cases := []struct {
		name    string
		card    CreditCard
		closing time.Time
		want    time.Time
	}{
		{
			"vencimento depois do fechamento vence no mesmo mês",
			CreditCard{ClosingDay: 10, DueDay: 20},
			onDay(2026, time.September, 10), onDay(2026, time.September, 20),
		},
		{
			"vencimento antes do fechamento vence no mês seguinte",
			CreditCard{ClosingDay: 25, DueDay: 15},
			onDay(2026, time.September, 25), onDay(2026, time.October, 15),
		},
		{
			"vencimento no mesmo dia do fechamento vence no mês seguinte",
			CreditCard{ClosingDay: 10, DueDay: 10},
			onDay(2026, time.September, 10), onDay(2026, time.October, 10),
		},
		{
			"vira o ano",
			CreditCard{ClosingDay: 25, DueDay: 15},
			onDay(2026, time.December, 25), onDay(2027, time.January, 15),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.card.DueDateFor(tc.closing); !got.Equal(tc.want) {
				t.Errorf("DueDateFor(%s) = %s, want %s",
					tc.closing.Format("2006-01-02"), got.Format("2006-01-02"), tc.want.Format("2006-01-02"))
			}
		})
	}
}

func TestCreditCardValidate(t *testing.T) {
	cases := []struct {
		name string
		card CreditCard
		ok   bool
	}{
		{"válido", CreditCard{Name: "Nubank", ClosingDay: 10, DueDay: 20}, true},
		{"nome vazio", CreditCard{Name: "  ", ClosingDay: 10, DueDay: 20}, false},
		{"fechamento 0", CreditCard{Name: "Nubank", ClosingDay: 0, DueDay: 20}, false},
		{"fechamento 29", CreditCard{Name: "Nubank", ClosingDay: 29, DueDay: 20}, false},
		{"vencimento 0", CreditCard{Name: "Nubank", ClosingDay: 10, DueDay: 0}, false},
		{"vencimento 29", CreditCard{Name: "Nubank", ClosingDay: 10, DueDay: 29}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.card.Validate()
			if tc.ok && err != nil {
				t.Fatalf("Validate() = %v, want nil", err)
			}
			if !tc.ok {
				if err == nil {
					t.Fatal("Validate() = nil, want an error")
				}
				if !errors.Is(err, ErrValidation) {
					t.Errorf("Validate() = %v, want it to wrap ErrValidation", err)
				}
			}
		})
	}
}
