package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/rafael/estus-vault/backend/internal/domain"
)

type NoteCategoryRepo struct{ db *DB }

func NewNoteCategoryRepo(db *DB) *NoteCategoryRepo { return &NoteCategoryRepo{db: db} }

func scanNoteCategory(row scanner, c *domain.NoteCategory) error {
	return row.Scan(&c.ID, &c.Name, &c.Color, &c.CreatedAt)
}

func (r *NoteCategoryRepo) List(ctx context.Context) ([]domain.NoteCategory, error) {
	rows, err := r.db.Pool.Query(ctx, `select id, name, color, created_at from note_categories order by name`)
	if err != nil {
		return nil, fmt.Errorf("list note categories: %w", err)
	}
	defer rows.Close()

	var out []domain.NoteCategory
	for rows.Next() {
		var c domain.NoteCategory
		if err := scanNoteCategory(rows, &c); err != nil {
			return nil, fmt.Errorf("scan note category: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *NoteCategoryRepo) Create(ctx context.Context, c domain.NoteCategory) (domain.NoteCategory, error) {
	row := r.db.Pool.QueryRow(ctx, `
		insert into note_categories (id, name, color)
		values (gen_random_uuid(), $1, $2)
		returning id, name, color, created_at`,
		c.Name, c.Color,
	)
	var out domain.NoteCategory
	if err := scanNoteCategory(row, &out); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			return domain.NoteCategory{}, fmt.Errorf("%w: a category named %q already exists", domain.ErrConflict, c.Name)
		}
		return domain.NoteCategory{}, fmt.Errorf("create note category: %w", err)
	}
	return out, nil
}

func (r *NoteCategoryRepo) Update(ctx context.Context, id string, c domain.NoteCategory) (domain.NoteCategory, error) {
	row := r.db.Pool.QueryRow(ctx, `
		update note_categories set name = $2, color = $3
		where id = $1
		returning id, name, color, created_at`,
		id, c.Name, c.Color,
	)
	var out domain.NoteCategory
	if err := scanNoteCategory(row, &out); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.NoteCategory{}, domain.ErrNotFound
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			return domain.NoteCategory{}, fmt.Errorf("%w: a category named %q already exists", domain.ErrConflict, c.Name)
		}
		return domain.NoteCategory{}, fmt.Errorf("update note category %s: %w", id, err)
	}
	return out, nil
}

// Delete removes a note category and, since a note's category_id is a
// plain nullable FK with no cascade, first unlinks every note pointing to
// it — deleting a category should demote its notes to "Geral", not fail
// with a foreign key error or silently orphan them.
func (r *NoteCategoryRepo) Delete(ctx context.Context, id string) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin delete note category %s: %w", id, err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `update notes set category_id = null where category_id = $1`, id); err != nil {
		return fmt.Errorf("unlink notes from category %s: %w", id, err)
	}
	tag, err := tx.Exec(ctx, `delete from note_categories where id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete note category %s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return tx.Commit(ctx)
}
