package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/rafael/estus-vault/backend/internal/domain"
	"github.com/rafael/estus-vault/backend/internal/service"
)

type NoteHandlers struct {
	notes *service.NoteService
}

func NewNoteHandlers(notes *service.NoteService) *NoteHandlers {
	return &NoteHandlers{notes: notes}
}

func (h *NoteHandlers) List(w http.ResponseWriter, r *http.Request) {
	notes, err := h.notes.List(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]noteDTO, len(notes))
	for i, n := range notes {
		out[i] = toNoteDTO(n)
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *NoteHandlers) Get(w http.ResponseWriter, r *http.Request) {
	n, err := h.notes.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toNoteDTO(n))
}

func (h *NoteHandlers) Create(w http.ResponseWriter, r *http.Request) {
	var req createNoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, fmt.Errorf("%w: invalid JSON body", domain.ErrValidation))
		return
	}
	n, err := h.notes.Create(r.Context(), req.Title, req.Body, req.Pinned)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toNoteDTO(n))
}

func (h *NoteHandlers) Update(w http.ResponseWriter, r *http.Request) {
	var req updateNoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, fmt.Errorf("%w: invalid JSON body", domain.ErrValidation))
		return
	}
	n, err := h.notes.Update(r.Context(), r.PathValue("id"), req.Title, req.Body, req.Pinned)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toNoteDTO(n))
}

func (h *NoteHandlers) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.notes.Delete(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}
