package telegram

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/rafael/estus-vault/backend/internal/domain"
)

func TestUpdateStoresTheTokenEncrypted(t *testing.T) {
	tb := newTestBot(t, nil)
	tb.withToken(t)
	s := tb.store.settings()
	if s.TokenSecret == "" || strings.Contains(s.TokenSecret, "123:abc") || s.BotUsername != "estus_bot" {
		t.Fatalf("settings = %+v", s)
	}
	if token, fromEnv := tb.token(s); token != "123:abc" || fromEnv {
		t.Fatalf("token = %q, fromEnv = %v", token, fromEnv)
	}
}

func TestUpdateRejectsATokenTelegramRefuses(t *testing.T) {
	tb := newTestBot(t, nil)
	tb.api.meErr = &APIError{Code: 401, Description: "Unauthorized"}
	err := tb.Update(context.Background(), SettingsInput{Token: ptr("bad")})
	if !errors.Is(err, domain.ErrValidation) || !strings.HasSuffix(err.Error(), ": O Telegram recusou o token — confira no @BotFather.") {
		t.Fatalf("err = %v", err)
	}
	if tb.store.settings().TokenSecret != "" {
		t.Fatal("a refused token must not be stored")
	}
}

func TestUpdateNeedsTheVaultKeyToStoreAToken(t *testing.T) {
	tb := newTestBot(t, func(c *Config) { c.VaultKey = nil })
	err := tb.Update(context.Background(), SettingsInput{Token: ptr("123:abc")})
	if !errors.Is(err, domain.ErrValidation) || !strings.Contains(err.Error(), "VAULT_ENCRYPTION_KEY") {
		t.Fatalf("err = %v", err)
	}
}

func TestANewBotForgetsTheOwner(t *testing.T) {
	tb := newTestBot(t, nil)
	tb.paired(t)
	_ = tb.store.SetOffset(context.Background(), 50)
	tb.api.me = User{ID: 2, Username: "outro_bot"}
	if err := tb.Update(context.Background(), SettingsInput{Token: ptr("456:def")}); err != nil {
		t.Fatal(err)
	}
	if s := tb.store.settings(); s.ChatID != nil || s.OwnerName != "" || s.UpdateOffset != 0 || s.BotUsername != "outro_bot" {
		t.Fatalf("settings = %+v", s)
	}
}

func TestRemovingTheTokenForgetsEverything(t *testing.T) {
	tb := newTestBot(t, nil)
	tb.paired(t)
	if err := tb.Update(context.Background(), SettingsInput{Token: ptr("")}); err != nil {
		t.Fatal(err)
	}
	if s := tb.store.settings(); s.TokenSecret != "" || s.BotUsername != "" || s.ChatID != nil {
		t.Fatalf("settings = %+v", s)
	}
}

func TestUpdateValidatesTimes(t *testing.T) {
	tb := newTestBot(t, nil)
	ctx := context.Background()
	if err := tb.Update(ctx, SettingsInput{MorningTime: ptr("13:00")}); err == nil || !strings.HasSuffix(err.Error(), ": Horário da manhã precisa ficar entre 04:00 e 11:59.") {
		t.Errorf("message = %v", err)
	}
	for _, in := range []SettingsInput{
		{MorningTime: ptr("13:00")},
		{MorningTime: ptr("7:00")},
		{EveningTime: ptr("11:00")},
		{EventsMinutesBefore: ptr(0)},
		{EventsMinutesBefore: ptr(1441)},
	} {
		if err := tb.Update(ctx, in); !errors.Is(err, domain.ErrValidation) {
			t.Errorf("Update(%+v) = %v, want a validation error", in, err)
		}
	}
	if err := tb.Update(ctx, SettingsInput{MorningTime: ptr("06:30"), EveningTime: ptr("22:15"), EventsMinutesBefore: ptr(15), RemindersEnabled: ptr(false)}); err != nil {
		t.Fatal(err)
	}
	if s := tb.store.settings(); s.MorningTime != "06:30" || s.EveningTime != "22:15" || s.EventsMinutesBefore != 15 || s.RemindersEnabled {
		t.Fatalf("settings = %+v", s)
	}
}

