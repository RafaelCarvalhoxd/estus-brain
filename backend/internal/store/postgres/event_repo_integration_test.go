package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/rafael/estus-vault/backend/internal/domain"
)

func TestEventRepo_Integration(t *testing.T) {
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

	repo := NewEventRepo(db)

	starts := time.Date(2026, time.October, 10, 14, 0, 0, 0, time.UTC)
	ends := starts.Add(time.Hour)

	created, err := repo.Create(ctx, domain.Event{
		Title:    "Consulta médica",
		Location: "Clínica Central",
		StartsAt: starts,
		EndsAt:   ends,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer func() {
		db.Pool.Exec(ctx, `delete from events where id = $1`, created.ID)
	}()
	if created.ID == "" {
		t.Fatalf("expected an id to be assigned")
	}

	inRange, err := repo.ListRange(ctx, starts.Add(-time.Hour), ends.Add(time.Hour))
	if err != nil {
		t.Fatalf("list range: %v", err)
	}
	found := false
	for _, e := range inRange {
		if e.ID == created.ID {
			found = true
		}
	}
	if !found {
		t.Errorf("expected created event to appear in overlapping range")
	}

	outOfRange, err := repo.ListRange(ctx, starts.AddDate(1, 0, 0), ends.AddDate(1, 0, 1))
	if err != nil {
		t.Fatalf("list range (out of range): %v", err)
	}
	for _, e := range outOfRange {
		if e.ID == created.ID {
			t.Errorf("event should not appear a year outside its range")
		}
	}

	googleID := "google-event-" + created.ID
	syncedStart := starts.Add(24 * time.Hour)
	syncedEnd := syncedStart.Add(time.Hour)
	if err := repo.UpsertFromGoogle(ctx, googleID, domain.Event{
		Title:    "Reunião importada",
		StartsAt: syncedStart,
		EndsAt:   syncedEnd,
	}); err != nil {
		t.Fatalf("upsert from google (insert): %v", err)
	}
	defer func() {
		db.Pool.Exec(ctx, `delete from events where google_event_id = $1`, googleID)
	}()

	afterFirst, err := repo.ListRange(ctx, syncedStart.Add(-time.Minute), syncedEnd.Add(time.Minute))
	if err != nil {
		t.Fatalf("list range after first upsert: %v", err)
	}
	if len(afterFirst) != 1 {
		t.Fatalf("expected exactly 1 event after first upsert, got %d", len(afterFirst))
	}
	if afterFirst[0].Title != "Reunião importada" {
		t.Errorf("unexpected title after first upsert: %q", afterFirst[0].Title)
	}

	// Re-running the same upsert with a changed title must update the
	// existing row rather than insert a duplicate.
	if err := repo.UpsertFromGoogle(ctx, googleID, domain.Event{
		Title:    "Reunião importada (atualizada)",
		StartsAt: syncedStart,
		EndsAt:   syncedEnd,
	}); err != nil {
		t.Fatalf("upsert from google (update): %v", err)
	}

	afterSecond, err := repo.ListRange(ctx, syncedStart.Add(-time.Minute), syncedEnd.Add(time.Minute))
	if err != nil {
		t.Fatalf("list range after second upsert: %v", err)
	}
	if len(afterSecond) != 1 {
		t.Fatalf("expected upsert to update in place, got %d rows", len(afterSecond))
	}
	if afterSecond[0].Title != "Reunião importada (atualizada)" {
		t.Errorf("expected title to be updated, got %q", afterSecond[0].Title)
	}
	if afterSecond[0].GoogleEventID == nil || *afterSecond[0].GoogleEventID != googleID {
		t.Errorf("expected google_event_id to be set to %q", googleID)
	}

	if err := repo.Delete(ctx, created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := repo.Delete(ctx, created.ID); err != domain.ErrNotFound {
		t.Errorf("delete already-deleted event = %v, want domain.ErrNotFound", err)
	}
}
