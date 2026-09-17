# Gastos fixos como contas recorrentes — plano de implementação

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Cadastrar um gasto que se repete todo mês uma vez só, ver a conta do
mês seguinte nascer ao abrir o mês, e fazer quitar a conta registrar a
despesa no orçamento.

**Architecture:** Uma conta recorrente é uma *série* de ocorrências em
`bills`, ligadas por `series_id`, sem tabela de modelo: o molde é a última
ocorrência. Abrir um mês materializa o que falta, de trás para frente.
Quitar uma conta a pagar grava a conta e o lançamento na mesma transação de
banco, usando o mesmo padrão de helper de pacote que `diet_repo` e
`training_repo` já usam.

**Tech Stack:** Go 1.26 (stdlib `net/http` com `mux.HandleFunc("VERB /path/{id}")`,
pgx v5, Postgres 16), Next.js 16.3.4 App Router com Server Actions, React 19.

**Spec:** `docs/superpowers/specs/2026-09-16-gastos-fixos-design.md`

## Global Constraints

- **Dinheiro é `bigint` de centavos.** `amount_cents > 0` é constraint do banco.
- **Comentários e identificadores em inglês; texto de UI e mensagens de erro
  ao usuário em português do Brasil.** É o padrão de todo o repo.
- **`writeError` mapeia `domain.ErrValidation` para 422 e `domain.ErrConflict`
  para 409** (`internal/httpapi/respond.go:31-43`). Não é 400.
- **Arquivos `"use server"` só exportam funções async** — exportar um objeto
  ou função síncrona quebra o build do Next (commit `9e656de`).
- **O frontend não tem harness de teste** (sem vitest/jest/playwright). Não
  adicione um. A verificação é `npx tsc --noEmit` mais roteiro manual.
- **O app roda build de produção.** Mudança no frontend só aparece depois de
  `./scripts/estus-brain.sh --rebuild`, que também recompila o backend.
  Esperar com `until curl -fsS -o /dev/null http://127.0.0.1:37887/financeiro/contas; do sleep 3; done`.
  Frontend na 37887, API na 37888.
- **O Postgres em localhost:5434 contém dados REAIS do dono.** Nunca
  `TRUNCATE`, `DROP`, `DELETE` sem `WHERE` estreito, nem reseed. Testes de
  integração criam e apagam só o que eles mesmos criaram.
  `DATABASE_URL="postgres://estus:estus@localhost:5434/estus_vault?sslmode=disable"`
- **Limpeza de teste com `defer`, nunca `t.Cleanup`, quando o teste também
  faz `defer db.Close()`**: `t.Cleanup` roda depois dos `defer`, com o pool já
  fechado, e o delete falha em silêncio. Isso já deixou linhas órfãs no banco
  real uma vez.
- **Commit só dos arquivos da tarefa.** Nunca `git add -A`, `git add .` nem
  `git commit -a`.

## File Structure

| Arquivo | Responsabilidade |
|---|---|
| `backend/internal/domain/bill.go` (modificar) | A conta, a série e como a próxima ocorrência nasce |
| `backend/internal/domain/bill_test.go` (criar) | Bordas do calendário e das regras de validação |
| `backend/migrations/0019_recurring_bills.{up,down}.sql` (criar) | `series_id`, `amount_estimated`, `payment_method`; remove `recurring` |
| `backend/internal/store/postgres/bill_repo.go` (modificar) | Colunas novas, listagem por mês, séries ativas, `Pay`/`Unpay` numa transação |
| `backend/internal/store/postgres/bill_repo_test.go` (criar) | Round-trip contra o Postgres real |
| `backend/internal/store/postgres/transaction_repo.go` (modificar) | Extrai `insertTransactions` para ser reusado dentro da transação do `Pay` |
| `backend/internal/service/bill_service.go` (modificar) | `Materialize` e `Pay`, com as regras |
| `backend/internal/service/bill_service_test.go` (criar) | Materialização e pagamento, com fakes |
| `backend/internal/httpapi/dto_bills.go` (modificar) | Campos novos e corpo da confirmação de pagamento |
| `backend/internal/httpapi/handlers_bills.go` (modificar) | Rota de pagamento e o mês na listagem |
| `frontend/app/financeiro/contas/page.tsx` (modificar) | Mês em foco |
| `frontend/components/BillForm.tsx` (modificar) | Campos de série |
| `frontend/components/BillsBoard.tsx` (modificar) | Marca de estimado e confirmação de pagamento |
| `frontend/app/financeiro/page.tsx` (modificar) | Tile de gastos fixos |

---

### Task 1: A série no domínio

Funções puras. Nada depende delas ainda.

**Files:**
- Modify: `backend/internal/domain/bill.go`
- Test: `backend/internal/domain/bill_test.go` (criar — não existe nenhum)

**Interfaces:**
- Consumes: nada.
- Produces:
  - `Bill.SeriesID *string`, `Bill.AmountEstimated bool`, `Bill.PaymentMethod *PaymentMethod` (campo `Recurring bool` removido)
  - `func (b Bill) Recurring() bool`
  - `func (b Bill) NextOccurrence(estimated bool) Bill`

- [ ] **Step 1: Escrever os testes que falham**

Criar `backend/internal/domain/bill_test.go`:

