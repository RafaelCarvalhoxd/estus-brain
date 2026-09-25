package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/rafael/estus-vault/backend/internal/domain"
	"github.com/rafael/estus-vault/backend/internal/service"
)

type EventHandlers struct {
	events *service.EventService
}

func NewEventHandlers(events *service.EventService) *EventHandlers {
	return &EventHandlers{events: events}
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

// Update handles PUT /api/events/{id}.
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
