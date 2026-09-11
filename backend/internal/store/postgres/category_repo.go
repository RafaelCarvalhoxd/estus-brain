package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/rafael/estus-vault/backend/internal/domain"
)

type CategoryRepo struct{ db *DB }

func NewCategoryRepo(db *DB) *CategoryRepo { return &CategoryRepo{db: db} }

func scanCategory(row scanner, c *domain.Category) error {
	var budget *int64
	if err := row.Scan(&c.ID, &c.Name, &c.Nature, &c.Color, &budget, &c.CreatedAt); err != nil {
		return err
	}
	if budget != nil {
		cents := domain.Cents(*budget)
		c.MonthlyBudgetCents = &cents
	}
	return nil
}

// scanner is the common subset of pgx.Row and pgx.Rows this package needs —
// letting scanCategory take either without pgx.Rows satisfying pgx.Row's
// exact interface out of the box.
type scanner interface {
	Scan(dest ...any) error
}

func (r *CategoryRepo) List(ctx context.Context) ([]domain.Category, error) {
	rows, err := r.db.Pool.Query(ctx, `
		select id, name, nature, color, monthly_budget_cents, created_at
		from categories
		order by name`)
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	defer rows.Close()

	var out []domain.Category
	for rows.Next() {
		var c domain.Category
		if err := scanCategory(rows, &c); err != nil {
			return nil, fmt.Errorf("scan category: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *CategoryRepo) Get(ctx context.Context, id string) (domain.Category, error) {
	var c domain.Category
	row := r.db.Pool.QueryRow(ctx, `
		select id, name, nature, color, monthly_budget_cents, created_at
		from categories where id = $1`, id,
	)
	if err := scanCategory(row, &c); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Category{}, domain.ErrNotFound
		}
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
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			return domain.Category{}, fmt.Errorf("%w: a category named %q already exists", domain.ErrConflict, c.Name)
		}
		return domain.Category{}, fmt.Errorf("create category: %w", err)
	}
	return c, nil
}

// Update changes name, nature and color — the fields set once at creation
// in every other module, but editable here because miscategorizing or
// renaming a category is a mistake worth being able to fix without
// recreating it (and losing its id, which transactions and bills point to).
func (r *CategoryRepo) Update(ctx context.Context, id string, c domain.Category) (domain.Category, error) {
	row := r.db.Pool.QueryRow(ctx, `
		update categories set name = $2, nature = $3, color = $4
		where id = $1
		returning id, name, nature, color, monthly_budget_cents, created_at`,
		id, c.Name, c.Nature, c.Color,
	)
	var out domain.Category
	if err := scanCategory(row, &out); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Category{}, domain.ErrNotFound
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			return domain.Category{}, fmt.Errorf("%w: a category named %q already exists", domain.ErrConflict, c.Name)
		}
		return domain.Category{}, fmt.Errorf("update category %s: %w", id, err)
	}
	return out, nil
}

// Delete refuses (with a clear error, not a raw FK violation) to remove a
// category still referenced by a transaction or bill — losing that link
// would silently corrupt the ledger it's attached to.
func (r *CategoryRepo) Delete(ctx context.Context, id string) error {
	tag, err := r.db.Pool.Exec(ctx, `delete from categories where id = $1`, id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgForeignKeyViolation {
			return fmt.Errorf("%w: category is used by existing transactions or bills", domain.ErrConflict)
		}
		return fmt.Errorf("delete category %s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// UpdateBudget sets or clears (budgetCents == nil) a category's monthly
// spending target. It's the only mutable field on a category today — name/
// color/nature are set once at creation and not exposed for editing yet.
func (r *CategoryRepo) UpdateBudget(ctx context.Context, id string, budgetCents *domain.Cents) (domain.Category, error) {
	var raw *int64
	if budgetCents != nil {
		v := int64(*budgetCents)
		raw = &v
	}
	var c domain.Category
	row := r.db.Pool.QueryRow(ctx, `
		update categories set monthly_budget_cents = $2
		where id = $1
		returning id, name, nature, color, monthly_budget_cents, created_at`,
		id, raw,
	)
	if err := scanCategory(row, &c); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Category{}, domain.ErrNotFound
		}
		return domain.Category{}, fmt.Errorf("update category budget %s: %w", id, err)
	}
	return c, nil
}
