package httpapi

import (
	"time"

	"github.com/rafael/estus-vault/backend/internal/domain"
)

type contributionDTO struct {
	ID     string   `json:"id"`
	Amount moneyDTO `json:"amount"`
	Date   string   `json:"date"`
	// Initial is the opening amount, which never left the balance.
	Initial bool `json:"initial"`
}

func toContributionDTO(c domain.InvestmentContribution) contributionDTO {
	return contributionDTO{ID: c.ID, Amount: toMoneyDTO(c.AmountCents), Date: c.Date.Format(time.DateOnly), Initial: c.Initial}
}

type investmentDTO struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Kind          string            `json:"kind"`
	RateBP        int64             `json:"rate_bp"`
	RatePeriod    string            `json:"rate_period"`
	Invested      moneyDTO          `json:"invested"`
	Estimated     moneyDTO          `json:"estimated"`
	Contributions []contributionDTO `json:"contributions"`
}

func toInvestmentDTO(inv domain.Investment, today time.Time) investmentDTO {
	dto := investmentDTO{
		ID: inv.ID, Name: inv.Name, Kind: string(inv.Kind), RateBP: inv.RateBP, RatePeriod: string(inv.RatePeriod),
		Invested:      toMoneyDTO(inv.InvestedCents()),
		Estimated:     toMoneyDTO(inv.EstimatedCents(today)),
		Contributions: make([]contributionDTO, len(inv.Contributions)),
	}
	for i, c := range inv.Contributions {
		dto.Contributions[i] = toContributionDTO(c)
	}
	return dto
}

type investmentListDTO struct {
	Investments []investmentDTO `json:"investments"`
	Invested    moneyDTO        `json:"invested"`
	Estimated   moneyDTO        `json:"estimated"`
	AsOf        string          `json:"as_of"`
}

func toInvestmentListDTO(list []domain.Investment, today time.Time) investmentListDTO {
	out := investmentListDTO{Investments: make([]investmentDTO, len(list)), AsOf: today.Format(time.DateOnly)}
	var invested, estimated domain.Cents
	for i, inv := range list {
		out.Investments[i] = toInvestmentDTO(inv, today)
		invested += inv.InvestedCents()
		estimated += inv.EstimatedCents(today)
	}
	out.Invested, out.Estimated = toMoneyDTO(invested), toMoneyDTO(estimated)
	return out
}
