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
			series_id, amount_estimated, payment_method, transaction_id,
			series_ended, amount_varies, credit_card_id
		)
		values (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		returning id, created_at`,
		b.Description, b.AmountCents, b.DueDate, b.Direction, b.CategoryID, b.PaidAt,
		b.SeriesID, b.AmountEstimated, b.PaymentMethod, b.TransactionID,
		b.SeriesEnded, b.AmountVaries, b.CreditCardID,
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
			series_id, amount_estimated, payment_method, transaction_id, created_at,
			series_ended, amount_varies, credit_card_id, invoice_card_id, invoice_month
		from bills where id = $1`, id,
	).Scan(&b.ID, &b.Description, &b.AmountCents, &b.DueDate, &b.Direction, &b.CategoryID, &b.PaidAt,
		&b.SeriesID, &b.AmountEstimated, &b.PaymentMethod, &b.TransactionID, &b.CreatedAt,
		&b.SeriesEnded, &b.AmountVaries, &b.CreditCardID, &b.InvoiceCardID, &b.InvoiceMonth)
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
			series_id, amount_estimated, payment_method, transaction_id, created_at,
			series_ended, amount_varies, credit_card_id, invoice_card_id, invoice_month
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
			paid_at, series_id, amount_estimated, payment_method, transaction_id, created_at,
			series_ended, amount_varies, credit_card_id, invoice_card_id, invoice_month
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
// each series' next month is copied from, and the row Materialize checks
// SeriesEnded on to decide whether that series still grows new occurrences.
func (r *BillRepo) LatestPerSeries(ctx context.Context) ([]domain.Bill, error) {
	rows, err := r.db.Pool.Query(ctx, `
		select distinct on (series_id)
			id, description, amount_cents, due_date, direction, category_id,
			paid_at, series_id, amount_estimated, payment_method, transaction_id, created_at,
			series_ended, amount_varies, credit_card_id, invoice_card_id, invoice_month
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
			&b.SeriesID, &b.AmountEstimated, &b.PaymentMethod, &b.TransactionID, &b.CreatedAt,
			&b.SeriesEnded, &b.AmountVaries, &b.CreditCardID, &b.InvoiceCardID, &b.InvoiceMonth); err != nil {
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
			series_id, amount_estimated, payment_method, transaction_id, created_at,
			series_ended, amount_varies, credit_card_id, invoice_card_id, invoice_month`,
		id, paidAt,
	).Scan(&b.ID, &b.Description, &b.AmountCents, &b.DueDate, &b.Direction, &b.CategoryID, &b.PaidAt,
		&b.SeriesID, &b.AmountEstimated, &b.PaymentMethod, &b.TransactionID, &b.CreatedAt,
		&b.SeriesEnded, &b.AmountVaries, &b.CreditCardID, &b.InvoiceCardID, &b.InvoiceMonth)
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
			paid_at, series_id, amount_estimated, payment_method, transaction_id, created_at,
			series_ended, amount_varies, credit_card_id, invoice_card_id, invoice_month`,
		id, paidAt, int64(expense.AmountCents), expense.ID,
	).Scan(&b.ID, &b.Description, &b.AmountCents, &b.DueDate, &b.Direction, &b.CategoryID,
		&b.PaidAt, &b.SeriesID, &b.AmountEstimated, &b.PaymentMethod, &b.TransactionID, &b.CreatedAt,
		&b.SeriesEnded, &b.AmountVaries, &b.CreditCardID, &b.InvoiceCardID, &b.InvoiceMonth)
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

	// RETURNING reflects the row AFTER the update, so a single
	// "update ... transaction_id = null ... returning transaction_id" would
	// always hand back null — the pointer has to be read before it is
	// cleared, in the same locked row, or the expense it names is lost
	// forever (the FK is NO ACTION, so nothing else can recover it).
	var transactionID *string
	err = tx.QueryRow(ctx, `
		select transaction_id from bills where id = $1 and paid_at is not null for update`,
		id).Scan(&transactionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Bill{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Bill{}, fmt.Errorf("read bill %s before unpay: %w", id, err)
	}

	if _, err := tx.Exec(ctx, `
		update bills set paid_at = null, transaction_id = null where id = $1`, id); err != nil {
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
			amount_estimated = $8, payment_method = $9,
			amount_varies = $10, series_ended = $11, credit_card_id = $12
		where id = $1
		returning id, description, amount_cents, due_date, direction, category_id, paid_at,
			series_id, amount_estimated, payment_method, transaction_id, created_at,
			series_ended, amount_varies, credit_card_id, invoice_card_id, invoice_month`,
		b.ID, b.Description, b.AmountCents, b.DueDate, b.Direction, b.CategoryID,
		b.SeriesID, b.AmountEstimated, b.PaymentMethod,
		b.AmountVaries, b.SeriesEnded, b.CreditCardID,
	).Scan(&b.ID, &b.Description, &b.AmountCents, &b.DueDate, &b.Direction, &b.CategoryID, &b.PaidAt,
		&b.SeriesID, &b.AmountEstimated, &b.PaymentMethod, &b.TransactionID, &b.CreatedAt,
		&b.SeriesEnded, &b.AmountVaries, &b.CreditCardID, &b.InvoiceCardID, &b.InvoiceMonth)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Bill{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Bill{}, fmt.Errorf("update bill %s: %w", b.ID, err)
	}
	return b, nil
}

