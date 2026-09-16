package service

import (
	"context"
	"fmt"
	"time"

	"github.com/rafael/estus-vault/backend/internal/domain"
	"github.com/rafael/estus-vault/backend/internal/store/postgres"
)

type HabitService struct {
	habits *postgres.HabitRepo
}

func NewHabitService(repo *postgres.HabitRepo) *HabitService {
	return &HabitService{habits: repo}
}

func (s *HabitService) List(ctx context.Context) ([]domain.Habit, error) {
	return s.habits.List(ctx)
}

func (s *HabitService) Create(ctx context.Context, h domain.Habit) (domain.Habit, error) {
	if err := h.Validate(); err != nil {
		return domain.Habit{}, err
	}
	if h.StartDay.IsZero() {
		return domain.Habit{}, fmt.Errorf("%w: start day is required", domain.ErrValidation)
	}
	return s.habits.Create(ctx, h)
}

func (s *HabitService) Update(ctx context.Context, id string, h domain.Habit) error {
	if err := h.Validate(); err != nil {
		return err
	}
	return s.habits.Update(ctx, id, h)
}

func (s *HabitService) Delete(ctx context.Context, id string) error {
	return s.habits.Delete(ctx, id)
}

func (s *HabitService) SetLog(ctx context.Context, id string, day time.Time, count int) error {
	if count < 0 || count > 10000 {
		return fmt.Errorf("%w: count must be between 0 and 10000", domain.ErrValidation)
	}
	return s.habits.SetLog(ctx, id, day, count)
}
