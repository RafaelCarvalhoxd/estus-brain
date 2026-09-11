package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/rafael/estus-vault/backend/internal/domain"
	"github.com/rafael/estus-vault/backend/internal/service"
	"github.com/rafael/estus-vault/backend/internal/store/postgres"
)

type Handlers struct {
	categories   *postgres.CategoryRepo
	cards        *postgres.CreditCardRepo
	transactions *service.TransactionService
	dashboard    *service.DashboardService
}

func NewHandlers(categories *postgres.CategoryRepo, cards *postgres.CreditCardRepo, transactions *service.TransactionService, dashboard *service.DashboardService) *Handlers {
	return &Handlers{categories: categories, cards: cards, transactions: transactions, dashboard: dashboard}
}

func (h *Handlers) ListCategories(w http.ResponseWriter, r *http.Request) {
	cats, err := h.categories.List(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]categoryDTO, len(cats))
	for i, c := range cats {
		out[i] = toCategoryDTO(c)
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handlers) ListCreditCards(w http.ResponseWriter, r *http.Request) {
	cards, err := h.cards.List(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]creditCardDTO, len(cards))
	for i, c := range cards {
		out[i] = toCreditCardDTO(c)
	}
	writeJSON(w, http.StatusOK, out)
}

// MonthSummary handles GET /api/months/{month}, where {month} is YYYY-MM.
func (h *Handlers) MonthSummary(w http.ResponseWriter, r *http.Request) {
	ym, err := parseYearMonth(r.PathValue("month"))
	if err != nil {
		writeError(w, fmt.Errorf("%w: %v", domain.ErrValidation, err))
		return
	}
	summary, err := h.dashboard.MonthSummary(r.Context(), ym)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toMonthSummaryDTO(summary))
}

func (h *Handlers) CreateTransaction(w http.ResponseWriter, r *http.Request) {
	var req createTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, fmt.Errorf("%w: invalid JSON body", domain.ErrValidation))
		return
	}
	input, err := req.toInput()
	if err != nil {
		writeError(w, fmt.Errorf("%w: purchase_date must be YYYY-MM-DD", domain.ErrValidation))
		return
	}

	txns, err := h.transactions.Create(r.Context(), input)
	if err != nil {
		writeError(w, err)
		return
	}

	competences := make([]string, len(txns))
	for i, t := range txns {
		competences[i] = yearMonthISO(t.CompetenceMonth)
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"created":           len(txns),
		"competence_months": competences,
	})
}

func parseYearMonth(s string) (domain.YearMonth, error) {
	t, err := time.Parse("2006-01", s)
	if err != nil {
		return domain.YearMonth{}, err
	}
	return domain.YearMonthOf(t), nil
}
