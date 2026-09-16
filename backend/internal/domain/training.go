package domain

import (
	"fmt"
	"strings"
	"time"
)

// Days of the week travel as a 7-bit mask: bit 0 = Sunday … bit 6 =
// Saturday, the same numbering as JavaScript's Date.getDay(), so the
// frontend never has to translate.
const allWeekdays = 1<<7 - 1

// DaysToMask turns a list of weekdays (0–6) into the stored mask.
func DaysToMask(days []int) (int, error) {
	mask := 0
	for _, d := range days {
		if d < 0 || d > 6 {
			return 0, fmt.Errorf("%w: invalid weekday %d", ErrValidation, d)
		}
		mask |= 1 << d
	}
	return mask, nil
}

// MaskToDays lists the weekdays set in a mask, Sunday first.
func MaskToDays(mask int) []int {
	days := []int{}
	for d := 0; d < 7; d++ {
		if mask&(1<<d) != 0 {
			days = append(days, d)
		}
	}
	return days
}

func validateWeekdays(mask int) error {
	if mask < 1 || mask > allWeekdays {
		return fmt.Errorf("%w: pick at least one day of the week", ErrValidation)
	}
	return nil
}

// Workout is one session in the training plan — "Treino A — peito e
// tríceps" — scheduled on some days of the week.
type Workout struct {
	ID        string
	Name      string
	Focus     string
	Weekdays  int
	Notes     string
	Exercises []Exercise
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Exercise is one line of a workout. Reps and weight are free text because
// plans are written that way: "8-12", "até a falha", "20 kg", "peso corporal".
type Exercise struct {
	ID          string
	Name        string
	Sets        int
	Reps        string
	Weight      string
	RestSeconds int
	Notes       string
}

func (w Workout) Validate() error {
	name := strings.TrimSpace(w.Name)
	if name == "" {
		return fmt.Errorf("%w: workout name is required", ErrValidation)
	}
	if len(name) > 120 || len(w.Focus) > 120 {
		return fmt.Errorf("%w: name or focus is too long", ErrValidation)
	}
	if len(w.Notes) > 2000 {
		return fmt.Errorf("%w: notes are too long", ErrValidation)
	}
	if err := validateWeekdays(w.Weekdays); err != nil {
		return err
	}
	if len(w.Exercises) > 60 {
		return fmt.Errorf("%w: too many exercises", ErrValidation)
	}
	for i, e := range w.Exercises {
		if strings.TrimSpace(e.Name) == "" {
			return fmt.Errorf("%w: exercise %d needs a name", ErrValidation, i+1)
		}
		if len(e.Name) > 120 || len(e.Reps) > 20 || len(e.Weight) > 30 || len(e.Notes) > 500 {
			return fmt.Errorf("%w: exercise %d has a field that is too long", ErrValidation, i+1)
		}
		if e.Sets < 1 || e.Sets > 50 {
			return fmt.Errorf("%w: exercise %d needs between 1 and 50 sets", ErrValidation, i+1)
		}
		if e.RestSeconds < 0 || e.RestSeconds > 3600 {
			return fmt.Errorf("%w: exercise %d rest must be between 0 and 3600 seconds", ErrValidation, i+1)
		}
	}
	return nil
}
