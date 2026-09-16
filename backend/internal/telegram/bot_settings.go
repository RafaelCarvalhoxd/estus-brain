package telegram

import (
	"context"
	"encoding/base64"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/rafael/estus-vault/backend/internal/domain"
)

// View is the settings screen's picture of the bot. It never carries the token.
type View struct {
	HasToken            bool       `json:"has_token"`
	FromEnv             bool       `json:"from_env"`
	CanStoreToken       bool       `json:"can_store_token"`
	BotUsername         string     `json:"bot_username"`
	Paired              bool       `json:"paired"`
	OwnerName           string     `json:"owner_name"`
	PairingCode         string     `json:"pairing_code,omitempty"`
	PairingExpiresAt    *time.Time `json:"pairing_expires_at,omitempty"`
	MorningEnabled      bool       `json:"morning_enabled"`
	MorningTime         string     `json:"morning_time"`
	EveningEnabled      bool       `json:"evening_enabled"`
	EveningTime         string     `json:"evening_time"`
	RemindersEnabled    bool       `json:"reminders_enabled"`
	EventsEnabled       bool       `json:"events_enabled"`
	EventsMinutesBefore int        `json:"events_minutes_before"`
	Status              Status     `json:"status"`
}

func (b *Bot) View(ctx context.Context) (View, error) {
	s, err := b.cfg.Store.Settings(ctx)
	if err != nil {
		return View{}, err
	}
	token, fromEnv := b.token(s)
	v := View{
		HasToken: token != "", FromEnv: fromEnv, CanStoreToken: b.cfg.VaultKey != nil,
		BotUsername: s.BotUsername, Paired: s.ChatID != nil, OwnerName: s.OwnerName,
		MorningEnabled: s.MorningEnabled, MorningTime: s.MorningTime,
		EveningEnabled: s.EveningEnabled, EveningTime: s.EveningTime,
		RemindersEnabled: s.RemindersEnabled, EventsEnabled: s.EventsEnabled, EventsMinutesBefore: s.EventsMinutesBefore,
	}
	if s.PairingCode != "" && s.PairingExpiresAt != nil && b.now().Before(*s.PairingExpiresAt) {
		v.PairingCode, v.PairingExpiresAt = s.PairingCode, s.PairingExpiresAt
	}
	b.mu.Lock()
	v.Status = b.status
	b.mu.Unlock()
	return v, nil
}

// SettingsInput is a partial settings change; nil fields stay as they are.
type SettingsInput struct {
	Token               *string `json:"token"` // "" removes the stored token
	MorningEnabled      *bool   `json:"morning_enabled"`
	MorningTime         *string `json:"morning_time"`
	EveningEnabled      *bool   `json:"evening_enabled"`
	EveningTime         *string `json:"evening_time"`
	RemindersEnabled    *bool   `json:"reminders_enabled"`
	EventsEnabled       *bool   `json:"events_enabled"`
	EventsMinutesBefore *int    `json:"events_minutes_before"`
}

var (
	// The morning summary is caught up only until noon, so it has to be set
	// before then; the evening one runs until midnight.
	morningTimeRe = regexp.MustCompile(`^(0[4-9]|1[01]):[0-5]\d$`)
	eveningTimeRe = regexp.MustCompile(`^(1[2-9]|2[0-3]):[0-5]\d$`)
)

