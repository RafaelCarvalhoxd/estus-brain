package telegram

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/rafael/estus-vault/backend/internal/assistant"
)

// buttonTexts is every button of the last message sent, row by row, flattened.
func buttonTexts(t *testing.T, tb *testBot) []string {
	t.Helper()
	msgs := tb.api.messages()
	if len(msgs) == 0 {
		t.Fatal("nothing was sent")
	}
	var out []string
	for _, row := range msgs[len(msgs)-1].Keyboard {
		for _, b := range row {
			out = append(out, b.Text)
		}
	}
	return out
}

// tap presses a menu button as Telegram would.
func tap(tb *testBot, data string) Update {
	return Update{CallbackQuery: &CallbackQuery{
		ID:      "cb",
		Data:    data,
		Message: &Message{MessageID: 9, Chat: Chat{ID: 99, Type: "private"}},
	}}
}

func TestMenuHomeShowsFavouritesAndModules(t *testing.T) {
	tb := newTestBot(t, nil)
	tb.paired(t)
	tb.handleAll(context.Background(), textUpdate(99, "private", "/acoes"))

	texts := strings.Join(buttonTexts(t, tb), "|")
	for _, want := range []string{"Lançar gasto", "Resumo do mês", "Financeiro", "Lembretes", "Hábitos"} {
		if !strings.Contains(texts, want) {
			t.Errorf("home menu missing %q; got %s", want, texts)
		}
	}
}

func TestMenuModuleListsItsActions(t *testing.T) {
	tb := newTestBot(t, nil)
	tb.paired(t)
	tb.handleAll(context.Background(), tap(tb, "a:m:financeiro"))

	texts := strings.Join(buttonTexts(t, tb), "|")
	if !strings.Contains(texts, "Lançar gasto") || !strings.Contains(texts, "Voltar") {
		t.Errorf("module menu = %s", texts)
	}
	// A tool whose required input is a list can't be filled in by questions.
	if strings.Contains(texts, "Lançar vários gastos") {
		t.Errorf("a list-input tool must stay out of the menu: %s", texts)
	}
}

func TestMenuRunsAnActionWithoutFields(t *testing.T) {
	tb := newTestBot(t, nil)
	tb.paired(t)
	tb.tools.result = map[string]any{"total": "R$ 1.645,00", "mes": "2026-09"}
	tb.handleAll(context.Background(), tap(tb, "a:t:finance_month_summary"))

	if got := tb.tools.called(); len(got) != 1 || got[0].name != "finance_month_summary" {
		t.Fatalf("calls = %+v", got)
	}
	if reply := lastText(t, tb); !strings.Contains(reply, "R$ 1.645,00") {
		t.Fatalf("reply = %q", reply)
	}
}

func TestMenuAsksForEachRequiredField(t *testing.T) {
	tb := newTestBot(t, nil)
	tb.paired(t)
	ctx := context.Background()
	tb.tools.result = map[string]any{"descricao": "Mercado", "valor": "R$ 45,00", "categoria": "Alimentação"}

	tb.handleAll(ctx, tap(tb, "a:t:finance_create_transaction"))
	if reply := lastText(t, tb); !strings.Contains(reply, "O que foi") {
		t.Fatalf("first question = %q", reply)
	}
	tb.handleAll(ctx, textUpdate(99, "private", "Mercado"))
	if reply := lastText(t, tb); !strings.Contains(reply, "Valor") {
		t.Fatalf("second question = %q", reply)
	}
	if len(tb.tools.called()) != 0 {
		t.Fatal("the tool ran before every question was answered")
	}
	tb.handleAll(ctx, textUpdate(99, "private", "45,90"))

	calls := tb.tools.called()
	if len(calls) != 1 {
		t.Fatalf("calls = %+v", calls)
	}
	args := string(calls[0].args)
	if !strings.Contains(args, `"description":"Mercado"`) || !strings.Contains(args, `"amount":45.9`) {
		t.Fatalf("args = %s", args)
	}
	if reply := lastText(t, tb); !strings.Contains(reply, "Alimentação") {
		t.Fatalf("result = %q", reply)
	}
	// The assistant never saw any of it.
	if len(tb.chat.reqs) != 0 {
		t.Fatal("menu answers must not reach the assistant")
	}
}

