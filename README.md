# Estus Brain

Seu app pessoal — um cérebro com dez módulos em volta, rodando num servidor só seu:

- **Núcleo** (`/`) — não é um módulo, é a única porta de entrada (não há
  barra lateral): um cérebro 3D em WebGL (three.js, `components/brain/`)
  com os módulos em volta. Cada módulo mora numa região — frontal esquerdo
  = Financeiro, frontal direito = Hábitos, faixa motora = Treino, ínsula
  (paladar) = Dieta, parietal direito = Agenda, parietal esquerdo = Quadros,
  temporal = Notas, occipital = Lembretes, cerebelo = Senhas, núcleo/tronco
  = Documentos. Passar o mouse num logo acende a região e mostra uma linha de
  status ao vivo; clicar (ou teclar 1–9 e 0) dá zoom nela e
  abre o módulo. Dá pra girar o cérebro arrastando e
  aproximar com a roda; o painel de ajustes no topo guarda densidade das
  dobras, brilho, velocidade, tamanho e relevo no navegador. Respeita
  `prefers-reduced-motion` (vira uma imagem parada).
- **Financeiro** (`/financeiro`) — um módulo com cinco seções por abas:
  - *Dashboard* — o pulso do mês: total gasto (com variação vs. mês anterior),
    gasto por categoria, ritmo semanal.
  - *Lançamentos* — o extrato completo do mês e o formulário de novo
    lançamento, com a regra que motivou o projeto: uma compra no crédito
    conta como despesa no mês em que a fatura do cartão vence — calculado a
    partir do dia de fechamento e do dia de vencimento do cartão — nunca
    simplesmente no mês da compra.
  - *Categorias* — o detalhamento: gasto por categoria e a comparação mês a
    mês, categoria a categoria.
  - *Cartões* — cadastro dos cartões de crédito (nome, dia de fechamento e
    de vencimento), que é o que alimenta a regra de competência acima.
  - *Contas* — contas a pagar e a receber, com status derivado (atrasado/
    pendente/pago) a partir da data de vencimento — separado do extrato de
    lançamentos porque é sobre o que ainda vai acontecer, não sobre o que já
    aconteceu.
- **Senhas** (`/senhas`) — um cofre de senhas criptografado (AES-256-GCM).
  Revelar uma senha pede de novo a senha do app (`APP_PASSWORD`), mesmo
  com o login feito.
