# Estus Brain no Telegram — design

Data: 2026-09-15 · Status: aprovado em conversa, aguardando revisão do spec

## Objetivo

Conversar com o Estus Brain pelo Telegram (mesmo motor de IA e ferramentas do
chat web) e receber dele, sem pedir, resumo da manhã, fechamento da noite,
lembretes na hora e aviso antes de compromissos. O token do bot é cadastrado
na tela de configuração.

## Decisões

- **Onde roda:** hoje no Mac, depois num servidor. Por isso o bot usa **long
  polling** (`getUpdates`) — sem domínio, sem HTTPS, igual nos dois lugares.
  Nada no desenho depende do macOS.
- **Abordagem:** dentro do backend Go (pacote novo `internal/telegram`),
  rodando no mesmo processo da API. Descartados: fluxo no n8n (webhook exige
  URL pública; config espalhada) e binário separado (mais um processo para
  subir e configurar).
- **Relatórios e alertas sem IA:** texto montado pelo próprio app a partir dos
  dados. Sempre chegam (sem motor, sem créditos), na hora e de graça. A IA só
  entra quando a pessoa escreve para o bot.
- **Segurança:** só um chat — o do dono — pareado por código de uso único.
  Todo o resto é ignorado.

## Dados

Migration `0016_telegram`.

`telegram_settings` (linha única, `id = 1`):

| coluna | tipo | uso |
|---|---|---|
| `token_secret` | text | token cifrado (AES-256-GCM com `VAULT_ENCRYPTION_KEY`, nonce+ciphertext em base64, como as API keys do assistente); vazio = sem token |
| `bot_username` | text | `@` do bot, confirmado via `getMe` ao salvar |
| `chat_id` | bigint null | chat do dono; null = não pareado |
| `owner_name` | text | nome do Telegram do dono, para mostrar na tela |
| `pairing_code` | text | código de 6 dígitos em vigor; vazio = nenhum |
| `pairing_expires_at` | timestamptz null | validade do código (10 min) |
| `update_offset` | bigint | último `update_id` processado + 1 |
| `conversation_id` | uuid null | conversa "Telegram" do assistente em uso |
| `morning_enabled` / `morning_time` | bool / text | padrão `true` / `"07:00"` |
| `evening_enabled` / `evening_time` | bool / text | padrão `true` / `"21:00"` |
| `reminders_enabled` | bool | padrão `true` |
| `events_enabled` / `events_minutes_before` | bool / int | padrão `true` / `30` |
| `last_morning_on` / `last_evening_on` | date null | dia (no fuso do dono) do último envio |
| `updated_at` | timestamptz | |

`telegram_sent` (`kind text`, `ref text`, `sent_at timestamptz`, PK
`(kind, ref)`): alertas já enviados. `kind` = `reminder` (ref = id do
lembrete + `due_at` RFC3339) ou `event` (ref = id do evento + `starts_at`
RFC3339). Incluir o horário no `ref` faz um item remarcado avisar de novo.
Linhas com mais de 30 dias são apagadas pelo agendador uma vez por dia.

Token: sem `VAULT_ENCRYPTION_KEY` o campo da tela fica desabilitado e vale
`TELEGRAM_BOT_TOKEN` do ambiente. Token salvo tem precedência sobre o do
ambiente. Fuso: `ASSISTANT_TZ`.

## Componentes

`internal/telegram/`:

- **`client.go`** — cliente mínimo da Bot API sobre `net/http`: `GetMe`,
  `GetUpdates(offset, timeout=30s)`, `SendMessage(chatID, html, buttons)`,
  `SendChatAction(typing)`, `AnswerCallbackQuery`, `EditMessageText`. Erros
  não-2xx viram `*APIError{Code, Description}`.
- **`format.go`** — Markdown do motor → subconjunto HTML do Telegram (`<b>`,
  `<i>`, `<code>`, `<pre>`, `<a>`; listas viram `•`; resto escapado) e
  divisão em partes de até 4096 caracteres sem cortar no meio de tag.
- **`reports.go`** — funções puras `MorningReport(Snapshot)`,
  `EveningReport(Snapshot)`, `ReminderAlert(Reminder)`, `EventAlert(Event)`
  que devolvem texto. `Snapshot` é montado a partir dos services (lembretes,
  eventos, contas, transações do dia e do mês, hábitos, treino).
- **`schedule.go`** — função pura `Due(now, settings, state) []Job`: decide
  o que mandar neste minuto. Sem I/O.
