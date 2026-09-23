package domain

import "time"

// YearMonth identifies a calendar month without a day component — every
// aggregate in the dashboard (monthly total, category breakdown, comparison
// table) is keyed by this, never by a full date.
type YearMonth struct {
	Year  int
	Month int // 1-12
}

func YearMonthOf(t time.Time) YearMonth {
	return YearMonth{Year: t.Year(), Month: int(t.Month())}
}

// Add returns the year-month n months after m (n may be negative).
func (m YearMonth) Add(n int) YearMonth {
	t := time.Date(m.Year, time.Month(m.Month+n), 1, 0, 0, 0, 0, time.UTC)
	return YearMonthOf(t)
}

func (m YearMonth) Before(other YearMonth) bool {
	if m.Year != other.Year {
		return m.Year < other.Year
	}
	return m.Month < other.Month
}

// FirstDay returns the first calendar day of the month, used as the value
// stored in the transactions.competence_month column.
func (m YearMonth) FirstDay() time.Time {
	return time.Date(m.Year, time.Month(m.Month), 1, 0, 0, 0, 0, time.UTC)
}

// CompetenceMonth is the month an expense counts in: the month it was made,
// whatever the payment method. A card purchase's invoice month is separate
// (InvoiceMonth).
func CompetenceMonth(purchaseDate time.Time) YearMonth {
	return YearMonthOf(purchaseDate)
}

// InvoiceMonth is the month of the invoice a card purchase is billed on:
// the invoice that closes on or after the purchase, named by its due date.
func InvoiceMonth(purchaseDate time.Time, card CreditCard) YearMonth {
	return YearMonthOf(card.DueDateFor(card.ClosingDateFor(purchaseDate)))
}
