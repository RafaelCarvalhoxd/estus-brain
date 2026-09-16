package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/rafael/estus-vault/backend/internal/domain"
)

type NoteRepo struct{ db *DB }

func NewNoteRepo(db *DB) *NoteRepo { return &NoteRepo{db: db} }

// A note id comes straight from the URL; one that isn't even a UUID names no
// note, rather than being a server error.
func noteErr(op string, err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "22P02" {
		return domain.ErrNotFound
	}
	return fmt.Errorf("%s: %w", op, err)
}

// jsonArg hands a document to Postgres as text (cast to jsonb in the query),
// or NULL when there is none.
func jsonArg(doc []byte) any {
	if len(doc) == 0 || string(doc) == "null" {
		return nil
	}
	return string(doc)
}

const noteColumns = `id, title, body, content, pinned, category_id, created_at, updated_at`

func scanNote(row scanner, n *domain.Note) error {
	var content []byte
	if err := row.Scan(&n.ID, &n.Title, &n.Body, &content, &n.Pinned, &n.CategoryID, &n.CreatedAt, &n.UpdatedAt); err != nil {
		return err
	}
	n.Content = content
	return nil
}

func (r *NoteRepo) Create(ctx context.Context, n domain.Note) (domain.Note, error) {
	row := r.db.Pool.QueryRow(ctx, `
		insert into notes (id, title, body, content, pinned, category_id)
		values (gen_random_uuid(), $1, $2, $3::jsonb, $4, $5)
		returning `+noteColumns,
		n.Title, n.Body, jsonArg(n.Content), n.Pinned, n.CategoryID,
	)
	var out domain.Note
	if err := scanNote(row, &out); err != nil {
		return domain.Note{}, fmt.Errorf("create note: %w", err)
	}
	return out, nil
}

// List leaves the documents out: the list shows titles and text, and a
// document with images can be megabytes.
func (r *NoteRepo) List(ctx context.Context) ([]domain.Note, error) {
	rows, err := r.db.Pool.Query(ctx, `
		select id, title, body, null::jsonb, pinned, category_id, created_at, updated_at
		from notes
		order by pinned desc, updated_at desc`)
	if err != nil {
		return nil, fmt.Errorf("list notes: %w", err)
	}
	defer rows.Close()

	var out []domain.Note
	for rows.Next() {
		var n domain.Note
		if err := scanNote(rows, &n); err != nil {
			return nil, fmt.Errorf("scan note: %w", err)
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (r *NoteRepo) Get(ctx context.Context, id string) (domain.Note, error) {
	var n domain.Note
	err := scanNote(r.db.Pool.QueryRow(ctx, `select `+noteColumns+` from notes where id = $1`, id), &n)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Note{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Note{}, noteErr("get note", err)
	}
	return n, nil
}

func (r *NoteRepo) Update(ctx context.Context, id string, n domain.Note) (domain.Note, error) {
	var out domain.Note
	err := scanNote(r.db.Pool.QueryRow(ctx, `
		update notes
		set title = $2, body = $3, content = $4::jsonb, pinned = $5, category_id = $6, updated_at = now()
		where id = $1
		returning `+noteColumns,
		id, n.Title, n.Body, jsonArg(n.Content), n.Pinned, n.CategoryID,
	), &out)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Note{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Note{}, noteErr("update note", err)
	}
	return out, nil
}

func (r *NoteRepo) Delete(ctx context.Context, id string) error {
	tag, err := r.db.Pool.Exec(ctx, `delete from notes where id = $1`, id)
	if err != nil {
		return noteErr("delete note", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}
