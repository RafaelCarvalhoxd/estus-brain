package telegram

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/rafael/estus-vault/backend/internal/assistant"
	"github.com/rafael/estus-vault/backend/internal/domain"
)

func textUpdate(chatID int64, chatType, text string) Update {
	return Update{Message: &Message{MessageID: 1, From: &User{ID: chatID, FirstName: "Rafa"}, Chat: Chat{ID: chatID, Type: chatType}, Text: text}}
}

func lastText(t *testing.T, tb *testBot) string {
	t.Helper()
	msgs := tb.api.messages()
	if len(msgs) == 0 {
		t.Fatal("nothing was sent")
	}
	return msgs[len(msgs)-1].Text
}

func TestPairingWithTheCode(t *testing.T) {
	tb := newTestBot(t, nil)
	ctx := context.Background()
	tb.withToken(t)
	p, err := tb.StartPairing(ctx)
	if err != nil {
		t.Fatal(err)
	}
	tb.handleAll(ctx, textUpdate(99, "private", "/start "+p.Code))
	s := tb.store.settings()
	if s.ChatID == nil || *s.ChatID != 99 || s.OwnerName != "Rafa" || s.PairingCode != "" {
		t.Fatalf("settings = %+v", s)
	}
	if !strings.Contains(lastText(t, tb), "Pareado") {
		t.Fatalf("reply = %q", lastText(t, tb))
	}
}

func TestFiveWrongCodesThrowTheCodeAway(t *testing.T) {
	tb := newTestBot(t, nil)
	ctx := context.Background()
	tb.withToken(t)
	if _, err := tb.StartPairing(ctx); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= maxPairingAttempts; i++ {
		tb.handleAll(ctx, textUpdate(77, "private", "/start 000000"))
		if !strings.Contains(lastText(t, tb), "Código inválido") {
			t.Fatalf("reply = %q", lastText(t, tb))
		}
		if code := tb.store.settings().PairingCode; (i < maxPairingAttempts) != (code != "") {
			t.Fatalf("after %d wrong tries code = %q", i, code)
		}
	}
}

func TestPairingIgnoresGroups(t *testing.T) {
	tb := newTestBot(t, nil)
	ctx := context.Background()
	tb.withToken(t)
	p, _ := tb.StartPairing(ctx)
	tb.handleAll(ctx, textUpdate(-100, "group", "/start "+p.Code))
	if tb.store.settings().ChatID != nil || len(tb.api.messages()) != 0 {
		t.Fatal("a group must not pair or get a reply")
	}
}

func TestACodeCannotBeReused(t *testing.T) {
	tb := newTestBot(t, nil)
	ctx := context.Background()
	tb.withToken(t)
	p, _ := tb.StartPairing(ctx)
	tb.handleAll(ctx, textUpdate(99, "private", "/start "+p.Code))
	tb.handleAll(ctx, textUpdate(77, "private", "/start "+p.Code))
	if s := tb.store.settings(); *s.ChatID != 99 {
		t.Fatalf("chat = %d", *s.ChatID)
	}
	for _, m := range tb.api.messages() {
		if m.ChatID == 77 {
			t.Fatalf("replied to a stranger: %+v", m)
		}
	}
}

func TestOtherChatsAreIgnoredOncePaired(t *testing.T) {
	tb := newTestBot(t, nil)
	tb.paired(t)
	tb.handleAll(context.Background(), textUpdate(77, "private", "oi"))
	if len(tb.chat.reqs) != 0 || len(tb.api.messages()) != 0 {
		t.Fatal("a stranger's message reached the assistant or got a reply")
	}
}

func TestMessagesGoToTheAssistant(t *testing.T) {
	tb := newTestBot(t, nil)
	tb.paired(t)
	tb.chat.calls = []fakeCall{{events: []assistant.Event{
		{Type: "conversation", ConversationID: "conv-1"},
		{Type: "text", Text: "**Total**: R$ 10"},
	}}}
	tb.handleAll(context.Background(), textUpdate(99, "private", "quanto gastei?"))

	if len(tb.chat.reqs) != 1 || tb.chat.reqs[0].Message != "quanto gastei?" || tb.chat.reqs[0].ConversationID != "" || tb.chat.reqs[0].Title != "Telegram · 15/09" {
		t.Fatalf("requests = %+v", tb.chat.reqs)
	}
	if s := tb.store.settings(); s.ConversationID == nil || *s.ConversationID != "conv-1" {
		t.Fatalf("conversation = %v", s.ConversationID)
	}
	msgs := tb.api.messages()
	if len(msgs) != 1 || msgs[0].Text != "<b>Total</b>: R$ 10" || !msgs[0].HTML || msgs[0].ChatID != 99 {
		t.Fatalf("sent = %+v", msgs)
	}
}

