package httpapi

import (
	"time"

	"github.com/rafael/estus-vault/backend/internal/domain"
	"github.com/rafael/estus-vault/backend/internal/service"
)

type reminderDTO struct {
	ID        string  `json:"id"`
	Title     string  `json:"title"`
	DueAt     *string `json:"due_at,omitempty"`
	Done      bool    `json:"done"`
	CreatedAt string  `json:"created_at"`
}

func toReminderDTO(r domain.Reminder) reminderDTO {
	dto := reminderDTO{
		ID:        r.ID,
		Title:     r.Title,
		Done:      r.Done,
		CreatedAt: r.CreatedAt.Format(time.RFC3339),
	}
	if r.DueAt != nil {
		s := r.DueAt.Format(time.RFC3339)
		dto.DueAt = &s
	}
	return dto
}

type createReminderRequest struct {
	Title string  `json:"title"`
	DueAt *string `json:"due_at,omitempty"` // RFC3339, optional
}

func (r createReminderRequest) toInput() (service.NewReminderInput, error) {
	in := service.NewReminderInput{Title: r.Title}
	if r.DueAt != nil && *r.DueAt != "" {
		t, err := time.Parse(time.RFC3339, *r.DueAt)
		if err != nil {
			return service.NewReminderInput{}, err
		}
		in.DueAt = &t
	}
	return in, nil
}

type setReminderDoneRequest struct {
	Done bool `json:"done"`
}
