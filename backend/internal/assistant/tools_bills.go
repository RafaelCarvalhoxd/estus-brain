package assistant

import (
	"context"
	"strings"
	"time"

	"github.com/rafael/estus-vault/backend/internal/domain"
	"github.com/rafael/estus-vault/backend/internal/service"
)

type billRow struct {
	ID          string `json:"id"`
	Description string `json:"descricao"`
	Amount      string `json:"valor"`
	Due         string `json:"vencimento"`
	Direction   string `json:"tipo"`
	Status      string `json:"status"`
	Recurring   bool   `json:"recorrente,omitempty"`
}

func (r *Registry) billRow(b domain.Bill) billRow {
	return billRow{b.ID, b.Description, money(b.AmountCents), b.DueDate.Format(dayLayout), string(b.Direction), b.Status(r.today()), b.Recurring()}
}

func (r *Registry) addBills() {
	d := r.deps
	if d.Bills == nil {
		return
	}

	r.add(Tool{
		Name: "bills_list", Title: "Contas a pagar e receber", Module: "contas", ReadOnly: true,
		Description: "Lista contas a pagar e/ou a receber, filtrando por status e por mês de vencimento. Ex.: contas a pagar do mês que vem.",
		Input: object(map[string]any{
			"direction": enum("pagar, receber ou ambas; padrão ambas", "pagar", "receber", "ambas"),
			"status":    enum("abertas (pendentes e atrasadas), atrasadas, pagas ou todas; padrão abertas", "abertas", "atrasadas", "pagas", "todas"),
			"month":     str("Mês de vencimento AAAA-MM, ou atual/proximo/passado; vazio = qualquer mês"),
		}),
		run: typed(func(ctx context.Context, in struct {
			Direction string `json:"direction"`
			Status    string `json:"status"`
			Month     string `json:"month"`
		}) (any, error) {
			var dir *domain.BillDirection
			switch normalize(in.Direction) {
			case "pagar":
				v := domain.BillPayable
				dir = &v
			case "receber":
				v := domain.BillReceivable
				dir = &v
			}
			status := normalize(in.Status)
			if status == "" {
				status = "abertas"
			}
			bills, err := d.Bills.List(ctx, dir, status == "abertas" || status == "atrasadas")
			if err != nil {
				return nil, err
			}
			var month *domain.YearMonth
			if strings.TrimSpace(in.Month) != "" {
				m, err := r.parseMonth(in.Month)
				if err != nil {
					return nil, err
				}
				month = &m
			}
			rows := []billRow{}
			var payable, receivable domain.Cents
			for _, b := range bills {
				st := b.Status(r.today())
				if status == "atrasadas" && st != "atrasado" {
					continue
				}
				if status == "pagas" && b.PaidAt == nil {
					continue
				}
				if month != nil && domain.YearMonthOf(b.DueDate) != *month {
					continue
				}
				rows = append(rows, r.billRow(b))
				if b.Direction == domain.BillPayable {
					payable += b.AmountCents
				} else {
					receivable += b.AmountCents
				}
			}
			out := map[string]any{"contas": rows, "total_a_pagar": money(payable), "total_a_receber": money(receivable)}
			if month != nil {
				out["mes"] = monthString(*month)
			}
			return out, nil
		}),
	})

	r.add(Tool{
		Name: "bills_summary", Title: "Resumo das contas", Module: "contas", ReadOnly: true,
		Description: "Total em aberto a pagar e a receber e quantas contas estão atrasadas.",
		Input:       object(map[string]any{}),
		run: typed(func(ctx context.Context, _ struct{}) (any, error) {
			s, err := d.Bills.Summary(ctx)
			if err != nil {
				return nil, err
			}
			return map[string]any{"a_pagar": money(s.PayableOpenCents), "a_receber": money(s.ReceivableOpenCents), "atrasadas": s.OverdueCount}, nil
		}),
	})

	r.add(Tool{
		Name: "bills_create", Title: "Nova conta", Module: "contas",
		Description: "Cadastra uma conta a pagar ou a receber.",
		Input: object(map[string]any{
			"description": str("O que é, ex.: \"Aluguel\""),
			"amount":      number("Valor em reais"),
			"due_date":    str("Vencimento AAAA-MM-DD"),
			"direction":   enum("pagar ou receber; padrão pagar", "pagar", "receber"),
			"category":    str("Categoria de gasto (opcional)"),
			"recurring":   boolean("Se repete todo mês"),
		}, "description", "amount", "due_date"),
		run: typed(func(ctx context.Context, in struct {
			Description string  `json:"description"`
			Amount      float64 `json:"amount"`
			DueDate     string  `json:"due_date"`
			Direction   string  `json:"direction"`
			Category    string  `json:"category"`
			Recurring   bool    `json:"recurring"`
		}) (any, error) {
			due, err := r.parseDay(in.DueDate)
			if err != nil {
				return nil, err
			}
			dir := domain.BillPayable
			if normalize(in.Direction) == "receber" {
				dir = domain.BillReceivable
			}
			input := service.NewBillInput{
				Description: strings.TrimSpace(in.Description),
				AmountCents: cents(in.Amount),
				DueDate:     due,
				Direction:   dir,
				Recurring:   in.Recurring,
			}
			if strings.TrimSpace(in.Category) != "" && d.Categories != nil {
				cat, err := r.resolveCategory(ctx, in.Category)
				if err != nil {
					return nil, err
				}
				input.CategoryID = &cat.ID
			}
			b, err := d.Bills.Create(ctx, input)
			if err != nil {
				return nil, err
			}
			return r.billRow(b), nil
		}),
	})

	r.add(Tool{
		Name: "bills_mark_paid", Title: "Marcar conta como recebida", Module: "contas",
		Description: "Marca uma conta A RECEBER como recebida, pelo id de bills_list. NÃO use para uma conta a pagar (ex.: \"já paguei a luz\") — uma conta a pagar só é quitada registrando a despesa; essa ferramenta recusa e não faz nada nesse caso.",
		Input: object(map[string]any{
			"id":      str("Id da conta"),
			"paid_on": str("Data do pagamento AAAA-MM-DD; padrão hoje"),
		}, "id"),
		run: typed(func(ctx context.Context, in struct {
			ID     string `json:"id"`
			PaidOn string `json:"paid_on"`
		}) (any, error) {
			day, err := r.parseDay(in.PaidOn)
			if err != nil {
				return nil, err
			}
			b, err := d.Bills.MarkPaid(ctx, in.ID, time.Date(day.Year(), day.Month(), day.Day(), 12, 0, 0, 0, r.deps.Location))
			if err != nil {
				return nil, err
			}
			return r.billRow(b), nil
		}),
	})

	r.add(Tool{
		Name: "bills_delete", Title: "Excluir conta", Module: "contas", Destructive: true,
		Description: "Exclui uma conta pelo id de bills_list. Se for a ocorrência mais recente de uma conta recorrente ativa, também encerra a repetição — os meses anteriores continuam no histórico.",
		Input:       object(map[string]any{"id": str("Id da conta")}, "id"),
		run: typed(func(ctx context.Context, in struct {
			ID string `json:"id"`
		}) (any, error) {
			return map[string]any{"ok": true}, d.Bills.Delete(ctx, in.ID)
		}),
	})
}
