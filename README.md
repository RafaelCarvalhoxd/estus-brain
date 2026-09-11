# Estus Vault

Controle financeiro pessoal: lançamentos, categorias, cartão de crédito com
parcelamento, e a regra que motivou o projeto — uma compra no crédito só
conta como despesa no mês seguinte ao da compra, nunca no mês da compra em
si. Hoje cobre só finanças; a arquitetura não assume isso — o backend é um
serviço isolado com sua própria API, então um segundo domínio no futuro
entra como um novo serviço (ou módulo), não como uma reforma deste.

## Arquitetura

```
┌─────────────┐        server-side fetch        ┌──────────────┐        ┌────────────┐
│   Browser   │ ───────────────────────────────▶│   Next.js    │───────▶│  Go API    │───▶ Postgres
│  (você)     │◀─────────── HTML/RSC ────────────│  (frontend)  │◀───────│ (backend)  │
└─────────────┘                                  └──────────────┘        └────────────┘
```

- **O navegador nunca fala com o Go diretamente.** Toda leitura acontece em
  Server Components (`app/page.tsx`), toda escrita via Server Action
  (`app/actions.ts`). Isso significa que o backend não precisa de CORS, não
  precisa de um listener público, e pode viver inteiramente atrás do mTLS —
  só o processo Next.js precisa alcançá-lo.
- **Dinheiro é inteiro, sempre.** `amount_cents bigint` no Postgres,
  `domain.Cents int64` no Go. Nenhuma soma financeira passa por `float`.
- **A regra da fatura é uma função pura e testada:**
  `domain.CompetenceMonth` (`backend/internal/domain/billing.go`), coberta
  por `billing_test.go`. É o único lugar do sistema que decide em qual mês
  uma despesa conta.
- **Backend em camadas:** `domain` (regras de negócio, sem I/O) →
  `store/postgres` (SQL explícito, sem ORM) → `service` (orquestra
  repositórios) → `httpapi` (HTTP puro, sem framework — `net/http` do Go
  1.22+ já resolve rotas com padrão `GET /api/months/{month}`).

## Rodando localmente

Pré-requisitos: Go 1.25+, Node 20+, Docker (para o Postgres).

```bash
# 1. sobe o Postgres
docker compose up -d postgres

# 2. backend (aplica as migrations sozinho ao subir)
cd backend
cp .env.example .env   # ajuste se mudou a porta do Postgres
export $(cat .env | xargs)
go run ./cmd/api

# 3. frontend, em outro terminal
cd frontend
echo "API_URL=http://localhost:8080" > .env.local
npm install
npm run dev
```

Abra `http://localhost:3000`. O seed (`backend/migrations/0002_seed.up.sql`)
já cria as 7 categorias e um cartão; lançamentos você cria pelo formulário
"Novo lançamento" ou via `POST /api/transactions`.

Para rodar tudo containerizado: `docker compose up --build`.

## O que falta antes de "subir na infra"

1. **mTLS na borda** — combinado à parte: um reverse proxy (Caddy/Traefik/
   Nginx) validando certificado de cliente na frente do Next.js. O Go nunca
   fica exposto publicamente nesse desenho, então só o Next precisa do
   certificado de servidor + verificação de cliente.
2. **Backups do Postgres** — é o único dado que importa nesse sistema;
   `pg_dump` agendado é suficiente para volume pessoal.
3. **Gastos recorrentes automáticos** — hoje `is_recurring` é só uma flag
   manual por lançamento (conta no tile "Recorrentes", mas não se recria
   sozinho todo mês). Uma tabela `recurring_rules` + um job mensal é o
   próximo passo natural quando isso incomodar.
4. **Página de cartões e categorias** — a navegação lateral já tem os
   links; só a Visão Geral está implementada.

## Testes

```bash
cd backend && go test ./...   # cobre a regra de competência e o split de parcelas
cd frontend && npm run lint && npx tsc --noEmit
```
