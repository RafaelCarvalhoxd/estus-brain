package service

import (
	"context"
	"fmt"
	"time"

	"github.com/rafael/estus-vault/backend/internal/domain"
	"github.com/rafael/estus-vault/backend/internal/store/postgres"
)

type InvestmentService struct {
	investments *postgres.InvestmentRepo
	// loc is the owner's timezone: "today" for the estimate is read in it.
	loc *time.Location
	now func() time.Time
}

func NewInvestmentService(repo *postgres.InvestmentRepo, loc *time.Location) *InvestmentService {
	return &InvestmentService{investments: repo, loc: loc, now: time.Now}
}

type InvestmentInput struct {
	Name       string
	Kind       domain.InvestmentKind
	RateBP     int64
	RatePeriod domain.RatePeriod
}

func (in InvestmentInput) investment() domain.Investment {
	return domain.Investment{Name: in.Name, Kind: in.Kind, RateBP: in.RateBP, RatePeriod: in.RatePeriod}
}

// Today is the owner's calendar day, as the midnight-UTC date the
// contributions are stored with.
func (s *InvestmentService) Today() time.Time {
	t := s.now().In(s.loc)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// Create opens an investment with its first contribution.
func (s *InvestmentService) Create(ctx context.Context, in InvestmentInput, amount domain.Cents, date time.Time) (domain.Investment, error) {
	inv := in.investment()
	if err := inv.Validate(); err != nil {
		return domain.Investment{}, err
	}
	first := domain.InvestmentContribution{AmountCents: amount, Date: date}
	if err := first.Validate(); err != nil {
		return domain.Investment{}, err
	}
	created, err := s.investments.Create(ctx, inv, first)
	if err != nil {
		return domain.Investment{}, fmt.Errorf("save investment: %w", err)
	}
	return created, nil
}

func (s *InvestmentService) List(ctx context.Context) ([]domain.Investment, error) {
	return s.investments.List(ctx)
}

func (s *InvestmentService) Update(ctx context.Context, id string, in InvestmentInput) error {
	inv := in.investment()
	inv.ID = id
	if err := inv.Validate(); err != nil {
		return err
	}
	return s.investments.Update(ctx, inv)
}

func (s *InvestmentService) Delete(ctx context.Context, id string) error {
	return s.investments.Delete(ctx, id)
}

func (s *InvestmentService) Contribute(ctx context.Context, investmentID string, amount domain.Cents, date time.Time) (domain.InvestmentContribution, error) {
	c := domain.InvestmentContribution{InvestmentID: investmentID, AmountCents: amount, Date: date}
	if err := c.Validate(); err != nil {
		return domain.InvestmentContribution{}, err
	}
	return s.investments.AddContribution(ctx, c)
}

func (s *InvestmentService) DeleteContribution(ctx context.Context, investmentID, id string) error {
	return s.investments.DeleteContribution(ctx, investmentID, id)
}

// MonthTotal is what was contributed in ym, for the month's outflow.
func (s *InvestmentService) MonthTotal(ctx context.Context, ym domain.YearMonth) (domain.Cents, error) {
	return s.investments.MonthTotal(ctx, ym)
}
