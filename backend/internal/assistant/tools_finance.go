package assistant

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/rafael/estus-vault/backend/internal/domain"
	"github.com/rafael/estus-vault/backend/internal/service"
	"github.com/rafael/estus-vault/backend/internal/store/postgres"
)

type txnInput struct {
	Description    string  `json:"description"`
	Amount         float64 `json:"amount"`
	PerInstallment bool    `json:"per_installment"`
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
		"amount":          number("Valor em reais: o total da compra, ou o de cada parcela com per_installment"),
		"per_installment": boolean("true quando amount é o valor de CADA parcela (ex.: 12x de 40 → amount 40, installments 12)"),
		"category":        str("Nome da categoria. Prefira uma existente (finance_list_categories); um nome novo é criado sozinho. Vazio: eu escolho pela descrição"),
		"category_nature": enum("Natureza de uma categoria nova; padrão variavel", "essencial", "variavel", "investimento"),
		"payment_method":  enum("Forma de pagamento; padrão pix", "pix", "debito", "credito"),
		"date":            str("Data da compra AAAA-MM-DD, ou hoje/ontem; padrão hoje"),
		"credit_card":     str("Nome do cartão, obrigatório quando payment_method=credito"),
		"installments":    integer("Número de parcelas no crédito; padrão 1. Cada parcela aparece no seu mês e entra na fatura daquele mês — não use recurring junto"),
		"recurring":       boolean("Gasto fixo que se repete todo mês (assinatura, aluguel); não vale para compra parcelada"),
	}
}

