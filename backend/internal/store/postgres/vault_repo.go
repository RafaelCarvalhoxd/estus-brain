package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/rafael/estus-vault/backend/internal/domain"
)

type VaultRepo struct{ db *DB }

func NewVaultRepo(db *DB) *VaultRepo { return &VaultRepo{db: db} }

func (r *VaultRepo) Create(ctx context.Context, e domain.VaultEntry) (domain.VaultEntry, error) {
	err := r.db.Pool.QueryRow(ctx, `
		insert into vault_entries (id, title, username, url, notes, password_ciphertext, password_nonce)
		values (gen_random_uuid(), $1, $2, $3, $4, $5, $6)
		returning id, created_at, updated_at`,
		e.Title, e.Username, e.URL, e.Notes, e.PasswordCiphertext, e.PasswordNonce,
	).Scan(&e.ID, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		return domain.VaultEntry{}, fmt.Errorf("create vault entry: %w", err)
	}
	return e, nil
}

// List returns metadata only — no ciphertext column in the query at all,
// so a bug elsewhere that logs or serializes a List result can never leak
// even encrypted password bytes.
func (r *VaultRepo) List(ctx context.Context) ([]domain.VaultEntry, error) {
	rows, err := r.db.Pool.Query(ctx, `
		select id, title, username, url, updated_at
		from vault_entries
		order by title`)
	if err != nil {
		return nil, fmt.Errorf("list vault entries: %w", err)
	}
	defer rows.Close()

	var out []domain.VaultEntry
	for rows.Next() {
		var e domain.VaultEntry
		if err := rows.Scan(&e.ID, &e.Title, &e.Username, &e.URL, &e.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan vault entry: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// Get includes the ciphertext and nonce — only the reveal path should call
// this.
func (r *VaultRepo) Get(ctx context.Context, id string) (domain.VaultEntry, error) {
	var e domain.VaultEntry
	err := r.db.Pool.QueryRow(ctx, `
		select id, title, username, url, notes, password_ciphertext, password_nonce, created_at, updated_at
		from vault_entries where id = $1`, id,
	).Scan(&e.ID, &e.Title, &e.Username, &e.URL, &e.Notes, &e.PasswordCiphertext, &e.PasswordNonce, &e.CreatedAt, &e.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.VaultEntry{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.VaultEntry{}, fmt.Errorf("get vault entry: %w", err)
	}
	return e, nil
}

func (r *VaultRepo) Update(ctx context.Context, e domain.VaultEntry) (domain.VaultEntry, error) {
	err := r.db.Pool.QueryRow(ctx, `
		update vault_entries
		set title = $1, username = $2, url = $3, notes = $4,
		    password_ciphertext = $5, password_nonce = $6, updated_at = now()
		where id = $7
		returning updated_at`,
		e.Title, e.Username, e.URL, e.Notes, e.PasswordCiphertext, e.PasswordNonce, e.ID,
	).Scan(&e.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.VaultEntry{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.VaultEntry{}, fmt.Errorf("update vault entry: %w", err)
	}
	return e, nil
}

func (r *VaultRepo) Delete(ctx context.Context, id string) error {
	tag, err := r.db.Pool.Exec(ctx, `delete from vault_entries where id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete vault entry: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}