```go
package domain

import (
	"errors"
	"testing"
	"time"
)

func billOn(y int, m time.Month, d int) Bill {
	series := "series-1"
	method := PaymentPix
	cat := "cat-1"
	return Bill{
		ID: "bill-1", Description: "Aluguel", AmountCents: 200000,
		DueDate: time.Date(y, m, d, 0, 0, 0, 0, time.UTC),
		Direction: BillPayable, CategoryID: &cat,
		SeriesID: &series, PaymentMethod: &method,
	}
}

func TestNextOccurrenceKeepsTheDueDay(t *testing.T) {
	next := billOn(2026, time.September, 10).NextOccurrence(false)
	want := time.Date(2026, time.October, 10, 0, 0, 0, 0, time.UTC)
	if !next.DueDate.Equal(want) {
		t.Errorf("vencimento = %s, want %s", next.DueDate.Format("2006-01-02"), want.Format("2006-01-02"))
	}
}

// Dia 31 num mês de 30 cai no último dia — nunca escorrega para o mês
// seguinte, o que mudaria a competência do lançamento em silêncio.
func TestNextOccurrenceClampsToTheLastDayOfAShorterMonth(t *testing.T) {
	cases := []struct {
		name string
		from Bill
		want time.Time
	}{
		{"31 de outubro vira 30 de novembro", billOn(2026, time.October, 31), time.Date(2026, time.November, 30, 0, 0, 0, 0, time.UTC)},
		{"31 de janeiro vira 28 de fevereiro", billOn(2027, time.January, 31), time.Date(2027, time.February, 28, 0, 0, 0, 0, time.UTC)},
		{"31 de dezembro vira 31 de janeiro", billOn(2026, time.December, 31), time.Date(2027, time.January, 31, 0, 0, 0, 0, time.UTC)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.from.NextOccurrence(false).DueDate; !got.Equal(tc.want) {
				t.Errorf("vencimento = %s, want %s", got.Format("2006-01-02"), tc.want.Format("2006-01-02"))
			}
		})
	}
}

func TestNextOccurrenceCarriesTheSeriesAndClearsTheSettlement(t *testing.T) {
	paid := time.Date(2026, time.September, 9, 0, 0, 0, 0, time.UTC)
	from := billOn(2026, time.September, 10)
	from.PaidAt = &paid
	next := from.NextOccurrence(false)

	if next.SeriesID == nil || *next.SeriesID != *from.SeriesID {
		t.Errorf("série = %v, want %v", next.SeriesID, from.SeriesID)
	}
	if next.ID == from.ID {
		t.Error("a ocorrência nova não pode reusar o id da anterior")
	}
	if next.PaidAt != nil {
		t.Error("a ocorrência nova nasce pendente, não paga")
	}
	if next.AmountCents != from.AmountCents || next.Description != from.Description {
		t.Errorf("valor/descrição não vieram junto: %+v", next)
	}
	if next.CategoryID == nil || *next.CategoryID != *from.CategoryID {
		t.Error("categoria não veio junto")
	}
	if next.PaymentMethod == nil || *next.PaymentMethod != *from.PaymentMethod {
		t.Error("forma de pagamento não veio junto")
	}
}

func TestNextOccurrenceMarksTheAmountAsEstimatedWhenAsked(t *testing.T) {
	if got := billOn(2026, time.September, 10).NextOccurrence(true); !got.AmountEstimated {
		t.Error("a série de valor variável deve nascer com o valor estimado")
	}
	if got := billOn(2026, time.September, 10).NextOccurrence(false); got.AmountEstimated {
		t.Error("a série de valor fixo não deve nascer estimada")
	}
}

func TestRecurringIsHavingASeries(t *testing.T) {
	if !billOn(2026, time.September, 10).Recurring() {
		t.Error("conta com série deve ser recorrente")
	}
	one := billOn(2026, time.September, 10)
	one.SeriesID = nil
	if one.Recurring() {
		t.Error("conta sem série não é recorrente")
	}
}

// Só a conta a pagar recorrente vira lançamento, e lançamento exige os dois.
func TestValidateRequiresCategoryAndMethodOnAPayableSeries(t *testing.T) {
	noCategory := billOn(2026, time.September, 10)
	noCategory.CategoryID = nil
	if err := noCategory.Validate(); !errors.Is(err, ErrValidation) {
		t.Errorf("série a pagar sem categoria = %v, want ErrValidation", err)
	}

	noMethod := billOn(2026, time.September, 10)
	noMethod.PaymentMethod = nil
	if err := noMethod.Validate(); !errors.Is(err, ErrValidation) {
		t.Errorf("série a pagar sem forma de pagamento = %v, want ErrValidation", err)
	}

	receivable := billOn(2026, time.September, 10)
	receivable.Direction = BillReceivable
	receivable.CategoryID, receivable.PaymentMethod = nil, nil
	if err := receivable.Validate(); err != nil {
		t.Errorf("série a receber sem categoria/forma = %v, want nil", err)
	}

	oneOff := billOn(2026, time.September, 10)
	oneOff.SeriesID, oneOff.CategoryID, oneOff.PaymentMethod = nil, nil, nil
	if err := oneOff.Validate(); err != nil {
		t.Errorf("conta avulsa sem categoria/forma = %v, want nil", err)
	}
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd backend && go test ./internal/domain/ -count=1`
Expected: FAIL na compilação — `Bill` não tem `SeriesID`, `AmountEstimated`,
`PaymentMethod`, `NextOccurrence` nem o método `Recurring`.

- [ ] **Step 3: Implementar**

Em `backend/internal/domain/bill.go`, trocar o campo `Recurring bool` do
struct `Bill` por:

```go
	// SeriesID links the occurrences of one monthly bill across months; nil
	// means a one-off. There is no template row: the mould for the next month
	// is simply the latest occurrence.
	SeriesID *string
	// AmountEstimated says the amount was carried over from the previous
	// month and not yet confirmed — "the power bill is usually R$180", not
	// "the power bill came to R$180".
	AmountEstimated bool
	// PaymentMethod is the default used when settling this bill, so paying it
	// is one click instead of a form.
	PaymentMethod *PaymentMethod
	// TransactionID is the expense this bill created when it was settled, so
	// undoing the payment removes exactly that one.
	TransactionID *string
```

E acrescentar ao fim do arquivo:

```go
// Recurring reports whether this bill is one occurrence of a monthly series.
func (b Bill) Recurring() bool { return b.SeriesID != nil }

// NextOccurrence is the bill that continues b's series in the month after
// b's own. It copies everything that describes the obligation and drops
// everything that describes this month's settlement, so the new occurrence
// starts pending. estimated marks the carried-over amount as a guess, which
// is what a series whose value changes every month needs.
func (b Bill) NextOccurrence(estimated bool) Bill {
	next := b
	next.ID = newID()
	next.PaidAt = nil
	next.CreatedAt = time.Time{}
	next.AmountEstimated = estimated
	next.DueDate = nextMonthSameDay(b.DueDate)
	return next
}

// nextMonthSameDay is the same day of the following month, clamped to that
// month's last day. A bill due on the 31st must land on the 30th of a
// 30-day month, never roll into the month after — that would silently move
// the expense into a different budget month.
func nextMonthSameDay(d time.Time) time.Time {
	year, month, day := d.Date()
	firstOfNext := time.Date(year, month+1, 1, 0, 0, 0, 0, d.Location())
	lastDay := firstOfNext.AddDate(0, 1, -1).Day()
	return time.Date(firstOfNext.Year(), firstOfNext.Month(), min(day, lastDay), 0, 0, 0, 0, d.Location())
}
```

Confira o nome real do gerador de id já usado no pacote `domain` (em
`transaction.go` ele aparece como `newID()`) e use-o em vez de inventar.

Em `Validate`, antes do `return nil` final, acrescentar:

```go
	if b.Recurring() && b.Direction == BillPayable {
		if b.CategoryID == nil || *b.CategoryID == "" {
			return fmt.Errorf("%w: a recurring bill needs a category, because settling it records an expense", ErrValidation)
		}
		if b.PaymentMethod == nil || !b.PaymentMethod.Valid() {
			return fmt.Errorf("%w: a recurring bill needs a payment method, because settling it records an expense", ErrValidation)
		}
	}
```

- [ ] **Step 4: Rodar e ver passar**

Run: `cd backend && go test ./internal/domain/ -count=1 -v -run 'TestNextOccurrence|TestRecurringIsHavingASeries|TestValidateRequires'`
Expected: PASS em todos os subtestes.

O pacote inteiro ainda não compila fora do domínio: `bill_repo.go`,
`bill_service.go` e `dto_bills.go` ainda usam o campo `Recurring`. Isso é
esperado e a Task 2 conserta. **Não** conserte esses arquivos aqui, e **não**
rode `go build ./...` esperando sucesso nesta tarefa.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/domain/bill.go backend/internal/domain/bill_test.go
git commit -m "feat(contas): série mensal e próxima ocorrência no domínio"
```

---

### Task 2: Migration e repositório

Fecha a compilação que a Task 1 deixou aberta.

**Files:**
- Create: `backend/migrations/0019_recurring_bills.up.sql` e `.down.sql`
- Modify: `backend/internal/store/postgres/bill_repo.go`
- Modify: `backend/internal/httpapi/dto_bills.go` e `backend/internal/service/bill_service.go` (só o mínimo para compilar: trocar `Recurring` por `SeriesID`)
- Test: `backend/internal/store/postgres/bill_repo_test.go` (criar — não existe nenhum)

**Interfaces:**
- Consumes (Task 1): `domain.Bill` com `SeriesID`/`AmountEstimated`/`PaymentMethod`.
- Produces:
  - `func (r *BillRepo) ListByMonth(ctx context.Context, ym domain.YearMonth, direction *domain.BillDirection) ([]domain.Bill, error)`
  - `func (r *BillRepo) LatestPerSeries(ctx context.Context) ([]domain.Bill, error)`

- [ ] **Step 1: Escrever a migration**

`backend/migrations/0019_recurring_bills.up.sql`:

```sql
-- A recurring bill becomes a series of occurrences linked by series_id.
-- There is no template row: the mould for next month is the latest
-- occurrence, so correcting this month's amount is what defines the next.
alter table bills add column series_id uuid;
alter table bills add column amount_estimated boolean not null default false;
alter table bills add column payment_method text
    check (payment_method is null or payment_method in ('debito', 'credito', 'pix'));

