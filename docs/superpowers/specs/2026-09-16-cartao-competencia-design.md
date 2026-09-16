# Cartão de crédito e competência pelo vencimento — design

Data: 2026-09-16 · Status: aprovado em conversa, aguardando revisão do spec

Fase 1 de duas. A Fase 2 (gastos fixos como contas recorrentes) tem spec
própria e não depende desta.

## Objetivo

Cadastrar cartões de crédito pelo app — hoje só nascem de um seed SQL — e
fazer o mês em que um gasto no crédito entra no orçamento respeitar o ciclo
real do cartão, em vez de cair sempre no mês seguinte.

## A decisão central: reverter uma regra deliberada

O código atual joga **todo** gasto no crédito para o mês seguinte, qualquer
que seja o dia da compra. Isso não é esquecimento: `domain/billing.go:35-45`
argumenta que misturar fechamento com competência faria "uma compra do dia 28
pular dois meses silenciosamente, o que não é o modelo mental de quem faz
orçamento semana a semana".

O argumento está errado, e esta spec o reverte. Quando o cartão fecha 25 e
vence 15, uma compra em 28/09 é cobrada mesmo em 15/11 — dois meses depois. O
comentário tratou a realidade como se fosse um defeito do cálculo. O dono
pediu explicitamente o comportamento real ("se um cartão fechou dia 10, não
vai mais entrar nesse mês").

O comentário antigo será substituído por um que explica a regra nova **e
registra que a anterior foi abandonada de propósito**, para que ninguém
"conserte" isso de volta mais tarde.

## A regra

Competência de um gasto no crédito = **mês do vencimento da fatura que o
cobra**. É o mês em que o dinheiro sai da conta, que é o que o orçamento
mede.

Dois passos, ambos usando só `closing_day` e `due_day` do cartão:

1. **Fechamento:** se o dia da compra ≤ `closing_day`, a fatura fecha nesse
   mês; senão, no mês seguinte.
2. **Vencimento:** se `due_day` > `closing_day`, vence no mesmo mês do
   fechamento; senão, no mês seguinte.

Débito e Pix não mudam: competência = o mês em que a compra aconteceu.

### Casos conferidos

Cartão que fecha 25 e vence 15 (o "Cartão principal" do seed):

| Compra | Fecha | Vence | Competência |
|---|---|---|---|
| 05/09 | 25/09 | 15/10 | outubro |
| 28/09 | 25/10 | 15/11 | novembro |

Cartão que fecha 10 e vence 20:

| Compra | Fecha | Vence | Competência |
|---|---|---|---|
| 05/09 | 10/09 | 20/09 | setembro |
| 15/09 | 10/10 | 20/10 | outubro |

### Escolhas de borda

- **Compra no próprio dia do fechamento** entra na fatura que fecha naquele
  dia (`≤`, não `<`). Aprovado em conversa.
- `closing_day` e `due_day` são 1-28 por constraint do banco desde o
  `0001_init`, então não existe o problema de "dia 31 em fevereiro". A
  constraint fica; o formulário do cartão a repete em vez de deixar o banco
  recusar com erro feio.
- Virada de ano sai de graça: as datas são montadas com `time.Date`, que
  normaliza mês 13 para janeiro do ano seguinte.

## Dados

**Nenhuma migration.** `credit_cards.closing_day`, `credit_cards.due_day` e
`transactions.competence_month` já existem (`0001_init.up.sql`). A tabela de
transações também já guarda a competência calculada na escrita, então nada
precisa ser recomputado em consulta.

Não há lançamento antigo para corrigir: a base foi zerada em 2026-09-16,
quando o dono começou a usar o app de verdade. A regra nova nasce valendo
para tudo que entrar.

## Domínio

`CreditCard` ganha o ciclo de faturamento como duas funções pequenas e
testáveis isoladamente:

```go
// ClosingDateFor é a data em que fecha a fatura que cobra uma compra feita em
// purchase.
func (c CreditCard) ClosingDateFor(purchase time.Time) time.Time

// DueDateFor é a data em que vence a fatura que fechou em closing.
func (c CreditCard) DueDateFor(closing time.Time) time.Time
```

`DueDateFor` hoje recebe um `YearMonth` e não tem **nenhum chamador** — é
código morto que descreve a regra antiga. Será redefinida com a assinatura
acima em vez de ganhar uma irmã.

`CompetenceMonth` passa a receber o cartão:

```go
// card é nil para débito e Pix.
func CompetenceMonth(purchaseDate time.Time, method PaymentMethod, card *CreditCard) YearMonth
```

Um `PaymentCredit` com `card` nil é um erro de programação, não uma entrada
do usuário: o serviço já recusa crédito sem cartão (`ErrCreditCardMissing`) e
já carrega o cartão antes de gravar. A função devolve o mês da compra nesse
caso, para não inventar uma data, e o teste fixa esse contrato.

`NewInstallmentPurchase` passa a receber `CreditCard` em vez de `cardID
string`. A primeira parcela usa a competência da regra nova; cada parcela
seguinte continua somando um mês — é assim que a fatura do cartão cobra, e
não muda.

## API

Completa o CRUD que hoje só tem leitura:

| rota | o que faz |
|---|---|
| `GET /api/credit-cards` | já existe |
| `POST /api/credit-cards` | cria: `name`, `closing_day`, `due_day` |
| `PUT /api/credit-cards/{id}` | edita os mesmos campos |
| `DELETE /api/credit-cards/{id}` | apaga |

Validação: nome não vazio; `closing_day` e `due_day` entre 1 e 28.

Apagar um cartão que tem lançamento é recusado. A FK
`transactions.credit_card_id` já impede no banco, mas hoje isso viraria um
500: o repositório passa a traduzir a violação de FK em `domain.ErrConflict`
com a mensagem de que o cartão tem lançamentos e por isso não pode ser
apagado.

**Editar `closing_day`/`due_day` não recalcula lançamentos já gravados.** A
competência é fotografada na escrita, de propósito — mexer no passado por
causa de uma correção de cadastro seria pior do que deixá-la. A tela avisa
isso em uma linha quando os dias são alterados.

## Frontend

Tela de cartões dentro de Financeiro: lista com nome, fechamento e
vencimento, e um formulário para criar/editar/apagar, seguindo o padrão já
usado em contas a pagar (`NewBillModal` / `BillForm`).

O `<select name="credit_card_id">` do lançamento (`NewTransactionForm.tsx:102`)
continua igual — passa a enxergar os cartões cadastrados porque a lista vem
da mesma rota.

## Testes

TDD, teste antes de cada pedaço de produção.

- **`ClosingDateFor` / `DueDateFor`**: tabela cobrindo compra antes, no dia e
  depois do fechamento; `due_day` maior e menor que `closing_day`; virada de
  ano (dezembro → janeiro).
- **`CompetenceMonth`**: as duas tabelas de "Casos conferidos" acima, viram
  teste literal; mais débito e Pix inalterados, e crédito com `card` nil.
- **`NewInstallmentPurchase`**: parcelada começando logo antes e logo depois
  do fechamento, conferindo a competência de cada parcela e que a soma das
  partes bate com o total.
- **Repositório de cartão**: round-trip de create/update/delete contra o
  Postgres real (o pacote já tem esse padrão, pulando sem `DATABASE_URL`), e
  delete recusado quando existe lançamento.
- **Handlers**: validação de dia fora de 1-28 e nome vazio.

## Fora de escopo

- Gastos fixos e contas recorrentes — Fase 2.
- Entidade "fatura" no banco, fatura por cartão na tela, status paga/em
  aberto. `OpenInvoiceTotalAll` continua como está: soma todos os cartões
  juntos e nunca é renderizado. Sob a regra nova ele segue coerente ("o que
  vence no mês que vem"), mas quem for mexer nisso faz na Fase 2.
- O bug do saldo do dashboard (entradas vêm de contas recebidas, saídas só de
  lançamentos, e conta paga nunca vira despesa) — é da Fase 2, que liga
  conta a pagar e lançamento.
