package httpapi

import (
	"time"

	"github.com/rafael/estus-vault/backend/internal/domain"
	"github.com/rafael/estus-vault/backend/internal/service"
)

type reminderDTO struct {
	ID    string  `json:"id"`
	Title string  `json:"title"`
	DueAt *string `json:"due_at,omitempty"`
	Done  bool    `json:"done"`
	// RepeatDays are weekdays, 0 = Sunday … 6 = Saturday.
	RepeatDays []int `json:"repeat_days"`
	// RepeatMonthDay is the day of the month it repeats on; absent = none.
	RepeatMonthDay *int   `json:"repeat_month_day,omitempty"`
	CreatedAt      string `json:"created_at"`
}

func toReminderDTO(r domain.Reminder) reminderDTO {
	dto := reminderDTO{
		ID:         r.ID,
		Title:      r.Title,
		Done:       r.Done,
		CreatedAt:  r.CreatedAt.Format(time.RFC3339),
		RepeatDays: []int{},
	}
	for _, d := range r.RepeatDays {
		dto.RepeatDays = append(dto.RepeatDays, int(d))
	}
	if r.RepeatMonthDay != 0 {
		day := r.RepeatMonthDay
		dto.RepeatMonthDay = &day
	}
	if r.DueAt != nil {
		s := r.DueAt.Format(time.RFC3339)
		dto.DueAt = &s
	}
	return dto
}

type createReminderRequest struct {
	Title          string  `json:"title"`
	DueAt          *string `json:"due_at,omitempty"` // RFC3339, optional
	RepeatDays     []int   `json:"repeat_days,omitempty"`
	RepeatMonthDay int     `json:"repeat_month_day,omitempty"`
}

func (r createReminderRequest) toInput() (service.NewReminderInput, error) {
	in := service.NewReminderInput{Title: r.Title, RepeatMonthDay: r.RepeatMonthDay}
	for _, d := range r.RepeatDays {
		in.RepeatDays = append(in.RepeatDays, time.Weekday(d))
	}
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
