package domain

import (
	"fmt"
	"time"
)

// Reminder is a simple to-do with an optional due date/time — it may just
// be "someday", in which case DueAt is nil.
type Reminder struct {
	ID        string
	Title     string
	DueAt     *time.Time
	Done      bool
	CreatedAt time.Time
}

func (r Reminder) Validate() error {
	if r.Title == "" {
		return fmt.Errorf("%w: title is required", ErrValidation)
	}
	return nil
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
