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

// CompetenceMonth is the single rule the whole app is designed around: a
// purchase counts against the month the money actually leaves the account.
// Debit and Pix leave it the day they happen. A credit purchase leaves it
// when the invoice that charges it is due, which is what card is for: the
// purchase joins the invoice closing on or after it, and that invoice's due
// date names the month.
//
// This deliberately replaced an earlier rule that sent every credit purchase
// to the month after the purchase, ignoring the closing day. That rule was
// simpler but wrong: on a card closing on the 25th and due on the 15th, a
// purchase on the 28th really is charged almost two months later, and hiding
// that made the budget lie. Do not "simplify" it back.
//
// card is nil for debit and Pix. A credit purchase with a nil card is a
// programming error — the service refuses credit without a card long before
// this point — so it falls back to the purchase month rather than inventing
// a date.
func CompetenceMonth(purchaseDate time.Time, method PaymentMethod, card *CreditCard) YearMonth {
	if method != PaymentCredit || card == nil {
		return YearMonthOf(purchaseDate)
	}
	return YearMonthOf(card.DueDateFor(card.ClosingDateFor(purchaseDate)))
}