func TestTheConversationContinues(t *testing.T) {
	tb := newTestBot(t, nil)
	tb.paired(t)
	s := tb.store.settings()
	s.ConversationID = ptr("conv-1")
	_ = tb.store.SaveSettings(context.Background(), s)
	tb.handleAll(context.Background(), textUpdate(99, "private", "e ontem?"))
	if tb.chat.reqs[0].ConversationID != "conv-1" {
		t.Fatalf("request = %+v", tb.chat.reqs[0])
	}
}

func TestADeletedConversationStartsOver(t *testing.T) {
	tb := newTestBot(t, nil)
	tb.paired(t)
	s := tb.store.settings()
	s.ConversationID = ptr("gone")
	_ = tb.store.SaveSettings(context.Background(), s)
	tb.chat.calls = []fakeCall{
		{err: domain.ErrNotFound},
		{events: []assistant.Event{{Type: "conversation", ConversationID: "conv-2"}, {Type: "text", Text: "oi"}}},
	}
	tb.handleAll(context.Background(), textUpdate(99, "private", "oi"))
	if len(tb.chat.reqs) != 2 || tb.chat.reqs[1].ConversationID != "" || *tb.store.settings().ConversationID != "conv-2" {
		t.Fatalf("requests = %+v", tb.chat.reqs)
	}
	if lastText(t, tb) != "oi" {
		t.Fatalf("reply = %q", lastText(t, tb))
	}
}

func TestNoEngineSaysTheCommandsStillWork(t *testing.T) {
	tb := newTestBot(t, nil)
	tb.paired(t)
	tb.chat.calls = []fakeCall{{err: assistant.ErrNoProvider}}
	tb.handleAll(context.Background(), textUpdate(99, "private", "oi"))
	if reply := lastText(t, tb); !strings.Contains(reply, "Nenhum motor de IA") || !strings.Contains(reply, "/hoje") {
		t.Fatalf("reply = %q", reply)
	}
}

func TestAnEngineErrorIsForwarded(t *testing.T) {
	tb := newTestBot(t, nil)
	tb.paired(t)
	tb.chat.calls = []fakeCall{{events: []assistant.Event{{Type: "error", Error: "Falta a chave de API do OpenAI."}}}}
	tb.handleAll(context.Background(), textUpdate(99, "private", "oi"))
	if reply := lastText(t, tb); !strings.Contains(reply, "⚠️ Falta a chave de API do OpenAI.") {
		t.Fatalf("reply = %q", reply)
	}
}

func TestCommands(t *testing.T) {
	tb := newTestBot(t, nil)
	tb.paired(t)
	ctx := context.Background()
	tb.data.snap = Snapshot{Habits: []domain.Habit{{Name: "Ler", Weekdays: 127, Target: 1}}}

	tb.handleAll(ctx, textUpdate(99, "private", "/hoje"))
	if reply := lastText(t, tb); !strings.Contains(reply, "Bom dia") || !strings.Contains(reply, "• Ler") {
		t.Fatalf("/hoje = %q", reply)
	}
	tb.handleAll(ctx, textUpdate(99, "private", "/noite@estus_bot"))
	if reply := lastText(t, tb); !strings.Contains(reply, "Fechamento") {
		t.Fatalf("/noite = %q", reply)
	}
	tb.handleAll(ctx, textUpdate(99, "private", "/ajuda"))
	if reply := lastText(t, tb); !strings.Contains(reply, "/nova") {
		t.Fatalf("/ajuda = %q", reply)
	}
	s := tb.store.settings()
	s.ConversationID = ptr("conv-1")
	_ = tb.store.SaveSettings(ctx, s)
	tb.handleAll(ctx, textUpdate(99, "private", "/nova"))
	if tb.store.settings().ConversationID != nil || !strings.Contains(lastText(t, tb), "conversa nova") {
		t.Fatalf("/nova left %v, replied %q", tb.store.settings().ConversationID, lastText(t, tb))
	}
	if len(tb.chat.reqs) != 0 {
		t.Fatal("commands must not reach the assistant")
	}
}

func TestNonTextMessages(t *testing.T) {
	tb := newTestBot(t, nil)
	tb.paired(t)
	tb.handleAll(context.Background(), textUpdate(99, "private", ""))
	if !strings.Contains(lastText(t, tb), "só entendo texto") {
		t.Fatalf("reply = %q", lastText(t, tb))
	}
}

