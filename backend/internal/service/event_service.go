package service

import (
	"context"
	"fmt"
	"time"

	"github.com/rafael/estus-vault/backend/internal/domain"
	"github.com/rafael/estus-vault/backend/internal/store/postgres"
)

type EventService struct {
	events *postgres.EventRepo
}

func NewEventService(repo *postgres.EventRepo) *EventService {
	return &EventService{events: repo}
}

type NewEventInput struct {
	Title    string
	Location string
	Notes    string
	StartsAt time.Time
	EndsAt   time.Time
}

func (s *EventService) Create(ctx context.Context, in NewEventInput) (domain.Event, error) {
	e := domain.Event{
		Title:    in.Title,
		Location: in.Location,
		Notes:    in.Notes,
		StartsAt: in.StartsAt,
		EndsAt:   in.EndsAt,
	}
	if err := e.Validate(); err != nil {
		return domain.Event{}, err
	}

	created, err := s.events.Create(ctx, e)
	if err != nil {
		return domain.Event{}, fmt.Errorf("save event: %w", err)
	}

	return created, nil
}

func (s *EventService) ListRange(ctx context.Context, from, to time.Time) ([]domain.Event, error) {
	events, err := s.events.ListRange(ctx, from, to)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	return events, nil
}

func (s *EventService) Update(ctx context.Context, id string, in NewEventInput) (domain.Event, error) {
	e := domain.Event{
		ID:       id,
		Title:    in.Title,
		Location: in.Location,
		Notes:    in.Notes,
		StartsAt: in.StartsAt,
		EndsAt:   in.EndsAt,
	}
	if err := e.Validate(); err != nil {
		return domain.Event{}, err
	}
	return s.events.Update(ctx, e)
}

func (s *EventService) Delete(ctx context.Context, id string) error {
	return s.events.Delete(ctx, id)
}
