package httpapi

import (
	"fmt"
	"net/http"

	"github.com/rafael/estus-vault/backend/internal/service"
)

type CardSpendingHandlers struct {
	spending *service.CardSpendingService
}

func NewCardSpendingHandlers(spending *service.CardSpendingService) *CardSpendingHandlers {
	return &CardSpendingHandlers{spending: spending}
}

type cardInvoiceDTO struct {
	Month     string   `json:"month"` // YYYY-MM
	DueDate   string   `json:"due_date"`
	Total     moneyDTO `json:"total"`
	Purchases int      `json:"purchases"`
	Paid      bool     `json:"paid"`
}

type cardCategoryDTO struct {
	Name  string   `json:"name"`
	Color string   `json:"color"`
	Total moneyDTO `json:"total"`
}

type cardOverviewDTO struct {
	Card       creditCardDTO     `json:"card"`
	Current    cardInvoiceDTO    `json:"current"`
	Months     []cardInvoiceDTO  `json:"months"`
	Categories []cardCategoryDTO `json:"categories"`
}

func toCardInvoiceDTO(i service.CardInvoice) cardInvoiceDTO {
	return cardInvoiceDTO{
		Month:     fmt.Sprintf("%04d-%02d", i.Month.Year, i.Month.Month),
		DueDate:   i.DueDate.Format("2006-01-02"),
		Total:     toMoneyDTO(i.TotalCents),
		Purchases: i.Purchases,
		Paid:      i.Paid,
	}
}

// Overview handles GET /api/credit-cards/spending.
func (h *CardSpendingHandlers) Overview(w http.ResponseWriter, r *http.Request) {
	overviews, err := h.spending.Overview(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]cardOverviewDTO, len(overviews))
	for i, ov := range overviews {
		dto := cardOverviewDTO{
			Card:       toCreditCardDTO(ov.Card),
			Current:    toCardInvoiceDTO(ov.Current),
			Months:     make([]cardInvoiceDTO, len(ov.Months)),
			Categories: make([]cardCategoryDTO, len(ov.Categories)),
		}
		for j, m := range ov.Months {
			dto.Months[j] = toCardInvoiceDTO(m)
		}
		for j, c := range ov.Categories {
			dto.Categories[j] = cardCategoryDTO{Name: c.Name, Color: c.Color, Total: toMoneyDTO(c.TotalCents)}
		}
		out[i] = dto
	}
	writeJSON(w, http.StatusOK, out)
}
