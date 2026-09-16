// Package telegram connects Estus Brain to one Telegram bot: the owner chats
// with the assistant there, and the bot sends the daily reports and the
// reminder and event alerts on its own. It long-polls, so it needs no public
// address and runs the same on a laptop or a server.
package telegram

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"

	"github.com/rafael/estus-vault/backend/internal/assistant"
	"github.com/rafael/estus-vault/backend/internal/domain"
	"github.com/rafael/estus-vault/backend/internal/store/postgres"
)

// API is the part of the Bot API the bot uses; *Client implements it.
type API interface {
	GetMe(ctx context.Context) (User, error)
	GetUpdates(ctx context.Context, offset int64, timeout int) ([]Update, error)
	SendMessage(ctx context.Context, chatID int64, text string, html bool, buttons []Button) (Message, error)
	SendKeyboard(ctx context.Context, chatID int64, text string, rows [][]Button) (Message, error)
	EditKeyboard(ctx context.Context, chatID, messageID int64, text string, rows [][]Button) error
	SendChatAction(ctx context.Context, chatID int64) error
	AnswerCallbackQuery(ctx context.Context, id, text string) error
	EditMessageText(ctx context.Context, chatID, messageID int64, text string, html bool) error
	GetFile(ctx context.Context, fileID string) (File, error)
	DownloadFile(ctx context.Context, filePath string) ([]byte, error)
}

// Store is where the bot keeps its settings; *postgres.TelegramRepo implements it.
type Store interface {
	Settings(ctx context.Context) (postgres.TelegramSettings, error)
	SaveSettings(ctx context.Context, s postgres.TelegramSettings) error
	// SaveSettingsNewBot also sets the update offset back to 0, in the same write.
	SaveSettingsNewBot(ctx context.Context, s postgres.TelegramSettings) error
	SetOffset(ctx context.Context, offset int64) error
	SetOnlineAt(ctx context.Context, t time.Time) error
	SetReportDay(ctx context.Context, kind string, day *time.Time) error
	MarkSent(ctx context.Context, kind, ref string) (bool, error)
	UnmarkSent(ctx context.Context, kind, ref string) error
	PruneSent(ctx context.Context, before time.Time) error
}

// Chatter answers a message with the chosen AI engine; *assistant.Chat implements it.
type Chatter interface {
	Send(ctx context.Context, req assistant.SendRequest, emit func(assistant.Event)) error
}

// Data is the owner's data the reports and alerts read.
type Data interface {
	Snapshot(ctx context.Context, now time.Time) Snapshot
	Reminders(ctx context.Context) ([]domain.Reminder, error)
	Events(ctx context.Context, from, to time.Time) ([]domain.Event, error)
	CompleteReminder(ctx context.Context, id string) (domain.Reminder, error)
}

// ToolBox is the assistant's tool registry, behind the ready-made actions;
// *assistant.Registry implements it.
type ToolBox interface {
	Tools(modules ...string) []assistant.Tool
	Call(ctx context.Context, name string, raw json.RawMessage) (any, error)
}

// Transcriber turns a voice message into text; *assistant.Voice implements it.
type Transcriber interface {
	Available(ctx context.Context) bool
	Transcribe(ctx context.Context, audio []byte, filename string) (assistant.Transcript, error)
}

type Config struct {
	Store Store
	Chat  Chatter
	Data  Data
	// Voice transcribes voice messages; nil means the bot only reads text.
	Voice Transcriber
	// Tools backs the ready-made actions menu; nil leaves /acoes out.
	Tools    ToolBox
	Location *time.Location
	// VaultKey encrypts the stored token; without it only EnvToken works.
	VaultKey *[32]byte
	EnvToken string
	// NewAPI builds the Bot API client for a token; tests swap in a fake.
	NewAPI func(token string) API
	Now    func() time.Time
}

// Status is what the settings screen shows about the bot.
type Status struct {
	State   string `json:"state"` // off | unpaired | ok | error
	Message string `json:"message"`
}

const (
	msgNoToken  = "Sem token do bot."
	msgUnpaired = "Esperando o pareamento."
	msgOK       = "Funcionando."
	msgBadToken = "O Telegram recusou o token — gere outro no @BotFather."
	msgOffline  = "Sem conexão com o Telegram. Tentando de novo."
	msgConflict = "Outro Estus está usando este bot — desligue um deles."
	msgBlocked  = "O bot foi bloqueado no Telegram."
	// The vault key changed, or the stored secret got corrupted.
	msgUnreadableToken = "O token salvo não pôde ser lido (a VAULT_ENCRYPTION_KEY mudou?) — salve o token de novo."
)

