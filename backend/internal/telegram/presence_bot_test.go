package telegram

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestGoOnlineOnlyReportsTheFirstTime(t *testing.T) {
	tb := newTestBot(t, nil)
	if !tb.goOnline() {
		t.Fatal("the first goOnline should ask for a notice")
	}
	if tb.goOnline() {
		t.Error("a second goOnline should stay quiet")
	}
}

func TestGoOfflineStaysQuietUntilTheBotHasBeenOnline(t *testing.T) {
	tb := newTestBot(t, nil)
	if tb.goOffline() {
		t.Error("goOffline before ever answering should stay quiet")
	}
	tb.goOnline()
	if !tb.goOffline() {
		t.Fatal("goOffline after being online should ask for a notice")
	}
	if tb.goOffline() {
		t.Error("a second goOffline should stay quiet")
	}
}

func TestGoOnlineReportsAgainAfterAnOutage(t *testing.T) {
	tb := newTestBot(t, nil)
	tb.goOnline()
	tb.goOffline()
	if !tb.goOnline() {
		t.Fatal("coming back from an outage should ask for a notice")
	}
}

// countSent is how many messages sent so far contain text.
func countSent(tb *testBot, text string) int {
	n := 0
	for _, m := range tb.api.messages() {
		if strings.Contains(m.Text, text) {
			n++
		}
	}
	return n
}

func TestPollAnnouncesTheBotIsConnectedOnlyOnce(t *testing.T) {
	tb := newTestBot(t, nil)
	tb.paired(t)
	// Two empty batches: two successful polls, one notice.
	tb.api.updates = [][]Update{{}, {}}
	tb.start(t)
	eventually(t, "the connected notice", func() bool { return countSent(tb, "Estus conectado") == 1 })
	eventually(t, "the second poll", func() bool { return len(tb.api.polledOffsets()) >= 3 })
	if n := countSent(tb, "Estus conectado"); n != 1 {
		t.Fatalf("connected notices = %d, want 1", n)
	}
}

func TestAnUnpairedBotSaysNothing(t *testing.T) {
	tb := newTestBot(t, nil)
	tb.withToken(t) // a token, but no chat to talk to yet
	tb.start(t)
	eventually(t, "the first poll", func() bool { return len(tb.api.polledOffsets()) >= 1 })
	if msgs := tb.api.messages(); len(msgs) != 0 {
		t.Fatalf("sent = %+v, want nothing", msgs)
	}
}

func TestPollRecordsWhenTheBotWasLastOnline(t *testing.T) {
	tb := newTestBot(t, nil)
	tb.paired(t)
	tb.api.updates = [][]Update{{}} // one poll that comes back empty
	tb.start(t)
	eventually(t, "last_online_at", func() bool { return tb.store.settings().LastOnlineAt != nil })
	if got := *tb.store.settings().LastOnlineAt; !got.Equal(tb.now) {
		t.Errorf("last online = %v, want %v", got, tb.now)
	}
}

func TestStoppingAnnouncesTheBotIsDisconnected(t *testing.T) {
	tb := newTestBot(t, nil)
	tb.paired(t)
	tb.api.updates = [][]Update{{}} // one poll that comes back empty
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		tb.Run(ctx)
	}()
	eventually(t, "the connected notice", func() bool { return tb.api.sentContaining("Estus conectado") })
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not stop")
	}
	if !tb.api.sentContaining("Estus desconectado") {
		t.Fatalf("sent = %+v", tb.api.messages())
	}
}

// A bot that never got as far as answering has nothing to say goodbye about.
func TestStoppingAnUnpairedBotSaysNothing(t *testing.T) {
	tb := newTestBot(t, nil)
	tb.withToken(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		tb.Run(ctx)
	}()
	eventually(t, "the first poll", func() bool { return len(tb.api.polledOffsets()) >= 1 })
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not stop")
	}
	if msgs := tb.api.messages(); len(msgs) != 0 {
		t.Fatalf("sent = %+v, want nothing", msgs)
	}
}
