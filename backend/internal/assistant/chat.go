package assistant

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
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

// ---------------------------------------------------------------- providers

type Message struct {
	Role    string `json:"role"` // user | assistant
	Content string `json:"content"`
}

type ChatRequest struct {
	System  string
	History []Message
	Message string
	// Attachment carries an image the agent gets as real pixels. Anything
	// else the person attached is already folded into Message as text by
	// Chat.Send.
	Attachment *Attachment
}

type ChatOutcome struct {
	Text string
}

type ProviderStatus struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Available bool     `json:"available"`
	Detail    string   `json:"detail"`
	Model     string   `json:"model"`
	Models    []string `json:"models,omitempty"`
}

// ---------------------------------------------------------------- chat

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
	// Not the shared httpClient: its Timeout also covers reading the streamed
	// body, so a long answer would be cut off. The request's context (the
	// browser) bounds the call instead.
	c.agent = &agentProvider{config: c.agentConfig, client: &http.Client{}}
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
	return mergeAgentConfig(c.savedAgentConfig(s), c.cfg.Env), nil
}

func (c *Chat) savedAgentConfig(s postgres.AssistantSettings) agentConfig {
	return agentConfig{URL: s.AgentURL, Token: openSecret(c.cfg.VaultKey, s.Secrets[agentTokenSecret]), Model: s.AgentModel}
}

// mergeAgentConfig fills each field the settings screen left empty from env.
func mergeAgentConfig(saved, env agentConfig) agentConfig {
	if saved.URL == "" {
		saved.URL = env.URL
	}
	if saved.Token == "" {
		saved.Token = env.Token
	}
	if saved.Model == "" {
		saved.Model = env.Model
	}
	return saved
}

// sealSecret encrypts a stored secret as base64(nonce|ciphertext).
func sealSecret(key [32]byte, value string) (string, error) {
	ct, nonce, err := domain.EncryptPassword(key, value)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(append(nonce, ct...)), nil
}

// openSecret reverses sealSecret; anything unreadable (no key, another key,
// a damaged blob) reads as no secret.
func openSecret(key *[32]byte, blob string) string {
	if blob == "" || key == nil {
		return ""
	}
	raw, err := base64.StdEncoding.DecodeString(blob)
	if err != nil || len(raw) <= 12 {
		return ""
	}
	value, err := domain.DecryptPassword(*key, raw[12:], raw[:12])
	if err != nil {
		return ""
	}
	return value
}

// normalizeAgentURL trims the address the owner typed; empty is allowed and
// means "use AGENT_URL".
func normalizeAgentURL(raw string) (string, error) {
	s := strings.TrimRight(strings.TrimSpace(raw), "/")
	if s == "" {
		return "", nil
	}
	u, err := url.Parse(s)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", fmt.Errorf("%w: o endereço do agente precisa começar com http:// ou https:// (ex.: http://127.0.0.1:8642)", domain.ErrValidation)
	}
	return s, nil
}

// agentHostChanged says whether two agent addresses point at different
// servers (host and port), so a token meant for one isn't sent to the other.
func agentHostChanged(oldURL, newURL string) bool {
	host := func(s string) string {
		if u, err := url.Parse(s); err == nil {
			return u.Host
		}
		return s
	}
	return host(oldURL) != host(newURL)
}

// ---- settings

type SettingsView struct {
	Provider  string           `json:"provider"` // agent | none
	Providers []ProviderStatus `json:"providers"`
	// Agent keeps what the settings screen saved apart from what .env gives,
	// so the form edits only the saved values and never copies .env into
	// the database.
	Agent struct {
		URL      string `json:"url"`
		Model    string `json:"model"`
		HasToken bool   `json:"has_token"`
		EnvURL   string `json:"env_url"`
		EnvModel string `json:"env_model"`
		EnvToken bool   `json:"env_token"`
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
	view.Agent.URL, view.Agent.Model, view.Agent.HasToken = s.AgentURL, s.AgentModel, s.Secrets[agentTokenSecret] != ""
	view.Agent.EnvURL, view.Agent.EnvModel, view.Agent.EnvToken = c.cfg.Env.URL, c.cfg.Env.Model, c.cfg.Env.Token != ""
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
		next, err := normalizeAgentURL(*u.AgentURL)
		if err != nil {
			return err
		}
		// A saved token belongs to the server it was saved for; moving to
		// another one without a new token drops it (AGENT_TOKEN still applies).
		before := mergeAgentConfig(agentConfig{URL: s.AgentURL}, c.cfg.Env).URL
		after := mergeAgentConfig(agentConfig{URL: next}, c.cfg.Env).URL
		if u.AgentToken == nil && agentHostChanged(before, after) {
			delete(s.Secrets, agentTokenSecret)
		}
		s.AgentURL = next
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
			blob, err := sealSecret(*c.cfg.VaultKey, token)
			if err != nil {
				return err
			}
			s.Secrets[agentTokenSecret] = blob
		}
	}
	return c.repo.SaveSettings(ctx, s)
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
	// AttachmentID is a document already uploaded (POST /api/assistant/attachments)
	// that this message refers to — an image, PDF, spreadsheet or text file.
	AttachmentID string `json:"attachment_id"`
	// Title names a conversation Send creates; empty uses the message's start.
	Title string `json:"-"`
}

