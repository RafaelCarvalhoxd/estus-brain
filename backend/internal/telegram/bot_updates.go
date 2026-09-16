package telegram

import (
	"context"
	"errors"
	"log/slog"
	"path"
	"strings"
	"time"

	"github.com/rafael/estus-vault/backend/internal/assistant"
	"github.com/rafael/estus-vault/backend/internal/domain"
	"github.com/rafael/estus-vault/backend/internal/store/postgres"
)

// Telegram caps a message at 4096 characters; the rest is room for the tags
// a split closes and reopens.
const messageLimit = 4000

const helpText = "Sou o Estus Brain. Escreva o que precisa — \"quanto gastei hoje?\", \"me lembra de pagar a luz amanhã às 9\" — ou use:\n" +
	"/acoes — ações prontas em botões (sem precisar de IA)\n" +
	"/hoje — resumo do dia\n" +
	"/noite — fechamento do dia\n" +
	"/nova — começar uma conversa nova\n" +
	"/ajuda — esta lista"

// askQueue is how many messages can wait for the assistant at once.
const askQueue = 32

// askJob is a message for the assistant, answered by answerLoop: typed text,
// or a voice note still to be downloaded and transcribed.
type askJob struct {
	api    API
	chatID int64
	text   string
	voice  *voiceNote
	// sentAt is when the owner wrote the message, which can be hours before
	// it was handed over; zero when the update carried no date.
	sentAt time.Time
}

// voiceNote is a voice message or audio file waiting in line.
type voiceNote struct {
	fileID   string
	filename string // fallback name when Telegram's path has no extension
}

// maxVoiceSeconds is the longest voice message the bot will transcribe.
const maxVoiceSeconds = 180

// handle processes one update. Only private chats count; once paired, only
// the owner's. Everything but a question for the assistant is dealt with
// right here, so buttons and commands don't wait behind a slow answer.
func (b *Bot) handle(ctx context.Context, api API, u Update) {
	defer logPanic("update")
	s, err := b.cfg.Store.Settings(ctx)
	if err != nil {
		slog.Warn("telegram: read settings", "error", err)
		return
	}
	if cq := u.CallbackQuery; cq != nil {
		if s.ChatID != nil && cq.Message != nil && cq.Message.Chat.ID == *s.ChatID && b.handleMenuCallback(ctx, api, *s.ChatID, cq) {
			return
		}
		b.handleCallback(ctx, api, s, cq)
		return
	}
	m := u.Message
	if m == nil || m.Chat.Type != "private" {
		return
	}
	if s.ChatID == nil {
		b.tryPair(ctx, api, m)
		return
	}
	if m.Chat.ID != *s.ChatID {
		slog.Info("telegram: ignored a message from another chat", "chat_id", m.Chat.ID)
		return
	}
	chatID, sentAt := *s.ChatID, sentTime(m)
	if note, seconds := voiceIn(m); note != nil {
		switch {
		case b.cfg.Voice == nil:
			_ = b.sendHTML(ctx, api, chatID, "Por enquanto só entendo texto aqui.", nil)
		case seconds > maxVoiceSeconds:
			_ = b.sendHTML(ctx, api, chatID, "Áudio longo demais — mande até 3 minutos.", nil)
		default:
			// Downloading and transcribing take a moment: the worker does it.
			b.enqueue(ctx, askJob{api: api, chatID: chatID, voice: note, sentAt: sentAt})
		}
		return
	}
	text := strings.TrimSpace(m.Text)
	if text == "" {
		_ = b.sendHTML(ctx, api, chatID, "Por enquanto eu só entendo texto.", nil)
		return
	}
	switch command(text) {
	case "/acoes", "/atalhos":
		b.openMenu(ctx, api, chatID)
	case "/cancelar":
		if b.action() == nil {
			_ = b.sendHTML(ctx, api, chatID, "Não tem nada em andamento.", nil)
			return
		}
		b.setAction(nil)
		_ = b.sendHTML(ctx, api, chatID, "Cancelei.", nil)
	case "/hoje":
		_ = b.sendLong(ctx, api, chatID, MorningReport(b.cfg.Data.Snapshot(ctx, b.now())))
	case "/noite":
		_ = b.sendLong(ctx, api, chatID, EveningReport(b.cfg.Data.Snapshot(ctx, b.now())))
	case "/nova":
		if err := b.setConversation(ctx, nil); err != nil {
			slog.Warn("telegram: reset conversation", "error", err)
		}
		_ = b.sendHTML(ctx, api, chatID, "Pronto: a próxima mensagem começa uma conversa nova.", nil)
	case "/ajuda", "/start":
		_ = b.sendHTML(ctx, api, chatID, esc(helpText), nil)
	default:
		// A ready-made action waiting for details takes the answer first.
		if b.menuAnswer(ctx, api, chatID, text) {
			return
		}
		b.enqueue(ctx, askJob{api: api, chatID: chatID, text: text, sentAt: sentAt})
	}
}

