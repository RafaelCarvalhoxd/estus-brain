package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/rafael/estus-vault/backend/internal/domain"
	"github.com/rafael/estus-vault/backend/internal/googlecal"
	"github.com/rafael/estus-vault/backend/internal/store/postgres"
)

type EventService struct {
	events *postgres.EventRepo
	google *GoogleService
}

func NewEventService(repo *postgres.EventRepo, google *GoogleService) *EventService {
	return &EventService{events: repo, google: google}
}

// Repo exposes the underlying repository for the sync pull path, which
// needs to upsert-by-google-id directly rather than through Create/Delete.
func (s *EventService) Repo() *postgres.EventRepo { return s.events }

type NewEventInput struct {
	Title    string
	Location string
	Notes    string
	StartsAt time.Time
	EndsAt   time.Time
}

// Create saves the event locally and, when Google Calendar is connected,
// best-effort pushes it there too. A failed push never fails the request:
// the local ledger of what's on the agenda is the source of truth, and
// sync is a convenience on top of it.
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

	s.pushToGoogleBestEffort(ctx, created)
	return created, nil
}

func (s *EventService) pushToGoogleBestEffort(ctx context.Context, e domain.Event) {
	if s.google == nil {
		return
	}
	connected, err := s.google.IsConnected(ctx)
	if err != nil || !connected {
		return
	}
	cfg, tok, err := s.google.currentToken(ctx)
	if err != nil {
		slog.Warn("skip google push: token unavailable", "error", err)
		return
	}
	googleID, err := googlecal.CreateGoogleEvent(ctx, cfg, tok, e.Title, e.Location, e.Notes, e.StartsAt, e.EndsAt)
	if err != nil {
		slog.Warn("push event to google failed", "event_id", e.ID, "error", err)
		return
	}
	if err := s.events.SetGoogleEventID(ctx, e.ID, googleID); err != nil {
		slog.Warn("link local event to google id failed", "event_id", e.ID, "error", err)
	}
}

func (s *EventService) ListRange(ctx context.Context, from, to time.Time) ([]domain.Event, error) {
	events, err := s.events.ListRange(ctx, from, to)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	return events, nil
}

func (s *EventService) Delete(ctx context.Context, id string) error {
	return s.events.Delete(ctx, id)
}
