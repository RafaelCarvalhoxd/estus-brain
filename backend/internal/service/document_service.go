package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/google/uuid"

	"github.com/rafael/estus-vault/backend/internal/domain"
	"github.com/rafael/estus-vault/backend/internal/store/postgres"
)

// DocumentService owns both halves of a stored document: the row in
// Postgres and the file on disk. Nothing else in the process touches the
// storage directory, so the two can never drift apart without going through
// here — an upload that fails to record itself deletes its own file, and a
// deleted row takes its file with it.
type DocumentService struct {
	folders *postgres.DocumentFolderRepo
	docs    *postgres.DocumentRepo
	root    string
}

func NewDocumentService(folders *postgres.DocumentFolderRepo, docs *postgres.DocumentRepo, root string) (*DocumentService, error) {
	if root == "" {
		return nil, fmt.Errorf("documents dir is required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve documents dir: %w", err)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, fmt.Errorf("create documents dir %s: %w", abs, err)
	}
	return &DocumentService{folders: folders, docs: docs, root: abs}, nil
}

// Root is where the files live, reported once at boot so the operator can
// see which directory is being filled.
func (s *DocumentService) Root() string { return s.root }

func (s *DocumentService) ListFolders(ctx context.Context) ([]domain.DocumentFolder, error) {
	return s.folders.List(ctx)
}

func (s *DocumentService) CreateFolder(ctx context.Context, parentID *string, name string) (domain.DocumentFolder, error) {
	f := domain.DocumentFolder{ParentID: parentID, Name: name}
	if err := f.Validate(); err != nil {
		return domain.DocumentFolder{}, err
	}
	if parentID != nil {
		if _, err := s.folders.Get(ctx, *parentID); err != nil {
			return domain.DocumentFolder{}, err
		}
	}
	return s.folders.Create(ctx, f)
}

func (s *DocumentService) RenameFolder(ctx context.Context, id, name string) (domain.DocumentFolder, error) {
	f := domain.DocumentFolder{Name: name}
	if err := f.Validate(); err != nil {
		return domain.DocumentFolder{}, err
	}
	return s.folders.Rename(ctx, id, name)
}

// DeleteFolder refuses to delete a folder that still holds anything. The
// alternative — cascading — would delete files on disk as a side effect of
// a single click, which is the wrong default for an archive.
func (s *DocumentService) DeleteFolder(ctx context.Context, id string) error {
	subfolders, docs, err := s.folders.Counts(ctx, id)
	if err != nil {
		return err
	}
	if subfolders > 0 || docs > 0 {
		return fmt.Errorf("%w: folder is not empty", domain.ErrConflict)
	}
	return s.folders.Delete(ctx, id)
}

func (s *DocumentService) List(ctx context.Context, folderID *string) ([]domain.Document, error) {
	return s.docs.List(ctx, folderID)
}

func (s *DocumentService) Count(ctx context.Context) (int, error) {
	return s.docs.Count(ctx)
}

func (s *DocumentService) Get(ctx context.Context, id string) (domain.Document, error) {
	return s.docs.Get(ctx, id)
}

// Save streams an upload to disk and then records it. Size is checked while
// copying rather than trusting a declared length, and the half-written file
// is removed if anything goes wrong.
func (s *DocumentService) Save(ctx context.Context, folderID *string, name, contentType string, src io.Reader) (domain.Document, error) {
	if folderID != nil {
		if _, err := s.folders.Get(ctx, *folderID); err != nil {
			return domain.Document{}, err
		}
	}

	safe := domain.SafeFileName(name)
	stored := uuid.NewString() + "__" + safe
	path := filepath.Join(s.root, stored)

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return domain.Document{}, fmt.Errorf("create document file: %w", err)
	}

	// One byte past the limit is enough to know it's too big.
	written, copyErr := io.Copy(file, io.LimitReader(src, domain.MaxDocumentBytes+1))
	closeErr := file.Close()
	if copyErr != nil || closeErr != nil {
		os.Remove(path)
		return domain.Document{}, fmt.Errorf("write document file: %w", errors.Join(copyErr, closeErr))
	}

	if contentType == "" {
		contentType = "application/octet-stream"
	}
	doc := domain.Document{
		FolderID:    folderID,
		Name:        safe,
		StoredName:  stored,
		ContentType: contentType,
		SizeBytes:   written,
	}
	if err := doc.Validate(); err != nil {
		os.Remove(path)
		return domain.Document{}, err
	}

	saved, err := s.docs.Create(ctx, doc)
	if err != nil {
		os.Remove(path)
		return domain.Document{}, err
	}
	return saved, nil
}

func (s *DocumentService) Move(ctx context.Context, id string, folderID *string, name string) (domain.Document, error) {
	if folderID != nil {
		if _, err := s.folders.Get(ctx, *folderID); err != nil {
			return domain.Document{}, err
		}
	}
	safe := domain.SafeFileName(name)
	if safe == "" {
		return domain.Document{}, fmt.Errorf("%w: file name is required", domain.ErrValidation)
	}
	return s.docs.Move(ctx, id, folderID, safe)
}

// Open hands back the document's metadata and an open file for streaming.
// The caller closes the file.
func (s *DocumentService) Open(ctx context.Context, id string) (domain.Document, *os.File, error) {
	doc, err := s.docs.Get(ctx, id)
	if err != nil {
		return domain.Document{}, nil, err
	}
	// StoredName was generated here, never supplied by a caller, but it is
	// checked anyway: a path that isn't a plain name inside the storage
	// directory is a bug worth failing loudly on.
	if filepath.Base(doc.StoredName) != doc.StoredName {
		return domain.Document{}, nil, fmt.Errorf("invalid stored name for document %s", id)
	}
	file, err := os.Open(filepath.Join(s.root, doc.StoredName))
	if err != nil {
		if os.IsNotExist(err) {
			return domain.Document{}, nil, fmt.Errorf("%w: file is missing from storage", domain.ErrNotFound)
		}
		return domain.Document{}, nil, fmt.Errorf("open document file: %w", err)
	}
	return doc, file, nil
}

// Delete removes the row first: a file left behind is recoverable, a row
// pointing at nothing is not.
func (s *DocumentService) Delete(ctx context.Context, id string) error {
	doc, err := s.docs.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := s.docs.Delete(ctx, id); err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(s.root, doc.StoredName)); err != nil && !os.IsNotExist(err) {
		slog.Error("document row deleted but file remains", "stored_name", doc.StoredName, "error", err)
	}
	return nil
}
