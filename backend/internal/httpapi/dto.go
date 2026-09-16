package httpapi

import (
	"strings"
	"time"

	"github.com/rafael/estus-vault/backend/internal/domain"
	"github.com/rafael/estus-vault/backend/internal/service"
	"github.com/rafael/estus-vault/backend/internal/store/postgres"
)

// Every DTO exposes amounts as both cents (for exact frontend math, e.g.
// summing) and a pre-formatted "R$ 1.234,56" string (so the frontend never
// re-implements Brazilian currency formatting).

type categoryDTO struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Nature        string    `json:"nature"`
	Color         string    `json:"color"`
	MonthlyBudget *moneyDTO `json:"monthly_budget,omitempty"`
}

func toCategoryDTO(c domain.Category) categoryDTO {
	dto := categoryDTO{ID: c.ID, Name: c.Name, Nature: string(c.Nature), Color: c.Color}
	if c.MonthlyBudgetCents != nil {
		m := toMoneyDTO(*c.MonthlyBudgetCents)
		dto.MonthlyBudget = &m
	}
	return dto
}

type updateCategoryBudgetRequest struct {
	MonthlyBudgetCents *int64 `json:"monthly_budget_cents"`
}

type categoryRequest struct {
	Name   string `json:"name"`
	Nature string `json:"nature"`
	Color  string `json:"color"`
}

func (r categoryRequest) toDomain() domain.Category {
	return domain.Category{Name: r.Name, Nature: domain.CategoryNature(r.Nature), Color: r.Color}
}

type creditCardRequest struct {
	Name       string `json:"name"`
	ClosingDay int    `json:"closing_day"`
	DueDay     int    `json:"due_day"`
}

func (r creditCardRequest) toDomain() domain.CreditCard {
	return domain.CreditCard{Name: strings.TrimSpace(r.Name), ClosingDay: r.ClosingDay, DueDay: r.DueDay}
}

type creditCardDTO struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	ClosingDay int    `json:"closing_day"`
	DueDay     int    `json:"due_day"`
}

func toCreditCardDTO(c domain.CreditCard) creditCardDTO {
	return creditCardDTO{ID: c.ID, Name: c.Name, ClosingDay: c.ClosingDay, DueDay: c.DueDay}
}

type moneyDTO struct {
	Cents     int64  `json:"cents"`
	Formatted string `json:"formatted"`
}

func toMoneyDTO(c domain.Cents) moneyDTO {
	return moneyDTO{Cents: int64(c), Formatted: c.String()}
}

type transactionDTO struct {
	ID                string   `json:"id"`
	Description       string   `json:"description"`
	Amount            moneyDTO `json:"amount"`
	CategoryID        string   `json:"category_id"`
	CategoryName      string   `json:"category_name"`
	CategoryColor     string   `json:"category_color"`
	PaymentMethod     string   `json:"payment_method"`
	PurchaseDate      string   `json:"purchase_date"`
	CompetenceMonth   string   `json:"competence_month"`
	IsRecurring       bool     `json:"is_recurring"`
	InstallmentNumber int      `json:"installment_number,omitempty"`
	InstallmentTotal  int      `json:"installment_total,omitempty"`
}

func toTransactionDTO(t postgres.TransactionRow) transactionDTO {
	return transactionDTO{
		ID:                t.ID,
		Description:       t.Description,
		Amount:            toMoneyDTO(t.AmountCents),
		CategoryID:        t.CategoryID,
		CategoryName:      t.CategoryName,
		CategoryColor:     t.CategoryColor,
		PaymentMethod:     string(t.PaymentMethod),
		PurchaseDate:      t.PurchaseDate.Format("2006-01-02"),
		CompetenceMonth:   yearMonthISO(t.CompetenceMonth),
		IsRecurring:       t.IsRecurring,
		InstallmentNumber: t.InstallmentNumber,
		InstallmentTotal:  t.InstallmentTotal,
	}
}

type categorySliceDTO struct {
	CategoryID    string    `json:"category_id"`
	Name          string    `json:"name"`
	Color         string    `json:"color"`
	Total         moneyDTO  `json:"total"`
	MonthlyBudget *moneyDTO `json:"monthly_budget,omitempty"`
}

