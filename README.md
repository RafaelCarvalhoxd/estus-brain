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
  Não tem checagem extra pra revelar: a proteção é o mTLS na borda (ver
  "Deploy com mTLS") — quem chega até o app já provou quem é com o
  certificado.
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
- **Agenda** (`/agenda`) — eventos locais, com sincronização opcional (via
  OAuth) com o Google Calendar.
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
  com o cérebro. Sem IA nenhuma, ele já funciona com **atalhos prontos** por
  módulo: "Quanto gastei este mês?", "Contas a pagar do mês que vem",
  "Lançar um gasto" (com formulário), "Lançar vários gastos", "Criar
  lembrete", "Treino de hoje"… As respostas vêm em cards, com ações nas
  linhas (excluir lançamento, marcar conta como paga). Texto livre como
  "gastei 45 no mercado" abre o formulário já preenchido. Com um **motor de
  IA** conectado, qualquer pergunta vai para ele, que usa as mesmas
  ferramentas e mantém a conversa (histórico salvo, dá para trocar de motor
  no meio). Motores:
  - *Claude (sua conta)* e *GPT (sua conta ChatGPT)* — rodam o Claude Code e
    o Codex já logados neste servidor, sem API key, com shell e arquivos
    desligados: só enxergam o MCP do Estus Brain. Uso pessoal; com
    `ASSISTANT_MULTI_USER=true` ficam desligados.
  - *Claude / GPT por API key* — chaves salvas criptografadas com
    `VAULT_ENCRYPTION_KEY` (ou `ANTHROPIC_API_KEY` / `OPENAI_API_KEY`).
  - *Ollama* — modelo local.
  - *Apple Intelligence* — o modelo on-device do macOS, via a ponte Swift em
    `apple-bridge/` (`swift build -c release`), iniciada sob demanda. Como o
    modelo é pequeno, recebe só as ferramentas do módulo selecionado.

### Ferramentas e MCP

Tudo que o assistente faz é uma **ferramenta** em
`backend/internal/assistant` (33 hoje: gastos, categorias, cartões, contas,
notas, lembretes, agenda, hábitos, treino, dieta, documentos, quadros e um
resumo do dia). A mesma lista serve os atalhos do chat
(`POST /api/assistant/tools/{nome}`), os motores de IA e o **servidor MCP**
em `/mcp` (streamable HTTP, exige `Authorization: Bearer <token>`). O cofre
de senhas fica de fora de propósito, e ferramentas que excluem exigem
`confirm: true`.

Para usar por fora, a tela *Motor de IA e conexões* mostra o token e os
trechos prontos:
- **Claude Code / Codex**: apontam direto para `/mcp` com o token.
- **Claude Desktop** (só fala stdio): `go build -o estus-mcp ./cmd/estus-mcp`
  em `backend/` gera um relay que repassa para o `/mcp` do app, com suporte
  ao certificado cliente do mTLS (`ESTUS_CLIENT_CERT` / `ESTUS_CLIENT_KEY`).
- **Claude.ai / ChatGPT na web**: os conectores chamam da nuvem deles, então
  precisam de um endereço público só para `/mcp`; por enquanto com o token
  na URL (`?token=`). Login OAuth para esses conectores ainda não existe.

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
- **A app não tem login nem sessão de usuário — quem entra é quem tem o
  certificado.** Em produção o único ponto exposto na internet é o proxy
  Caddy (`deploy/Caddyfile`), que exige um certificado de cliente (mTLS)
  assinado pela sua própria CA antes de sequer repassar a requisição pro
  Next.js. Ver "Deploy com mTLS" abaixo.
- **Dinheiro é inteiro, sempre.** `amount_cents bigint` no Postgres,
  `domain.Cents int64` no Go. Nenhuma soma financeira passa por `float`.
- **A regra da fatura é uma função pura e testada:**
  `domain.CompetenceMonth` (`backend/internal/domain/billing.go`).
- **Senhas nunca ficam em texto puro em repouso.** `vault_entries` guarda
  `password_ciphertext`/`password_nonce` (AES-256-GCM); a chave mestra vem
  de `VAULT_ENCRYPTION_KEY` (variável de ambiente, nunca no banco). A rota de
  listagem nunca seleciona essas colunas — só a rota de "revelar" as
  descriptografa, e ela pode ser chamada à vontade por quem já passou pelo
  mTLS (não há checagem extra dentro do app).
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

