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
// and ?open=1 (only bills not yet paid), or scoped to ?month=YYYY-MM — which
// is how the Contas screen reads a month now that it has navigation. A
// month-scoped request materializes that month's recurring series first (see
// BillService.ListByMonth), so opening a month is what makes its bills exist.
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

	var bills []domain.Bill
	var err error
	if raw := r.URL.Query().Get("month"); raw != "" {
		ym, parseErr := parseYearMonth(raw)
		if parseErr != nil {
			writeError(w, fmt.Errorf("%w: month must be YYYY-MM", domain.ErrValidation))
			return
		}
		bills, err = h.bills.ListByMonth(r.Context(), ym, direction)
	} else {
		onlyOpen := r.URL.Query().Get("open") == "1"
		bills, err = h.bills.List(r.Context(), direction, onlyOpen)
	}
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

func (h *BillHandlers) Update(w http.ResponseWriter, r *http.Request) {
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

	bill, err := h.bills.Update(r.Context(), r.PathValue("id"), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toBillDTO(bill, time.Now()))
}

func (h *BillHandlers) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.bills.Delete(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *BillHandlers) Summary(w http.ResponseWriter, r *http.Request) {
	summary, err := h.bills.Summary(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toBillSummaryDTO(summary))
}

// ReceivedTotal handles GET /api/bills/received?month=YYYY-MM.
func (h *BillHandlers) ReceivedTotal(w http.ResponseWriter, r *http.Request) {
	ym, err := parseYearMonth(r.URL.Query().Get("month"))
	if err != nil {
		writeError(w, fmt.Errorf("%w: month must be YYYY-MM", domain.ErrValidation))
		return
	}
	total, err := h.bills.ReceivedTotal(r.Context(), ym)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]moneyDTO{"received": toMoneyDTO(total)})
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

// Pay handles POST /api/bills/{id}/pay: settling a payable bill, which
// records its expense in the transactions ledger in the same commit.
func (h *BillHandlers) Pay(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req payBillRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, fmt.Errorf("%w: invalid JSON body", domain.ErrValidation))
		return
	}
	input, err := req.toInput()
	if err != nil {
		writeError(w, fmt.Errorf("%w: paid_on must be YYYY-MM-DD", domain.ErrValidation))
		return
	}

	bill, _, err := h.bills.Pay(r.Context(), id, input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toBillDTO(bill, time.Now()))
}

// Unpay handles DELETE /api/bills/{id}/paid: undoing a payment removes the
// expense it created and puts the bill back to pending.
func (h *BillHandlers) Unpay(w http.ResponseWriter, r *http.Request) {
	bill, err := h.bills.Unpay(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toBillDTO(bill, time.Now()))
}

// EndSeries handles POST /api/bills/{id}/end-series: stops a recurring
// bill's series from growing new occurrences, without deleting or altering
// anything already materialized.
func (h *BillHandlers) EndSeries(w http.ResponseWriter, r *http.Request) {
	bill, err := h.bills.EndSeries(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toBillDTO(bill, time.Now()))
}

// ResumeSeries handles DELETE /api/bills/{id}/end-series: undoes EndSeries,
// so the next month opened picks the series back up from where it left off.
func (h *BillHandlers) ResumeSeries(w http.ResponseWriter, r *http.Request) {
	bill, err := h.bills.ResumeSeries(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toBillDTO(bill, time.Now()))
}