type categoryComparisonDTO struct {
	CategoryID string   `json:"category_id"`
	Name       string   `json:"name"`
	Color      string   `json:"color"`
	Current    moneyDTO `json:"current"`
	Previous   moneyDTO `json:"previous"`
}

type weekBucketDTO struct {
	Label string   `json:"label"`
	Total moneyDTO `json:"total"`
}

type monthSummaryDTO struct {
	Month            string                  `json:"month"`
	Total            moneyDTO                `json:"total"`
	PreviousMonth    moneyDTO                `json:"previous_month"`
	Recurring        moneyDTO                `json:"recurring"`
	OpenInstallments moneyDTO                `json:"open_installments"`
	OpenInvoice      moneyDTO                `json:"open_invoice"`
	Categories       []categorySliceDTO      `json:"categories"`
	Comparison       []categoryComparisonDTO `json:"comparison"`
	Weeks            []weekBucketDTO         `json:"weeks"`
	Transactions     []transactionDTO        `json:"transactions"`
}

func toMonthSummaryDTO(s service.MonthSummary) monthSummaryDTO {
	categories := make([]categorySliceDTO, len(s.Categories))
	for i, c := range s.Categories {
		categories[i] = categorySliceDTO{CategoryID: c.CategoryID, Name: c.Name, Color: c.Color, Total: toMoneyDTO(c.TotalCents)}
		if c.MonthlyBudgetCents != nil {
			m := toMoneyDTO(*c.MonthlyBudgetCents)
			categories[i].MonthlyBudget = &m
		}
	}
	comparison := make([]categoryComparisonDTO, len(s.Comparison))
	for i, c := range s.Comparison {
		comparison[i] = categoryComparisonDTO{
			CategoryID: c.CategoryID, Name: c.Name, Color: c.Color,
			Current: toMoneyDTO(c.CurrentCents), Previous: toMoneyDTO(c.PreviousCents),
		}
	}
	weeks := make([]weekBucketDTO, len(s.Weeks))
	for i, w := range s.Weeks {
		weeks[i] = weekBucketDTO{Label: w.Label, Total: toMoneyDTO(w.TotalCents)}
	}
	transactions := make([]transactionDTO, len(s.Transactions))
	for i, t := range s.Transactions {
		transactions[i] = toTransactionDTO(t)
	}

	return monthSummaryDTO{
		Month:            yearMonthISO(s.Month),
		Total:            toMoneyDTO(s.TotalCents),
		PreviousMonth:    toMoneyDTO(s.PreviousMonthCents),
		Recurring:        toMoneyDTO(s.RecurringCents),
		OpenInstallments: toMoneyDTO(s.OpenInstallmentCents),
		OpenInvoice:      toMoneyDTO(s.OpenInvoiceCents),
		Categories:       categories,
		Comparison:       comparison,
		Weeks:            weeks,
		Transactions:     transactions,
	}
}

func yearMonthISO(ym domain.YearMonth) string {
	return ym.FirstDay().Format("2006-01")
}

type createTransactionRequest struct {
	Description   string `json:"description"`
	AmountCents   int64  `json:"amount_cents"`
	CategoryID    string `json:"category_id"`
	PaymentMethod string `json:"payment_method"`
	PurchaseDate  string `json:"purchase_date"` // YYYY-MM-DD
	CreditCardID  string `json:"credit_card_id,omitempty"`
	Installments  int    `json:"installments,omitempty"`
	IsRecurring   bool   `json:"is_recurring,omitempty"`
}

func (r createTransactionRequest) toInput() (service.NewTransactionInput, error) {
	date, err := time.Parse("2006-01-02", r.PurchaseDate)
	if err != nil {
		return service.NewTransactionInput{}, err
	}
	installments := r.Installments
	if installments < 1 {
		installments = 1
	}
	return service.NewTransactionInput{
		Description:   r.Description,
		AmountCents:   domain.Cents(r.AmountCents),
		CategoryID:    r.CategoryID,
		PaymentMethod: domain.PaymentMethod(r.PaymentMethod),
		PurchaseDate:  date,
		CreditCardID:  r.CreditCardID,
		Installments:  installments,
		IsRecurring:   r.IsRecurring,
	}, nil
}
