package postgres

import (
	"context"
	"fmt"
	"time"
)

// TelegramSettings is the single row that ties Estus Brain to one Telegram
// bot and one owner chat, plus what the bot sends on its own.
type TelegramSettings struct {
	TokenSecret         string // base64(nonce|ciphertext); "" = no stored token
	BotUsername         string
	ChatID              *int64 // the owner's chat; nil until paired
	OwnerName           string
	PairingCode         string
	PairingExpiresAt    *time.Time
	UpdateOffset        int64
	ConversationID      *string
	MorningEnabled      bool
	MorningTime         string // "HH:MM"
	EveningEnabled      bool
	EveningTime         string
	RemindersEnabled    bool
	EventsEnabled       bool
	EventsMinutesBefore int
	LastMorningOn       *time.Time
	LastEveningOn       *time.Time
	// LastOnlineAt is when the bot last polled successfully; nil until it
	// ever has. It is what the "conectado" notice measures the absence from.
	LastOnlineAt *time.Time
}

type TelegramRepo struct{ db *DB }

func NewTelegramRepo(db *DB) *TelegramRepo { return &TelegramRepo{db: db} }

func (r *TelegramRepo) Settings(ctx context.Context) (TelegramSettings, error) {
	var s TelegramSettings
	err := r.db.Pool.QueryRow(ctx, `
		select token_secret, bot_username, chat_id, owner_name, pairing_code, pairing_expires_at,
			update_offset, conversation_id, morning_enabled, morning_time, evening_enabled, evening_time,
			reminders_enabled, events_enabled, events_minutes_before, last_morning_on, last_evening_on,
			last_online_at
		from telegram_settings where id = 1`).Scan(
		&s.TokenSecret, &s.BotUsername, &s.ChatID, &s.OwnerName, &s.PairingCode, &s.PairingExpiresAt,
		&s.UpdateOffset, &s.ConversationID, &s.MorningEnabled, &s.MorningTime, &s.EveningEnabled, &s.EveningTime,
		&s.RemindersEnabled, &s.EventsEnabled, &s.EventsMinutesBefore, &s.LastMorningOn, &s.LastEveningOn,
		&s.LastOnlineAt)
	if err != nil {
		return TelegramSettings{}, fmt.Errorf("telegram settings: %w", err)
	}
	return s, nil
}

// SaveSettings writes the configuration and pairing columns. The update
// offset and the report days have their own setters, so the poller, the
// scheduler and a settings change never overwrite each other.
func (r *TelegramRepo) SaveSettings(ctx context.Context, s TelegramSettings) error {
	return r.saveSettings(ctx, s, false)
}

// SaveSettingsNewBot is SaveSettings for another bot (or none): update ids are
// per bot, so the offset goes back to 0 in the same statement, never left
// stale by a failed second write.
func (r *TelegramRepo) SaveSettingsNewBot(ctx context.Context, s TelegramSettings) error {
	return r.saveSettings(ctx, s, true)
}

func (r *TelegramRepo) saveSettings(ctx context.Context, s TelegramSettings, resetOffset bool) error {
	_, err := r.db.Pool.Exec(ctx, `
		update telegram_settings set
			token_secret = $1, bot_username = $2, chat_id = $3, owner_name = $4,
			pairing_code = $5, pairing_expires_at = $6, conversation_id = $7,
			morning_enabled = $8, morning_time = $9, evening_enabled = $10, evening_time = $11,
			reminders_enabled = $12, events_enabled = $13, events_minutes_before = $14,
			update_offset = case when $15 then 0 else update_offset end,
			updated_at = now()
		where id = 1`,
		s.TokenSecret, s.BotUsername, s.ChatID, s.OwnerName,
		s.PairingCode, s.PairingExpiresAt, s.ConversationID,
		s.MorningEnabled, s.MorningTime, s.EveningEnabled, s.EveningTime,
		s.RemindersEnabled, s.EventsEnabled, s.EventsMinutesBefore, resetOffset)
	if err != nil {
		return fmt.Errorf("save telegram settings: %w", err)
	}
	return nil
}

func (r *TelegramRepo) SetOffset(ctx context.Context, offset int64) error {
	if _, err := r.db.Pool.Exec(ctx, `update telegram_settings set update_offset = $1 where id = 1`, offset); err != nil {
		return fmt.Errorf("save telegram offset: %w", err)
	}
	return nil
}

// SetOnlineAt records that the bot was answering at t. Like the offset, it
// has its own setter so the poller never overwrites a settings change.
func (r *TelegramRepo) SetOnlineAt(ctx context.Context, t time.Time) error {
	if _, err := r.db.Pool.Exec(ctx, `update telegram_settings set last_online_at = $1 where id = 1`, t); err != nil {
		return fmt.Errorf("save telegram last online: %w", err)
	}
	return nil
}

// SetReportDay records the owner's day a report went out ("morning" or
// "evening"); nil clears it so the report is tried again.
func (r *TelegramRepo) SetReportDay(ctx context.Context, kind string, day *time.Time) error {
	column := map[string]string{"morning": "last_morning_on", "evening": "last_evening_on"}[kind]
	if column == "" {
		return fmt.Errorf("unknown telegram report %q", kind)
	}
	var value any
	if day != nil {
		value = day.Format("2006-01-02")
	}
	if _, err := r.db.Pool.Exec(ctx, `update telegram_settings set `+column+` = $1::date where id = 1`, value); err != nil {
		return fmt.Errorf("save telegram %s report day: %w", kind, err)
	}
	return nil
}

// MarkSent records an alert as sent and reports whether it wasn't already.
func (r *TelegramRepo) MarkSent(ctx context.Context, kind, ref string) (bool, error) {
	tag, err := r.db.Pool.Exec(ctx, `insert into telegram_sent (kind, ref) values ($1, $2) on conflict do nothing`, kind, ref)
	if err != nil {
		return false, fmt.Errorf("mark telegram alert sent: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

func (r *TelegramRepo) UnmarkSent(ctx context.Context, kind, ref string) error {
	if _, err := r.db.Pool.Exec(ctx, `delete from telegram_sent where kind = $1 and ref = $2`, kind, ref); err != nil {
		return fmt.Errorf("unmark telegram alert: %w", err)
	}
	return nil
}

func (r *TelegramRepo) PruneSent(ctx context.Context, before time.Time) error {
	if _, err := r.db.Pool.Exec(ctx, `delete from telegram_sent where sent_at < $1`, before); err != nil {
		return fmt.Errorf("prune telegram alerts: %w", err)
	}
	return nil
}