func TestStartPairing(t *testing.T) {
	tb := newTestBot(t, nil)
	ctx := context.Background()
	if _, err := tb.StartPairing(ctx); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("pairing without a token = %v", err)
	}
	tb.withToken(t)
	p, err := tb.StartPairing(ctx)
	if err != nil || !regexp.MustCompile(`^\d{6}$`).MatchString(p.Code) || !p.ExpiresAt.Equal(tb.now.Add(pairingTTL)) || p.Link != "https://t.me/estus_bot?start="+p.Code {
		t.Fatalf("pairing = %+v, %v", p, err)
	}
	if v, _ := tb.View(ctx); v.PairingCode != p.Code {
		t.Fatalf("view code = %q", v.PairingCode)
	}
	tb.paired(t)
	if _, err := tb.StartPairing(ctx); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("pairing while paired = %v", err)
	}
}

func TestViewHidesAnExpiredCode(t *testing.T) {
	tb := newTestBot(t, nil)
	tb.withToken(t)
	s := tb.store.settings()
	s.PairingCode, s.PairingExpiresAt = "123456", ptr(tb.now.Add(-1))
	_ = tb.store.SaveSettings(context.Background(), s)
	if v, _ := tb.View(context.Background()); v.PairingCode != "" || v.PairingExpiresAt != nil {
		t.Fatalf("view = %+v", v)
	}
}

func TestEnvironmentToken(t *testing.T) {
	tb := newTestBot(t, func(c *Config) { c.VaultKey, c.EnvToken = nil, "999:env" })
	v, err := tb.View(context.Background())
	if err != nil || !v.HasToken || !v.FromEnv || v.CanStoreToken {
		t.Fatalf("view = %+v, %v", v, err)
	}
}

func TestUnpairAndSendTest(t *testing.T) {
	tb := newTestBot(t, nil)
	ctx := context.Background()
	tb.withToken(t)
	if err := tb.SendTest(ctx); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("test before pairing = %v", err)
	}
	tb.paired(t)
	if err := tb.SendTest(ctx); err != nil {
		t.Fatal(err)
	}
	if msgs := tb.api.messages(); len(msgs) != 1 || msgs[0].ChatID != 99 {
		t.Fatalf("sent = %+v", msgs)
	}
	tb.api.sendErr = func(string, bool) error {
		return &APIError{Code: 403, Description: "Forbidden: bot was blocked by the user"}
	}
	if err := tb.SendTest(ctx); !errors.Is(err, domain.ErrValidation) || !strings.Contains(err.Error(), "bloqueado") {
		t.Fatalf("blocked test = %v", err)
	}
	if v, _ := tb.View(ctx); v.Status.Message != msgBlocked {
		t.Fatalf("status = %+v", v.Status)
	}
	if err := tb.Unpair(ctx); err != nil {
		t.Fatal(err)
	}
	if s := tb.store.settings(); s.ChatID != nil || s.OwnerName != "" {
		t.Fatalf("settings = %+v", s)
	}
}

func TestANewBotResetsTheOffsetWithTheSettings(t *testing.T) {
	tb := newTestBot(t, nil)
	tb.paired(t)
	_ = tb.store.SetOffset(context.Background(), 50)
	tb.offset = 50
	// A separate offset write is what used to fail and leave the new bot deaf.
	tb.store.offsetErr = errors.New("database down")
	tb.api.me = User{ID: 2, Username: "outro_bot"}
	if err := tb.Update(context.Background(), SettingsInput{Token: ptr("456:def")}); err != nil {
		t.Fatal(err)
	}
	if s := tb.store.settings(); s.UpdateOffset != 0 || s.BotUsername != "outro_bot" || tb.offset != 0 {
		t.Fatalf("settings = %+v, offset in memory = %d", s, tb.offset)
	}
}

func TestUpdateChecksTheTokenWithoutHoldingTheLock(t *testing.T) {
	tb := newTestBot(t, nil)
	held := false
	tb.api.onGetMe = func() {
		if tb.mu.TryLock() {
			tb.mu.Unlock()
		} else {
			held = true
		}
	}
	tb.withToken(t)
	if held {
		t.Fatal("GetMe ran with the settings lock held")
	}
}
