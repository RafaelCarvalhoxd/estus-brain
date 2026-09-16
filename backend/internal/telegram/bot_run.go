package telegram

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/rafael/estus-vault/backend/internal/store/postgres"
)

// pollTimeout is how long Telegram holds a getUpdates call open, in seconds.
const pollTimeout = 30

// Run polls Telegram, answers messages and runs the scheduler until ctx ends.
func (b *Bot) Run(ctx context.Context) {
	var wg sync.WaitGroup
	for _, loop := range []func(context.Context){b.pollLoop, b.answerLoop, b.scheduleLoop} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			loop(ctx)
		}()
	}
	wg.Wait()
	b.sayGoodbye(ctx)
}

// sayGoodbye tells the owner the bot is going down, on the way out of Run.
// It runs after ctx is already canceled, so it borrows a few seconds of its
// own — a clean shutdown is the one moment the bot can still reach Telegram
// to say it is leaving.
func (b *Bot) sayGoodbye(ctx context.Context) {
	if !b.goOffline() {
		return
	}
	sendCtx, cancel := saveContext(ctx)
	defer cancel()
	s, err := b.cfg.Store.Settings(sendCtx)
	if err != nil || s.ChatID == nil {
		return
	}
	token, _ := b.token(s)
	if token == "" {
		return
	}
	if err := b.sendHTML(sendCtx, b.client(token), *s.ChatID, disconnectedNotice("Desligando."), nil); err != nil {
		slog.Warn("telegram: say goodbye", "error", err)
	}
}

func (b *Bot) pollLoop(ctx context.Context) {
	backoff := time.Second
	for ctx.Err() == nil {
		b.mu.Lock()
		generation := b.generation
		b.mu.Unlock()
		s, err := b.cfg.Store.Settings(ctx)
		if err != nil {
			slog.Warn("telegram: read settings", "error", err)
			b.wait(ctx, 10*time.Second)
			continue
		}
		token, fromEnv := b.token(s)
		if token == "" {
			b.showNoToken(s)
			b.wait(ctx, 0)
			continue
		}
		api := b.client(token)
		if fromEnv && s.BotUsername == "" {
			b.learnUsername(ctx, api)
		}
		b.pollStarting(s, token)

		// A settings change cancels the long poll so it takes effect at once;
		// one that came in since the settings were read cancels it right away.
		pollCtx, cancel := context.WithCancel(ctx)
		b.mu.Lock()
		b.cancelPoll = cancel
		if b.generation != generation {
			cancel()
		}
		// The saved offset can lag behind when writing it failed.
		offset := max(s.UpdateOffset, b.offset)
		b.mu.Unlock()
		updates, err := api.GetUpdates(pollCtx, offset, pollTimeout)
		b.mu.Lock()
		b.cancelPoll = nil
		b.mu.Unlock()
		reloaded := pollCtx.Err() != nil
		cancel()
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			if reloaded {
				continue
			}
			message, delay := pollFailure(err, backoff)
			slog.Warn("telegram: poll", "error", err)
			b.setStatus("error", message)
			b.noteOffline(ctx, api, s, message)
			b.wait(ctx, delay)
			if message == msgOffline {
				backoff = min(backoff*2, time.Minute)
			}
			continue
		}
		backoff = time.Second
		b.pollSucceeded(s, generation)
		b.noteOnline(ctx, api, s)
		for _, u := range updates {
			if !b.takeUpdate(ctx, generation, u.UpdateID+1) {
				break // the settings changed: the next poll reads the rest
			}
			// Handled with the long-lived ctx: a settings change must not cut
			// an answer short.
			b.handle(ctx, api, u)
		}
	}
}

