package domain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// A note can hold pasted images, so its document may be large — within reason.
const (
	MaxNoteContentBytes = 10 << 20
	MaxNoteTitleChars   = 300
)

// Note is a personal note written in the editor. Content is the editor's own
// JSON document, kept opaque; Body is its plain-text copy, used for search and
// previews. Notes written before the editor existed have Body only.
// CategoryID nil means "Geral" — a virtual bucket the frontend groups
// uncategorized notes under, not a real category row that has to exist.
type Note struct {
	ID         string
	Title      string
	Body       string
	Content    json.RawMessage
	Pinned     bool
	CategoryID *string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Validate allows an empty note: one is emptied mid-edit all the time, and
// the editor only creates a note once something has been written.
func (n Note) Validate() error {
	if len([]rune(strings.TrimSpace(n.Title))) > MaxNoteTitleChars {
		return fmt.Errorf("%w: title is too long", ErrValidation)
	}
	if len(n.Content) > MaxNoteContentBytes {
		return fmt.Errorf("%w: note is larger than %d MB", ErrValidation, MaxNoteContentBytes>>20)
	}
	if len(n.Content) > 0 {
		trimmed := bytes.TrimSpace(n.Content)
		if bytes.Equal(trimmed, []byte("null")) {
			return nil
		}
		if len(trimmed) == 0 || trimmed[0] != '{' || !json.Valid(trimmed) {
			return fmt.Errorf("%w: content must be a JSON object", ErrValidation)
		}
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
