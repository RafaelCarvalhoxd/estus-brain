package domain

import (
	"fmt"
	"time"
)

// Event is a local agenda entry. GoogleEventID is set once the event is
// linked to (created from, or pushed to) a Google Calendar event; nil means
// it only exists locally.
type Event struct {
	ID            string
	Title         string
	Location      string
	Notes         string
	StartsAt      time.Time
	EndsAt        time.Time
	GoogleEventID *string
	CreatedAt     time.Time
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