func TestRejectedHTMLIsSentAsPlainText(t *testing.T) {
	tb := newTestBot(t, nil)
	tb.paired(t)
	tb.api.sendErr = func(_ string, html bool) error {
		if html {
			return &APIError{Code: 400, Description: "Bad Request: can't parse entities: unexpected end tag"}
		}
		return nil
	}
	if err := tb.sendHTML(context.Background(), tb.api, 99, "<b>a &amp; b", nil); err != nil {
		t.Fatal(err)
	}
	if msgs := tb.api.messages(); len(msgs) != 1 || msgs[0].HTML || msgs[0].Text != "a & b" {
		t.Fatalf("sent = %+v", msgs)
	}
}

func TestTheDoneButtonCompletesTheReminder(t *testing.T) {
	tb := newTestBot(t, nil)
	tb.paired(t)
	cb := Update{CallbackQuery: &CallbackQuery{ID: "cb1", Data: "done:r1", Message: &Message{MessageID: 5, Chat: Chat{ID: 99, Type: "private"}}}}
	tb.handleAll(context.Background(), cb)
	if len(tb.data.completed) != 1 || tb.data.completed[0] != "r1" {
		t.Fatalf("completed = %v", tb.data.completed)
	}
	if len(tb.api.answers) != 1 || tb.api.answers[0] != "Concluído" || len(tb.api.edits) != 1 || tb.api.edits[0].Text != "✅ <s>Pagar luz</s>" || tb.api.edits[0].MessageID != 5 {
		t.Fatalf("answers = %v, edits = %+v", tb.api.answers, tb.api.edits)
	}

	tb.data.completeErr = domain.ErrNotFound
	tb.handleAll(context.Background(), cb)
	if tb.api.answers[1] != "Esse lembrete não existe mais." {
		t.Fatalf("answers = %v", tb.api.answers)
	}
}

func TestButtonsFromOtherChatsAreIgnored(t *testing.T) {
	tb := newTestBot(t, nil)
	tb.paired(t)
	tb.handleAll(context.Background(), Update{CallbackQuery: &CallbackQuery{ID: "cb1", Data: "done:r1", Message: &Message{MessageID: 5, Chat: Chat{ID: 77, Type: "private"}}}})
	if len(tb.data.completed) != 0 {
		t.Fatal("a stranger completed a reminder")
	}
}

func TestAFullQueueAsksToWait(t *testing.T) {
	tb := newTestBot(t, nil)
	tb.paired(t)
	for range askQueue {
		tb.asks <- askJob{api: tb.api, chatID: 99, text: "oi"}
	}
	tb.handle(context.Background(), tb.api, textUpdate(99, "private", "mais uma"))
	if reply := lastText(t, tb); !strings.Contains(reply, "muitas mensagens na fila") {
		t.Fatalf("reply = %q", reply)
	}
}

func TestAMessageQueuedBeforeUnpairingIsDropped(t *testing.T) {
	tb := newTestBot(t, nil)
	tb.paired(t)
	ctx := context.Background()
	tb.handle(ctx, tb.api, textUpdate(99, "private", "oi"))
	if err := tb.Unpair(ctx); err != nil {
		t.Fatal(err)
	}
	tb.answer(ctx, <-tb.asks)
	if len(tb.chat.reqs) != 0 || len(tb.api.messages()) != 0 {
		t.Fatal("answered a chat that is no longer paired")
	}
}

func voiceUpdate(chatID int64, fileID string, seconds int) Update {
	return Update{Message: &Message{
		MessageID: 2,
		From:      &User{ID: chatID, FirstName: "Rafa"},
		Chat:      Chat{ID: chatID, Type: "private"},
		Voice:     &Voice{FileID: fileID, Duration: seconds, MimeType: "audio/ogg"},
	}}
}

func TestAVoiceMessageIsTranscribedAndAnswered(t *testing.T) {
	tb := newTestBot(t, nil)
	tb.paired(t)
	tb.api.files = map[string][]byte{"f1": []byte("OGG")}
	tb.chat.calls = []fakeCall{{events: []assistant.Event{{Type: "text", Text: "Lancei R$ 30,00 em Transporte."}}}}

	tb.handleAll(context.Background(), voiceUpdate(99, "f1", 4))

	if got := tb.voice.calls(); len(got) != 1 || got[0].audio != "OGG" || got[0].filename != "audio.oga" {
		t.Fatalf("transcribed = %+v", got)
	}
	if len(tb.chat.reqs) != 1 || tb.chat.reqs[0].Message != "gastei 30 reais de uber" {
		t.Fatalf("requests = %+v", tb.chat.reqs)
	}
	msgs := tb.api.messages()
	if len(msgs) != 2 || msgs[0].Text != "🎙️ Entendi: gastei 30 reais de uber" || msgs[1].Text != "Lancei R$ 30,00 em Transporte." {
		t.Fatalf("sent = %+v", msgs)
	}
}

