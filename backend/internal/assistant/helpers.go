package assistant

import (
	"math"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"

	"github.com/rafael/estus-vault/backend/internal/domain"
)

const (
	dayLayout   = "2006-01-02"
	monthLayout = "2006-01"
)

func (r *Registry) now() time.Time { return time.Now().In(r.deps.Location) }

// today is the owner's calendar day, as a UTC midnight date (how the ledger
// stores bare dates).
func (r *Registry) today() time.Time {
	n := r.now()
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
}

// parseDay reads YYYY-MM-DD, or the words hoje/ontem/amanhã; empty means today.
func (r *Registry) parseDay(s string) (time.Time, error) {
	switch normalize(s) {
	case "", "hoje":
		return r.today(), nil
	case "ontem":
		return r.today().AddDate(0, 0, -1), nil
	case "amanha":
		return r.today().AddDate(0, 0, 1), nil
	}
	d, err := time.Parse(dayLayout, strings.TrimSpace(s))
	if err != nil {
		return time.Time{}, invalid("data %q inválida, use AAAA-MM-DD", s)
	}
	return d, nil
}

// parseMonth reads YYYY-MM, or atual/passado/proximo; empty means this month.
func (r *Registry) parseMonth(s string) (domain.YearMonth, error) {
	current := domain.YearMonthOf(r.now())
	switch normalize(s) {
	case "", "atual", "este", "este mes", "mes atual":
		return current, nil
	case "passado", "anterior", "mes passado":
		return current.Add(-1), nil
	case "proximo", "que vem", "mes que vem", "proximo mes":
		return current.Add(1), nil
	}
	t, err := time.Parse(monthLayout, strings.TrimSpace(s))
	if err != nil {
		return domain.YearMonth{}, invalid("mês %q inválido, use AAAA-MM", s)
	}
	return domain.YearMonthOf(t), nil
}

// parseLocalTime reads "YYYY-MM-DDTHH:MM" (or with seconds / offset) in the
// owner's time zone.
func (r *Registry) parseLocalTime(s string) (time.Time, error) {
	s = strings.TrimSpace(strings.Replace(s, " ", "T", 1))
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	for _, layout := range []string{"2006-01-02T15:04:05", "2006-01-02T15:04"} {
		if t, err := time.ParseInLocation(layout, s, r.deps.Location); err == nil {
			return t, nil
		}
	}
	return time.Time{}, invalid("data e hora %q inválidas, use AAAA-MM-DDTHH:MM", s)
}

func monthString(m domain.YearMonth) string { return m.FirstDay().Format(monthLayout) }

func cents(reais float64) domain.Cents { return domain.Cents(math.Round(reais * 100)) }

func money(c domain.Cents) string { return c.String() }

// normalize lowercases, trims and strips accents, for forgiving name matching.
func normalize(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	out, _, err := transform.String(t, strings.ToLower(strings.TrimSpace(s)))
	if err != nil {
		return strings.ToLower(strings.TrimSpace(s))
	}
	return out
}

// match finds the item whose name best matches query: an exact id, an exact
// name, then a unique prefix, then a unique substring. The second result is
// every name, for a helpful error when nothing (or too much) matched.
func match[T any](items []T, query string, id func(T) string, name func(T) string) (T, bool, []string) {
	var zero T
	q := normalize(query)
	names := make([]string, len(items))
	for i, it := range items {
		names[i] = name(it)
		if id(it) == strings.TrimSpace(query) || normalize(name(it)) == q {
			return it, true, nil
		}
	}
	for _, test := range []func(string) bool{
		func(n string) bool { return strings.HasPrefix(n, q) },
		func(n string) bool { return strings.Contains(n, q) },
	} {
		var found []T
		for _, it := range items {
			if q != "" && test(normalize(name(it))) {
				found = append(found, it)
			}
		}
		if len(found) == 1 {
			return found[0], true, nil
		}
		if len(found) > 1 {
			return zero, false, names
		}
	}
	return zero, false, names
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
