package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/rafael/estus-vault/backend/internal/domain"
	"github.com/rafael/estus-vault/backend/internal/service"
)

type InvestmentHandlers struct {
	investments *service.InvestmentService
}

func NewInvestmentHandlers(investments *service.InvestmentService) *InvestmentHandlers {
	return &InvestmentHandlers{investments: investments}
}

type investmentRequest struct {
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	RateBP     int64  `json:"rate_bp"`
	RatePeriod string `json:"rate_period"`
}

func (r investmentRequest) toInput() service.InvestmentInput {
	return service.InvestmentInput{
		Name: r.Name, Kind: domain.InvestmentKind(r.Kind), RateBP: r.RateBP, RatePeriod: domain.RatePeriod(r.RatePeriod),
	}
}

type contributionRequest struct {
	AmountCents int64 `json:"amount_cents"`
	// Date is YYYY-MM-DD; empty means today.
	Date string `json:"date,omitempty"`
}

func (h *InvestmentHandlers) date(raw string) (time.Time, error) {
	if raw == "" {
		return h.investments.Today(), nil
	}
	d, err := time.Parse(time.DateOnly, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: date must be YYYY-MM-DD", domain.ErrValidation)
	}
	return d, nil
}

// List handles GET /api/investments.
func (h *InvestmentHandlers) List(w http.ResponseWriter, r *http.Request) {
	list, err := h.investments.List(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toInvestmentListDTO(list, h.investments.Today()))
}

// Create handles POST /api/investments: the investment and its first
// contribution.
func (h *InvestmentHandlers) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		investmentRequest
		contributionRequest
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, fmt.Errorf("%w: invalid JSON body", domain.ErrValidation))
		return
	}
	date, err := h.date(req.Date)
	if err != nil {
		writeError(w, err)
		return
	}
	inv, err := h.investments.Create(r.Context(), req.toInput(), domain.Cents(req.AmountCents), date)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toInvestmentDTO(inv, h.investments.Today()))
}

// Update handles PUT /api/investments/{id}: name, kind and rate.
func (h *InvestmentHandlers) Update(w http.ResponseWriter, r *http.Request) {
	var req investmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, fmt.Errorf("%w: invalid JSON body", domain.ErrValidation))
		return
	}
	if err := h.investments.Update(r.Context(), r.PathValue("id"), req.toInput()); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

func (h *InvestmentHandlers) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.investments.Delete(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

// Contribute handles POST /api/investments/{id}/contributions.
func (h *InvestmentHandlers) Contribute(w http.ResponseWriter, r *http.Request) {
	var req contributionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, fmt.Errorf("%w: invalid JSON body", domain.ErrValidation))
		return
	}
	date, err := h.date(req.Date)
	if err != nil {
		writeError(w, err)
		return
	}
	c, err := h.investments.Contribute(r.Context(), r.PathValue("id"), domain.Cents(req.AmountCents), date)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toContributionDTO(c))
}

func (h *InvestmentHandlers) DeleteContribution(w http.ResponseWriter, r *http.Request) {
	if err := h.investments.DeleteContribution(r.Context(), r.PathValue("id"), r.PathValue("cid")); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}
