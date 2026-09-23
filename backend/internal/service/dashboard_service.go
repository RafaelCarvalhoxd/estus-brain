package service

import (
	"context"
	"fmt"

	"github.com/rafael/estus-vault/backend/internal/domain"
	"github.com/rafael/estus-vault/backend/internal/store/postgres"
)

type DashboardService struct {
	transactions *postgres.TransactionRepo
	// openFixed, when set, adds the month's unpaid recurring bills to its
	// fixed spending.
	openFixed func(ctx context.Context, ym domain.YearMonth) (domain.Cents, error)
}

// WithOpenFixedBills counts unpaid recurring bills as fixed spending.
func (s *DashboardService) WithOpenFixedBills(f func(ctx context.Context, ym domain.YearMonth) (domain.Cents, error)) {
	s.openFixed = f
}

func NewDashboardService(t *postgres.TransactionRepo) *DashboardService {
	return &DashboardService{transactions: t}
}

type CategorySlice struct {
	CategoryID         string
	Name               string
	Color              string
	Nature             domain.CategoryNature
	TotalCents         domain.Cents
	MonthlyBudgetCents *domain.Cents
}

type CategoryComparison struct {
	CategoryID    string
	Name          string
	Color         string
	CurrentCents  domain.Cents
	PreviousCents domain.Cents
}

// WeekBucket is one week's worth of spend within the viewed month. Weeks are
// bucketed by the date the money actually left the account: the purchase
// date for débito/pix, and the card's due date for crédito — a card's
// purchases land in the account on one day (the due date), not spread across
// the days they were swiped, so that's the day that belongs in a cash-flow
// view.
type WeekBucket struct {
	Label      string
	TotalCents domain.Cents
}

type MonthSummary struct {
	Month              domain.YearMonth
	TotalCents         domain.Cents
	PreviousMonthCents domain.Cents
	RecurringCents     domain.Cents
	// PaidOutCents is what actually left the account this month, the
	// balance's outflow.
	PaidOutCents domain.Cents
	// OpenFixedCents is the month's recurring bills still to be paid: fixed
	// spending that is not an expense yet.
	OpenFixedCents       domain.Cents
	VariableCents        domain.Cents
	OpenInstallmentCents domain.Cents
	OpenInvoiceCents     domain.Cents
	Categories           []CategorySlice
	Comparison           []CategoryComparison
	Weeks                []WeekBucket
	Transactions         []postgres.TransactionRow
}

func (s *DashboardService) MonthSummary(ctx context.Context, ym domain.YearMonth) (MonthSummary, error) {
	total, err := s.transactions.MonthTotal(ctx, ym)
	if err != nil {
		return MonthSummary{}, err
	}
	previousTotal, err := s.transactions.MonthTotal(ctx, ym.Add(-1))
	if err != nil {
		return MonthSummary{}, err
	}
	recurring, err := s.transactions.RecurringTotal(ctx, ym)
	if err != nil {
		return MonthSummary{}, err
	}
	paidOut, err := s.transactions.PaidOutTotal(ctx, ym)
	if err != nil {
		return MonthSummary{}, err
	}
	var openFixed domain.Cents
	if s.openFixed != nil {
		if openFixed, err = s.openFixed(ctx, ym); err != nil {
			return MonthSummary{}, err
		}
	}
	openInstallments, err := s.transactions.OpenInstallmentsTotal(ctx, ym)
	if err != nil {
		return MonthSummary{}, err
	}
	openInvoice, err := s.transactions.OpenInvoiceTotalAll(ctx, ym)
	if err != nil {
		return MonthSummary{}, err
	}

	currentByCat, err := s.transactions.TotalsByCategory(ctx, ym)
	if err != nil {
		return MonthSummary{}, err
	}
	previousByCat, err := s.transactions.TotalsByCategory(ctx, ym.Add(-1))
	if err != nil {
		return MonthSummary{}, err
	}
	previousTotals := make(map[string]domain.Cents, len(previousByCat))
	for _, c := range previousByCat {
		previousTotals[c.CategoryID] = c.TotalCents
	}

	categories := make([]CategorySlice, len(currentByCat))
	comparison := make([]CategoryComparison, len(currentByCat))
	for i, c := range currentByCat {
		categories[i] = CategorySlice{
			CategoryID: c.CategoryID, Name: c.Name, Color: c.Color, Nature: c.Nature,
			TotalCents: c.TotalCents, MonthlyBudgetCents: c.MonthlyBudgetCents,
		}
		comparison[i] = CategoryComparison{
			CategoryID:    c.CategoryID,
			Name:          c.Name,
			Color:         c.Color,
			CurrentCents:  c.TotalCents,
			PreviousCents: previousTotals[c.CategoryID],
		}
	}

	rows, err := s.transactions.ListByCompetenceMonth(ctx, ym)
	if err != nil {
		return MonthSummary{}, err
	}

	return MonthSummary{
		Month:                ym,
		TotalCents:           total,
		PreviousMonthCents:   previousTotal,
		RecurringCents:       recurring,
		PaidOutCents:         paidOut,
		OpenFixedCents:       openFixed,
		VariableCents:        total - recurring,
		OpenInstallmentCents: openInstallments,
		OpenInvoiceCents:     openInvoice,
		Categories:           categories,
		Comparison:           comparison,
		Weeks:                weeklyBuckets(rows),
		Transactions:         rows,
	}, nil
}

func weeklyBuckets(rows []postgres.TransactionRow) []WeekBucket {
	buckets := make([]domain.Cents, 5)
	for _, r := range rows {
		if r.CategoryKind == domain.KindIncome {
			continue
		}
		day := r.PurchaseDate.Day()
		if r.PaymentMethod == domain.PaymentCredit && r.CardDueDay != nil {
			day = *r.CardDueDay
		}
		idx := (day - 1) / 7
		if idx > 4 {
			idx = 4
		}
		buckets[idx] += r.AmountCents
	}

	out := make([]WeekBucket, 5)
	for i, total := range buckets {
		out[i] = WeekBucket{Label: fmt.Sprintf("S%d", i+1), TotalCents: total}
	}
	return out
}
