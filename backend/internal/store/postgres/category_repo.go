package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/rafael/estus-vault/backend/internal/domain"
)

type CategoryRepo struct{ db *DB }

func NewCategoryRepo(db *DB) *CategoryRepo { return &CategoryRepo{db: db} }

func (r *CategoryRepo) List(ctx context.Context) ([]domain.Category, error) {
	rows, err := r.db.Pool.Query(ctx, `
		select id, name, nature, color, created_at
		from categories
		order by name`)
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	defer rows.Close()

	var out []domain.Category
	for rows.Next() {
		var c domain.Category
		if err := rows.Scan(&c.ID, &c.Name, &c.Nature, &c.Color, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan category: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *CategoryRepo) Get(ctx context.Context, id string) (domain.Category, error) {
	var c domain.Category
	err := r.db.Pool.QueryRow(ctx, `
		select id, name, nature, color, created_at
		from categories where id = $1`, id,
	).Scan(&c.ID, &c.Name, &c.Nature, &c.Color, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Category{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Category{}, fmt.Errorf("get category: %w", err)
	}
	return c, nil
}

func (r *CategoryRepo) Create(ctx context.Context, c domain.Category) (domain.Category, error) {
	err := r.db.Pool.QueryRow(ctx, `
		insert into categories (id, name, nature, color)
		values (gen_random_uuid(), $1, $2, $3)
		returning id, created_at`,
		c.Name, c.Nature, c.Color,
	).Scan(&c.ID, &c.CreatedAt)
	if err != nil {
		return domain.Category{}, fmt.Errorf("create category: %w", err)
	}
	return c, nil
}
