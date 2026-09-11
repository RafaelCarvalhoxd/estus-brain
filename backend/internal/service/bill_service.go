package service

import (
	"context"
	"fmt"
	"time"

	"github.com/rafael/estus-vault/backend/internal/domain"
	"github.com/rafael/estus-vault/backend/internal/store/postgres"
)

type BillService struct {
	bills      *postgres.BillRepo
	categories *postgres.CategoryRepo
}

func NewBillService(repo *postgres.BillRepo, categories *postgres.CategoryRepo) *BillService {
	return &BillService{bills: repo, categories: categories}
}

type NewBillInput struct {
	Description string
	AmountCents domain.Cents
	DueDate     time.Time
	Direction   domain.BillDirection
	CategoryID  *string
	Recurring   bool
}

// BillSummary is the "em aberto" snapshot the /contas page's stat tiles are
// built from.
type BillSummary struct {
	PayableOpenCents    domain.Cents
	ReceivableOpenCents domain.Cents
	OverdueCount        int
}

func (s *BillService) Create(ctx context.Context, in NewBillInput) (domain.Bill, error) {
	bill := domain.Bill{
		Description: in.Description,
		AmountCents: in.AmountCents,
		DueDate:     in.DueDate,
		Direction:   in.Direction,
		CategoryID:  in.CategoryID,
		Recurring:   in.Recurring,
	}
	if err := bill.Validate(); err != nil {
		return domain.Bill{}, err
	}
	if in.CategoryID != nil {
		if _, err := s.categories.Get(ctx, *in.CategoryID); err != nil {
			return domain.Bill{}, fmt.Errorf("category %s: %w", *in.CategoryID, err)
		}
	}
	created, err := s.bills.Create(ctx, bill)
	if err != nil {
		return domain.Bill{}, fmt.Errorf("save bill: %w", err)
	}
	return created, nil
}

func (s *BillService) List(ctx context.Context, direction *domain.BillDirection, onlyOpen bool) ([]domain.Bill, error) {
	bills, err := s.bills.List(ctx, direction, onlyOpen)
	if err != nil {
		return nil, fmt.Errorf("list bills: %w", err)
	}
	return bills, nil
}

func (s *BillService) MarkPaid(ctx context.Context, id string, paidAt time.Time) (domain.Bill, error) {
	bill, err := s.bills.MarkPaid(ctx, id, paidAt)
	if err != nil {
		return domain.Bill{}, err
	}
	return bill, nil
}

func (s *BillService) Summary(ctx context.Context) (BillSummary, error) {
	payable, receivable, overdue, err := s.bills.OpenTotals(ctx)
	if err != nil {
		return BillSummary{}, fmt.Errorf("bill summary: %w", err)
	}
	return BillSummary{PayableOpenCents: payable, ReceivableOpenCents: receivable, OverdueCount: overdue}, nil
}
