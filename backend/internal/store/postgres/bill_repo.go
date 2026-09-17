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
		insert into bills (
			id, description, amount_cents, due_date, direction, category_id, paid_at,
			series_id, amount_estimated, payment_method, transaction_id
		)
		values (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		returning id, created_at`,
		b.Description, b.AmountCents, b.DueDate, b.Direction, b.CategoryID, b.PaidAt,
		b.SeriesID, b.AmountEstimated, b.PaymentMethod, b.TransactionID,
	).Scan(&b.ID, &b.CreatedAt)
	if err != nil {
		return domain.Bill{}, fmt.Errorf("create bill: %w", err)
	}
	return b, nil
}

// Get fetches a single bill by id, so callers that need one occurrence (for
// example to settle it) don't have to filter a full List.
func (r *BillRepo) Get(ctx context.Context, id string) (domain.Bill, error) {
	var b domain.Bill
	err := r.db.Pool.QueryRow(ctx, `
		select id, description, amount_cents, due_date, direction, category_id, paid_at,
			series_id, amount_estimated, payment_method, transaction_id, created_at
		from bills where id = $1`, id,
	).Scan(&b.ID, &b.Description, &b.AmountCents, &b.DueDate, &b.Direction, &b.CategoryID, &b.PaidAt,
		&b.SeriesID, &b.AmountEstimated, &b.PaymentMethod, &b.TransactionID, &b.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Bill{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Bill{}, fmt.Errorf("get bill %s: %w", id, err)
	}
	return b, nil
}

func (r *BillRepo) List(ctx context.Context, direction *domain.BillDirection, onlyOpen bool) ([]domain.Bill, error) {
	query := `
		select id, description, amount_cents, due_date, direction, category_id, paid_at,
			series_id, amount_estimated, payment_method, transaction_id, created_at
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
	return scanBills(rows)
}

// ListByMonth is every bill due in ym, which is how the Contas screen is
// read now that it has month navigation.
func (r *BillRepo) ListByMonth(ctx context.Context, ym domain.YearMonth, direction *domain.BillDirection) ([]domain.Bill, error) {
	start := ym.FirstDay()
	end := start.AddDate(0, 1, 0)

	var dirArg *string
	if direction != nil {
		s := string(*direction)
		dirArg = &s
	}

	rows, err := r.db.Pool.Query(ctx, `
		select id, description, amount_cents, due_date, direction, category_id,
			paid_at, series_id, amount_estimated, payment_method, transaction_id, created_at
		from bills
		where due_date >= $1 and due_date < $2
			and ($3::text is null or direction = $3)
		order by due_date, description`, start, end, dirArg)
	if err != nil {
		return nil, fmt.Errorf("list bills for %v: %w", ym, err)
	}
	defer rows.Close()
	return scanBills(rows)
}

// LatestPerSeries is the most recent occurrence of every series — the mould
// each series' next month is copied from.
func (r *BillRepo) LatestPerSeries(ctx context.Context) ([]domain.Bill, error) {
	rows, err := r.db.Pool.Query(ctx, `
		select distinct on (series_id)
			id, description, amount_cents, due_date, direction, category_id,
			paid_at, series_id, amount_estimated, payment_method, transaction_id, created_at
		from bills
		where series_id is not null
		order by series_id, due_date desc`)
	if err != nil {
		return nil, fmt.Errorf("latest bill per series: %w", err)
	}
	defer rows.Close()
	return scanBills(rows)
}

