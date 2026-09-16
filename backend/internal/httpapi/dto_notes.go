package httpapi

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/rafael/estus-vault/backend/internal/domain"
)

type noteDTO struct {
	ID         string          `json:"id"`
	Title      string          `json:"title"`
	Body       string          `json:"body"`
	Content    json.RawMessage `json:"content,omitempty"`
	Pinned     bool            `json:"pinned"`
	CategoryID *string         `json:"category_id,omitempty"`
	CreatedAt  string          `json:"created_at"`
	UpdatedAt  string          `json:"updated_at"`
}

func toNoteDTO(n domain.Note) noteDTO {
	return noteDTO{
		ID:         n.ID,
		Title:      n.Title,
		Body:       n.Body,
		Content:    n.Content,
		Pinned:     n.Pinned,
		CategoryID: n.CategoryID,
		CreatedAt:  n.CreatedAt.Format(time.RFC3339),
		UpdatedAt:  n.UpdatedAt.Format(time.RFC3339),
	}
}

// noteRequest serves both create and update: an update always sends the
// whole note, as the editor holds all of it.
type noteRequest struct {
	Title      string          `json:"title"`
	Body       string          `json:"body"`
	Content    json.RawMessage `json:"content,omitempty"`
	Pinned     bool            `json:"pinned,omitempty"`
	CategoryID *string         `json:"category_id,omitempty"`
}

func (r noteRequest) toDomain() domain.Note {
	return domain.Note{
		Title:      strings.TrimSpace(r.Title),
		Body:       r.Body,
		Content:    r.Content,
		Pinned:     r.Pinned,
		CategoryID: r.CategoryID,
	}
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