-- The expense a settled bill created. Without it, undoing a payment would
-- have to guess which transaction to remove from description and date.
alter table bills add column transaction_id uuid references transactions(id);

-- recurring was decorative: nothing ever read it to generate anything.
-- series_id is not null replaces it. No data to convert — the table has been
-- empty since the owner cleared it to start using the app for real.
alter table bills drop column recurring;

create index bills_series_due_idx on bills (series_id, due_date) where series_id is not null;
create index bills_due_date_idx on bills (due_date);
```

`backend/migrations/0019_recurring_bills.down.sql`:

```sql
drop index if exists bills_due_date_idx;
drop index if exists bills_series_due_idx;
alter table bills add column recurring boolean not null default false;
alter table bills drop column transaction_id;
alter table bills drop column payment_method;
alter table bills drop column amount_estimated;
alter table bills drop column series_id;
```

- [ ] **Step 2: Escrever o teste que falha**

Criar `backend/internal/store/postgres/bill_repo_test.go`:

```go
package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/rafael/estus-vault/backend/internal/domain"
)

func billRepoForTest(t *testing.T) (*BillRepo, *CategoryRepo, context.Context) {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	ctx := context.Background()
	db, err := Connect(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(db.Close)
	if err := db.Migrate(ctx, "../../../migrations"); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return NewBillRepo(db), NewCategoryRepo(db), ctx
}

func TestBillRepoRoundTripsTheSeriesColumns(t *testing.T) {
	repo, categories, ctx := billRepoForTest(t)

	cat, err := categories.Create(ctx, domain.Category{Name: "Categoria de teste (contas)", Nature: "essencial", Color: "#123456"})
	if err != nil {
		t.Fatalf("create category: %v", err)
	}
	defer categories.Delete(ctx, cat.ID)

	series, method := "11111111-1111-1111-1111-111111111111", domain.PaymentPix
	created, err := repo.Create(ctx, domain.Bill{
		Description: "Aluguel de teste", AmountCents: 200000,
		DueDate:   time.Date(2026, time.September, 10, 0, 0, 0, 0, time.UTC),
		Direction: domain.BillPayable, CategoryID: &cat.ID,
		SeriesID: &series, AmountEstimated: true, PaymentMethod: &method,
	})
	if err != nil {
		t.Fatalf("create bill: %v", err)
	}
	defer repo.Delete(ctx, created.ID)

	if created.SeriesID == nil || *created.SeriesID != series {
		t.Errorf("series_id = %v, want %s", created.SeriesID, series)
	}
	if !created.AmountEstimated {
		t.Error("amount_estimated não voltou true")
	}
	if created.PaymentMethod == nil || *created.PaymentMethod != domain.PaymentPix {
		t.Errorf("payment_method = %v, want pix", created.PaymentMethod)
	}

	inMonth, err := repo.ListByMonth(ctx, domain.YearMonth{Year: 2026, Month: 9}, nil)
	if err != nil {
		t.Fatalf("list by month: %v", err)
	}
	if !containsBill(inMonth, created.ID) {
		t.Error("a conta não apareceu no mês do vencimento dela")
	}

	other, err := repo.ListByMonth(ctx, domain.YearMonth{Year: 2026, Month: 10}, nil)
	if err != nil {
		t.Fatalf("list by month: %v", err)
	}
	if containsBill(other, created.ID) {
		t.Error("a conta apareceu num mês que não é o do vencimento")
	}

	latest, err := repo.LatestPerSeries(ctx)
	if err != nil {
		t.Fatalf("latest per series: %v", err)
	}
	if !containsBill(latest, created.ID) {
		t.Error("a única ocorrência da série deveria ser a mais recente dela")
	}
}

func containsBill(bills []domain.Bill, id string) bool {
	for _, b := range bills {
		if b.ID == id {
			return true
		}
	}
	return false
}
```

Confira a assinatura real de `CategoryRepo.Create` e de `domain.Category`
antes de rodar, e ajuste a construção da categoria ao que o repo expõe.

- [ ] **Step 3: Rodar e ver falhar**

Run:
```bash
cd backend && DATABASE_URL="postgres://estus:estus@localhost:5434/estus_vault?sslmode=disable" \
  go test ./internal/store/postgres/ -count=1 -run TestBillRepo
```
Expected: FAIL na compilação — `ListByMonth` e `LatestPerSeries` não existem.

- [ ] **Step 4: Implementar o repositório**

Em `backend/internal/store/postgres/bill_repo.go`, acrescentar
`series_id, amount_estimated, payment_method` a **todas** as listas de
colunas de `Create`, `List`, `MarkPaid` e `Update` (insert, select e scan),
trocando o antigo `recurring`. Depois acrescentar:

```go
// ListByMonth is every bill due in ym, which is how the Contas screen is
// read now that it has month navigation.
func (r *BillRepo) ListByMonth(ctx context.Context, ym domain.YearMonth, direction *domain.BillDirection) ([]domain.Bill, error) {
	start := ym.FirstDay()
	end := start.AddDate(0, 1, 0)
	rows, err := r.db.Pool.Query(ctx, `
		select id, description, amount_cents, due_date, direction, category_id,
			paid_at, series_id, amount_estimated, payment_method, transaction_id, created_at
		from bills
		where due_date >= $1 and due_date < $2
			and ($3::text is null or direction = $3)
		order by due_date, description`, start, end, direction)
	if err != nil {
		return nil, fmt.Errorf("list bills for %v: %w", ym, err)
	}
	defer rows.Close()
	return scanBills(rows)
}

// LatestPerSeries is the most recent occurrence of every series — the mould
// each series' next month is copied from.
func (r *BillRepo) LatestPerSeries(ctx context.Context) ([]domain.Bill, error) {
	rows, err := r.db.Pool.Query(ctx, `
		select distinct on (series_id)
			id, description, amount_cents, due_date, direction, category_id,
			paid_at, series_id, amount_estimated, payment_method, created_at
		from bills
		where series_id is not null
		order by series_id, due_date desc`)
	if err != nil {
		return nil, fmt.Errorf("latest bill per series: %w", err)
	}
	defer rows.Close()
	return scanBills(rows)
}
```

Extraia o laço de scan que `List` já tem para uma função de pacote
`scanBills(rows pgx.Rows) ([]domain.Bill, error)` e use-a nas três, em vez de
repetir o mesmo laço três vezes.

- [ ] **Step 5: Fechar a compilação do resto**

`dto_bills.go` e `bill_service.go` ainda citam o campo `Recurring`. Troque
cada uso pelo equivalente com `SeriesID`:
- `NewBillInput.Recurring bool` vira `SeriesID *string`, `AmountEstimated bool`
  e `PaymentMethod *domain.PaymentMethod`.
- No DTO de saída, `recurring` vira o booleano `b.Recurring()`, e entram
  `amount_estimated` e `payment_method`.
- No DTO de entrada, `recurring: true` passa a significar "crie uma série
  nova": o serviço gera um `series_id` novo quando o campo vem true e a conta
  ainda não tem série. A regra de geração fica na Task 3; aqui basta compilar
  e manter o comportamento atual de criar uma conta avulsa.

- [ ] **Step 6: Rodar e ver passar**

Run:
```bash
cd backend && go build ./... && \
  DATABASE_URL="postgres://estus:estus@localhost:5434/estus_vault?sslmode=disable" go test ./... -count=1
```
Expected: PASS em todos os pacotes.

- [ ] **Step 7: Conferir que o banco ficou limpo**

```bash
docker exec estus-vault-postgres-1 psql -U estus -d estus_vault \
  -c "select count(*) from bills;" -c "select name from categories order by name;"
```
Expected: zero contas e as 7 categorias do dono, sem "Categoria de teste
(contas)" sobrando. Se sobrou, diga em vez de apagar calado — o `defer`
falhou e isso é um defeito a reportar.

- [ ] **Step 8: Commit**

```bash
git add backend/migrations/0019_recurring_bills.up.sql backend/migrations/0019_recurring_bills.down.sql \
        backend/internal/store/postgres/bill_repo.go backend/internal/store/postgres/bill_repo_test.go \
        backend/internal/httpapi/dto_bills.go backend/internal/service/bill_service.go
git commit -m "feat(contas): colunas de série, listagem por mês e última ocorrência"
```

---

### Task 3: Materializar o mês

**Files:**
- Modify: `backend/internal/service/bill_service.go`
- Test: `backend/internal/service/bill_service_test.go` (criar — não existe nenhum)

**Interfaces:**
- Consumes (Tasks 1 e 2): `Bill.NextOccurrence`, `BillRepo.LatestPerSeries`, `BillRepo.Create`.
- Produces: `func (s *BillService) Materialize(ctx context.Context, ym domain.YearMonth) (int, error)` — devolve quantas ocorrências criou.

- [ ] **Step 1: Escrever os testes que falham**

O `BillService` hoje recebe `*postgres.BillRepo` concreto. Para testar
`Materialize` sem banco, extraia uma interface no pacote `service`, no topo
de `bill_service.go`:

```go
// billStore is the part of *postgres.BillRepo the service uses, so the
// materialization rules can be tested without a database.
type billStore interface {
	Create(ctx context.Context, b domain.Bill) (domain.Bill, error)
	List(ctx context.Context, direction *domain.BillDirection, onlyOpen bool) ([]domain.Bill, error)
	ListByMonth(ctx context.Context, ym domain.YearMonth, direction *domain.BillDirection) ([]domain.Bill, error)
	LatestPerSeries(ctx context.Context) ([]domain.Bill, error)
	MarkPaid(ctx context.Context, id string, paidAt time.Time) (domain.Bill, error)
	Update(ctx context.Context, b domain.Bill) (domain.Bill, error)
	Delete(ctx context.Context, id string) error
	ReceivedTotalForMonth(ctx context.Context, ym domain.YearMonth) (domain.Cents, error)
	OpenTotals(ctx context.Context) (domain.Cents, domain.Cents, int, error)
}
```

e troque o campo `bills *postgres.BillRepo` por `bills billStore`. O
construtor continua recebendo o repo concreto — ele satisfaz a interface.
Confirme cada assinatura acima contra o repo real antes de escrever; se
alguma diferir, a interface é que se ajusta.

Criar `backend/internal/service/bill_service_test.go`:

```go
package service

import (
	"context"
	"testing"
	"time"

	"github.com/rafael/estus-vault/backend/internal/domain"
)

// fakeBills keeps bills in memory, which is all Materialize needs.
type fakeBills struct {
	bills []domain.Bill
}

func (f *fakeBills) Create(_ context.Context, b domain.Bill) (domain.Bill, error) {
	if b.ID == "" {
		b.ID = "generated-" + b.DueDate.Format("2006-01")
	}
	f.bills = append(f.bills, b)
	return b, nil
}

func (f *fakeBills) ListByMonth(_ context.Context, ym domain.YearMonth, _ *domain.BillDirection) ([]domain.Bill, error) {
	var out []domain.Bill
	for _, b := range f.bills {
		if b.DueDate.Year() == ym.Year && int(b.DueDate.Month()) == ym.Month {
			out = append(out, b)
		}
	}
	return out, nil
}

func (f *fakeBills) LatestPerSeries(context.Context) ([]domain.Bill, error) {
	latest := map[string]domain.Bill{}
	for _, b := range f.bills {
		if b.SeriesID == nil {
			continue
		}
		if cur, ok := latest[*b.SeriesID]; !ok || b.DueDate.After(cur.DueDate) {
			latest[*b.SeriesID] = b
		}
	}
	var out []domain.Bill
	for _, b := range latest {
		out = append(out, b)
	}
	return out, nil
}

func (f *fakeBills) List(context.Context, *domain.BillDirection, bool) ([]domain.Bill, error) {
	return f.bills, nil
}
func (f *fakeBills) MarkPaid(context.Context, string, time.Time) (domain.Bill, error) {
	return domain.Bill{}, nil
}
func (f *fakeBills) Update(context.Context, domain.Bill) (domain.Bill, error) {
	return domain.Bill{}, nil
}
func (f *fakeBills) Delete(context.Context, string) error { return nil }
func (f *fakeBills) ReceivedTotalForMonth(context.Context, domain.YearMonth) (domain.Cents, error) {
	return 0, nil
}
func (f *fakeBills) OpenTotals(context.Context) (domain.Cents, domain.Cents, int, error) {
	return 0, 0, 0, nil
}

func seriesBill(id string, y int, m time.Month, d int, cents domain.Cents) domain.Bill {
	series, method, cat := "series-1", domain.PaymentPix, "cat-1"
	return domain.Bill{
		ID: id, Description: "Luz", AmountCents: cents,
		DueDate:   time.Date(y, m, d, 0, 0, 0, 0, time.UTC),
		Direction: domain.BillPayable, CategoryID: &cat,
		SeriesID: &series, PaymentMethod: &method,
	}
}

func serviceWith(f *fakeBills) *BillService {
	s := NewBillService(nil, nil)
	s.bills = f
	return s
}

func TestMaterializeIsIdempotent(t *testing.T) {
	f := &fakeBills{bills: []domain.Bill{seriesBill("b1", 2026, time.September, 10, 18000)}}
	s := serviceWith(f)
	ctx := context.Background()

	if _, err := s.Materialize(ctx, domain.YearMonth{Year: 2026, Month: 10}); err != nil {
		t.Fatalf("materialize: %v", err)
	}
	if _, err := s.Materialize(ctx, domain.YearMonth{Year: 2026, Month: 10}); err != nil {
		t.Fatalf("materialize de novo: %v", err)
	}
	october, _ := f.ListByMonth(ctx, domain.YearMonth{Year: 2026, Month: 10}, nil)
	if len(october) != 1 {
		t.Fatalf("outubro tem %d contas, want 1", len(october))
	}
}

// Chegar em dezembro partindo de setembro cria outubro a partir de setembro,
// novembro a partir de outubro e dezembro a partir de novembro. Copiar
// sempre de setembro perderia a correção de valor feita em outubro.
func TestMaterializeChainsMonthsCarryingCorrectionsForward(t *testing.T) {
	f := &fakeBills{bills: []domain.Bill{seriesBill("b1", 2026, time.September, 10, 18000)}}
	s := serviceWith(f)
	ctx := context.Background()

	if _, err := s.Materialize(ctx, domain.YearMonth{Year: 2026, Month: 10}); err != nil {
		t.Fatalf("materialize outubro: %v", err)
	}
	// O dono corrige o valor de outubro quando a conta chega.
	for i := range f.bills {
		if f.bills[i].DueDate.Month() == time.October {
			f.bills[i].AmountCents = 25000
			f.bills[i].AmountEstimated = false
		}
	}
	if _, err := s.Materialize(ctx, domain.YearMonth{Year: 2026, Month: 12}); err != nil {
		t.Fatalf("materialize dezembro: %v", err)
	}

	for _, month := range []time.Month{time.November, time.December} {
		got, _ := f.ListByMonth(ctx, domain.YearMonth{Year: 2026, Month: int(month)}, nil)
		if len(got) != 1 {
			t.Fatalf("%s tem %d contas, want 1", month, len(got))
		}
		if got[0].AmountCents != 25000 {
			t.Errorf("%s herdou %d, want 25000 (a correção de outubro)", month, got[0].AmountCents)
		}
	}
}

func TestMaterializeDoesNotInventThePastBeforeTheSeriesStarted(t *testing.T) {
	f := &fakeBills{bills: []domain.Bill{seriesBill("b1", 2026, time.September, 10, 18000)}}
	s := serviceWith(f)
	ctx := context.Background()

	n, err := s.Materialize(ctx, domain.YearMonth{Year: 2026, Month: 7})
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}
	if n != 0 {
		t.Errorf("criou %d contas para um mês anterior à série, want 0", n)
	}
}