var ErrNoProvider = errors.New("no AI engine selected")

// Send answers a message with the agent, streaming events, and keeps
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
	if settings.Provider != agentID {
		return ErrNoProvider
	}
	providerID := agentID

	var attachment *Attachment
	if req.AttachmentID != "" {
		attachment, err = resolveAttachment(ctx, c.cfg.Documents, req.AttachmentID)
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

	chatReq := ChatRequest{System: c.systemPrompt(req.Module)}
	chatReq.History, chatReq.Message = buildHistory(history, message)
	// An image goes to the agent as itself on ChatRequest.Attachment; every
	// other attachment is already text by now, so it's just more of the
	// message.
	if attachment != nil {
		if attachment.ImageBase64 != "" {
			chatReq.Attachment = attachment
		} else if attachment.Text != "" {
			chatReq.Message = fmt.Sprintf("[Anexo: %s]\n%s\n\n%s", attachment.Name, attachment.Text, chatReq.Message)
		}
	}

	outcome, chatErr := c.agent.Chat(ctx, chatReq, emit)
	if chatErr != nil && ctx.Err() != nil {
		// The person stopped the answer (or left); say that, not what the
		// engine made of being cut off.
		chatErr = fmt.Errorf("%w: %v", context.Canceled, chatErr)
	}
	text := strings.TrimSpace(outcome.Text)
	stored := map[string]any{}
	// No answer, or one cut short, always comes with an explanation.
	if chatErr != nil || text == "" {
		if chatErr != nil {
			slog.Warn("assistant chat failed", "provider", providerID, "error", chatErr)
		}
		why := explainFailure(chatErr)
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
	_ = c.repo.TouchConversation(saveCtx, conv.ID, providerID)
	emit(Event{Type: "done", ConversationID: conv.ID, MessageID: saved.ID, Provider: providerID})
	return nil
}

// buildHistory turns stored messages into turns that alternate and start
// with the person, as chat APIs want: blank messages are skipped, consecutive
// messages of one side (a flow can add several) merge, a leading assistant
// text is dropped, and an unanswered last user turn joins the new message.
func buildHistory(msgs []postgres.ConversationMessage, message string) ([]Message, string) {
	var history []Message
	for _, m := range msgs {
		if strings.TrimSpace(m.Content) == "" {
			continue
		}
		n := len(history)
		switch {
		case n == 0 && m.Role != "user":
			continue
		case n > 0 && history[n-1].Role == m.Role:
			history[n-1].Content += "\n\n" + m.Content
		default:
			history = append(history, Message{Role: m.Role, Content: m.Content})
		}
	}
	if n := len(history); n > 0 && history[n-1].Role == "user" {
		message = history[n-1].Content + "\n\n" + message
		history = history[:n-1]
	}
	return history, message
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

func (c *Chat) systemPrompt(module string) string {
	now := c.tools.now()
	var b strings.Builder
	b.WriteString("Você é o assistente do Estus Brain, o app pessoal do dono (finanças, contas, notas, lembretes, agenda, hábitos, treino, dieta, documentos e quadros). ")
	b.WriteString("Responda sempre em português do Brasil, de forma direta e curta; use listas curtas quando ajudar e Markdown simples.\n")
	fmt.Fprintf(&b, "Agora: %s, %s às %s (horário de São Paulo).\n", weekdaysPT[now.Weekday()], now.Format("2006-01-02"), now.Format("15:04"))
	b.WriteString("Você pode conversar sobre qualquer assunto. Quando a pergunta envolver os dados da pessoa, use as ferramentas do Estus Brain em vez de supor. ")
	b.WriteString("Para lançar, cadastrar ou editar, faça direto quando as informações estiverem claras; se faltar algo essencial (valor, data), pergunte. ")
	b.WriteString("Gasto sem categoria dita: escolha a categoria existente que melhor encaixa (liste antes); só quando nenhuma serve, passe um nome novo e curto — a categoria é criada sozinha. Avise na resposta quando criar uma categoria nova. ")
	b.WriteString("Se não entender o pedido, diga que não entendeu e o que ficou confuso, em vez de chutar. ")
	b.WriteString("Nunca diga que fez algo sem ter chamado a ferramenta que faz aquilo. ")
	b.WriteString("Se uma ferramenta devolver erro, explique em palavras simples por que não deu certo e o que a pessoa precisa informar ou corrigir (ex.: categoria que não existe: mostre as que existem). Nunca termine sem responder. ")
	b.WriteString("Antes de excluir qualquer coisa, confirme com a pessoa. Nunca invente ids: liste antes. Valores em R$ no formato brasileiro. ")
	b.WriteString(appRules + "\n")
	b.WriteString("Não fale sobre senhas: o cofre de senhas não está disponível para você.")
	if name, ok := moduleNames[module]; ok {
		fmt.Fprintf(&b, "\nA pessoa está no módulo de %s: priorize esse assunto.", name)
	}
	return b.String()
}
