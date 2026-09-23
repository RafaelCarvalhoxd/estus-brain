package service

import (
	"context"
	"fmt"
	"time"

	"github.com/rafael/estus-vault/backend/internal/domain"
	"github.com/rafael/estus-vault/backend/internal/store/postgres"
)

type ReminderService struct {
	reminders *postgres.ReminderRepo
	// loc is the owner's timezone: repeat weekdays are counted in it.
	loc *time.Location
	now func() time.Time
}

func NewReminderService(repo *postgres.ReminderRepo, loc *time.Location) *ReminderService {
	return &ReminderService{reminders: repo, loc: loc, now: time.Now}
}

type NewReminderInput struct {
	Title          string
	DueAt          *time.Time
	RepeatDays     []time.Weekday
	RepeatMonthDay int
}

func (s *ReminderService) build(in NewReminderInput) (domain.Reminder, error) {
	rem := domain.Reminder{
		Title:          in.Title,
		DueAt:          in.DueAt,
		RepeatDays:     domain.NormalizeRepeatDays(in.RepeatDays),
		RepeatMonthDay: in.RepeatMonthDay,
	}
	if err := rem.Validate(); err != nil {
		return domain.Reminder{}, err
	}
	if rem.Repeats() {
		first := rem.FirstDue(s.loc)
		rem.DueAt = &first
	}
	return rem, nil
}

func (s *ReminderService) Create(ctx context.Context, in NewReminderInput) (domain.Reminder, error) {
	rem, err := s.build(in)
	if err != nil {
		return domain.Reminder{}, err
	}
	created, err := s.reminders.Create(ctx, rem)
	if err != nil {
		return domain.Reminder{}, fmt.Errorf("save reminder: %w", err)
	}
	return created, nil
}

func (s *ReminderService) List(ctx context.Context) ([]domain.Reminder, error) {
	return s.reminders.List(ctx)
}

// SetDone marks a reminder done. A repeating one never stays done: it moves
// to its next day instead.
func (s *ReminderService) SetDone(ctx context.Context, id string, done bool) (domain.Reminder, error) {
	if done {
		rem, err := s.reminders.Get(ctx, id)
		if err != nil {
			return domain.Reminder{}, err
		}
		if rem.Repeats() && rem.DueAt != nil {
			next := rem.NextDue(s.now(), s.loc)
			rem.DueAt = &next
			return s.reminders.Update(ctx, id, rem)
		}
	}
	return s.reminders.SetDone(ctx, id, done)
}

func (s *ReminderService) Update(ctx context.Context, id string, in NewReminderInput) (domain.Reminder, error) {
	rem, err := s.build(in)
	if err != nil {
		return domain.Reminder{}, err
	}
	return s.reminders.Update(ctx, id, rem)
}

func (s *ReminderService) Delete(ctx context.Context, id string) error {
	return s.reminders.Delete(ctx, id)
}
