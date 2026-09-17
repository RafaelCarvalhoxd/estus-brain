package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

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
	// SeriesID is the series this bill already belongs to, if the caller
	// knows it (used by later occurrence flows). Nil combined with Recurring
	// true means "start a brand new series".
	SeriesID *string
	// Recurring says the caller wants this bill to be (or stay) part of a
	// monthly series. When true and SeriesID is nil, Create mints a fresh
	// series id — that generation lives here, not in the repository.
	Recurring       bool
	AmountEstimated bool
	PaymentMethod   *domain.PaymentMethod
}

// BillSummary is the "em aberto" snapshot the /contas page's stat tiles are
// built from.
type BillSummary struct {
	PayableOpenCents    domain.Cents
	ReceivableOpenCents domain.Cents
	OverdueCount        int
}

func (s *BillService) Create(ctx context.Context, in NewBillInput) (domain.Bill, error) {
	seriesID := in.SeriesID
	if in.Recurring && seriesID == nil {
		id := uuid.NewString()
		seriesID = &id
	}
	bill := domain.Bill{
		Description:     in.Description,
		AmountCents:     in.AmountCents,
		DueDate:         in.DueDate,
		Direction:       in.Direction,
		CategoryID:      in.CategoryID,
		SeriesID:        seriesID,
		AmountEstimated: in.AmountEstimated,
		PaymentMethod:   in.PaymentMethod,
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

func (s *BillService) Update(ctx context.Context, id string, in NewBillInput) (domain.Bill, error) {
	seriesID := in.SeriesID
	if in.Recurring && seriesID == nil {
		newSeries := uuid.NewString()
		seriesID = &newSeries
	}
	bill := domain.Bill{
		ID:              id,
		Description:     in.Description,
		AmountCents:     in.AmountCents,
		DueDate:         in.DueDate,
		Direction:       in.Direction,
		CategoryID:      in.CategoryID,
		SeriesID:        seriesID,
		AmountEstimated: in.AmountEstimated,
		PaymentMethod:   in.PaymentMethod,
	}
	if err := bill.Validate(); err != nil {
		return domain.Bill{}, err
	}
	if in.CategoryID != nil {
		if _, err := s.categories.Get(ctx, *in.CategoryID); err != nil {
			return domain.Bill{}, fmt.Errorf("category %s: %w", *in.CategoryID, err)
		}
	}
	updated, err := s.bills.Update(ctx, bill)
	if err != nil {
		return domain.Bill{}, err
	}
	return updated, nil
}

func (s *BillService) Delete(ctx context.Context, id string) error {
	return s.bills.Delete(ctx, id)
}

func (s *BillService) Summary(ctx context.Context) (BillSummary, error) {
	payable, receivable, overdue, err := s.bills.OpenTotals(ctx)
	if err != nil {
		return BillSummary{}, fmt.Errorf("bill summary: %w", err)
	}
	return BillSummary{PayableOpenCents: payable, ReceivableOpenCents: receivable, OverdueCount: overdue}, nil
}

func (s *BillService) ReceivedTotal(ctx context.Context, ym domain.YearMonth) (domain.Cents, error) {
	return s.bills.ReceivedTotalForMonth(ctx, ym)
}