func TestMenuRejectsABadNumber(t *testing.T) {
	tb := newTestBot(t, nil)
	tb.paired(t)
	ctx := context.Background()
	tb.handleAll(ctx, tap(tb, "a:t:finance_create_transaction"))
	tb.handleAll(ctx, textUpdate(99, "private", "Mercado"))
	tb.handleAll(ctx, textUpdate(99, "private", "muita coisa"))

	if reply := lastText(t, tb); !strings.Contains(reply, "número") {
		t.Fatalf("reply = %q", reply)
	}
	if len(tb.tools.called()) != 0 {
		t.Fatal("the tool ran with a bad number")
	}
	tb.handleAll(ctx, textUpdate(99, "private", "45"))
	if len(tb.tools.called()) != 1 {
		t.Fatal("answering again should run the tool")
	}
}

func TestMenuOffersChoicesAsButtons(t *testing.T) {
	tb := newTestBot(t, nil)
	tb.paired(t)
	ctx := context.Background()
	tb.handleAll(ctx, tap(tb, "a:t:test_choice"))

	texts := strings.Join(buttonTexts(t, tb), "|")
	if !strings.Contains(texts, "pix") || !strings.Contains(texts, "credito") {
		t.Fatalf("choices = %s", texts)
	}
	tb.handleAll(ctx, tap(tb, "a:v:pix"))
	calls := tb.tools.called()
	if len(calls) != 1 || !strings.Contains(string(calls[0].args), `"method":"pix"`) {
		t.Fatalf("calls = %+v", calls)
	}
}

func TestMenuConfirmsBeforeDeleting(t *testing.T) {
	tb := newTestBot(t, nil)
	tb.paired(t)
	ctx := context.Background()
	tb.handleAll(ctx, tap(tb, "a:t:test_delete"))
	tb.handleAll(ctx, textUpdate(99, "private", "abc-123"))

	if len(tb.tools.called()) != 0 {
		t.Fatal("deleted without confirmation")
	}
	if texts := strings.Join(buttonTexts(t, tb), "|"); !strings.Contains(texts, "Confirmar") {
		t.Fatalf("buttons = %s", texts)
	}
	tb.handleAll(ctx, tap(tb, "a:confirm"))
	calls := tb.tools.called()
	if len(calls) != 1 || !strings.Contains(string(calls[0].args), `"confirm":true`) {
		t.Fatalf("calls = %+v", calls)
	}
}

func TestMenuCancel(t *testing.T) {
	tb := newTestBot(t, nil)
	tb.paired(t)
	ctx := context.Background()
	tb.handleAll(ctx, tap(tb, "a:t:finance_create_transaction"))
	tb.handleAll(ctx, textUpdate(99, "private", "/cancelar"))
	if !strings.Contains(lastText(t, tb), "Cancelei") {
		t.Fatalf("reply = %q", lastText(t, tb))
	}
	// With nothing pending, a message goes to the assistant again.
	tb.handleAll(ctx, textUpdate(99, "private", "oi"))
	if len(tb.chat.reqs) != 1 {
		t.Fatalf("requests = %+v", tb.chat.reqs)
	}
}

func TestMenuForgetsAnActionLeftOpen(t *testing.T) {
	tb := newTestBot(t, nil)
	tb.paired(t)
	ctx := context.Background()
	tb.handleAll(ctx, tap(tb, "a:t:finance_create_transaction"))
	tb.now = tb.now.Add(menuTTL + time.Minute)

	tb.handleAll(ctx, textUpdate(99, "private", "Mercado"))
	if len(tb.tools.called()) != 0 {
		t.Fatal("an expired action still ran")
	}
	if len(tb.chat.reqs) != 1 || tb.chat.reqs[0].Message != "Mercado" {
		t.Fatalf("an expired action should let the message through: %+v", tb.chat.reqs)
	}
}

func TestMenuAnswerByVoice(t *testing.T) {
	tb := newTestBot(t, nil)
	tb.paired(t)
	ctx := context.Background()
	tb.api.files = map[string][]byte{"f1": []byte("OGG")}
	tb.voice.text = "Padaria"
	tb.handleAll(ctx, tap(tb, "a:t:finance_create_transaction"))
	tb.handleAll(ctx, voiceUpdate(99, "f1", 3))

	if reply := lastText(t, tb); !strings.Contains(reply, "Valor") {
		t.Fatalf("a spoken answer should move to the next question: %q", reply)
	}
	if len(tb.chat.reqs) != 0 {
		t.Fatal("a spoken menu answer must not reach the assistant")
	}
}

