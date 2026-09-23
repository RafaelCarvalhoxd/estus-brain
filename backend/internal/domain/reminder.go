package domain

import (
	"fmt"
	"slices"
	"time"
)

// Reminder is a simple to-do with an optional due date/time — it may just
// be "someday", in which case DueAt is nil.
type Reminder struct {
	ID    string
	Title string
	DueAt *time.Time
	Done  bool
	// RepeatDays, when set, makes the reminder come back: completing it
	// moves DueAt to the next of these weekdays, at the same local time.
	RepeatDays []time.Weekday
	// RepeatMonthDay (1-31), when set, repeats it monthly on that day
	// instead; 0 means no monthly repeat.
	RepeatMonthDay int
	CreatedAt      time.Time
}

func (r Reminder) Validate() error {
	if r.Title == "" {
		return fmt.Errorf("%w: title is required", ErrValidation)
	}
	if r.Repeats() && r.DueAt == nil {
		return fmt.Errorf("%w: a repeating reminder needs a date", ErrValidation)
	}
	if len(r.RepeatDays) > 0 && r.RepeatMonthDay != 0 {
		return fmt.Errorf("%w: repeat on weekdays or on a day of the month, not both", ErrValidation)
	}
	if r.RepeatMonthDay < 0 || r.RepeatMonthDay > 31 {
		return fmt.Errorf("%w: repeat_month_day must be 1-31", ErrValidation)
	}
	for _, d := range r.RepeatDays {
		if d < time.Sunday || d > time.Saturday {
			return fmt.Errorf("%w: repeat day %d is not a weekday (0-6)", ErrValidation, d)
		}
	}
	return nil
}

func (r Reminder) Repeats() bool { return len(r.RepeatDays) > 0 || r.RepeatMonthDay != 0 }

// NormalizeRepeatDays sorts the days and drops duplicates.
func NormalizeRepeatDays(days []time.Weekday) []time.Weekday {
	out := slices.Clone(days)
	slices.Sort(out)
	return slices.Compact(out)
}

// FirstDue moves DueAt forward, if needed, to the first repeat day, so a
// reminder set for Tuesday that repeats on Mondays starts next Monday.
func (r Reminder) FirstDue(loc *time.Location) time.Time {
	due := r.DueAt.In(loc)
	if r.RepeatMonthDay != 0 {
		if first := r.inMonth(due, 0); !first.Before(due) {
			return first
		}
		return r.inMonth(due, 1)
	}
	for i := 0; i < 7 && !slices.Contains(r.RepeatDays, due.Weekday()); i++ {
		due = due.AddDate(0, 0, 1)
	}
	return due
}

// NextDue is the first repeat day after both DueAt and now, at DueAt's
// local time — completing a late reminder skips the days already missed.
func (r Reminder) NextDue(now time.Time, loc *time.Location) time.Time {
	due := r.DueAt.In(loc)
	if r.RepeatMonthDay != 0 {
		for months := 1; ; months++ {
			if next := r.inMonth(due, months); next.After(now) {
				return next
			}
		}
	}
	for {
		due = due.AddDate(0, 0, 1)
		if due.After(now) && slices.Contains(r.RepeatDays, due.Weekday()) {
			return due
		}
	}
}

// inMonth is RepeatMonthDay in the month `months` after from's, at from's
// clock time, clamped to that month's last day.
func (r Reminder) inMonth(from time.Time, months int) time.Time {
	y, m, _ := from.Date()
	first := time.Date(y, m+time.Month(months), 1, from.Hour(), from.Minute(), from.Second(), 0, from.Location())
	last := first.AddDate(0, 1, -1).Day()
	return first.AddDate(0, 0, min(r.RepeatMonthDay, last)-1)
}

// Bucket classifies a reminder for the "Lembretes" board relative to now,
// independent of any storage or presentation concern.
func (r Reminder) Bucket(now time.Time) string {
	if r.Done {
		return "concluido"
	}
	if r.DueAt == nil {
		return "proximo"
	}
	if r.DueAt.Before(now) {
		return "atrasado"
	}
	if sameDay(*r.DueAt, now) {
		return "hoje"
	}
	return "proximo"
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}
