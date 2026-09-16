package telegram

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rafael/estus-vault/backend/internal/assistant"
	"github.com/rafael/estus-vault/backend/internal/domain"
	"github.com/rafael/estus-vault/backend/internal/store/postgres"
)

type sentMessage struct {
	ChatID    int64
	MessageID int64
	Text      string
	HTML      bool
	Buttons   []Button
	Keyboard  [][]Button
}

type fakeAPI struct {
	mu          sync.Mutex
	me          User
	meErr       error
	onGetMe     func()
	updates     [][]Update
	updatesErr  error
	offsets     []int64 // the offset of every getUpdates call
	sendErr     func(text string, html bool) error
	sent        []sentMessage
	edits       []sentMessage
	answers     []string
	files       map[string][]byte // file_id → bytes, for GetFile/DownloadFile
	downloadErr error
}

func (f *fakeAPI) GetMe(context.Context) (User, error) {
	if f.onGetMe != nil {
		f.onGetMe()
	}
	return f.me, f.meErr
}

// GetUpdates hands out the scripted batches; once they run out it holds the
// call open until ctx ends, like a long poll with nothing new.
func (f *fakeAPI) GetUpdates(ctx context.Context, offset int64, _ int) ([]Update, error) {
	f.mu.Lock()
	f.offsets = append(f.offsets, offset)
	if f.updatesErr != nil {
		defer f.mu.Unlock()
		return nil, f.updatesErr
	}
	if len(f.updates) > 0 {
		defer f.mu.Unlock()
		next := f.updates[0]
		f.updates = f.updates[1:]
		return next, nil
	}
	f.mu.Unlock()
	<-ctx.Done()
	return nil, ctx.Err()
}

func (f *fakeAPI) SendMessage(_ context.Context, chatID int64, text string, html bool, buttons []Button) (Message, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.sendErr != nil {
		if err := f.sendErr(text, html); err != nil {
			return Message{}, err
		}
	}
	f.sent = append(f.sent, sentMessage{ChatID: chatID, Text: text, HTML: html, Buttons: buttons})
	return Message{MessageID: int64(len(f.sent)), Chat: Chat{ID: chatID, Type: "private"}}, nil
}

func (f *fakeAPI) SendKeyboard(_ context.Context, chatID int64, text string, rows [][]Button) (Message, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.sendErr != nil {
		if err := f.sendErr(text, true); err != nil {
			return Message{}, err
		}
	}
	f.sent = append(f.sent, sentMessage{ChatID: chatID, Text: text, HTML: true, Keyboard: rows})
	return Message{MessageID: int64(len(f.sent)), Chat: Chat{ID: chatID, Type: "private"}}, nil
}

func (f *fakeAPI) EditKeyboard(_ context.Context, chatID, messageID int64, text string, rows [][]Button) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	// The menu edits itself in place; tests read it like any other message.
	f.sent = append(f.sent, sentMessage{ChatID: chatID, MessageID: messageID, Text: text, HTML: true, Keyboard: rows})
	return nil
}

func (f *fakeAPI) SendChatAction(context.Context, int64) error { return nil }

func (f *fakeAPI) AnswerCallbackQuery(_ context.Context, _ string, text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.answers = append(f.answers, text)
	return nil
}

func (f *fakeAPI) EditMessageText(_ context.Context, chatID, messageID int64, text string, html bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.edits = append(f.edits, sentMessage{ChatID: chatID, MessageID: messageID, Text: text, HTML: html})
	return nil
}

func (f *fakeAPI) GetFile(_ context.Context, fileID string) (File, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.files[fileID]; !ok {
		return File{}, &APIError{Code: 400, Description: "Bad Request: invalid file_id"}
	}
	return File{FileID: fileID, FilePath: "voice/" + fileID + ".oga"}, nil
}

func (f *fakeAPI) DownloadFile(_ context.Context, filePath string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.downloadErr != nil {
		return nil, f.downloadErr
	}
	return f.files[strings.TrimSuffix(strings.TrimPrefix(filePath, "voice/"), ".oga")], nil
}

func (f *fakeAPI) messages() []sentMessage {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]sentMessage(nil), f.sent...)
}

func (f *fakeAPI) answered() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.answers...)
}