- **`bot.go`** — `Bot`: guarda dependências (repo, `assistant.Chat`,
  services, fuso, chave do cofre), roda os dois loops e expõe `Reload()`
  (chamado quando a config muda) e `Status()`.
- **`store/postgres/telegram_repo.go`** — leitura/gravação de
  `telegram_settings` e `telegram_sent` (`MarkSent` com `insert … on conflict
  do nothing` devolvendo se inseriu; `UnmarkSent`).
- **`httpapi/handlers_telegram.go`** — rotas sob `/api/assistant/telegram/`
  (o proxy do Next em `app/api/assistant/[...path]` já cobre):
  - `GET settings` — config + status (sem o token; só `has_token`, `from_env`)
  - `PUT settings` — token (`""` remove), avisos e horários
  - `POST pair` — gera código; `DELETE pair` — despareia
  - `POST test` — manda uma mensagem de teste
- **Frontend** — seção "Telegram" em `components/assistant/EngineSettings.tsx`.

`main.go` constrói o `Bot` depois do `Chat` e o inicia com o contexto do
servidor; no desligamento os loops param junto.

## Fluxos

### Salvar token

`PUT settings` com token → `GetMe`. Falhou com 401 → erro de validação "o
Telegram recusou o token — confira no @BotFather", nada é salvo. Deu certo →
cifra, salva `bot_username`, zera `chat_id`/`update_offset` se o bot mudou,
`Reload()`.

### Pareamento

`POST pair` → código de 6 dígitos aleatório (`crypto/rand`), validade 10 min,
devolve `{code, expires_at, link: "https://t.me/<bot>"}`. Recebida
`/start <código>` num chat **privado**: se bate e não expirou → grava
`chat_id` e `owner_name`, apaga o código, responde "Pareado ✅ — mande /ajuda
para ver o que eu sei fazer". Enquanto não há dono, qualquer outra mensagem
num chat privado (código errado, expirado ou ausente) recebe "Código inválido
ou expirado. Gere outro na tela de configuração do Estus." Cada código aceita
**5 tentativas erradas**; na quinta ele é apagado e é preciso gerar outro
(contador em memória, zerado a cada código novo). Com dono pareado, mensagens
de qualquer outro chat são ignoradas sem resposta e só registradas no log
(`chat_id`, sem o texto). A tela consulta
`GET settings` a cada 3 s enquanto há código em vigor.

### Mensagem do dono

1. `SendChatAction(typing)` a cada 4 s enquanto o motor trabalha.
2. Comandos (sem IA): `/hoje` → resumo da manhã agora; `/noite` →
   fechamento agora; `/nova` → zera `conversation_id`; `/ajuda` → lista;
   `/start` sem código → mesma resposta de `/ajuda`.
3. Outro texto → `Chat.Send` com `conversation_id` salvo (vazio cria uma
   conversa nova, com título "Telegram · <data>"; o id é gravado de volta),
   módulo vazio e o motor escolhido. Junta os eventos `text`; no `error`
   usa a mensagem de `explain.go`. Sem motor → "Nenhum motor de IA está
   selecionado… Os comandos /hoje e /noite funcionam sem IA."
4. Resposta → `format.go` → `SendMessage` em partes. Se o Telegram recusar o
   HTML (400 "can't parse entities"), reenvia a mesma parte como texto puro.
5. Mensagens que não são texto (foto, áudio) → "Por enquanto eu só entendo
   texto."

Mensagens são processadas uma de cada vez, em ordem; `update_offset` é gravado
depois de cada update processado.

### Agendador (a cada minuto, só com dono pareado)

`Due` produz jobs:

- **Manhã:** `morning_enabled`, hora local ≥ `morning_time`,
  `last_morning_on` ≠ hoje, e hora local < 12:00. Conteúdo: compromissos de
  hoje com horário; lembretes de hoje e atrasados; contas vencendo hoje e
  atrasadas; treino de hoje; hábitos programados para hoje. Seções vazias
  somem; tudo vazio → "Dia livre: nada na agenda, lembretes ou contas."
- **Noite:** `evening_enabled`, hora local ≥ `evening_time`,
  `last_evening_on` ≠ hoje (até 23:59). Conteúdo: gasto de hoje e total do
  mês; hábitos de hoje ainda não marcados; lembretes de hoje ainda abertos;
  compromissos de amanhã.
