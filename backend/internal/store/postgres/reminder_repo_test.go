package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/rafael/estus-vault/backend/internal/domain"
)

func TestReminderRepo(t *testing.T) {
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

	repo := NewReminderRepo(db)

	due := time.Now().Add(24 * time.Hour).Truncate(time.Second)
	created, err := repo.Create(ctx, domain.Reminder{Title: "Pagar aluguel", DueAt: &due})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected generated id")
	}
	if created.Done {
		t.Error("expected new reminder to not be done")
	}
	defer repo.Delete(ctx, created.ID)

	withoutDue, err := repo.Create(ctx, domain.Reminder{Title: "Algum dia"})
	if err != nil {
		t.Fatalf("create without due date: %v", err)
	}
	defer repo.Delete(ctx, withoutDue.ID)

	list, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	found := 0
	for _, r := range list {
		if r.ID == created.ID || r.ID == withoutDue.ID {
			found++
		}
	}
	if found != 2 {
		t.Fatalf("expected both created reminders in list, found %d", found)
	}

	done, err := repo.SetDone(ctx, created.ID, true)
	if err != nil {
		t.Fatalf("set done: %v", err)
	}
	if !done.Done {
		t.Error("expected reminder to be marked done")
	}

	if _, err := repo.SetDone(ctx, "00000000-0000-0000-0000-000000000000", true); err != domain.ErrNotFound {
		t.Errorf("expected ErrNotFound for missing id, got %v", err)
	}

	if err := repo.Delete(ctx, created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := repo.Delete(ctx, created.ID); err != domain.ErrNotFound {
		t.Errorf("expected ErrNotFound deleting already-deleted reminder, got %v", err)
	}
}
