package assistant

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/rafael/estus-vault/backend/internal/domain"
	"github.com/rafael/estus-vault/backend/internal/service"
)

type txnInput struct {
	Description    string  `json:"description"`
	Amount         float64 `json:"amount"`
	Category       string  `json:"category"`
	CategoryNature string  `json:"category_nature"`
	PaymentMethod  string  `json:"payment_method"`
	Date           string  `json:"date"`
	CreditCard     string  `json:"credit_card"`
	Installments   int     `json:"installments"`
	Recurring      bool    `json:"recurring"`
}

var txnProps = func() map[string]any {
	return map[string]any{
		"description":     str("O que foi, ex.: \"Mercado\""),
		"amount":          number("Valor total em reais, ex.: 45.9"),
		"category":        str("Nome da categoria. Prefira uma existente (finance_list_categories); um nome novo é criado sozinho. Vazio: eu escolho pela descrição"),
		"category_nature": enum("Natureza de uma categoria nova; padrão variavel", "essencial", "variavel", "investimento"),
		"payment_method":  enum("Forma de pagamento; padrão pix", "pix", "debito", "credito"),
		"date":            str("Data da compra AAAA-MM-DD, ou hoje/ontem; padrão hoje"),
		"credit_card":     str("Nome do cartão, obrigatório quando payment_method=credito"),
		"installments":    integer("Número de parcelas no crédito; padrão 1"),
		"recurring":       boolean("Gasto fixo que se repete todo mês"),
	}
}

type txnResult struct {
	Description     string `json:"descricao"`
	Amount          string `json:"valor"`
	Category        string `json:"categoria"`
	CategoryCreated bool   `json:"categoria_criada,omitempty"`
	Payment         string `json:"pagamento"`
	Date            string `json:"data"`
	Month           string `json:"mes_competencia"`
	Installments    int    `json:"parcelas"`
}

func (r *Registry) resolveCategory(ctx context.Context, query string) (domain.Category, error) {
	cats, err := r.deps.Categories.List(ctx)
	if err != nil {
		return domain.Category{}, err
	}
	if strings.TrimSpace(query) == "" {
		return domain.Category{}, invalid("informe a categoria; existentes: %s", joinNames(cats, func(c domain.Category) string { return c.Name }))
	}
	c, ok, names := match(cats, query, func(c domain.Category) string { return c.ID }, func(c domain.Category) string { return c.Name })
	if !ok {
		return domain.Category{}, invalid("categoria %q não encontrada; existentes: %s", query, strings.Join(names, ", "))
	}
	return c, nil
}

// categoryFor finds the category an expense goes in, creating one when the
// owner named something that doesn't exist yet, or when nothing on file fits
// what was bought. The second result says whether it had to create it.
func (r *Registry) categoryFor(ctx context.Context, query, description, nature string) (domain.Category, bool, error) {
	cats, err := r.deps.Categories.List(ctx)
	if err != nil {
		return domain.Category{}, false, err
	}
	id := func(c domain.Category) string { return c.ID }
	name := func(c domain.Category) string { return c.Name }
	if q := strings.TrimSpace(query); q != "" {
		if c, ok, _ := match(cats, q, id, name); ok {
			return c, false, nil
		}
		c, err := r.createCategory(ctx, cats, q, nature)
		return c, err == nil, err
	}
	if c, ok := guessCategory(description, cats); ok {
		return c, false, nil
	}
	c, err := r.createCategory(ctx, cats, description, nature)
	return c, err == nil, err
}

// createCategory adds a category for an expense that had nowhere to go.
func (r *Registry) createCategory(ctx context.Context, existing []domain.Category, name, nature string) (domain.Category, error) {
	c := domain.Category{
		Name:   categoryName(name),
		Nature: domain.NatureDiscretionary,
		Color:  nextCategoryColor(existing),
	}
	if n := domain.CategoryNature(normalize(nature)); n.Valid() {
		c.Nature = n
	}
	if c.Name == "" {
		return domain.Category{}, invalid("informe a categoria; existentes: %s", joinNames(existing, func(c domain.Category) string { return c.Name }))
	}
	created, err := r.deps.Categories.Create(ctx, c)
	if errors.Is(err, domain.ErrConflict) {
		// Created in the meantime (another tool call, or the web): use that one.
		cats, listErr := r.deps.Categories.List(ctx)
		if listErr != nil {
			return domain.Category{}, listErr
		}
		found, ok, _ := match(cats, c.Name, func(c domain.Category) string { return c.ID }, func(c domain.Category) string { return c.Name })
		if !ok {
			return domain.Category{}, err
		}
		return found, nil
	}
	if err != nil {
		return domain.Category{}, err
	}
	return created, nil
}

