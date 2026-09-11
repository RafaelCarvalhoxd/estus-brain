package domain

import (
	"fmt"
	"time"
)

// BillDirection tells which way the money moves for a scheduled obligation:
// a bill to pay, or money someone else owes you.
type BillDirection string

const (
	BillPayable    BillDirection = "pagar"
	BillReceivable BillDirection = "receber"
)

func (d BillDirection) Valid() bool {
	switch d {
	case BillPayable, BillReceivable:
		return true
	}
	return false
}

// Bill is a scheduled obligation — a bill to pay or money to receive — kept
// separate from the transactions ledger: it tracks whether something has
// been settled, not what it cost against a monthly budget.
type Bill struct {
	ID          string
	Description string
	AmountCents Cents
	DueDate     time.Time
	Direction   BillDirection
	CategoryID  *string
	PaidAt      *time.Time
	Recurring   bool
	CreatedAt   time.Time
}

// Status is derived from PaidAt and DueDate rather than stored, so a bill
// that ages past its due date without being touched becomes "atrasado"
// automatically instead of requiring a background job to flip a column.
func (b Bill) Status(now time.Time) string {
	if b.PaidAt != nil {
		if b.Direction == BillReceivable {
			return "recebido"
		}
		return "pago"
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	dueDate := time.Date(b.DueDate.Year(), b.DueDate.Month(), b.DueDate.Day(), 0, 0, 0, 0, now.Location())
	if dueDate.Before(today) {
		return "atrasado"
	}
	return "pendente"
}

func (b Bill) Validate() error {
	if b.Description == "" {
		return fmt.Errorf("%w: description is required", ErrValidation)
	}
	if b.AmountCents <= 0 {
		return fmt.Errorf("%w: amount must be positive", ErrValidation)
	}
	if !b.Direction.Valid() {
		return fmt.Errorf("%w: invalid direction %q", ErrValidation, b.Direction)
	}
	return nil
}
