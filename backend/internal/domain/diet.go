package domain

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"
)

var mealTime = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)

// Meal is one block of the diet plan — "Almoço, 12:30" — scheduled on some
// days of the week, made of the foods eaten in it.
type Meal struct {
	ID        string
	Name      string
	Time      string // "HH:MM", local time
	Weekdays  int
	Notes     string
	Items     []MealItem
	CreatedAt time.Time
	UpdatedAt time.Time
}

// MealItem is one food in a meal, with its macros for the quantity served.
type MealItem struct {
	ID       string
	Food     string
	Quantity string
	Kcal     float64
	ProteinG float64
	CarbsG   float64
	FatG     float64
}

// DietTargets are the daily goals the diet screen measures against. Zero
// means no target for that value.
type DietTargets struct {
	Kcal     float64
	ProteinG float64
	CarbsG   float64
	FatG     float64
}

func validMacro(v, max float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0 && v <= max
}

func (m Meal) Validate() error {
	name := strings.TrimSpace(m.Name)
	if name == "" {
		return fmt.Errorf("%w: meal name is required", ErrValidation)
	}
	if len(name) > 120 {
		return fmt.Errorf("%w: meal name is too long", ErrValidation)
	}
	if !mealTime.MatchString(m.Time) {
		return fmt.Errorf("%w: meal time must be HH:MM", ErrValidation)
	}
	if len(m.Notes) > 2000 {
		return fmt.Errorf("%w: notes are too long", ErrValidation)
	}
	if err := validateWeekdays(m.Weekdays); err != nil {
		return err
	}
	if len(m.Items) > 50 {
		return fmt.Errorf("%w: too many foods in one meal", ErrValidation)
	}
	for i, it := range m.Items {
		if strings.TrimSpace(it.Food) == "" {
			return fmt.Errorf("%w: food %d needs a name", ErrValidation, i+1)
		}
		if len(it.Food) > 120 || len(it.Quantity) > 40 {
			return fmt.Errorf("%w: food %d has a field that is too long", ErrValidation, i+1)
		}
		if !validMacro(it.Kcal, 10000) || !validMacro(it.ProteinG, 2000) || !validMacro(it.CarbsG, 2000) || !validMacro(it.FatG, 2000) {
			return fmt.Errorf("%w: food %d has invalid macros", ErrValidation, i+1)
		}
	}
	return nil
}

func (t DietTargets) Validate() error {
	if !validMacro(t.Kcal, 20000) || !validMacro(t.ProteinG, 2000) || !validMacro(t.CarbsG, 3000) || !validMacro(t.FatG, 2000) {
		return fmt.Errorf("%w: targets must be zero or positive", ErrValidation)
	}
	return nil
}
