package domain

import (
	"fmt"
	"strings"
	"time"
)

// Note is a freeform personal note. A note with neither title nor body says
// nothing, so at least one must be filled in. CategoryID nil means "Geral"
// — a virtual bucket the frontend groups uncategorized notes under, not a
// real category row that has to exist.
type Note struct {
	ID         string
	Title      string
	Body       string
	Pinned     bool
	CategoryID *string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (n Note) Validate() error {
	if strings.TrimSpace(n.Title) == "" && strings.TrimSpace(n.Body) == "" {
		return fmt.Errorf("%w: title or body is required", ErrValidation)
	}
	return nil
}

// NoteCategory groups notes for browsing — "Trabalho", "Pessoal", "Ideias"
// — independent of the finance categories.
type NoteCategory struct {
	ID        string
	Name      string
	Color     string
	CreatedAt time.Time
}

func (c NoteCategory) Validate() error {
	if strings.TrimSpace(c.Name) == "" {
		return fmt.Errorf("%w: name is required", ErrValidation)
	}
	if strings.TrimSpace(c.Color) == "" {
		return fmt.Errorf("%w: color is required", ErrValidation)
	}
	return nil
}
