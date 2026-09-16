# Cartão de crédito e competência pelo vencimento — plano de implementação

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Cadastrar cartões de crédito pelo app e fazer um gasto no crédito
entrar no mês em que a fatura que o cobra vence, em vez de sempre no mês
seguinte à compra.

**Architecture:** O ciclo de faturamento vira duas funções puras em
`domain.CreditCard` (quando fecha, quando vence). `domain.CompetenceMonth`
passa a receber o cartão e devolver o mês do vencimento. O repositório do
cartão ganha Create/Update/Delete, o router ganha as três rotas, e o
frontend ganha uma aba "Cartões" espelhando a de Categorias.

**Tech Stack:** Go 1.26 (stdlib `net/http` com `mux.HandleFunc("VERB /path/{id}")`,
pgx v5, Postgres 16), Next.js 16.3.4 App Router com Server Actions, React 19.

**Spec:** `docs/superpowers/specs/2026-09-16-cartao-competencia-design.md`

## Global Constraints

- **Sem migration.** `credit_cards.closing_day`, `credit_cards.due_day` e
  `transactions.competence_month` já existem em `0001_init.up.sql`. Nenhuma
  tarefa cria arquivo em `backend/migrations/`.
- **`closing_day` e `due_day` são 1-28**, por constraint do banco. Domínio e
  formulário repetem o limite; ninguém o afrouxa.
- **Dinheiro é `bigint` de centavos**; datas de competência são sempre o dia
  1 do mês (`YearMonth.FirstDay()`).
- **Comentários e identificadores em inglês; texto de UI e mensagens de erro
  ao usuário em português do Brasil.** É o padrão de todo o repo.
- **Arquivos `"use server"` só exportam funções async** — exportar um objeto
  ou função síncrona quebra o build do Next (já aconteceu neste repo, commit
  `9e656de`).
- **Rodar `cd backend && go test ./...` e `cd frontend && npx tsc --noEmit`
  antes de cada commit.** Os testes de Postgres pulam sozinhos sem
  `DATABASE_URL`; para rodá-los de verdade:
  `DATABASE_URL="postgres://estus:estus@localhost:5434/estus_vault?sslmode=disable"`.
- **Não rodar `TRUNCATE` nem recriar o banco.** Ele contém dados reais do
  dono desde 2026-09-16.

## File Structure

| Arquivo | Responsabilidade |
|---|---|
| `backend/internal/domain/creditcard.go` (modificar) | O cartão e seu ciclo: quando a fatura fecha, quando vence, validação |
| `backend/internal/domain/creditcard_test.go` (criar) | Tabelas de borda do ciclo |
| `backend/internal/domain/billing.go` (modificar) | `CompetenceMonth` — a regra única do orçamento |
| `backend/internal/domain/billing_test.go` (**modificar**, já existe e é rastreado) | Os casos conferidos da spec, literais, substituindo os da regra antiga |
| `backend/internal/domain/transaction.go` (modificar) | `NewInstallmentPurchase` passa a receber o cartão |
| `backend/internal/store/postgres/creditcard_repo.go` (modificar) | Create/Update/Delete, FK → `ErrConflict` |
| `backend/internal/store/postgres/creditcard_repo_test.go` (criar) | Round-trip contra o Postgres real |
| `backend/internal/httpapi/dto.go` (modificar) | `creditCardRequest` |
| `backend/internal/httpapi/handlers.go` (modificar) | Create/Update/DeleteCreditCard |
| `backend/internal/httpapi/router.go` (modificar) | POST/PUT/DELETE `/api/credit-cards` |
| `frontend/lib/api.ts` (modificar) | `createCreditCard` / `updateCreditCard` / `deleteCreditCard` |
| `frontend/app/financeiro/cartoes/actions.ts` (criar) | Server actions do cartão |
| `frontend/app/financeiro/cartoes/page.tsx` (criar) | Página da aba |
| `frontend/components/CreditCardManager.tsx` (criar) | Lista + formulário |
| `frontend/components/FinanceTabs.tsx` (modificar) | Aba "Cartões" |
| `frontend/app/ui.css` (modificar) | Linhas da lista de cartões |

---

### Task 1: Ciclo de faturamento do cartão

Funções puras, sem banco e sem HTTP. Nada depende delas ainda.

**Files:**
- Modify: `backend/internal/domain/creditcard.go`
- Test: `backend/internal/domain/creditcard_test.go` (criar)

**Interfaces:**
- Consumes: nada.
- Produces:
  - `func (c CreditCard) ClosingDateFor(purchase time.Time) time.Time`
  - `func (c CreditCard) DueDateFor(closing time.Time) time.Time`
  - `func (c CreditCard) Validate() error`

- [ ] **Step 1: Escrever os testes que falham**

Criar `backend/internal/domain/creditcard_test.go`:

```go
package domain

import (
	"errors"
	"testing"
	"time"
)

// onDay, not day: internal/domain's habit_test.go already defines a day()
// that parses a string, and two helpers with one name cannot share a package.

func onDay(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func TestClosingDateFor(t *testing.T) {
	card := CreditCard{ClosingDay: 10, DueDay: 20}
	cases := []struct {
		name     string
		purchase time.Time
		want     time.Time
	}{
		{"antes do fechamento", onDay(2026, time.September, 5), onDay(2026, time.September, 10)},
		{"no dia do fechamento fecha hoje", onDay(2026, time.September, 10), onDay(2026, time.September, 10)},
		{"depois do fechamento", onDay(2026, time.September, 15), onDay(2026, time.October, 10)},
		{"vira o ano", onDay(2026, time.December, 28), onDay(2027, time.January, 10)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := card.ClosingDateFor(tc.purchase); !got.Equal(tc.want) {
				t.Errorf("ClosingDateFor(%s) = %s, want %s",
					tc.purchase.Format("2006-01-02"), got.Format("2006-01-02"), tc.want.Format("2006-01-02"))
			}
		})
	}
}

func TestDueDateFor(t *testing.T) {
	cases := []struct {
		name    string
		card    CreditCard
		closing time.Time
		want    time.Time
	}{
		{
			"vencimento depois do fechamento vence no mesmo mês",
			CreditCard{ClosingDay: 10, DueDay: 20},
			onDay(2026, time.September, 10), onDay(2026, time.September, 20),
		},
		{
			"vencimento antes do fechamento vence no mês seguinte",
			CreditCard{ClosingDay: 25, DueDay: 15},
			onDay(2026, time.September, 25), onDay(2026, time.October, 15),
		},
		{
			"vencimento no mesmo dia do fechamento vence no mês seguinte",
			CreditCard{ClosingDay: 10, DueDay: 10},
			onDay(2026, time.September, 10), onDay(2026, time.October, 10),
		},
		{
			"vira o ano",
			CreditCard{ClosingDay: 25, DueDay: 15},
			onDay(2026, time.December, 25), onDay(2027, time.January, 15),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.card.DueDateFor(tc.closing); !got.Equal(tc.want) {
				t.Errorf("DueDateFor(%s) = %s, want %s",
					tc.closing.Format("2006-01-02"), got.Format("2006-01-02"), tc.want.Format("2006-01-02"))
			}
		})
	}
}

func TestCreditCardValidate(t *testing.T) {
	cases := []struct {
		name string
		card CreditCard
		ok   bool
	}{
		{"válido", CreditCard{Name: "Nubank", ClosingDay: 10, DueDay: 20}, true},
		{"nome vazio", CreditCard{Name: "  ", ClosingDay: 10, DueDay: 20}, false},
		{"fechamento 0", CreditCard{Name: "Nubank", ClosingDay: 0, DueDay: 20}, false},
		{"fechamento 29", CreditCard{Name: "Nubank", ClosingDay: 29, DueDay: 20}, false},
		{"vencimento 0", CreditCard{Name: "Nubank", ClosingDay: 10, DueDay: 0}, false},
		{"vencimento 29", CreditCard{Name: "Nubank", ClosingDay: 10, DueDay: 29}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.card.Validate()
			if tc.ok && err != nil {
				t.Fatalf("Validate() = %v, want nil", err)
			}
			if !tc.ok {
				if err == nil {
					t.Fatal("Validate() = nil, want an error")
				}
				if !errors.Is(err, ErrValidation) {
					t.Errorf("Validate() = %v, want it to wrap ErrValidation", err)
				}
			}
		})
	}
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd backend && go test ./internal/domain/ -run 'TestClosingDateFor|TestDueDateFor|TestCreditCardValidate'`
Expected: FAIL na compilação — `c.ClosingDateFor undefined`, `c.Validate undefined`, e
`DueDateFor` com assinatura errada (hoje recebe `YearMonth`).

- [ ] **Step 3: Implementar**

Substituir o corpo de `backend/internal/domain/creditcard.go` por:

```go
package domain

import (
	"fmt"
	"strings"
	"time"
)

// CreditCard models a real card's billing cycle: when its invoice closes and
// when that invoice has to be paid. Both matter to the budget, because
// CompetenceMonth books a purchase in the month its invoice is due.
type CreditCard struct {
	ID         string
	Name       string
	ClosingDay int // day of month the invoice closes, 1-28
	DueDay     int // day of month the invoice is due, 1-28
	CreatedAt  time.Time
}

// ClosingDateFor is the date the invoice that charges a purchase made on
// purchase closes. Buying ON the closing day still makes that day's invoice —
// one more day of grace, which is how the owner reads it.
func (c CreditCard) ClosingDateFor(purchase time.Time) time.Time {
	month := purchase.Month()
	if purchase.Day() > c.ClosingDay {
		month++
	}
	// time.Date normalizes month 13 into January of the next year.
	return time.Date(purchase.Year(), month, c.ClosingDay, 0, 0, 0, 0, purchase.Location())
}

// DueDateFor is the date the invoice that closed on closing has to be paid.
// A due day at or before the closing day belongs to the next month: a card
// that closes on the 25th and is due on the 15th is paid the month after it
// closes.
func (c CreditCard) DueDateFor(closing time.Time) time.Time {
	month := closing.Month()
	if c.DueDay <= c.ClosingDay {
		month++
	}
	return time.Date(closing.Year(), month, c.DueDay, 0, 0, 0, 0, closing.Location())
}

// Validate keeps the day range in step with the check constraint the
// credit_cards table has carried since 0001_init, so a bad day is refused
// with a readable message instead of a Postgres error.
func (c CreditCard) Validate() error {
	if strings.TrimSpace(c.Name) == "" {
		return fmt.Errorf("%w: name is required", ErrValidation)
	}
	if c.ClosingDay < 1 || c.ClosingDay > 28 {
		return fmt.Errorf("%w: closing day must be between 1 and 28", ErrValidation)
	}
	if c.DueDay < 1 || c.DueDay > 28 {
		return fmt.Errorf("%w: due day must be between 1 and 28", ErrValidation)
	}
	return nil
}
```

- [ ] **Step 4: Rodar e ver passar**

