package httpapi

import (
	"time"

	"github.com/rafael/estus-vault/backend/internal/domain"
	"github.com/rafael/estus-vault/backend/internal/service"
)

type billDTO struct {
	ID              string                `json:"id"`
	Description     string                `json:"description"`
	Amount          moneyDTO              `json:"amount"`
	DueDate         string                `json:"due_date"`
	Direction       string                `json:"direction"`
	CategoryID      *string               `json:"category_id,omitempty"`
	PaidAt          *string               `json:"paid_at,omitempty"`
	Recurring       bool                  `json:"recurring"`
	AmountEstimated bool                  `json:"amount_estimated"`
	PaymentMethod   *domain.PaymentMethod `json:"payment_method,omitempty"`
	Status          string                `json:"status"`
}

func toBillDTO(b domain.Bill, now time.Time) billDTO {
	dto := billDTO{
		ID:              b.ID,
		Description:     b.Description,
		Amount:          toMoneyDTO(b.AmountCents),
		DueDate:         b.DueDate.Format("2006-01-02"),
		Direction:       string(b.Direction),
		CategoryID:      b.CategoryID,
		Recurring:       b.Recurring(),
		AmountEstimated: b.AmountEstimated,
		PaymentMethod:   b.PaymentMethod,
		Status:          b.Status(now),
	}
	if b.PaidAt != nil {
		formatted := b.PaidAt.Format("2006-01-02")
		dto.PaidAt = &formatted
	}
	return dto
}

type billSummaryDTO struct {
	PayableOpen    moneyDTO `json:"payable_open"`
	ReceivableOpen moneyDTO `json:"receivable_open"`
	OverdueCount   int      `json:"overdue_count"`
}

func toBillSummaryDTO(s service.BillSummary) billSummaryDTO {
	return billSummaryDTO{
		PayableOpen:    toMoneyDTO(s.PayableOpenCents),
		ReceivableOpen: toMoneyDTO(s.ReceivableOpenCents),
		OverdueCount:   s.OverdueCount,
	}
}

type createBillRequest struct {
	Description     string                `json:"description"`
	AmountCents     int64                 `json:"amount_cents"`
	DueDate         string                `json:"due_date"` // YYYY-MM-DD
	Direction       string                `json:"direction"`
	CategoryID      *string               `json:"category_id,omitempty"`
	Recurring       bool                  `json:"recurring,omitempty"`
	AmountEstimated bool                  `json:"amount_estimated,omitempty"`
	PaymentMethod   *domain.PaymentMethod `json:"payment_method,omitempty"`
}

func (r createBillRequest) toInput() (service.NewBillInput, error) {
	dueDate, err := time.Parse("2006-01-02", r.DueDate)
	if err != nil {
		return service.NewBillInput{}, err
	}
	return service.NewBillInput{
		Description:     r.Description,
		AmountCents:     domain.Cents(r.AmountCents),
		DueDate:         dueDate,
		Direction:       domain.BillDirection(r.Direction),
		CategoryID:      r.CategoryID,
		Recurring:       r.Recurring,
		AmountEstimated: r.AmountEstimated,
		PaymentMethod:   r.PaymentMethod,
	}, nil
}

type markBillPaidRequest struct {
	PaidAt string `json:"paid_at,omitempty"` // YYYY-MM-DD, defaults to today
}

// payBillRequest is what the owner confirms when settling a payable bill:
// the amount that actually left the account, and how.
type payBillRequest struct {
	PaidOn        string  `json:"paid_on"` // YYYY-MM-DD
	AmountCents   int64   `json:"amount_cents"`
	CategoryID    string  `json:"category_id"`
	PaymentMethod string  `json:"payment_method"`
	CreditCardID  *string `json:"credit_card_id,omitempty"`
}

func (r payBillRequest) toInput() (service.PaymentInput, error) {
	paidOn, err := time.Parse("2006-01-02", r.PaidOn)
	if err != nil {
		return service.PaymentInput{}, err
	}
	return service.PaymentInput{
		PaidAt:        paidOn,
		AmountCents:   domain.Cents(r.AmountCents),
		CategoryID:    r.CategoryID,
		PaymentMethod: domain.PaymentMethod(r.PaymentMethod),
		CreditCardID:  r.CreditCardID,
	}, nil
}
