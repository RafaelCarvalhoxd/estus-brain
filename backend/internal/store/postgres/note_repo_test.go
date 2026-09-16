package postgres

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/rafael/estus-vault/backend/internal/domain"
)

func TestNoteRepo(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	db, err := Connect(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer db.Close()

	repo := NewNoteRepo(db)

	created, err := repo.Create(ctx, domain.Note{Title: "Ideia", Body: "escrever mais testes"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer repo.Delete(ctx, created.ID)

	if created.ID == "" {
		t.Fatal("expected id to be set")
	}
	if created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
		t.Fatal("expected timestamps to be set")
	}

	list, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	found := false
	for _, n := range list {
		if n.ID == created.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("expected created note to appear in list")
	}

	got, err := repo.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Title != "Ideia" {
		t.Fatalf("expected title Ideia, got %q", got.Title)
	}

	doc := []byte(`{"type": "doc", "content": [{"type": "paragraph"}]}`)
	updated, err := repo.Update(ctx, created.ID, domain.Note{Title: "Ideia revisada", Body: "novo corpo", Content: doc, Pinned: true})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if !updated.Pinned || updated.Title != "Ideia revisada" {
		t.Fatalf("update did not apply: %+v", updated)
	}
	if len(updated.Content) == 0 {
		t.Fatal("expected content to be stored")
	}
	withDoc, err := repo.Get(ctx, created.ID)
	if err != nil || !strings.Contains(string(withDoc.Content), `"paragraph"`) {
		t.Fatalf("expected content back from get, got %q (%v)", withDoc.Content, err)
	}
	if !updated.UpdatedAt.After(created.UpdatedAt) && updated.UpdatedAt != created.UpdatedAt {
		t.Fatalf("expected updated_at to advance")
	}

	pinnedList, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("list after pin: %v", err)
	}
	if len(pinnedList) == 0 || pinnedList[0].ID != created.ID {
		t.Fatal("expected pinned note to be first in list")
	}

	if err := repo.Delete(ctx, created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err = repo.Get(ctx, created.ID)
	if err != domain.ErrNotFound {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}