Para rodar tudo containerizado: `docker compose up --build`. O serviço
`caddy` (mTLS) fica atrás de um profile e não sobe nesse comando — ele é só
para produção, ver "Deploy com mTLS" abaixo.

## App do Mac

`scripts/make-app.sh` cria um **Estus Brain.app** em `~/Applications` — ícone
próprio no Dock, Spotlight e Launchpad. Abrir o app sobe o que estiver
faltando (Docker, Postgres, backend, frontend) e abre uma janela do Chrome
em *app mode*, sem barra de endereço nem abas.

```bash
scripts/make-app.sh          # cria/atualiza o .app
scripts/make-app.sh --icon   # redesenha o ícone antes (fonte: scripts/icon/)
```

O `.app` é só um atalho de três linhas: a lógica mora em
`scripts/estus-brain.sh`, que também roda direto no terminal.

```bash
scripts/estus-brain.sh            # sobe o que falta e abre a janela
scripts/estus-brain.sh --rebuild  # força rebuild do frontend
scripts/estus-brain.sh --stop     # derruba backend e frontend
```

Detalhes que valem saber:

- **Portas 37887 (app) e 37888 (API)**, não as 3000/8080 do passo a passo
  acima — são portas que nenhuma outra ferramenta de dev costuma disputar, e
  ficam abaixo de 49152, onde o macOS começa a sortear portas efêmeras. Se
  você seguiu o passo a passo antes, ajuste `PORT` em `backend/.env` e
  `API_URL` em `frontend/.env.local`.
- O script **força** `FRONTEND_URL` para a porta do app, sobrescrevendo o
  `.env`.
- Roda o **build de produção** (`next start`), não o dev server — bem mais
  leve de memória. O rebuild acontece sozinho quando algo em `app/`,
  `components/`, `lib/`, `public/`, `next.config.*` ou `package.json` for
  mais novo que o último build.
- A janela usa o **perfil padrão do Chrome**.
- Esse fluxo é só local, sem mTLS — o Chrome fala direto com `localhost`. O
  mTLS entra quando você expõe o app numa VPS pública; ver "Deploy com
  mTLS".
- Logs em `~/Library/Logs/EstusBrain/`.
- O `.app` guarda o caminho absoluto do repo: se mover o projeto de pasta,
  rode `scripts/make-app.sh` de novo.

## Telegram

Converse com o Estus Brain pelo Telegram e receba dele o resumo da manhã, o
fechamento da noite, lembretes na hora (com botão "Concluído") e aviso antes
dos compromissos.

