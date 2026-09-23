package domain

import (
	"fmt"
	"strings"
	"time"
)

// CreditCard models a real card's billing cycle: when its invoice closes and
// when that invoice has to be paid. InvoiceMonth uses both to pick the
// invoice a purchase is billed on.
type CreditCard struct {
	ID         string
	Name       string
	ClosingDay int // day of month the invoice closes, 1-28
	DueDay     int // day of month the invoice is due, 1-28
	CreatedAt  time.Time
}

// ClosingDateFor is the date the invoice that charges a purchase made on
// purchase closes. Buying ON the closing day still makes that day's invoice —
// one more day of grace, which is how the owner reads it.
func (c CreditCard) ClosingDateFor(purchase time.Time) time.Time {
	month := purchase.Month()
	if purchase.Day() > c.ClosingDay {
		month++
	}
	// time.Date normalizes month 13 into January of the next year.
	return time.Date(purchase.Year(), month, c.ClosingDay, 0, 0, 0, 0, purchase.Location())
}

// DueDateFor is the date the invoice that closed on closing has to be paid.
// A due day at or before the closing day belongs to the next month: a card
// that closes on the 25th and is due on the 15th is paid the month after it
// closes.
func (c CreditCard) DueDateFor(closing time.Time) time.Time {
	month := closing.Month()
	if c.DueDay <= c.ClosingDay {
		month++
	}
	return time.Date(closing.Year(), month, c.DueDay, 0, 0, 0, 0, closing.Location())
}

// Validate keeps the day range in step with the check constraint the
// credit_cards table has carried since 0001_init, so a bad day is refused
// with a readable message instead of a Postgres error.
func (c CreditCard) Validate() error {
	if strings.TrimSpace(c.Name) == "" {
		return fmt.Errorf("%w: name is required", ErrValidation)
	}
	if c.ClosingDay < 1 || c.ClosingDay > 28 {
		return fmt.Errorf("%w: closing day must be between 1 and 28", ErrValidation)
	}
	if c.DueDay < 1 || c.DueDay > 28 {
		return fmt.Errorf("%w: due day must be between 1 and 28", ErrValidation)
	}
	return nil
}
