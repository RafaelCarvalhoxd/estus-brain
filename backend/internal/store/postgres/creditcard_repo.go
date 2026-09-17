package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/rafael/estus-vault/backend/internal/domain"
)

type CreditCardRepo struct{ db *DB }

func NewCreditCardRepo(db *DB) *CreditCardRepo { return &CreditCardRepo{db: db} }

func (r *CreditCardRepo) List(ctx context.Context) ([]domain.CreditCard, error) {
	rows, err := r.db.Pool.Query(ctx, `
		select id, name, closing_day, due_day, created_at
		from credit_cards order by name`)
	if err != nil {
		return nil, fmt.Errorf("list credit cards: %w", err)
	}
	defer rows.Close()

	var out []domain.CreditCard
	for rows.Next() {
		var c domain.CreditCard
		if err := rows.Scan(&c.ID, &c.Name, &c.ClosingDay, &c.DueDay, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan credit card: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *CreditCardRepo) Get(ctx context.Context, id string) (domain.CreditCard, error) {
	var c domain.CreditCard
	err := r.db.Pool.QueryRow(ctx, `
		select id, name, closing_day, due_day, created_at
		from credit_cards where id = $1`, id,
	).Scan(&c.ID, &c.Name, &c.ClosingDay, &c.DueDay, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.CreditCard{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.CreditCard{}, fmt.Errorf("get credit card: %w", err)
	}
	return c, nil
}

func (r *CreditCardRepo) Create(ctx context.Context, c domain.CreditCard) (domain.CreditCard, error) {
	var out domain.CreditCard
	err := r.db.Pool.QueryRow(ctx, `
		insert into credit_cards (name, closing_day, due_day)
		values ($1, $2, $3)
		returning id, name, closing_day, due_day, created_at`,
		c.Name, c.ClosingDay, c.DueDay,
	).Scan(&out.ID, &out.Name, &out.ClosingDay, &out.DueDay, &out.CreatedAt)
	if err != nil {
		if isDuplicateName(err) {
			return domain.CreditCard{}, fmt.Errorf("%w: a credit card named %q already exists", domain.ErrConflict, c.Name)
		}
		return domain.CreditCard{}, fmt.Errorf("create credit card: %w", err)
	}
	return out, nil
}

func (r *CreditCardRepo) Update(ctx context.Context, id string, c domain.CreditCard) (domain.CreditCard, error) {
	var out domain.CreditCard
	err := r.db.Pool.QueryRow(ctx, `
		update credit_cards set name = $2, closing_day = $3, due_day = $4
		where id = $1
		returning id, name, closing_day, due_day, created_at`,
		id, c.Name, c.ClosingDay, c.DueDay,
	).Scan(&out.ID, &out.Name, &out.ClosingDay, &out.DueDay, &out.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.CreditCard{}, domain.ErrNotFound
	}
	if err != nil {
		if isDuplicateName(err) {
			return domain.CreditCard{}, fmt.Errorf("%w: a credit card named %q already exists", domain.ErrConflict, c.Name)
		}
		return domain.CreditCard{}, fmt.Errorf("update credit card %s: %w", id, err)
	}
	return out, nil
}

// isDuplicateName reports the unique violation on credit_cards.name, which
// 0018 added so the assistant can resolve a card by the name the owner says.
func isDuplicateName(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation
}

// Delete refuses a card that still has transactions. The foreign key already
// stops it; without translating the violation it would surface as a 500.
func (r *CreditCardRepo) Delete(ctx context.Context, id string) error {
	tag, err := r.db.Pool.Exec(ctx, `delete from credit_cards where id = $1`, id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgForeignKeyViolation {
			return fmt.Errorf("%w: credit card is used by existing transactions", domain.ErrConflict)
		}
		return fmt.Errorf("delete credit card %s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}
