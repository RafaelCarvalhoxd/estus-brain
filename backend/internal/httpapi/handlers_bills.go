package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/rafael/estus-vault/backend/internal/domain"
	"github.com/rafael/estus-vault/backend/internal/service"
)

type BillHandlers struct {
	bills *service.BillService
}

func NewBillHandlers(s *service.BillService) *BillHandlers {
	return &BillHandlers{bills: s}
}

// List handles GET /api/bills, optionally filtered by ?direction=pagar|receber
// and ?open=1 (only bills not yet paid).
func (h *BillHandlers) List(w http.ResponseWriter, r *http.Request) {
	var direction *domain.BillDirection
	if raw := r.URL.Query().Get("direction"); raw != "" {
		d := domain.BillDirection(raw)
		if !d.Valid() {
			writeError(w, fmt.Errorf("%w: invalid direction %q", domain.ErrValidation, raw))
			return
		}
		direction = &d
	}
	onlyOpen := r.URL.Query().Get("open") == "1"

	bills, err := h.bills.List(r.Context(), direction, onlyOpen)
	if err != nil {
		writeError(w, err)
		return
	}
	now := time.Now()
	out := make([]billDTO, len(bills))
	for i, b := range bills {
		out[i] = toBillDTO(b, now)
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *BillHandlers) Create(w http.ResponseWriter, r *http.Request) {
	var req createBillRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, fmt.Errorf("%w: invalid JSON body", domain.ErrValidation))
		return
	}
	input, err := req.toInput()
	if err != nil {
		writeError(w, fmt.Errorf("%w: due_date must be YYYY-MM-DD", domain.ErrValidation))
		return
	}

	bill, err := h.bills.Create(r.Context(), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toBillDTO(bill, time.Now()))
}

func (h *BillHandlers) Summary(w http.ResponseWriter, r *http.Request) {
	summary, err := h.bills.Summary(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toBillSummaryDTO(summary))
}

// MarkPaid handles POST /api/bills/{id}/paid.
func (h *BillHandlers) MarkPaid(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req markBillPaidRequest
	if r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, fmt.Errorf("%w: invalid JSON body", domain.ErrValidation))
			return
		}
	}

	paidAt := time.Now()
	if req.PaidAt != "" {
		parsed, err := time.Parse("2006-01-02", req.PaidAt)
		if err != nil {
			writeError(w, fmt.Errorf("%w: paid_at must be YYYY-MM-DD", domain.ErrValidation))
			return
		}
		paidAt = parsed
	}

	bill, err := h.bills.MarkPaid(r.Context(), id, paidAt)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toBillDTO(bill, time.Now()))
}
