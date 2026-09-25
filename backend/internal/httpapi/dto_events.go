package httpapi

import (
	"time"

	"github.com/rafael/estus-vault/backend/internal/domain"
	"github.com/rafael/estus-vault/backend/internal/service"
)

type eventDTO struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Location string `json:"location"`
	Notes    string `json:"notes"`
	StartsAt string `json:"starts_at"`
	EndsAt   string `json:"ends_at"`
}

func toEventDTO(e domain.Event) eventDTO {
	return eventDTO{
		ID:       e.ID,
		Title:    e.Title,
		Location: e.Location,
		Notes:    e.Notes,
		StartsAt: e.StartsAt.Format(time.RFC3339),
		EndsAt:   e.EndsAt.Format(time.RFC3339),
	}
}

type createEventRequest struct {
	Title    string `json:"title"`
	Location string `json:"location,omitempty"`
	Notes    string `json:"notes,omitempty"`
	StartsAt string `json:"starts_at"` // RFC3339
	EndsAt   string `json:"ends_at"`   // RFC3339
}

func (r createEventRequest) toInput() (service.NewEventInput, error) {
	startsAt, err := time.Parse(time.RFC3339, r.StartsAt)
	if err != nil {
		return service.NewEventInput{}, err
	}
	endsAt, err := time.Parse(time.RFC3339, r.EndsAt)
	if err != nil {
		return service.NewEventInput{}, err
	}
	return service.NewEventInput{
		Title:    r.Title,
		Location: r.Location,
		Notes:    r.Notes,
		StartsAt: startsAt,
		EndsAt:   endsAt,
	}, nil
}
