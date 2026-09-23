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
	Ended       bool   `json:"repeticao_encerrada,omitempty"`
	Invoice     bool   `json:"fatura_do_cartao,omitempty"`
	Estimated   bool   `json:"valor_estimado,omitempty"`
	Payment     string `json:"pagamento,omitempty"`
}

func (r *Registry) billRow(b domain.Bill) billRow {
	row := billRow{
		ID: b.ID, Description: b.Description, Amount: money(b.AmountCents), Due: b.DueDate.Format(dayLayout),
		Direction: string(b.Direction), Status: b.Status(r.today()), Recurring: b.Recurring(),
		Ended: b.Recurring() && b.SeriesEnded, Invoice: b.IsInvoice(), Estimated: b.AmountEstimated,
	}
	if b.PaymentMethod != nil {
		row.Payment = string(*b.PaymentMethod)
	}
	return row
}

// billFields is what creating or editing a bill can set; on an edit, empty
// fields keep the bill's current values.
type billFields struct {
	Description   string  `json:"description"`
	Amount        float64 `json:"amount"`
	DueDate       string  `json:"due_date"`
	Direction     string  `json:"direction"`
	Category      string  `json:"category"`
	PaymentMethod string  `json:"payment_method"`
	CreditCard    string  `json:"credit_card"`
	AmountVaries  *bool   `json:"amount_varies"`
}

func billFieldProps() map[string]any {
	return map[string]any{
		"description":    str("O que é, ex.: \"Aluguel\""),
		"amount":         number("Valor em reais"),
		"due_date":       str("Vencimento AAAA-MM-DD"),
		"direction":      enum("pagar ou receber", "pagar", "receber"),
		"category":       str("Categoria (existente). Obrigatória numa conta a pagar recorrente"),
		"payment_method": enum("Como a conta costuma ser paga. Obrigatório numa conta a pagar recorrente", "pix", "debito", "credito"),
		"credit_card":    str("Cartão (nome), quando payment_method=credito"),
		"amount_varies":  boolean("O valor muda todo mês (ex.: luz): os próximos meses ficam como estimados"),
	}
}

// apply writes fields onto in, resolving names to ids.
func (r *Registry) applyBillFields(ctx context.Context, f billFields, in *service.NewBillInput) error {
	if v := strings.TrimSpace(f.Description); v != "" {
		in.Description = v
	}
	if f.Amount > 0 {
		in.AmountCents = cents(f.Amount)
		in.AmountEstimated = false
	}
	if strings.TrimSpace(f.DueDate) != "" {
		due, err := r.parseDay(f.DueDate)
		if err != nil {
			return err
		}
		in.DueDate = due
	}
	switch normalize(f.Direction) {
	case "pagar":
		in.Direction = domain.BillPayable
	case "receber":
		in.Direction = domain.BillReceivable
	}
	if strings.TrimSpace(f.Category) != "" && r.deps.Categories != nil {
		cat, err := r.resolveCategory(ctx, f.Category)
		if err != nil {
			return err
		}
		in.CategoryID = &cat.ID
	}
	if strings.TrimSpace(f.PaymentMethod) != "" {
		method, err := parsePaymentMethod(f.PaymentMethod, "")
		if err != nil {
			return err
		}
		in.PaymentMethod = &method
		if method != domain.PaymentCredit {
			in.CreditCardID = nil
		}
	}
	if in.PaymentMethod != nil && *in.PaymentMethod == domain.PaymentCredit && (strings.TrimSpace(f.CreditCard) != "" || in.CreditCardID == nil) {
		card, err := r.resolveCard(ctx, f.CreditCard)
		if err != nil {
			return err
		}
		in.CreditCardID = &card.ID
	}
	if f.AmountVaries != nil {
		in.AmountVaries = *f.AmountVaries
	}
	return nil
}