func (f *fakeAPI) polledOffsets() []int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]int64(nil), f.offsets...)
}

// sentContaining reports whether any message sent so far contains text.
func (f *fakeAPI) sentContaining(text string) bool {
	for _, m := range f.messages() {
		if strings.Contains(m.Text, text) {
			return true
		}
	}
	return false
}

// fakeStore mimics TelegramRepo, including SaveSettings leaving the offset
// and report days alone and every call failing once ctx is canceled.
type fakeStore struct {
	mu        sync.Mutex
	s         postgres.TelegramSettings
	sent      map[string]bool
	offsetErr error // makes SetOffset fail
}

func newFakeStore() *fakeStore {
	return &fakeStore{sent: map[string]bool{}, s: postgres.TelegramSettings{
		MorningEnabled: true, MorningTime: "07:00", EveningEnabled: true, EveningTime: "21:00",
		RemindersEnabled: true, EventsEnabled: true, EventsMinutesBefore: 30,
	}}
}

func (f *fakeStore) Settings(ctx context.Context) (postgres.TelegramSettings, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return postgres.TelegramSettings{}, err
	}
	return f.s, nil
}

func (f *fakeStore) SaveSettings(ctx context.Context, s postgres.TelegramSettings) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	s.UpdateOffset, s.LastMorningOn, s.LastEveningOn = f.s.UpdateOffset, f.s.LastMorningOn, f.s.LastEveningOn
	s.LastOnlineAt = f.s.LastOnlineAt
	f.s = s
	return nil
}

func (f *fakeStore) SaveSettingsNewBot(ctx context.Context, s postgres.TelegramSettings) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	s.LastMorningOn, s.LastEveningOn = f.s.LastMorningOn, f.s.LastEveningOn
	s.LastOnlineAt = f.s.LastOnlineAt
	s.UpdateOffset = 0
	f.s = s
	return nil
}

func (f *fakeStore) SetOffset(ctx context.Context, offset int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if f.offsetErr != nil {
		return f.offsetErr
	}
	f.s.UpdateOffset = offset
	return nil
}

func (f *fakeStore) SetOnlineAt(ctx context.Context, t time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	f.s.LastOnlineAt = &t
	return nil
}

func (f *fakeStore) SetReportDay(ctx context.Context, kind string, day *time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	switch kind {
	case "morning":
		f.s.LastMorningOn = day
	case "evening":
		f.s.LastEveningOn = day
	default:
		return errors.New("unknown report")
	}
	return nil
}

func (f *fakeStore) MarkSent(ctx context.Context, kind, ref string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if f.sent[kind+"|"+ref] {
		return false, nil
	}
	f.sent[kind+"|"+ref] = true
	return true, nil
}

func (f *fakeStore) UnmarkSent(ctx context.Context, kind, ref string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	delete(f.sent, kind+"|"+ref)
	return nil
}

func (f *fakeStore) PruneSent(context.Context, time.Time) error { return nil }

func (f *fakeStore) settings() postgres.TelegramSettings {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.s
}

func (f *fakeStore) marked() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.sent)
}

// fakeChat answers each Send with the next scripted call (the last one repeats).
type fakeCall struct {
	events []assistant.Event
	err    error
}

type fakeChat struct {
	mu    sync.Mutex
	calls []fakeCall
	reqs  []assistant.SendRequest
	// onSend runs at the start of every Send with its 1-based number, outside the lock.
	onSend func(ctx context.Context, n int)
}

func (f *fakeChat) Send(ctx context.Context, req assistant.SendRequest, emit func(assistant.Event)) error {
	f.mu.Lock()
	f.reqs = append(f.reqs, req)
	n, onSend := len(f.reqs), f.onSend
	f.mu.Unlock()
	if onSend != nil {
		onSend(ctx, n)
	}
	f.mu.Lock()
	if len(f.calls) == 0 {
		f.mu.Unlock()
		return nil
	}
	call := f.calls[0]
	if len(f.calls) > 1 {
		f.calls = f.calls[1:]
	}
	f.mu.Unlock()
	for _, e := range call.events {
		emit(e)
	}
	return call.err
}

type transcription struct {
	audio    string
	filename string
}

type fakeVoice struct {
	mu        sync.Mutex
	available bool
	text      string
	err       error
	got       []transcription
}