// SetSeriesEnded flips whether a series still grows new occurrences, without
// touching anything else about the bills: EndSeries (ended=true) stops
// Materialize from creating any more occurrences past the latest one, and
// ResumeSeries (ended=false) undoes it. It follows MarkPaid's shape — one
// column, no other field rewritten — rather than reusing Update, which also
// revalidates and rewrites every describable field; ending a series is
// neither of those.
//
// It updates EVERY occurrence of the series, not just the row the owner
// clicked: Materialize only ever reads LatestPerSeries, so a flag stamped on
// an older occurrence alone is invisible to it — the owner could end the
// series from a past month's row, see "· Repetição encerrada" on that row,
// and still get next month's occurrence materialized right back, because the
// actual latest occurrence never got the flag. Setting it series-wide is also
// what keeps the label consistent across every month of the series, instead
// of only the one row that was clicked. A bill with no series (series_id is
// null) matches no row here — `series_id = (select series_id ...)` is never
// true against NULL — so it comes back as domain.ErrNotFound; the one-off
// guard against ending a non-series bill lives in BillService.EndSeries,
// which checks Recurring() before ever calling this.
func (r *BillRepo) SetSeriesEnded(ctx context.Context, id string, ended bool) (domain.Bill, error) {
	tag, err := r.db.Pool.Exec(ctx, `
		update bills set series_ended = $2
		where series_id = (select series_id from bills where id = $1)`,
		id, ended,
	)
	if err != nil {
		return domain.Bill{}, fmt.Errorf("set series_ended for bill %s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return domain.Bill{}, domain.ErrNotFound
	}
	return r.Get(ctx, id)
}

// DeleteUnpaidAfter removes the occurrences of id's series that fall due
// after it and are still unpaid. Paid ones stay: they carry the expense
// transaction that paid them.
func (r *BillRepo) DeleteUnpaidAfter(ctx context.Context, id string) (int, error) {
	tag, err := r.db.Pool.Exec(ctx, `
		delete from bills b
		using bills ref
		where ref.id = $1
			and b.series_id = ref.series_id
			and b.due_date > ref.due_date
			and b.paid_at is null`,
		id,
	)
	if err != nil {
		return 0, fmt.Errorf("delete bills after %s: %w", id, err)
	}
	return int(tag.RowsAffected()), nil
}

// SyncInvoices makes each card's invoice bill, for every due month from
// `from` on, equal the sum of that card's credit purchases competing for
// the month: it creates missing invoices, updates unpaid ones and deletes
// unpaid ones left with no purchase. Paid invoices are never touched, and
// months before `from` are left as they are.
func (r *BillRepo) SyncInvoices(ctx context.Context, from domain.YearMonth) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin sync invoices: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		insert into bills (description, amount_cents, due_date, direction, invoice_card_id, invoice_month)
		select 'Fatura ' || c.name, t.cents,
			make_date(extract(year from t.month)::int, extract(month from t.month)::int, c.due_day),
			'pagar', c.id, t.month
		from (
			select credit_card_id as card_id, invoice_month as month, sum(amount_cents) as cents
			from transactions
			where credit_card_id is not null and payment_method = 'credito' and invoice_month >= $1
			group by credit_card_id, invoice_month
		) t
		join credit_cards c on c.id = t.card_id
		on conflict (invoice_card_id, invoice_month) where invoice_card_id is not null
		do update set amount_cents = excluded.amount_cents, description = excluded.description, due_date = excluded.due_date
		where bills.paid_at is null`,
		from.FirstDay()); err != nil {
		return fmt.Errorf("upsert invoices: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		delete from bills b
		where b.invoice_card_id is not null and b.paid_at is null and b.invoice_month >= $1
			and not exists (
				select 1 from transactions t
				where t.credit_card_id = b.invoice_card_id and t.payment_method = 'credito'
					and t.invoice_month = b.invoice_month
			)`,
		from.FirstDay()); err != nil {
		return fmt.Errorf("delete empty invoices: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit sync invoices: %w", err)
	}
	return nil
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

