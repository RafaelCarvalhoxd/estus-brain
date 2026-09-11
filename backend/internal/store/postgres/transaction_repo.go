package postgres

import (
	"context"
	"fmt"

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
	CardDueDay    *int
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

	for _, t := range txns {
		_, err := tx.Exec(ctx, `
			insert into transactions (
				id, description, amount_cents, category_id, payment_method,
				purchase_date, credit_card_id, competence_month,
				installment_group_id, installment_number, installment_total,
				is_recurring
			) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
			t.ID, t.Description, int64(t.AmountCents), t.CategoryID, t.PaymentMethod,
			t.PurchaseDate, t.CreditCardID, t.CompetenceMonth.FirstDay(),
			t.InstallmentGroupID, t.InstallmentNumber, t.InstallmentTotal,
			t.IsRecurring,
		)
		if err != nil {
			return fmt.Errorf("insert transaction %s: %w", t.Description, err)
		}
	}
	return tx.Commit(ctx)
}

func (r *TransactionRepo) ListByCompetenceMonth(ctx context.Context, ym domain.YearMonth) ([]TransactionRow, error) {
	rows, err := r.db.Pool.Query(ctx, `
		select
			t.id, t.description, t.amount_cents, t.category_id, t.payment_method,
			t.purchase_date, t.credit_card_id, t.installment_group_id,
			t.installment_number, t.installment_total, t.is_recurring, t.created_at,
			c.name, c.color, cc.due_day
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
		if err := rows.Scan(
			&tr.ID, &tr.Description, &amount, &tr.CategoryID, &tr.PaymentMethod,
			&tr.PurchaseDate, &tr.CreditCardID, &tr.InstallmentGroupID,
			&tr.InstallmentNumber, &tr.InstallmentTotal, &tr.IsRecurring, &tr.CreatedAt,
			&tr.CategoryName, &tr.CategoryColor, &tr.CardDueDay,
		); err != nil {
			return nil, fmt.Errorf("scan transaction row: %w", err)
		}
		tr.AmountCents = domain.Cents(amount)
		tr.CompetenceMonth = ym
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
		select coalesce(sum(amount_cents), 0) from transactions where competence_month = $1`,
		ym.FirstDay(),
	).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("month total for %v: %w", ym, err)
	}
	return domain.Cents(total), nil
}

func (r *TransactionRepo) RecurringTotal(ctx context.Context, ym domain.YearMonth) (domain.Cents, error) {
	var total int64
	err := r.db.Pool.QueryRow(ctx, `
		select coalesce(sum(amount_cents), 0) from transactions
		where competence_month = $1 and is_recurring`,
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
		where installment_total > 1 and competence_month > $1`,
		asOf.FirstDay(),
	).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("open installments as of %v: %w", asOf, err)
	}
	return domain.Cents(total), nil
}

// OpenInvoiceTotalAll sums every card's next invoice combined — used by the
// dashboard's "fatura atual" tile, which reports the household total rather
// than breaking it down per card.
func (r *TransactionRepo) OpenInvoiceTotalAll(ctx context.Context, viewedMonth domain.YearMonth) (domain.Cents, error) {
	var total int64
	err := r.db.Pool.QueryRow(ctx, `
		select coalesce(sum(amount_cents), 0) from transactions
		where credit_card_id is not null and competence_month = $1`,
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

func (r *TransactionRepo) Delete(ctx context.Context, id string) error {
	tag, err := r.db.Pool.Exec(ctx, `delete from transactions where id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete transaction %s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}