// sentTime is when the message was written, or the zero time when the update
// carried no date (a synthetic update in a test, or an older Bot API shape).
func sentTime(m *Message) time.Time {
	if m.Date == 0 {
		return time.Time{}
	}
	return time.Unix(m.Date, 0)
}

// enqueue puts a message in line for the assistant, or says the line is full.
func (b *Bot) enqueue(ctx context.Context, job askJob) {
	select {
	case b.asks <- job:
	default:
		_ = b.sendHTML(ctx, job.api, job.chatID, "Estou com muitas mensagens na fila; tente de novo em instantes.", nil)
	}
}

// voiceIn is the voice note or audio file in a message, with its length in seconds.
func voiceIn(m *Message) (*voiceNote, int) {
	switch {
	case m.Voice != nil:
		return &voiceNote{fileID: m.Voice.FileID, filename: "audio.ogg"}, m.Voice.Duration
	case m.Audio != nil:
		name := "audio" + path.Ext(m.Audio.FileName)
		if name == "audio" {
			name = "audio.ogg"
		}
		return &voiceNote{fileID: m.Audio.FileID, filename: name}, m.Audio.Duration
	}
	return nil, 0
}

// answerLoop answers the queued messages one at a time, in the order they came.
func (b *Bot) answerLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-b.asks:
			b.answer(ctx, job)
		}
	}
}

func (b *Bot) answer(ctx context.Context, job askJob) {
	defer logPanic("answer")
	s, err := b.cfg.Store.Settings(ctx)
	if err != nil {
		slog.Warn("telegram: read settings", "error", err)
		return
	}
	if s.ChatID == nil || *s.ChatID != job.chatID {
		return // unpaired while the message waited in line
	}
	stop := b.typing(ctx, job.api, job.chatID)
	defer stop()
	text := job.text
	if job.voice != nil {
		heard, failure := b.listen(ctx, job)
		if failure != "" {
			stop()
			_ = b.sendHTML(ctx, job.api, job.chatID, esc(failure), nil)
			return
		}
		_ = b.sendHTML(ctx, job.api, job.chatID, "🎙️ Entendi: "+esc(heard), nil)
		// A spoken answer to a ready-made action's question stays with it.
		if b.menuAnswer(ctx, job.api, job.chatID, heard) {
			return
		}
		text = heard
	}
	reply := b.ask(ctx, s, text, job.sentAt)
	stop()
	_ = b.sendLong(ctx, job.api, job.chatID, ToHTML(reply))
}

// listen downloads and transcribes a voice note. It returns what was heard,
// or the message to send instead when that didn't work.
func (b *Bot) listen(ctx context.Context, job askJob) (heard, failure string) {
	const cantHear = "Não consegui ouvir o áudio agora. Tente de novo ou mande em texto."
	if b.cfg.Voice == nil || !b.cfg.Voice.Available(ctx) {
		return "", "Por enquanto só entendo texto aqui."
	}
	file, err := job.api.GetFile(ctx, job.voice.fileID)
	var audio []byte
	if err == nil {
		audio, err = job.api.DownloadFile(ctx, file.FilePath)
	}
	if err != nil {
		slog.Warn("telegram: download voice message", "error", err)
		return "", cantHear
	}
	name := job.voice.filename
	if ext := path.Ext(file.FilePath); ext != "" {
		name = "audio" + ext
	}
	t, err := b.cfg.Voice.Transcribe(ctx, audio, name)
	switch {
	case errors.Is(err, assistant.ErrVoiceRejected):
		return "", assistant.VoiceErrorMessage(err)
	case err != nil:
		slog.Warn("telegram: transcribe voice message", "error", err)
		return "", cantHear
	case t.Text == "":
		return "", "Não entendi o áudio — pode repetir?"
	}
	slog.Info("telegram: voice message transcribed", "audio_seconds", t.Seconds)
	return t.Text, ""
}