func TestMaterializeStopsAtTheSafetyCap(t *testing.T) {
	f := &fakeBills{bills: []domain.Bill{seriesBill("b1", 2026, time.September, 10, 18000)}}
	s := serviceWith(f)

	n, err := s.Materialize(context.Background(), domain.YearMonth{Year: 2040, Month: 1})
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}
	if n != 24 {
		t.Errorf("criou %d contas, want 24 (o teto)", n)
	}
}

func TestMaterializeIgnoresOneOffBills(t *testing.T) {
	one := seriesBill("b1", 2026, time.September, 10, 18000)
	one.SeriesID = nil
	f := &fakeBills{bills: []domain.Bill{one}}
	s := serviceWith(f)

	n, err := s.Materialize(context.Background(), domain.YearMonth{Year: 2026, Month: 12})
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}
	if n != 0 {
		t.Errorf("criou %d contas a partir de uma conta avulsa, want 0", n)
	}
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd backend && go test ./internal/service/ -count=1 -run TestMaterialize`
Expected: FAIL na compilação — `Materialize` não existe e `s.bills` ainda é
o tipo concreto.

- [ ] **Step 3: Implementar**

Em `backend/internal/service/bill_service.go`:

```go
// materializeCap is how many months one call may create. Opening a month
// years away should return what fits instead of writing hundreds of rows;
// the next opening carries on from where this one stopped.
const materializeCap = 24

