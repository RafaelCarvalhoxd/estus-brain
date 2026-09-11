package service

import (
	"context"

	"github.com/rafael/estus-vault/backend/internal/domain"
	"github.com/rafael/estus-vault/backend/internal/store/postgres"
)

type NoteService struct {
	notes *postgres.NoteRepo
}

func NewNoteService(repo *postgres.NoteRepo) *NoteService {
	return &NoteService{notes: repo}
}

func (s *NoteService) List(ctx context.Context) ([]domain.Note, error) {
	return s.notes.List(ctx)
}

func (s *NoteService) Get(ctx context.Context, id string) (domain.Note, error) {
	return s.notes.Get(ctx, id)
}

func (s *NoteService) Create(ctx context.Context, title, body string, pinned bool) (domain.Note, error) {
	n := domain.Note{Title: title, Body: body, Pinned: pinned}
	if err := n.Validate(); err != nil {
		return domain.Note{}, err
	}
	return s.notes.Create(ctx, n)
}

func (s *NoteService) Update(ctx context.Context, id, title, body string, pinned bool) (domain.Note, error) {
	n := domain.Note{Title: title, Body: body, Pinned: pinned}
	if err := n.Validate(); err != nil {
		return domain.Note{}, err
	}
	return s.notes.Update(ctx, id, title, body, pinned)
}

func (s *NoteService) Delete(ctx context.Context, id string) error {
	return s.notes.Delete(ctx, id)
}
