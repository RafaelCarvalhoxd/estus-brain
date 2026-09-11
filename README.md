# Estus Vault

Seu app pessoal — não só de finanças. Hoje tem cinco módulos e uma home:

- **Visão geral** (`/`) — não é um módulo, é um mosaico: o essencial de cada
  módulo abaixo (gasto do mês, contas em aberto, próximos lembretes, próximos
  eventos, notas recentes), cada card levando direto pra tela cheia daquilo.
- **Financeiro** (`/financeiro`) — um módulo com quatro seções por abas:
  - *Dashboard* — o pulso do mês: total gasto (com variação vs. mês anterior),
    gasto por categoria, ritmo semanal.
  - *Lançamentos* — o extrato completo do mês e o formulário de novo
    lançamento, com a regra que motivou o projeto: uma compra no crédito só
    conta como despesa no mês seguinte ao da compra, nunca no mês da compra
    em si.
  - *Categorias* — o detalhamento: gasto por categoria e a comparação mês a
    mês, categoria a categoria.
  - *Contas* — contas a pagar e a receber, com status derivado (atrasado/
    pendente/pago) a partir da data de vencimento — separado do extrato de
    lançamentos porque é sobre o que ainda vai acontecer, não sobre o que já
    aconteceu.
- **Senhas** (`/senhas`) — um cofre de senhas criptografado (AES-256-GCM), que
  só revela uma senha depois de uma checagem biométrica (Touch ID/Face ID)
  via WebAuthn.
- **Notas** (`/notas`) — notas soltas, fixáveis.
- **Lembretes** (`/lembretes`) — lembretes simples com data opcional,
  agrupados por atrasado/hoje/próximos.
- **Agenda** (`/agenda`) — eventos locais, com sincronização opcional (via
  OAuth) com o Google Calendar.

A arquitetura não assume "só finanças": o backend é um serviço isolado com
sua própria API, cada módulo mora em arquivos próprios (domínio, repositório,
serviço, handler, tela) e um módulo em falta (ex.: sem `VAULT_ENCRYPTION_KEY`)
não derruba os outros — o servidor simplesmente não monta aquelas rotas.

## Arquitetura

```
┌─────────────┐        server-side fetch        ┌──────────────┐        ┌────────────┐
│   Browser   │ ───────────────────────────────▶│   Next.js    │───────▶│  Go API    │───▶ Postgres
│  (você)     │◀─────────── HTML/RSC ────────────│  (frontend)  │◀───────│ (backend)  │
└─────────────┘                                  └──────────────┘        └────────────┘
```

- **O navegador nunca fala com o Go diretamente**, com uma única exceção: o
  fluxo de WebAuthn (Touch ID/Face ID) precisa rodar no navegador de verdade,
  então esse caminho específico passa por Route Handlers do Next.js
  (`app/api/vault-webauthn/...`, `app/api/google/oauth/start`) que só
  repassam a chamada para o Go — o Go continua nunca exposto publicamente.
  Toda leitura normal acontece em Server Components, toda escrita normal via
  Server Action.
- **Dinheiro é inteiro, sempre.** `amount_cents bigint` no Postgres,
  `domain.Cents int64` no Go. Nenhuma soma financeira passa por `float`.
- **A regra da fatura é uma função pura e testada:**
  `domain.CompetenceMonth` (`backend/internal/domain/billing.go`).
- **Senhas nunca ficam em texto puro em repouso.** `vault_entries` guarda
  `password_ciphertext`/`password_nonce` (AES-256-GCM); a chave mestra vem
  de `VAULT_ENCRYPTION_KEY` (variável de ambiente, nunca no banco). A rota de
  listagem nunca seleciona essas colunas — só a rota de "revelar", que exige
  uma verificação WebAuthn válida por chamada.
- **Backend em camadas, um handler/serviço/repo por módulo:** `domain`
  (regras de negócio, sem I/O) → `store/postgres` (SQL explícito, sem ORM) →
  `service` (orquestra repositórios) → `httpapi` (HTTP puro, sem framework —
  `net/http` do Go 1.22+ resolve rotas com padrão `GET /api/months/{month}`).
  Cada módulo define seu próprio tipo de handlers (`BillHandlers`,
  `VaultHandlers`, `NoteHandlers`, `ReminderHandlers`, `EventHandlers`) e o
  `router.go` só monta as rotas de um módulo se ele foi construído com
  sucesso em `main.go` — é assim que o vault "desliga sozinho" sem
  `VAULT_ENCRYPTION_KEY`.

## Rodando localmente

Pré-requisitos: Go 1.26+, Node 20+, Docker (para o Postgres).