// noteOnline records that the bot is answering and, the first time after a
// silence, tells the owner — with how long it was away, read from s, which
// still holds the last online time from before this poll.
func (b *Bot) noteOnline(ctx context.Context, api API, s postgres.TelegramSettings) {
	now := b.now()
	fresh := b.goOnline()
	if err := b.cfg.Store.SetOnlineAt(ctx, now); err != nil {
		slog.Warn("telegram: save last online", "error", err)
	}
	if !fresh || s.ChatID == nil {
		return
	}
	if err := b.sendHTML(ctx, api, *s.ChatID, connectedNotice(s.LastOnlineAt, now), nil); err != nil {
		slog.Warn("telegram: say hello", "error", err)
	}
}

// noteOffline tells the owner the bot stopped answering, once per outage.
// The send usually fails — the poll failed because Telegram is out of reach,
// and so is this message — which is exactly why the notice on the way back
// says how long the silence lasted. It still gets through for an outage that
// isn't the network's fault, such as a second Estus stealing the bot.
func (b *Bot) noteOffline(ctx context.Context, api API, s postgres.TelegramSettings, reason string) {
	if !b.goOffline() || s.ChatID == nil {
		return
	}
	if err := b.sendHTML(ctx, api, *s.ChatID, disconnectedNotice(reason), nil); err != nil {
		slog.Debug("telegram: could not announce the disconnection", "error", err)
	}
}

// takeUpdate moves the offset past an update before it is handled, so a
// restart mid-answer drops the message instead of running its actions twice.
// It refuses once the settings changed since the poll began: after a bot
// change the batch's update ids belong to the old bot.
func (b *Bot) takeUpdate(ctx context.Context, generation uint64, next int64) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.generation != generation {
		return false
	}
	b.offset = next
	saveCtx, cancel := saveContext(ctx)
	defer cancel()
	if err := b.cfg.Store.SetOffset(saveCtx, next); err != nil {
		slog.Warn("telegram: save offset", "error", err)
	}
	return true
}

// showNoToken says why there is no bot to poll.
func (b *Bot) showNoToken(s postgres.TelegramSettings) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.statusToken = ""
	if b.tokenUnreadable(s) {
		b.status = Status{State: "error", Message: msgUnreadableToken}
	} else {
		b.status = Status{State: "off", Message: msgNoToken}
	}
}

// pollStarting shows the saved state while the first long poll is still open,
// keeping a problem already found with this same token.
func (b *Bot) pollStarting(s postgres.TelegramSettings, token string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	known := b.statusToken == token && b.status.State == "error" && (s.ChatID != nil || b.status.Message != msgBlocked)
	b.statusToken = token
	switch {
	case known:
	case s.ChatID == nil:
		b.status = Status{State: "unpaired", Message: msgUnpaired}
	default:
		b.status = Status{State: "ok", Message: msgOK}
	}
}

// pollSucceeded shows the bot as working, unless sending is what's broken.
func (b *Bot) pollSucceeded(s postgres.TelegramSettings, generation uint64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.generation != generation {
		return // s is stale; the next poll sets the status
	}
	switch {
	case s.ChatID == nil:
		b.status = Status{State: "unpaired", Message: msgUnpaired}
	case b.status.Message != msgBlocked:
		b.status = Status{State: "ok", Message: msgOK}
	}
}

// pollFailure says what a failed getUpdates means for the owner and how long
// to wait before trying again; 0 waits for a settings change.
func pollFailure(err error, backoff time.Duration) (string, time.Duration) {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		switch apiErr.Code {
		case 401, 404:
			return msgBadToken, 0
		case 409:
			return msgConflict, time.Minute
		}
	}
	return msgOffline, backoff
}

// wait sleeps for d, or until a settings change or ctx ends; d == 0 waits
// only for those.
func (b *Bot) wait(ctx context.Context, d time.Duration) {
	var timer <-chan time.Time
	if d > 0 {
		t := time.NewTimer(d)
		defer t.Stop()
		timer = t.C
	}
	select {
	case <-ctx.Done():
	case <-b.reload:
	case <-timer:
	}
}

