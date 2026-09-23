package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/rafael/estus-vault/backend/internal/domain"
)

// CardSpendingRepo reads credit card purchases grouped the way the Cartões
// dashboard shows them: by card and invoice (competence) month.
type CardSpendingRepo struct{ db *DB }

func NewCardSpendingRepo(db *DB) *CardSpendingRepo { return &CardSpendingRepo{db: db} }

type CardMonthTotal struct {
	CardID     string
	Month      domain.YearMonth
	TotalCents domain.Cents
	Purchases  int
}

// MonthTotals sums each card's credit purchases per invoice month, from
// `from` to `to` inclusive. Months with no purchase are absent.
func (r *CardSpendingRepo) MonthTotals(ctx context.Context, from, to domain.YearMonth) ([]CardMonthTotal, error) {
	rows, err := r.db.Pool.Query(ctx, `
		select credit_card_id, invoice_month, sum(amount_cents), count(*)
		from transactions
		where credit_card_id is not null and payment_method = 'credito'
			and invoice_month between $1 and $2
		group by credit_card_id, invoice_month`,
		from.FirstDay(), to.FirstDay())
	if err != nil {
		return nil, fmt.Errorf("card month totals: %w", err)
	}
	defer rows.Close()
	var out []CardMonthTotal
	for rows.Next() {
		var t CardMonthTotal
		var month time.Time
		var total int64
		if err := rows.Scan(&t.CardID, &month, &total, &t.Purchases); err != nil {
			return nil, fmt.Errorf("scan card month total: %w", err)
		}
		t.Month = domain.YearMonthOf(month)
		t.TotalCents = domain.Cents(total)
		out = append(out, t)
	}
	return out, rows.Err()
}

type CardCategoryTotal struct {
	Name       string
	Color      string
	TotalCents domain.Cents
}

// CategoryTotals is one card's invoice for month, split by category, the
// largest first.
func (r *CardSpendingRepo) CategoryTotals(ctx context.Context, cardID string, month domain.YearMonth) ([]CardCategoryTotal, error) {
	rows, err := r.db.Pool.Query(ctx, `
		select c.name, c.color, sum(t.amount_cents)
		from transactions t
		join categories c on c.id = t.category_id
		where t.credit_card_id = $1 and t.payment_method = 'credito' and t.invoice_month = $2
		group by c.name, c.color
		order by sum(t.amount_cents) desc, c.name`,
		cardID, month.FirstDay())
	if err != nil {
		return nil, fmt.Errorf("card category totals: %w", err)
	}
	defer rows.Close()
	var out []CardCategoryTotal
	for rows.Next() {
		var t CardCategoryTotal
		var total int64
		if err := rows.Scan(&t.Name, &t.Color, &total); err != nil {
			return nil, fmt.Errorf("scan card category total: %w", err)
		}
		t.TotalCents = domain.Cents(total)
		out = append(out, t)
	}
	return out, rows.Err()
}

// PaidInvoices lists the invoice months already paid, per card.
func (r *CardSpendingRepo) PaidInvoices(ctx context.Context) (map[string]map[domain.YearMonth]bool, error) {
	rows, err := r.db.Pool.Query(ctx, `
		select invoice_card_id, invoice_month from bills
		where invoice_card_id is not null and paid_at is not null`)
	if err != nil {
		return nil, fmt.Errorf("paid invoices: %w", err)
	}
	defer rows.Close()
	out := map[string]map[domain.YearMonth]bool{}
	for rows.Next() {
		var cardID string
		var month time.Time
		if err := rows.Scan(&cardID, &month); err != nil {
			return nil, fmt.Errorf("scan paid invoice: %w", err)
		}
		if out[cardID] == nil {
			out[cardID] = map[domain.YearMonth]bool{}
		}
		out[cardID][domain.YearMonthOf(month)] = true
	}
	return out, rows.Err()
}
