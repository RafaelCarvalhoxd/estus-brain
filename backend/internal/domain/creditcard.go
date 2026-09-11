package domain

import "time"

// CreditCard models enough of a real card's billing cycle to answer "when is
// this invoice due", without affecting which competence month a purchase
// counts against — see billing.go for why that rule is deliberately simpler.
type CreditCard struct {
	ID         string
	Name       string
	ClosingDay int // day of month the invoice closes, 1-28
	DueDay     int // day of month the invoice is due, 1-28
	CreatedAt  time.Time
}

// DueDateFor returns the date the invoice covering the given competence
// month is due.
func (c CreditCard) DueDateFor(competenceMonth YearMonth) time.Time {
	return time.Date(competenceMonth.Year, time.Month(competenceMonth.Month), c.DueDay, 0, 0, 0, 0, time.UTC)
}
