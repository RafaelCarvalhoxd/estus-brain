package domain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// A scene can carry pasted images as data URLs, so it's allowed to be large
// — but not unbounded.
const (
	MaxBoardSceneBytes   = 20 << 20
	MaxBoardPreviewBytes = 600 << 10
)

// Board is one whiteboard. Scene is the editor's own JSON, kept opaque.
type Board struct {
	ID        string
	Name      string
	Scene     json.RawMessage
	Preview   string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func ValidateBoardName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("%w: board name is required", ErrValidation)
	}
	if len(name) > 120 {
		return fmt.Errorf("%w: board name is too long", ErrValidation)
	}
	return nil
}

// ValidateBoardScene checks only what the backend relies on: the scene is a
// JSON object of reasonable size. Its contents belong to the editor.
func ValidateBoardScene(scene []byte) error {
	if len(scene) > MaxBoardSceneBytes {
		return fmt.Errorf("%w: board is larger than %d MB", ErrValidation, MaxBoardSceneBytes>>20)
	}
	trimmed := bytes.TrimSpace(scene)
	if len(trimmed) == 0 || trimmed[0] != '{' || !json.Valid(trimmed) {
		return fmt.Errorf("%w: scene must be a JSON object", ErrValidation)
	}
	return nil
}

// ValidateBoardPreview keeps the thumbnail to an SVG document of bounded
// size; anything else is dropped rather than stored.
func ValidateBoardPreview(preview string) error {
	if preview == "" {
		return nil
	}
	if len(preview) > MaxBoardPreviewBytes {
		return fmt.Errorf("%w: preview is too large", ErrValidation)
	}
	if !strings.HasPrefix(strings.TrimSpace(preview), "<svg") {
		return fmt.Errorf("%w: preview must be an SVG", ErrValidation)
	}
	return nil
}
