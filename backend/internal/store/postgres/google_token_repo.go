package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/rafael/estus-vault/backend/internal/domain"
)

// GoogleToken is the persisted OAuth token for the single connected Google
// account this app supports.
type GoogleToken struct {
	ID           string
	AccessToken  string
	RefreshToken string
	Expiry       time.Time
}

type GoogleTokenRepo struct{ db *DB }

func NewGoogleTokenRepo(db *DB) *GoogleTokenRepo { return &GoogleTokenRepo{db: db} }

// Get returns the stored token, or domain.ErrNotFound when no account has
// been connected yet.
func (r *GoogleTokenRepo) Get(ctx context.Context) (GoogleToken, error) {
	var t GoogleToken
	err := r.db.Pool.QueryRow(ctx, `
		select id, access_token, refresh_token, expiry
		from google_oauth_tokens
		order by created_at desc
		limit 1`,
	).Scan(&t.ID, &t.AccessToken, &t.RefreshToken, &t.Expiry)
	if errors.Is(err, pgx.ErrNoRows) {
		return GoogleToken{}, domain.ErrNotFound
	}
	if err != nil {
		return GoogleToken{}, fmt.Errorf("get google token: %w", err)
	}
	return t, nil
}

// Save replaces the stored token with a single row: there's only ever one
// connected account, so a fresh connect or refresh clears whatever was
// there before.
func (r *GoogleTokenRepo) Save(ctx context.Context, t GoogleToken) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin save google token: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `delete from google_oauth_tokens`); err != nil {
		return fmt.Errorf("clear google token: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		insert into google_oauth_tokens (id, access_token, refresh_token, expiry)
		values (gen_random_uuid(), $1, $2, $3)`,
		t.AccessToken, t.RefreshToken, t.Expiry,
	); err != nil {
		return fmt.Errorf("insert google token: %w", err)
	}
	return tx.Commit(ctx)
}
