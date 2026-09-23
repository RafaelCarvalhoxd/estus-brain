# Agente externo Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** O chat do Estus passa a responder por um agente externo (Hermes, OpenClaw ou outro no formato OpenAI), e Telegram, ponte Apple e os motores de IA próprios saem, junto com o código que ficar morto.

**Architecture:** Um único `Provider` (`agent`) manda a conversa para `POST {url}/v1/chat/completions` com streaming SSE e repassa o texto ao chat como hoje. O agente usa as ferramentas do Estus pelo MCP (`/mcp`), que não muda. A voz passa para as APIs do navegador (`SpeechRecognition`, `speechSynthesis`).

**Tech Stack:** Go 1.x (`net/http`, `httptest`), Postgres (migrações SQL numeradas, aplicadas no boot), Next.js/React (TypeScript).

**Spec:** `docs/superpowers/specs/2026-09-23-agente-externo-design.md`

## Global Constraints

- Branch: `feat/agente-externo`. Commits sem linha de coautoria ou assinatura de IA.
- O banco local tem dados reais: nunca truncar nem reexecutar seed. Migrações só por arquivo novo (`0029_…`, `0030_…`), aplicadas no boot pelo `db.go`.
- Textos de UI e mensagens de erro em português do Brasil.
- Formato de commit do repo: `tipo(área): descrição em português`.
- Comandos de verificação: backend `cd backend && go build ./... && go vet ./... && go test ./...`; frontend `cd frontend && npx tsc --noEmit && npm run lint`.
- Testes de integração do Postgres (`*_integration_test.go`) exigem banco; rode só os unitários se não houver `DATABASE_URL` de teste.

---

### Task 1: Remover o Telegram

**Files:**
- Delete: `backend/internal/telegram/` (pasta inteira)
- Delete: `backend/internal/httpapi/handlers_telegram.go`
- Delete: `backend/internal/store/postgres/telegram_repo.go`, `backend/internal/store/postgres/telegram_repo_test.go`
- Delete: `backend/internal/assistant/sendtime.go`, `backend/internal/assistant/sendtime_test.go`
- Delete: `frontend/components/assistant/TelegramSettings.tsx`
- Create: `backend/migrations/0029_drop_telegram.up.sql`, `backend/migrations/0029_drop_telegram.down.sql`
- Modify: `backend/cmd/api/main.go` (bloco `telegram.New` até o `select` do `botDone`, import, campo `Telegram:` em `Modules`)
- Modify: `backend/internal/httpapi/router.go` (campo `Telegram` de `Modules` e o bloco `if tg := m.Telegram`)
- Modify: `backend/internal/assistant/chat.go` (`SendRequest.SentAt`, parâmetro `sentAt` de `systemPrompt`)
- Modify: `backend/internal/config/config.go` (`TelegramBotToken`)
- Modify: `backend/.env.example` (bloco `TELEGRAM_BOT_TOKEN`)
- Modify: `frontend/components/assistant/EngineSettings.tsx` (aba Telegram e import)
- Modify: `frontend/components/assistant/types.ts` (`TelegramView`)
- Modify: `frontend/app/chat/chat.css` (bloco `/* Telegram tab */` até a próxima seção)
- Modify: `README.md` (seção do Telegram, hoje por volta das linhas 241-273)

**Interfaces:**
- Produces: `func (c *Chat) systemPrompt(module string) string` (sem `sentAt`).

- [ ] **Step 1: Apagar os arquivos**

```bash
git rm -rq backend/internal/telegram backend/internal/httpapi/handlers_telegram.go \
  backend/internal/store/postgres/telegram_repo.go backend/internal/store/postgres/telegram_repo_test.go \
  backend/internal/assistant/sendtime.go backend/internal/assistant/sendtime_test.go \
  frontend/components/assistant/TelegramSettings.tsx
```

- [ ] **Step 2: Criar a migração**

`backend/migrations/0029_drop_telegram.up.sql`:

```sql
-- Telegram left the app: an external agent (Hermes, OpenClaw) now reaches
-- the owner on its own channels. Its pairing, schedule and sent-log go too.
drop table if exists telegram_sent;
drop table if exists telegram_settings;
```

`backend/migrations/0029_drop_telegram.down.sql`: copie o corpo de `0016_telegram.up.sql` (sem as colunas que 0017 criou e 0022 removeu), precedido de:

```sql
-- Recreates the empty Telegram schema; the data dropped by the up is gone.
```

- [ ] **Step 3: Tirar o Telegram do `main.go` e do router**

Em `backend/cmd/api/main.go` apague: o import `.../internal/telegram`; o bloco que começa em `// Telegram: the owner's chat with the assistant` até o fechamento do `go func() { ... telegramBot.Run(ctx) }()`; a linha `Telegram: httpapi.NewTelegramHandlers(telegramBot),`; e no fim de `run` o bloco:

```go
	// Let the bot finish its last writes before the deferred closes take the
	// database and the AI engines away from it.
	cancel()
	select {
	case <-botDone:
		slog.Info("telegram bot stopped")
	case <-time.After(10 * time.Second):
		slog.Warn("telegram bot did not stop in time")
	}
```

Mantenha `return serveErr`. Se `cancel` ficar sem uso, mantenha o `defer cancel()` que já existe onde o contexto é criado.

Em `backend/internal/httpapi/router.go` apague o campo `Telegram *TelegramHandlers` de `Modules` e o bloco `if tg := m.Telegram; tg != nil { ... }`.

- [ ] **Step 4: Tirar `SentAt` do chat**

Em `backend/internal/assistant/chat.go`:
- apague o campo `SentAt time.Time \`json:"-"\`` e o comentário dele em `SendRequest`;
- troque `System:  c.systemPrompt(req.Module, req.SentAt),` por `System:  c.systemPrompt(req.Module),`;
- troque a assinatura para `func (c *Chat) systemPrompt(module string) string` e apague a linha `b.WriteString(sendTimeLine(sentAt, now))`.

Em `backend/internal/config/config.go` apague `TelegramBotToken` (campo, comentário e a linha em `Load`). Em `backend/.env.example` apague o bloco do `TELEGRAM_BOT_TOKEN`.

- [ ] **Step 5: Compilar e testar o backend**

Run: `cd backend && go build ./... && go vet ./... && go test ./internal/assistant/... ./internal/httpapi/... ./internal/domain/...`
Expected: sem erros. Se algum teste citar `sendTimeLine` ou Telegram, apague esse teste.

- [ ] **Step 6: Tirar o Telegram do frontend**

Em `EngineSettings.tsx`: apague `import { TelegramSettings } from "./TelegramSettings";`, o botão da aba `telegram`, a linha `{tab === "telegram" && <TelegramSettings />}` e `"telegram"` do tipo do `useState` da aba. Em `types.ts` apague `TelegramView` e o comentário acima dela. Em `frontend/app/chat/chat.css` apague da linha `/* Telegram tab */` até antes do próximo comentário de seção. No `README.md` apague a seção do Telegram.

- [ ] **Step 7: Verificar o frontend**

Run: `cd frontend && npx tsc --noEmit && npm run lint`
Expected: sem erros. `grep -rni telegram frontend/app frontend/components frontend/lib backend/internal backend/cmd` só pode achar comentários de migrações antigas.

- [ ] **Step 8: Commit**

```bash
git add -A backend frontend README.md
git commit -m "refactor(assistant): remove a integração com Telegram"
```

---

### Task 2: Motor "Agente externo"

**Files:**
- Create: `backend/internal/assistant/httpjson.go`
- Create: `backend/internal/assistant/provider_agent.go`
- Test: `backend/internal/assistant/provider_agent_test.go`
- Modify: `backend/internal/assistant/providers_api.go` (tirar de lá o que foi para `httpjson.go`)

