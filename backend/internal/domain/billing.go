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

// CompetenceMonth is the single rule the whole app was designed around: a
// purchase made on a debit card or via Pix counts against the budget the
// month it happened. A purchase made on a credit card counts against the
// budget the FOLLOWING month, because that's the month the invoice — and the
// real cash outflow — lands in. This is intentionally independent of the
// card's closing day: the closing day only decides which due date a
// transaction's invoice carries (see CreditCard.DueDateFor), never which
// month it's budgeted against. Mixing the two would make a purchase on the
// 28th silently jump two months ahead, which is not the mental model anyone
// budgeting week to week actually uses.
func CompetenceMonth(purchaseDate time.Time, method PaymentMethod) YearMonth {
	ym := YearMonthOf(purchaseDate)
	if method == PaymentCredit {
		return ym.Add(1)
	}
	return ym
}
