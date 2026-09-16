package telegram

import (
	"fmt"
	"time"
)

// This file builds the two lines the owner sees in the chat when the bot
// starts or stops answering. They exist because the bot usually runs on a
// laptop: knowing it is listening — and, on the way back, how long it wasn't
// — is the difference between trusting what you sent it and wondering.

const (
	connectedHead    = "🟢 <b>Estus conectado</b>"
	disconnectedHead = "🔴 <b>Estus desconectado</b>"
)

// connectedNotice greets the owner when the bot starts answering again,
// naming the gap when the last time it was online is known. A since in the
// future (a clock that moved, or a write from a process still shutting down)
// is treated as unknown rather than reported as a negative absence.
func connectedNotice(since *time.Time, now time.Time) string {
	if since == nil {
		return connectedHead
	}
	gap := now.Sub(*since)
	if gap <= 0 {
		return connectedHead
	}
	return fmt.Sprintf("%s\nEstive fora desde %s (%s).",
		connectedHead, since.In(now.Location()).Format("02/01 15:04"), awayFor(gap))
}

// disconnectedNotice says the bot is about to stop answering, and why.
func disconnectedNotice(reason string) string {
	return disconnectedHead + "\n" + esc(reason)
}

// goOnline marks the bot as answering and reports whether that is news the
// owner has not been told yet. goOffline is its mirror: it stays quiet for a
// bot that never got as far as answering, so a laptop that boots with no
// network doesn't announce a disconnection that never happened.
func (b *Bot) goOnline() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.online {
		return false
	}
	b.online = true
	return true
}

func (b *Bot) goOffline() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.online {
		return false
	}
	b.online = false
	return true
}

// awayFor spells a gap the way the owner would say it out loud.
func awayFor(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "menos de 1 min"
	case d < time.Hour:
		return fmt.Sprintf("%d min", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh%02d", int(d.Hours()), int(d.Minutes())%60)
	default:
		return fmt.Sprintf("%dd %dh", int(d.Hours())/24, int(d.Hours())%24)
	}
}
