package httpapi

import (
	"time"

	"github.com/rafael/estus-vault/backend/internal/domain"
)

type documentFolderDTO struct {
	ID        string  `json:"id"`
	ParentID  *string `json:"parent_id,omitempty"`
	Name      string  `json:"name"`
	CreatedAt string  `json:"created_at"`
}

func toDocumentFolderDTO(f domain.DocumentFolder) documentFolderDTO {
	return documentFolderDTO{
		ID:        f.ID,
		ParentID:  f.ParentID,
		Name:      f.Name,
		CreatedAt: f.CreatedAt.Format(time.RFC3339),
	}
}

// The stored file name never leaves the server: it is an implementation
// detail of where the bytes sit, and exposing it would invite callers to
// build paths out of it.
type documentDTO struct {
	ID          string  `json:"id"`
	FolderID    *string `json:"folder_id,omitempty"`
	Name        string  `json:"name"`
	ContentType string  `json:"content_type"`
	SizeBytes   int64   `json:"size_bytes"`
	CreatedAt   string  `json:"created_at"`
}

func toDocumentDTO(d domain.Document) documentDTO {
	return documentDTO{
		ID:          d.ID,
		FolderID:    d.FolderID,
		Name:        d.Name,
		ContentType: d.ContentType,
		SizeBytes:   d.SizeBytes,
		CreatedAt:   d.CreatedAt.Format(time.RFC3339),
	}
}

type documentFolderRequest struct {
	ParentID *string `json:"parent_id,omitempty"`
	Name     string  `json:"name"`
}

type moveDocumentRequest struct {
	FolderID *string `json:"folder_id,omitempty"`
	Name     string  `json:"name"`
}
