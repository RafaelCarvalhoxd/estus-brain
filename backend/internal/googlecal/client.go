// Package googlecal wraps the Google Calendar API and OAuth2 flow behind a
// small set of functions the service layer can call without knowing
// anything about the oauth2/calendar SDKs directly. It is its own package
// so it never needs to import, or be imported by, anything else agents are
// touching concurrently.
package googlecal

import (
	"context"
	"fmt"
	"os"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

var calendarScopes = []string{calendar.CalendarEventsScope}

// OAuthConfigFromEnv builds the OAuth2 config from GOOGLE_CLIENT_ID,
// GOOGLE_CLIENT_SECRET and GOOGLE_REDIRECT_URL. These usually aren't set in
// this app's environment — a human has to create real credentials in the
// Google Cloud Console — so callers must treat the returned error as an
// expected "not configured" state, not a crash.
func OAuthConfigFromEnv() (*oauth2.Config, error) {
	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	redirectURL := os.Getenv("GOOGLE_REDIRECT_URL")
	if clientID == "" || clientSecret == "" || redirectURL == "" {
		return nil, fmt.Errorf("google calendar integration is not configured: set GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET and GOOGLE_REDIRECT_URL")
	}
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes:       calendarScopes,
		Endpoint:     google.Endpoint,
	}, nil
}

func AuthURL(cfg *oauth2.Config, state string) string {
	return cfg.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
}

func Exchange(ctx context.Context, cfg *oauth2.Config, code string) (*oauth2.Token, error) {
	tok, err := cfg.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("exchange google oauth code: %w", err)
	}
	return tok, nil
}

func newService(ctx context.Context, cfg *oauth2.Config, token *oauth2.Token) (*calendar.Service, error) {
	svc, err := calendar.NewService(ctx, option.WithTokenSource(cfg.TokenSource(ctx, token)))
	if err != nil {
		return nil, fmt.Errorf("create calendar service: %w", err)
	}
	return svc, nil
}

// FetchEvents pulls events from the primary calendar in [from, to).
// SingleEvents expands recurring events into instances server-side, so
// callers never need to implement RRULE handling themselves.
func FetchEvents(ctx context.Context, cfg *oauth2.Config, token *oauth2.Token, from, to time.Time) ([]*calendar.Event, error) {
	svc, err := newService(ctx, cfg, token)
	if err != nil {
		return nil, err
	}
	resp, err := svc.Events.List("primary").
		TimeMin(from.Format(time.RFC3339)).
		TimeMax(to.Format(time.RFC3339)).
		SingleEvents(true).
		OrderBy("startTime").
		Context(ctx).
		Do()
	if err != nil {
		return nil, fmt.Errorf("list google calendar events: %w", err)
	}
	return resp.Items, nil
}

// CreateGoogleEvent pushes a locally-created event to the primary calendar
// and returns the new Google event ID.
func CreateGoogleEvent(ctx context.Context, cfg *oauth2.Config, token *oauth2.Token, title, location, notes string, startsAt, endsAt time.Time) (string, error) {
	svc, err := newService(ctx, cfg, token)
	if err != nil {
		return "", err
	}
	ev := &calendar.Event{
		Summary:     title,
		Location:    location,
		Description: notes,
		Start:       &calendar.EventDateTime{DateTime: startsAt.Format(time.RFC3339)},
		End:         &calendar.EventDateTime{DateTime: endsAt.Format(time.RFC3339)},
	}
	created, err := svc.Events.Insert("primary", ev).Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("create google calendar event: %w", err)
	}
	return created.Id, nil
}