func (r *Registry) addBills() {
	d := r.deps
	if d.Bills == nil {
		return
	}

	r.add(Tool{
		Name: "bills_list", Title: "Contas a pagar e receber", Module: "contas", ReadOnly: true,
		Description: "Lista contas a pagar e/ou a receber, filtrando por status e por mês de vencimento. Inclui as faturas dos cartões (fatura_do_cartao: a soma das compras no crédito daquele mês) e as contas recorrentes do mês. Ex.: contas a pagar do mês que vem.",
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
			var month *domain.YearMonth
			var bills []domain.Bill
			var err error
			if strings.TrimSpace(in.Month) != "" {
				m, err := r.parseMonth(in.Month)
				if err != nil {
					return nil, err
				}
				month = &m
				// ListByMonth is what creates a month's recurring bills.
				bills, err = d.Bills.ListByMonth(ctx, m, dir)
				if err != nil {
					return nil, err
				}
			} else if bills, err = d.Bills.List(ctx, dir, status == "abertas" || status == "atrasadas"); err != nil {
				return nil, err
			}
			rows := []billRow{}
			var payable, receivable domain.Cents
			for _, b := range bills {
				st := b.Status(r.today())
				if (status == "abertas" || status == "atrasadas") && b.PaidAt != nil {
					continue
				}
				if status == "atrasadas" && st != "atrasado" {
					continue
				}
				if status == "pagas" && b.PaidAt == nil {
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
		Description: "Contas em aberto de um mês: total a pagar, a receber, o saldo (a receber − a pagar) e quantas estão atrasadas. No mês atual também conta as que ficaram em aberto de meses anteriores.",
		Input:       object(map[string]any{"month": str("Mês AAAA-MM, ou atual/proximo/passado; padrão atual")}),
		run: typed(func(ctx context.Context, in struct {
			Month string `json:"month"`
		}) (any, error) {
			ym, err := r.parseMonth(in.Month)
			if err != nil {
				return nil, err
			}
			if _, err := d.Bills.ListByMonth(ctx, ym, nil); err != nil {
				return nil, err
			}
			s, err := d.Bills.Summary(ctx, ym)
			if err != nil {
				return nil, err
			}
			return map[string]any{
				"mes":       monthString(ym),
				"a_pagar":   money(s.PayableOpenCents),
				"a_receber": money(s.ReceivableOpenCents),
				"saldo":     money(s.ReceivableOpenCents - s.PayableOpenCents),
				"atrasadas": s.OverdueCount,
			}, nil
		}),
	})

	createProps := billFieldProps()
	createProps["recurring"] = boolean("Se repete todo mês até ser encerrada; conta a pagar recorrente precisa de category e payment_method")
	r.add(Tool{
		Name: "bills_create", Title: "Nova conta", Module: "contas",
		Description: "Cadastra uma conta a pagar ou a receber, avulsa ou recorrente (repete todo mês até ser encerrada). Contas a pagar recorrentes contam como gasto fixo.",
		Input:       object(createProps, "description", "amount", "due_date"),
		run: typed(func(ctx context.Context, in struct {
			billFields
			Recurring bool `json:"recurring"`
		}) (any, error) {
			input := service.NewBillInput{Direction: domain.BillPayable, Recurring: in.Recurring}
			if err := r.applyBillFields(ctx, in.billFields, &input); err != nil {
				return nil, err
			}
			if input.DueDate.IsZero() {
				return nil, invalid("informe o vencimento")
			}
			input.AmountEstimated = false
			b, err := d.Bills.Create(ctx, input)
			if err != nil {
				return nil, err
			}
			return r.billRow(b), nil
		}),
	})

	r.add(Tool{
		Name: "bills_update", Title: "Editar conta", Module: "contas",
		Description: "Edita uma conta pelo id de bills_list: descrição, valor, vencimento, tipo, categoria, forma de pagamento, cartão e se o valor varia. Campos não enviados ficam como estão. A fatura do cartão não se edita: ela é a soma das compras.",
		Input:       object(func() map[string]any { p := billFieldProps(); p["id"] = str("Id da conta"); return p }(), "id"),
		run: typed(func(ctx context.Context, in struct {
			ID string `json:"id"`
			billFields
		}) (any, error) {
			b, err := d.Bills.Get(ctx, in.ID)
			if err != nil {
				return nil, err
			}
			input := service.NewBillInput{
				Description: b.Description, AmountCents: b.AmountCents, DueDate: b.DueDate, Direction: b.Direction,
				CategoryID: b.CategoryID, AmountEstimated: b.AmountEstimated, AmountVaries: b.AmountVaries,
				PaymentMethod: b.PaymentMethod, CreditCardID: b.CreditCardID,
			}
			if err := r.applyBillFields(ctx, in.billFields, &input); err != nil {
				return nil, err
			}
			updated, err := d.Bills.Update(ctx, in.ID, input)
			if err != nil {
				return nil, err
			}
			return r.billRow(updated), nil
		}),
	})

	r.add(Tool{
		Name: "bills_pay", Title: "Pagar conta", Module: "contas",
		Description: "Quita uma conta A PAGAR pelo id de bills_list (ex.: \"já paguei a luz\") e registra a despesa no mês do pagamento. Valor, categoria e forma de pagamento vêm da conta quando não informados. Numa fatura do cartão, só marca como paga: as compras já são as despesas. Numa conta a receber, marca como recebida.",
		Input: object(map[string]any{
			"id":             str("Id da conta"),
			"paid_on":        str("Data do pagamento AAAA-MM-DD; padrão hoje"),
			"amount":         number("Valor pago em reais, se diferente do da conta"),
			"category":       str("Categoria da despesa, se a conta não tiver"),
			"payment_method": enum("Como pagou, se diferente do da conta", "pix", "debito", "credito"),
			"credit_card":    str("Cartão, quando pagou no crédito"),
		}, "id"),
		run: typed(func(ctx context.Context, in struct {
			ID            string  `json:"id"`
			PaidOn        string  `json:"paid_on"`
			Amount        float64 `json:"amount"`
			Category      string  `json:"category"`
			PaymentMethod string  `json:"payment_method"`
			CreditCard    string  `json:"credit_card"`
		}) (any, error) {
			paidAt, err := r.paidAt(in.PaidOn)
			if err != nil {
				return nil, err
			}
			b, err := d.Bills.Get(ctx, in.ID)
			if err != nil {
				return nil, err
			}
			if b.Direction != domain.BillPayable || b.IsInvoice() {
				paid, err := d.Bills.MarkPaid(ctx, in.ID, paidAt)
				if err != nil {
					return nil, err
				}
				return r.billRow(paid), nil
			}
			pay := service.PaymentInput{PaidAt: paidAt, AmountCents: b.AmountCents}
			if in.Amount > 0 {
				pay.AmountCents = cents(in.Amount)
			}
			if b.CategoryID != nil {
				pay.CategoryID = *b.CategoryID
			}
			if strings.TrimSpace(in.Category) != "" {
				cat, err := r.resolveCategory(ctx, in.Category)
				if err != nil {
					return nil, err
				}
				pay.CategoryID = cat.ID
			}
			if pay.CategoryID == "" {
				return nil, invalid("a conta não tem categoria; informe category")
			}
			fallback := domain.PaymentPix
			if b.PaymentMethod != nil {
				fallback = *b.PaymentMethod
			}
			if pay.PaymentMethod, err = parsePaymentMethod(in.PaymentMethod, fallback); err != nil {
				return nil, err
			}
			if pay.PaymentMethod == domain.PaymentCredit {
				pay.CreditCardID = b.CreditCardID
				if strings.TrimSpace(in.CreditCard) != "" || pay.CreditCardID == nil {
					card, err := r.resolveCard(ctx, in.CreditCard)
					if err != nil {
						return nil, err
					}
					pay.CreditCardID = &card.ID
				}
			}
			paid, _, err := d.Bills.Pay(ctx, in.ID, pay)
			if err != nil {
				return nil, err
			}
			return r.billRow(paid), nil
		}),
	})

	r.add(Tool{
		Name: "bills_mark_received", Title: "Marcar conta como recebida", Module: "contas",
		Description: "Marca uma conta A RECEBER como recebida, pelo id de bills_list; entra nas entradas do mês. Para conta a pagar use bills_pay.",
		Input: object(map[string]any{
			"id":      str("Id da conta"),
			"paid_on": str("Data do recebimento AAAA-MM-DD; padrão hoje"),
		}, "id"),
		run: typed(func(ctx context.Context, in struct {
			ID     string `json:"id"`
			PaidOn string `json:"paid_on"`
		}) (any, error) {
			paidAt, err := r.paidAt(in.PaidOn)
			if err != nil {
				return nil, err
			}
			b, err := d.Bills.MarkPaid(ctx, in.ID, paidAt)
			if err != nil {
				return nil, err
			}
			return r.billRow(b), nil
		}),
	})

	r.add(Tool{
		Name: "bills_unpay", Title: "Desfazer pagamento", Module: "contas",
		Description: "Desfaz o pagamento ou recebimento de uma conta pelo id: ela volta a ficar em aberto e a despesa criada pelo pagamento é apagada.",
		Input:       object(map[string]any{"id": str("Id da conta")}, "id"),
		run: typed(func(ctx context.Context, in struct {
			ID string `json:"id"`
		}) (any, error) {
			b, err := d.Bills.Unpay(ctx, in.ID)
			if err != nil {
				return nil, err
			}
			return r.billRow(b), nil
		}),
	})

	r.add(Tool{
		Name: "bills_end_series", Title: "Encerrar repetição", Module: "contas",
		Description: "Encerra a repetição de uma conta recorrente pelo id de uma das ocorrências: não cria mais meses depois dela. Com delete_following, também exclui as ocorrências seguintes ainda não pagas (as pagas ficam).",
		Input: object(map[string]any{
			"id":               str("Id da ocorrência a partir da qual encerrar"),
			"delete_following": boolean("Excluir também as ocorrências seguintes não pagas; confirme com a pessoa antes"),
		}, "id"),
		run: typed(func(ctx context.Context, in struct {
			ID              string `json:"id"`
			DeleteFollowing bool   `json:"delete_following"`
		}) (any, error) {
			b, err := d.Bills.EndSeries(ctx, in.ID, in.DeleteFollowing)
			if err != nil {
				return nil, err
			}
			return r.billRow(b), nil
		}),
	})

	r.add(Tool{
		Name: "bills_resume_series", Title: "Retomar repetição", Module: "contas",
		Description: "Retoma a repetição de uma conta recorrente encerrada, pelo id de uma ocorrência.",
		Input:       object(map[string]any{"id": str("Id da conta")}, "id"),
		run: typed(func(ctx context.Context, in struct {
			ID string `json:"id"`
		}) (any, error) {
			b, err := d.Bills.ResumeSeries(ctx, in.ID)
			if err != nil {
				return nil, err
			}
			return r.billRow(b), nil
		}),
	})

	r.add(Tool{
		Name: "bills_delete", Title: "Excluir conta", Module: "contas", Destructive: true,
		Description: "Exclui uma conta pelo id de bills_list. Se for a ocorrência mais recente de uma conta recorrente ativa, também encerra a repetição — os meses anteriores continuam no histórico. A fatura do cartão não se exclui: exclua as compras.",
		Input:       object(map[string]any{"id": str("Id da conta")}, "id"),
		run: typed(func(ctx context.Context, in struct {
			ID string `json:"id"`
		}) (any, error) {
			return map[string]any{"ok": true}, d.Bills.Delete(ctx, in.ID)
		}),
	})
}

// paidAt reads a payment day in the owner's timezone, at noon so the day
// survives any timezone conversion.
func (r *Registry) paidAt(raw string) (time.Time, error) {
	day, err := r.parseDay(raw)
	if err != nil {
		return time.Time{}, err
	}
	return time.Date(day.Year(), day.Month(), day.Day(), 12, 0, 0, 0, r.deps.Location), nil
}