// Materialize creates the missing occurrences of every active series up to
// and including ym, oldest first, each copied from the one before it, and
// returns how many it created. It is idempotent: a month a series already
// has an occurrence in is left alone.
//
// This is a write driven by a read, on purpose: the app runs on a laptop
// that is off for days at a time, so a scheduler on the first of the month
// would need catch-up logic for every month it slept through.
func (s *BillService) Materialize(ctx context.Context, ym domain.YearMonth) (int, error) {
	latest, err := s.bills.LatestPerSeries(ctx)
	if err != nil {
		return 0, err
	}
	created := 0
	for _, last := range latest {
		current := last
		for created < materializeCap {
			currentMonth := domain.YearMonthOf(current.DueDate)
			if !currentMonth.Before(ym) {
				break
			}
			next := current.NextOccurrence(current.AmountEstimated)
			saved, err := s.bills.Create(ctx, next)
			if err != nil {
				return created, err
			}
			created++
			current = saved
		}
	}
	return created, nil
}
```

O `estimated` da ocorrência nova acompanha o da anterior: uma série cujo
valor o dono nunca confirma segue estimada, e confirmar o valor de um mês
(limpando `AmountEstimated`) faz o mês seguinte nascer com o valor
confirmado e já não estimado.

- [ ] **Step 4: Rodar e ver passar**

Run: `cd backend && go test ./internal/service/ -count=1 -v -run TestMaterialize`
Expected: PASS nos cinco testes.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/service/bill_service.go backend/internal/service/bill_service_test.go
git commit -m "feat(contas): materializar as contas recorrentes do mês ao abri-lo"
```

---

### Task 4: Quitar vira lançamento

**Files:**
- Modify: `backend/internal/store/postgres/transaction_repo.go` (extrair `insertTransactions`)
- Modify: `backend/internal/store/postgres/bill_repo.go` (`Pay`, `Unpay`)
- Modify: `backend/internal/service/bill_service.go` (`Pay` com as regras)
- Modify: `backend/internal/httpapi/dto_bills.go`, `handlers_bills.go`, `router.go`
- Test: `backend/internal/store/postgres/bill_repo_test.go` (acrescentar)

**Interfaces:**
- Consumes: `domain.CompetenceMonth(purchaseDate, method, card)` da Fase 1; `domain.NewInstallmentPurchase` **não** é usado aqui (uma conta paga é sempre uma parcela única).
- Produces:
  - `func insertTransactions(ctx context.Context, tx pgx.Tx, txns []domain.Transaction) error`
  - `func (r *BillRepo) Pay(ctx context.Context, id string, paidAt time.Time, expense domain.Transaction) (domain.Bill, error)`
  - `func (r *BillRepo) Unpay(ctx context.Context, id string) (domain.Bill, error)`
  - `service.PaymentInput` e `func (s *BillService) Pay(ctx context.Context, id string, in PaymentInput) (domain.Bill, domain.Transaction, error)`

- [ ] **Step 1: Escrever o teste que falha**

Acrescentar a `backend/internal/store/postgres/bill_repo_test.go`:

```go
// Paying a bill has to record the expense in the same database transaction:
// a bill marked paid with no matching transaction is exactly the balance bug
// this phase exists to fix.
func TestBillRepoPayWritesTheBillAndTheExpenseTogether(t *testing.T) {
	repo, categories, ctx := billRepoForTest(t)

	cat, err := categories.Create(ctx, domain.Category{Name: "Categoria de teste (pagamento)", Nature: "essencial", Color: "#654321"})
	if err != nil {
		t.Fatalf("create category: %v", err)
	}
	defer categories.Delete(ctx, cat.ID)

	method := domain.PaymentPix
	bill, err := repo.Create(ctx, domain.Bill{
		Description: "Conta de teste (pagamento)", AmountCents: 5000,
		DueDate:   time.Date(2026, time.September, 10, 0, 0, 0, 0, time.UTC),
		Direction: domain.BillPayable, CategoryID: &cat.ID, PaymentMethod: &method,
	})
	if err != nil {
		t.Fatalf("create bill: %v", err)
	}
	defer repo.Delete(ctx, bill.ID)

	expense := domain.Transaction{
		ID: domain.NewID(), Description: "Conta de teste (pagamento)", AmountCents: 5000,
		CategoryID: cat.ID, PaymentMethod: domain.PaymentPix,
		PurchaseDate:    time.Date(2026, time.September, 10, 0, 0, 0, 0, time.UTC),
		CompetenceMonth: domain.YearMonth{Year: 2026, Month: 9},
		InstallmentTotal: 1, IsRecurring: false,
	}
	defer NewTransactionRepo(repoDB(t, repo)).Delete(ctx, expense.ID)

	paid, err := repo.Pay(ctx, bill.ID, time.Now(), expense)
	if err != nil {
		t.Fatalf("pay: %v", err)
	}
	if paid.PaidAt == nil {
		t.Error("a conta não ficou paga")
	}

	txns, err := NewTransactionRepo(repoDB(t, repo)).ListByCompetenceMonth(ctx, domain.YearMonth{Year: 2026, Month: 9})
	if err != nil {
		t.Fatalf("list transactions: %v", err)
	}
	found := false
	for _, row := range txns {
		if row.ID == expense.ID {
			found = true
		}
	}
	if !found {
		t.Error("o lançamento não foi criado junto com o pagamento")
	}
}
```