- **Notas** (`/notas`) — um app de escrita: cadernos na lateral, a lista no
  meio e a nota aberta à direita, com editor
  [Tiptap](https://tiptap.dev) (MIT). Títulos, listas, checklists, citações,
  blocos de código com cores, tabelas, links, marca-texto e imagens coladas
  ou arrastadas (reduzidas no navegador antes de salvar). Digite `/` para
  inserir blocos; atalhos de markdown (`## `, `- `, `[ ] `) também funcionam.
  Salva sozinho enquanto você escreve (⌘S força), e uma nota deixada vazia é
  apagada ao sair. "Nota do dia" abre (ou cria) a nota com a data de hoje no
  caderno Diário. O documento vai pro Postgres como `jsonb`, junto com uma
  cópia em texto puro usada na busca e no overview; notas antigas abrem como
  texto. Limite de 10 MB por nota.
- **Hábitos** (`/habitos`) — hábitos de marcar ("ler 20 minutos") ou de
  contar até uma meta ("8 copos de água"), em todos ou alguns dias da semana.
  A tela mostra o checklist de hoje, a sequência atual e a melhor de cada
  hábito, os últimos 7 dias (dá pra marcar dias passados) e um mapa do ano.
  Dias fora da programação não quebram a sequência, e o dia de hoje só conta
  contra quando acaba. "Hoje" é calculado no fuso de São Paulo pelo Next; o
  backend não tem fuso.
- **Lembretes** (`/lembretes`) — lembretes simples com data opcional,
  agrupados por atrasado/hoje/próximos.
- **Agenda** (`/agenda`) — eventos com data, horário e local, numa grade
  do mês.
- **Treino** (`/treino`) — seus treinos (nome, foco, dias da semana e
  exercícios com séries, repetições, carga e descanso). A tela abre no treino
  de hoje, mostra a semana em sete colunas e, em dia de descanso, diz qual é
  o próximo.
- **Dieta** (`/dieta`) — refeições com horário, dias da semana e alimentos
  com kcal, proteína, carboidrato e gordura. A tela mostra os macros do dia
  contra as metas diárias, a refeição de agora e a próxima, e os totais de
  cada dia da semana. "Hoje" e "agora" são calculados no fuso de São Paulo.
- **Documentos** (`/documentos`) — arquivos em pastas e subpastas. Os bytes
  ficam no disco do servidor, em `DOCUMENTS_DIR` (por padrão
  `backend/data/documents`, fora do git), e só os metadados vão pro
  Postgres. O nome enviado é sanitizado e o arquivo é gravado como
  `<uuid>__<nome>`, então nada escapa da pasta; pasta com conteúdo não pode
  ser excluída. Download passa por um route handler do Next, como o resto.
- **Quadros** (`/quadros`) — lousas pra desenhar fluxos, diagramas e
  rascunhos, com o editor do [Excalidraw](https://github.com/excalidraw/excalidraw)
  (MIT) embutido. Tudo salva sozinho pouco depois de cada mudança (e ao sair
  do quadro); a cena vai inteira pro Postgres como `jsonb`, junto com uma
  miniatura SVG que a lista e o overview mostram. As fontes do editor são
  copiadas de `node_modules` pra `public/excalidraw-assets` antes do `dev` e
  do `build` (`scripts/copy-excalidraw-assets.mjs`), então nada é buscado de
  CDN. O salvamento passa por um route handler do Next (uma cena com imagens
  passa fácil do limite de corpo das server actions); o limite é 20 MB por
  quadro.

- **Conversa** (`/chat`, o balão ao lado da engrenagem na home) — um chat
  com o cérebro. Sem agente nenhum, ele já funciona com **atalhos prontos**
  por módulo: "Quanto gastei este mês?", "Contas a pagar do mês que vem",
  "Lançar um gasto" (com formulário), "Lançar vários gastos", "Criar
  lembrete", "Treino de hoje"… As respostas vêm em cards, com ações nas
  linhas (excluir lançamento, marcar conta como paga). Texto livre como
  "gastei 45 no mercado" abre o formulário já preenchido. Com um **agente
  externo** conectado (veja "Conectando um agente" abaixo), qualquer
  pergunta vai para ele, que usa as mesmas ferramentas pelo MCP e mantém a
  conversa (histórico salvo).

### Ferramentas e MCP

Tudo que o assistente faz é uma **ferramenta** em
`backend/internal/assistant` (33 hoje: gastos, categorias, cartões, contas,
notas, lembretes, agenda, hábitos, treino, dieta, documentos, quadros e um
resumo do dia). A mesma lista serve os atalhos do chat
(`POST /api/assistant/tools/{nome}`) e o **servidor MCP** em `/mcp`
(streamable HTTP, exige `Authorization: Bearer <token>`). O cofre de senhas
fica de fora de propósito, e ferramentas que excluem exigem `confirm: true`.
Gerar e editar imagem (`generate_image`/`edit_image`) só funcionam com
`OPENAI_API_KEY` definida no ambiente do backend.

Para usar por fora, a tela de configuração do agente (Chat > Configurar
agente… > MCP) mostra o token e um trecho pronto: o agente externo (Hermes,
OpenClaw…) aponta para `/mcp` com o token. Para um agente que só fala stdio,
`go build -o estus-mcp ./cmd/estus-mcp` em `backend/` gera um relay que
repassa para o `/mcp` do app. Conectores que chamam da nuvem (ex.:
Claude.ai/ChatGPT na web) precisam de um endereço público só para `/mcp`;
por enquanto com o token na URL (`?token=`) — login OAuth para esses
conectores ainda não existe.

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

- **O navegador nunca fala com o Go diretamente** — toda leitura normal
  acontece em Server Components, toda escrita normal via Server Action, e as
  chamadas que precisam rodar no navegador (ex.: `POST
  /api/vault/[id]/reveal`) passam por Route Handlers do Next.js que só
  repassam para o Go. O Go nunca é exposto publicamente.
- **Uma senha, no ambiente, é o login.** O Next.js (`frontend/proxy.ts`)
  manda para `/entrar` toda requisição sem o cookie de sessão; o login
  compara o que foi digitado com `APP_PASSWORD`. O cookie dura 30 dias e é
  assinado com uma chave derivada da senha: trocar a senha desloga todo
  mundo. Só `/mcp` fica de fora, protegido pelo token do MCP.
- **Dinheiro é inteiro, sempre.** `amount_cents bigint` no Postgres,
  `domain.Cents int64` no Go. Nenhuma soma financeira passa por `float`.
- **A regra da fatura é uma função pura e testada:**
  `domain.CompetenceMonth` (`backend/internal/domain/billing.go`).
- **Senhas nunca ficam em texto puro em repouso.** `vault_entries` guarda
  `password_ciphertext`/`password_nonce` (AES-256-GCM); a chave mestra vem
  de `VAULT_ENCRYPTION_KEY` (variável de ambiente, nunca no banco). A rota de
  listagem nunca seleciona essas colunas — só a rota de "revelar" as
  descriptografa, e o Go só responde se o corpo trouxer a `APP_PASSWORD`
  certa.
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

Defina `APP_PASSWORD` em `backend/.env` e em `frontend/.env.local` (o mesmo
valor): sem ela ninguém entra.

Para rodar tudo containerizado: `docker compose up --build`, com
`APP_PASSWORD` no `.env` da raiz. Localmente o `docker compose` também
carrega o `docker-compose.override.yml`, que publica as portas em
`127.0.0.1` (Postgres na 5434, API na 8080, app na 3000).

## Deploy (Cubeship)

O Cubeship funciona como o Dokploy: ele sobe o `docker-compose.yml` e põe
o domínio na frente de um serviço.

1. Crie um app do tipo Docker Compose apontando para este repositório, com
   o arquivo `docker-compose.yml`.
2. Defina as variáveis no painel:
   - `APP_PASSWORD` — a senha de login. Sem ela ninguém entra.
   - `POSTGRES_PASSWORD` — senha forte do banco. Só vale na primeira subida,
     quando o volume do Postgres ainda está vazio.
   - `VAULT_ENCRYPTION_KEY` — `openssl rand -base64 32`. Sem ela o cofre de
     senhas fica desligado. Guarde uma cópia fora do servidor: sem a chave,
     as senhas do backup não abrem.
   - Opcionais: `MCP_TOKEN`, `ASSISTANT_TZ`, `AGENT_URL`, `AGENT_TOKEN`,
     `AGENT_MODEL`, `OPENAI_API_KEY`.
3. Aponte o domínio para o serviço `frontend`, porta `3000`, com HTTPS. É o
   único serviço exposto: o `/mcp` também passa por ele.

Dois volumes guardam os dados: `estus_vault_pgdata` (o banco) e
`estus_vault_data` (arquivos de Documentos e o token do MCP). As migrations
rodam sozinhas quando o backend sobe.

Para levar os dados da máquina para o servidor:

```bash
docker compose exec -T postgres pg_dump -U estus -d estus_vault --clean --if-exists > estus.sql
```

Depois, no servidor, rode `psql -U estus -d estus_vault < estus.sql` no
container do Postgres, com o backend parado. Os arquivos de Documentos
(`backend/data/documents`) vão para o volume `estus_vault_data`, em
`documents/`.

## Conectando um agente

O chat do Estus responde por um agente externo que fala o formato de chat
da OpenAI — [Hermes Agent](https://hermes-agent.nousresearch.com/docs/user-guide/features/api-server/)
ou [OpenClaw](https://docs.openclaw.ai/gateway/openai-http-api), por exemplo.
O agente usa as ferramentas do Estus pelo MCP.

1. Ligue a API do agente.
   - Hermes: defina `API_SERVER_KEY` e suba o API server (porta padrão `8642`).
   - OpenClaw: habilite o endpoint `chatCompletions` no Gateway.
2. No MCP do agente, adicione `http://127.0.0.1:<porta do Estus>/mcp` com o
   cabeçalho `Authorization: Bearer <token>`. O token e um exemplo pronto
   ficam em Chat > Configurar agente… > MCP.
3. Em Chat > Configurar agente… > Agente, preencha o endereço e o token do
   agente e clique em "Salvar e ligar".
4. Avisos: crie no agente uma tarefa agendada que chama as ferramentas de
   lembretes e agenda do Estus (ex.: toda manhã às 8h, "liste os lembretes e
   eventos de hoje e me mande") e envia pelo canal dele (WhatsApp, Telegram…).
   O Estus não manda notificações sozinho.

Também dá para configurar pelo `.env` do backend: `AGENT_URL`, `AGENT_TOKEN`
e `AGENT_MODEL`. O que a tela salva vale primeiro; cada campo deixado vazio
nela usa o `.env` (a tela mostra esse valor como "do .env: …" e não o copia
para o banco). Trocar o endereço para outro servidor sem colar um token novo
apaga o token salvo, para ele não ir para o servidor novo. A instalação nova
começa sem agente (`provider: "none"`), então mesmo configurando só pelo
`.env` é preciso um clique em "Salvar e ligar" na aba Agente para ligar o
agente.

## O que falta antes de "subir na infra"

1. **Backups do Postgres** — agora com mais dado sensível (senhas
   criptografadas inclusas) que antes; `pg_dump` agendado + guardar
   `VAULT_ENCRYPTION_KEY` em um cofre separado do backup do banco (backup do
   banco sem a chave é só ruído para quem não deveria ler; junte os dois e
   você perdeu a proteção).
2. **Gastos recorrentes automáticos** — hoje `is_recurring` é só uma flag
   manual por lançamento financeiro; não se recria sozinho todo mês.
3. **Gestão de cartões** — hoje um cartão só existe via seed/API; não tem
   tela para cadastrar um novo cartão, editar dia de fechamento/vencimento,
   ou ver a fatura por cartão quando houver mais de um.

## Testes

```bash
cd backend && go test ./...   # cobre competência de fatura, split de parcelas,
                               # criptografia do vault, status de contas/lembretes
cd frontend && npm run lint && npx tsc --noEmit
```
