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
	// SeriesID links the occurrences of one monthly bill across months; nil
	// means a one-off. There is no template row: the mould for the next month
	// is simply the latest occurrence.
	SeriesID *string
	// SeriesEnded marks that the owner explicitly stopped this series from
	// growing new occurrences — set only by BillService.EndSeries /
	// ResumeSeries, never by a plain edit. Materialize skips a series whose
	// latest occurrence has this set. It carries no meaning about deleting a
	// row: deleting an occurrence is a plain delete, nothing more — a
	// dedicated flag is what replaced the old (unimplementable, once
	// Materialize runs on every month view) rule that "deleting the last
	// occurrence ends the series."
	SeriesEnded bool
	// AmountVaries says THIS SERIES' amount is expected to change every
	// month — a property of the series, not of one occurrence. It is copied
	// forward unchanged by NextOccurrence and seeds each new occurrence's
	// AmountEstimated, so a series whose amount fluctuates keeps starting
	// each new month as a guess even after an occurrence has been paid or
	// corrected (which only ever clears that occurrence's own
	// AmountEstimated, never AmountVaries).
	AmountVaries bool
	// AmountEstimated says THIS OCCURRENCE's amount was carried over and not
	// yet confirmed — "the power bill is usually R$180", not "the power bill
	// came to R$180". Paying or editing the amount clears it; it says
	// nothing about whether future occurrences will also be guesses — that
	// is what AmountVaries is for.
	AmountEstimated bool
	// PaymentMethod is the default used when settling this bill, so paying it
	// is one click instead of a form.
	PaymentMethod *PaymentMethod
	// CreditCardID is the card this bill settles on when PaymentMethod is
	// crédito — stored the same way, so paying doesn't ask again every month.
	CreditCardID *string
	// TransactionID is the expense this bill created when it was settled, so
	// undoing the payment removes exactly that one.
	TransactionID *string
	// InvoiceCardID marks the bill as that card's invoice for InvoiceMonth
	// (first day of the due month). Its amount follows the card's purchases,
	// and paying it records no expense.
	InvoiceCardID *string
	InvoiceMonth  *time.Time
	CreatedAt     time.Time
}

func (b Bill) IsInvoice() bool { return b.InvoiceCardID != nil }

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
	if b.Recurring() && b.Direction == BillPayable {
		if b.CategoryID == nil || *b.CategoryID == "" {
			return fmt.Errorf("%w: a recurring bill needs a category, because settling it records an expense", ErrValidation)
		}
		if b.PaymentMethod == nil || !b.PaymentMethod.Valid() {
			return fmt.Errorf("%w: a recurring bill needs a payment method, because settling it records an expense", ErrValidation)
		}
		if *b.PaymentMethod == PaymentCredit && (b.CreditCardID == nil || *b.CreditCardID == "") {
			return fmt.Errorf("%w: a recurring bill paid by credit needs a card", ErrValidation)
		}
	}
	return nil
}

// Recurring reports whether this bill is one occurrence of a monthly series.
func (b Bill) Recurring() bool { return b.SeriesID != nil }

// NextOccurrence is the bill that continues b's series in the month after
// b's own. It copies everything that describes the obligation — including
// AmountVaries, a property of the series carried forward via the plain
// struct copy below, not of one occurrence — and drops everything that
// describes this month's settlement, so the new occurrence starts pending.
// estimated marks the carried-over amount as a guess, which is what a series
// whose value changes every month needs; callers should pass the series' own
// AmountVaries here, not the outgoing occurrence's AmountEstimated (which
// paying or editing may have already cleared, independent of whether the
// series still varies).
func (b Bill) NextOccurrence(estimated bool) Bill {
	next := b
	next.ID = newID()
	next.PaidAt = nil
	next.TransactionID = nil
	next.CreatedAt = time.Time{}
	next.AmountEstimated = estimated
	next.DueDate = nextMonthSameDay(b.DueDate)
	return next
}

// nextMonthSameDay is the same day of the following month, clamped to that
// month's last day. A bill due on the 31st must land on the 30th of a
// 30-day month, never roll into the month after — that would silently move
// the expense into a different budget month.
func nextMonthSameDay(d time.Time) time.Time {
	year, month, day := d.Date()
	firstOfNext := time.Date(year, month+1, 1, 0, 0, 0, 0, d.Location())
	lastDay := firstOfNext.AddDate(0, 1, -1).Day()
	return time.Date(firstOfNext.Year(), firstOfNext.Month(), min(day, lastDay), 0, 0, 0, 0, d.Location())
}