`repoDB` é um helper que você precisa escrever para pegar o `*DB` a partir
do teste — o mais simples é `billRepoForTest` passar a devolver também o
`*DB` e o teste usar esse valor. Ajuste a assinatura do helper em vez de
inventar acesso ao campo privado.

Confirme os nomes reais de `domain.NewID`, dos campos de `domain.Transaction`
e de `TransactionRepo.Delete` antes de rodar; se algum diferir, o teste é
que se ajusta ao repo.

- [ ] **Step 2: Rodar e ver falhar**

Run:
```bash
cd backend && DATABASE_URL="postgres://estus:estus@localhost:5434/estus_vault?sslmode=disable" \
  go test ./internal/store/postgres/ -count=1 -run TestBillRepoPay
```
Expected: FAIL na compilação — `Pay` não existe.

- [ ] **Step 3: Extrair o insert de lançamentos**

Em `backend/internal/store/postgres/transaction_repo.go`, mover o corpo do
laço de `CreateBatch` para uma função de pacote, no mesmo estilo que
`diet_repo.go` e `training_repo.go` já usam para `insertMealItems` e
`insertExercises`:

```go
// insertTransactions writes txns inside an already-open database
// transaction, so a caller that has more to write in the same commit — a
// bill being settled, say — can reuse the exact same insert.
func insertTransactions(ctx context.Context, tx pgx.Tx, txns []domain.Transaction) error {
	for _, t := range txns {
		_, err := tx.Exec(ctx, `
			insert into transactions (
				id, description, amount_cents, category_id, payment_method,
				purchase_date, credit_card_id, competence_month,
				installment_group_id, installment_number, installment_total,
				is_recurring
			) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
			t.ID, t.Description, int64(t.AmountCents), t.CategoryID, t.PaymentMethod,
			t.PurchaseDate, t.CreditCardID, t.CompetenceMonth.FirstDay(),
			t.InstallmentGroupID, t.InstallmentNumber, t.InstallmentTotal,
			t.IsRecurring,
		)
		if err != nil {
			return fmt.Errorf("insert transaction %s: %w", t.Description, err)
		}
	}
	return nil
}
```

e faça `CreateBatch` chamá-la, mantendo o `Begin`/`Rollback`/`Commit` dele:

```go
func (r *TransactionRepo) CreateBatch(ctx context.Context, txns []domain.Transaction) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction batch: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := insertTransactions(ctx, tx, txns); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
```

- [ ] **Step 4: Implementar Pay e Unpay no repositório**

Em `backend/internal/store/postgres/bill_repo.go`:

```go
// Pay settles a bill and records its expense in one commit. Either both
// land or neither does: a bill marked paid with no transaction behind it is
// money that vanished from the budget.
func (r *BillRepo) Pay(ctx context.Context, id string, paidAt time.Time, expense domain.Transaction) (domain.Bill, error) {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return domain.Bill{}, fmt.Errorf("begin pay bill: %w", err)
	}
	defer tx.Rollback(ctx)

	var b domain.Bill
	err = tx.QueryRow(ctx, `
		update bills set paid_at = $2, amount_cents = $3, amount_estimated = false
		where id = $1 and paid_at is null
		returning id, description, amount_cents, due_date, direction, category_id,
			paid_at, series_id, amount_estimated, payment_method, created_at`,
		id, paidAt, int64(expense.AmountCents),
	).Scan(&b.ID, &b.Description, &b.AmountCents, &b.DueDate, &b.Direction, &b.CategoryID,
		&b.PaidAt, &b.SeriesID, &b.AmountEstimated, &b.PaymentMethod, &b.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Bill{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Bill{}, fmt.Errorf("pay bill %s: %w", id, err)
	}
	if err := insertTransactions(ctx, tx, []domain.Transaction{expense}); err != nil {
		return domain.Bill{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Bill{}, fmt.Errorf("commit pay bill %s: %w", id, err)
	}
	return b, nil
}
```

O `where paid_at is null` é o que impede pagar duas vezes: a segunda
tentativa não casa linha nenhuma e vira `ErrNotFound` em vez de criar um
lançamento duplicado.

`Unpay` é o espelho: apaga o lançamento pela descrição e data não dá — use
uma coluna nova `bills.transaction_id uuid references transactions(id)`,
gravada no `Pay` e lida no `Unpay` para apagar exatamente aquele lançamento.
**Acrescente essa coluna à migration 0019 da Task 2** (`alter table bills add
column transaction_id uuid references transactions(id);` no up, e o drop
correspondente no down) — sem ela não há como desfazer um pagamento sem
adivinhar qual lançamento era.

```go
// Unpay reverses Pay: the bill goes back to pending and the expense it
// created is removed, in one commit.
func (r *BillRepo) Unpay(ctx context.Context, id string) (domain.Bill, error) {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return domain.Bill{}, fmt.Errorf("begin unpay bill: %w", err)
	}
	defer tx.Rollback(ctx)

	var transactionID *string
	err = tx.QueryRow(ctx, `
		update bills set paid_at = null, transaction_id = null
		where id = $1 and paid_at is not null
		returning transaction_id`, id).Scan(&transactionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Bill{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Bill{}, fmt.Errorf("unpay bill %s: %w", id, err)
	}
	if transactionID != nil {
		if _, err := tx.Exec(ctx, `delete from transactions where id = $1`, *transactionID); err != nil {
			return domain.Bill{}, fmt.Errorf("delete expense of bill %s: %w", id, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Bill{}, fmt.Errorf("commit unpay bill %s: %w", id, err)
	}
	return r.Get(ctx, id)
}
```

Se `BillRepo` não tiver um `Get`, escreva um no mesmo molde do `Get` de
`creditcard_repo.go`. O `returning transaction_id` do update precisa que a
coluna exista — confira que a migration da Task 2 foi atualizada.

- [ ] **Step 5: Implementar Pay no serviço**

Em `backend/internal/service/bill_service.go`:

```go
// PaymentInput is what the owner confirms when settling a bill: the amount
// that actually left the account, and how.
type PaymentInput struct {
	PaidAt        time.Time
	AmountCents   domain.Cents
	CategoryID    string
	PaymentMethod domain.PaymentMethod
	CreditCardID  *string
}

// Pay settles a payable bill and records the expense behind it. A
// receivable is settled with MarkPaid instead: the transactions ledger is
// an expense ledger, and a credit there would be a negative every aggregate
// would have to special-case.
func (s *BillService) Pay(ctx context.Context, id string, in PaymentInput) (domain.Bill, domain.Transaction, error) {
	bill, err := s.bills.Get(ctx, id)
	if err != nil {
		return domain.Bill{}, domain.Transaction{}, err
	}
	if bill.Direction != domain.BillPayable {
		return domain.Bill{}, domain.Transaction{}, fmt.Errorf("%w: only a payable bill records an expense", domain.ErrValidation)
	}
	if in.AmountCents <= 0 {
		return domain.Bill{}, domain.Transaction{}, fmt.Errorf("%w: amount must be positive", domain.ErrValidation)
	}
	if !in.PaymentMethod.Valid() {
		return domain.Bill{}, domain.Transaction{}, fmt.Errorf("%w: invalid payment method %q", domain.ErrValidation, in.PaymentMethod)
	}
	if in.PaymentMethod == domain.PaymentCredit && (in.CreditCardID == nil || *in.CreditCardID == "") {
		return domain.Bill{}, domain.Transaction{}, domain.ErrCreditCardMissing
	}
	if _, err := s.categories.Get(ctx, in.CategoryID); err != nil {
		return domain.Bill{}, domain.Transaction{}, fmt.Errorf("category %s: %w", in.CategoryID, err)
	}

	var card *domain.CreditCard
	if in.PaymentMethod == domain.PaymentCredit {
		c, err := s.cards.Get(ctx, *in.CreditCardID)
		if err != nil {
			return domain.Bill{}, domain.Transaction{}, fmt.Errorf("credit card %s: %w", *in.CreditCardID, err)
		}
		card = &c
	}

	expense := domain.Transaction{
		ID:               domain.NewID(),
		Description:      bill.Description,
		AmountCents:      in.AmountCents,
		CategoryID:       in.CategoryID,
		PaymentMethod:    in.PaymentMethod,
		PurchaseDate:     in.PaidAt,
		CreditCardID:     in.CreditCardID,
		CompetenceMonth:  domain.CompetenceMonth(in.PaidAt, in.PaymentMethod, card),
		InstallmentTotal: 1,
		IsRecurring:      bill.Recurring(),
	}
	paid, err := s.bills.Pay(ctx, id, in.PaidAt, expense)
	if err != nil {
		return domain.Bill{}, domain.Transaction{}, err
	}
	return paid, expense, nil
}
```

`BillService` precisa do repositório de cartões para o ramo do crédito:
acrescente `cards *postgres.CreditCardRepo` ao struct e ao construtor, e
atualize a chamada de `NewBillService` em `backend/cmd/api/main.go`. Inclua
`Get` e `Pay` na interface `billStore` da Task 3, e os métodos
correspondentes no `fakeBills` daquele teste.

- [ ] **Step 6: Rota**

Em `dto_bills.go`, o corpo do pagamento:

```go
type payBillRequest struct {
	PaidOn        string  `json:"paid_on"`        // AAAA-MM-DD
	AmountCents   int64   `json:"amount_cents"`
	CategoryID    string  `json:"category_id"`
	PaymentMethod string  `json:"payment_method"`
	CreditCardID  *string `json:"credit_card_id"`
}
```

Em `handlers_bills.go`, um handler `Pay` que decodifica isso, converte
`PaidOn` com `time.Parse("2006-01-02", ...)` e chama `s.Pay`, devolvendo
200 com o DTO da conta. E um handler `Unpay` que chama `s.Unpay`. Em
`router.go`, ao lado das rotas de bills já existentes:

```go
		mux.HandleFunc("POST /api/bills/{id}/pay", b.Pay)
		mux.HandleFunc("DELETE /api/bills/{id}/paid", b.Unpay)
```

A rota `POST /api/bills/{id}/paid` que já existe continua servindo as contas
a receber; não a remova.

- [ ] **Step 7: Rodar tudo e ver passar**

Run:
```bash
cd backend && go build ./... && \
  DATABASE_URL="postgres://estus:estus@localhost:5434/estus_vault?sslmode=disable" go test ./... -count=1
```
Expected: PASS em todos os pacotes.

- [ ] **Step 8: Conferir que o banco ficou limpo**

```bash
docker exec estus-vault-postgres-1 psql -U estus -d estus_vault \
  -c "select count(*) from bills;" -c "select count(*) from transactions;" \
  -c "select name from categories order by name;"
```
Expected: zero contas, zero lançamentos, as 7 categorias do dono. Qualquer
sobra é defeito a reportar, não a apagar em silêncio.

- [ ] **Step 9: Commit**

```bash
git add backend/internal/store/postgres/transaction_repo.go backend/internal/store/postgres/bill_repo.go \
        backend/internal/store/postgres/bill_repo_test.go backend/internal/service/bill_service.go \
        backend/internal/service/bill_service_test.go backend/internal/httpapi/dto_bills.go \
        backend/internal/httpapi/handlers_bills.go backend/internal/httpapi/router.go \
        backend/cmd/api/main.go backend/migrations/0019_recurring_bills.up.sql \
        backend/migrations/0019_recurring_bills.down.sql
git commit -m "feat(contas): quitar uma conta a pagar registra a despesa no mesmo commit"
```

---

### Task 5: Contas por mês e campos de série

Sem harness de teste no frontend: a verificação é `tsc` mais o roteiro do
Step 5.

**Files:**
- Modify: `frontend/lib/bills.ts`
- Modify: `frontend/app/financeiro/contas/page.tsx`
- Modify: `frontend/app/financeiro/contas/actions.ts`
- Modify: `frontend/components/BillForm.tsx`, `NewBillModal.tsx`, `BillsBoard.tsx`
- Modify: `frontend/app/financeiro/contas/bills.css`

**Interfaces:**
- Consumes (Tasks 2 e 4): `GET /api/bills?month=AAAA-MM`, os campos
  `series_id`, `amount_estimated`, `payment_method` no DTO.
- Produces: a rota `/financeiro/contas?month=AAAA-MM`.

- [ ] **Step 1: Mês na página**

Leia `frontend/app/financeiro/categorias/page.tsx` antes: é o molde exato de
uma aba do Financeiro com mês. Em `contas/page.tsx`, receber
`searchParams: Promise<{ month?: string }>`, resolver com
`params.month ?? currentYearMonth()`, e renderizar
`<TopBar month={month} basePath="/financeiro/contas" />` acima do conteúdo.
Passar o mês para `listBills`.

Em `frontend/lib/bills.ts`, `listBills` ganha o parâmetro do mês e o envia
como `?month=`.

- [ ] **Step 2: Campos de série no formulário**

Em `BillForm.tsx`, acrescentar:
- um switch **"Repetir todo mês"** (o componente `Switch` já é usado em
  `BillsBoard.tsx`; reuse-o);
- um switch **"O valor muda todo mês"**, visível só quando o primeiro está
  ligado, com a dica `A conta do mês seguinte nasce com o valor deste mês, para você corrigir quando ela chegar.`;
- um `<select>` de **categoria** e um de **forma de pagamento** (Débito,
  Crédito, Pix), obrigatórios quando "Repetir todo mês" está ligado e a
  direção é "pagar".

A mensagem de erro do cliente, quando faltar um dos dois, é exatamente:
`Conta que se repete precisa de categoria e forma de pagamento — é assim que ela vira gasto quando você quita.`

- [ ] **Step 3: Marca de valor estimado na lista**

Em `BillsBoard.tsx`, uma conta com `amount_estimated` mostra o valor seguido
de `· estimado` em `.bill-estimated`. Em `bills.css`:

```css
.bill-estimated {
  font-size: 11.5px;
  color: var(--ink-mute);
  margin-left: 6px;
}
```

O dono precisa distinguir "R$180 é o que a luz costuma vir" de "R$180 é o
que a luz veio".

- [ ] **Step 4: Typecheck**

Run: `cd frontend && npx tsc --noEmit`
Expected: sem saída.

- [ ] **Step 5: Verificar no navegador**

```bash
cd /Users/rafael/Projects/estus-vault && ./scripts/estus-brain.sh --rebuild
until curl -fsS -o /dev/null http://127.0.0.1:37887/financeiro/contas; do sleep 3; done
```

Em `http://localhost:37887/financeiro/contas`, confirmar um a um e reportar
o que viu:
1. A navegação por mês aparece e mudar de mês muda a URL para `?month=`.
2. Criar uma conta **avulsa** a pagar, sem repetição: aparece no mês do
   vencimento e não no seguinte.
3. Criar uma conta **recorrente** sem categoria: recusa com a frase do Step 2.
4. Criar uma conta recorrente com categoria e forma: aparece no mês.
5. Navegar para o mês seguinte: a recorrente nasceu lá sozinha, pendente.
6. Voltar e avançar de novo: continua **uma só** — não duplicou.
7. Apagar as contas de teste que você criou e conferir no banco:
   ```bash
   docker exec estus-vault-postgres-1 psql -U estus -d estus_vault -c "select count(*) from bills;"
   ```
   Deve voltar a zero.

- [ ] **Step 6: Commit**

```bash
git add frontend/lib/bills.ts frontend/app/financeiro/contas frontend/components/BillForm.tsx \
        frontend/components/NewBillModal.tsx frontend/components/BillsBoard.tsx
git commit -m "feat(contas): navegação por mês e campos de conta recorrente"
```

---

### Task 6: Confirmação de pagamento e tile de gastos fixos

**Files:**
- Modify: `frontend/components/BillsBoard.tsx`
- Modify: `frontend/app/financeiro/contas/actions.ts`
- Modify: `frontend/lib/bills.ts`
- Modify: `frontend/app/financeiro/page.tsx`

**Interfaces:**
- Consumes (Task 4): `POST /api/bills/{id}/pay` e `DELETE /api/bills/{id}/paid`.
- Produces: nada — é a ponta da cadeia.

- [ ] **Step 1: Confirmação ao quitar**

Marcar uma conta **a pagar** como paga passa a abrir o `Modal` (o mesmo
componente que `NewBillModal` usa) com:
- **Valor** pré-preenchido com o da conta, editável — é o valor que
  realmente saiu;
- **Categoria** pré-selecionada com a da conta;
- **Forma de pagamento** pré-selecionada com a da conta;
- **Cartão**, visível só quando a forma é Crédito;
- **Data do pagamento**, pré-preenchida com hoje.

Confirmar chama a action nova, que chama `POST /api/bills/{id}/pay`. Conta
**a receber** continua no caminho de hoje (`POST /api/bills/{id}/paid`), sem
modal.

Em `actions.ts`, apenas funções async exportadas — a montagem do corpo fica
em helper privado. Erro 422 do backend deve chegar ao usuário como a
mensagem do corpo, não como o erro cru do fetch: reuse o mesmo tratamento
que `frontend/app/financeiro/cartoes/actions.ts` faz.

- [ ] **Step 2: Desfazer pagamento**

Uma conta paga ganha a ação **"Desfazer pagamento"**, que chama
`DELETE /api/bills/{id}/paid` e remove o lançamento junto. O `confirm()` diz:
`Desfazer o pagamento? O lançamento criado por ele também será apagado.`

- [ ] **Step 3: Tile de gastos fixos**

Em `frontend/app/financeiro/page.tsx`, acrescentar ao `kpi-grid`, depois do
tile de Saídas:

```tsx
        <div className="tile">
          <p className="tile-label">Gastos fixos</p>
          <p className="tile-figure tab">{summary.recurring.formatted}</p>
          <p className="tile-sub">do total do mês</p>
        </div>
```

`summary.recurring` já existe em `lib/types.ts:69` e já é calculado pelo
backend — nunca foi desenhado.

- [ ] **Step 4: Typecheck**

Run: `cd frontend && npx tsc --noEmit`
Expected: sem saída.

- [ ] **Step 5: Verificar no navegador**

```bash
cd /Users/rafael/Projects/estus-vault && ./scripts/estus-brain.sh --rebuild
until curl -fsS -o /dev/null http://127.0.0.1:37887/financeiro/contas; do sleep 3; done
```

⚠️ Isto grava no banco real do dono. Crie apenas contas com "(teste)" no
nome e apague tudo ao final, conferindo no banco.

1. Criar uma conta recorrente a pagar de R$100, categoria qualquer, Pix.
2. Marcar como paga: o modal abre com valor, categoria e forma preenchidos.
3. Mudar o valor para R$120 e confirmar.
4. Em **Lançamentos**, o gasto de R$120 aparece com a descrição da conta.
5. No **Dashboard**, o tile "Gastos fixos" mostra R$120 e "Saídas" também
   passou a contá-lo — era esse o bug do saldo.
6. **Desfazer pagamento**: a conta volta a pendente e o lançamento some de
   Lançamentos.
7. Apagar as contas de teste e conferir:
   ```bash
   docker exec estus-vault-postgres-1 psql -U estus -d estus_vault \
     -c "select count(*) from bills;" -c "select count(*) from transactions;"
   ```
   Ambos devem voltar a zero.

- [ ] **Step 6: Commit**

```bash
git add frontend/components/BillsBoard.tsx frontend/app/financeiro/contas/actions.ts \
        frontend/lib/bills.ts frontend/app/financeiro/page.tsx
git commit -m "feat(contas): confirmar pagamento vira lançamento, e tile de gastos fixos"
```

---

## Self-Review

**Cobertura da spec:**

| Requisito | Tarefa |
|---|---|
| `series_id`, `amount_estimated`, `payment_method` | 2 |
| Coluna `recurring` removida | 2 |
| Sem tabela de modelo; molde é a última ocorrência | 1 (`NextOccurrence`) + 3 (`LatestPerSeries`) |
| Categoria e forma obrigatórias em série a pagar | 1 (`Validate`) + 5 (formulário) |
| Dia 31 em mês de 30 cai no último dia | 1 |
| Materializar ao abrir o mês, idempotente | 3 |
| Ordem encadeada, herdando correções | 3 |
| Não inventa passado anterior à série | 3 |
| Série encerrada não gera | 3 (`LatestPerSeries` só vê séries com ocorrência viva) |
| Teto de 24 meses | 3 |
| Quitar grava conta e lançamento no mesmo commit | 4 |
| Só a pagar vira lançamento; `Pay` convive com `MarkPaid` | 4 |
| Lançamento com `is_recurring` e competência da Fase 1 | 4 |
| Desfazer apaga os dois | 4 (`Unpay`) |
| Contas com navegação por mês | 5 |
| Marca de valor estimado | 5 |
| Confirmação ao quitar, com campos preenchidos | 6 |
| Tile de gastos fixos | 6 |

**Sem placeholders:** todo passo de código traz o código. Os pontos em que
mando conferir um nome contra o repo (`newID`, `CategoryRepo.Create`,
`domain.NewID`, `TransactionRepo.Delete`, as assinaturas da `billStore`) são
verificações contra código existente, não decisões em aberto.

**Consistência de tipos:** `Materialize` devolve `(int, error)` na Task 3 e é
usada assim nos testes. `Pay` do repositório recebe
`(ctx, id, paidAt, expense)` e é chamada assim pelo serviço na Task 4.
`insertTransactions(ctx, tx, txns)` é definida na Task 4 Step 3 e usada no
Step 4. `PaymentInput` tem os mesmos cinco campos no serviço e no
`payBillRequest` do DTO.

**Um defeito que corrigi durante a revisão:** a Task 4 precisa de uma coluna
`bills.transaction_id` para saber qual lançamento apagar ao desfazer um
pagamento, e essa coluna pertence à migration escrita na Task 2 — reescrever
uma migration já aplicada é pior do que escrevê-la certa na primeira vez. A
Task 2 Step 1 agora a contém, no up e no down, e `domain.Bill` ganha o campo
`TransactionID *string` correspondente junto dos outros três (Task 1 Step 3),
lido e escrito em todas as listas de colunas do repositório (Task 2 Step 4).
