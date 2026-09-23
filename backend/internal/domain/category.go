package domain

import (
	"fmt"
	"strings"
	"time"
)

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

// CategoryKind says which way the money goes.
type CategoryKind string

const (
	KindExpense CategoryKind = "despesa"
	KindIncome  CategoryKind = "receita"
)

func (k CategoryKind) Valid() bool { return k == KindExpense || k == KindIncome }

type Category struct {
	ID                 string
	Name               string
	Nature             CategoryNature
	Kind               CategoryKind
	Color              string // hex, used by the frontend chart legend
	MonthlyBudgetCents *Cents // nil = no budget set
	CreatedAt          time.Time
}

func (c Category) Validate() error {
	if strings.TrimSpace(c.Name) == "" {
		return fmt.Errorf("%w: name is required", ErrValidation)
	}
	if !c.Nature.Valid() {
		return fmt.Errorf("%w: invalid nature %q", ErrValidation, c.Nature)
	}
	if !c.Kind.Valid() {
		return fmt.Errorf("%w: invalid kind %q (despesa or receita)", ErrValidation, c.Kind)
	}
	if strings.TrimSpace(c.Color) == "" {
		return fmt.Errorf("%w: color is required", ErrValidation)
	}
	return nil
}
