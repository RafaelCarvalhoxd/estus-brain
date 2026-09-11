package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/rafael/estus-vault/backend/internal/domain"
	"github.com/rafael/estus-vault/backend/internal/service"
)

type ReminderHandlers struct {
	reminders *service.ReminderService
}

func NewReminderHandlers(reminders *service.ReminderService) *ReminderHandlers {
	return &ReminderHandlers{reminders: reminders}
}

func (h *ReminderHandlers) List(w http.ResponseWriter, r *http.Request) {
	reminders, err := h.reminders.List(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]reminderDTO, len(reminders))
	for i, rem := range reminders {
		out[i] = toReminderDTO(rem)
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *ReminderHandlers) Create(w http.ResponseWriter, r *http.Request) {
	var req createReminderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, fmt.Errorf("%w: invalid JSON body", domain.ErrValidation))
		return
	}
	input, err := req.toInput()
	if err != nil {
		writeError(w, fmt.Errorf("%w: due_at must be RFC3339", domain.ErrValidation))
		return
	}
	rem, err := h.reminders.Create(r.Context(), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toReminderDTO(rem))
}

// SetDone handles PATCH /api/reminders/{id}.
func (h *ReminderHandlers) SetDone(w http.ResponseWriter, r *http.Request) {
	var req setReminderDoneRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, fmt.Errorf("%w: invalid JSON body", domain.ErrValidation))
		return
	}
	rem, err := h.reminders.SetDone(r.Context(), r.PathValue("id"), req.Done)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toReminderDTO(rem))
}

// Update handles PUT /api/reminders/{id} — title and due date only; use
// PATCH to toggle done.
func (h *ReminderHandlers) Update(w http.ResponseWriter, r *http.Request) {
	var req createReminderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, fmt.Errorf("%w: invalid JSON body", domain.ErrValidation))
		return
	}
	input, err := req.toInput()
	if err != nil {
		writeError(w, fmt.Errorf("%w: due_at must be RFC3339", domain.ErrValidation))
		return
	}
	rem, err := h.reminders.Update(r.Context(), r.PathValue("id"), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toReminderDTO(rem))
}

func (h *ReminderHandlers) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.reminders.Delete(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}