func joinNames[T any](items []T, name func(T) string) string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = name(it)
	}
	return strings.Join(out, ", ")
}

func (r *Registry) createTransaction(ctx context.Context, in txnInput) (txnResult, error) {
	if strings.TrimSpace(in.Description) == "" {
		return txnResult{}, invalid("informe a descrição")
	}
	if in.Amount <= 0 {
		return txnResult{}, invalid("o valor precisa ser maior que zero")
	}
	cat, createdCategory, err := r.categoryFor(ctx, in.Category, in.Description, in.CategoryNature)
	if err != nil {
		return txnResult{}, err
	}
	method := domain.PaymentMethod(normalize(in.PaymentMethod))
	if method == "" {
		method = domain.PaymentPix
	}
	if method != domain.PaymentPix && method != domain.PaymentDebit && method != domain.PaymentCredit {
		return txnResult{}, invalid("forma de pagamento %q inválida: use pix, debito ou credito", in.PaymentMethod)
	}
	date, err := r.parseDay(in.Date)
	if err != nil {
		return txnResult{}, err
	}
	input := service.NewTransactionInput{
		Description:   strings.TrimSpace(in.Description),
		AmountCents:   cents(in.Amount),
		CategoryID:    cat.ID,
		PaymentMethod: method,
		PurchaseDate:  date,
		Installments:  max(in.Installments, 1),
		IsRecurring:   in.Recurring,
	}
	if method == domain.PaymentCredit {
		cards, err := r.deps.Cards.List(ctx)
		if err != nil {
			return txnResult{}, err
		}
		card, ok, names := match(cards, in.CreditCard, func(c domain.CreditCard) string { return c.ID }, func(c domain.CreditCard) string { return c.Name })
		if !ok && len(cards) == 1 && strings.TrimSpace(in.CreditCard) == "" {
			card, ok = cards[0], true
		}
		if !ok {
			return txnResult{}, invalid("cartão %q não encontrado; cartões: %s", in.CreditCard, strings.Join(names, ", "))
		}
		input.CreditCardID = card.ID
	}
	txns, err := r.deps.Transactions.Create(ctx, input)
	if err != nil {
		return txnResult{}, err
	}
	return txnResult{
		Description:     input.Description,
		Amount:          money(input.AmountCents),
		Category:        cat.Name,
		CategoryCreated: createdCategory,
		Payment:         string(method),
		Date:            date.Format(dayLayout),
		Month:           monthString(txns[0].CompetenceMonth),
		Installments:    len(txns),
	}, nil
}