// command is a message's leading /command, without any @botname.
func command(text string) string {
	if !strings.HasPrefix(text, "/") {
		return ""
	}
	cmd := strings.Fields(text)[0]
	if i := strings.Index(cmd, "@"); i > 0 {
		cmd = cmd[:i]
	}
	return strings.ToLower(cmd)
}

// tryPair handles a private message while nobody is paired.
func (b *Bot) tryPair(ctx context.Context, api API, m *Message) {
	b.mu.Lock()
	s, err := b.cfg.Store.Settings(ctx)
	if err != nil {
		b.mu.Unlock()
		slog.Warn("telegram: read settings", "error", err)
		return
	}
	if pairingMatches(s.PairingCode, s.PairingExpiresAt, b.now(), m.Text) {
		chatID := m.Chat.ID
		s.ChatID = &chatID
		if m.From != nil {
			s.OwnerName = m.From.FirstName
		}
		s.PairingCode, s.PairingExpiresAt, s.ConversationID = "", nil, nil
		b.attempts = 0
		err := b.cfg.Store.SaveSettings(ctx, s)
		b.mu.Unlock()
		if err != nil {
			slog.Warn("telegram: save pairing", "error", err)
			_ = b.sendHTML(ctx, api, chatID, "Não consegui salvar o pareamento. Tente de novo.", nil)
			return
		}
		b.setStatus("ok", msgOK)
		_ = b.sendHTML(ctx, api, chatID, "Pareado ✅ — mande /ajuda para ver o que eu sei fazer.", nil)
		return
	}
	if s.PairingCode != "" {
		b.attempts++
		if b.attempts >= maxPairingAttempts {
			s.PairingCode, s.PairingExpiresAt = "", nil
			b.attempts = 0
			if err := b.cfg.Store.SaveSettings(ctx, s); err != nil {
				slog.Warn("telegram: drop pairing code", "error", err)
			}
		}
	}
	b.mu.Unlock()
	_ = b.sendHTML(ctx, api, m.Chat.ID, "Código inválido ou expirado. Gere outro na tela de configuração do Estus.", nil)
}

// ask runs a message through the assistant and returns the Markdown to send
// back: the answer, an explanation of what went wrong, or both.
func (b *Bot) ask(ctx context.Context, s postgres.TelegramSettings, text string, sentAt time.Time) string {
	conversation := ""
	if s.ConversationID != nil {
		conversation = *s.ConversationID
	}
	var answer strings.Builder
	var failure, started string
	send := func(conversation string) error {
		answer.Reset()
		failure, started = "", ""
		req := assistant.SendRequest{ConversationID: conversation, Message: text, SentAt: sentAt, Title: "Telegram · " + b.now().Format("02/01")}
		return b.cfg.Chat.Send(ctx, req, func(e assistant.Event) {
			switch e.Type {
			case "conversation":
				started = e.ConversationID
			case "text":
				answer.WriteString(e.Text)
			case "error":
				failure = e.Error
			}
		})
	}
	err := send(conversation)
	if errors.Is(err, domain.ErrNotFound) && conversation != "" {
		// The conversation was deleted on the web: start a new one.
		conversation = ""
		err = send("")
	}
	if started != "" && started != conversation {
		if err := b.setConversation(ctx, &started); err != nil {
			slog.Warn("telegram: save conversation", "error", err)
		}
	}
	if err != nil {
		failure = assistant.SendErrorMessage(err)
		if errors.Is(err, assistant.ErrNoProvider) {
			failure += " Os comandos /hoje e /noite funcionam sem IA."
		}
	}
	reply := strings.TrimSpace(answer.String())
	if failure != "" {
		if reply != "" {
			reply += "\n\n"
		}
		reply += "⚠️ " + failure
	}
	if reply == "" {
		reply = "Não consegui responder agora. Tente de novo."
	}
	return reply
}