**Interfaces:**
- Produces:
  - `type agentConfig struct { URL, Token, Model string }`
  - `type agentProvider struct { config func(ctx context.Context) (agentConfig, error); client *http.Client }`
  - `func (p *agentProvider) ID() string` → `"agent"`
  - `func (p *agentProvider) Status(ctx context.Context) ProviderStatus`
  - `func (p *agentProvider) Chat(ctx context.Context, req ChatRequest, emit func(Event)) (ChatOutcome, error)`
  - `var errAgentNotConfigured error`
  - `type agentUnreachableError struct { URL string; Err error }`
  - Em `httpjson.go`: `httpClient`, `postJSON`, `apiError`, `hostOf`, `userContentOpenAI` (movidos, mesmas assinaturas).

- [ ] **Step 1: Mover os helpers HTTP**

Crie `backend/internal/assistant/httpjson.go` com o `package assistant`, os imports necessários e, recortados de `providers_api.go` sem mudar nada: `var httpClient`, `func postJSON`, `type apiError` e seu `Error()`, `func hostOf`, `func userContentOpenAI`. Comentário do topo do arquivo:

```go
// HTTP helpers shared by the agent engine and the image tools.
```

Run: `cd backend && go build ./...`
Expected: compila (as funções só mudaram de arquivo).

- [ ] **Step 2: Escrever os testes que falham**

`backend/internal/assistant/provider_agent_test.go`:

```go
package assistant

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeAgent answers like an OpenAI-compatible agent: /v1/models lists one
// model and /v1/chat/completions streams chunks, with a Hermes-style custom
// event in the middle that the provider must skip.
func fakeAgent(t *testing.T, token string, got *map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+token {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"error":{"message":"bad token"}}`)
			return
		}
		switch r.URL.Path {
		case "/v1/models":
			fmt.Fprint(w, `{"data":[{"id":"hermes-agent"},{"id":"outro"}]}`)
		case "/v1/chat/completions":
			body, _ := io.ReadAll(r.Body)
			if got != nil {
				_ = json.Unmarshal(body, got)
			}
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"role\":\"assistant\"}}]}\n\n")
			fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Olá, \"}}]}\n\n")
			fmt.Fprint(w, "event: hermes.tool.progress\ndata: {\"tool\":\"finance_summary\"}\n\n")
			fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"tudo certo.\"}}]}\n\n")
			fmt.Fprint(w, "data: [DONE]\n\n")
		default:
			http.NotFound(w, r)
		}
	}))
}

func agentFor(cfg agentConfig) *agentProvider {
	return &agentProvider{
		config: func(context.Context) (agentConfig, error) { return cfg, nil },
		client: http.DefaultClient,
	}
}

func TestAgentChatStreamsText(t *testing.T) {
	var got map[string]any
	srv := fakeAgent(t, "tok", &got)
	defer srv.Close()

	var events []Event
	out, err := agentFor(agentConfig{URL: srv.URL, Token: "tok", Model: "hermes-agent"}).Chat(context.Background(), ChatRequest{
		System:  "sys",
		History: []Message{{Role: "user", Content: "oi"}, {Role: "assistant", Content: "oi!"}},
		Message: "como estão as contas?",
	}, func(e Event) { events = append(events, e) })
	if err != nil {
		t.Fatal(err)
	}
	if out.Text != "Olá, tudo certo." {
		t.Fatalf("text = %q", out.Text)
	}
	if len(events) != 2 || events[0].Type != "text" || events[1].Text != "tudo certo." {
		t.Fatalf("events = %+v", events)
	}
	if got["model"] != "hermes-agent" || got["stream"] != true {
		t.Fatalf("body = %v", got)
	}
	msgs := got["messages"].([]any)
	if len(msgs) != 4 {
		t.Fatalf("messages = %v, want system + 2 history + new", msgs)
	}
	first := msgs[0].(map[string]any)
	last := msgs[3].(map[string]any)
	if first["role"] != "system" || last["content"] != "como estão as contas?" {
		t.Fatalf("messages = %v", msgs)
	}
}

func TestAgentChatSendsImageAsImageURL(t *testing.T) {
	var got map[string]any
	srv := fakeAgent(t, "tok", &got)
	defer srv.Close()

	_, err := agentFor(agentConfig{URL: srv.URL, Token: "tok", Model: "m"}).Chat(context.Background(), ChatRequest{
		Message:    "o que é isso?",
		Attachment: &Attachment{ImageBase64: "AAAA", ImageMediaType: "image/png"},
	}, func(Event) {})
	if err != nil {
		t.Fatal(err)
	}
	msgs := got["messages"].([]any)
	parts := msgs[len(msgs)-1].(map[string]any)["content"].([]any)
	img := parts[1].(map[string]any)["image_url"].(map[string]any)["url"]
	if img != "data:image/png;base64,AAAA" {
		t.Fatalf("image url = %v", img)
	}
}

func TestAgentChatEmptyModelUsesFirstListed(t *testing.T) {
	var got map[string]any
	srv := fakeAgent(t, "tok", &got)
	defer srv.Close()

	if _, err := agentFor(agentConfig{URL: srv.URL, Token: "tok"}).Chat(context.Background(), ChatRequest{Message: "oi"}, func(Event) {}); err != nil {
		t.Fatal(err)
	}
	if got["model"] != "hermes-agent" {
		t.Fatalf("model = %v, want the first from /v1/models", got["model"])
	}
}

