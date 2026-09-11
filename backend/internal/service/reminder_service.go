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
}

func NewReminderService(repo *postgres.ReminderRepo) *ReminderService {
	return &ReminderService{reminders: repo}
}

type NewReminderInput struct {
	Title string
	DueAt *time.Time
}

func (s *ReminderService) Create(ctx context.Context, in NewReminderInput) (domain.Reminder, error) {
	rem := domain.Reminder{Title: in.Title, DueAt: in.DueAt}
	if err := rem.Validate(); err != nil {
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

func (s *ReminderService) SetDone(ctx context.Context, id string, done bool) (domain.Reminder, error) {
	return s.reminders.SetDone(ctx, id, done)
}

func (s *ReminderService) Update(ctx context.Context, id string, in NewReminderInput) (domain.Reminder, error) {
	rem := domain.Reminder{Title: in.Title, DueAt: in.DueAt}
	if err := rem.Validate(); err != nil {
		return domain.Reminder{}, err
	}
	return s.reminders.Update(ctx, id, in.Title, in.DueAt)
}

func (s *ReminderService) Delete(ctx context.Context, id string) error {
	return s.reminders.Delete(ctx, id)
}
