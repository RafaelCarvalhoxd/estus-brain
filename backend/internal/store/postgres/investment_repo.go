package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/rafael/estus-vault/backend/internal/domain"
)

type InvestmentRepo struct{ db *DB }

// Ids come from the URL: one that is not a UUID, or a contribution to an
// investment that does not exist, names nothing rather than being a server
// error.
func investmentErr(op string, err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && (pgErr.Code == "22P02" || pgErr.Code == pgForeignKeyViolation) {
		return domain.ErrNotFound
	}
	return fmt.Errorf("%s: %w", op, err)
}

func NewInvestmentRepo(db *DB) *InvestmentRepo { return &InvestmentRepo{db: db} }

// Create saves the investment with its first contribution in one
// transaction, so an investment never exists without money in it.
func (r *InvestmentRepo) Create(ctx context.Context, inv domain.Investment, first domain.InvestmentContribution) (domain.Investment, error) {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return domain.Investment{}, fmt.Errorf("begin create investment: %w", err)
	}
	defer tx.Rollback(ctx)

	err = tx.QueryRow(ctx, `
		insert into investments (name, kind, rate_bp, rate_period)
		values ($1, $2, $3, $4)
		returning id, created_at`,
		inv.Name, string(inv.Kind), inv.RateBP, string(inv.RatePeriod),
	).Scan(&inv.ID, &inv.CreatedAt)
	if err != nil {
		return domain.Investment{}, fmt.Errorf("create investment: %w", err)
	}
	first.InvestmentID = inv.ID
	err = tx.QueryRow(ctx, `
		insert into investment_contributions (investment_id, amount_cents, contributed_on)
		values ($1, $2, $3)
		returning id, created_at`,
		first.InvestmentID, int64(first.AmountCents), first.Date,
	).Scan(&first.ID, &first.CreatedAt)
	if err != nil {
		return domain.Investment{}, fmt.Errorf("create first contribution: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Investment{}, fmt.Errorf("commit create investment: %w", err)
	}
	inv.Contributions = []domain.InvestmentContribution{first}
	return inv, nil
}

// List returns every investment, newest first, each with its
// contributions oldest first.
func (r *InvestmentRepo) List(ctx context.Context) ([]domain.Investment, error) {
	rows, err := r.db.Pool.Query(ctx, `
		select id, name, kind, rate_bp, rate_period, created_at
		from investments
		order by created_at desc`)
	if err != nil {
		return nil, fmt.Errorf("list investments: %w", err)
	}
	defer rows.Close()

	var out []domain.Investment
	index := map[string]int{}
	for rows.Next() {
		var inv domain.Investment
		var kind, period string
		if err := rows.Scan(&inv.ID, &inv.Name, &kind, &inv.RateBP, &period, &inv.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan investment: %w", err)
		}
		inv.Kind, inv.RatePeriod = domain.InvestmentKind(kind), domain.RatePeriod(period)
		index[inv.ID] = len(out)
		out = append(out, inv)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	crows, err := r.db.Pool.Query(ctx, `
		select id, investment_id, amount_cents, contributed_on, created_at
		from investment_contributions
		order by contributed_on asc, created_at asc`)
	if err != nil {
		return nil, fmt.Errorf("list contributions: %w", err)
	}
	defer crows.Close()
	for crows.Next() {
		var c domain.InvestmentContribution
		var cents int64
		if err := crows.Scan(&c.ID, &c.InvestmentID, &cents, &c.Date, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan contribution: %w", err)
		}
		c.AmountCents = domain.Cents(cents)
		if i, ok := index[c.InvestmentID]; ok {
			out[i].Contributions = append(out[i].Contributions, c)
		}
	}
	return out, crows.Err()
}

func (r *InvestmentRepo) Update(ctx context.Context, inv domain.Investment) error {
	tag, err := r.db.Pool.Exec(ctx, `
		update investments set name = $2, kind = $3, rate_bp = $4, rate_period = $5
		where id = $1`,
		inv.ID, inv.Name, string(inv.Kind), inv.RateBP, string(inv.RatePeriod))
	if err != nil {
		return investmentErr("update investment", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *InvestmentRepo) Delete(ctx context.Context, id string) error {
	tag, err := r.db.Pool.Exec(ctx, `delete from investments where id = $1`, id)
	if err != nil {
		return investmentErr("delete investment", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *InvestmentRepo) AddContribution(ctx context.Context, c domain.InvestmentContribution) (domain.InvestmentContribution, error) {
	err := r.db.Pool.QueryRow(ctx, `
		insert into investment_contributions (investment_id, amount_cents, contributed_on)
		values ($1, $2, $3)
		returning id, created_at`,
		c.InvestmentID, int64(c.AmountCents), c.Date,
	).Scan(&c.ID, &c.CreatedAt)
	if err != nil {
		return domain.InvestmentContribution{}, investmentErr("add contribution", err)
	}
	return c, nil
}

// DeleteContribution refuses to remove an investment's last contribution:
// deleting the investment is how its money goes back.
func (r *InvestmentRepo) DeleteContribution(ctx context.Context, investmentID, id string) error {
	var remaining int
	err := r.db.Pool.QueryRow(ctx, `
		select count(*) from investment_contributions where investment_id = $1`, investmentID,
	).Scan(&remaining)
	if err != nil {
		return investmentErr("count contributions", err)
	}
	var deleted string
	err = r.db.Pool.QueryRow(ctx, `
		delete from investment_contributions
		where id = $2 and investment_id = $1 and $3 > 1
		returning id`, investmentID, id, remaining,
	).Scan(&deleted)
	if errors.Is(err, pgx.ErrNoRows) {
		if remaining <= 1 {
			return fmt.Errorf("%w: an investment keeps at least one contribution", domain.ErrConflict)
		}
		return domain.ErrNotFound
	}
	if err != nil {
		return investmentErr("delete contribution", err)
	}
	return nil
}

// MonthTotal is what was contributed in ym: money that left the account.
func (r *InvestmentRepo) MonthTotal(ctx context.Context, ym domain.YearMonth) (domain.Cents, error) {
	var total int64
	err := r.db.Pool.QueryRow(ctx, `
		select coalesce(sum(amount_cents), 0) from investment_contributions
		where contributed_on >= $1 and contributed_on < $2`,
		ym.FirstDay(), ym.Add(1).FirstDay(),
	).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("investment month total for %v: %w", ym, err)
	}
	return domain.Cents(total), nil
}