func TestAgentChatErrors(t *testing.T) {
	srv := fakeAgent(t, "tok", nil)
	defer srv.Close()
	closed := httptest.NewServer(http.NotFoundHandler())
	closedURL := closed.URL
	closed.Close()

	cases := []struct {
		name  string
		cfg   agentConfig
		check func(error) bool
	}{
		{"not configured", agentConfig{}, func(err error) bool { return errors.Is(err, errAgentNotConfigured) }},
		{"bad token", agentConfig{URL: srv.URL, Token: "errado", Model: "m"}, func(err error) bool {
			var api *apiError
			return errors.As(err, &api) && api.Status == http.StatusUnauthorized
		}},
		{"unreachable", agentConfig{URL: closedURL, Model: "m"}, func(err error) bool {
			var down *agentUnreachableError
			return errors.As(err, &down) && down.URL == closedURL
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := agentFor(tc.cfg).Chat(context.Background(), ChatRequest{Message: "oi"}, func(Event) {})
			if err == nil || !tc.check(err) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestAgentStatus(t *testing.T) {
	srv := fakeAgent(t, "tok", nil)
	defer srv.Close()

	ok := agentFor(agentConfig{URL: srv.URL, Token: "tok"}).Status(context.Background())
	if !ok.Available || strings.Join(ok.Models, ",") != "hermes-agent,outro" || !strings.Contains(ok.Detail, srv.URL) {
		t.Fatalf("status = %+v", ok)
	}
	bad := agentFor(agentConfig{URL: srv.URL, Token: "errado"}).Status(context.Background())
	if bad.Available || !strings.Contains(bad.Detail, "token") {
		t.Fatalf("status = %+v", bad)
	}
	none := agentFor(agentConfig{}).Status(context.Background())
	if none.Available || !strings.Contains(none.Detail, "endereço") {
		t.Fatalf("status = %+v", none)
	}
}
```

- [ ] **Step 3: Rodar e ver falhar**

Run: `cd backend && go test ./internal/assistant/ -run 'TestAgent' -v`
Expected: FAIL de compilação: `undefined: agentProvider`.

- [ ] **Step 4: Implementar**

`backend/internal/assistant/provider_agent.go`:

```go
package assistant

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// The one engine: an external agent (Hermes Agent, OpenClaw…) reached over
// the OpenAI chat format. It runs its own model and reaches Estus's tools
// through /mcp, so this side only relays the conversation and its answer.

const agentID = "agent"

var errAgentNotConfigured = errors.New("agente externo sem endereço configurado")

// agentConfig is where the agent lives. An empty Model means "the first one
// the agent lists", since each agent names its models its own way.
type agentConfig struct {
	URL, Token, Model string
}

// agentUnreachableError is a failure to connect at all, kept apart from an
// HTTP error so the chat can say "is it running?" with the address.
type agentUnreachableError struct {
	URL string
	Err error
}

func (e *agentUnreachableError) Error() string { return fmt.Sprintf("agente em %s: %v", e.URL, e.Err) }
func (e *agentUnreachableError) Unwrap() error { return e.Err }

type agentProvider struct {
	config func(ctx context.Context) (agentConfig, error)
	client *http.Client
}

func (p *agentProvider) ID() string { return agentID }

func (p *agentProvider) Status(ctx context.Context) ProviderStatus {
	st := ProviderStatus{ID: agentID, Name: "Agente externo", Kind: "local", Capabilities: Capabilities{SupportsVision: true}}
	cfg, err := p.config(ctx)
	if err != nil {
		st.Detail = err.Error()
		return st
	}
	st.Model = cfg.Model
	if cfg.URL == "" {
		st.Detail = "Configure o endereço do agente"
		return st
	}
	models, err := p.models(ctx, cfg)
	if err != nil {
		st.Detail = agentStatusDetail(cfg.URL, err)
		return st
	}
	st.Available, st.Models = true, models
	st.Detail = "Conectado a " + cfg.URL
	return st
}

func agentStatusDetail(url string, err error) string {
	var api *apiError
	switch {
	case errors.As(err, &api) && (api.Status == http.StatusUnauthorized || api.Status == http.StatusForbidden):
		return "O agente recusou o token"
	case errors.As(err, &api):
		return fmt.Sprintf("O agente respondeu %d", api.Status)
	default:
		return "Agente não respondeu em " + url
	}
}

func (p *agentProvider) do(ctx context.Context, cfg agentConfig, method, path string, body []byte) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(cfg.URL, "/")+path, reader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "text/event-stream")
	}
	if cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.Token)
	}
	res, err := p.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, err
		}
		return nil, &agentUnreachableError{URL: cfg.URL, Err: err}
	}
	if res.StatusCode >= 300 {
		defer res.Body.Close()
		raw, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return nil, &apiError{Host: hostOf(cfg.URL), Status: res.StatusCode, Message: truncate(string(raw), 300)}
	}
	return res, nil
}

func (p *agentProvider) models(ctx context.Context, cfg agentConfig) ([]string, error) {
	res, err := p.do(ctx, cfg, http.MethodGet, "/v1/models", nil)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	var list struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&list); err != nil {
		return nil, fmt.Errorf("lista de modelos do agente: %w", err)
	}
	out := make([]string, 0, len(list.Data))
	for _, m := range list.Data {
		out = append(out, m.ID)
	}
	return out, nil
}