func (r *Registry) addFinance() {
	d := r.deps
	if d.Transactions == nil || d.Categories == nil {
		return
	}

	r.add(Tool{
		Name: "finance_list_categories", Title: "Categorias de gasto", Module: "financeiro", ReadOnly: true,
		Description: "Lista as categorias de gasto, com orçamento mensal quando houver.",
		Input:       object(map[string]any{}),
		run: typed(func(ctx context.Context, _ struct{}) (any, error) {
			cats, err := d.Categories.List(ctx)
			if err != nil {
				return nil, err
			}
			type row struct {
				Name   string `json:"nome"`
				Nature string `json:"natureza"`
				Budget string `json:"orcamento,omitempty"`
			}
			out := make([]row, len(cats))
			for i, c := range cats {
				out[i] = row{Name: c.Name, Nature: string(c.Nature)}
				if c.MonthlyBudgetCents != nil {
					out[i].Budget = money(*c.MonthlyBudgetCents)
				}
			}
			return map[string]any{"categorias": out}, nil
		}),
	})

	r.add(Tool{
		Name: "finance_list_credit_cards", Title: "Cartões de crédito", Module: "financeiro", ReadOnly: true,
		Description: "Lista os cartões de crédito com dia de fechamento e vencimento.",
		Input:       object(map[string]any{}),
		run: typed(func(ctx context.Context, _ struct{}) (any, error) {
			cards, err := d.Cards.List(ctx)
			if err != nil {
				return nil, err
			}
			type row struct {
				Name    string `json:"nome"`
				Closing int    `json:"fecha_dia"`
				Due     int    `json:"vence_dia"`
			}
			out := make([]row, len(cards))
			for i, c := range cards {
				out[i] = row{c.Name, c.ClosingDay, c.DueDay}
			}
			return map[string]any{"cartoes": out}, nil
		}),
	})

	r.add(Tool{
		Name: "finance_create_transaction", Title: "Lançar gasto", Module: "financeiro",
		Description: "Registra um gasto. A categoria pode ser uma existente, um nome novo (criado na hora) ou vazia (escolhida pela descrição). No crédito, a compra entra no mês em que a fatura do cartão vence — calculado a partir do dia de fechamento e do dia de vencimento do cartão, não simplesmente no mês seguinte — e pode ser parcelada.",
		Input:       object(txnProps(), "description", "amount"),
		run: typed(func(ctx context.Context, in txnInput) (any, error) {
			return r.createTransaction(ctx, in)
		}),
	})

	r.add(Tool{
		Name: "finance_create_transactions", Title: "Lançar vários gastos", Module: "financeiro",
		Description: "Registra vários gastos de uma vez. Cada item tem os mesmos campos de finance_create_transaction.",
		Input: object(map[string]any{
			"items": array("Os gastos a lançar", object(txnProps(), "description", "amount")),
		}, "items"),
		run: typed(func(ctx context.Context, in struct {
			Items []txnInput `json:"items"`
		}) (any, error) {
			if len(in.Items) == 0 {
				return nil, invalid("nenhum gasto informado")
			}
			if len(in.Items) > 50 {
				return nil, invalid("no máximo 50 gastos por vez")
			}
			var done []txnResult
			var failed []string
			var total domain.Cents
			for i, item := range in.Items {
				res, err := r.createTransaction(ctx, item)
				if err != nil {
					failed = append(failed, fmt.Sprintf("item %d (%s): %v", i+1, item.Description, err))
					continue
				}
				total += cents(item.Amount)
				done = append(done, res)
			}
			return map[string]any{"lancados": done, "total": money(total), "falhas": failed}, nil
		}),
	})

	r.add(Tool{
		Name: "finance_month_summary", Title: "Resumo do mês", Module: "financeiro", ReadOnly: true,
		Description: "Quanto foi gasto num mês: total, comparação com o mês anterior e gasto por categoria com orçamento.",
		Input:       object(map[string]any{"month": str("Mês AAAA-MM, ou atual/passado/proximo; padrão atual")}),
		run: typed(func(ctx context.Context, in struct {
			Month string `json:"month"`
		}) (any, error) {
			ym, err := r.parseMonth(in.Month)
			if err != nil {
				return nil, err
			}
			sum, err := d.Dashboard.MonthSummary(ctx, ym)
			if err != nil {
				return nil, err
			}
			type cat struct {
				Name   string `json:"categoria"`
				Total  string `json:"total"`
				Budget string `json:"orcamento,omitempty"`
				Over   bool   `json:"estourou,omitempty"`
			}
			cats := []cat{}
			for _, c := range sum.Categories {
				if c.TotalCents == 0 && c.MonthlyBudgetCents == nil {
					continue
				}
				row := cat{Name: c.Name, Total: money(c.TotalCents)}
				if c.MonthlyBudgetCents != nil {
					row.Budget = money(*c.MonthlyBudgetCents)
					row.Over = c.TotalCents > *c.MonthlyBudgetCents
				}
				cats = append(cats, row)
			}
			return map[string]any{
				"mes":             monthString(ym),
				"total":           money(sum.TotalCents),
				"total_centavos":  int64(sum.TotalCents),
				"mes_anterior":    money(sum.PreviousMonthCents),
				"variacao":        money(sum.TotalCents - sum.PreviousMonthCents),
				"fixos":           money(sum.RecurringCents),
				"variaveis":       money(sum.VariableCents),
				"por_categoria":   cats,
				"num_lancamentos": len(sum.Transactions),
			}, nil
		}),
	})

	r.add(Tool{
		Name: "finance_list_transactions", Title: "Lançamentos do mês", Module: "financeiro", ReadOnly: true,
		Description: "Lista os lançamentos de um mês, com filtro opcional por categoria ou texto. Traz o id para editar ou excluir.",
		Input: object(map[string]any{
			"month":    str("Mês AAAA-MM, ou atual/passado; padrão atual"),
			"category": str("Filtrar por nome de categoria"),
			"search":   str("Filtrar por texto na descrição"),
			"limit":    integer("Máximo de itens; padrão 30"),
		}),
		run: typed(func(ctx context.Context, in struct {
			Month    string `json:"month"`
			Category string `json:"category"`
			Search   string `json:"search"`
			Limit    int    `json:"limit"`
		}) (any, error) {
			ym, err := r.parseMonth(in.Month)
			if err != nil {
				return nil, err
			}
			rows, err := d.TransactionLog.ListByCompetenceMonth(ctx, ym)
			if err != nil {
				return nil, err
			}
			limit := in.Limit
			if limit <= 0 || limit > 200 {
				limit = 30
			}
			type row struct {
				ID          string `json:"id"`
				Date        string `json:"data"`
				Description string `json:"descricao"`
				Amount      string `json:"valor"`
				Category    string `json:"categoria"`
				Payment     string `json:"pagamento"`
				Installment string `json:"parcela,omitempty"`
			}
			out := []row{}
			var total domain.Cents
			for _, t := range rows {
				if in.Category != "" && !strings.Contains(normalize(t.CategoryName), normalize(in.Category)) {
					continue
				}
				if in.Search != "" && !strings.Contains(normalize(t.Description), normalize(in.Search)) {
					continue
				}
				total += t.AmountCents
				if len(out) >= limit {
					continue
				}
				item := row{t.ID, t.PurchaseDate.Format(dayLayout), t.Description, money(t.AmountCents), t.CategoryName, string(t.PaymentMethod), ""}
				if t.InstallmentTotal > 1 {
					item.Installment = fmt.Sprintf("%d/%d", t.InstallmentNumber, t.InstallmentTotal)
				}
				out = append(out, item)
			}
			return map[string]any{"mes": monthString(ym), "lancamentos": out, "total_filtrado": money(total)}, nil
		}),
	})

	r.add(Tool{
		Name: "finance_update_transaction", Title: "Editar lançamento", Module: "financeiro",
		Description: "Muda a descrição e/ou a categoria de um lançamento (pelo id de finance_list_transactions).",
		Input: object(map[string]any{
			"id":          str("Id do lançamento"),
			"description": str("Nova descrição (vazio mantém)"),
			"category":    str("Nova categoria (vazio mantém)"),
		}, "id"),
		run: typed(func(ctx context.Context, in struct {
			ID          string `json:"id"`
			Description string `json:"description"`
			Category    string `json:"category"`
		}) (any, error) {
			current, err := r.findTransaction(ctx, in.ID)
			if err != nil {
				return nil, err
			}
			desc, catID := current.Description, current.CategoryID
			if strings.TrimSpace(in.Description) != "" {
				desc = strings.TrimSpace(in.Description)
			}
			if strings.TrimSpace(in.Category) != "" {
				cat, err := r.resolveCategory(ctx, in.Category)
				if err != nil {
					return nil, err
				}
				catID = cat.ID
			}
			if err := d.Transactions.Update(ctx, in.ID, desc, catID); err != nil {
				return nil, err
			}
			return map[string]any{"ok": true, "descricao": desc}, nil
		}),
	})

	r.add(Tool{
		Name: "finance_delete_transaction", Title: "Excluir lançamento", Module: "financeiro", Destructive: true,
		Description: "Exclui um lançamento pelo id (de finance_list_transactions). Numa compra parcelada, exclui só aquela parcela.",
		Input:       object(map[string]any{"id": str("Id do lançamento")}, "id"),
		run: typed(func(ctx context.Context, in struct {
			ID string `json:"id"`
		}) (any, error) {
			if err := d.Transactions.Delete(ctx, in.ID); err != nil {
				return nil, err
			}
			return map[string]any{"ok": true}, nil
		}),
	})
}

// findTransaction looks a transaction up by id across the recent months the
// ledger is browsed in.
func (r *Registry) findTransaction(ctx context.Context, id string) (domain.Transaction, error) {
	month := domain.YearMonthOf(r.now())
	for i := -12; i <= 13; i++ {
		rows, err := r.deps.TransactionLog.ListByCompetenceMonth(ctx, month.Add(-i))
		if err != nil {
			return domain.Transaction{}, err
		}
		for _, t := range rows {
			if t.ID == id {
				return t.Transaction, nil
			}
		}
	}
	return domain.Transaction{}, fmt.Errorf("transaction %s: %w", id, domain.ErrNotFound)
}
