package service

import (
	"context"
	"strings"

	"github.com/rafael/estus-vault/backend/internal/domain"
	"github.com/rafael/estus-vault/backend/internal/store/postgres"
)

type BoardService struct {
	boards *postgres.BoardRepo
}

func NewBoardService(repo *postgres.BoardRepo) *BoardService {
	return &BoardService{boards: repo}
}

func (s *BoardService) List(ctx context.Context) ([]domain.Board, error) {
	return s.boards.List(ctx)
}

func (s *BoardService) Get(ctx context.Context, id string) (domain.Board, error) {
	return s.boards.Get(ctx, id)
}

func (s *BoardService) Create(ctx context.Context, name string) (domain.Board, error) {
	if err := domain.ValidateBoardName(name); err != nil {
		return domain.Board{}, err
	}
	return s.boards.Create(ctx, strings.TrimSpace(name))
}

func (s *BoardService) Rename(ctx context.Context, id, name string) error {
	if err := domain.ValidateBoardName(name); err != nil {
		return err
	}
	return s.boards.Rename(ctx, id, strings.TrimSpace(name))
}

// SaveScene stores the scene; a thumbnail that doesn't pass is dropped
// rather than failing the save — losing work over a preview would be wrong.
func (s *BoardService) SaveScene(ctx context.Context, id string, scene []byte, preview string) error {
	if err := domain.ValidateBoardScene(scene); err != nil {
		return err
	}
	if domain.ValidateBoardPreview(preview) != nil {
		preview = ""
	}
	return s.boards.SaveScene(ctx, id, scene, preview)
}

func (s *BoardService) Delete(ctx context.Context, id string) error {
	return s.boards.Delete(ctx, id)
}
