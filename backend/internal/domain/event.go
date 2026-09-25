package domain

import (
	"fmt"
	"time"
)

// Event is an agenda entry.
type Event struct {
	ID        string
	Title     string
	Location  string
	Notes     string
	StartsAt  time.Time
	EndsAt    time.Time
	CreatedAt time.Time
}

func (e Event) Validate() error {
	if e.Title == "" {
		return fmt.Errorf("%w: title is required", ErrValidation)
	}
	if !e.EndsAt.After(e.StartsAt) {
		return fmt.Errorf("%w: ends_at must be after starts_at", ErrValidation)
	}
	return nil
}