// scanBills reads every row into a domain.Bill, using the same column order
// List, ListByMonth and LatestPerSeries all select in.
func scanBills(rows pgx.Rows) ([]domain.Bill, error) {
	var out []domain.Bill
	for rows.Next() {
		var b domain.Bill
		if err := rows.Scan(&b.ID, &b.Description, &b.AmountCents, &b.DueDate, &b.Direction, &b.CategoryID, &b.PaidAt,
			&b.SeriesID, &b.AmountEstimated, &b.PaymentMethod, &b.TransactionID, &b.CreatedAt); err != nil {
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
		returning id, description, amount_cents, due_date, direction, category_id, paid_at,
			series_id, amount_estimated, payment_method, transaction_id, created_at`,
		id, paidAt,
	).Scan(&b.ID, &b.Description, &b.AmountCents, &b.DueDate, &b.Direction, &b.CategoryID, &b.PaidAt,
		&b.SeriesID, &b.AmountEstimated, &b.PaymentMethod, &b.TransactionID, &b.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Bill{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Bill{}, fmt.Errorf("mark bill paid: %w", err)
	}
	return b, nil
}

// Pay settles a payable bill and records its expense in one commit. Either
// both land or neither does: a bill marked paid with no transaction behind
// it is money that vanished from the budget. The `paid_at is null` guard
// makes paying twice impossible — a second attempt matches no row and comes
// back as domain.ErrNotFound instead of creating a duplicate expense.
func (r *BillRepo) Pay(ctx context.Context, id string, paidAt time.Time, expense domain.Transaction) (domain.Bill, error) {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return domain.Bill{}, fmt.Errorf("begin pay bill: %w", err)
	}
	defer tx.Rollback(ctx)

	// The bill row must be locked and confirmed unpaid before the expense is
	// inserted: bills.transaction_id has a foreign key into transactions, so
	// the transaction row has to exist first, but we must not insert it for a
	// bill that turns out to already be paid. "for update" holds the row lock
	// across both statements, so a concurrent Pay on the same id blocks here
	// instead of racing.
	var exists string
	err = tx.QueryRow(ctx, `select id from bills where id = $1 and paid_at is null for update`, id).Scan(&exists)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Bill{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Bill{}, fmt.Errorf("lock bill %s: %w", id, err)
	}

	if err := insertTransactions(ctx, tx, []domain.Transaction{expense}); err != nil {
		return domain.Bill{}, err
	}

	var b domain.Bill
	err = tx.QueryRow(ctx, `
		update bills set paid_at = $2, amount_cents = $3, amount_estimated = false, transaction_id = $4
		where id = $1 and paid_at is null
		returning id, description, amount_cents, due_date, direction, category_id,
			paid_at, series_id, amount_estimated, payment_method, transaction_id, created_at`,
		id, paidAt, int64(expense.AmountCents), expense.ID,
	).Scan(&b.ID, &b.Description, &b.AmountCents, &b.DueDate, &b.Direction, &b.CategoryID,
		&b.PaidAt, &b.SeriesID, &b.AmountEstimated, &b.PaymentMethod, &b.TransactionID, &b.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Bill{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Bill{}, fmt.Errorf("pay bill %s: %w", id, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Bill{}, fmt.Errorf("commit pay bill %s: %w", id, err)
	}
	return b, nil
}

// Unpay reverses Pay: the bill goes back to pending and the expense it
// created is removed, in one commit.
func (r *BillRepo) Unpay(ctx context.Context, id string) (domain.Bill, error) {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return domain.Bill{}, fmt.Errorf("begin unpay bill: %w", err)
	}
	defer tx.Rollback(ctx)

	var transactionID *string
	err = tx.QueryRow(ctx, `
		update bills set paid_at = null, transaction_id = null
		where id = $1 and paid_at is not null
		returning transaction_id`, id).Scan(&transactionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Bill{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Bill{}, fmt.Errorf("unpay bill %s: %w", id, err)
	}
	if transactionID != nil {
		if _, err := tx.Exec(ctx, `delete from transactions where id = $1`, *transactionID); err != nil {
			return domain.Bill{}, fmt.Errorf("delete expense of bill %s: %w", id, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Bill{}, fmt.Errorf("commit unpay bill %s: %w", id, err)
	}
	return r.Get(ctx, id)
}

// Update edits the fields that describe the obligation itself. It leaves
// paid_at and transaction_id untouched — those describe this occurrence's
// settlement and are only ever changed by MarkPaid (and, later, undoing a
// payment), never by a plain metadata edit — but still returns and scans
// them so the caller gets the current, complete row back.
func (r *BillRepo) Update(ctx context.Context, b domain.Bill) (domain.Bill, error) {
	err := r.db.Pool.QueryRow(ctx, `
		update bills set
			description = $2, amount_cents = $3, due_date = $4,
			direction = $5, category_id = $6, series_id = $7,
			amount_estimated = $8, payment_method = $9
		where id = $1
		returning id, description, amount_cents, due_date, direction, category_id, paid_at,
			series_id, amount_estimated, payment_method, transaction_id, created_at`,
		b.ID, b.Description, b.AmountCents, b.DueDate, b.Direction, b.CategoryID,
		b.SeriesID, b.AmountEstimated, b.PaymentMethod,
	).Scan(&b.ID, &b.Description, &b.AmountCents, &b.DueDate, &b.Direction, &b.CategoryID, &b.PaidAt,
		&b.SeriesID, &b.AmountEstimated, &b.PaymentMethod, &b.TransactionID, &b.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Bill{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Bill{}, fmt.Errorf("update bill %s: %w", b.ID, err)
	}
	return b, nil
}

func (r *BillRepo) Delete(ctx context.Context, id string) error {
	tag, err := r.db.Pool.Exec(ctx, `delete from bills where id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete bill %s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// ReceivedTotalForMonth sums receivable bills actually received (paid_at
// set) within the given month — the closest thing this app has to
// "entradas", since there's no separate income ledger.
func (r *BillRepo) ReceivedTotalForMonth(ctx context.Context, ym domain.YearMonth) (domain.Cents, error) {
	var total int64
	err := r.db.Pool.QueryRow(ctx, `
		select coalesce(sum(amount_cents), 0) from bills
		where direction = 'receber'
		and paid_at >= $1 and paid_at < $2`,
		ym.FirstDay(), ym.Add(1).FirstDay(),
	).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("received total for %v: %w", ym, err)
	}
	return domain.Cents(total), nil
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
