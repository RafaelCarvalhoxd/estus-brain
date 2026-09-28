package domain

import (
	"errors"
	"testing"
	"time"
)

func testDay(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

func TestInvestmentValidate(t *testing.T) {
	ok := Investment{Name: "CDB Inter", Kind: InvestmentCDB, RateBP: 100, RatePeriod: RatePerMonth}
	if err := ok.Validate(); err != nil {
		t.Fatalf("valid investment refused: %v", err)
	}
	bad := []Investment{
		{Name: " ", Kind: InvestmentCDB, RatePeriod: RatePerMonth},
		{Name: "x", Kind: "imovel", RatePeriod: RatePerMonth},
		{Name: "x", Kind: InvestmentCDB, RatePeriod: "semana"},
		{Name: "x", Kind: InvestmentCDB, RatePeriod: RatePerYear, RateBP: -1},
		{Name: "x", Kind: InvestmentCDB, RatePeriod: RatePerYear, RateBP: maxRateBP + 1},
	}
	for _, inv := range bad {
		if err := inv.Validate(); !errors.Is(err, ErrValidation) {
			t.Errorf("%+v: got %v, want ErrValidation", inv, err)
		}
	}
}

func TestInvestmentEstimatedCents(t *testing.T) {
	inv := Investment{
		RateBP: 100, RatePeriod: RatePerMonth,
		Contributions: []InvestmentContribution{
			{AmountCents: 100_000, Date: testDay(2025, 9, 28)},
			{AmountCents: 50_000, Date: testDay(2026, 9, 28)},
		},
	}
	if got := inv.InvestedCents(); got != 150_000 {
		t.Fatalf("invested = %d, want 150000", got)
	}
	// A year at 1% a month is 1.01^12 ≈ 1.126825; a contribution made today
	// has not yielded yet.
	got := inv.EstimatedCents(testDay(2026, 9, 28))
	if want := Cents(112_683 + 50_000); got != want {
		t.Errorf("estimated = %d, want %d", got, want)
	}

	yearly := Investment{RateBP: 1200, RatePeriod: RatePerYear,
		Contributions: []InvestmentContribution{{AmountCents: 100_000, Date: testDay(2025, 9, 28)}}}
	if got := yearly.EstimatedCents(testDay(2026, 9, 28)); got != 112_000 {
		t.Errorf("12%% a year for a year = %d, want 112000", got)
	}

	flat := Investment{RatePeriod: RatePerYear,
		Contributions: []InvestmentContribution{{AmountCents: 100_000, Date: testDay(2025, 1, 1)}}}
	if got := flat.EstimatedCents(testDay(2026, 9, 28)); got != 100_000 {
		t.Errorf("0%% rate = %d, want 100000", got)
	}
}

func TestContributionValidate(t *testing.T) {
	if err := (InvestmentContribution{AmountCents: 1, Date: testDay(2026, 1, 1)}).Validate(); err != nil {
		t.Fatalf("valid contribution refused: %v", err)
	}
	if err := (InvestmentContribution{AmountCents: 0, Date: testDay(2026, 1, 1)}).Validate(); !errors.Is(err, ErrValidation) {
		t.Errorf("zero amount accepted")
	}
	if err := (InvestmentContribution{AmountCents: 1}).Validate(); !errors.Is(err, ErrValidation) {
		t.Errorf("missing date accepted")
	}
}