```bash
# 1. sobe o Postgres
docker compose up -d postgres

# 2. backend (aplica as migrations sozinho ao subir)
cd backend
cp .env.example .env
# preencha pelo menos VAULT_ENCRYPTION_KEY (openssl rand -base64 32)
# se quiser o cofre de senhas ativo; sem isso o resto do app funciona igual.
export $(cat .env | xargs)
go run ./cmd/api

# 3. frontend, em outro terminal
cd frontend
echo "API_URL=http://localhost:8080" > .env.local
npm install
npm run dev
```

Abra `http://localhost:3000`. O seed (`backend/migrations/0002_seed.up.sql`)
já cria as 7 categorias e um cartão.

Para rodar tudo containerizado: `docker compose up --build` (o
`docker-compose.yml` ainda não passa as variáveis do vault/agenda para o
container do backend — adicione-as em `environment:` do serviço `backend`
quando for usar esses módulos containerizados).

## Configurando o cofre de senhas (Touch ID/Face ID)

1. Defina `VAULT_ENCRYPTION_KEY`, `WEBAUTHN_RP_ID` e `WEBAUTHN_RP_ORIGIN` no
   `.env` do backend (valores de exemplo já em `.env.example`, funcionam em
   `localhost` sem HTTPS — os navegadores isentam `localhost` da exigência de
   contexto seguro do WebAuthn).
2. Abra `/senhas` e clique em "Configurar Touch ID" uma vez — isso registra
   seu Mac/iPhone como autenticador.
3. A partir daí, "Revelar" em qualquer senha pede a checagem biométrica na
   hora, por chamada — não existe uma sessão "destravada" que fica aberta.
4. **Em produção**, atrás do seu proxy mTLS, `WEBAUTHN_RP_ORIGIN` precisa ser
   a origem HTTPS real (ex.: `https://vault.seudominio.com`) e `WEBAUTHN_RP_ID`
   o domínio efetivo — um valor errado aqui faz toda checagem falhar
   silenciosamente na verificação, não é só um aviso.

## Configurando a sincronização com Google Calendar

A Agenda funciona 100% localmente sem isso — a sincronização é opcional.
Para ativar:

1. No [Google Cloud Console](https://console.cloud.google.com/), crie (ou
   escolha) um projeto e ative a **Google Calendar API** em
   "APIs e serviços → Biblioteca".
2. Configure a tela de consentimento OAuth em
   "APIs e serviços → Tela de consentimento OAuth" (modo "Externo" funciona
   para uso pessoal — adicione sua própria conta Google como usuário de
   teste enquanto o app estiver em modo "Testando").
3. Crie uma credencial em "APIs e serviços → Credenciais → Criar
   credenciais → ID do cliente OAuth", tipo **Aplicativo da Web**, e
   registre `http://localhost:8080/api/google/oauth/callback` (ou a URL real
   do seu backend em produção) como URI de redirecionamento autorizada.
4. Copie o Client ID e o Client Secret gerados para `GOOGLE_CLIENT_ID` e
   `GOOGLE_CLIENT_SECRET` no `.env`, e confirme que `GOOGLE_REDIRECT_URL`
   bate exatamente com a URI registrada.
5. Em `/agenda`, clique em "Conectar Google Agenda" e depois em
   "Sincronizar agora".

## O que falta antes de "subir na infra"

1. **mTLS na borda** — um reverse proxy (Caddy/Traefik/Nginx) validando
   certificado de cliente na frente do Next.js. O Go nunca fica exposto
   publicamente nesse desenho.
2. **Backups do Postgres** — agora com mais dado sensível (senhas
   criptografadas inclusas) que antes; `pg_dump` agendado + guardar
   `VAULT_ENCRYPTION_KEY` em um cofre separado do backup do banco (backup do
   banco sem a chave é só ruído para quem não deveria ler; junte os dois e
   você perdeu a proteção).
3. **Gastos recorrentes automáticos** — hoje `is_recurring` é só uma flag
   manual por lançamento financeiro; não se recria sozinho todo mês.
4. **Push/dois-sentidos completo na Agenda** — a sincronização de hoje é
   "puxar do Google" sob demanda + "empurrar ao criar localmente";
   não há webhook de mudanças, então uma edição feita direto no Google só
   aparece aqui depois do próximo "Sincronizar agora".
5. **Gestão de cartões** — hoje um cartão só existe via seed/API; não tem
   tela para cadastrar um novo cartão, editar dia de fechamento/vencimento,
   ou ver a fatura por cartão quando houver mais de um.

## Testes

```bash
cd backend && go test ./...   # cobre competência de fatura, split de parcelas,
                               # criptografia do vault, status de contas/lembretes
cd frontend && npm run lint && npx tsc --noEmit
```