Run: `cd backend && go test ./internal/domain/ -run 'TestClosingDateFor|TestDueDateFor|TestCreditCardValidate' -v`
Expected: PASS em todos os subtestes.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/domain/creditcard.go backend/internal/domain/creditcard_test.go
git commit -m "feat(domain): credit card billing cycle and validation"
```

---

### Task 2: Competência pelo vencimento

Muda a regra central e os dois chamadores dela, numa tacada — separar em
duas tarefas deixaria o repo sem compilar no meio.

**Files:**
- Modify: `backend/internal/domain/billing.go:34-52`
- Modify: `backend/internal/domain/transaction.go:46-76`
- Modify: `backend/internal/service/transaction_service.go:98`
- Modify: `backend/internal/domain/billing_test.go` (JÁ EXISTE e é rastreado — 81 linhas, 3 testes da regra antiga)
- Modify: `backend/internal/domain/creditcard_test.go` (remover o helper `onDay`)

**Interfaces:**
- Consumes (da Task 1): `CreditCard.ClosingDateFor`, `CreditCard.DueDateFor`.
- Produces:
  - `func CompetenceMonth(purchaseDate time.Time, method PaymentMethod, card *CreditCard) YearMonth`
  - `func NewInstallmentPurchase(desc string, total Cents, categoryID string, card CreditCard, purchaseDate time.Time, installments int) []Transaction`

- [ ] **Step 1: Reescrever os testes que falham**

`backend/internal/domain/billing_test.go` **já existe, é rastreado pelo git e
tem 81 linhas**. NÃO crie do zero: os três testes dele afirmam a regra antiga
e precisam ser atualizados, mas um deles guarda uma invariante que não tem
nada a ver com competência e **precisa sobreviver** — a soma das parcelas
nunca perde nem inventa centavos, e o resto vai na última.

Ele também já define `date(y, m, d)`, idêntico ao `onDay` criado na Task 1.
Use `date` e **apague o `onDay` de `creditcard_test.go`**, trocando seus usos
por `date` — dois helpers iguais no mesmo pacote é duplicação que a Task 1
introduziu por engano.

Substituir o conteúdo de `backend/internal/domain/billing_test.go` por:

```go
package domain

import (
	"testing"
	"time"
)

