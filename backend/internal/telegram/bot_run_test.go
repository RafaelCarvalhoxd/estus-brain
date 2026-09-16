package telegram

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rafael/estus-vault/backend/internal/assistant"
	"github.com/rafael/estus-vault/backend/internal/domain"
)

func TestPollFailure(t *testing.T) {
	cases := []struct {
		err      error
		wantMsg  string
		wantWait time.Duration
	}{
		{&APIError{Code: 401}, msgBadToken, 0},
		{&APIError{Code: 404}, msgBadToken, 0},
		{&APIError{Code: 409}, msgConflict, time.Minute},
		{errors.New("dial tcp: no such host"), msgOffline, 8 * time.Second},
	}
	for _, tc := range cases {
		msg, wait := pollFailure(tc.err, 8*time.Second)
		if msg != tc.wantMsg || wait != tc.wantWait {
			t.Errorf("pollFailure(%v) = %q, %v", tc.err, msg, wait)
		}
	}
}

// onlyAlerts turns the daily reports off so a tick sends alerts alone.
func onlyAlerts(t *testing.T, tb *testBot) {
	t.Helper()
	if err := tb.Update(context.Background(), SettingsInput{MorningEnabled: ptr(false), EveningEnabled: ptr(false)}); err != nil {
		t.Fatal(err)
	}
}

func TestTickSendsAReminderOnce(t *testing.T) {
	tb := newTestBot(t, nil)
	tb.paired(t)
	onlyAlerts(t, tb)
	tb.data.reminders = []domain.Reminder{{ID: "r1", Title: "Pagar luz", DueAt: ptr(tb.now.Add(-5 * time.Minute))}}
	ctx := context.Background()
	tb.tick(ctx)
	tb.tick(ctx)
	msgs := tb.api.messages()
	if len(msgs) != 1 || msgs[0].Text != "🔔 <b>Pagar luz</b>" || len(msgs[0].Buttons) != 1 || msgs[0].Buttons[0].CallbackData != "done:r1" {
		t.Fatalf("sent = %+v", msgs)
	}
}

func TestTickRetriesAnAlertThatFailedToSend(t *testing.T) {
	tb := newTestBot(t, nil)
	tb.paired(t)
	onlyAlerts(t, tb)
	tb.data.reminders = []domain.Reminder{{ID: "r1", Title: "Pagar luz", DueAt: ptr(tb.now.Add(-5 * time.Minute))}}
	ctx := context.Background()
	tb.api.sendErr = func(string, bool) error { return errors.New("network down") }
	tb.tick(ctx)
	tb.api.sendErr = nil
	tb.tick(ctx)
	if msgs := tb.api.messages(); len(msgs) != 1 {
		t.Fatalf("sent = %+v", msgs)
	}
}

func TestTickSendsTheMorningReportOnce(t *testing.T) {
	tb := newTestBot(t, nil)
	tb.paired(t)
	ctx := context.Background()
	tb.tick(ctx)
	tb.tick(ctx)
	msgs := tb.api.messages()
	if len(msgs) != 1 || !strings.Contains(msgs[0].Text, "Bom dia") {
		t.Fatalf("sent = %+v", msgs)
	}
	if day := tb.store.settings().LastMorningOn; day == nil || day.Format("2006-01-02") != "2026-09-15" {
		t.Fatalf("last morning = %v", day)
	}
}

func TestTickTriesTheMorningReportAgainAfterAFailure(t *testing.T) {
	tb := newTestBot(t, nil)
	tb.paired(t)
	tb.api.sendErr = func(string, bool) error { return errors.New("network down") }
	tb.tick(context.Background())
	if day := tb.store.settings().LastMorningOn; day != nil {
		t.Fatalf("a failed report must not count as sent, last morning = %v", day)
	}
}

func TestTickSendsAnEventHeadsUp(t *testing.T) {
	tb := newTestBot(t, nil)
	tb.paired(t)
	onlyAlerts(t, tb)
	tb.data.events = []domain.Event{{ID: "e1", Title: "Dentista", StartsAt: tb.now.Add(20 * time.Minute), EndsAt: tb.now.Add(time.Hour)}}
	tb.tick(context.Background())
	if msgs := tb.api.messages(); len(msgs) != 1 || msgs[0].Text != "📅 <b>Dentista</b> às 07:20" {
		t.Fatalf("sent = %+v", msgs)
	}
}

func TestTickDoesNothingUnpaired(t *testing.T) {
	tb := newTestBot(t, nil)
	tb.withToken(t)
	tb.data.reminders = []domain.Reminder{{ID: "r1", Title: "Pagar luz", DueAt: ptr(tb.now.Add(-time.Minute))}}
	tb.tick(context.Background())
	if msgs := tb.api.messages(); len(msgs) != 0 {
		t.Fatalf("sent = %+v", msgs)
	}
}

