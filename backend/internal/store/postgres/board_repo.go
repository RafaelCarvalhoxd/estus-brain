package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/rafael/estus-vault/backend/internal/domain"
)

type BoardRepo struct{ db *DB }

func NewBoardRepo(db *DB) *BoardRepo { return &BoardRepo{db: db} }

// A board id comes straight from the URL; one that isn't even a UUID names
// no board, rather than being a server error.
func boardErr(op string, err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "22P02" {
		return domain.ErrNotFound
	}
	return fmt.Errorf("%s: %w", op, err)
}

// List returns every board without its scene — the list only needs names
// and thumbnails, and scenes can be megabytes each.
func (r *BoardRepo) List(ctx context.Context) ([]domain.Board, error) {
	rows, err := r.db.Pool.Query(ctx, `
		select id, name, preview, created_at, updated_at
		from boards order by updated_at desc`)
	if err != nil {
		return nil, fmt.Errorf("list boards: %w", err)
	}
	defer rows.Close()

	var out []domain.Board
	for rows.Next() {
		var b domain.Board
		if err := rows.Scan(&b.ID, &b.Name, &b.Preview, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan board: %w", err)
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (r *BoardRepo) Get(ctx context.Context, id string) (domain.Board, error) {
	var b domain.Board
	var scene []byte
	err := r.db.Pool.QueryRow(ctx, `
		select id, name, scene, preview, created_at, updated_at
		from boards where id = $1`, id,
	).Scan(&b.ID, &b.Name, &scene, &b.Preview, &b.CreatedAt, &b.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Board{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Board{}, boardErr("get board", err)
	}
	b.Scene = json.RawMessage(scene)
	return b, nil
}

func (r *BoardRepo) Create(ctx context.Context, name string) (domain.Board, error) {
	var id string
	if err := r.db.Pool.QueryRow(ctx, `insert into boards (name) values ($1) returning id`, name).Scan(&id); err != nil {
		return domain.Board{}, fmt.Errorf("create board: %w", err)
	}
	return r.Get(ctx, id)
}

func (r *BoardRepo) Rename(ctx context.Context, id, name string) error {
	tag, err := r.db.Pool.Exec(ctx, `update boards set name = $2, updated_at = now() where id = $1`, id, name)
	if err != nil {
		return boardErr("rename board", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *BoardRepo) SaveScene(ctx context.Context, id string, scene []byte, preview string) error {
	tag, err := r.db.Pool.Exec(ctx, `
		update boards set scene = $2::jsonb, preview = $3, updated_at = now()
		where id = $1`, id, string(scene), preview)
	if err != nil {
		return boardErr("save board scene", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *BoardRepo) Delete(ctx context.Context, id string) error {
	tag, err := r.db.Pool.Exec(ctx, `delete from boards where id = $1`, id)
	if err != nil {
		return boardErr("delete board", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}
