package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

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