func TestRunHandlesUpdatesAndStops(t *testing.T) {
	tb := newTestBot(t, nil)
	tb.paired(t)
	onlyAlerts(t, tb)
	tb.api.updates = [][]Update{{func() Update { u := textUpdate(99, "private", "/ajuda"); u.UpdateID = 41; return u }()}}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		tb.Run(ctx)
		close(done)
	}()
	deadline := time.Now().Add(2 * time.Second)
	for tb.store.settings().UpdateOffset != 42 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not stop")
	}
	// The goodbye notice is what comes last now, so look for the answer to
	// /ajuda anywhere in what went out.
	if tb.store.settings().UpdateOffset != 42 || !tb.api.sentContaining("/nova") {
		t.Fatalf("offset = %d, sent = %+v", tb.store.settings().UpdateOffset, tb.api.messages())
	}
	if v, _ := tb.View(context.Background()); v.Status.State != "ok" {
		t.Fatalf("status = %+v", v.Status)
	}
}

func TestRunSavesTheOffsetBeforeTheAssistantAnswers(t *testing.T) {
	tb := newTestBot(t, nil)
	tb.paired(t)
	onlyAlerts(t, tb)
	tb.api.updates = [][]Update{{textUpdateID(41, 99, "anota um gasto de 10 reais")}}
	started := make(chan struct{})
	tb.chat.onSend = func(ctx context.Context, _ int) {
		close(started)
		<-ctx.Done() // the process is shut down mid-answer
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		tb.Run(ctx)
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("the message never reached the assistant")
	}
	// Saved before the answer, so a restart can't replay the message and its tool actions.
	if got := tb.store.settings().UpdateOffset; got != 42 {
		t.Errorf("offset while answering = %d, want 42", got)
	}
	cancel()
	<-done
	if got := tb.store.settings().UpdateOffset; got != 42 {
		t.Errorf("offset after shutdown = %d, want 42", got)
	}
}

func TestRunKeepsTheOffsetWhenTheDatabaseFails(t *testing.T) {
	tb := newTestBot(t, nil)
	tb.paired(t)
	onlyAlerts(t, tb)
	tb.store.offsetErr = errors.New("database down")
	tb.api.updates = [][]Update{{textUpdateID(41, 99, "/ajuda")}}
	tb.start(t)
	eventually(t, "a second poll", func() bool { return len(tb.api.polledOffsets()) >= 2 })
	if got := tb.api.polledOffsets()[1]; got != 42 {
		t.Fatalf("second poll offset = %d, want 42 (offsets %v)", got, tb.api.polledOffsets())
	}
}