func (b *Bot) setConversation(ctx context.Context, id *string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	s, err := b.cfg.Store.Settings(ctx)
	if err != nil {
		return err
	}
	s.ConversationID = id
	return b.cfg.Store.SaveSettings(ctx, s)
}

// typing shows "digitando…" until stop is called; Telegram clears the
// indicator after about 5 s, so it is repeated.
func (b *Bot) typing(ctx context.Context, api API, chatID int64) (stop func()) {
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(4 * time.Second)
		defer ticker.Stop()
		for {
			_ = api.SendChatAction(ctx, chatID)
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return func() {
		cancel()
		<-done
	}
}

func (b *Bot) handleCallback(ctx context.Context, api API, s postgres.TelegramSettings, cq *CallbackQuery) {
	id, ok := strings.CutPrefix(cq.Data, "done:")
	if s.ChatID == nil || cq.Message == nil || cq.Message.Chat.ID != *s.ChatID || !ok {
		_ = api.AnswerCallbackQuery(ctx, cq.ID, "")
		return
	}
	r, err := b.cfg.Data.CompleteReminder(ctx, id)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		_ = api.AnswerCallbackQuery(ctx, cq.ID, "Esse lembrete não existe mais.")
	case err != nil:
		slog.Warn("telegram: complete reminder", "error", err)
		_ = api.AnswerCallbackQuery(ctx, cq.ID, "Não consegui concluir agora. Tente de novo.")
	default:
		_ = api.AnswerCallbackQuery(ctx, cq.ID, "Concluído")
		if err := api.EditMessageText(ctx, *s.ChatID, cq.Message.MessageID, ReminderDone(r), true); err != nil {
			slog.Warn("telegram: edit reminder alert", "error", err)
		}
	}
}

// sendLong sends an HTML message in as many parts as it takes.
func (b *Bot) sendLong(ctx context.Context, api API, chatID int64, html string) error {
	for _, part := range Split(html, messageLimit) {
		if err := b.sendHTML(ctx, api, chatID, part, nil); err != nil {
			return err
		}
	}
	return nil
}

// sendHTML sends one message. When Telegram can't parse the HTML it sends
// the same text without formatting, so nothing is lost.
// sendKeyboard sends a menu message; a failure only gets logged, like any
// other send.
func (b *Bot) sendKeyboard(ctx context.Context, api API, chatID int64, text string, rows [][]Button) error {
	_, err := api.SendKeyboard(ctx, chatID, text, rows)
	if err != nil {
		slog.Warn("telegram: send menu", "error", err)
		b.noteSendError(err)
		return err
	}
	b.noteSendOK()
	return nil
}

// editKeyboard replaces a menu in place; if Telegram refuses (an old
// message, say), the menu is sent again instead.
func (b *Bot) editKeyboard(ctx context.Context, api API, chatID, messageID int64, text string, rows [][]Button) error {
	if messageID != 0 {
		if err := api.EditKeyboard(ctx, chatID, messageID, text, rows); err == nil {
			return nil
		}
	}
	return b.sendKeyboard(ctx, api, chatID, text, rows)
}

func (b *Bot) sendHTML(ctx context.Context, api API, chatID int64, text string, buttons []Button) error {
	_, err := api.SendMessage(ctx, chatID, text, true, buttons)
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.Code == 400 && strings.Contains(apiErr.Description, "can't parse entities") {
		_, err = api.SendMessage(ctx, chatID, PlainText(text), false, buttons)
	}
	if err != nil {
		slog.Warn("telegram: send message", "error", err)
		b.noteSendError(err)
		return err
	}
	b.noteSendOK()
	return nil
}
