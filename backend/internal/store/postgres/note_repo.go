package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/rafael/estus-vault/backend/internal/domain"
)

type NoteRepo struct{ db *DB }

func NewNoteRepo(db *DB) *NoteRepo { return &NoteRepo{db: db} }

func (r *NoteRepo) Create(ctx context.Context, n domain.Note) (domain.Note, error) {
	err := r.db.Pool.QueryRow(ctx, `
		insert into notes (id, title, body, pinned)
		values (gen_random_uuid(), $1, $2, $3)
		returning id, created_at, updated_at`,
		n.Title, n.Body, n.Pinned,
	).Scan(&n.ID, &n.CreatedAt, &n.UpdatedAt)
	if err != nil {
		return domain.Note{}, fmt.Errorf("create note: %w", err)
	}
	return n, nil
}

func (r *NoteRepo) List(ctx context.Context) ([]domain.Note, error) {
	rows, err := r.db.Pool.Query(ctx, `
		select id, title, body, pinned, created_at, updated_at
		from notes
		order by pinned desc, updated_at desc`)
	if err != nil {
		return nil, fmt.Errorf("list notes: %w", err)
	}
	defer rows.Close()

	var out []domain.Note
	for rows.Next() {
		var n domain.Note
		if err := rows.Scan(&n.ID, &n.Title, &n.Body, &n.Pinned, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan note: %w", err)
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (r *NoteRepo) Get(ctx context.Context, id string) (domain.Note, error) {
	var n domain.Note
	err := r.db.Pool.QueryRow(ctx, `
		select id, title, body, pinned, created_at, updated_at
		from notes where id = $1`, id,
	).Scan(&n.ID, &n.Title, &n.Body, &n.Pinned, &n.CreatedAt, &n.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Note{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Note{}, fmt.Errorf("get note: %w", err)
	}
	return n, nil
}

func (r *NoteRepo) Update(ctx context.Context, id, title, body string, pinned bool) (domain.Note, error) {
	var n domain.Note
	err := r.db.Pool.QueryRow(ctx, `
		update notes
		set title = $2, body = $3, pinned = $4, updated_at = now()
		where id = $1
		returning id, title, body, pinned, created_at, updated_at`,
		id, title, body, pinned,
	).Scan(&n.ID, &n.Title, &n.Body, &n.Pinned, &n.CreatedAt, &n.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Note{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Note{}, fmt.Errorf("update note: %w", err)
	}
	return n, nil
}

func (r *NoteRepo) Delete(ctx context.Context, id string) error {
	tag, err := r.db.Pool.Exec(ctx, `delete from notes where id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete note: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}