func TestRollbacksSurviveACanceledContext(t *testing.T) {
	t.Run("report", func(t *testing.T) {
		tb := newTestBot(t, nil)
		tb.paired(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		tb.api.sendErr = func(string, bool) error { cancel(); return context.Canceled }
		tb.tick(ctx)
		if day := tb.store.settings().LastMorningOn; day != nil {
			t.Fatalf("a report cut short by shutdown must be retried, last morning = %v", day)
		}
	})
	t.Run("alert", func(t *testing.T) {
		tb := newTestBot(t, nil)
		tb.paired(t)
		onlyAlerts(t, tb)
		tb.data.reminders = []domain.Reminder{{ID: "r1", Title: "Pagar luz", DueAt: ptr(tb.now.Add(-5 * time.Minute))}}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		tb.api.sendErr = func(string, bool) error { cancel(); return context.Canceled }
		tb.tick(ctx)
		if n := tb.store.marked(); n != 0 {
			t.Fatalf("an alert cut short by shutdown must be retried, %d still marked", n)
		}
	})
}

func TestAPanicInAnUpdateKeepsThePollerRunning(t *testing.T) {
	tb := newTestBot(t, nil)
	tb.paired(t)
	onlyAlerts(t, tb)
	tb.data.onSnapshot = func() { panic("boom") }
	tb.api.updates = [][]Update{{textUpdateID(41, 99, "/hoje"), textUpdateID(42, 99, "/ajuda")}}
	tb.start(t)
	eventually(t, "the /ajuda reply", func() bool { return tb.api.sentContaining("/nova") })
}

func TestAPanicInATickIsRecovered(t *testing.T) {
	tb := newTestBot(t, nil)
	tb.paired(t)
	onlyAlerts(t, tb)
	tb.data.onReminders = func() { panic("boom") }
	func() {
		defer func() {
			if v := recover(); v != nil {
				t.Fatalf("the panic escaped tick: %v", v)
			}
		}()
		tb.tick(context.Background())
	}()
}

func TestAPanicInAnAnswerKeepsTheWorkerRunning(t *testing.T) {
	tb := newTestBot(t, nil)
	tb.paired(t)
	onlyAlerts(t, tb)
	tb.chat.calls = []fakeCall{{events: []assistant.Event{{Type: "text", Text: "resposta"}}}}
	tb.chat.onSend = func(_ context.Context, n int) {
		if n == 1 {
			panic("boom")
		}
	}
	tb.api.updates = [][]Update{{textUpdateID(41, 99, "oi"), textUpdateID(42, 99, "tudo bem?")}}
	tb.start(t)
	eventually(t, "the second answer", func() bool { return tb.api.sentContaining("resposta") })
}

func TestAButtonIsAnsweredWhileTheAssistantThinks(t *testing.T) {
	tb := newTestBot(t, nil)
	tb.paired(t)
	onlyAlerts(t, tb)
	release := make(chan struct{})
	defer close(release)
	tb.chat.onSend = func(ctx context.Context, _ int) {
		select {
		case <-release:
		case <-ctx.Done():
		}
	}
	cb := Update{UpdateID: 42, CallbackQuery: &CallbackQuery{ID: "cb1", Data: "done:r1", Message: &Message{MessageID: 5, Chat: Chat{ID: 99, Type: "private"}}}}
	tb.api.updates = [][]Update{{textUpdateID(41, 99, "resume meu mês")}, {cb}}
	tb.start(t)
	eventually(t, "the button's answer", func() bool {
		a := tb.api.answered()
		return len(a) == 1 && a[0] == "Concluído"
	})
}

func TestABotChangeStopsTheBatch(t *testing.T) {
	tb := newTestBot(t, nil)
	tb.paired(t)
	onlyAlerts(t, tb)
	_ = tb.store.SetOffset(context.Background(), 41)
	tb.data.onSnapshot = func() {
		tb.data.onSnapshot = nil
		tb.api.me = User{ID: 2, Username: "outro_bot"}
		if err := tb.Update(context.Background(), SettingsInput{Token: ptr("456:def")}); err != nil {
			t.Errorf("change bot: %v", err)
		}
	}
	tb.api.updates = [][]Update{{textUpdateID(41, 99, "/hoje"), textUpdateID(42, 99, "/ajuda")}}
	tb.start(t)
	eventually(t, "a poll after the change", func() bool { return len(tb.api.polledOffsets()) >= 2 })
	if got := tb.api.polledOffsets(); got[0] != 41 || got[1] != 0 {
		t.Errorf("poll offsets = %v, want 41 then 0", got)
	}
	if got := tb.store.settings().UpdateOffset; got != 0 {
		t.Errorf("stored offset = %d, want 0", got)
	}
	if tb.api.sentContaining("/nova") {
		t.Error("the old bot's batch went on after the change")
	}
}

func TestStatusShowsBeforeTheFirstPollReturns(t *testing.T) {
	unpaired := newTestBot(t, nil)
	unpaired.withToken(t)
	unpaired.start(t)
	eventually(t, "the unpaired status", func() bool {
		v, _ := unpaired.View(context.Background())
		return v.Status == Status{State: "unpaired", Message: msgUnpaired}
	})

	paired := newTestBot(t, nil)
	paired.paired(t)
	onlyAlerts(t, paired)
	paired.start(t)
	eventually(t, "the ok status", func() bool {
		v, _ := paired.View(context.Background())
		return v.Status == Status{State: "ok", Message: msgOK}
	})
}

func TestASuccessfulSendClearsBlocked(t *testing.T) {
	tb := newTestBot(t, nil)
	tb.paired(t)
	ctx := context.Background()
	tb.api.sendErr = func(string, bool) error {
		return &APIError{Code: 403, Description: "Forbidden: bot was blocked by the user"}
	}
	_ = tb.SendTest(ctx)
	if v, _ := tb.View(ctx); v.Status.Message != msgBlocked {
		t.Fatalf("status = %+v", v.Status)
	}
	tb.api.sendErr = nil
	tb.handle(ctx, tb.api, textUpdate(99, "private", "/ajuda"))
	if v, _ := tb.View(ctx); v.Status != (Status{State: "ok", Message: msgOK}) {
		t.Fatalf("status after a delivered message = %+v", v.Status)
	}
}

func TestAnUnreadableTokenIsReported(t *testing.T) {
	tb := newTestBot(t, func(c *Config) { c.EnvToken = "999:env" })
	s := tb.store.settings()
	s.TokenSecret = "c2FsdmEgY29tIG91dHJhIGNoYXZlIGRvIGNvZnJl" // sealed with another key
	_ = tb.store.SaveSettings(context.Background(), s)
	tb.start(t)
	eventually(t, "the unreadable token status", func() bool {
		v, _ := tb.View(context.Background())
		return v.Status == Status{State: "error", Message: msgUnreadableToken}
	})
	if got := tb.api.polledOffsets(); len(got) != 0 {
		t.Fatalf("polled with the environment's token instead: %v", got)
	}
}