func (f *fakeVoice) Available(context.Context) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.available
}

func (f *fakeVoice) Transcribe(_ context.Context, audio []byte, filename string) (assistant.Transcript, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.got = append(f.got, transcription{audio: string(audio), filename: filename})
	return assistant.Transcript{Text: f.text, Seconds: 2}, f.err
}

func (f *fakeVoice) calls() []transcription {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]transcription(nil), f.got...)
}

type fakeData struct {
	snap        Snapshot
	reminders   []domain.Reminder
	events      []domain.Event
	completed   []string
	completeErr error
	onSnapshot  func()
	onReminders func()
}

func (f *fakeData) Snapshot(_ context.Context, now time.Time) Snapshot {
	if f.onSnapshot != nil {
		f.onSnapshot()
	}
	s := f.snap
	s.Now = now
	return s
}

func (f *fakeData) Reminders(context.Context) ([]domain.Reminder, error) {
	if f.onReminders != nil {
		f.onReminders()
	}
	return f.reminders, nil
}

func (f *fakeData) Events(context.Context, time.Time, time.Time) ([]domain.Event, error) {
	return f.events, nil
}

func (f *fakeData) CompleteReminder(_ context.Context, id string) (domain.Reminder, error) {
	f.completed = append(f.completed, id)
	if f.completeErr != nil {
		return domain.Reminder{}, f.completeErr
	}
	return domain.Reminder{ID: id, Title: "Pagar luz", Done: true}, nil
}

type testBot struct {
	*Bot
	api   *fakeAPI
	store *fakeStore
	chat  *fakeChat
	data  *fakeData
	tools *fakeToolBox
	voice *fakeVoice
	now   time.Time
}

// newTestBot builds a bot over fakes at Tuesday 2026-09-15 07:00 in São
// Paulo; tweak adjusts the config before New.
func newTestBot(t *testing.T, tweak func(*Config)) *testBot {
	t.Helper()
	tb := &testBot{
		api:   &fakeAPI{me: User{ID: 1, Username: "estus_bot"}},
		store: newFakeStore(),
		chat:  &fakeChat{},
		data:  &fakeData{},
		tools: &fakeToolBox{tools: testTools()},
		voice: &fakeVoice{available: true, text: "gastei 30 reais de uber"},
		now:   time.Date(2026, time.September, 15, 7, 0, 0, 0, saoPaulo(t)),
	}
	key := [32]byte{1, 2, 3}
	cfg := Config{
		Store: tb.store, Chat: tb.chat, Data: tb.data, Location: saoPaulo(t), VaultKey: &key,
		Voice:  tb.voice,
		Tools:  tb.tools,
		NewAPI: func(string) API { return tb.api },
		Now:    func() time.Time { return tb.now },
	}
	if tweak != nil {
		tweak(&cfg)
	}
	tb.Bot = New(cfg)
	return tb
}

func ptr[T any](v T) *T { return &v }

// handleAll handles u as the poller does, then answers whatever it queued for
// the assistant, as the worker would.
func (tb *testBot) handleAll(ctx context.Context, u Update) {
	tb.handle(ctx, tb.api, u)
	for {
		select {
		case job := <-tb.asks:
			tb.answer(ctx, job)
		default:
			return
		}
	}
}

// start runs the bot until the test ends and waits for Run to return.
func (tb *testBot) start(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		tb.Run(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("Run did not stop")
		}
	})
}

// eventually fails the test unless cond turns true within two seconds.
func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func textUpdateID(id int64, chatID int64, text string) Update {
	u := textUpdate(chatID, "private", text)
	u.UpdateID = id
	return u
}

// withToken saves a token through Update, as the settings screen would.
func (tb *testBot) withToken(t *testing.T) {
	t.Helper()
	if err := tb.Update(context.Background(), SettingsInput{Token: ptr("123:abc")}); err != nil {
		t.Fatalf("save token: %v", err)
	}
}

// paired saves a token and pairs chat 99, owned by Rafa.
func (tb *testBot) paired(t *testing.T) {
	t.Helper()
	tb.withToken(t)
	s := tb.store.settings()
	s.ChatID, s.OwnerName = ptr(int64(99)), "Rafa"
	_ = tb.store.SaveSettings(context.Background(), s)
}