func (p *agentProvider) Chat(ctx context.Context, req ChatRequest, emit func(Event)) (ChatOutcome, error) {
	cfg, err := p.config(ctx)
	if err != nil {
		return ChatOutcome{}, err
	}
	if cfg.URL == "" {
		return ChatOutcome{}, errAgentNotConfigured
	}
	if cfg.Model == "" {
		models, err := p.models(ctx, cfg)
		if err != nil {
			return ChatOutcome{}, err
		}
		if len(models) == 0 {
			return ChatOutcome{}, fmt.Errorf("o agente não lista nenhum modelo; informe um nos ajustes")
		}
		cfg.Model = models[0]
	}

	messages := make([]map[string]any, 0, len(req.History)+2)
	if req.System != "" {
		messages = append(messages, map[string]any{"role": "system", "content": req.System})
	}
	for _, m := range req.History {
		messages = append(messages, map[string]any{"role": m.Role, "content": m.Content})
	}
	messages = append(messages, map[string]any{"role": "user", "content": userContentOpenAI(req)})
	body, err := json.Marshal(map[string]any{"model": cfg.Model, "stream": true, "messages": messages})
	if err != nil {
		return ChatOutcome{}, err
	}

	res, err := p.do(ctx, cfg, http.MethodPost, "/v1/chat/completions", body)
	if err != nil {
		return ChatOutcome{}, err
	}
	defer res.Body.Close()

	var text strings.Builder
	scanner := bufio.NewScanner(res.Body)
	scanner.Buffer(make([]byte, 64<<10), 4<<20)
	for scanner.Scan() {
		// Only "data:" lines carry OpenAI chunks; "event:" lines, keep-alive
		// comments and an agent's own events (hermes.tool.progress) are skipped.
		data, ok := strings.CutPrefix(scanner.Text(), "data:")
		if !ok {
			continue
		}
		data = strings.TrimSpace(data)
		if data == "[DONE]" {
			break
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if json.Unmarshal([]byte(data), &chunk) != nil || len(chunk.Choices) == 0 || chunk.Choices[0].Delta.Content == "" {
			continue
		}
		piece := chunk.Choices[0].Delta.Content
		text.WriteString(piece)
		emit(Event{Type: "text", Text: piece})
	}
	return ChatOutcome{Text: text.String()}, scanner.Err()
}
```

- [ ] **Step 5: Rodar e ver passar**

Run: `cd backend && go test ./internal/assistant/ -run 'TestAgent' -v`
Expected: PASS nos 5 testes.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/assistant/httpjson.go backend/internal/assistant/provider_agent.go backend/internal/assistant/provider_agent_test.go backend/internal/assistant/providers_api.go
git commit -m "feat(assistant): motor que conversa com um agente externo no formato OpenAI"
```

---

### Task 3: Chat só com o agente, sem ponte Apple nem motores próprios

**Files:**
- Delete: `backend/internal/assistant/providers_api.go`, `providers_cli.go`, `provider_apple.go`, `bridge.go`, `voice.go`, `voice_test.go`, `routing.go`, `routing_test.go`
- Delete: `backend/internal/httpapi/handlers_assistant_test.go` (só testa `writeVoiceError`)
- Delete: `apple-bridge/` (pasta inteira)
- Create: `backend/migrations/0030_assistant_agent.up.sql`, `backend/migrations/0030_assistant_agent.down.sql`
- Modify: `backend/internal/assistant/chat.go`
- Modify: `backend/internal/assistant/explain.go`, `backend/internal/assistant/explain_test.go`
- Modify: `backend/internal/assistant/attachments.go`
- Modify: `backend/internal/assistant/tools_images.go`, `backend/internal/assistant/registry.go`
- Modify: `backend/internal/store/postgres/assistant_repo.go`
- Modify: `backend/internal/httpapi/handlers_assistant.go`, `backend/internal/httpapi/router.go`
- Modify: `backend/internal/config/config.go`, `backend/cmd/api/main.go`, `backend/.env.example`

**Interfaces:**
- Consumes: `agentProvider`, `agentConfig`, `errAgentNotConfigured`, `agentUnreachableError` (Task 2).
- Produces (usados pela Task 4 via JSON):
  - `GET /api/assistant/settings` →
    `{"provider":"agent"|"none","providers":[ProviderStatus],"agent":{"url":"","model":"","has_token":false},"mcp":{"url":"","token":""},"can_store_keys":true}`
  - `PUT /api/assistant/settings` aceita `{"provider"?, "agent_url"?, "agent_model"?, "agent_token"?}`; `agent_token: ""` apaga o token salvo.
  - `func NewChat(tools *Registry, repo *postgres.AssistantRepo, cfg ChatConfig) *Chat`
  - `postgres.AssistantSettings{Provider, AgentURL, AgentModel string; Secrets map[string]string; UpdatedAt time.Time}`
  - `func (r *AssistantRepo) TouchConversation(ctx context.Context, id, provider string) error`

- [ ] **Step 1: Migração**

`backend/migrations/0030_assistant_agent.up.sql`:

```sql
-- One engine left: an external agent. Its address and model live here and
-- its token in secrets (encrypted, like the API keys it replaces). The old
-- engines' keys, models and Ollama address go, and so does session_id,
-- which only the command-line engines resumed by.
alter table assistant_settings
    add column agent_url   text not null default '',
    add column agent_model text not null default '';
update assistant_settings
    set provider = case when provider = 'none' then 'none' else 'agent' end,
        secrets  = '{}'::jsonb;
alter table assistant_settings drop column models, drop column ollama_url;
alter table assistant_conversations drop column session_id;
```

`backend/migrations/0030_assistant_agent.down.sql`:

```sql
-- Restores the columns empty; the dropped keys and sessions are gone.
alter table assistant_conversations add column session_id text not null default '';
alter table assistant_settings
    add column models     jsonb not null default '{}'::jsonb,
    add column ollama_url text  not null default 'http://127.0.0.1:11434';
alter table assistant_settings drop column agent_url, drop column agent_model;
```

- [ ] **Step 2: Repositório**

Em `backend/internal/store/postgres/assistant_repo.go`:

```go
type AssistantSettings struct {
	Provider   string
	AgentURL   string
	AgentModel string
	Secrets    map[string]string // name → base64(nonce|ciphertext)
	UpdatedAt  time.Time
}
```

`Settings`: `select provider, agent_url, agent_model, secrets, updated_at from assistant_settings where id = 1`, escaneando nessa ordem; tire o `Models`.
`SaveSettings`: `update assistant_settings set provider = $1, agent_url = $2, agent_model = $3, secrets = $4::jsonb, updated_at = now() where id = 1`.
Tire `SessionID` de `Conversation`, `session_id` de `conversationColumns` e de `scanConversation`, e troque `TouchConversation` para:

```go
func (r *AssistantRepo) TouchConversation(ctx context.Context, id, provider string) error {
	_, err := r.db.Pool.Exec(ctx, `update assistant_conversations set provider = $2, updated_at = now() where id = $1`, id, provider)
	if err != nil {
		return fmt.Errorf("touch conversation: %w", err)
	}
	return nil
}
```

(Ajuste o SQL existente de `TouchConversation` mantendo o mesmo estilo; o essencial é não gravar mais `session_id`.)

- [ ] **Step 3: Apagar motores, ponte e voz**

```bash
git rm -q backend/internal/assistant/providers_cli.go backend/internal/assistant/provider_apple.go \
  backend/internal/assistant/bridge.go backend/internal/assistant/voice.go backend/internal/assistant/voice_test.go \
  backend/internal/assistant/routing.go backend/internal/assistant/routing_test.go \
  backend/internal/httpapi/handlers_assistant_test.go
git rm -rq apple-bridge
```

Depois apague `backend/internal/assistant/providers_api.go` (o que era compartilhado já está em `httpjson.go` desde a Task 2): `git rm -q backend/internal/assistant/providers_api.go`.

- [ ] **Step 4: Reescrever a configuração e os ajustes em `chat.go`**

Substitua `ChatConfig`, `Chat`, `NewChat`, `Close`, `Voice`, `defaultModels`, `model`, `SettingsView`, `Settings`, `SettingsUpdate`, `UpdateSettings`, `apiKey` e `apiKeyFrom` por:

```go
type ChatConfig struct {
	MCPURL   string
	MCPToken string
	// VaultKey encrypts the stored agent token; without it the token comes
	// only from AGENT_TOKEN.
	VaultKey *[32]byte
	// Env is the agent config from AGENT_URL / AGENT_TOKEN / AGENT_MODEL,
	// used for whatever the settings screen left empty.
	Env agentConfig
	// Documents resolves a chat attachment (see SendRequest.AttachmentID) —
	// the same storage generate_image and documents_write already use.
	Documents *service.DocumentService
}

type Chat struct {
	tools *Registry
	repo  *postgres.AssistantRepo
	cfg   ChatConfig
	agent *agentProvider
	mu    sync.Mutex
}

func NewChat(tools *Registry, repo *postgres.AssistantRepo, cfg ChatConfig) *Chat {
	c := &Chat{tools: tools, repo: repo, cfg: cfg}
	c.agent = &agentProvider{config: c.agentConfig, client: httpClient}
	return c
}

// NewAgentEnv builds ChatConfig.Env from the process environment's values.
func NewAgentEnv(url, token, model string) agentConfig {
	return agentConfig{URL: strings.TrimRight(strings.TrimSpace(url), "/"), Token: strings.TrimSpace(token), Model: strings.TrimSpace(model)}
}

const agentTokenSecret = "agent"

// agentConfig is the saved agent settings, each empty field filled from env.
func (c *Chat) agentConfig(ctx context.Context) (agentConfig, error) {
	s, err := c.repo.Settings(ctx)
	if err != nil {
		return agentConfig{}, err
	}
	cfg := agentConfig{URL: s.AgentURL, Token: c.secret(s, agentTokenSecret), Model: s.AgentModel}
	if cfg.URL == "" {
		cfg.URL = c.cfg.Env.URL
	}
	if cfg.Token == "" {
		cfg.Token = c.cfg.Env.Token
	}
	if cfg.Model == "" {
		cfg.Model = c.cfg.Env.Model
	}
	return cfg, nil
}

func (c *Chat) secret(s postgres.AssistantSettings, name string) string {
	blob, ok := s.Secrets[name]
	if !ok || c.cfg.VaultKey == nil {
		return ""
	}
	raw, err := base64.StdEncoding.DecodeString(blob)
	if err != nil || len(raw) <= 12 {
		return ""
	}
	value, err := domain.DecryptPassword(*c.cfg.VaultKey, raw[12:], raw[:12])
	if err != nil {
		return ""
	}
	return value
}

// ---- settings

type SettingsView struct {
	Provider  string           `json:"provider"` // agent | none
	Providers []ProviderStatus `json:"providers"`
	Agent     struct {
		URL      string `json:"url"`
		Model    string `json:"model"`
		HasToken bool   `json:"has_token"`
	} `json:"agent"`
	MCP struct {
		URL   string `json:"url"`
		Token string `json:"token"`
	} `json:"mcp"`
	CanStoreKeys bool `json:"can_store_keys"`
}

func (c *Chat) Settings(ctx context.Context) (SettingsView, error) {
	s, err := c.repo.Settings(ctx)
	if err != nil {
		return SettingsView{}, err
	}
	view := SettingsView{Provider: s.Provider, CanStoreKeys: c.cfg.VaultKey != nil}
	view.MCP.URL, view.MCP.Token = c.cfg.MCPURL, c.cfg.MCPToken
	cfg, err := c.agentConfig(ctx)
	if err != nil {
		return SettingsView{}, err
	}
	view.Agent.URL, view.Agent.Model, view.Agent.HasToken = cfg.URL, cfg.Model, cfg.Token != ""
	sctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	view.Providers = []ProviderStatus{c.agent.Status(sctx)}
	return view, nil
}

type SettingsUpdate struct {
	Provider   *string `json:"provider"`
	AgentURL   *string `json:"agent_url"`
	AgentModel *string `json:"agent_model"`
	AgentToken *string `json:"agent_token"` // "" removes it
}

func (c *Chat) UpdateSettings(ctx context.Context, u SettingsUpdate) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	s, err := c.repo.Settings(ctx)
	if err != nil {
		return err
	}
	if u.Provider != nil {
		if *u.Provider != "none" && *u.Provider != agentID {
			return fmt.Errorf("%w: unknown provider %q", domain.ErrValidation, *u.Provider)
		}
		s.Provider = *u.Provider
	}
	if u.AgentURL != nil {
		s.AgentURL = strings.TrimRight(strings.TrimSpace(*u.AgentURL), "/")
	}
	if u.AgentModel != nil {
		s.AgentModel = strings.TrimSpace(*u.AgentModel)
	}
	if u.AgentToken != nil {
		token := strings.TrimSpace(*u.AgentToken)
		if token == "" {
			delete(s.Secrets, agentTokenSecret)
		} else {
			if c.cfg.VaultKey == nil {
				return fmt.Errorf("%w: defina VAULT_ENCRYPTION_KEY para salvar o token, ou use AGENT_TOKEN", domain.ErrValidation)
			}
			ct, nonce, err := domain.EncryptPassword(*c.cfg.VaultKey, token)
			if err != nil {
				return err
			}
			s.Secrets[agentTokenSecret] = base64.StdEncoding.EncodeToString(append(nonce, ct...))
		}
	}
	return c.repo.SaveSettings(ctx, s)
}
```

- [ ] **Step 5: Simplificar `Send` e o resto de `chat.go`**

Em `Send`:
- troque a escolha do motor (de `providerID := req.Provider` até o `if c.cfg.MultiUser ...`) por:

```go
	if settings.Provider != agentID {
		return ErrNoProvider
	}
	providerID := agentID
```

- chame `resolveAttachment(ctx, c.cfg.Documents, req.AttachmentID)` (sem `providerID`);
- `Model: ""` sai de `chatReq` (o agente resolve o modelo); apague o bloco `if conv.Provider == providerID { chatReq.SessionID = conv.SessionID }`;
- `outcome, chatErr := c.agent.Chat(ctx, chatReq, emit)`;
- `stored := map[string]any{}` no lugar de `map[string]any{"tool_calls": outcome.ToolCalls}`;
- `why := explainFailure(chatErr)`;
- no fim, troque o bloco `session := ...` e `TouchConversation(saveCtx, conv.ID, providerID, session)` por `_ = c.repo.TouchConversation(saveCtx, conv.ID, providerID)`.

Em `SendRequest` apague o campo `Provider`. Em `ChatRequest` apague `Module`, `SessionID` e `Model`. Em `ChatOutcome` deixe só `Text string`. Apague `runTool`, `quote`, `toolsFor` e `maxToolRounds` se ainda existir. Apague o tipo `Provider` (interface) — só havia um motor. `ProviderStatus` e `Capabilities` ficam (a tela usa); tire de `ProviderStatus` os campos `NeedsKey` e `HasKey`.

No `systemPrompt`, apague as frases que só existiam para modelos pequenos com ferramentas cortadas (o comentário "The model is given only some modules' tools when it is small" e as duas linhas `b.WriteString` logo abaixo dele). O resto do prompt fica.

- [ ] **Step 6: Anexos, explicações de erro e imagens**

Em `attachments.go`: apague `visionCapableProviders`; troque a assinatura para `func resolveAttachment(ctx context.Context, documents *service.DocumentService, documentID string) (*Attachment, error)` e a condição para `if strings.HasPrefix(doc.ContentType, "image/") {` (o agente sempre recebe a imagem). Ajuste os comentários que citam Anthropic/OpenAI para "o agente".

Substitua o conteúdo de `explain.go` (mantendo `Explanation` e as funções de erro de envio que `TestSendErrorMessage` cobre) por esta `explainFailure`:

```go
const settingsHint = "nos ajustes do chat"

// explainFailure says why the agent gave no answer; err is nil when it
// answered with nothing at all.
func explainFailure(err error) Explanation {
	if err == nil {
		return Explanation{Message: "O agente não respondeu nada. Tente reformular o pedido."}
	}
	return Explanation{Message: describeError(err), Detail: err.Error()}
}

func describeError(err error) string {
	var down *agentUnreachableError
	var api *apiError
	switch {
	case errors.Is(err, context.Canceled):
		return "Você interrompeu a resposta antes de ela terminar."
	case errors.Is(err, context.DeadlineExceeded):
		return "O agente demorou demais para responder e a resposta foi cancelada. Tente de novo."
	case errors.Is(err, errAgentNotConfigured):
		return "Configure o endereço do agente " + settingsHint + "."
	case errors.As(err, &down):
		return fmt.Sprintf("O agente não respondeu em %s. Confira se ele está rodando e se o endereço %s está certo.", down.URL, settingsHint)
	case errors.As(err, &api) && (api.Status == 401 || api.Status == 403):
		return "O agente recusou o token. Confira o token " + settingsHint + "."
	case errors.As(err, &api) && api.Status == 404:
		return "O agente não aceitou o pedido (404). No OpenClaw, ligue o endpoint chatCompletions do Gateway; confira também o modelo " + settingsHint + "."
	case errors.As(err, &api):
		return fmt.Sprintf("O agente respondeu %d. Veja os detalhes técnicos.", api.Status)
	}
	return "O agente não conseguiu responder por um erro inesperado. Tente de novo; se continuar, veja os detalhes técnicos."
}
```

Apague `engineNames`, `failedToolsNote` e qualquer helper que só eles usavam. Se `ErrNoProvider` tiver mensagem em `SendErrorMessage` citando "motor", troque o texto para "Nenhum agente ligado. Configure o agente nos ajustes do chat." e ajuste o `want` em `TestSendErrorMessage` para `"Nenhum agente"`.

Troque a tabela de `TestExplainFailure` por:

```go
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"silent", nil, "não respondeu nada"},
		{"canceled", context.Canceled, "interrompeu"},
		{"timeout", context.DeadlineExceeded, "demorou demais"},
		{"not configured", errAgentNotConfigured, "Configure o endereço"},
		{"down", &agentUnreachableError{URL: "http://localhost:8642", Err: errors.New("connection refused")}, "http://localhost:8642"},
		{"bad token", &apiError{Host: "localhost:8642", Status: 401, Message: "no"}, "recusou o token"},
		{"not found", &apiError{Host: "localhost:8642", Status: 404, Message: "no"}, "chatCompletions"},
		{"server error", &apiError{Host: "localhost:8642", Status: 500, Message: "boom"}, "respondeu 500"},
		{"unexpected", errors.New("boom"), "erro inesperado"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := explainFailure(tc.err)
			if !strings.Contains(got.Message, tc.want) {
				t.Fatalf("message = %q, want it to contain %q", got.Message, tc.want)
			}
			if tc.err != nil && got.Detail != tc.err.Error() {
				t.Fatalf("detail = %q, want the raw error", got.Detail)
			}
		})
	}
```

Em `tools_images.go`: troque as duas chamadas `apiKeyFrom(ctx, d.AssistantRepo, d.VaultKey, "openai", os.Getenv("OPENAI_API_KEY"))` por `os.Getenv("OPENAI_API_KEY")`, e a guarda de `addImages` por `if d.Documents == nil {`. Em `registry.go` apague os campos `AssistantRepo` e `VaultKey` de `Deps` e o comentário deles. Troque "Requer a API key da OpenAI" na descrição de `edit_image` por "Requer OPENAI_API_KEY no servidor".

- [ ] **Step 7: HTTP, config e `main.go`**

Em `handlers_assistant.go` apague `Transcribe`, `Speak` e `writeVoiceError`. Em `router.go` apague as rotas `voice/transcribe` e `voice/speak`.

Em `config.go` apague `MultiUser` e adicione:

```go
	// AgentURL, AgentToken and AgentModel point the chat at an external
	// agent when the settings screen leaves them empty.
	AgentURL   string
	AgentToken string
	AgentModel string
```

com `AgentURL: os.Getenv("AGENT_URL"), AgentToken: os.Getenv("AGENT_TOKEN"), AgentModel: os.Getenv("AGENT_MODEL"),` em `Load`.

Em `main.go`, troque a montagem do `chatCfg` por:

```go
	chatCfg := assistant.ChatConfig{
		MCPURL:    "http://127.0.0.1:" + cfg.Port + "/mcp",
		MCPToken:  mcpToken,
		Env:       assistant.NewAgentEnv(cfg.AgentURL, cfg.AgentToken, cfg.AgentModel),
		Documents: documentServiceOrNil(documentService, documentErr),
	}
```

apague `appleBridge := ...`, os campos `AssistantRepo:` e `VaultKey:` de `assistant.Deps`, o comentário "Built ahead of the tool registry…", `defer chat.Close()`, e troque `assistant.NewChat(tools, assistantRepo, chatCfg, appleBridge)` por `assistant.NewChat(tools, assistantRepo, chatCfg)`. Se `getenv` ficar sem uso, apague a função.

Em `backend/.env.example`: apague `ASSISTANT_MULTI_USER`, `ANTHROPIC_API_KEY`, `APPLE_BRIDGE_BIN`, `APPLE_BRIDGE_URL`, `ASSISTANT_VOICE` e os comentários deles; mantenha `OPENAI_API_KEY` com o comentário `# Só para gerar imagem (generate_image/edit_image). Sem ela, usa o Draw Things local.`; adicione:

```bash
# Agente externo do chat (Hermes, OpenClaw…). A tela de ajustes do chat
# tem prioridade; estes valores cobrem o que ficar vazio lá.
# Hermes: http://127.0.0.1:8642 com o API_SERVER_KEY dele como token.
AGENT_URL=
AGENT_TOKEN=
AGENT_MODEL=
```

- [ ] **Step 8: Compilar e testar**

Run: `cd backend && go build ./... && go vet ./... && go test ./internal/assistant/... ./internal/httpapi/... ./internal/domain/... ./internal/service/...`
Expected: sem erros. Qualquer teste que ainda cite `claude_code`, `apple`, `ollama`, `modulesFor`, `Voice` ou `SessionID` testa código removido: apague o caso.

Run: `grep -rn "AppleBridge\|provider_apple\|claude_code\|ollama\|MultiUser\|SessionID" backend --include='*.go'`
Expected: nada.

- [ ] **Step 9: Commit**

```bash
git add -A backend apple-bridge
git commit -m "refactor(assistant): chat só com o agente externo, sem ponte Apple nem motores próprios"
```

---

### Task 4: Frontend — ajustes do agente e voz do navegador

**Files:**
- Create: `frontend/components/assistant/browserVoice.ts`
- Delete: `frontend/components/assistant/useVoiceRecorder.ts`, `frontend/components/assistant/wav.ts`
- Modify: `frontend/components/assistant/types.ts`
- Modify: `frontend/components/assistant/EngineSettings.tsx`
- Modify: `frontend/components/assistant/ChatApp.tsx`
- Modify: `frontend/app/chat/chat.css`

**Interfaces:**
- Consumes: JSON de `GET/PUT /api/assistant/settings` da Task 3.
- Produces:
  - `useDictation(onHeard: (text: string) => void, onNotice: (msg: string) => void): { supported: boolean; listening: boolean; start: () => void; stop: () => void }`
  - `speakText(text: string, onEnd: () => void): boolean`
  - `stopSpeech(): void`

- [ ] **Step 1: Tipos**

Em `types.ts`, troque `Capabilities`, `ProviderStatus` e `AssistantSettings` por:

```ts
/** What the agent's model can do, as the settings screen shows it. */
export interface Capabilities {
  supports_image_gen: boolean;
  supports_vision: boolean;
  supports_voice: boolean;
}

export interface ProviderStatus {
  id: string;
  name: string;
  kind: "login" | "api" | "local";
  available: boolean;
  detail: string;
  model: string;
  models?: string[];
  capabilities: Capabilities;
}

export interface AssistantSettings {
  /** "agent" when the chat answers through the external agent, "none" for shortcuts only. */
  provider: string;
  providers: ProviderStatus[];
  agent: { url: string; model: string; has_token: boolean };
  mcp: { url: string; token: string };
  can_store_keys: boolean;
}
```

- [ ] **Step 2: Voz do navegador**

`frontend/components/assistant/browserVoice.ts`:

```ts
"use client";

import { useCallback, useEffect, useRef, useState } from "react";

// Dictation and read-aloud from the browser itself: the agents behind the
// chat take no audio. Chrome sends dictated audio to Google to transcribe.

type RecognitionResult = ArrayLike<{ transcript: string }>;

interface Recognition {
  lang: string;
  interimResults: boolean;
  continuous: boolean;
  start(): void;
  stop(): void;
  abort(): void;
  onresult: ((e: { results: ArrayLike<RecognitionResult> }) => void) | null;
  onerror: ((e: { error: string }) => void) | null;
  onend: (() => void) | null;
}

function recognitionCtor(): (new () => Recognition) | null {
  if (typeof window === "undefined") return null;
  const w = window as unknown as { SpeechRecognition?: new () => Recognition; webkitSpeechRecognition?: new () => Recognition };
  return w.SpeechRecognition ?? w.webkitSpeechRecognition ?? null;
}

export function useDictation(onHeard: (text: string) => void, onNotice: (msg: string) => void) {
  const [supported, setSupported] = useState(false);
  const [listening, setListening] = useState(false);
  const recRef = useRef<Recognition | null>(null);
  // The latest callbacks, so the caller can pass plain functions that read
  // fresh state without making start() change on every render.
  const heardRef = useRef(onHeard);
  const noticeRef = useRef(onNotice);
  useEffect(() => {
    heardRef.current = onHeard;
    noticeRef.current = onNotice;
  });

  useEffect(() => {
    // Only known in the browser; the server render always says unsupported.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setSupported(recognitionCtor() !== null);
    return () => recRef.current?.abort();
  }, []);

  const start = useCallback(() => {
    const Ctor = recognitionCtor();
    if (!Ctor || recRef.current) return;
    const rec = new Ctor();
    rec.lang = "pt-BR";
    rec.interimResults = false;
    rec.continuous = false;
    let heard = "";
    let failed = false;
    rec.onresult = (e) => {
      heard = Array.from(e.results)
        .map((r) => r[0]?.transcript ?? "")
        .join(" ")
        .trim();
    };
    rec.onerror = (e) => {
      failed = true;
      if (e.error === "not-allowed" || e.error === "service-not-allowed") noticeRef.current("O navegador bloqueou o microfone.");
      else if (e.error === "no-speech") noticeRef.current("Não ouvi nada — tente de novo.");
      else if (e.error !== "aborted") noticeRef.current("Não consegui ouvir. Tente de novo.");
    };
    rec.onend = () => {
      recRef.current = null;
      setListening(false);
      if (heard) heardRef.current(heard);
      else if (!failed) noticeRef.current("Não ouvi nada — tente de novo.");
    };
    recRef.current = rec;
    setListening(true);
    rec.start();
  }, []);

  const stop = useCallback(() => recRef.current?.stop(), []);

  return { supported, listening, start, stop };
}

/** Reads text aloud with the first pt-BR voice; false when the browser can't. */
export function speakText(text: string, onEnd: () => void): boolean {
  if (typeof window === "undefined" || !("speechSynthesis" in window)) return false;
  const utterance = new SpeechSynthesisUtterance(text);
  utterance.lang = "pt-BR";
  const voice = window.speechSynthesis.getVoices().find((v) => v.lang.replace("_", "-").startsWith("pt-BR"));
  if (voice) utterance.voice = voice;
  utterance.onend = onEnd;
  utterance.onerror = onEnd;
  window.speechSynthesis.cancel();
  window.speechSynthesis.speak(utterance);
  return true;
}

export function stopSpeech() {
  if (typeof window !== "undefined" && "speechSynthesis" in window) window.speechSynthesis.cancel();
}
```

```bash
git rm -q frontend/components/assistant/useVoiceRecorder.ts frontend/components/assistant/wav.ts
```

- [ ] **Step 3: ChatApp com a voz nova**

Em `ChatApp.tsx`:
- troque `import { useVoiceRecorder } from "./useVoiceRecorder";` por `import { speakText, stopSpeech, useDictation } from "./browserVoice";` e tire `IconStop` do import de ícones se ficar sem uso;
- apague o bloco `visionSwitchTarget` (a constante e o `<span className="cx-vision-hint">…</span>` que a usa);
- troque a seção `// ---- voice` inteira (de `const voiceOn` até `const recorder = useVoiceRecorder(...)`) por:

```tsx
  // ---- voice

  const [speaking, setSpeaking] = useState(false);
  const [voiceNotice, setVoiceNotice] = useState<string | null>(null);

  const stopSpeaking = useCallback(() => {
    stopSpeech();
    setSpeaking(false);
  }, []);

  // Leaving the chat shouldn't leave an answer talking to an empty room.
  useEffect(() => () => stopSpeech(), []);

  const speak = ({ id, spoken }: Reply) => {
    stopSpeaking();
    if (!spoken.trim()) return;
    if (speakText(spoken, () => setSpeaking(false))) setSpeaking(true);
    else update(id, { spokenError: "Este navegador não lê em voz alta." });
  };

  const dictation = useDictation((heard) => {
    setVoiceNotice(null);
    void sendText(heard, true).then((reply) => speak(reply));
  }, setVoiceNotice);
  const voiceOn = dictation.supported;
```

- troque o bloco da barra de voz (`{(voiceNotice || speaking || recorder.state === "processing") && (` … `<span>{recorder.state === "processing" ? "Transcrevendo…" : voiceNotice}</span>`) para usar `{(voiceNotice || speaking || dictation.listening) && (` e `<span>{dictation.listening ? "Ouvindo…" : voiceNotice}</span>`;
- troque o botão do microfone por:

```tsx
          {voiceOn && (
            <button
              type="button"
              className={`cx-mic${dictation.listening ? " is-recording" : ""}`}
              aria-label={dictation.listening ? "Parar de ouvir" : "Falar"}
              disabled={busy}
              onClick={() => {
                if (dictation.listening) {
                  dictation.stop();
                  return;
                }
                setVoiceNotice(null);
                stopSpeaking();
                dictation.start();
              }}
            >
              <IconMic />
            </button>
          )}
```

- troque o texto do botão `Configurar motores…` por `Configurar agente…`.

A seção de voz já fica abaixo de `sendText` no arquivo; mantenha essa ordem.

- [ ] **Step 4: Tela de ajustes**

Reescreva `EngineSettings.tsx` mantendo `Copy` e `Snippet` como estão e trocando o resto por:

```tsx
export function EngineSettings({ settings, onClose, onChanged }: { settings: AssistantSettings | null; onClose: () => void; onChanged: () => void }) {
  const [tab, setTab] = useState<"agent" | "mcp">("agent");
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [reveal, setReveal] = useState(false);
  const [origin, setOrigin] = useState("");

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setOrigin(window.location.origin);
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && onClose();
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  const save = async (body: Record<string, unknown>) => {
    setSaving(true);
    setError(null);
    const res = await fetch("/api/assistant/settings", { method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
    setSaving(false);
    if (!res.ok) {
      const data = (await res.json().catch(() => null)) as { error?: string } | null;
      setError(data?.error ?? "Não foi possível salvar.");
      return;
    }
    onChanged();
  };

  const token = settings?.mcp.token ?? "";
  const shown = reveal ? token : token ? `${token.slice(0, 10)}…${token.slice(-4)}` : "";
  const mcpURL = `${origin}/mcp`;

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-shell is-wide es" onClick={(e) => e.stopPropagation()} role="dialog" aria-label="Agente e conexões">
        <div className="panel">
          <button className="icon-btn modal-close" type="button" aria-label="Fechar" onClick={onClose}>
            <IconClose />
          </button>
          <div className="panel-head">
            <h2>Agente e conexões</h2>
          </div>
          <div className="seg es-tabs" role="tablist">
            <button type="button" role="tab" aria-selected={tab === "agent"} className={tab === "agent" ? "active" : ""} onClick={() => setTab("agent")}>
              Agente
            </button>
            <button type="button" role="tab" aria-selected={tab === "mcp"} className={tab === "mcp" ? "active" : ""} onClick={() => setTab("mcp")}>
              MCP
            </button>
          </div>
          {error && <p className="form-error">{error}</p>}

          {tab === "agent" &&
            (settings === null ? <p className="es-hint">Verificando o agente…</p> : <AgentForm settings={settings} saving={saving} save={save} onTest={onChanged} />)}

          {tab === "mcp" && settings && (
            <div className="es-mcp">
              <p className="es-hint">
                O agente lê e grava no Estus por aqui: gastos, contas, notas, lembretes, agenda, hábitos… O cofre de senhas nunca é exposto. Configure
                este endereço e o token no MCP do seu agente.
              </p>
              <div className="es-token">
                <span>Token</span>
                <code>{shown}</code>
                <button type="button" className="btn-text" onClick={() => setReveal((v) => !v)}>
                  {reveal ? "Ocultar" : "Mostrar"}
                </button>
                <Copy text={token} />
              </div>
              <Snippet
                token={token}
                reveal={reveal}
                title="Hermes Agent"
                hint="Em ~/.hermes/config.yaml, na seção mcp_servers."
                text={`mcp_servers:\n  estus:\n    url: "${settings.mcp.url}"\n    headers:\n      Authorization: "Bearer ${token}"`}
              />
              <Snippet
                token={token}
                reveal={reveal}
                title="OpenClaw e outros"
                hint="Qualquer agente com MCP por HTTP: este endereço, com o token no cabeçalho Authorization."
                text={`URL: ${settings.mcp.url}\nAuthorization: Bearer ${token}`}
              />
              <p className="es-hint">Agente em outra máquina: troque o endereço por {mcpURL}.</p>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

function AgentForm({
  settings,
  saving,
  save,
  onTest,
}: {
  settings: AssistantSettings;
  saving: boolean;
  save: (body: Record<string, unknown>) => Promise<void>;
  onTest: () => void;
}) {
  const status = settings.providers.find((p) => p.id === "agent");
  const [url, setURL] = useState(settings.agent.url);
  const [model, setModel] = useState(settings.agent.model);
  const [token, setToken] = useState("");
  const on = settings.provider === "agent";
  const dirty = url !== settings.agent.url || model !== settings.agent.model || token.trim() !== "";

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    const body: Record<string, unknown> = { agent_url: url, agent_model: model, provider: "agent" };
    if (token.trim()) body.agent_token = token;
    await save(body);
    setToken("");
  };

  return (
    <form className="es-agent" onSubmit={(e) => void submit(e)}>
      <p className={`es-hint${status?.available ? " is-ok" : ""}`}>
        <b>{status?.available ? "Conectado." : "Não conectado."}</b> {status?.detail}
      </p>
      <label className="es-field">
        <span>Endereço</span>
        <input value={url} placeholder="http://127.0.0.1:8642" onChange={(e) => setURL(e.target.value)} />
      </label>
      <label className="es-field">
        <span>Token</span>
        <input
          type="password"
          value={token}
          autoComplete="off"
          placeholder={settings.agent.has_token ? "Token salvo — cole outro para trocar" : "API_SERVER_KEY do Hermes ou token do Gateway"}
          onChange={(e) => setToken(e.target.value)}
          disabled={!settings.can_store_keys}
        />
      </label>
      {!settings.can_store_keys && <p className="es-hint">Para salvar o token aqui, defina VAULT_ENCRYPTION_KEY. Ou use AGENT_TOKEN no .env.</p>}
      <label className="es-field">
        <span>Modelo</span>
        {status?.models && status.models.length > 0 ? (
          <select value={model} onChange={(e) => setModel(e.target.value)}>
            <option value="">O primeiro que o agente oferece</option>
            {status.models.map((m) => (
              <option key={m} value={m}>
                {m}
              </option>
            ))}
          </select>
        ) : (
          <input value={model} placeholder="hermes-agent ou openclaw/default" onChange={(e) => setModel(e.target.value)} />
        )}
      </label>
      <div className="es-inline">
        <button type="submit" className="btn-primary" disabled={saving || (!dirty && on)}>
          {saving ? "Salvando…" : "Salvar e ligar"}
        </button>
        <button type="button" className="btn-outline" onClick={onTest}>
          Testar conexão
        </button>
        {on && (
          <button type="button" className="btn-text" onClick={() => void save({ provider: "none" })}>
            Desligar
          </button>
        )}
        {settings.agent.has_token && (
          <button type="button" className="btn-text" onClick={() => void save({ agent_token: "" })}>
            Remover token
          </button>
        )}
      </div>
      <p className="es-hint">Voz: o ditado e a leitura usam o próprio navegador. No Chrome, o ditado passa pelo servidor do Google.</p>
    </form>
  );
}
```

Apague `KIND_LABEL`, `CapabilityGaps` e `EngineRow`. Tire `Capabilities` e `ProviderStatus` do import de tipos se ficarem sem uso.

- [ ] **Step 5: CSS**

Em `frontend/app/chat/chat.css`, apague as regras `.es-engines`, `.es-engine*` (todas as que começam por `.es-engine` ou `label.es-engine`) e `.cx-vision-hint` se existir. Adicione, perto das outras regras `.es-`:

```css
/* The agent form: label above input, one field per row. */
.es-agent {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.es-field {
  display: flex;
  flex-direction: column;
  gap: 5px;
  font-size: 12.5px;
  font-weight: 600;
  color: var(--ink-soft);
}
.es-field input,
.es-field select {
  padding: 8px 10px;
  border-radius: 8px;
  border: 1px solid var(--hairline);
  background: var(--surface);
  color: var(--ink);
  font: inherit;
  font-weight: 400;
}
```

- [ ] **Step 6: Verificar**

Run: `cd frontend && npx tsc --noEmit && npm run lint`
Expected: sem erros.

Run: `grep -rn "voice/speak\|voice/transcribe\|useVoiceRecorder\|ollama_url\|needs_key\|multi_user" frontend/app frontend/components frontend/lib`
Expected: nada.

- [ ] **Step 7: Commit**

```bash
git add -A frontend
git commit -m "feat(chat): ajustes do agente externo e voz pelo navegador"
```

---

### Task 5: Testar ponta a ponta com um agente falso

**Files:**
- Create (fora do repo, no scratchpad da sessão, sem commit): `main.go` de um agente falso.

- [ ] **Step 1: Subir um agente falso**

Crie num diretório temporário um `main.go` que escuta `127.0.0.1:18642`, exige `Authorization: Bearer teste`, responde `GET /v1/models` com `{"data":[{"id":"fake"}]}` e `POST /v1/chat/completions` com SSE de duas partes (`"Oi do "`, `"agente falso."`) e `data: [DONE]`. Rode com `go run .` em background.

- [ ] **Step 2: Subir o Estus e conversar**

Run: `scripts/estus-brain.sh --rebuild`
Nos ajustes do chat, aba Agente: endereço `http://127.0.0.1:18642`, token `teste`, clique em "Salvar e ligar". Expected: "Conectado." e o modelo `fake` na lista.
Mande "oi" no chat. Expected: a resposta "Oi do agente falso." aparece aos poucos e fica na conversa depois de recarregar.
Pare o agente falso e mande outra mensagem. Expected: "O agente não respondeu em http://127.0.0.1:18642…".

- [ ] **Step 3: Desfazer a configuração de teste**

Nos ajustes, clique em "Remover token" e "Desligar", e apague o endereço, para não deixar o agente falso configurado nos dados reais.

---

### Task 6: README e varredura de código morto

**Files:**
- Modify: `README.md`
- Modify: qualquer arquivo com código sem chamador achado pela varredura

- [ ] **Step 1: README**

- Apague as menções a Claude Code, Codex, Anthropic, OpenAI (como motor), Ollama e Apple Intelligence como motores do chat, e a seção "Voz" (hoje por volta das linhas 275-295), incluindo `apple-bridge` nos requisitos.
- Na parte do MCP, troque os exemplos de Claude Code/Codex/Desktop por "o agente externo (Hermes, OpenClaw…) aponta para `/mcp` com o token".
- Adicione a seção:

```markdown
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
   ficam em Chat > Configurar agente > MCP.
3. Em Chat > Configurar agente > Agente, preencha o endereço e o token do
   agente e clique em "Salvar e ligar".
4. Avisos: crie no agente uma tarefa agendada que chama as ferramentas de
   lembretes e agenda do Estus (ex.: toda manhã às 8h, "liste os lembretes e
   eventos de hoje e me mande") e envia pelo canal dele (WhatsApp, Telegram…).
   O Estus não manda notificações sozinho.

Também dá para configurar pelo `.env` do backend: `AGENT_URL`, `AGENT_TOKEN`
e `AGENT_MODEL`.
```

- [ ] **Step 2: Varredura do backend**

Run: `cd backend && go run golang.org/x/tools/cmd/deadcode@latest -test ./...`
Expected: uma lista de funções sem chamador. Para cada item de `internal/assistant`, `internal/httpapi`, `internal/store/postgres` e `internal/config`, apague a função (e testes que só a testavam) quando ela ficou sem uso por causa deste trabalho. Não apague funções de outros módulos sem confirmar que ninguém as usa (`grep -rn NomeDaFuncao backend`).

Run: `cd backend && go build ./... && go vet ./... && go test ./internal/...`
Expected: sem erros (os de integração podem ser pulados sem banco).

- [ ] **Step 3: Varredura do frontend**

Run: `cd frontend && npx --yes ts-unused-exports tsconfig.json --excludePathsFromReport='app/;proxy.ts;next.config.ts'`
Expected: lista de exports sem uso. Apague os que ficaram sem uso por causa deste trabalho (tipos e helpers de motores, voz antiga, Telegram).

Run: `grep -rn "\.cx-vision-hint\|\.es-engine\|\.tg-" frontend/app` e apague regras CSS sem uso.

Run: `cd frontend && npx tsc --noEmit && npm run lint`
Expected: sem erros.

- [ ] **Step 4: Última busca**

Run: `grep -rni "telegram\|apple.bridge\|apple intelligence\|ollama\|claude code\|codex" --exclude-dir=node_modules --exclude-dir=.next --exclude-dir=.git --exclude-dir=docs --exclude-dir=migrations .`
Expected: nada fora de documentação histórica.

- [ ] **Step 5: Commit**

```bash
git add -A README.md backend frontend
git commit -m "chore: remove código morto e documenta a conexão com o agente"
```