type txnResult struct {
	Description     string `json:"descricao"`
	Amount          string `json:"valor"`
	Category        string `json:"categoria"`
	CategoryCreated bool   `json:"categoria_criada,omitempty"`
	Payment         string `json:"pagamento"`
	Date            string `json:"data"`
	Month           string `json:"mes"`
	Invoice         string `json:"fatura,omitempty"`
	Installments    int    `json:"parcelas"`
	EachInstallment string `json:"valor_parcela,omitempty"`
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
		Kind:   domain.KindExpense,
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

// resolveCard finds a card by name; with a single card on file, an empty
// name means that one.
func (r *Registry) resolveCard(ctx context.Context, query string) (domain.CreditCard, error) {
	cards, err := r.deps.Cards.List(ctx)
	if err != nil {
		return domain.CreditCard{}, err
	}
	card, ok, names := match(cards, query, func(c domain.CreditCard) string { return c.ID }, func(c domain.CreditCard) string { return c.Name })
	if !ok && len(cards) == 1 && strings.TrimSpace(query) == "" {
		return cards[0], nil
	}
	if !ok {
		return domain.CreditCard{}, invalid("cartão %q não encontrado; cartões: %s", query, strings.Join(names, ", "))
	}
	return card, nil
}

func parsePaymentMethod(raw string, fallback domain.PaymentMethod) (domain.PaymentMethod, error) {
	method := domain.PaymentMethod(normalize(raw))
	if method == "" {
		return fallback, nil
	}
	if !method.Valid() {
		return "", invalid("forma de pagamento %q inválida: use pix, debito ou credito", raw)
	}
	return method, nil
}

// purchaseCents is the whole purchase: amount as given, or one installment
// times the number of installments.
func purchaseCents(amount float64, perInstallment bool, installments int) domain.Cents {
	if perInstallment {
		return cents(amount) * domain.Cents(max(installments, 1))
	}
	return cents(amount)
}

func txnResultFor(input service.NewTransactionInput, cat domain.Category, created bool, txns []domain.Transaction) txnResult {
	res := txnResult{
		Description:     input.Description,
		Amount:          money(input.AmountCents),
		Category:        cat.Name,
		CategoryCreated: created,
		Payment:         string(input.PaymentMethod),
		Date:            input.PurchaseDate.Format(dayLayout),
		Month:           monthString(txns[0].CompetenceMonth),
		Installments:    len(txns),
	}
	if txns[0].InvoiceMonth != nil {
		res.Invoice = monthString(*txns[0].InvoiceMonth)
	}
	if len(txns) > 1 {
		res.EachInstallment = money(txns[0].AmountCents)
	}
	return res
}

func (r *Registry) createTransaction(ctx context.Context, in txnInput) (txnResult, error) {
	if strings.TrimSpace(in.Description) == "" {
		return txnResult{}, invalid("informe a descrição")
	}
	total := purchaseCents(in.Amount, in.PerInstallment, in.Installments)
	if total <= 0 {
		return txnResult{}, invalid("o valor precisa ser maior que zero")
	}
	cat, createdCategory, err := r.categoryFor(ctx, in.Category, in.Description, in.CategoryNature)
	if err != nil {
		return txnResult{}, err
	}
	method, err := parsePaymentMethod(in.PaymentMethod, domain.PaymentPix)
	if err != nil {
		return txnResult{}, err
	}
	date, err := r.parseDay(in.Date)
	if err != nil {
		return txnResult{}, err
	}
	input := service.NewTransactionInput{
		Description:   strings.TrimSpace(in.Description),
		AmountCents:   total,
		CategoryID:    cat.ID,
		PaymentMethod: method,
		PurchaseDate:  date,
		Installments:  max(in.Installments, 1),
		IsRecurring:   in.Recurring && in.Installments <= 1,
	}
	if method == domain.PaymentCredit {
		card, err := r.resolveCard(ctx, in.CreditCard)
		if err != nil {
			return txnResult{}, err
		}
		input.CreditCardID = card.ID
	} else {
		input.Installments = 1
	}
	txns, err := r.deps.Transactions.Create(ctx, input)
	if err != nil {
		return txnResult{}, err
	}
	return txnResultFor(input, cat, createdCategory, txns), nil
}

func (r *Registry) addFinance() {
	d := r.deps
	if d.Transactions == nil || d.Categories == nil {
		return
	}

	r.add(Tool{
		Name: "finance_list_categories", Title: "Categorias de gasto", Module: "financeiro", ReadOnly: true,
		Description: "Lista as categorias com tipo (despesa ou receita), natureza e orçamento mensal. Só despesas contam como gasto; receitas contam como entrada.",
		Input:       object(map[string]any{}),
		run: typed(func(ctx context.Context, _ struct{}) (any, error) {
			cats, err := d.Categories.List(ctx)
			if err != nil {
				return nil, err
			}
			type row struct {
				Name   string `json:"nome"`
				Kind   string `json:"tipo"`
				Nature string `json:"natureza"`
				Budget string `json:"orcamento,omitempty"`
			}
			out := make([]row, len(cats))
			for i, c := range cats {
				out[i] = row{Name: c.Name, Kind: string(c.Kind), Nature: string(c.Nature)}
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
		Description: "Registra um lançamento (gasto; numa categoria de receita, conta como entrada). A categoria pode ser uma existente, um nome novo (criado na hora) ou vazia (escolhida pela descrição). O lançamento aparece no mês da compra. No crédito ele também entra na fatura do cartão (pelo dia de fechamento e vencimento), que vira uma conta a pagar; o dinheiro só sai quando a fatura é paga. Parcelado: informe installments e o total em amount, ou o valor da parcela em amount com per_installment=true — não use recurring.",
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
				total += purchaseCents(item.Amount, item.PerInstallment, item.Installments)
				done = append(done, res)
			}
			return map[string]any{"lancados": done, "total": money(total), "falhas": failed}, nil
		}),
	})

	r.add(Tool{
		Name: "finance_month_summary", Title: "Resumo do mês", Module: "financeiro", ReadOnly: true,
		Description: "Resumo financeiro de um mês. gasto = despesas lançadas no mês (inclui crédito). entradas = contas recebidas + lançamentos em categoria de receita. saidas = dinheiro que saiu de fato (débito, pix e faturas de cartão pagas; compra no crédito só sai quando a fatura é paga). saldo = entradas − saidas. fixos = recorrentes e parcelas (até a última) + contas recorrentes ainda a pagar; o resto é variável. Traz também gasto por categoria com orçamento e contas em aberto do mês.",
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
			out := map[string]any{
				"mes":             monthString(ym),
				"gasto":           money(sum.TotalCents),
				"gasto_centavos":  int64(sum.TotalCents),
				"mes_anterior":    money(sum.PreviousMonthCents),
				"variacao":        money(sum.TotalCents - sum.PreviousMonthCents),
				"saidas":          money(sum.PaidOutCents),
				"fixos_pagos":     money(sum.RecurringCents),
				"fixos_a_pagar":   money(sum.OpenFixedCents),
				"variaveis":       money(sum.VariableCents),
				"por_categoria":   cats,
				"num_lancamentos": len(sum.Transactions),
			}
			fixed := sum.RecurringCents + sum.OpenFixedCents
			out["fixos"] = money(fixed)
			if all := fixed + sum.VariableCents; all > 0 {
				pct := int((fixed*100 + all/2) / all)
				out["pct_fixos"] = pct
				out["pct_variaveis"] = 100 - pct
			}
			if d.Bills != nil {
				received, err := d.Bills.ReceivedTotal(ctx, ym)
				if err != nil {
					return nil, err
				}
				out["entradas"] = money(received)
				out["saldo"] = money(received - sum.PaidOutCents)
				bs, err := d.Bills.Summary(ctx, ym)
				if err != nil {
					return nil, err
				}
				out["contas_a_pagar_em_aberto"] = money(bs.PayableOpenCents)
				out["contas_a_receber_em_aberto"] = money(bs.ReceivableOpenCents)
				out["saldo_das_contas"] = money(bs.ReceivableOpenCents - bs.PayableOpenCents)
			}
			return out, nil
		}),
	})

	r.add(Tool{
		Name: "finance_list_transactions", Title: "Lançamentos do mês", Module: "financeiro", ReadOnly: true,
		Description: "Lista os lançamentos de um mês (pelo mês da compra; cada parcela no seu mês), com filtro opcional por categoria ou texto. Traz o id para editar ou excluir, a fatura em que a compra no crédito entra e o total da compra parcelada.",
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
				Income      bool   `json:"receita,omitempty"`
				Payment     string `json:"pagamento"`
				Card        string `json:"cartao,omitempty"`
				Invoice     string `json:"fatura,omitempty"`
				Installment string `json:"parcela,omitempty"`
				Purchase    string `json:"total_da_compra,omitempty"`
				Recurring   bool   `json:"recorrente,omitempty"`
			}
			cardNames := map[string]string{}
			if cards, err := d.Cards.List(ctx); err == nil {
				for _, c := range cards {
					cardNames[c.ID] = c.Name
				}
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
				item := row{
					ID: t.ID, Date: t.PurchaseDate.Format(dayLayout), Description: t.Description,
					Amount: money(t.AmountCents), Category: t.CategoryName, Income: t.CategoryKind == domain.KindIncome,
					Payment: string(t.PaymentMethod), Recurring: t.IsRecurring,
				}
				if t.CreditCardID != nil {
					item.Card = cardNames[*t.CreditCardID]
				}
				if t.InvoiceMonth != nil {
					item.Invoice = monthString(*t.InvoiceMonth)
				}
				if t.InstallmentTotal > 1 {
					item.Installment = fmt.Sprintf("%d/%d", t.InstallmentNumber, t.InstallmentTotal)
					item.Purchase = money(t.PurchaseTotalCents)
				}
				out = append(out, item)
			}
			return map[string]any{"mes": monthString(ym), "lancamentos": out, "total_filtrado": money(total)}, nil
		}),
	})

	r.add(Tool{
		Name: "finance_update_transaction", Title: "Editar lançamento", Module: "financeiro",
		Description: "Edita um lançamento por completo (pelo id de finance_list_transactions): descrição, categoria, valor, data, forma de pagamento, cartão, parcelas e recorrência. Campos não enviados ficam como estão. Numa compra parcelada, a edição vale para a compra inteira (todas as parcelas) e amount é o total da compra.",
		Input: object(map[string]any{
			"id":              str("Id do lançamento (qualquer parcela da compra)"),
			"description":     str("Nova descrição"),
			"category":        str("Nova categoria (existente)"),
			"amount":          number("Novo valor: o total da compra, ou o de cada parcela com per_installment"),
			"per_installment": boolean("true quando amount é o valor de cada parcela"),
			"date":            str("Nova data da compra AAAA-MM-DD"),
			"payment_method":  enum("Nova forma de pagamento", "pix", "debito", "credito"),
			"credit_card":     str("Novo cartão (nome), para crédito"),
			"installments":    integer("Novo número de parcelas (crédito)"),
			"recurring":       boolean("Se repete todo mês"),
		}, "id"),
		run: typed(func(ctx context.Context, in struct {
			ID             string  `json:"id"`
			Description    string  `json:"description"`
			Category       string  `json:"category"`
			Amount         float64 `json:"amount"`
			PerInstallment bool    `json:"per_installment"`
			Date           string  `json:"date"`
			PaymentMethod  string  `json:"payment_method"`
			CreditCard     string  `json:"credit_card"`
			Installments   int     `json:"installments"`
			Recurring      *bool   `json:"recurring"`
		}) (any, error) {
			current, err := r.findTransaction(ctx, in.ID)
			if err != nil {
				return nil, err
			}
			input := service.NewTransactionInput{
				Description:   current.Description,
				AmountCents:   current.PurchaseTotalCents,
				CategoryID:    current.CategoryID,
				PaymentMethod: current.PaymentMethod,
				PurchaseDate:  current.PurchaseDate,
				Installments:  max(current.InstallmentTotal, 1),
				IsRecurring:   current.IsRecurring,
			}
			if current.CreditCardID != nil {
				input.CreditCardID = *current.CreditCardID
			}
			cat := domain.Category{ID: current.CategoryID, Name: current.CategoryName}
			if v := strings.TrimSpace(in.Description); v != "" {
				input.Description = v
			}
			if strings.TrimSpace(in.Category) != "" {
				if cat, err = r.resolveCategory(ctx, in.Category); err != nil {
					return nil, err
				}
				input.CategoryID = cat.ID
			}
			if input.PaymentMethod, err = parsePaymentMethod(in.PaymentMethod, input.PaymentMethod); err != nil {
				return nil, err
			}
			if in.Installments > 0 {
				input.Installments = in.Installments
			}
			if in.Amount > 0 {
				input.AmountCents = purchaseCents(in.Amount, in.PerInstallment, input.Installments)
			}
			if strings.TrimSpace(in.Date) != "" {
				if input.PurchaseDate, err = r.parseDay(in.Date); err != nil {
					return nil, err
				}
			}
			if input.PaymentMethod == domain.PaymentCredit {
				if strings.TrimSpace(in.CreditCard) != "" || input.CreditCardID == "" {
					card, err := r.resolveCard(ctx, in.CreditCard)
					if err != nil {
						return nil, err
					}
					input.CreditCardID = card.ID
				}
			} else {
				input.CreditCardID = ""
				input.Installments = 1
			}
			if in.Recurring != nil {
				input.IsRecurring = *in.Recurring
			}
			if input.Installments > 1 {
				input.IsRecurring = false
			}
			txns, err := d.Transactions.Replace(ctx, in.ID, input)
			if err != nil {
				return nil, err
			}
			return txnResultFor(input, cat, false, txns), nil
		}),
	})

	r.add(Tool{
		Name: "finance_delete_transaction", Title: "Excluir lançamento", Module: "financeiro", Destructive: true,
		Description: "Exclui um lançamento pelo id (de finance_list_transactions). Numa compra parcelada, exclui a compra inteira: todas as parcelas. Um lançamento que veio do pagamento de uma conta só sai desfazendo o pagamento (bills_unpay).",
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

	r.add(Tool{
		Name: "finance_create_category", Title: "Nova categoria", Module: "financeiro",
		Description: "Cria uma categoria. tipo despesa conta como gasto; receita conta como entrada.",
		Input: object(map[string]any{
			"name":   str("Nome, ex.: \"Pets\""),
			"kind":   enum("Tipo; padrão despesa", "despesa", "receita"),
			"nature": enum("Natureza; padrão variavel", "essencial", "variavel", "investimento"),
			"color":  str("Cor em hex, ex.: #2a78d6; vazio escolhe uma"),
			"budget": number("Orçamento mensal em reais (opcional)"),
		}, "name"),
		run: typed(func(ctx context.Context, in categoryInput) (any, error) {
			cats, err := d.Categories.List(ctx)
			if err != nil {
				return nil, err
			}
			c := domain.Category{Name: strings.TrimSpace(in.Name), Kind: domain.KindExpense, Nature: domain.NatureDiscretionary, Color: nextCategoryColor(cats)}
			if err := in.apply(&c); err != nil {
				return nil, err
			}
			if err := c.Validate(); err != nil {
				return nil, err
			}
			created, err := d.Categories.Create(ctx, c)
			if err != nil {
				return nil, err
			}
			if in.Budget != nil && *in.Budget > 0 {
				b := cents(*in.Budget)
				if created, err = d.Categories.UpdateBudget(ctx, created.ID, &b); err != nil {
					return nil, err
				}
			}
			return map[string]any{"ok": true, "nome": created.Name, "tipo": created.Kind}, nil
		}),
	})

	r.add(Tool{
		Name: "finance_update_category", Title: "Editar categoria", Module: "financeiro",
		Description: "Edita uma categoria pelo nome: novo nome, tipo (despesa/receita), natureza, cor ou orçamento mensal (0 remove o orçamento). Campos não enviados ficam como estão.",
		Input: object(map[string]any{
			"category": str("Nome atual da categoria"),
			"name":     str("Novo nome"),
			"kind":     enum("Novo tipo", "despesa", "receita"),
			"nature":   enum("Nova natureza", "essencial", "variavel", "investimento"),
			"color":    str("Nova cor em hex"),
			"budget":   number("Novo orçamento mensal em reais; 0 remove"),
		}, "category"),
		run: typed(func(ctx context.Context, in struct {
			Category string `json:"category"`
			categoryInput
		}) (any, error) {
			c, err := r.resolveCategory(ctx, in.Category)
			if err != nil {
				return nil, err
			}
			if v := strings.TrimSpace(in.Name); v != "" {
				c.Name = v
			}
			if err := in.apply(&c); err != nil {
				return nil, err
			}
			if err := c.Validate(); err != nil {
				return nil, err
			}
			updated, err := d.Categories.Update(ctx, c.ID, c)
			if err != nil {
				return nil, err
			}
			if in.Budget != nil {
				var b *domain.Cents
				if *in.Budget > 0 {
					v := cents(*in.Budget)
					b = &v
				}
				if updated, err = d.Categories.UpdateBudget(ctx, c.ID, b); err != nil {
					return nil, err
				}
			}
			return map[string]any{"ok": true, "nome": updated.Name, "tipo": updated.Kind, "natureza": updated.Nature}, nil
		}),
	})

	r.add(Tool{
		Name: "finance_delete_category", Title: "Excluir categoria", Module: "financeiro", Destructive: true,
		Description: "Exclui uma categoria pelo nome. Recusa se ainda houver lançamentos ou contas nela.",
		Input:       object(map[string]any{"category": str("Nome da categoria")}, "category"),
		run: typed(func(ctx context.Context, in struct {
			Category string `json:"category"`
		}) (any, error) {
			c, err := r.resolveCategory(ctx, in.Category)
			if err != nil {
				return nil, err
			}
			return map[string]any{"ok": true}, d.Categories.Delete(ctx, c.ID)
		}),
	})

	r.add(Tool{
		Name: "finance_create_credit_card", Title: "Novo cartão", Module: "financeiro",
		Description: "Cadastra um cartão de crédito com dia de fechamento e de vencimento da fatura (1 a 28).",
		Input: object(map[string]any{
			"name":        str("Nome do cartão"),
			"closing_day": integer("Dia em que a fatura fecha"),
			"due_day":     integer("Dia em que a fatura vence"),
		}, "name", "closing_day", "due_day"),
		run: typed(func(ctx context.Context, in struct {
			Name       string `json:"name"`
			ClosingDay int    `json:"closing_day"`
			DueDay     int    `json:"due_day"`
		}) (any, error) {
			c := domain.CreditCard{Name: strings.TrimSpace(in.Name), ClosingDay: in.ClosingDay, DueDay: in.DueDay}
			if err := c.Validate(); err != nil {
				return nil, err
			}
			created, err := d.Cards.Create(ctx, c)
			if err != nil {
				return nil, err
			}
			return map[string]any{"ok": true, "nome": created.Name, "fecha_dia": created.ClosingDay, "vence_dia": created.DueDay}, nil
		}),
	})

	r.add(Tool{
		Name: "finance_update_credit_card", Title: "Editar cartão", Module: "financeiro",
		Description: "Edita um cartão pelo nome: novo nome, dia de fechamento ou de vencimento. Compras já lançadas continuam na fatura em que estavam.",
		Input: object(map[string]any{
			"credit_card": str("Nome atual do cartão"),
			"name":        str("Novo nome"),
			"closing_day": integer("Novo dia de fechamento"),
			"due_day":     integer("Novo dia de vencimento"),
		}, "credit_card"),
		run: typed(func(ctx context.Context, in struct {
			CreditCard string `json:"credit_card"`
			Name       string `json:"name"`
			ClosingDay int    `json:"closing_day"`
			DueDay     int    `json:"due_day"`
		}) (any, error) {
			c, err := r.resolveCard(ctx, in.CreditCard)
			if err != nil {
				return nil, err
			}
			if v := strings.TrimSpace(in.Name); v != "" {
				c.Name = v
			}
			if in.ClosingDay > 0 {
				c.ClosingDay = in.ClosingDay
			}
			if in.DueDay > 0 {
				c.DueDay = in.DueDay
			}
			if err := c.Validate(); err != nil {
				return nil, err
			}
			updated, err := d.Cards.Update(ctx, c.ID, c)
			if err != nil {
				return nil, err
			}
			return map[string]any{"ok": true, "nome": updated.Name, "fecha_dia": updated.ClosingDay, "vence_dia": updated.DueDay}, nil
		}),
	})

	r.add(Tool{
		Name: "finance_delete_credit_card", Title: "Excluir cartão", Module: "financeiro", Destructive: true,
		Description: "Exclui um cartão pelo nome. Recusa se houver compras nele.",
		Input:       object(map[string]any{"credit_card": str("Nome do cartão")}, "credit_card"),
		run: typed(func(ctx context.Context, in struct {
			CreditCard string `json:"credit_card"`
		}) (any, error) {
			c, err := r.resolveCard(ctx, in.CreditCard)
			if err != nil {
				return nil, err
			}
			return map[string]any{"ok": true}, d.Cards.Delete(ctx, c.ID)
		}),
	})

	if d.CardSpending != nil {
		r.add(Tool{
			Name: "finance_card_invoices", Title: "Faturas dos cartões", Module: "financeiro", ReadOnly: true,
			Description: "Fatura atual de cada cartão (a que recebe uma compra feita hoje): total, vencimento, se já foi paga, gasto por categoria e as faturas dos meses ao redor. Para pagar uma fatura, use bills_list e bills_pay.",
			Input:       object(map[string]any{}),
			run: typed(func(ctx context.Context, _ struct{}) (any, error) {
				overviews, err := d.CardSpending.Overview(ctx)
				if err != nil {
					return nil, err
				}
				type invoice struct {
					Month     string `json:"mes"`
					Due       string `json:"vencimento"`
					Total     string `json:"total"`
					Purchases int    `json:"compras"`
					Paid      bool   `json:"paga,omitempty"`
				}
				toInvoice := func(i service.CardInvoice) invoice {
					return invoice{monthString(i.Month), i.DueDate.Format(dayLayout), money(i.TotalCents), i.Purchases, i.Paid}
				}
				type card struct {
					Name       string            `json:"cartao"`
					Current    invoice           `json:"fatura_atual"`
					Months     []invoice         `json:"por_mes"`
					Categories map[string]string `json:"por_categoria"`
				}
				out := make([]card, len(overviews))
				for i, ov := range overviews {
					c := card{Name: ov.Card.Name, Current: toInvoice(ov.Current), Categories: map[string]string{}}
					for _, m := range ov.Months {
						c.Months = append(c.Months, toInvoice(m))
					}
					for _, cat := range ov.Categories {
						c.Categories[cat.Name] = money(cat.TotalCents)
					}
					out[i] = c
				}
				return map[string]any{"cartoes": out}, nil
			}),
		})
	}
}

// categoryInput is the editable part of a category; empty fields are left
// alone by apply.
type categoryInput struct {
	Name   string   `json:"name"`
	Kind   string   `json:"kind"`
	Nature string   `json:"nature"`
	Color  string   `json:"color"`
	Budget *float64 `json:"budget"`
}

func (in categoryInput) apply(c *domain.Category) error {
	if v := normalize(in.Kind); v != "" {
		c.Kind = domain.CategoryKind(v)
		if !c.Kind.Valid() {
			return invalid("tipo %q inválido: use despesa ou receita", in.Kind)
		}
	}
	if v := normalize(in.Nature); v != "" {
		c.Nature = domain.CategoryNature(v)
		if !c.Nature.Valid() {
			return invalid("natureza %q inválida: use essencial, variavel ou investimento", in.Nature)
		}
	}
	if v := strings.TrimSpace(in.Color); v != "" {
		c.Color = v
	}
	return nil
}

// findTransaction looks a transaction up by id across the recent months the
// ledger is browsed in.
func (r *Registry) findTransaction(ctx context.Context, id string) (postgres.TransactionRow, error) {
	month := domain.YearMonthOf(r.now())
	for i := -12; i <= 13; i++ {
		rows, err := r.deps.TransactionLog.ListByCompetenceMonth(ctx, month.Add(-i))
		if err != nil {
			return postgres.TransactionRow{}, err
		}
		for _, t := range rows {
			if t.ID == id {
				return t, nil
			}
		}
	}
	return postgres.TransactionRow{}, fmt.Errorf("transaction %s: %w", id, domain.ErrNotFound)
}
