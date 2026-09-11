// Package service holds application logic that coordinates the domain
// layer and repositories — the layer HTTP handlers call into, and the only
// layer allowed to open a database transaction that spans more than one
// repository call.
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/rafael/estus-vault/backend/internal/domain"
	"github.com/rafael/estus-vault/backend/internal/store/postgres"
)

type TransactionService struct {
	transactions *postgres.TransactionRepo
	categories   *postgres.CategoryRepo
	cards        *postgres.CreditCardRepo
}

func NewTransactionService(t *postgres.TransactionRepo, c *postgres.CategoryRepo, cc *postgres.CreditCardRepo) *TransactionService {
	return &TransactionService{transactions: t, categories: c, cards: cc}
}

// NewTransactionInput is what a "Novo lançamento" submission carries.
// Installments only makes sense for credit purchases; the service rejects
// anything else so the rule can't be bypassed by a malformed request rather
// than caught deep inside domain math.
type NewTransactionInput struct {
	Description   string
	AmountCents   domain.Cents
	CategoryID    string
	PaymentMethod domain.PaymentMethod
	PurchaseDate  time.Time
	CreditCardID  string // required when PaymentMethod == credito
	Installments  int    // 1 when not parceled
	IsRecurring   bool
}

func (in NewTransactionInput) validate() error {
	if in.Description == "" {
		return fmt.Errorf("%w: description is required", domain.ErrValidation)
	}
	if in.AmountCents <= 0 {
		return fmt.Errorf("%w: amount must be positive", domain.ErrValidation)
	}
	if !in.PaymentMethod.Valid() {
		return fmt.Errorf("%w: invalid payment method %q", domain.ErrValidation, in.PaymentMethod)
	}
	if in.PaymentMethod == domain.PaymentCredit && in.CreditCardID == "" {
		return domain.ErrCreditCardMissing
	}
	if in.PaymentMethod != domain.PaymentCredit && in.Installments > 1 {
		return fmt.Errorf("%w: installments require crédito payment method", domain.ErrValidation)
	}
	if in.Installments < 1 {
		in.Installments = 1
	}
	return nil
}

// Create records a purchase, expanding it into one row per installment when
// applicable. It validates that the category and (when present) the credit
// card actually exist before touching the ledger — a foreign key violation
// deep in a batch insert is a worse failure mode than a clear 404 up front.
func (s *TransactionService) Create(ctx context.Context, in NewTransactionInput) ([]domain.Transaction, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	if in.Installments < 1 {
		in.Installments = 1
	}

	if _, err := s.categories.Get(ctx, in.CategoryID); err != nil {
		return nil, fmt.Errorf("category %s: %w", in.CategoryID, err)
	}
	if in.PaymentMethod == domain.PaymentCredit {
		if _, err := s.cards.Get(ctx, in.CreditCardID); err != nil {
			return nil, fmt.Errorf("credit card %s: %w", in.CreditCardID, err)
		}
	}

	var txns []domain.Transaction
	if in.PaymentMethod == domain.PaymentCredit {
		txns = domain.NewInstallmentPurchase(in.Description, in.AmountCents, in.CategoryID, in.CreditCardID, in.PurchaseDate, in.Installments)
		if in.Installments == 1 {
			txns[0].IsRecurring = in.IsRecurring
		}
	} else {
		txns = []domain.Transaction{{
			ID:               domain.NewID(),
			Description:      in.Description,
			AmountCents:      in.AmountCents,
			CategoryID:       in.CategoryID,
			PaymentMethod:    in.PaymentMethod,
			PurchaseDate:     in.PurchaseDate,
			CompetenceMonth:  domain.CompetenceMonth(in.PurchaseDate, in.PaymentMethod),
			InstallmentTotal: 1,
			IsRecurring:      in.IsRecurring,
		}}
	}

	if err := s.transactions.CreateBatch(ctx, txns); err != nil {
		return nil, fmt.Errorf("save transaction: %w", err)
	}
	return txns, nil
}

func (s *TransactionService) Update(ctx context.Context, id, description, categoryID string) error {
	if description == "" {
		return fmt.Errorf("%w: description is required", domain.ErrValidation)
	}
	if _, err := s.categories.Get(ctx, categoryID); err != nil {
		return fmt.Errorf("category %s: %w", categoryID, err)
	}
	return s.transactions.UpdateDescriptionAndCategory(ctx, id, description, categoryID)
}

func (s *TransactionService) Delete(ctx context.Context, id string) error {
	return s.transactions.Delete(ctx, id)
}
