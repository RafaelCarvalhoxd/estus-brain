package postgres

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestTelegramRepo(t *testing.T) {
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

	repo := NewTelegramRepo(db)
	original, err := repo.Settings(ctx)
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	// The row is the dev database's real one. With a real pairing in it, a
	// crash mid-test could leave the running bot pointed at a fake chat.
	if original.ChatID != nil {
		t.Skip("telegram_settings holds a real pairing, skipping so it isn't touched")
	}
	// Put the row back when done.
	defer func() {
		_ = repo.SaveSettings(ctx, original)
		_ = repo.SetOffset(ctx, original.UpdateOffset)
		_ = repo.SetReportDay(ctx, "morning", original.LastMorningOn)
		_ = repo.SetReportDay(ctx, "evening", original.LastEveningOn)
	}()

	chat := int64(12345)
	expires := time.Now().Add(10 * time.Minute).Truncate(time.Microsecond)
	s := original
	s.BotUsername = "estus_test_bot"
	s.ChatID = &chat
	s.OwnerName = "Rafa"
	s.PairingCode = "123456"
	s.PairingExpiresAt = &expires
	s.MorningTime = "06:30"
	s.EventsMinutesBefore = 15
	s.UpdateOffset = 777 // SaveSettings must ignore it
	if err := repo.SaveSettings(ctx, s); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := repo.Settings(ctx)
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if got.BotUsername != "estus_test_bot" || got.ChatID == nil || *got.ChatID != chat || got.OwnerName != "Rafa" ||
		got.PairingCode != "123456" || got.PairingExpiresAt == nil || !got.PairingExpiresAt.Equal(expires) ||
		got.MorningTime != "06:30" || got.EventsMinutesBefore != 15 {
		t.Fatalf("round trip = %+v", got)
	}
	if got.UpdateOffset != original.UpdateOffset {
		t.Fatalf("SaveSettings changed update_offset to %d", got.UpdateOffset)
	}

	if err := repo.SetOffset(ctx, 99); err != nil {
		t.Fatalf("set offset: %v", err)
	}
	day := time.Date(2026, time.September, 15, 0, 0, 0, 0, time.UTC)
	if err := repo.SetReportDay(ctx, "morning", &day); err != nil {
		t.Fatalf("set report day: %v", err)
	}
	got, _ = repo.Settings(ctx)
	if got.UpdateOffset != 99 || got.LastMorningOn == nil || got.LastMorningOn.Format("2006-01-02") != "2026-09-15" {
		t.Fatalf("offset/report day = %d %v", got.UpdateOffset, got.LastMorningOn)
	}
	if err := repo.SaveSettingsNewBot(ctx, got); err != nil {
		t.Fatalf("save new bot: %v", err)
	}
	if got, _ := repo.Settings(ctx); got.UpdateOffset != 0 || got.BotUsername != "estus_test_bot" {
		t.Fatalf("SaveSettingsNewBot left offset %d, bot %q", got.UpdateOffset, got.BotUsername)
	}
	if err := repo.SetReportDay(ctx, "morning", nil); err != nil {
		t.Fatalf("clear report day: %v", err)
	}
	if got, _ = repo.Settings(ctx); got.LastMorningOn != nil {
		t.Fatalf("report day not cleared: %v", got.LastMorningOn)
	}
	if err := repo.SetReportDay(ctx, "noon", nil); err == nil {
		t.Fatal("expected an error for an unknown report kind")
	}

	online := time.Now().Add(-time.Hour).UTC().Truncate(time.Microsecond)
	if err := repo.SetOnlineAt(ctx, online); err != nil {
		t.Fatalf("set online at: %v", err)
	}
	got, _ = repo.Settings(ctx)
	if got.LastOnlineAt == nil || !got.LastOnlineAt.Equal(online) {
		t.Fatalf("last online = %v, want %v", got.LastOnlineAt, online)
	}
	// It has its own setter so a settings change never rolls it back.
	if err := repo.SaveSettings(ctx, got); err != nil {
		t.Fatalf("save: %v", err)
	}
	if got, _ = repo.Settings(ctx); got.LastOnlineAt == nil || !got.LastOnlineAt.Equal(online) {
		t.Fatalf("SaveSettings changed last_online_at to %v", got.LastOnlineAt)
	}

	ref := "ref-" + time.Now().Format(time.RFC3339Nano)
	defer repo.UnmarkSent(ctx, "test", ref)
	if fresh, err := repo.MarkSent(ctx, "test", ref); err != nil || !fresh {
		t.Fatalf("first mark = %v, %v", fresh, err)
	}
	if fresh, err := repo.MarkSent(ctx, "test", ref); err != nil || fresh {
		t.Fatalf("second mark = %v, %v; want false", fresh, err)
	}
	if err := repo.UnmarkSent(ctx, "test", ref); err != nil {
		t.Fatalf("unmark: %v", err)
	}
	if fresh, _ := repo.MarkSent(ctx, "test", ref); !fresh {
		t.Fatal("mark after unmark should be fresh")
	}
	if err := repo.PruneSent(ctx, time.Now().Add(-30*24*time.Hour)); err != nil {
		t.Fatalf("prune: %v", err)
	}

	spent, err := NewTransactionRepo(db).SpentOn(ctx, time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil || spent != 0 {
		t.Fatalf("SpentOn on an empty day = %v, %v", spent, err)
	}
}
