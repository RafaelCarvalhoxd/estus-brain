package domain

import "time"

// CategoryNature classifies why money left the account, independent of what
// it was spent on — this is the "natureza da operação" the budget cares
// about: is this something you must pay, something you chose to pay, or
// money moved into future value.
type CategoryNature string

const (
	NatureEssential     CategoryNature = "essencial"    // rent, utilities, groceries
	NatureDiscretionary CategoryNature = "variavel"     // dining out, entertainment
	NatureInvestment    CategoryNature = "investimento" // savings, investments
)

func (n CategoryNature) Valid() bool {
	switch n {
	case NatureEssential, NatureDiscretionary, NatureInvestment:
		return true
	}
	return false
}

type Category struct {
	ID        string
	Name      string
	Nature    CategoryNature
	Color     string // hex, used by the frontend chart legend
	CreatedAt time.Time
}
