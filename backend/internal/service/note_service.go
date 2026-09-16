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

func (s *NoteService) Create(ctx context.Context, n domain.Note) (domain.Note, error) {
	if err := n.Validate(); err != nil {
		return domain.Note{}, err
	}
	return s.notes.Create(ctx, n)
}

func (s *NoteService) Update(ctx context.Context, id string, n domain.Note) (domain.Note, error) {
	if err := n.Validate(); err != nil {
		return domain.Note{}, err
	}
	return s.notes.Update(ctx, id, n)
}

func (s *NoteService) Delete(ctx context.Context, id string) error {
	return s.notes.Delete(ctx, id)
}

type NoteCategoryService struct {
	categories *postgres.NoteCategoryRepo
}

func NewNoteCategoryService(repo *postgres.NoteCategoryRepo) *NoteCategoryService {
	return &NoteCategoryService{categories: repo}
}

func (s *NoteCategoryService) List(ctx context.Context) ([]domain.NoteCategory, error) {
	return s.categories.List(ctx)
}

func (s *NoteCategoryService) Create(ctx context.Context, name, color string) (domain.NoteCategory, error) {
	c := domain.NoteCategory{Name: name, Color: color}
	if err := c.Validate(); err != nil {
		return domain.NoteCategory{}, err
	}
	return s.categories.Create(ctx, c)
}

func (s *NoteCategoryService) Update(ctx context.Context, id, name, color string) (domain.NoteCategory, error) {
	c := domain.NoteCategory{Name: name, Color: color}
	if err := c.Validate(); err != nil {
		return domain.NoteCategory{}, err
	}
	return s.categories.Update(ctx, id, c)
}

func (s *NoteCategoryService) Delete(ctx context.Context, id string) error {
	return s.categories.Delete(ctx, id)
}