- **Lembrete:** `reminders_enabled`, não concluído, `due_at` ≤ agora e
  `due_at` > agora − 12 h, sem registro em `telegram_sent`. Mensagem
  "🔔 <b>Título</b>" + botão inline "✅ Concluído" (`callback_data =
  "done:<id>"`).
- **Compromisso:** `events_enabled`, `starts_at − minutos` ≤ agora <
  `starts_at`, sem registro. Mensagem "📅 <b>Título</b> às 15:00 · Local".

Envio: grava a marca (data do relatório ou `MarkSent`) → envia → se o envio
falhar, desfaz a marca para tentar no minuto seguinte. Falha ao gravar a marca
= não envia.

Botão "✅ Concluído": callback só aceito do chat do dono → `SetDone(id,
true)` → `AnswerCallbackQuery("Concluído")` → edita a mensagem para
"✅ <s>Título</s>". Lembrete já apagado → "Esse lembrete não existe mais."

## Tela de configuração

Seção "Telegram" em "Motor de IA e conexões":

1. **Token** — campo mascarado, link para @BotFather com o passo a passo em
   uma linha, Salvar e remover. Salvo: "Conectado a @bot".
   Do ambiente: "Usando TELEGRAM_BOT_TOKEN do servidor".
2. **Pareamento** — "Gerar código" → `/start 482913`, contagem regressiva,
   botão "Abrir no Telegram". Pareado: "Pareado com <nome>" + "Desparear".
3. **Avisos** (só com dono pareado) — manhã (switch + hora), noite (switch +
   hora), lembretes na hora (switch), compromissos (switch + minutos antes),
   botão "Mandar teste".
4. **Status** — "Funcionando" ou o último problema.

Validação ao salvar: manhã entre `04:00` e `11:59` (a janela de recuperação
vai até o meio-dia, então uma manhã depois disso nunca sairia), noite entre
`12:00` e `23:59`, minutos antes de compromisso entre 1 e 1440. Horário fora
disso → "Horário da manhã precisa ficar entre 04:00 e 11:59." (e equivalentes).

## Erros

O `Bot` guarda o último problema; `GET settings` o devolve e a tela mostra.

| situação | comportamento | texto na tela |
|---|---|---|
| 401 no polling | para de tentar até a config mudar | "O Telegram recusou o token — gere outro no @BotFather." |
| rede / 5xx | espera 1 s, 2 s, 4 s… até 60 s | "Sem conexão com o Telegram. Tentando de novo." |
| 409 no `getUpdates` | espera 60 s e tenta de novo | "Outro Estus está usando este bot — desligue um deles." |
| 403 ao enviar | registra; alerta desmarcado | "O bot foi bloqueado no Telegram." |
| motor falha | responde com a explicação de `explain.go` | — |
| sem `VAULT_ENCRYPTION_KEY` | campo de token desabilitado | "Defina VAULT_ENCRYPTION_KEY para salvar o token, ou use TELEGRAM_BOT_TOKEN." |
| erro ao ler dados de um relatório | a seção some e o relatório vai sem ela; registra no log | — |

O token nunca aparece em log, resposta HTTP ou mensagem de erro (a URL da Bot
API contém o token — erros de rede são reescritos sem a URL).

## Testes

- **Puros:** `reports` (cada seção, vazios, dia livre), `schedule.Due` com
  relógio fixo (manhã perdida antes/depois do meio-dia, noite, lembrete com
  11 h e 13 h de atraso, evento remarcado, evento já começado, nada duas
  vezes), `format` (escape, negrito, código, links, divisão em 4096),
  pareamento (certo, errado, expirado, reuso, chat de grupo, código apagado
  na quinta tentativa errada).
- **Cliente:** `httptest` simulando a Bot API — `getUpdates`, `sendMessage`,
  callback, 401, 409, 400 de parse com reenvio em texto puro, e checagem de
  que o token não vaza na mensagem de erro.
- **Repo:** teste de integração no padrão dos existentes (`MarkSent`
  idempotente, `UnmarkSent`, salvar/ler settings).
- **Ponta a ponta (manual, com o token real):** parear, mandar mensagem,
  `/hoje`, lembrete para daqui 2 min com "Concluído", evento com aviso.

## Documentação

Seção "Telegram" no README (criar bot no @BotFather, colar token, parear) e
`TELEGRAM_BOT_TOKEN=` no `backend/.env.example`.

## Fora do escopo

Webhook, grupos, mais de um dono, fotos/áudio/documentos, adiar lembrete,
relatórios escritos por IA, mensagens em outros idiomas.
