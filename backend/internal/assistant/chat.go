package assistant

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/rafael/estus-vault/backend/internal/domain"
	"github.com/rafael/estus-vault/backend/internal/service"
	"github.com/rafael/estus-vault/backend/internal/store/postgres"
)

// ---------------------------------------------------------------- events

// Event is one step of an answer as it streams to the chat: text arriving,
// a tool starting and finishing, the end, or an error.
type Event struct {
	Type           string          `json:"type"` // conversation | text | tool | tool_result | error | done
	ConversationID string          `json:"conversation_id,omitempty"`
	MessageID      string          `json:"message_id,omitempty"`
	Text           string          `json:"text,omitempty"`
	ToolID         string          `json:"tool_id,omitempty"`
	Tool           string          `json:"tool,omitempty"`
	Args           json.RawMessage `json:"args,omitempty"`
	Result         any             `json:"result,omitempty"`
	Error          string          `json:"error,omitempty"`
	Detail         string          `json:"detail,omitempty"` // the raw error behind Error, for "detalhes técnicos"
	Provider       string          `json:"provider,omitempty"`
}

// ToolCall is the record of one tool used while answering.
type ToolCall struct {
	ID     string          `json:"id"`
	Name   string          `json:"name"`
	Args   json.RawMessage `json:"args"`
	Result any             `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

// ---------------------------------------------------------------- providers

type Message struct {
	Role    string `json:"role"` // user | assistant
	Content string `json:"content"`
}

type ChatRequest struct {
	System    string
	History   []Message
	Message   string
	Module    string
	SessionID string
	Model     string
	// Attachment carries an image a vision-capable provider can send as real
	// pixels — see visionCapableProviders. Anything else the person attached
	// is already folded into Message as text by Chat.Send, so most providers
	// never need to know this field exists.
	Attachment *Attachment
}

type ChatOutcome struct {
	Text      string
	SessionID string
	ToolCalls []ToolCall
}

type Provider interface {
	ID() string
	Status(ctx context.Context) ProviderStatus
	Chat(ctx context.Context, req ChatRequest, emit func(Event)) (ChatOutcome, error)
}

type ProviderStatus struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Kind      string   `json:"kind"` // login | api | local
	Available bool     `json:"available"`
	Detail    string   `json:"detail"`
	NeedsKey  bool     `json:"needs_key,omitempty"`
	HasKey    bool     `json:"has_key,omitempty"`
	Model     string   `json:"model"`
	Models    []string `json:"models,omitempty"`
	// Capabilities describe what this engine's own model can do, regardless
	// of whether Estus has wired the feature up yet — a fact for the
	// settings screen to show ("Claude não gera imagem"), not a switch.
	Capabilities Capabilities `json:"capabilities"`
}

// Capabilities is what one engine's own model can do. SupportsImageGen is
// the only one Estus actually wires to a tool today (generate_image, gated
// by it in toolsFor); SupportsVision and SupportsVoice are informational,
// ahead of file upload wiring them to something real.
type Capabilities struct {
	SupportsImageGen bool `json:"supports_image_gen"`
	SupportsVision   bool `json:"supports_vision"`
	SupportsVoice    bool `json:"supports_voice"`
}

// ---------------------------------------------------------------- chat

type ChatConfig struct {
	Dir       string
	MCPURL    string
	MCPToken  string
	ToolsURL  string
	MultiUser bool
	// VaultKey encrypts stored API keys; without it keys come only from env.
	VaultKey *[32]byte
	// AppleBridgeBin is the path to the Apple Intelligence helper, started on demand.
	AppleBridgeBin string
	AppleBridgeURL string
	// Voice is the macOS voice that reads answers aloud, e.g. "Luciana".
	Voice string
	// Documents resolves a chat attachment (see SendRequest.AttachmentID) —
	// the same storage generate_image and documents_write already use.
	Documents *service.DocumentService
}

type Chat struct {
	tools     *Registry
	repo      *postgres.AssistantRepo
	cfg       ChatConfig
	providers map[string]Provider
	order     []string
	mu        sync.Mutex
	bridge    *AppleBridge
	voice     *Voice
}

// NewChat takes bridge rather than building its own: the generate_image tool
// (registered on tools before Chat exists) needs the very same instance, or
// two processes would race for the same port.
func NewChat(tools *Registry, repo *postgres.AssistantRepo, cfg ChatConfig, bridge *AppleBridge) *Chat {
	c := &Chat{tools: tools, repo: repo, cfg: cfg, providers: map[string]Provider{}, bridge: bridge}
	c.voice = newVoice(c.bridge, cfg.Voice)
	for _, p := range []Provider{
		&claudeCodeProvider{chat: c},
		&codexProvider{chat: c},
		&anthropicProvider{chat: c},
		&openAIProvider{chat: c},
		&ollamaProvider{chat: c},
		&appleProvider{chat: c},
	} {
		c.providers[p.ID()] = p
		c.order = append(c.order, p.ID())
	}
	return c
}

// Close stops helpers the chat started (the Apple bridge).
func (c *Chat) Close() {
	c.bridge.stop()
}

// Voice is the Mac's speech, shared by the web chat and the Telegram bot.
func (c *Chat) Voice() *Voice { return c.voice }

var defaultModels = map[string]string{
	"claude_code": "sonnet",
	"codex":       "",
	"anthropic":   "claude-sonnet-5",
	"openai":      "gpt-5-mini",
	"ollama":      "",
	"apple":       "on-device",
}

func (c *Chat) model(s postgres.AssistantSettings, provider string) string {
	if m := strings.TrimSpace(s.Models[provider]); m != "" {
		return m
	}
	return defaultModels[provider]
}

// ---- settings

type SettingsView struct {
	Provider  string           `json:"provider"`
	OllamaURL string           `json:"ollama_url"`
	Providers []ProviderStatus `json:"providers"`
	MCP       struct {
		URL   string `json:"url"`
		Token string `json:"token"`
	} `json:"mcp"`
	CanStoreKeys bool        `json:"can_store_keys"`
	MultiUser    bool        `json:"multi_user"`
	Voice        VoiceStatus `json:"voice"`
}

func (c *Chat) Settings(ctx context.Context) (SettingsView, error) {
	s, err := c.repo.Settings(ctx)
	if err != nil {
		return SettingsView{}, err
	}
	view := SettingsView{Provider: s.Provider, OllamaURL: s.OllamaURL, CanStoreKeys: c.cfg.VaultKey != nil, MultiUser: c.cfg.MultiUser}
	view.MCP.URL = c.cfg.MCPURL
	view.MCP.Token = c.cfg.MCPToken

	statuses := make([]ProviderStatus, len(c.order))
	var wg sync.WaitGroup
	for i, id := range c.order {
		wg.Add(1)
		go func(i int, p Provider) {
			defer wg.Done()
			sctx, cancel := context.WithTimeout(ctx, 4*time.Second)
			defer cancel()
			statuses[i] = p.Status(sctx)
		}(i, c.providers[id])
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		vctx, cancel := context.WithTimeout(ctx, 6*time.Second)
		defer cancel()
		view.Voice = c.voice.Status(vctx)
	}()
	wg.Wait()
	view.Providers = statuses
	return view, nil
}

type SettingsUpdate struct {
	Provider  *string           `json:"provider"`
	Models    map[string]string `json:"models"`
	Keys      map[string]string `json:"keys"` // provider → key; "" removes it
	OllamaURL *string           `json:"ollama_url"`
}

func (c *Chat) UpdateSettings(ctx context.Context, u SettingsUpdate) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	s, err := c.repo.Settings(ctx)
	if err != nil {
		return err
	}
	if u.Provider != nil {
		if *u.Provider != "none" {
			if _, ok := c.providers[*u.Provider]; !ok {
				return fmt.Errorf("%w: unknown provider %q", domain.ErrValidation, *u.Provider)
			}
		}
		s.Provider = *u.Provider
	}
	for p, m := range u.Models {
		if _, ok := c.providers[p]; ok {
			s.Models[p] = strings.TrimSpace(m)
		}
	}
	for p, key := range u.Keys {
		if p != "anthropic" && p != "openai" {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" {
			delete(s.Secrets, p)
			continue
		}
		if c.cfg.VaultKey == nil {
			return fmt.Errorf("%w: set VAULT_ENCRYPTION_KEY to store API keys, or use ANTHROPIC_API_KEY / OPENAI_API_KEY", domain.ErrValidation)
		}
		ct, nonce, err := domain.EncryptPassword(*c.cfg.VaultKey, key)
		if err != nil {
			return err
		}
		s.Secrets[p] = base64.StdEncoding.EncodeToString(append(nonce, ct...))
	}
	if u.OllamaURL != nil {
		s.OllamaURL = strings.TrimRight(strings.TrimSpace(*u.OllamaURL), "/")
	}
	return c.repo.SaveSettings(ctx, s)
}

// apiKey returns a stored key (decrypted) or the env fallback.
func (c *Chat) apiKey(ctx context.Context, provider, envValue string) string {
	return apiKeyFrom(ctx, c.repo, c.cfg.VaultKey, provider, envValue)
}

// apiKeyFrom is apiKey's logic as a free function: generate_image needs it
// too, from the registry, which is built before any Chat exists.
func apiKeyFrom(ctx context.Context, repo *postgres.AssistantRepo, vaultKey *[32]byte, provider, envValue string) string {
	s, err := repo.Settings(ctx)
	if err == nil && vaultKey != nil {
		if blob, ok := s.Secrets[provider]; ok {
			raw, err := base64.StdEncoding.DecodeString(blob)
			if err == nil && len(raw) > 12 {
				if key, err := domain.DecryptPassword(*vaultKey, raw[12:], raw[:12]); err == nil {
					return key
				}
			}
		}
	}
	return envValue
}

// ---- conversations

type ConversationView struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Module    string    `json:"module"`
	Provider  string    `json:"provider"`
	UpdatedAt time.Time `json:"updated_at"`
}

type MessageView struct {
	ID        string          `json:"id"`
	Role      string          `json:"role"`
	Content   string          `json:"content"`
	Data      json.RawMessage `json:"data"`
	Provider  string          `json:"provider"`
	CreatedAt time.Time       `json:"created_at"`
}

func (c *Chat) Conversations(ctx context.Context) ([]ConversationView, error) {
	list, err := c.repo.ListConversations(ctx, 100)
	if err != nil {
		return nil, err
	}
	out := make([]ConversationView, len(list))
	for i, cv := range list {
		out[i] = ConversationView{cv.ID, cv.Title, cv.Module, cv.Provider, cv.UpdatedAt}
	}
	return out, nil
}

func (c *Chat) Conversation(ctx context.Context, id string) (ConversationView, []MessageView, error) {
	cv, err := c.repo.GetConversation(ctx, id)
	if err != nil {
		return ConversationView{}, nil, err
	}
	msgs, err := c.repo.Messages(ctx, id, 500)
	if err != nil {
		return ConversationView{}, nil, err
	}
	out := make([]MessageView, len(msgs))
	for i, m := range msgs {
		out[i] = MessageView{m.ID, m.Role, m.Content, m.Data, m.Provider, m.CreatedAt}
	}
	return ConversationView{cv.ID, cv.Title, cv.Module, cv.Provider, cv.UpdatedAt}, out, nil
}

func (c *Chat) DeleteConversation(ctx context.Context, id string) error {
	return c.repo.DeleteConversation(ctx, id)
}

func titleFrom(message string) string {
	t := strings.Join(strings.Fields(message), " ")
	if t == "" {
		return "Nova conversa"
	}
	return truncate(t, 60)
}

// Record stores a turn of a ready-made flow (no AI involved), creating the
// conversation when needed, so flows and AI answers share one history.
func (c *Chat) Record(ctx context.Context, conversationID, module, userText, assistantText string, data json.RawMessage) (string, error) {
	if conversationID == "" {
		cv, err := c.repo.CreateConversation(ctx, titleFrom(userText), module)
		if err != nil {
			return "", err
		}
		conversationID = cv.ID
	}
	if strings.TrimSpace(userText) != "" {
		if _, err := c.repo.AddMessage(ctx, postgres.ConversationMessage{ConversationID: conversationID, Role: "user", Content: userText}); err != nil {
			return "", err
		}
	}
	if _, err := c.repo.AddMessage(ctx, postgres.ConversationMessage{ConversationID: conversationID, Role: "assistant", Content: assistantText, Data: data, Provider: "fluxo"}); err != nil {
		return "", err
	}
	return conversationID, nil
}

// ---- sending

type SendRequest struct {
	ConversationID string `json:"conversation_id"`
	Message        string `json:"message"`
	Module         string `json:"module"`
	Provider       string `json:"provider"`
	// AttachmentID is a document already uploaded (POST /api/assistant/attachments)
	// that this message refers to — an image, PDF, spreadsheet or text file.
	AttachmentID string `json:"attachment_id"`
	// Title names a conversation Send creates; empty uses the message's start.
	Title string `json:"-"`
	// SentAt is when the person actually wrote the message, for a channel
	// that can deliver it late (Telegram queues up to 24h while the bot is
	// off). Zero means "now": the web chat has no such delay.
	SentAt time.Time `json:"-"`
}

var ErrNoProvider = errors.New("no AI engine selected")

// Send answers a message with the chosen engine, streaming events, and keeps
// both sides of the turn in the conversation.
func (c *Chat) Send(ctx context.Context, req SendRequest, emit func(Event)) error {
	message := strings.TrimSpace(req.Message)
	if message == "" {
		return fmt.Errorf("%w: a mensagem está vazia", domain.ErrValidation)
	}
	settings, err := c.repo.Settings(ctx)
	if err != nil {
		return err
	}
	providerID := req.Provider
	if providerID == "" {
		providerID = settings.Provider
	}
	provider, ok := c.providers[providerID]
	if !ok {
		return ErrNoProvider
	}
	if c.cfg.MultiUser && (providerID == "claude_code" || providerID == "codex") {
		return fmt.Errorf("%w: motores que usam o login da sua assinatura ficam desligados no modo multiusuário; escolha outro motor", domain.ErrValidation)
	}

	var attachment *Attachment
	if req.AttachmentID != "" {
		attachment, err = resolveAttachment(ctx, c.cfg.Documents, req.AttachmentID, providerID)
		if err != nil {
			return fmt.Errorf("anexo: %w", err)
		}
	}

	var conv postgres.Conversation
	if req.ConversationID != "" {
		conv, err = c.repo.GetConversation(ctx, req.ConversationID)
	} else {
		title := req.Title
		if title == "" {
			title = titleFrom(message)
		}
		conv, err = c.repo.CreateConversation(ctx, title, req.Module)
	}
	if err != nil {
		return err
	}
	emit(Event{Type: "conversation", ConversationID: conv.ID, Provider: providerID})

	history, err := c.repo.Messages(ctx, conv.ID, 30)
	if err != nil {
		return err
	}
	var userData json.RawMessage
	if attachment != nil {
		userData, _ = json.Marshal(map[string]any{"attachment": map[string]string{
			"document_id": req.AttachmentID, "name": attachment.Name, "content_type": attachment.ContentType,
		}})
	}
	if _, err := c.repo.AddMessage(ctx, postgres.ConversationMessage{ConversationID: conv.ID, Role: "user", Content: message, Data: userData}); err != nil {
		return err
	}

	chatReq := ChatRequest{
		System:  c.systemPrompt(req.Module, req.SentAt),
		Message: message,
		Module:  req.Module,
		Model:   c.model(settings, providerID),
	}
	// A CLI engine's session only continues if that same engine answered last.
	if conv.Provider == providerID {
		chatReq.SessionID = conv.SessionID
	}
	// Engines want turns that alternate and start with the person: merge
	// consecutive messages of one side (a flow can add several) and drop any
	// leading assistant text.
	for _, m := range history {
		if strings.TrimSpace(m.Content) == "" {
			continue
		}
		n := len(chatReq.History)
		switch {
		case n == 0 && m.Role != "user":
			continue
		case n > 0 && chatReq.History[n-1].Role == m.Role:
			chatReq.History[n-1].Content += "\n\n" + m.Content
		default:
			chatReq.History = append(chatReq.History, Message{Role: m.Role, Content: m.Content})
		}
	}
	if n := len(chatReq.History); n > 0 && chatReq.History[n-1].Role == "user" {
		chatReq.Message = chatReq.History[n-1].Content + "\n\n" + chatReq.Message
		chatReq.History = chatReq.History[:n-1]
	}
	// A vision-capable engine gets the image itself on ChatRequest.Attachment
	// (each such provider builds its own content blocks from it); every other
	// attachment is already text by now, so it's just more of the message —
	// no provider needs to know it came from a file.
	if attachment != nil {
		if attachment.ImageBase64 != "" {
			chatReq.Attachment = attachment
		} else if attachment.Text != "" {
			chatReq.Message = fmt.Sprintf("[Anexo: %s]\n%s\n\n%s", attachment.Name, attachment.Text, chatReq.Message)
		}
	}

	outcome, chatErr := provider.Chat(ctx, chatReq, emit)
	if chatErr != nil && ctx.Err() != nil {
		// The person stopped the answer (or left); say that, not what the
		// engine made of being cut off.
		chatErr = fmt.Errorf("%w: %v", context.Canceled, chatErr)
	}
	text := strings.TrimSpace(outcome.Text)
	stored := map[string]any{"tool_calls": outcome.ToolCalls}
	// No answer, or one cut short, always comes with an explanation.
	if chatErr != nil || text == "" {
		if chatErr != nil {
			slog.Warn("assistant chat failed", "provider", providerID, "error", chatErr)
		}
		why := explainFailure(providerID, chatReq.Model, chatErr, outcome)
		if text != "" && !errors.Is(chatErr, context.Canceled) {
			why.Message = "A resposta foi interrompida. " + why.Message
		}
		emit(Event{Type: "error", Error: why.Message, Detail: why.Detail})
		stored["error"], stored["error_detail"] = why.Message, why.Detail
	}
	data, _ := json.Marshal(stored)
	// Store even on failure so the history shows what happened; a fresh
	// context so a disconnected browser doesn't lose the answer.
	saveCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, err := c.repo.AddMessage(saveCtx, postgres.ConversationMessage{ConversationID: conv.ID, Role: "assistant", Content: text, Data: data, Provider: providerID})
	if err != nil {
		return err
	}
	session := outcome.SessionID
	if session == "" && chatErr == nil {
		session = chatReq.SessionID
	}
	_ = c.repo.TouchConversation(saveCtx, conv.ID, providerID, session)
	emit(Event{Type: "done", ConversationID: conv.ID, MessageID: saved.ID, Provider: providerID})
	return nil
}

var moduleNames = map[string]string{
	"financeiro": "finanças (gastos, categorias, cartões)",
	"contas":     "contas a pagar e a receber",
	"notas":      "notas",
	"lembretes":  "lembretes",
	"agenda":     "agenda",
	"habitos":    "hábitos",
	"treino":     "treino",
	"dieta":      "dieta",
	"documentos": "documentos",
	"quadros":    "quadros",
}

var weekdaysPT = []string{"domingo", "segunda-feira", "terça-feira", "quarta-feira", "quinta-feira", "sexta-feira", "sábado"}

func (c *Chat) systemPrompt(module string, sentAt time.Time) string {
	now := c.tools.now()
	var b strings.Builder
	b.WriteString("Você é o assistente do Estus Brain, o app pessoal do dono (finanças, contas, notas, lembretes, agenda, hábitos, treino, dieta, documentos e quadros). ")
	b.WriteString("Responda sempre em português do Brasil, de forma direta e curta; use listas curtas quando ajudar e Markdown simples.\n")
	fmt.Fprintf(&b, "Agora: %s, %s às %s (horário de São Paulo).\n", weekdaysPT[now.Weekday()], now.Format("2006-01-02"), now.Format("15:04"))
	b.WriteString(sendTimeLine(sentAt, now))
	b.WriteString("Você pode conversar sobre qualquer assunto. Quando a pergunta envolver os dados da pessoa, use as ferramentas do Estus Brain em vez de supor. ")
	b.WriteString("Para lançar, cadastrar ou editar, faça direto quando as informações estiverem claras; se faltar algo essencial (valor, data), pergunte. ")
	b.WriteString("Gasto sem categoria dita: escolha a categoria existente que melhor encaixa (liste antes); só quando nenhuma serve, passe um nome novo e curto — a categoria é criada sozinha. Avise na resposta quando criar uma categoria nova. ")
	b.WriteString("Se não entender o pedido, diga que não entendeu e o que ficou confuso, em vez de chutar. ")
	// The model is given only some modules' tools when it is small. Asked for
	// a habit while holding only the finance tools, one booked an expense and
	// then said the habit had been created — the wrong action AND a false
	// report. Routing makes that rare; this makes it honest when it happens.
	b.WriteString("Se não existir ferramenta para o que foi pedido, diga que não consegue fazer isso por aqui e onde a pessoa consegue. ")
	b.WriteString("Nunca use uma ferramenta de outro assunto como aproximação — lançar um gasto não é criar um hábito. ")
	b.WriteString("Nunca diga que fez algo sem ter chamado a ferramenta que faz aquilo. ")
	b.WriteString("Se uma ferramenta devolver erro, explique em palavras simples por que não deu certo e o que a pessoa precisa informar ou corrigir (ex.: categoria que não existe: mostre as que existem). Nunca termine sem responder. ")
	b.WriteString("Antes de excluir qualquer coisa, confirme com a pessoa. Nunca invente ids: liste antes. Valores em R$ no formato brasileiro. ")
	b.WriteString("Não fale sobre senhas: o cofre de senhas não está disponível para você.")
	if name, ok := moduleNames[module]; ok {
		fmt.Fprintf(&b, "\nA pessoa está no módulo de %s: priorize esse assunto.", name)
	}
	return b.String()
}

// runTool executes a tool for an API-based engine, streaming its start and
// result, and returns the text handed back to the model.
func (c *Chat) runTool(ctx context.Context, id, name string, args json.RawMessage, emit func(Event), calls *[]ToolCall) string {
	if len(args) == 0 {
		args = json.RawMessage(`{}`)
	}
	emit(Event{Type: "tool", ToolID: id, Tool: name, Args: args})
	call := ToolCall{ID: id, Name: name, Args: args}
	result, err := c.tools.Call(ctx, name, args)
	var text string
	if err != nil {
		call.Error = ToolErrorMessage(err)
		text = `{"erro": ` + quote(call.Error) + `}`
		emit(Event{Type: "tool_result", ToolID: id, Tool: name, Error: call.Error})
	} else {
		call.Result = result
		b, _ := json.Marshal(result)
		text = string(b)
		emit(Event{Type: "tool_result", ToolID: id, Tool: name, Result: result})
	}
	*calls = append(*calls, call)
	return truncate(text, 20000)
}

func quote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// toolsFor picks the tools an engine is given: all of them, or for small
// on-device models only the selected module's plus the day overview.
// generate_image/edit_image are offered to every engine — the tool itself
// picks a real backend (OpenAI, else Draw Things locally) regardless of
// which engine called it, so there is no provider to gate this on.
func (c *Chat) toolsFor(module, message string, small bool) []Tool {
	if !small {
		return c.tools.Tools()
	}
	// A module chosen on screen is a stronger signal than anything in the
	// text. Without one, the message itself decides — see routing.go, which
	// exists because "crie um hábito" used to reach a model holding only the
	// finance tools.
	mods := []string{"geral"}
	if module != "" {
		mods = append(mods, module)
	} else {
		mods = modulesFor(message)
	}
	return c.tools.Tools(mods...)
}
