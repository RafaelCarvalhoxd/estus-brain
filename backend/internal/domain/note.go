package domain

import (
	"fmt"
	"strings"
	"time"
)

// Note is a freeform personal note. A note with neither title nor body says
// nothing, so at least one must be filled in.
type Note struct {
	ID        string
	Title     string
	Body      string
	Pinned    bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (n Note) Validate() error {
	if strings.TrimSpace(n.Title) == "" && strings.TrimSpace(n.Body) == "" {
		return fmt.Errorf("%w: title or body is required", ErrValidation)
	}
	return nil
}
