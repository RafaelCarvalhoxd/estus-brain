package domain

import (
	"fmt"
	"math"
	"strings"
	"time"
)

type InvestmentKind string

const (
	InvestmentCDB      InvestmentKind = "cdb"
	InvestmentTesouro  InvestmentKind = "tesouro"
	InvestmentLCI      InvestmentKind = "lci_lca"
	InvestmentPoupanca InvestmentKind = "poupanca"
	InvestmentFundo    InvestmentKind = "fundo"
	InvestmentAcoes    InvestmentKind = "acoes"
	InvestmentFII      InvestmentKind = "fii"
	InvestmentCripto   InvestmentKind = "cripto"
	InvestmentOutro    InvestmentKind = "outro"
)

func (k InvestmentKind) Valid() bool {
	switch k {
	case InvestmentCDB, InvestmentTesouro, InvestmentLCI, InvestmentPoupanca, InvestmentFundo,
		InvestmentAcoes, InvestmentFII, InvestmentCripto, InvestmentOutro:
		return true
	}
	return false
}

// RatePeriod is what the rate is per: a month or a year.
type RatePeriod string

const (
	RatePerMonth RatePeriod = "mes"
	RatePerYear  RatePeriod = "ano"
)

// maxRateBP caps the rate at 1000%, far above anything real, so a typo
// like 100000 instead of 1,00 is refused.
const maxRateBP = 100_000

type Investment struct {
	ID   string
	Name string
	Kind InvestmentKind
	// RateBP is the rate in hundredths of a percentage point: 1% = 100.
	RateBP        int64
	RatePeriod    RatePeriod
	Contributions []InvestmentContribution
	CreatedAt     time.Time
}

type InvestmentContribution struct {
	ID           string
	InvestmentID string
	AmountCents  Cents
	// Date is a calendar day (midnight UTC); its month is the month the
	// money left the account.
	Date time.Time
	// Initial is the amount the investment was opened with: money already
	// invested before, so it never leaves the month's balance.
	Initial   bool
	CreatedAt time.Time
}

func (i Investment) Validate() error {
	if strings.TrimSpace(i.Name) == "" {
		return fmt.Errorf("%w: name is required", ErrValidation)
	}
	if !i.Kind.Valid() {
		return fmt.Errorf("%w: unknown investment kind %q", ErrValidation, i.Kind)
	}
	if i.RatePeriod != RatePerMonth && i.RatePeriod != RatePerYear {
		return fmt.Errorf("%w: rate_period must be mes or ano", ErrValidation)
	}
	if i.RateBP < 0 || i.RateBP > maxRateBP {
		return fmt.Errorf("%w: rate must be between 0%% and 1000%%", ErrValidation)
	}
	return nil
}

func (c InvestmentContribution) Validate() error {
	if c.AmountCents <= 0 {
		return fmt.Errorf("%w: amount must be positive", ErrValidation)
	}
	if c.Date.IsZero() {
		return fmt.Errorf("%w: date is required", ErrValidation)
	}
	return nil
}

// InvestedCents is everything put in, without yield.
func (i Investment) InvestedCents() Cents {
	var total Cents
	for _, c := range i.Contributions {
		total += c.AmountCents
	}
	return total
}

// EstimatedCents compounds each contribution at the rate from its day to
// asOf. It is an estimate, not a ledger figure, so the float math here is
// rounded to cents once per contribution and never feeds a balance.
func (i Investment) EstimatedCents(asOf time.Time) Cents {
	rate := float64(i.RateBP) / 10_000
	periodDays := 365.0
	if i.RatePeriod == RatePerMonth {
		periodDays = 365.0 / 12
	}
	day := time.Date(asOf.Year(), asOf.Month(), asOf.Day(), 0, 0, 0, 0, time.UTC)
	var total Cents
	for _, c := range i.Contributions {
		days := day.Sub(c.Date).Hours() / 24
		if days <= 0 || rate == 0 {
			total += c.AmountCents
			continue
		}
		total += Cents(math.Round(float64(c.AmountCents) * math.Pow(1+rate, days/periodDays)))
	}
	return total
}
