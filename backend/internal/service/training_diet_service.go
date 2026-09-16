package service

import (
	"context"

	"github.com/rafael/estus-vault/backend/internal/domain"
	"github.com/rafael/estus-vault/backend/internal/store/postgres"
)

type TrainingService struct {
	workouts *postgres.WorkoutRepo
}

func NewTrainingService(repo *postgres.WorkoutRepo) *TrainingService {
	return &TrainingService{workouts: repo}
}

func (s *TrainingService) List(ctx context.Context) ([]domain.Workout, error) {
	return s.workouts.List(ctx)
}

func (s *TrainingService) Create(ctx context.Context, w domain.Workout) (domain.Workout, error) {
	if err := w.Validate(); err != nil {
		return domain.Workout{}, err
	}
	return s.workouts.Create(ctx, w)
}

func (s *TrainingService) Update(ctx context.Context, id string, w domain.Workout) (domain.Workout, error) {
	if err := w.Validate(); err != nil {
		return domain.Workout{}, err
	}
	return s.workouts.Update(ctx, id, w)
}

func (s *TrainingService) Delete(ctx context.Context, id string) error {
	return s.workouts.Delete(ctx, id)
}

type DietService struct {
	meals *postgres.MealRepo
}

func NewDietService(repo *postgres.MealRepo) *DietService {
	return &DietService{meals: repo}
}

func (s *DietService) List(ctx context.Context) ([]domain.Meal, error) {
	return s.meals.List(ctx)
}

func (s *DietService) Create(ctx context.Context, m domain.Meal) (domain.Meal, error) {
	if err := m.Validate(); err != nil {
		return domain.Meal{}, err
	}
	return s.meals.Create(ctx, m)
}

func (s *DietService) Update(ctx context.Context, id string, m domain.Meal) (domain.Meal, error) {
	if err := m.Validate(); err != nil {
		return domain.Meal{}, err
	}
	return s.meals.Update(ctx, id, m)
}

func (s *DietService) Delete(ctx context.Context, id string) error {
	return s.meals.Delete(ctx, id)
}

func (s *DietService) Targets(ctx context.Context) (domain.DietTargets, error) {
	return s.meals.Targets(ctx)
}

func (s *DietService) SetTargets(ctx context.Context, t domain.DietTargets) (domain.DietTargets, error) {
	if err := t.Validate(); err != nil {
		return domain.DietTargets{}, err
	}
	return s.meals.SetTargets(ctx, t)
}