func TestRenderResult(t *testing.T) {
	text := renderResult(map[string]any{
		"total":  "R$ 1.645,00",
		"vazio":  "",
		"contas": []any{map[string]any{"descricao": "Aluguel", "valor": "R$ 1.500,00"}},
	})
	for _, want := range []string{"total: R$ 1.645,00", "Aluguel", "R$ 1.500,00"} {
		if !strings.Contains(text, want) {
			t.Errorf("renderResult missing %q in:\n%s", want, text)
		}
	}
	if strings.Contains(text, "vazio") {
		t.Errorf("empty values should be left out:\n%s", text)
	}
	if got := renderResult(map[string]any{}); !strings.Contains(got, "Feito") {
		t.Errorf("an empty result should still say something: %q", got)
	}
}

// fakeToolBox stands in for the assistant's tool registry.
type fakeToolBox struct {
	tools  []assistant.Tool
	result any
	err    error
	calls  []toolCall
}

type toolCall struct {
	name string
	args []byte
}

func (f *fakeToolBox) Tools(modules ...string) []assistant.Tool {
	if len(modules) == 0 {
		return f.tools
	}
	var out []assistant.Tool
	for _, t := range f.tools {
		for _, m := range modules {
			if t.Module == m {
				out = append(out, t)
			}
		}
	}
	return out
}

func (f *fakeToolBox) Call(_ context.Context, name string, raw json.RawMessage) (any, error) {
	f.calls = append(f.calls, toolCall{name: name, args: append([]byte(nil), raw...)})
	if f.err != nil {
		return nil, f.err
	}
	if f.result == nil {
		return map[string]any{"ok": true}, nil
	}
	return f.result, nil
}

func (f *fakeToolBox) called() []toolCall { return f.calls }

// testTools mirrors the shape of the real registry closely enough to drive
// the menu: a couple of real tools, plus one per menu behaviour.
func testTools() []assistant.Tool {
	return []assistant.Tool{
		{
			Name: "finance_create_transaction", Title: "Lançar gasto", Module: "financeiro",
			Input: assistant.Schema{"type": "object", "properties": map[string]any{
				"description": map[string]any{"type": "string", "description": "O que foi, ex.: \"Mercado\""},
				"amount":      map[string]any{"type": "number", "description": "Valor total em reais"},
				"category":    map[string]any{"type": "string", "description": "Nome da categoria"},
			}, "required": []string{"description", "amount"}},
		},
		{
			Name: "finance_create_transactions", Title: "Lançar vários gastos", Module: "financeiro",
			Input: assistant.Schema{"type": "object", "properties": map[string]any{
				"items": map[string]any{"type": "array", "description": "Os gastos"},
			}, "required": []string{"items"}},
		},
		{
			Name: "finance_month_summary", Title: "Resumo do mês", Module: "financeiro", ReadOnly: true,
			Input: assistant.Schema{"type": "object", "properties": map[string]any{
				"month": map[string]any{"type": "string", "description": "Mês AAAA-MM"},
			}, "required": []string{}},
		},
		{
			Name: "reminders_create", Title: "Criar lembrete", Module: "lembretes",
			Input: assistant.Schema{"type": "object", "properties": map[string]any{
				"title": map[string]any{"type": "string", "description": "O lembrete"},
			}, "required": []string{"title"}},
		},
		{
			Name: "habits_today", Title: "Hábitos de hoje", Module: "habitos", ReadOnly: true,
			Input: assistant.Schema{"type": "object", "properties": map[string]any{}, "required": []string{}},
		},
		{
			Name: "test_choice", Title: "Escolher", Module: "financeiro",
			Input: assistant.Schema{"type": "object", "properties": map[string]any{
				"method": map[string]any{"type": "string", "description": "Forma de pagamento", "enum": []string{"pix", "debito", "credito"}},
			}, "required": []string{"method"}},
		},
		{
			Name: "test_delete", Title: "Excluir teste", Module: "financeiro", Destructive: true,
			Input: assistant.Schema{"type": "object", "properties": map[string]any{
				"id":      map[string]any{"type": "string", "description": "Id do lançamento"},
				"confirm": map[string]any{"type": "boolean", "description": "Precisa ser true"},
			}, "required": []string{"id", "confirm"}},
		},
	}
}
