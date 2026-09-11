// Package domain holds the core business types and rules of Estus Vault,
// independent of HTTP or storage concerns.
package domain

import "fmt"

// Cents represents an amount of Brazilian reais as an integer number of
// cents. Money is never represented as a float anywhere in the system —
// floating point arithmetic silently loses cents on sums across hundreds of
// transactions, which is unacceptable for a ledger.
type Cents int64

// Split divides the amount into n parts as evenly as possible. Because cents
// don't always divide evenly, any remainder (at most n-1 cents) is added to
// the last installment, matching how card issuers round.
func (c Cents) Split(n int) []Cents {
	if n <= 1 {
		return []Cents{c}
	}
	base := int64(c) / int64(n)
	remainder := int64(c) % int64(n)
	parts := make([]Cents, n)
	for i := 0; i < n; i++ {
		parts[i] = Cents(base)
	}
	parts[n-1] += Cents(remainder)
	return parts
}

// String renders the amount as "R$ 1.234,56".
func (c Cents) String() string {
	sign := ""
	v := int64(c)
	if v < 0 {
		sign = "-"
		v = -v
	}
	reais := v / 100
	cents := v % 100

	// group thousands with '.'
	digits := fmt.Sprintf("%d", reais)
	var grouped []byte
	for i, d := range []byte(digits) {
		if i > 0 && (len(digits)-i)%3 == 0 {
			grouped = append(grouped, '.')
		}
		grouped = append(grouped, d)
	}
	return fmt.Sprintf("%sR$ %s,%02d", sign, grouped, cents)
}
