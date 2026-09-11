package httpapi

import (
	"time"

	"github.com/rafael/estus-vault/backend/internal/domain"
)

type noteDTO struct {
	ID         string  `json:"id"`
	Title      string  `json:"title"`
	Body       string  `json:"body"`
	Pinned     bool    `json:"pinned"`
	CategoryID *string `json:"category_id,omitempty"`
	CreatedAt  string  `json:"created_at"`
	UpdatedAt  string  `json:"updated_at"`
}

func toNoteDTO(n domain.Note) noteDTO {
	return noteDTO{
		ID:         n.ID,
		Title:      n.Title,
		Body:       n.Body,
		Pinned:     n.Pinned,
		CategoryID: n.CategoryID,
		CreatedAt:  n.CreatedAt.Format(time.RFC3339),
		UpdatedAt:  n.UpdatedAt.Format(time.RFC3339),
	}
}

type createNoteRequest struct {
	Title      string  `json:"title"`
	Body       string  `json:"body"`
	Pinned     bool    `json:"pinned,omitempty"`
	CategoryID *string `json:"category_id,omitempty"`
}

type updateNoteRequest struct {
	Title      string  `json:"title"`
	Body       string  `json:"body"`
	Pinned     bool    `json:"pinned,omitempty"`
	CategoryID *string `json:"category_id,omitempty"`
}

type noteCategoryDTO struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Color     string `json:"color"`
	CreatedAt string `json:"created_at"`
}

func toNoteCategoryDTO(c domain.NoteCategory) noteCategoryDTO {
	return noteCategoryDTO{ID: c.ID, Name: c.Name, Color: c.Color, CreatedAt: c.CreatedAt.Format(time.RFC3339)}
}

type noteCategoryRequest struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}