type Bot struct {
	cfg    Config
	reload chan struct{}
	asks   chan askJob // messages waiting for the assistant

	mu          sync.Mutex // serializes settings changes; guards the fields below
	attempts    int        // wrong pairing tries against the code in force
	status      Status
	statusToken string // the token status was worked out for
	cancelPoll  context.CancelFunc
	generation  uint64      // bumped by every reload, so a poll can tell it's stale
	offset      int64       // the next update id, also kept here in case saving it fails
	online      bool        // the owner has been told the bot is answering
	pending     *menuAction // the ready-made action waiting for answers

	apiMu    sync.Mutex
	api      API
	apiToken string

	lastPrune time.Time // touched only by the scheduler goroutine
}

func New(cfg Config) *Bot {
	if cfg.NewAPI == nil {
		cfg.NewAPI = func(token string) API { return NewClient(DefaultBaseURL, token) }
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Bot{cfg: cfg, reload: make(chan struct{}, 1), asks: make(chan askJob, askQueue), status: Status{State: "off", Message: msgNoToken}}
}

func (b *Bot) now() time.Time { return b.cfg.Now().In(b.cfg.Location) }

// token is the bot token in use: the stored one, else the environment's. A
// stored token that can't be decrypted gives "", not the environment's: the
// owner picked that bot, and quietly running another would hide the problem.
func (b *Bot) token(s postgres.TelegramSettings) (token string, fromEnv bool) {
	if s.TokenSecret != "" && b.cfg.VaultKey != nil {
		raw, err := base64.StdEncoding.DecodeString(s.TokenSecret)
		if err == nil && len(raw) > 12 {
			if t, err := domain.DecryptPassword(*b.cfg.VaultKey, raw[12:], raw[:12]); err == nil {
				return t, false
			}
		}
		return "", false
	}
	return b.cfg.EnvToken, b.cfg.EnvToken != ""
}

// tokenUnreadable reports a stored token that token couldn't decrypt.
func (b *Bot) tokenUnreadable(s postgres.TelegramSettings) bool {
	token, _ := b.token(s)
	return token == "" && s.TokenSecret != "" && b.cfg.VaultKey != nil
}

// client is the Bot API for token, reused while the token stays the same.
func (b *Bot) client(token string) API {
	b.apiMu.Lock()
	defer b.apiMu.Unlock()
	if b.api == nil || b.apiToken != token {
		b.api, b.apiToken = b.cfg.NewAPI(token), token
	}
	return b.api
}

func (b *Bot) setStatus(state, message string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.status = Status{State: state, Message: message}
}

// noteSendError keeps a blocked bot visible on the settings screen.
func (b *Bot) noteSendError(err error) {
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.Code == 403 {
		b.setStatus("error", msgBlocked)
	}
}

// noteSendOK clears a blocked bot once a message gets through again.
func (b *Bot) noteSendOK() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.status.Message == msgBlocked {
		b.status = Status{State: "ok", Message: msgOK}
	}
}

// Reload makes the loops pick up a settings change now.
func (b *Bot) Reload() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.reloadLocked()
}

func (b *Bot) reloadLocked() {
	b.generation++
	if b.cancelPoll != nil {
		b.cancelPoll()
	}
	select {
	case b.reload <- struct{}{}:
	default:
	}
}

// saveContext outlives ctx for a few seconds, for the writes that keep the
// bot's records true when a shutdown cuts a send short.
func saveContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
}

// logPanic, deferred, keeps a panic in one update, answer or tick from taking
// the whole API down; the loop around it carries on.
func logPanic(where string) {
	if v := recover(); v != nil {
		slog.Error("telegram: panic recovered", "in", where, "panic", v, "stack", string(debug.Stack()))
	}
}

func invalid(msg string) error { return fmt.Errorf("%w: %s", domain.ErrValidation, msg) }

// forgetOwner drops the pairing, for a new bot or an explicit unpair.
func forgetOwner(s *postgres.TelegramSettings) {
	s.ChatID, s.OwnerName, s.ConversationID = nil, "", nil
	s.PairingCode, s.PairingExpiresAt = "", nil
}