func date(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// As duas tabelas de "Casos conferidos" da spec, literais.
func TestCompetenceMonthFollowsTheInvoiceDueDate(t *testing.T) {
	closes25due15 := CreditCard{ClosingDay: 25, DueDay: 15}
	closes10due20 := CreditCard{ClosingDay: 10, DueDay: 20}
	cases := []struct {
		name     string
		card     CreditCard
		purchase time.Time
		want     YearMonth
	}{
		{"fecha 25 vence 15: compra antes do fechamento", closes25due15,
			date(2026, time.September, 5), YearMonth{2026, 10}},
		{"fecha 25 vence 15: compra depois do fechamento", closes25due15,
			date(2026, time.September, 28), YearMonth{2026, 11}},
		{"fecha 10 vence 20: compra antes do fechamento", closes10due20,
			date(2026, time.September, 5), YearMonth{2026, 9}},
		{"fecha 10 vence 20: compra depois do fechamento", closes10due20,
			date(2026, time.September, 15), YearMonth{2026, 10}},
		{"vira o ano", closes10due20,
			date(2026, time.December, 15), YearMonth{2027, 1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CompetenceMonth(tc.purchase, PaymentCredit, &tc.card); got != tc.want {
				t.Errorf("CompetenceMonth = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCompetenceMonthLeavesDebitAndPixOnTheMonthOfThePurchase(t *testing.T) {
	purchase := date(2026, time.September, 28)
	for _, method := range []PaymentMethod{PaymentDebit, PaymentPix} {
		if got := CompetenceMonth(purchase, method, nil); got != (YearMonth{2026, 9}) {
			t.Errorf("CompetenceMonth(%s) = %v, want setembro", method, got)
		}
	}
}

// Crédito sem cartão é erro de programação — o serviço já recusa antes de
// chegar aqui. A regra não inventa uma data: devolve o mês da compra.
func TestCompetenceMonthWithoutACardFallsBackToThePurchaseMonth(t *testing.T) {
	if got := CompetenceMonth(date(2026, time.September, 28), PaymentCredit, nil); got != (YearMonth{2026, 9}) {
		t.Errorf("CompetenceMonth = %v, want setembro", got)
	}
}

func TestNewInstallmentPurchase_SplitsRemainderIntoLastInstallment(t *testing.T) {
	card := CreditCard{ID: "card-1", ClosingDay: 10, DueDay: 20}
	// 8 de maio é antes do fechamento: a fatura fecha 10/05 e vence 20/05,
	// então a primeira parcela cai em maio e as seguintes andam um mês.
	txns := NewInstallmentPurchase("Notebook Dell", 302400, "cat-compras", card, date(2026, time.May, 8), 12)

	if len(txns) != 12 {
		t.Fatalf("got %d installments, want 12", len(txns))
	}

	var sum Cents
	for i, tx := range txns {
		sum += tx.AmountCents
		if tx.InstallmentNumber != i+1 {
			t.Errorf("installment %d has InstallmentNumber %d", i, tx.InstallmentNumber)
		}
		if tx.InstallmentTotal != 12 {
			t.Errorf("installment %d has InstallmentTotal %d, want 12", i, tx.InstallmentTotal)
		}
		if tx.CompetenceMonth != (YearMonth{2026, 5}).Add(i) {
			t.Errorf("installment %d competence = %v, want %v", i, tx.CompetenceMonth, (YearMonth{2026, 5}).Add(i))
		}
		if tx.CreditCardID == nil || *tx.CreditCardID != "card-1" {
			t.Errorf("installment %d card = %v", i, tx.CreditCardID)
		}
	}
	if sum != 302400 {
		t.Errorf("installments sum to %d cents, want 302400 (no cents lost or invented)", sum)
	}
	// 302400 / 12 divides evenly, but the invariant that matters is the sum —
	// verify a case that doesn't divide evenly too.
	txns2 := NewInstallmentPurchase("Presente", 1000, "cat", card, date(2026, time.January, 1), 3)
	sum = 0
	for _, tx := range txns2 {
		sum += tx.AmountCents
	}
	if sum != 1000 {
		t.Errorf("uneven split sum = %d, want 1000", sum)
	}
	if txns2[2].AmountCents != 334 { // 333, 333, 334
		t.Errorf("last installment = %d, want remainder folded in (334)", txns2[2].AmountCents)
	}
}

func TestSingleInstallmentHasNoGroupID(t *testing.T) {
	card := CreditCard{ID: "card-1", ClosingDay: 10, DueDay: 20}
	txns := NewInstallmentPurchase("Uber", 3890, "cat", card, date(2026, time.August, 26), 1)
	if len(txns) != 1 {
		t.Fatalf("got %d transactions, want 1", len(txns))
	}
	if txns[0].InstallmentGroupID != nil {
		t.Errorf("a non-installment purchase should not have an InstallmentGroupID")
	}
	// 26/08 é depois do fechamento: fecha 10/09, vence 20/09.
	if txns[0].CompetenceMonth != (YearMonth{2026, 9}) {
		t.Errorf("competence = %v, want setembro", txns[0].CompetenceMonth)
	}
}
```

Em seguida, em `backend/internal/domain/creditcard_test.go`: apague a função
`onDay` e o comentário de duas linhas acima dela, e troque todas as chamadas
`onDay(` por `date(`. O arquivo passa a usar o helper que já existia.

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd backend && go test ./internal/domain/ -count=1`
Expected: FAIL na compilação — `CompetenceMonth` aceita 2 argumentos, não 3,
e `NewInstallmentPurchase` espera `cardID string`.

- [ ] **Step 3: Reescrever a regra**

Em `backend/internal/domain/billing.go`, substituir o comentário e a função
(hoje linhas 34-52) por:

```go
// CompetenceMonth is the single rule the whole app is designed around: a
// purchase counts against the month the money actually leaves the account.
// Debit and Pix leave it the day they happen. A credit purchase leaves it
// when the invoice that charges it is due, which is what card is for: the
// purchase joins the invoice closing on or after it, and that invoice's due
// date names the month.
//
// This deliberately replaced an earlier rule that sent every credit purchase
// to the month after the purchase, ignoring the closing day. That rule was
// simpler but wrong: on a card closing on the 25th and due on the 15th, a
// purchase on the 28th really is charged almost two months later, and hiding
// that made the budget lie. Do not "simplify" it back.
//
// card is nil for debit and Pix. A credit purchase with a nil card is a
// programming error — the service refuses credit without a card long before
// this point — so it falls back to the purchase month rather than inventing
// a date.
func CompetenceMonth(purchaseDate time.Time, method PaymentMethod, card *CreditCard) YearMonth {
	if method != PaymentCredit || card == nil {
		return YearMonthOf(purchaseDate)
	}
	return YearMonthOf(card.DueDateFor(card.ClosingDateFor(purchaseDate)))
}
```

- [ ] **Step 4: Passar o cartão para as parcelas**

Em `backend/internal/domain/transaction.go`, na assinatura de
`NewInstallmentPurchase` trocar `cardID string` por `card CreditCard`, e
dentro dela trocar as duas linhas que dependiam disso:

```go
func NewInstallmentPurchase(desc string, total Cents, categoryID string, card CreditCard, purchaseDate time.Time, installments int) []Transaction {
```

```go
	firstCompetence := CompetenceMonth(purchaseDate, PaymentCredit, &card)
```

e, dentro do laço, o campo do cartão:

```go
			CreditCardID:       &card.ID,
```

- [ ] **Step 5: Atualizar o serviço**

Em `backend/internal/service/transaction_service.go`, a chamada da linha 79
já carrega o cartão; guardar o resultado e usá-lo nos dois pontos. Trocar:

```go
		if _, err := s.cards.Get(ctx, in.CreditCardID); err != nil {
			return nil, fmt.Errorf("credit card %s: %w", in.CreditCardID, err)
		}
```

por:

```go
		card, err := s.cards.Get(ctx, in.CreditCardID)
		if err != nil {
			return nil, fmt.Errorf("credit card %s: %w", in.CreditCardID, err)
		}
```

Trocar a chamada de `NewInstallmentPurchase` (linha 86) para passar `card` no
lugar de `in.CreditCardID`, e a linha 98 para:

```go
			CompetenceMonth:  domain.CompetenceMonth(in.PurchaseDate, in.PaymentMethod, cardFor(in, card)),
```

Como `card` só existe no ramo do crédito, declare-o antes do `if` com
`var card domain.CreditCard` e passe `&card` apenas quando o método for
crédito — a forma mais direta, sem helper novo:

```go
			CompetenceMonth:  domain.CompetenceMonth(in.PurchaseDate, in.PaymentMethod, cardPtr),
```

onde `cardPtr` é `nil` por padrão e recebe `&card` dentro do ramo do crédito.

- [ ] **Step 6: Rodar tudo e ver passar**

Run: `cd backend && go build ./... && go test ./...`
Expected: PASS em todos os pacotes. Se algum teste antigo de
`transaction_service` presumia a competência velha, corrija a expectativa
dele para a regra nova (e só ela) — não afrouxe a regra.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/domain/billing.go backend/internal/domain/billing_test.go \
        backend/internal/domain/transaction.go backend/internal/service/transaction_service.go
git commit -m "feat(domain): book credit purchases in the month the invoice is due"
```

---

### Task 3: Create/Update/Delete no repositório do cartão

**Files:**
- Modify: `backend/internal/store/postgres/creditcard_repo.go`
- Test: `backend/internal/store/postgres/creditcard_repo_test.go` (criar)

**Interfaces:**
- Consumes (Task 1): `domain.CreditCard`.
- Produces:
  - `func (r *CreditCardRepo) Create(ctx context.Context, c domain.CreditCard) (domain.CreditCard, error)`
  - `func (r *CreditCardRepo) Update(ctx context.Context, id string, c domain.CreditCard) (domain.CreditCard, error)`
  - `func (r *CreditCardRepo) Delete(ctx context.Context, id string) error`

- [ ] **Step 1: Escrever o teste que falha**

Criar `backend/internal/store/postgres/creditcard_repo_test.go`:

```go
package postgres

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/rafael/estus-vault/backend/internal/domain"
)

func TestCreditCardRepoCRUD(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	ctx := context.Background()
	db, err := Connect(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer db.Close()
	if err := db.Migrate(ctx, "../../../migrations"); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repo := NewCreditCardRepo(db)

	created, err := repo.Create(ctx, domain.CreditCard{Name: "Cartão de teste", ClosingDay: 10, DueDay: 20})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// Apagado mesmo que o teste falhe no meio: é o banco real do dono.
	defer repo.Delete(ctx, created.ID)

	if created.ID == "" || created.Name != "Cartão de teste" || created.ClosingDay != 10 || created.DueDay != 20 {
		t.Fatalf("created = %+v", created)
	}
	if created.CreatedAt.IsZero() {
		t.Error("created_at não foi devolvido")
	}

	got, err := repo.Get(ctx, created.ID)
	if err != nil || got.Name != "Cartão de teste" {
		t.Fatalf("get = %+v, %v", got, err)
	}

	updated, err := repo.Update(ctx, created.ID, domain.CreditCard{Name: "Renomeado", ClosingDay: 5, DueDay: 25})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Name != "Renomeado" || updated.ClosingDay != 5 || updated.DueDay != 25 {
		t.Fatalf("updated = %+v", updated)
	}
	if updated.ID != created.ID {
		t.Errorf("update trocou o id: %s → %s", created.ID, updated.ID)
	}

	if err := repo.Delete(ctx, created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := repo.Get(ctx, created.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("get depois do delete = %v, want ErrNotFound", err)
	}
}

func TestCreditCardRepoMissingRows(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	ctx := context.Background()
	db, err := Connect(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer db.Close()
	repo := NewCreditCardRepo(db)

	const missing = "00000000-0000-0000-0000-000000000000"
	if err := repo.Delete(ctx, missing); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("delete inexistente = %v, want ErrNotFound", err)
	}
	if _, err := repo.Update(ctx, missing, domain.CreditCard{Name: "x", ClosingDay: 1, DueDay: 2}); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("update inexistente = %v, want ErrNotFound", err)
	}
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run:
```bash
cd backend && DATABASE_URL="postgres://estus:estus@localhost:5434/estus_vault?sslmode=disable" \
  go test ./internal/store/postgres/ -count=1 -run TestCreditCardRepo
```
Expected: FAIL na compilação — `repo.Create`, `repo.Update`, `repo.Delete` não existem.

- [ ] **Step 3: Implementar**

Acrescentar ao fim de `backend/internal/store/postgres/creditcard_repo.go`
(o arquivo já importa `errors`, `fmt`, `pgx` e `domain`; acrescente
`"github.com/jackc/pgx/v5/pgconn"` aos imports):

```go
func (r *CreditCardRepo) Create(ctx context.Context, c domain.CreditCard) (domain.CreditCard, error) {
	var out domain.CreditCard
	err := r.db.Pool.QueryRow(ctx, `
		insert into credit_cards (name, closing_day, due_day)
		values ($1, $2, $3)
		returning id, name, closing_day, due_day, created_at`,
		c.Name, c.ClosingDay, c.DueDay,
	).Scan(&out.ID, &out.Name, &out.ClosingDay, &out.DueDay, &out.CreatedAt)
	if err != nil {
		return domain.CreditCard{}, fmt.Errorf("create credit card: %w", err)
	}
	return out, nil
}

func (r *CreditCardRepo) Update(ctx context.Context, id string, c domain.CreditCard) (domain.CreditCard, error) {
	var out domain.CreditCard
	err := r.db.Pool.QueryRow(ctx, `
		update credit_cards set name = $2, closing_day = $3, due_day = $4
		where id = $1
		returning id, name, closing_day, due_day, created_at`,
		id, c.Name, c.ClosingDay, c.DueDay,
	).Scan(&out.ID, &out.Name, &out.ClosingDay, &out.DueDay, &out.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.CreditCard{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.CreditCard{}, fmt.Errorf("update credit card %s: %w", id, err)
	}
	return out, nil
}

// Delete refuses a card that still has transactions. The foreign key already
// stops it; without translating the violation it would surface as a 500.
func (r *CreditCardRepo) Delete(ctx context.Context, id string) error {
	tag, err := r.db.Pool.Exec(ctx, `delete from credit_cards where id = $1`, id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgForeignKeyViolation {
			return fmt.Errorf("%w: credit card is used by existing transactions", domain.ErrConflict)
		}
		return fmt.Errorf("delete credit card %s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}
```

- [ ] **Step 4: Rodar e ver passar**

Run:
```bash
cd backend && DATABASE_URL="postgres://estus:estus@localhost:5434/estus_vault?sslmode=disable" \
  go test ./internal/store/postgres/ -count=1 -run TestCreditCardRepo -v
```
Expected: PASS nos dois testes.

- [ ] **Step 5: Confirmar que o banco ficou limpo**

Run:
```bash
docker exec estus-vault-postgres-1 psql -U estus -d estus_vault \
  -c "select name from credit_cards order by name;"
```
Expected: só os cartões de verdade. Nenhum "Cartão de teste" sobrando — se
sobrou, apague-o e descubra por que o `defer` não rodou antes de seguir.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/store/postgres/creditcard_repo.go backend/internal/store/postgres/creditcard_repo_test.go
git commit -m "feat(store): credit card create, update and delete"
```

---

### Task 4: Rotas HTTP do cartão

**Files:**
- Modify: `backend/internal/httpapi/dto.go` (perto de `creditCardDTO`, linha ~46)
- Modify: `backend/internal/httpapi/handlers.go` (perto de `ListCreditCards`, linha ~109)
- Modify: `backend/internal/httpapi/router.go:44`
- Test: `backend/internal/httpapi/handlers_creditcards_test.go` (criar)

**Interfaces:**
- Consumes (Tasks 1 e 3): `domain.CreditCard.Validate`, `CreditCardRepo.Create/Update/Delete`.
- Produces: `POST /api/credit-cards`, `PUT /api/credit-cards/{id}`,
  `DELETE /api/credit-cards/{id}`. Corpo JSON:
  `{"name": string, "closing_day": int, "due_day": int}`.

- [ ] **Step 1: Escrever o teste que falha**

`internal/httpapi` hoje tem um único teste, e ele cobre uma função pura —
não existe helper de router de teste, e `Handlers` depende de repositórios
concretos do Postgres. Logo, um teste de rota aqui é necessariamente de
integração, no mesmo molde dos de `store/postgres`: pula sem `DATABASE_URL`.

Criar `backend/internal/httpapi/handlers_creditcards_test.go`:

```go
package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/rafael/estus-vault/backend/internal/service"
	"github.com/rafael/estus-vault/backend/internal/store/postgres"
)

// creditCardRouter builds the real router over the real repositories — the
// only way to exercise these routes, since Handlers takes concrete repos.
func creditCardRouter(t *testing.T) (http.Handler, *postgres.DB) {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	ctx := context.Background()
	db, err := postgres.Connect(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(db.Close)
	categories := postgres.NewCategoryRepo(db)
	cards := postgres.NewCreditCardRepo(db)
	transactions := postgres.NewTransactionRepo(db)
	handlers := NewHandlers(
		categories,
		cards,
		service.NewTransactionService(transactions, categories, cards),
		service.NewDashboardService(transactions, categories),
	)
	return NewRouter(handlers, Modules{}), db
}

func postCard(t *testing.T, router http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/credit-cards", strings.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestCreateCreditCardRejectsBadInput(t *testing.T) {
	router, _ := creditCardRouter(t)
	for _, body := range []string{
		`{"name":"Nubank","closing_day":0,"due_day":20}`,
		`{"name":"Nubank","closing_day":29,"due_day":20}`,
		`{"name":"Nubank","closing_day":10,"due_day":0}`,
		`{"name":"Nubank","closing_day":10,"due_day":29}`,
		`{"name":"   ","closing_day":10,"due_day":20}`,
		`{`,
	} {
		// writeError maps ErrValidation to 422, not 400 — see respond.go.
		if rec := postCard(t, router, body); rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("POST %s = %d, want 422", body, rec.Code)
		}
	}
}

func TestCreateAndDeleteCreditCardRoundTrip(t *testing.T) {
	router, _ := creditCardRouter(t)
	rec := postCard(t, router, `{"name":"Cartão de rota","closing_day":10,"due_day":20}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST = %d, want 201; body %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil || created.ID == "" {
		t.Fatalf("resposta = %s, %v", rec.Body.String(), err)
	}

	put := httptest.NewRequest(http.MethodPut, "/api/credit-cards/"+created.ID,
		strings.NewReader(`{"name":"Renomeado","closing_day":5,"due_day":25}`))
	putRec := httptest.NewRecorder()
	router.ServeHTTP(putRec, put)
	if putRec.Code != http.StatusOK {
		t.Fatalf("PUT = %d, want 200; body %s", putRec.Code, putRec.Body.String())
	}

	del := httptest.NewRequest(http.MethodDelete, "/api/credit-cards/"+created.ID, nil)
	delRec := httptest.NewRecorder()
	router.ServeHTTP(delRec, del)
	if delRec.Code != http.StatusNoContent {
		t.Fatalf("DELETE = %d, want 204; body %s", delRec.Code, delRec.Body.String())
	}
}
```

Acrescente `"encoding/json"` aos imports. Os dois testes criam e apagam o
próprio cartão; nenhum dado do dono é tocado.

- [ ] **Step 2: Rodar e ver falhar**

Run:
```bash
cd backend && DATABASE_URL="postgres://estus:estus@localhost:5434/estus_vault?sslmode=disable" \
  go test ./internal/httpapi/ -count=1 -run 'TestCreateCreditCard|TestCreateAndDelete'
```
Expected: FAIL — a rota POST não existe, então o mux devolve 405 ou 404, não 422/201.

Se `NewHandlers`, `NewRouter`, `Modules` ou os construtores de serviço
tiverem assinatura diferente da usada acima, ajuste a chamada ao que o
código realmente expõe (confira em `backend/cmd/api/main.go`, que monta
exatamente isso) — o teste é que se adapta ao repo, não o contrário.

- [ ] **Step 3: DTO**

Em `backend/internal/httpapi/dto.go`, logo acima de `creditCardDTO`:

```go
type creditCardRequest struct {
	Name       string `json:"name"`
	ClosingDay int    `json:"closing_day"`
	DueDay     int    `json:"due_day"`
}

func (r creditCardRequest) toDomain() domain.CreditCard {
	return domain.CreditCard{Name: strings.TrimSpace(r.Name), ClosingDay: r.ClosingDay, DueDay: r.DueDay}
}
```

Se `strings` ainda não estiver importado em `dto.go`, acrescente.

- [ ] **Step 4: Handlers**

Em `backend/internal/httpapi/handlers.go`, logo abaixo de `ListCreditCards`:

```go
func (h *Handlers) CreateCreditCard(w http.ResponseWriter, r *http.Request) {
	var req creditCardRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, fmt.Errorf("%w: invalid JSON body", domain.ErrValidation))
		return
	}
	card := req.toDomain()
	if err := card.Validate(); err != nil {
		writeError(w, err)
		return
	}
	created, err := h.cards.Create(r.Context(), card)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toCreditCardDTO(created))
}

func (h *Handlers) UpdateCreditCard(w http.ResponseWriter, r *http.Request) {
	var req creditCardRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, fmt.Errorf("%w: invalid JSON body", domain.ErrValidation))
		return
	}
	card := req.toDomain()
	if err := card.Validate(); err != nil {
		writeError(w, err)
		return
	}
	updated, err := h.cards.Update(r.Context(), r.PathValue("id"), card)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toCreditCardDTO(updated))
}

func (h *Handlers) DeleteCreditCard(w http.ResponseWriter, r *http.Request) {
	if err := h.cards.Delete(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

Confira o nome real do campo do repositório no struct `Handlers` (linha ~15
declara `categories`; o do cartão vem do parâmetro `cards` de `NewHandlers`)
e o nome real do conversor de DTO já usado por `ListCreditCards` — use-os em
vez de inventar `h.cards` / `toCreditCardDTO` se diferirem.

- [ ] **Step 5: Rotas**

Em `backend/internal/httpapi/router.go`, logo após a linha 44:

```go
	mux.HandleFunc("POST /api/credit-cards", h.CreateCreditCard)
	mux.HandleFunc("PUT /api/credit-cards/{id}", h.UpdateCreditCard)
	mux.HandleFunc("DELETE /api/credit-cards/{id}", h.DeleteCreditCard)
```

- [ ] **Step 6: Rodar e ver passar**

Run:
```bash
cd backend && go build ./... && \
  DATABASE_URL="postgres://estus:estus@localhost:5434/estus_vault?sslmode=disable" go test ./... -count=1
```
Expected: PASS. Confirme que `writeError` traduz `domain.ErrConflict` em 409 —
se não traduzir, acrescente o caso lá, porque é o que faz o delete bloqueado
chegar legível ao frontend.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/httpapi/
git commit -m "feat(api): create, update and delete credit cards"
```

---

### Task 5: Aba Cartões no frontend

Sem harness de teste no frontend: a verificação é `tsc` mais o roteiro manual
do Step 7, feito no navegador.

**Files:**
- Modify: `frontend/lib/api.ts` (perto de `updateCategoryBudget`, linha ~60)
- Create: `frontend/app/financeiro/cartoes/actions.ts`
- Create: `frontend/app/financeiro/cartoes/page.tsx`
- Create: `frontend/components/CreditCardManager.tsx`
- Modify: `frontend/components/FinanceTabs.tsx:6-11`
- Modify: `frontend/app/ui.css`

**Interfaces:**
- Consumes (Task 4): `POST/PUT/DELETE /api/credit-cards`.
- Produces: rota `/financeiro/cartoes`.

- [ ] **Step 1: Cliente de API**

Em `frontend/lib/api.ts`, depois de `updateCategoryBudget`:

```ts
export interface CreditCardInput {
  name: string;
  closing_day: number;
  due_day: number;
}

export function createCreditCard(input: CreditCardInput): Promise<CreditCard> {
  return apiFetch<CreditCard>("/api/credit-cards", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function updateCreditCard(id: string, input: CreditCardInput): Promise<CreditCard> {
  return apiFetch<CreditCard>(`/api/credit-cards/${id}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export function deleteCreditCard(id: string): Promise<unknown> {
  return apiFetch(`/api/credit-cards/${id}`, { method: "DELETE" });
}
```

Confira o nome do tipo do cartão já exportado em `frontend/lib/types.ts` e
use-o; se ainda não existir, declare `CreditCard` lá com `id`, `name`,
`closing_day`, `due_day`.

- [ ] **Step 2: Server actions**

Criar `frontend/app/financeiro/cartoes/actions.ts`. Só funções async são
exportadas — a validação dos dias é helper privado:

```ts
"use server";

import { revalidatePath } from "next/cache";
import {
  createCreditCard,
  updateCreditCard,
  deleteCreditCard,
  type CreditCardInput,
} from "@/lib/api";

function invalid(input: CreditCardInput): string | null {
  if (!input.name.trim()) return "Nome é obrigatório.";
  for (const [label, day] of [
    ["fechamento", input.closing_day],
    ["vencimento", input.due_day],
  ] as const) {
    if (!Number.isInteger(day) || day < 1 || day > 28) {
      return `O dia de ${label} precisa estar entre 1 e 28.`;
    }
  }
  return null;
}

function revalidateCards() {
  revalidatePath("/financeiro");
  revalidatePath("/financeiro/cartoes");
  revalidatePath("/financeiro/lancamentos");
}

export async function createCreditCardAction(input: CreditCardInput): Promise<{ error?: string }> {
  const problem = invalid(input);
  if (problem) return { error: problem };
  try {
    await createCreditCard(input);
  } catch (err) {
    return { error: err instanceof Error ? err.message : "Falha ao salvar." };
  }
  revalidateCards();
  return {};
}

export async function updateCreditCardAction(
  id: string,
  input: CreditCardInput,
): Promise<{ error?: string }> {
  const problem = invalid(input);
  if (problem) return { error: problem };
  try {
    await updateCreditCard(id, input);
  } catch (err) {
    return { error: err instanceof Error ? err.message : "Falha ao salvar." };
  }
  revalidateCards();
  return {};
}

export async function deleteCreditCardAction(id: string): Promise<{ error?: string }> {
  try {
    await deleteCreditCard(id);
  } catch (err) {
    return { error: err instanceof Error ? err.message : "Falha ao excluir." };
  }
  revalidateCards();
  return {};
}
```

`revalidateCards` é síncrona mas **não exportada** — isso é permitido; o que
o Next recusa é export síncrono.

- [ ] **Step 3: Componente**

Criar `frontend/components/CreditCardManager.tsx`, espelhando
`components/CategoryManager.tsx` (leia-o antes: mesma estrutura de
`Form` / `Row` / `Manager`, mesmo `Modal`, mesmos ícones). O que muda:

- `FormState` é `{ name: string; closingDay: string; dueDay: string }` — os
  dias como texto, convertidos com `Number(...)` na hora de chamar a action,
  para o campo poder ficar vazio enquanto se digita.
- Dois `<input type="number" min={1} max={28}>` lado a lado num `div.row2`,
  rotulados "Fecha no dia" e "Vence no dia".
- Uma linha de aviso fixa abaixo dos campos, exatamente com este texto:
  `Mudar esses dias não altera lançamentos já feitos — só vale para os próximos.`
  Renderize-a com `className="empty-note"`.
- A linha da lista mostra `{card.name}` e
  `Fecha dia {card.closing_day} · vence dia {card.due_day}`.
- O `confirm()` do excluir diz:
  `Excluir o cartão "<nome>"? Só é possível se não houver lançamentos nele.`
- O erro devolvido pela action aparece na linha, como o
  `deleteError` do `CategoryManager`.

- [ ] **Step 4: Página**

Criar `frontend/app/financeiro/cartoes/page.tsx`:

```tsx
import { listCreditCards } from "@/lib/api";
import { CreditCardManager } from "@/components/CreditCardManager";

export default async function CartoesPage() {
  const cards = await listCreditCards();
  return <CreditCardManager cards={cards} />;
}
```

Confira o nome real da função de listagem em `frontend/lib/api.ts` (linhas
~34-35) e use-o.

- [ ] **Step 5: Aba**

Em `frontend/components/FinanceTabs.tsx`, acrescentar entre "Categorias" e
"Contas":

```ts
  { href: "/financeiro/cartoes", label: "Cartões" },
```

- [ ] **Step 6: Estilo**

Em `frontend/app/ui.css`, ao lado das regras `.category-manager-*`,
acrescentar o equivalente para o cartão:

```css
.credit-card-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 9px 0;
  flex-wrap: wrap;
}
.credit-card-row + .credit-card-row {
  border-top: 1px solid var(--hairline);
}
.credit-card-name {
  font-size: 13px;
  font-weight: 550;
  flex: 1;
  min-width: 0;
}
.credit-card-cycle {
  font-size: 12px;
  color: var(--ink-mute);
  flex: none;
}
```

- [ ] **Step 7: Verificar no navegador**

```bash
cd frontend && npx tsc --noEmit
cd .. && ./scripts/estus-brain.sh --rebuild
```

Depois, em `http://localhost:37887/financeiro/cartoes`, confirmar um a um:

1. A aba "Cartões" aparece e o "Cartão principal" está listado com
   "Fecha dia 25 · vence dia 15".
2. Criar "Nubank" com fechamento 10 e vencimento 20 — aparece na lista.
3. Editar o Nubank para fechamento 5 — a lista reflete.
4. Tentar salvar com dia 29 — recusa com a mensagem de 1 a 28, sem gravar.
5. Em Lançamentos, o `select` de cartão agora oferece o Nubank.
6. Lançar no Nubank R$ 10 no crédito com data **05/09** e conferir no banco
   que a competência ficou em **setembro**:
   ```bash
   docker exec estus-vault-postgres-1 psql -U estus -d estus_vault \
     -c "select description, purchase_date, competence_month from transactions order by created_at desc limit 1;"
   ```
7. Lançar outro no Nubank com data **15/09** e conferir que a competência
   ficou em **outubro**.
8. Apagar os dois lançamentos de teste pela UI, depois apagar o Nubank —
   deve funcionar agora que não tem lançamento.
9. Tentar apagar um cartão que **tenha** lançamento e confirmar que a
   mensagem de erro aparece na linha, sem tela branca nem 500.
10. Conferir que o banco voltou ao estado anterior: só o "Cartão principal",
    e nenhum lançamento de teste sobrando.

- [ ] **Step 8: Commit**

```bash
git add frontend/
git commit -m "feat(web): manage credit cards from Financeiro"
```

---

## Self-Review

**Cobertura da spec:**

| Requisito da spec | Tarefa |
|---|---|
| Regra de fechamento e vencimento | 1 |
| Compra no dia do fechamento entra nessa fatura | 1 (Step 1, caso "no dia do fechamento fecha hoje") |
| Virada de ano | 1 (casos "vira o ano") |
| Competência = mês do vencimento | 2 |
| As duas tabelas de casos conferidos | 2 (Step 1) |
| Débito e Pix inalterados | 2 |
| Crédito com card nil | 2 |
| Parcelas seguem a regra nova | 2 |
| Comentário antigo substituído, registrando a reversão | 2 (Step 3) |
| `DueDateFor` redefinida, não duplicada | 1 (Step 3 reescreve o arquivo) |
| POST/PUT/DELETE de cartão | 3 e 4 |
| Validação 1-28 e nome vazio | 1 (domínio), 4 (handler), 5 (action) |
| Delete bloqueado vira conflito, não 500 | 3 (Step 3), 4 (Step 6), 5 (Step 7 item 9) |
| Editar dias não recalcula o passado | 5 (Step 3, linha de aviso) — é o comportamento que já existe, a tarefa só o torna visível |
| Tela de cartões | 5 |
| `select` de cartão enxerga os novos | 5 (Step 7 item 5) — sai de graça, mesma rota |
| Sem migration | Global Constraints |

**Sem placeholders:** todo passo de código traz o código. Os três pontos em
que mando conferir um nome no repo (`h.cards`/`toCreditCardDTO` na Task 4,
`listCreditCards` e o tipo `CreditCard` na Task 5, e o helper de router de
teste) são verificações contra o código existente, não decisões em aberto.

**Consistência de tipos:** `CreditCard.ClosingDateFor` e `DueDateFor`
devolvem `time.Time` e são consumidas assim na Task 2. `CompetenceMonth`
recebe `*CreditCard` em todas as chamadas (Task 2 Steps 3-5).
`NewInstallmentPurchase` recebe `CreditCard` por valor e usa `&card` ao
chamar `CompetenceMonth` — coerente entre os Steps 4 e 5. `CreditCardInput`
no frontend tem os mesmos três campos do `creditCardRequest` do backend, com
os mesmos nomes JSON.

**Um risco conhecido:** a Task 2 Step 5 descreve a mudança no
`transaction_service.go` em prosa porque o trecho exato depende do arredor;
quem executar deve ler as linhas 75-105 antes de editar, e o Step 6 (build +
suíte inteira) é o portão que pega qualquer erro ali.
