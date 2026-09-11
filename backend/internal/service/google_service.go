package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/api/calendar/v3"

	"github.com/rafael/estus-vault/backend/internal/domain"
	"github.com/rafael/estus-vault/backend/internal/googlecal"
	"github.com/rafael/estus-vault/backend/internal/store/postgres"
)

// GoogleService owns the single connected Google account: the OAuth
// handshake, token persistence and refresh, and pulling events into the
// local store. Everything here degrades to "not connected" rather than
// erroring loudly when GOOGLE_CLIENT_ID/SECRET/REDIRECT_URL aren't set,
// since that's the expected state until a human sets up real credentials.
type GoogleService struct {
	tokens *postgres.GoogleTokenRepo
}

func NewGoogleService(tokens *postgres.GoogleTokenRepo) *GoogleService {
	return &GoogleService{tokens: tokens}
}

func (s *GoogleService) IsConnected(ctx context.Context) (bool, error) {
	_, err := s.tokens.Get(ctx)
	if errors.Is(err, domain.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check google connection: %w", err)
	}
	return true, nil
}

func (s *GoogleService) AuthURL(state string) (string, error) {
	cfg, err := googlecal.OAuthConfigFromEnv()
	if err != nil {
		return "", err
	}
	return googlecal.AuthURL(cfg, state), nil
}

func (s *GoogleService) HandleCallback(ctx context.Context, code string) error {
	cfg, err := googlecal.OAuthConfigFromEnv()
	if err != nil {
		return err
	}
	tok, err := googlecal.Exchange(ctx, cfg, code)
	if err != nil {
		return err
	}
	return s.saveToken(ctx, tok)
}

func (s *GoogleService) saveToken(ctx context.Context, tok *oauth2.Token) error {
	return s.tokens.Save(ctx, postgres.GoogleToken{
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		Expiry:       tok.Expiry,
	})
}

// currentToken loads the stored token and, when it's expired, refreshes it
// through the oauth2 TokenSource and persists the refreshed pair — the
// stored refresh_token from the initial consent keeps this working
// indefinitely without asking the user to reconnect.
func (s *GoogleService) currentToken(ctx context.Context) (*oauth2.Config, *oauth2.Token, error) {
	cfg, err := googlecal.OAuthConfigFromEnv()
	if err != nil {
		return nil, nil, err
	}
	stored, err := s.tokens.Get(ctx)
	if err != nil {
		return nil, nil, err
	}
	tok := &oauth2.Token{
		AccessToken:  stored.AccessToken,
		RefreshToken: stored.RefreshToken,
		Expiry:       stored.Expiry,
	}
	if !tok.Valid() {
		refreshed, err := cfg.TokenSource(ctx, tok).Token()
		if err != nil {
			return nil, nil, fmt.Errorf("refresh google token: %w", err)
		}
		if err := s.saveToken(ctx, refreshed); err != nil {
			return nil, nil, err
		}
		tok = refreshed
	}
	return cfg, tok, nil
}

// Sync pulls events from Google Calendar in [from, to) and upserts them
// into the local store, keyed by google_event_id so re-running it never
// duplicates rows.
func (s *GoogleService) Sync(ctx context.Context, eventRepo *postgres.EventRepo, from, to time.Time) (int, error) {
	cfg, tok, err := s.currentToken(ctx)
	if err != nil {
		return 0, err
	}
	items, err := googlecal.FetchEvents(ctx, cfg, tok, from, to)
	if err != nil {
		return 0, err
	}

	imported := 0
	for _, item := range items {
		if item.Id == "" || item.Status == "cancelled" {
			continue
		}
		startsAt, ok := parseGoogleTime(item.Start)
		if !ok {
			slog.Warn("skip google event with no start time", "event_id", item.Id)
			continue
		}
		endsAt, ok := parseGoogleTime(item.End)
		if !ok {
			endsAt = startsAt.Add(time.Hour)
		}
		e := domain.Event{
			Title:    item.Summary,
			Location: item.Location,
			Notes:    item.Description,
			StartsAt: startsAt,
			EndsAt:   endsAt,
		}
		if e.Title == "" {
			e.Title = "(sem título)"
		}
		if err := eventRepo.UpsertFromGoogle(ctx, item.Id, e); err != nil {
			return imported, fmt.Errorf("upsert synced event %s: %w", item.Id, err)
		}
		imported++
	}
	return imported, nil
}

// parseGoogleTime handles both timed events (DateTime, RFC3339) and
// all-day events (Date only, "2026-01-02") without crashing on the latter.
func parseGoogleTime(dt *calendar.EventDateTime) (time.Time, bool) {
	if dt == nil {
		return time.Time{}, false
	}
	if dt.DateTime != "" {
		t, err := time.Parse(time.RFC3339, dt.DateTime)
		if err != nil {
			return time.Time{}, false
		}
		return t, true
	}
	if dt.Date != "" {
		t, err := time.ParseInLocation("2006-01-02", dt.Date, time.Local)
		if err != nil {
			return time.Time{}, false
		}
		return t, true
	}
	return time.Time{}, false
}
