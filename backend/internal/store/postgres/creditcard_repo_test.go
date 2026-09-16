package postgres

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/rafael/estus-vault/backend/internal/domain"
)

func TestCreditCardRepoCRUD(t *testing.T) {
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
	if err := db.Migrate(ctx, "../../../migrations"); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repo := NewCreditCardRepo(db)

	created, err := repo.Create(ctx, domain.CreditCard{Name: "Cartão de teste", ClosingDay: 10, DueDay: 20})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// Apagado mesmo que o teste falhe no meio: é o banco real do dono.
	defer repo.Delete(ctx, created.ID)

	if created.ID == "" || created.Name != "Cartão de teste" || created.ClosingDay != 10 || created.DueDay != 20 {
		t.Fatalf("created = %+v", created)
	}
	if created.CreatedAt.IsZero() {
		t.Error("created_at não foi devolvido")
	}

	got, err := repo.Get(ctx, created.ID)
	if err != nil || got.Name != "Cartão de teste" {
		t.Fatalf("get = %+v, %v", got, err)
	}

	updated, err := repo.Update(ctx, created.ID, domain.CreditCard{Name: "Renomeado", ClosingDay: 5, DueDay: 25})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Name != "Renomeado" || updated.ClosingDay != 5 || updated.DueDay != 25 {
		t.Fatalf("updated = %+v", updated)
	}
	if updated.ID != created.ID {
		t.Errorf("update trocou o id: %s → %s", created.ID, updated.ID)
	}

	if err := repo.Delete(ctx, created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := repo.Get(ctx, created.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("get depois do delete = %v, want ErrNotFound", err)
	}
}

func TestCreditCardRepoMissingRows(t *testing.T) {
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
	repo := NewCreditCardRepo(db)

	const missing = "00000000-0000-0000-0000-000000000000"
	if err := repo.Delete(ctx, missing); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("delete inexistente = %v, want ErrNotFound", err)
	}
	if _, err := repo.Update(ctx, missing, domain.CreditCard{Name: "x", ClosingDay: 1, DueDay: 2}); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("update inexistente = %v, want ErrNotFound", err)
	}
}
