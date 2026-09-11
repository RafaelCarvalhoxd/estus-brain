package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/rafael/estus-vault/backend/internal/domain"
)

type BillRepo struct{ db *DB }

func NewBillRepo(db *DB) *BillRepo { return &BillRepo{db: db} }

func (r *BillRepo) Create(ctx context.Context, b domain.Bill) (domain.Bill, error) {
	err := r.db.Pool.QueryRow(ctx, `
		insert into bills (id, description, amount_cents, due_date, direction, category_id, paid_at, recurring)
		values (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7)
		returning id, created_at`,
		b.Description, b.AmountCents, b.DueDate, b.Direction, b.CategoryID, b.PaidAt, b.Recurring,
	).Scan(&b.ID, &b.CreatedAt)
	if err != nil {
		return domain.Bill{}, fmt.Errorf("create bill: %w", err)
	}
	return b, nil
}

func (r *BillRepo) List(ctx context.Context, direction *domain.BillDirection, onlyOpen bool) ([]domain.Bill, error) {
	query := `
		select id, description, amount_cents, due_date, direction, category_id, paid_at, recurring, created_at
		from bills
		where ($1::text is null or direction = $1)
		and (not $2 or paid_at is null)
		order by due_date asc`

	var dirArg *string
	if direction != nil {
		s := string(*direction)
		dirArg = &s
	}

	rows, err := r.db.Pool.Query(ctx, query, dirArg, onlyOpen)
	if err != nil {
		return nil, fmt.Errorf("list bills: %w", err)
	}
	defer rows.Close()

	var out []domain.Bill
	for rows.Next() {
		var b domain.Bill
		if err := rows.Scan(&b.ID, &b.Description, &b.AmountCents, &b.DueDate, &b.Direction, &b.CategoryID, &b.PaidAt, &b.Recurring, &b.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan bill: %w", err)
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (r *BillRepo) MarkPaid(ctx context.Context, id string, paidAt time.Time) (domain.Bill, error) {
	var b domain.Bill
	err := r.db.Pool.QueryRow(ctx, `
		update bills set paid_at = $2
		where id = $1
		returning id, description, amount_cents, due_date, direction, category_id, paid_at, recurring, created_at`,
		id, paidAt,
	).Scan(&b.ID, &b.Description, &b.AmountCents, &b.DueDate, &b.Direction, &b.CategoryID, &b.PaidAt, &b.Recurring, &b.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Bill{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Bill{}, fmt.Errorf("mark bill paid: %w", err)
	}
	return b, nil
}

func (r *BillRepo) OpenTotals(ctx context.Context) (payableCents, receivableCents domain.Cents, overdueCount int, err error) {
	rows, err := r.db.Pool.Query(ctx, `
		select direction, coalesce(sum(amount_cents), 0)
		from bills
		where paid_at is null
		group by direction`)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("open totals: %w", err)
	}
	for rows.Next() {
		var direction string
		var total int64
		if err := rows.Scan(&direction, &total); err != nil {
			rows.Close()
			return 0, 0, 0, fmt.Errorf("scan open totals: %w", err)
		}
		switch domain.BillDirection(direction) {
		case domain.BillPayable:
			payableCents = domain.Cents(total)
		case domain.BillReceivable:
			receivableCents = domain.Cents(total)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, 0, 0, fmt.Errorf("open totals: %w", err)
	}
	rows.Close()

	err = r.db.Pool.QueryRow(ctx, `
		select count(*) from bills
		where paid_at is null and due_date < current_date`,
	).Scan(&overdueCount)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("overdue count: %w", err)
	}
	return payableCents, receivableCents, overdueCount, nil
}
