package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/rafael/estus-vault/backend/internal/domain"
)

type TransactionRepo struct{ db *DB }

func NewTransactionRepo(db *DB) *TransactionRepo { return &TransactionRepo{db: db} }

// TransactionRow is a transaction joined with the display data the
// dashboard and transaction list need, avoiding an N+1 lookup per row.
type TransactionRow struct {
	domain.Transaction
	CategoryName  string
	CategoryColor string
	CategoryKind  domain.CategoryKind
	CardDueDay    *int
	// PurchaseTotalCents is the whole purchase: the sum of all installments.
	PurchaseTotalCents domain.Cents
}

// CreateBatch inserts every transaction produced by a single user action —
// a plain purchase is a batch of one, a 12x installment purchase is a batch
// of twelve — atomically, so a crash mid-insert can never leave a purchase
// half-recorded across months.
func (r *TransactionRepo) CreateBatch(ctx context.Context, txns []domain.Transaction) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction batch: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := insertTransactions(ctx, tx, txns); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// insertTransactions writes txns inside an already-open database
// transaction, so a caller that has more to write in the same commit — a
// bill being settled, say — can reuse the exact same insert.
func insertTransactions(ctx context.Context, tx pgx.Tx, txns []domain.Transaction) error {
	for _, t := range txns {
		_, err := tx.Exec(ctx, `
			insert into transactions (
				id, description, amount_cents, category_id, payment_method,
				purchase_date, credit_card_id, competence_month,
				installment_group_id, installment_number, installment_total,
				is_recurring, invoice_month
			) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
			t.ID, t.Description, int64(t.AmountCents), t.CategoryID, t.PaymentMethod,
			t.PurchaseDate, t.CreditCardID, t.CompetenceMonth.FirstDay(),
			t.InstallmentGroupID, t.InstallmentNumber, t.InstallmentTotal,
			t.IsRecurring, invoiceMonthDate(t.InvoiceMonth),
		)
		if err != nil {
			return fmt.Errorf("insert transaction %s: %w", t.Description, err)
		}
	}
	return nil
}

func (r *TransactionRepo) ListByCompetenceMonth(ctx context.Context, ym domain.YearMonth) ([]TransactionRow, error) {
	rows, err := r.db.Pool.Query(ctx, `
		select
			t.id, t.description, t.amount_cents, t.category_id, t.payment_method,
			t.purchase_date, t.credit_card_id, t.installment_group_id,
			t.installment_number, t.installment_total, t.is_recurring, t.created_at,
			c.name, c.color, c.kind, cc.due_day, t.invoice_month,
			case when t.installment_group_id is null then t.amount_cents
				else (select sum(g.amount_cents) from transactions g where g.installment_group_id = t.installment_group_id) end
		from transactions t
		join categories c on c.id = t.category_id
		left join credit_cards cc on cc.id = t.credit_card_id
		where t.competence_month = $1
		order by t.created_at desc`,
		ym.FirstDay(),
	)
	if err != nil {
		return nil, fmt.Errorf("list transactions for %v: %w", ym, err)
	}
	defer rows.Close()

	var out []TransactionRow
	for rows.Next() {
		var tr TransactionRow
		var amount int64
		var invoice *time.Time
		var purchaseTotal int64
		if err := rows.Scan(
			&tr.ID, &tr.Description, &amount, &tr.CategoryID, &tr.PaymentMethod,
			&tr.PurchaseDate, &tr.CreditCardID, &tr.InstallmentGroupID,
			&tr.InstallmentNumber, &tr.InstallmentTotal, &tr.IsRecurring, &tr.CreatedAt,
			&tr.CategoryName, &tr.CategoryColor, &tr.CategoryKind, &tr.CardDueDay, &invoice, &purchaseTotal,
		); err != nil {
			return nil, fmt.Errorf("scan transaction row: %w", err)
		}
		tr.AmountCents = domain.Cents(amount)
		tr.CompetenceMonth = ym
		tr.PurchaseTotalCents = domain.Cents(purchaseTotal)
		if invoice != nil {
			m := domain.YearMonthOf(*invoice)
			tr.InvoiceMonth = &m
		}
		out = append(out, tr)
	}
	return out, rows.Err()
}

type CategoryTotal struct {
	CategoryID         string
	Name               string
	Color              string
	Nature             domain.CategoryNature
	TotalCents         domain.Cents
	MonthlyBudgetCents *domain.Cents
}

// TotalsByCategory sums every transaction competing for the given month,
// grouped by category, including categories with zero spend so the
// dashboard can render a stable, fully-ranked list.
func (r *TransactionRepo) TotalsByCategory(ctx context.Context, ym domain.YearMonth) ([]CategoryTotal, error) {
	rows, err := r.db.Pool.Query(ctx, `
		select c.id, c.name, c.color, c.nature, c.monthly_budget_cents, coalesce(sum(t.amount_cents), 0)
		from categories c
		left join transactions t on t.category_id = c.id and t.competence_month = $1
		where c.kind = 'despesa'
		group by c.id, c.name, c.color, c.nature, c.monthly_budget_cents
		order by coalesce(sum(t.amount_cents), 0) desc, c.name`,
		ym.FirstDay(),
	)
	if err != nil {
		return nil, fmt.Errorf("totals by category for %v: %w", ym, err)
	}
	defer rows.Close()

	var out []CategoryTotal
	for rows.Next() {
		var ct CategoryTotal
		var total int64
		var budget *int64
		if err := rows.Scan(&ct.CategoryID, &ct.Name, &ct.Color, &ct.Nature, &budget, &total); err != nil {
			return nil, fmt.Errorf("scan category total: %w", err)
		}
		ct.TotalCents = domain.Cents(total)
		if budget != nil {
			b := domain.Cents(*budget)
			ct.MonthlyBudgetCents = &b
		}
		out = append(out, ct)
	}
	return out, rows.Err()
}

func (r *TransactionRepo) MonthTotal(ctx context.Context, ym domain.YearMonth) (domain.Cents, error) {
	var total int64
	err := r.db.Pool.QueryRow(ctx, `
		select coalesce(sum(amount_cents), 0) from transactions
		where competence_month = $1 and category_id in (select id from categories where kind = 'despesa')`,
		ym.FirstDay(),
	).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("month total for %v: %w", ym, err)
	}
	return domain.Cents(total), nil
}

// PaidOutTotal is the money that actually left in ym: débito and pix
// expenses of the month, plus card invoices paid in it. Card purchases only
// count once their invoice is paid.
func (r *TransactionRepo) PaidOutTotal(ctx context.Context, ym domain.YearMonth) (domain.Cents, error) {
	var total int64
	err := r.db.Pool.QueryRow(ctx, `
		select
			(select coalesce(sum(amount_cents), 0) from transactions
				where competence_month = $1 and payment_method <> 'credito'
					and category_id in (select id from categories where kind = 'despesa'))
			+ (select coalesce(sum(amount_cents), 0) from bills
				where invoice_card_id is not null and paid_at >= $1 and paid_at < $2)`,
		ym.FirstDay(), ym.Add(1).FirstDay(),
	).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("paid out total for %v: %w", ym, err)
	}
	return domain.Cents(total), nil
}

// RecurringTotal is the month's fixed spending: expenses marked recurring
// and installments. An installment row exists only in its own month, so a
// purchase stops counting once its last installment is past.
func (r *TransactionRepo) RecurringTotal(ctx context.Context, ym domain.YearMonth) (domain.Cents, error) {
	var total int64
	err := r.db.Pool.QueryRow(ctx, `
		select coalesce(sum(amount_cents), 0) from transactions
		where competence_month = $1 and (is_recurring or installment_total > 1) and category_id in (select id from categories where kind = 'despesa')`,
		ym.FirstDay(),
	).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("recurring total for %v: %w", ym, err)
	}
	return domain.Cents(total), nil
}

// OpenInstallmentsTotal sums the remaining, not-yet-competent installments
// of every parceled purchase — the amount still owed beyond the month being
// viewed, which is what "9 de 12 restantes" reports on the dashboard.
func (r *TransactionRepo) OpenInstallmentsTotal(ctx context.Context, asOf domain.YearMonth) (domain.Cents, error) {
	var total int64
	err := r.db.Pool.QueryRow(ctx, `
		select coalesce(sum(amount_cents), 0) from transactions
		where installment_total > 1 and competence_month > $1 and category_id in (select id from categories where kind = 'despesa')`,
		asOf.FirstDay(),
	).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("open installments as of %v: %w", asOf, err)
	}
	return domain.Cents(total), nil
}

// OpenInvoiceTotalAll sums every card's next invoice combined. Nothing
// renders this value today — there is no "fatura atual" tile on the
// dashboard — so this is currently unused. Under the competence rule in
// domain.CompetenceMonth (viewedMonth+1 as "the invoice still open"), the
// result is only coherent for cards whose due_day is before their
// closing_day; for a card whose due_day falls after closing (the common
// case, due the month after it closes), viewedMonth+1 is the wrong bucket.
// Whoever wires up that tile needs to fix this query first.
func (r *TransactionRepo) OpenInvoiceTotalAll(ctx context.Context, viewedMonth domain.YearMonth) (domain.Cents, error) {
	var total int64
	err := r.db.Pool.QueryRow(ctx, `
		select coalesce(sum(amount_cents), 0) from transactions
		where credit_card_id is not null and invoice_month = $1`,
		viewedMonth.Add(1).FirstDay(),
	).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("open invoice total for %v: %w", viewedMonth, err)
	}
	return domain.Cents(total), nil
}

// UpdateDescriptionAndCategory changes only the two fields that never
// affect competence-month or installment math — everything else (amount,
// date, payment method, installments) is immutable after creation, because
// changing it would mean re-deriving which months and how many rows the
// purchase spans, and doing that safely on an existing installment group is
// a bigger feature than "edit a transaction". Delete and recreate covers
// that case for now.
func (r *TransactionRepo) UpdateDescriptionAndCategory(ctx context.Context, id, description, categoryID string) error {
	tag, err := r.db.Pool.Exec(ctx, `
		update transactions set description = $2, category_id = $3 where id = $1`,
		id, description, categoryID,
	)
	if err != nil {
		return fmt.Errorf("update transaction %s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// Delete refuses a transaction that a settled bill still points at.
// bills.transaction_id references transactions(id) with no `on delete`
// clause (NO ACTION), so Postgres rejects the delete outright rather than
// leaving the bill's pointer dangling; without translating that violation
// here it would surface as an opaque 500. The right fix for the owner is not
// "delete the expense" — it's undoing the payment in Contas, which deletes
// the bill's own copy of this same expense in one commit (see
// BillRepo.Unpay) — so the message points there instead of just saying no.
// purchaseRows matches id and, when it is an installment, every other
// installment of the same purchase.
const purchaseRows = `(id = $1 or installment_group_id = (select installment_group_id from transactions where id = $1))`

// Delete removes a purchase: an installment takes all its siblings with it.
func (r *TransactionRepo) Delete(ctx context.Context, id string) error {
	tag, err := r.db.Pool.Exec(ctx, `delete from transactions where `+purchaseRows, id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgForeignKeyViolation {
			return fmt.Errorf("%w: este lançamento veio da quitação de uma conta; desfaça o pagamento em Contas para removê-lo", domain.ErrConflict)
		}
		return fmt.Errorf("delete transaction %s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// Replace swaps the purchase id belongs to (all its installments) for txns.
// A bill paid by that purchase is pointed at the new row, which is why a
// purchase that settled a bill can't become an installment one.
func (r *TransactionRepo) Replace(ctx context.Context, id string, txns []domain.Transaction) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin replace transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var oldIDs []string
	rows, err := tx.Query(ctx, `select id from transactions where `+purchaseRows, id)
	if err != nil {
		return fmt.Errorf("find purchase of %s: %w", id, err)
	}
	for rows.Next() {
		var old string
		if err := rows.Scan(&old); err != nil {
			rows.Close()
			return fmt.Errorf("scan purchase row: %w", err)
		}
		oldIDs = append(oldIDs, old)
	}
	rows.Close()
	if len(oldIDs) == 0 {
		return domain.ErrNotFound
	}

	var linked int
	if err := tx.QueryRow(ctx, `select count(*) from bills where transaction_id = any($1)`, oldIDs).Scan(&linked); err != nil {
		return fmt.Errorf("check bills paid by %s: %w", id, err)
	}
	if linked > 0 && len(txns) != 1 {
		return fmt.Errorf("%w: este lançamento veio da quitação de uma conta e não pode virar parcelado", domain.ErrConflict)
	}

	if err := insertTransactions(ctx, tx, txns); err != nil {
		return err
	}
	if linked > 0 {
		if _, err := tx.Exec(ctx, `update bills set transaction_id = $1 where transaction_id = any($2)`, txns[0].ID, oldIDs); err != nil {
			return fmt.Errorf("move bill to the edited transaction: %w", err)
		}
	}
	if _, err := tx.Exec(ctx, `delete from transactions where id = any($1)`, oldIDs); err != nil {
		return fmt.Errorf("delete old rows of %s: %w", id, err)
	}
	return tx.Commit(ctx)
}

// SpentOn is what was bought on a calendar day, whatever month it counts
// against — an installment purchase counts in full on the day it was made.
func (r *TransactionRepo) SpentOn(ctx context.Context, day time.Time) (domain.Cents, error) {
	var total int64
	err := r.db.Pool.QueryRow(ctx,
		`select coalesce(sum(amount_cents), 0) from transactions where purchase_date = $1::date`,
		day.Format(domain.DayLayout),
	).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("spent on %s: %w", day.Format(domain.DayLayout), err)
	}
	return domain.Cents(total), nil
}

func invoiceMonthDate(m *domain.YearMonth) *time.Time {
	if m == nil {
		return nil
	}
	d := m.FirstDay()
	return &d
}
