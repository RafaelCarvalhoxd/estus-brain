package service

import (
	"context"
	"time"

	"github.com/rafael/estus-vault/backend/internal/domain"
	"github.com/rafael/estus-vault/backend/internal/store/postgres"
)

// CardSpendingService builds the Cartões dashboard: each card's current
// invoice, its categories and the months around it.
type CardSpendingService struct {
	spending *postgres.CardSpendingRepo
	cards    *postgres.CreditCardRepo
	loc      *time.Location
	now      func() time.Time
}

func NewCardSpendingService(spending *postgres.CardSpendingRepo, cards *postgres.CreditCardRepo, loc *time.Location) *CardSpendingService {
	return &CardSpendingService{spending: spending, cards: cards, loc: loc, now: time.Now}
}

// Months shown per card: this many before the current invoice, and one after
// it (installments already falling there).
const cardHistoryMonths = 4

type CardInvoice struct {
	Month      domain.YearMonth
	DueDate    time.Time
	TotalCents domain.Cents
	Purchases  int
	Paid       bool
}

type CardOverview struct {
	Card domain.CreditCard
	// Current is the invoice a purchase made today lands on.
	Current    CardInvoice
	Months     []CardInvoice
	Categories []postgres.CardCategoryTotal
}

func (s *CardSpendingService) Overview(ctx context.Context) ([]CardOverview, error) {
	cards, err := s.cards.List(ctx)
	if err != nil {
		return nil, err
	}
	paid, err := s.spending.PaidInvoices(ctx)
	if err != nil {
		return nil, err
	}
	today := s.now().In(s.loc)

	out := make([]CardOverview, 0, len(cards))
	for _, card := range cards {
		current := domain.InvoiceMonth(today, card)
		from, to := current.Add(-cardHistoryMonths), current.Add(1)
		totals, err := s.spending.MonthTotals(ctx, from, to)
		if err != nil {
			return nil, err
		}
		byMonth := map[domain.YearMonth]postgres.CardMonthTotal{}
		for _, t := range totals {
			if t.CardID == card.ID {
				byMonth[t.Month] = t
			}
		}
		invoice := func(m domain.YearMonth) CardInvoice {
			t := byMonth[m]
			return CardInvoice{
				Month:      m,
				DueDate:    time.Date(m.Year, time.Month(m.Month), card.DueDay, 0, 0, 0, 0, time.UTC),
				TotalCents: t.TotalCents,
				Purchases:  t.Purchases,
				Paid:       paid[card.ID][m],
			}
		}
		ov := CardOverview{Card: card, Current: invoice(current)}
		for m := from; !to.Before(m); m = m.Add(1) {
			ov.Months = append(ov.Months, invoice(m))
		}
		if ov.Categories, err = s.spending.CategoryTotals(ctx, card.ID, current); err != nil {
			return nil, err
		}
		out = append(out, ov)
	}
	return out, nil
}
