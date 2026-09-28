package assistant

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/rafael/estus-vault/backend/internal/domain"
	"github.com/rafael/estus-vault/backend/internal/service"
)

var investmentKinds = []string{"cdb", "tesouro", "lci_lca", "poupanca", "fundo", "acoes", "fii", "cripto", "outro"}

func rateString(inv domain.Investment) string {
	period := "ao ano"
	if inv.RatePeriod == domain.RatePerMonth {
		period = "ao mês"
	}
	return fmt.Sprintf("%s%% %s", strings.Replace(fmt.Sprintf("%g", float64(inv.RateBP)/100), ".", ",", 1), period)
}

func (r *Registry) resolveInvestment(ctx context.Context, query string) (domain.Investment, error) {
	list, err := r.deps.Investments.List(ctx)
	if err != nil {
		return domain.Investment{}, err
	}
	inv, ok, names := match(list, query, func(i domain.Investment) string { return i.ID }, func(i domain.Investment) string { return i.Name })
	if !ok {
		return domain.Investment{}, invalid("investimento %q não encontrado; existentes: %s", query, strings.Join(names, ", "))
	}
	return inv, nil
}

func (r *Registry) addInvestments() {
	d := r.deps
	if d.Investments == nil {
		return
	}

	r.add(Tool{
		Name: "investments_list", Title: "Investimentos", Module: "financeiro", ReadOnly: true,
		Description: "Lista os investimentos com tipo, rendimento, quanto foi aplicado e o valor estimado hoje (juros compostos sobre cada aporte desde a data dele). O dinheiro investido é um saldo à parte do saldo da conta.",
		Input:       object(map[string]any{}),
		run: typed(func(ctx context.Context, _ struct{}) (any, error) {
			list, err := d.Investments.List(ctx)
			if err != nil {
				return nil, err
			}
			type row struct {
				ID        string `json:"id"`
				Name      string `json:"nome"`
				Kind      string `json:"tipo"`
				Rate      string `json:"rendimento"`
				Invested  string `json:"aplicado"`
				Estimated string `json:"estimado_hoje"`
				Count     int    `json:"aportes"`
			}
			today := d.Investments.Today()
			out := make([]row, len(list))
			var invested, estimated domain.Cents
			for i, inv := range list {
				out[i] = row{
					ID: inv.ID, Name: inv.Name, Kind: string(inv.Kind), Rate: rateString(inv),
					Invested: money(inv.InvestedCents()), Estimated: money(inv.EstimatedCents(today)),
					Count: len(inv.Contributions),
				}
				invested += inv.InvestedCents()
				estimated += inv.EstimatedCents(today)
			}
			return map[string]any{
				"investimentos": out, "total_aplicado": money(invested), "total_estimado": money(estimated),
			}, nil
		}),
	})

	r.add(Tool{
		Name: "investments_create", Title: "Novo investimento", Module: "financeiro",
		Description: "Cadastra um investimento com o valor que já está nele. Esse valor inicial não sai do saldo da conta; para dinheiro novo saindo da conta, use investments_contribute depois.",
		Input: object(map[string]any{
			"name":        str("Nome, ex: CDB Nubank"),
			"kind":        enum("Tipo", investmentKinds...),
			"rate":        number("Rendimento em %, ex: 1 para 1% ou 0.85; 0 se não souber"),
			"rate_period": enum("Período do rendimento; padrão mes", "mes", "ano"),
			"amount":      number("Valor inicial em reais"),
			"date":        str("Desde quando está investido: AAAA-MM-DD ou hoje; padrão hoje"),
		}, "name", "kind", "amount"),
		run: typed(func(ctx context.Context, in struct {
			Name       string  `json:"name"`
			Kind       string  `json:"kind"`
			Rate       float64 `json:"rate"`
			RatePeriod string  `json:"rate_period"`
			Amount     float64 `json:"amount"`
			Date       string  `json:"date"`
		}) (any, error) {
			date, err := r.parseDay(in.Date)
			if err != nil {
				return nil, err
			}
			period := domain.RatePeriod(normalize(in.RatePeriod))
			if period == "" {
				period = domain.RatePerMonth
			}
			inv, err := d.Investments.Create(ctx, service.InvestmentInput{
				Name: strings.TrimSpace(in.Name), Kind: domain.InvestmentKind(normalize(in.Kind)),
				RateBP: int64(math.Round(in.Rate * 100)), RatePeriod: period,
			}, cents(in.Amount), date)
			if err != nil {
				return nil, err
			}
			return map[string]any{
				"investimento_id": inv.ID, "nome": inv.Name, "rendimento": rateString(inv),
				"valor_inicial": money(inv.InvestedCents()), "saiu_do_saldo": false,
			}, nil
		}),
	})

	r.add(Tool{
		Name: "investments_contribute", Title: "Aportar", Module: "financeiro",
		Description: "Registra um aporte num investimento existente. O aporte é dinheiro que saiu da conta: entra nas saídas do mês da data e baixa o saldo.",
		Input: object(map[string]any{
			"investment": str("Nome ou id do investimento"),
			"amount":     number("Valor do aporte em reais"),
			"date":       str("AAAA-MM-DD, hoje ou ontem; padrão hoje"),
		}, "investment", "amount"),
		run: typed(func(ctx context.Context, in struct {
			Investment string  `json:"investment"`
			Amount     float64 `json:"amount"`
			Date       string  `json:"date"`
		}) (any, error) {
			inv, err := r.resolveInvestment(ctx, in.Investment)
			if err != nil {
				return nil, err
			}
			date, err := r.parseDay(in.Date)
			if err != nil {
				return nil, err
			}
			c, err := d.Investments.Contribute(ctx, inv.ID, cents(in.Amount), date)
			if err != nil {
				return nil, err
			}
			return map[string]any{
				"investimento": inv.Name, "aporte": money(c.AmountCents), "data": c.Date.Format(dayLayout),
				"mes_do_saldo":   monthString(domain.YearMonthOf(c.Date)),
				"total_aplicado": money(inv.InvestedCents() + c.AmountCents),
			}, nil
		}),
	})
}