// ReceivedTotalForMonth is the month's "entradas": receivable bills
// received (paid_at set) within it, plus débito/pix transactions in a
// receita category.
func (r *BillRepo) ReceivedTotalForMonth(ctx context.Context, ym domain.YearMonth) (domain.Cents, error) {
	var total int64
	err := r.db.Pool.QueryRow(ctx, `
		select
			(select coalesce(sum(amount_cents), 0) from bills
				where direction = 'receber' and paid_at >= $1 and paid_at < $2)
			+ (select coalesce(sum(amount_cents), 0) from transactions
				where competence_month = $1 and payment_method <> 'credito'
					and category_id in (select id from categories where kind = 'receita'))`,
		ym.FirstDay(), ym.Add(1).FirstDay(),
	).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("received total for %v: %w", ym, err)
	}
	return domain.Cents(total), nil
}

// OpenTotals is bounded to due_date < the first day of NEXT month: opening a
// month is what materializes it (see ListByMonth), so clicking the
// month-navigation arrow ahead a few times used to fabricate that many
// months of every recurring series and inflate this total by rows nothing
// made due — three clicks took "A pagar em aberto" from a real R$1.680 to a
// fabricated R$6.720. A bill genuinely past due from an earlier month is
// still counted (the bound is an upper limit on due_date, not a floor): it
// really is open. Only bills due beyond the current month — which only exist
// because the owner looked ahead — are excluded.
//
// It covers the unpaid bills due in ym. When ym is today's month it also
// keeps the ones still unpaid from earlier months: they are open now.
// Overdue counts those among them already past their due date.
func (r *BillRepo) OpenTotals(ctx context.Context, ym domain.YearMonth, today time.Time) (payableCents, receivableCents domain.Cents, overdueCount int, err error) {
	from := ym.FirstDay()
	if !domain.YearMonthOf(today).Before(ym) && !ym.Before(domain.YearMonthOf(today)) {
		from = time.Time{}
	}
	to := ym.Add(1).FirstDay()
	todayDate := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)

	var payable, receivable int64
	err = r.db.Pool.QueryRow(ctx, `
		select
			coalesce(sum(amount_cents) filter (where direction = 'pagar'), 0),
			coalesce(sum(amount_cents) filter (where direction = 'receber'), 0),
			count(*) filter (where due_date < $3)
		from bills
		where paid_at is null and due_date >= $1 and due_date < $2`,
		from, to, todayDate,
	).Scan(&payable, &receivable, &overdueCount)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("open totals for %v: %w", ym, err)
	}
	return domain.Cents(payable), domain.Cents(receivable), overdueCount, nil
}
