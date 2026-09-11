package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/rafael/estus-vault/backend/internal/domain"
	"github.com/rafael/estus-vault/backend/internal/service"
)

type EventHandlers struct {
	events *service.EventService
	google *service.GoogleService
}

func NewEventHandlers(events *service.EventService, google *service.GoogleService) *EventHandlers {
	return &EventHandlers{events: events, google: google}
}

// defaultAgendaWindow mirrors the frontend's "próximos 30 dias" view, with
// a week of lookback so recently-past events don't vanish immediately.
func defaultAgendaWindow() (time.Time, time.Time) {
	now := time.Now()
	return now.AddDate(0, 0, -7), now.AddDate(0, 0, 30)
}

// ListRange handles GET /api/events?from=RFC3339&to=RFC3339.
func (h *EventHandlers) ListRange(w http.ResponseWriter, r *http.Request) {
	from, to := defaultAgendaWindow()
	if raw := r.URL.Query().Get("from"); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			writeError(w, fmt.Errorf("%w: from must be RFC3339", domain.ErrValidation))
			return
		}
		from = parsed
	}
	if raw := r.URL.Query().Get("to"); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			writeError(w, fmt.Errorf("%w: to must be RFC3339", domain.ErrValidation))
			return
		}
		to = parsed
	}

	events, err := h.events.ListRange(r.Context(), from, to)
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]eventDTO, len(events))
	for i, e := range events {
		out[i] = toEventDTO(e)
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *EventHandlers) Create(w http.ResponseWriter, r *http.Request) {
	var req createEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, fmt.Errorf("%w: invalid JSON body", domain.ErrValidation))
		return
	}
	input, err := req.toInput()
	if err != nil {
		writeError(w, fmt.Errorf("%w: starts_at/ends_at must be RFC3339", domain.ErrValidation))
		return
	}

	event, err := h.events.Create(r.Context(), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toEventDTO(event))
}

// Update handles PUT /api/events/{id}. Editing a locally-created event never
// re-pushes to Google — the initial best-effort push already linked it (or
// didn't); this keeps the update path simple and local-first.
func (h *EventHandlers) Update(w http.ResponseWriter, r *http.Request) {
	var req createEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, fmt.Errorf("%w: invalid JSON body", domain.ErrValidation))
		return
	}
	input, err := req.toInput()
	if err != nil {
		writeError(w, fmt.Errorf("%w: starts_at/ends_at must be RFC3339", domain.ErrValidation))
		return
	}

	event, err := h.events.Update(r.Context(), r.PathValue("id"), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toEventDTO(event))
}

// Delete handles DELETE /api/events/{id}.
func (h *EventHandlers) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.events.Delete(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

func (h *EventHandlers) GoogleStatus(w http.ResponseWriter, r *http.Request) {
	connected, err := h.google.IsConnected(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, googleStatusDTO{Connected: connected})
}

// GoogleAuthStart handles GET /api/google/oauth/start. It either redirects
// the browser straight to Google's consent screen, or — when
// GOOGLE_CLIENT_ID/SECRET/REDIRECT_URL aren't configured, which is the
// expected state until a human sets up real credentials — responds with a
// clear JSON error instead of crashing.
func (h *EventHandlers) GoogleAuthStart(w http.ResponseWriter, r *http.Request) {
	state, err := randomState()
	if err != nil {
		writeError(w, err)
		return
	}
	url, err := h.google.AuthURL(state)
	if err != nil {
		writeJSON(w, http.StatusNotImplemented, errorBody{Error: err.Error()})
		return
	}
	http.Redirect(w, r, url, http.StatusFound)
}

// GoogleCallback handles GET /api/google/oauth/callback, the redirect
// target Google sends the browser back to after consent.
func (h *EventHandlers) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	agendaURL := frontendAgendaURL()
	if code == "" {
		http.Redirect(w, r, agendaURL+"?google_error=1", http.StatusFound)
		return
	}
	if err := h.google.HandleCallback(r.Context(), code); err != nil {
		http.Redirect(w, r, agendaURL+"?google_error=1", http.StatusFound)
		return
	}
	http.Redirect(w, r, agendaURL+"?google_connected=1", http.StatusFound)
}

// GoogleSync handles POST /api/google/sync: triggers a pull for the default
// agenda window and reports how many events were imported/updated.
func (h *EventHandlers) GoogleSync(w http.ResponseWriter, r *http.Request) {
	from, to := defaultAgendaWindow()
	imported, err := h.google.Sync(r.Context(), h.events.Repo(), from, to)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, googleSyncResultDTO{Imported: imported})
}

func frontendAgendaURL() string {
	base := os.Getenv("FRONTEND_URL")
	if base == "" {
		base = "http://localhost:3000"
	}
	return base + "/agenda"
}

func randomState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate oauth state: %w", err)
	}
	return hex.EncodeToString(b), nil
}