// learnUsername fills in the bot's @ when the token came from the environment.
func (b *Bot) learnUsername(ctx context.Context, api API) {
	me, err := api.GetMe(ctx)
	if err != nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	s, err := b.cfg.Store.Settings(ctx)
	if err != nil {
		return
	}
	s.BotUsername = me.Username
	if err := b.cfg.Store.SaveSettings(ctx, s); err != nil {
		slog.Warn("telegram: save bot username", "error", err)
	}
}

func (b *Bot) scheduleLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		b.tick(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// tick sends whatever is due now.
func (b *Bot) tick(ctx context.Context) {
	defer logPanic("tick")
	s, err := b.cfg.Store.Settings(ctx)
	if err != nil {
		slog.Warn("telegram: read settings", "error", err)
		return
	}
	token, _ := b.token(s)
	if token == "" || s.ChatID == nil {
		return
	}
	api := b.client(token)
	now := b.now()
	if today := calendarDay(now); !today.Equal(b.lastPrune) {
		if err := b.cfg.Store.PruneSent(ctx, now.AddDate(0, 0, -30)); err != nil {
			slog.Warn("telegram: prune alerts", "error", err)
		}
		b.lastPrune = today
	}
	reminders, err := b.cfg.Data.Reminders(ctx)
	if err != nil {
		slog.Warn("telegram: load reminders", "error", err)
	}
	lead := time.Duration(s.EventsMinutesBefore) * time.Minute
	events, err := b.cfg.Data.Events(ctx, now, now.Add(lead+time.Minute))
	if err != nil {
		slog.Warn("telegram: load events", "error", err)
	}
	for _, job := range Due(now, s, reminders, events) {
		b.send(ctx, api, s, job, now)
	}
}

// send delivers one job. It records the job as sent first, so it can't go
// out twice, and takes the record back if delivery fails, so it's retried
// on the next minute.
func (b *Bot) send(ctx context.Context, api API, s postgres.TelegramSettings, job Job, now time.Time) {
	chatID := *s.ChatID
	switch job.Kind {
	case JobMorning, JobEvening:
		previous, build := s.LastMorningOn, MorningReport
		if job.Kind == JobEvening {
			previous, build = s.LastEveningOn, EveningReport
		}
		today := calendarDay(now)
		if err := b.cfg.Store.SetReportDay(ctx, string(job.Kind), &today); err != nil {
			slog.Warn("telegram: mark report", "report", job.Kind, "error", err)
			return
		}
		if err := b.sendLong(ctx, api, chatID, build(b.cfg.Data.Snapshot(ctx, now))); err != nil {
			// Taken back even when a shutdown cut the send short, or the
			// report would count as sent.
			saveCtx, cancel := saveContext(ctx)
			defer cancel()
			if err := b.cfg.Store.SetReportDay(saveCtx, string(job.Kind), previous); err != nil {
				slog.Warn("telegram: unmark report", "report", job.Kind, "error", err)
			}
		}
	case JobReminder, JobEvent:
		fresh, err := b.cfg.Store.MarkSent(ctx, string(job.Kind), job.Ref)
		if err != nil {
			slog.Warn("telegram: mark alert", "error", err)
			return
		}
		if !fresh {
			return
		}
		text, buttons := EventAlert(job.Event, b.cfg.Location), []Button(nil)
		if job.Kind == JobReminder {
			text = ReminderAlert(job.Reminder)
			buttons = []Button{{Text: "✅ Concluído", CallbackData: "done:" + job.Reminder.ID}}
		}
		if err := b.sendHTML(ctx, api, chatID, text, buttons); err != nil {
			saveCtx, cancel := saveContext(ctx)
			defer cancel()
			if err := b.cfg.Store.UnmarkSent(saveCtx, string(job.Kind), job.Ref); err != nil {
				slog.Warn("telegram: unmark alert", "error", err)
			}
		}
	}
}
