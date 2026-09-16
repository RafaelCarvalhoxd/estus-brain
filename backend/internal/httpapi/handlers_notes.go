package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/rafael/estus-vault/backend/internal/domain"
	"github.com/rafael/estus-vault/backend/internal/service"
)

type NoteHandlers struct {
	notes      *service.NoteService
	categories *service.NoteCategoryService
}

func NewNoteHandlers(notes *service.NoteService, categories *service.NoteCategoryService) *NoteHandlers {
	return &NoteHandlers{notes: notes, categories: categories}
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

// decodeNote caps the body a little above the largest note allowed, so an
// oversized one gets a clear validation error rather than a cut connection.
func decodeNote(w http.ResponseWriter, r *http.Request) (domain.Note, error) {
	r.Body = http.MaxBytesReader(w, r.Body, domain.MaxNoteContentBytes+(2<<20))
	var req noteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			return domain.Note{}, fmt.Errorf("%w: note is larger than %d MB", domain.ErrValidation, domain.MaxNoteContentBytes>>20)
		}
		return domain.Note{}, fmt.Errorf("%w: invalid JSON body", domain.ErrValidation)
	}
	return req.toDomain(), nil
}

func (h *NoteHandlers) Create(w http.ResponseWriter, r *http.Request) {
	in, err := decodeNote(w, r)
	if err != nil {
		writeError(w, err)
		return
	}
	n, err := h.notes.Create(r.Context(), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toNoteDTO(n))
}

func (h *NoteHandlers) Update(w http.ResponseWriter, r *http.Request) {
	in, err := decodeNote(w, r)
	if err != nil {
		writeError(w, err)
		return
	}
	n, err := h.notes.Update(r.Context(), r.PathValue("id"), in)
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

func (h *NoteHandlers) ListCategories(w http.ResponseWriter, r *http.Request) {
	cats, err := h.categories.List(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]noteCategoryDTO, len(cats))
	for i, c := range cats {
		out[i] = toNoteCategoryDTO(c)
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *NoteHandlers) CreateCategory(w http.ResponseWriter, r *http.Request) {
	var req noteCategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, fmt.Errorf("%w: invalid JSON body", domain.ErrValidation))
		return
	}
	c, err := h.categories.Create(r.Context(), req.Name, req.Color)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toNoteCategoryDTO(c))
}

func (h *NoteHandlers) UpdateCategory(w http.ResponseWriter, r *http.Request) {
	var req noteCategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, fmt.Errorf("%w: invalid JSON body", domain.ErrValidation))
		return
	}
	c, err := h.categories.Update(r.Context(), r.PathValue("id"), req.Name, req.Color)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toNoteCategoryDTO(c))
}

func (h *NoteHandlers) DeleteCategory(w http.ResponseWriter, r *http.Request) {
	if err := h.categories.Delete(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
