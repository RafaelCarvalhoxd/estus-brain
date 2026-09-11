package httpapi

import (
	"time"

	"github.com/rafael/estus-vault/backend/internal/domain"
)

type noteDTO struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	Pinned    bool   `json:"pinned"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func toNoteDTO(n domain.Note) noteDTO {
	return noteDTO{
		ID:        n.ID,
		Title:     n.Title,
		Body:      n.Body,
		Pinned:    n.Pinned,
		CreatedAt: n.CreatedAt.Format(time.RFC3339),
		UpdatedAt: n.UpdatedAt.Format(time.RFC3339),
	}
}

type createNoteRequest struct {
	Title  string `json:"title"`
	Body   string `json:"body"`
	Pinned bool   `json:"pinned,omitempty"`
}

type updateNoteRequest struct {
	Title  string `json:"title"`
	Body   string `json:"body"`
	Pinned bool   `json:"pinned,omitempty"`
}
