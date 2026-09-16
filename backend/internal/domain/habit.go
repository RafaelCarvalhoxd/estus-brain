package domain

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

type HabitKind string

const (
	// HabitCheck is done or not done: "ler 20 minutos".
	HabitCheck HabitKind = "check"
	// HabitCount is counted toward a daily target: "8 copos de água".
	HabitCount HabitKind = "count"
)

// DayLayout is how a calendar day crosses the API and is kept in the domain.
// Days are the owner's days: the frontend decides what "today" is in their
// time zone and the backend never converts.
const DayLayout = "2006-01-02"

var hexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

type Habit struct {
	ID       string
	Name     string
	Kind     HabitKind
	Target   int
	Unit     string
	Weekdays int
	Color    string
	Archived bool
	StartDay time.Time
	// Logs maps a day (DayLayout) to how much was done on it.
	Logs      map[string]int
	CreatedAt time.Time
}

func (h *Habit) Validate() error {
	h.Name = strings.TrimSpace(h.Name)
	h.Unit = strings.TrimSpace(h.Unit)
	if h.Name == "" {
		return fmt.Errorf("%w: habit name is required", ErrValidation)
	}
	if len([]rune(h.Name)) > 80 {
		return fmt.Errorf("%w: habit name is too long", ErrValidation)
	}
	switch h.Kind {
	case HabitCheck:
		h.Target = 1
		h.Unit = ""
	case HabitCount:
		if h.Target < 1 || h.Target > 1000 {
			return fmt.Errorf("%w: target must be between 1 and 1000", ErrValidation)
		}
		if len([]rune(h.Unit)) > 20 {
			return fmt.Errorf("%w: unit is too long", ErrValidation)
		}
	default:
		return fmt.Errorf("%w: kind must be check or count", ErrValidation)
	}
	if h.Weekdays < 1 || h.Weekdays > 127 {
		return fmt.Errorf("%w: pick at least one weekday", ErrValidation)
	}
	if !hexColor.MatchString(h.Color) {
		return fmt.Errorf("%w: color must be #rrggbb", ErrValidation)
	}
	return nil
}

// ScheduledOn reports whether the habit is meant to be done on that day.
func (h Habit) ScheduledOn(day time.Time) bool {
	return h.Weekdays&(1<<int(day.Weekday())) != 0
}

// DoneOn reports whether the day's target was met.
func (h Habit) DoneOn(day time.Time) bool {
	return h.Logs[day.Format(DayLayout)] >= h.Target
}

// Streaks walks the habit's scheduled days from its start to today. A run
// grows with every scheduled day whose target was met and breaks on one that
// wasn't — except today, which is still in progress. Days off the schedule
// neither extend nor break a run.
func (h Habit) Streaks(today time.Time) (current, best int) {
	start := h.StartDay
	for day := range h.Logs {
		if d, err := time.Parse(DayLayout, day); err == nil && d.Before(start) {
			start = d
		}
	}
	for day := start; !day.After(today); day = day.AddDate(0, 0, 1) {
		if !h.ScheduledOn(day) {
			continue
		}
		if h.DoneOn(day) {
			current++
			best = max(best, current)
		} else if !day.Equal(today) {
			current = 0
		}
	}
	return current, best
}

// ParseDay reads a calendar day, rejecting anything that isn't one.
func ParseDay(s string) (time.Time, error) {
	d, err := time.Parse(DayLayout, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: day must be YYYY-MM-DD", ErrValidation)
	}
	return d, nil
}
