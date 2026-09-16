package telegram

import (
	"context"
	"testing"
	"time"
)

// Telegram holds a message for up to 24h while the bot is off, then hands the
// whole backlog over at once. The assistant has to book "gastei 50 no
// mercado" on the day it was said, not on the day the laptop came back, so
// the send time travels with the message.
func TestTheAssistantIsToldWhenTheMessageWasSent(t *testing.T) {
	tb := newTestBot(t, nil)
	tb.paired(t)
	sent := tb.now.Add(-3 * time.Hour)
	u := textUpdate(99, "private", "gastei 50 no mercado")
	u.Message.Date = sent.Unix()
	tb.handleAll(context.Background(), u)
	if len(tb.chat.reqs) != 1 {
		t.Fatalf("requests = %+v", tb.chat.reqs)
	}
	if got := tb.chat.reqs[0].SentAt; !got.Equal(sent) {
		t.Errorf("SentAt = %v, want %v", got, sent)
	}
}

// A message with no date (a synthetic update, or a ready-made action) leaves
// SentAt zero, and the assistant falls back to the clock.
func TestAMessageWithoutADateLeavesTheSendTimeUnset(t *testing.T) {
	tb := newTestBot(t, nil)
	tb.paired(t)
	tb.handleAll(context.Background(), textUpdate(99, "private", "quanto gastei?"))
	if len(tb.chat.reqs) != 1 {
		t.Fatalf("requests = %+v", tb.chat.reqs)
	}
	if got := tb.chat.reqs[0].SentAt; !got.IsZero() {
		t.Errorf("SentAt = %v, want the zero time", got)
	}
}