func (b *Bot) Update(ctx context.Context, in SettingsInput) error {
	if in.MorningTime != nil && !morningTimeRe.MatchString(*in.MorningTime) {
		return invalid("Horário da manhã precisa ficar entre 04:00 e 11:59.")
	}
	if in.EveningTime != nil && !eveningTimeRe.MatchString(*in.EveningTime) {
		return invalid("Horário da noite precisa ficar entre 12:00 e 23:59.")
	}
	if in.EventsMinutesBefore != nil && (*in.EventsMinutesBefore < 1 || *in.EventsMinutesBefore > 1440) {
		return invalid("O aviso de compromisso precisa ser de 1 a 1440 minutos antes.")
	}
	// Telegram can take a while to answer GetMe: check the token before taking
	// the lock the poller and the settings screen share.
	token, me := "", User{}
	if in.Token != nil {
		token = strings.TrimSpace(*in.Token)
		if token != "" {
			if b.cfg.VaultKey == nil {
				return invalid("Defina VAULT_ENCRYPTION_KEY para salvar o token, ou use TELEGRAM_BOT_TOKEN.")
			}
			var err error
			if me, err = b.cfg.NewAPI(token).GetMe(ctx); err != nil {
				var apiErr *APIError
				if errors.As(err, &apiErr) && (apiErr.Code == 401 || apiErr.Code == 404) {
					return invalid("O Telegram recusou o token — confira no @BotFather.")
				}
				return invalid("Não consegui falar com o Telegram agora; tente de novo.")
			}
		}
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	s, err := b.cfg.Store.Settings(ctx)
	if err != nil {
		return err
	}
	if in.MorningTime != nil {
		s.MorningTime = *in.MorningTime
	}
	if in.EveningTime != nil {
		s.EveningTime = *in.EveningTime
	}
	if in.EventsMinutesBefore != nil {
		s.EventsMinutesBefore = *in.EventsMinutesBefore
	}
	for _, f := range []struct {
		in  *bool
		out *bool
	}{{in.MorningEnabled, &s.MorningEnabled}, {in.EveningEnabled, &s.EveningEnabled}, {in.RemindersEnabled, &s.RemindersEnabled}, {in.EventsEnabled, &s.EventsEnabled}} {
		if f.in != nil {
			*f.out = *f.in
		}
	}

	newBot := false
	if in.Token != nil {
		if token == "" {
			s.TokenSecret, s.BotUsername = "", ""
			forgetOwner(&s)
			newBot = true
		} else {
			ciphertext, nonce, err := domain.EncryptPassword(*b.cfg.VaultKey, token)
			if err != nil {
				return err
			}
			if me.Username != s.BotUsername {
				// Another bot: its chats and update ids have nothing to do with the old one.
				forgetOwner(&s)
				newBot = true
			}
			s.TokenSecret = base64.StdEncoding.EncodeToString(append(nonce, ciphertext...))
			s.BotUsername = me.Username
		}
	}
	if newBot {
		// Update ids are per bot, so the new one starts from 0. It's written
		// with the settings, and under the lock the poller takes to save an
		// offset, so the old bot's last batch can't put its offset back.
		if err := b.cfg.Store.SaveSettingsNewBot(ctx, s); err != nil {
			return err
		}
		b.offset = 0
	} else if err := b.cfg.Store.SaveSettings(ctx, s); err != nil {
		return err
	}
	b.reloadLocked()
	return nil
}

// Pairing is a code in force, for the owner to send to the bot.
type Pairing struct {
	Code      string    `json:"code"`
	ExpiresAt time.Time `json:"expires_at"`
	Link      string    `json:"link"`
}

func (b *Bot) StartPairing(ctx context.Context) (Pairing, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	s, err := b.cfg.Store.Settings(ctx)
	if err != nil {
		return Pairing{}, err
	}
	if token, _ := b.token(s); token == "" {
		return Pairing{}, invalid("Salve o token do bot antes de parear.")
	}
	if s.ChatID != nil {
		return Pairing{}, invalid("O bot já está pareado; desparear antes de gerar outro código.")
	}
	code, err := NewPairingCode()
	if err != nil {
		return Pairing{}, err
	}
	expires := b.now().Add(pairingTTL)
	s.PairingCode, s.PairingExpiresAt = code, &expires
	b.attempts = 0
	if err := b.cfg.Store.SaveSettings(ctx, s); err != nil {
		return Pairing{}, err
	}
	p := Pairing{Code: code, ExpiresAt: expires}
	if s.BotUsername != "" {
		p.Link = "https://t.me/" + s.BotUsername + "?start=" + code
	}
	return p, nil
}

func (b *Bot) Unpair(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	s, err := b.cfg.Store.Settings(ctx)
	if err != nil {
		return err
	}
	forgetOwner(&s)
	if err := b.cfg.Store.SaveSettings(ctx, s); err != nil {
		return err
	}
	b.reloadLocked()
	return nil
}

// SendTest proves the whole path works: token, pairing and delivery.
func (b *Bot) SendTest(ctx context.Context) error {
	s, err := b.cfg.Store.Settings(ctx)
	if err != nil {
		return err
	}
	token, _ := b.token(s)
	if token == "" || s.ChatID == nil {
		return invalid("Pareie o bot antes de mandar um teste.")
	}
	_, err = b.client(token).SendMessage(ctx, *s.ChatID, "👋 Teste do Estus Brain: as mensagens estão chegando.", false, nil)
	if err == nil {
		b.noteSendOK()
		return nil
	}
	b.noteSendError(err)
	var apiErr *APIError
	switch {
	case errors.As(err, &apiErr) && apiErr.Code == 403:
		return invalid("O bot foi bloqueado no Telegram — desbloqueie e tente de novo.")
	case errors.As(err, &apiErr) && apiErr.Code == 401:
		return invalid("O Telegram recusou o token — gere outro no @BotFather.")
	default:
		return invalid("Não consegui mandar agora; tente de novo.")
	}
}