1. No Telegram, fale com o [@BotFather](https://t.me/BotFather), mande
   `/newbot` e copie o token.
2. Em Conversa → Motor de IA e conexões → Telegram, cole o token e salve. O
   token fica cifrado com `VAULT_ENCRYPTION_KEY`; sem ela, defina
   `TELEGRAM_BOT_TOKEN` no `.env` do backend.
3. Clique em "Gerar código" e mande `/start <código>` para o bot (ou use
   "Abrir no Telegram"). O código vale 10 minutos e cai depois de 5 tentativas
   erradas. A partir daí só o seu chat é atendido.
4. Ajuste os avisos e use "Mandar teste".

No Telegram: `/hoje` e `/noite` mandam os resumos na hora (sem IA), `/nova`
começa outra conversa e `/ajuda` lista os comandos. O resto vai para o motor
de IA escolhido, numa conversa "Telegram" que também aparece no chat web.

`/acoes` abre as **ações prontas** em botões — as mesmas ferramentas que a IA
usa (lançar gasto, contas, lembretes, agenda, hábitos, treino, dieta, notas,
documentos), então funcionam sem motor de IA nenhum. A primeira tela traz as
mais usadas e os módulos; uma ação que precisa de detalhes pergunta um de cada
vez (dá para responder por áudio), opções viram botões, excluir pede
confirmação, e `/cancelar` sai. Ficando 10 minutos sem resposta, a ação é
esquecida — e ela também se perde se o backend reiniciar no meio.

O bot busca as mensagens sozinho (long polling): não precisa de domínio nem
HTTPS e funciona igual no Mac e num servidor. Com a máquina dormindo nada sai;
ao acordar, o resumo da manhã ainda vai se for antes do meio-dia, o da noite
até meia-noite, e lembretes com até 12 h de atraso. Não rode o mesmo bot em
duas máquinas ao mesmo tempo — o Telegram só entrega para uma.

## Voz

Converse com o Estus falando. Tudo é processado no próprio Mac, pela ponte
`apple-bridge/`: a transcrição usa o reconhecimento de fala on-device da Apple
(pt-BR) e a resposta é lida com uma voz do sistema. Nenhum áudio sai da máquina
e funciona com qualquer motor de IA (ou sem nenhum, só com os atalhos).

- **No chat web:** clique no 🎙️ ao lado da caixa de mensagem, fale e clique de
  novo (ou só pare de falar por 2 segundos; o limite é 2 minutos). A frase vira
  sua mensagem e a resposta aparece em texto e é lida em voz alta — "■ Parar voz"
  interrompe. Perguntas digitadas continuam só em texto.
- **No Telegram:** mande um áudio (até 3 minutos). O bot responde "🎙️ Entendi:
  …", faz o que foi pedido e responde em texto.

Requisitos: macOS 26+ e a ponte compilada (`cd apple-bridge && swift build -c release`).
Na primeira transcrição o macOS baixa o modelo de fala pt-BR. A voz padrão é
`Luciana`; para outra (ex.: uma "Premium", baixada em Ajustes do Sistema >
Acessibilidade > Conteúdo Falado), defina `ASSISTANT_VOICE` no `.env` do backend
com o nome exatamente como `say -v '?'` imprime — incluindo o parêntese quando
houver, como em `Eddy (Portuguese (Brazil))`.
Num servidor sem macOS a voz fica indisponível e o resto do app funciona normalmente.

## Deploy com mTLS

Em produção (VPS com IP público — ex.: Contabo) o único serviço exposto na
internet é o `caddy` do `docker-compose.yml`: ele termina o HTTPS público de
verdade (Let's Encrypt, via `DOMAIN`) e **exige um certificado de cliente
válido antes de repassar qualquer requisição** para o frontend. `backend` e
`frontend` publicam suas portas só em `127.0.0.1` — inalcançáveis de fora da
máquina mesmo que o firewall da VPS esteja aberto.

**Não existe login nem sessão de usuário no app.** O certificado de cliente
é a única credencial que existe: quem tem, entra e usa tudo (inclusive
revelar qualquer senha do módulo Senhas, sem checagem extra); quem não tem,
nem chega a completar a conexão HTTPS. Como funciona por dentro (CA própria,
o que o Caddy verifica, como revogar um certificado vazado, troubleshooting)
está em **[`deploy/mtls/README.md`](deploy/mtls/README.md)** — aqui vai só o
passo a passo para colocar no ar:

1. Aponte o DNS do seu domínio (ex.: `vault.seudominio.com`) para o IP da
   VPS. Abra as portas 80 e 443 no firewall (Caddy precisa da 80 para emitir
   o certificado via ACME).
2. Na raiz do projeto, crie um `.env` com `DOMAIN=vault.seudominio.com` (e
   `VAULT_ENCRYPTION_KEY` etc., se for usar esses módulos).
3. Gere a CA e o certificado do seu Mac: `deploy/mtls/issue-client-cert.sh
   mac`. Cria `deploy/mtls/ca/` (a CA — nunca vai pro git) e
   `deploy/mtls/clients/mac/client.p12`.
4. Importe o `client.p12` no Keychain do Mac (duplo clique, ou Keychain
   Access → File → Import Items) com a senha definida no passo 3. A partir
   daí o Safari/Chrome oferecem esse certificado sozinhos ao abrir o
   domínio do vault.
5. Suba tudo na VPS: `docker compose --profile mtls up -d --build`. O Caddy
   só passa a servir o site depois de emitir o certificado Let's Encrypt
   (leva alguns segundos na primeira vez).
6. Outro dispositivo (iPhone etc.): `deploy/mtls/issue-client-cert.sh
   iphone` — reaproveita a CA existente, emite mais um certificado.

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