func TestAnAudioFileIsTranscribedToo(t *testing.T) {
	tb := newTestBot(t, nil)
	tb.paired(t)
	tb.api.files = map[string][]byte{"f2": []byte("M4A")}
	u := voiceUpdate(99, "", 0)
	u.Message.Voice = nil
	u.Message.Audio = &Audio{FileID: "f2", Duration: 10, MimeType: "audio/mp4", FileName: "nota.m4a"}
	tb.handleAll(context.Background(), u)
	if got := tb.voice.calls(); len(got) != 1 || got[0].audio != "M4A" {
		t.Fatalf("transcribed = %+v", got)
	}
}

func TestAVoiceMessageWithNothingSaid(t *testing.T) {
	tb := newTestBot(t, nil)
	tb.paired(t)
	tb.api.files = map[string][]byte{"f1": []byte("OGG")}
	tb.voice.text = ""
	tb.handleAll(context.Background(), voiceUpdate(99, "f1", 3))
	if lastText(t, tb) != "Não entendi o áudio — pode repetir?" || len(tb.chat.reqs) != 0 {
		t.Fatalf("reply = %q, requests = %d", lastText(t, tb), len(tb.chat.reqs))
	}
}

func TestATooLongVoiceMessage(t *testing.T) {
	tb := newTestBot(t, nil)
	tb.paired(t)
	tb.api.files = map[string][]byte{"f1": []byte("OGG")}
	tb.handleAll(context.Background(), voiceUpdate(99, "f1", 181))
	if lastText(t, tb) != "Áudio longo demais — mande até 3 minutos." || len(tb.voice.calls()) != 0 {
		t.Fatalf("reply = %q, transcribed = %d", lastText(t, tb), len(tb.voice.calls()))
	}
}

func TestVoiceMessagesWithoutSpeech(t *testing.T) {
	noVoice := newTestBot(t, func(c *Config) { c.Voice = nil })
	noVoice.paired(t)
	noVoice.handleAll(context.Background(), voiceUpdate(99, "f1", 3))
	if lastText(t, noVoice) != "Por enquanto só entendo texto aqui." {
		t.Fatalf("no voice: reply = %q", lastText(t, noVoice))
	}

	offline := newTestBot(t, nil)
	offline.paired(t)
	offline.voice.available = false
	offline.api.files = map[string][]byte{"f1": []byte("OGG")}
	offline.handleAll(context.Background(), voiceUpdate(99, "f1", 3))
	if lastText(t, offline) != "Por enquanto só entendo texto aqui." || len(offline.voice.calls()) != 0 {
		t.Fatalf("unavailable: reply = %q", lastText(t, offline))
	}
}

func TestVoiceMessageFailures(t *testing.T) {
	failed := newTestBot(t, nil)
	failed.paired(t)
	failed.api.downloadErr = errors.New("connection reset")
	failed.api.files = map[string][]byte{"f1": []byte("OGG")}
	failed.handleAll(context.Background(), voiceUpdate(99, "f1", 3))
	if lastText(t, failed) != "Não consegui ouvir o áudio agora. Tente de novo ou mande em texto." {
		t.Fatalf("download failure: reply = %q", lastText(t, failed))
	}

	rejected := newTestBot(t, nil)
	rejected.paired(t)
	rejected.api.files = map[string][]byte{"f1": []byte("OGG")}
	rejected.voice.err = fmt.Errorf("%w: Formato de áudio não suportado.", assistant.ErrVoiceRejected)
	rejected.handleAll(context.Background(), voiceUpdate(99, "f1", 3))
	if lastText(t, rejected) != "Formato de áudio não suportado." {
		t.Fatalf("rejected: reply = %q", lastText(t, rejected))
	}
}

func TestVoiceFromOtherChatsIsIgnored(t *testing.T) {
	tb := newTestBot(t, nil)
	tb.paired(t)
	tb.api.files = map[string][]byte{"f1": []byte("OGG")}
	tb.handleAll(context.Background(), voiceUpdate(77, "f1", 3))
	if len(tb.voice.calls()) != 0 || len(tb.api.messages()) != 0 {
		t.Fatal("a stranger's voice message was transcribed or answered")
	}
}
