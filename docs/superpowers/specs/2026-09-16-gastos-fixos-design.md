# Gastos fixos como contas recorrentes — design

Data: 2026-09-16 · Status: aprovado em conversa, aguardando revisão do spec

Fase 2 de duas. A Fase 1 (cartão e competência pelo vencimento) já está na
`main`; esta não depende dela.

## Objetivo

Cadastrar um gasto que se repete todo mês — aluguel, luz, Netflix — uma vez
só, e ver a conta do mês seguinte nascer sozinha, com o valor já preenchido
quando ele é previsível e estimado quando varia. Quitar a conta passa a
registrar a despesa no orçamento, o que também conserta um erro de cálculo
que existe hoje no saldo.

## Decisões tomadas em conversa

- **Gasto fixo mora em Contas a Pagar**, não em Lançamentos. Um gasto fixo é
  uma *previsão* com vencimento e estado de pago; um lançamento é um *fato
  consumado*. Criar lançamentos futuros encheria o extrato de gastos que
  ainda não aconteceram e estragaria o "quanto gastei".
- **A conta do mês seguinte nasce quando o mês é aberto**, não num agendador.
  O app roda no notebook do dono, que fica desligado boa parte do tempo: um
  job na virada precisaria de recuperação, e de várias se o note ficasse
  semanas fora.
- **Valor fixo nasce pendente, não pago.** O app nunca afirma que saiu
  dinheiro que talvez não tenha saído.
- **Categoria e forma de pagamento ficam guardadas na conta** e são
  confirmadas na hora de quitar, não perguntadas do zero todo mês.
- **Contas ganha navegação por mês**, como as outras abas do Financeiro.

## O que existe hoje, e o que está quebrado

`bills` já tem CRUD completo (service, repo, rotas, tela) e uma coluna
`recurring boolean`. **Essa coluna é puramente decorativa**: nada a lê para
gerar a conta seguinte, e `MarkPaid` só grava `paid_at`.

Dois defeitos que esta fase corrige:

1. **O saldo do dashboard soma fontes assimétricas.** "Entradas" vem de contas
   recebidas (`getBillsReceived`), "Saídas" vem só de lançamentos. Uma conta
   a pagar quitada some do "em aberto" e **nunca entra como despesa** —
   `frontend/app/financeiro/page.tsx:27-29`.
2. **O total de gastos fixos é calculado e nunca desenhado.**
   `RecurringTotal` (`transaction_repo.go:149`) chega ao frontend como
   `summary.recurring` (`lib/types.ts:69`) e nenhum componente o lê.

Também existe um segundo conceito de "recorrente" — `transactions.is_recurring`
— igualmente decorativo. Esta fase **não** cria um terceiro: `is_recurring`
passa a ser o que o lançamento gerado por uma conta recorrente carrega, e é
assim que `RecurringTotal` continua somando o total certo.

## Dados

Migration `0019_recurring_bills`.

`bills` ganha três colunas:

| coluna | tipo | uso |
|---|---|---|
| `series_id` | uuid null | liga as ocorrências de uma mesma conta ao longo dos meses; null = conta avulsa |
| `amount_estimated` | boolean not null default false | o valor veio do mês anterior e ainda não foi confirmado |
| `payment_method` | text null, check igual ao de transactions | forma de pagamento padrão, usada ao quitar |

Índice `bills_series_due_idx on bills (series_id, due_date)` — a geração
pergunta "esta série já tem ocorrência neste mês?" a cada abertura de mês.

**Não existe tabela de modelo.** O molde de uma série é a sua **última
ocorrência**: o mês novo copia o anterior. Uma tabela a menos para
dessincronizar, e editar a conta deste mês naturalmente define como a do
mês que vem vai nascer.

A coluna `recurring` existente **é removida** na mesma migration:
`series_id is not null` passa a ser a definição de recorrente. Não há
conversão de dados a fazer — a tabela `bills` está vazia desde 2026-09-16,
quando o dono zerou o banco para começar a usar o app de verdade.

## Domínio

`domain.Bill` ganha `SeriesID *string`, `AmountEstimated bool` e
`PaymentMethod *PaymentMethod`, e perde `Recurring`.

```go
// Recurring reports whether this bill is one occurrence of a monthly series.
func (b Bill) Recurring() bool { return b.SeriesID != nil }

// NextOccurrence is the bill that continues b's series in the month after
// b's own, copying everything but the amount's confirmation: a series whose
// amount varies carries the last value forward as an estimate.
func (b Bill) NextOccurrence(estimated bool) Bill
```

O dia do vencimento vem do `due_date` da ocorrência anterior. Um vencimento
no dia 31 num mês de 30 dias cai no **último dia do mês** — nunca escorrega
para o mês seguinte, o que mudaria silenciosamente a competência.

`Validate` passa a exigir `CategoryID` e `PaymentMethod` quando
`SeriesID != nil` **e** `Direction == BillPayable`: é essa combinação que
vira lançamento ao ser quitada, e lançamento exige os dois. Uma série a
receber não precisa de nenhum dos dois, porque nunca gera lançamento.

## Geração ao abrir o mês

Uma função nova no serviço, chamada por `List` quando um mês é pedido:

```go
// Materialize creates the missing occurrences of every active series up to
// and including ym, oldest first, each copied from the one before it. It is
// idempotent: a month already materialized is left alone.
func (s *BillService) Materialize(ctx context.Context, ym domain.YearMonth) error
```

Regras:

- **Uma série está ativa** enquanto sua última ocorrência não foi apagada.
  Apagar a última ocorrência de uma série encerra a série — é como o dono
  diz "cancelei a Netflix". As ocorrências passadas ficam, porque são
  histórico.
- **Ordem importa:** para chegar em dezembro partindo de setembro, cria
  outubro a partir de setembro, novembro a partir de outubro, dezembro a
  partir de novembro. Copiar sempre de setembro perderia uma correção de
  valor feita em outubro.
- **Nunca materializa o passado anterior à série.** Abrir um mês anterior à
  primeira ocorrência não inventa contas.
- **Teto de segurança:** no máximo 24 meses por chamada. Abrir um mês muito
  distante devolve o que couber no teto em vez de escrever centenas de
  linhas; o mês seguinte continua de onde parou.

Materializar é uma escrita disparada por uma leitura. É deliberado e fica
documentado no código: é o que faz o app funcionar depois de semanas
desligado, sem agendador e sem depender de o note estar ligado na virada.

## Quitar vira lançamento

`MarkPaid` passa a receber o que confirma o pagamento:

```go
type PaymentInput struct {
    PaidAt        time.Time
    AmountCents   Cents          // o valor realmente pago
    CategoryID    string
    PaymentMethod domain.PaymentMethod
    CreditCardID  *string        // exigido quando PaymentMethod == credito
}

// Pay settles a payable bill and records the expense in the same database
// transaction: either both happen or neither does.
func (s *BillService) Pay(ctx context.Context, id string, in PaymentInput) (domain.Bill, domain.Transaction, error)
```

- **Só conta a pagar vira lançamento.** A receber continua sendo quitada
  pelo `MarkPaid` atual, que só grava `paid_at`: o ledger de lançamentos é de
  despesa, e um recebimento ali seria um valor negativo que nenhuma agregação
  espera. `Pay` **não substitui** `MarkPaid`, convive com ele — o handler
  escolhe um ou outro pela direção da conta, e recusa `Pay` numa conta a
  receber.
- O lançamento nasce com `is_recurring = true` quando a conta pertence a uma
  série, e com a competência calculada pela regra da Fase 1 — inclusive o
  ciclo do cartão, quando a forma de pagamento é crédito.
- **Os dois gravam na mesma transação de banco.** Uma conta marcada como paga
  sem o lançamento correspondente é exatamente o erro de saldo que esta fase
  existe para consertar.
- Desfazer o pagamento apaga o lançamento junto, na mesma transação.

A tela abre uma confirmação curta com valor, categoria e forma **já
preenchidos** pela conta. Confirmar é um clique; ajustar é para o mês em que
o valor veio diferente — e ajustar o valor de uma ocorrência estimada
também limpa o `amount_estimated` dela, que é o que faz o mês seguinte
herdar o valor corrigido.

## Telas

**Contas** ganha a navegação por mês do resto do Financeiro (`TopBar` com
`basePath="/financeiro/contas"`), e a listagem passa a ser do mês em foco.
O formulário ganha "repetir todo mês", "o valor muda todo mês" e os campos
de categoria e forma de pagamento, obrigatórios quando é recorrente.

Uma ocorrência com valor estimado é marcada na lista — o dono precisa
distinguir "R$180 é o que a luz costuma vir" de "R$180 é o que a luz veio".

**Dashboard** ganha o tile de gastos fixos, alimentado pelo
`summary.recurring` que já existe e nunca foi desenhado.

## Testes

TDD, teste antes de cada pedaço de produção.

- **`NextOccurrence`**: dia 31 num mês de 30 dias; virada de ano; série de
  valor fixo versus estimado.
- **`Materialize`**: idempotência (duas chamadas, uma ocorrência); três meses
  de uma vez, cada um herdando a correção do anterior e não da primeira;
  mês anterior ao início da série não cria nada; série encerrada não gera;
  teto de 24 meses.
- **`Pay`**: cria conta e lançamento juntos; falha no lançamento não deixa a
  conta paga (a transação de banco desfaz); conta a receber não cria
  lançamento; crédito sem cartão é recusado; desfazer apaga os dois.
- **Repositório**: round-trip das colunas novas, contra o Postgres real,
  pulando sem `DATABASE_URL`, com limpeza por `defer` — nunca `t.Cleanup`
  junto de `defer db.Close()`, que roda com o pool já fechado e falha calado.
- **Handlers**: validação de recorrente sem categoria ou sem forma de
  pagamento (422), e o corpo de confirmação do pagamento.

## Fora de escopo

- Recorrência que não seja mensal (semanal, anual, a cada N meses).
- Fatura do cartão como entidade própria, e o `OpenInvoiceTotalAll`, que
  segue incoerente para cartões cujo vencimento cai antes do fechamento e
  segue não renderizado em lugar nenhum.
- Nome único de conta. Diferente do cartão, o assistente não resolve conta
  por nome hoje, então duas contas "Luz" não causam o erro silencioso que a
  constraint do cartão preveniu.
- Notificação do Telegram para conta a vencer. O bot já avisa lembretes e
  eventos; contas ficariam naturais ali, mas não foram pedidas.

## Uma consequência explícita

A mesma despesa passa a existir em dois lugares: a **conta** (a previsão) e o
**lançamento** (o fato). É o preço de separar previsto de realizado, e é a
consequência direta de gasto fixo morar em Contas a Pagar. Quem for somar
"quanto gastei" deve somar lançamentos, nunca contas — e é por isso que
`Pay` grava os dois juntos ou nenhum.
