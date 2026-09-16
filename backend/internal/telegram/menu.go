package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rafael/estus-vault/backend/internal/assistant"
)

// The ready-made actions on Telegram: the assistant's own tools offered as
// buttons, so the bot is useful with no AI engine connected — and quicker
// with one. An action that needs details asks for them one question at a
// time; the answers are kept in memory only while the question is open.

const menuTTL = 10 * time.Minute

// favouriteTools open the menu, so the everyday ones are two taps away.
var favouriteTools = []string{
	"finance_create_transaction", "finance_month_summary", "bills_list",
	"reminders_create", "habits_today", "training_day",
}

// menuModules are the modules with actions, in the order the app shows them.
var menuModules = []struct{ key, label string }{
	{"financeiro", "💰 Financeiro"},
	{"contas", "📄 Contas"},
	{"notas", "📝 Notas"},
	{"lembretes", "🔔 Lembretes"},
	{"agenda", "📅 Agenda"},
	{"habitos", "✅ Hábitos"},
	{"treino", "🏋️ Treino"},
	{"dieta", "🥗 Dieta"},
	{"documentos", "📂 Documentos"},
	{"geral", "🧠 Geral"},
}

// menuField is one question: what to ask, and how to read the answer.
type menuField struct {
	name     string
	question string
	kind     string // string | number | integer | boolean
	choices  []string
}

// menuAction is the action being filled in, one question at a time.
type menuAction struct {
	tool       assistant.Tool
	fields     []menuField
	answers    map[string]any
	next       int
	confirming bool
	at         time.Time
}

func (b *Bot) menuEnabled() bool { return b.cfg.Tools != nil }

// action is the open action, or nil when there is none or it went stale.
func (b *Bot) action() *menuAction {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.pending == nil {
		return nil
	}
	if b.cfg.Now().Sub(b.pending.at) > menuTTL {
		b.pending = nil
		return nil
	}
	return b.pending
}

func (b *Bot) setAction(a *menuAction) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if a != nil {
		a.at = b.cfg.Now()
	}
	b.pending = a
}

// openMenu shows the first screen: the favourites, then the modules.
func (b *Bot) openMenu(ctx context.Context, api API, chatID int64) {
	b.setAction(nil)
	rows := b.homeKeyboard()
	if len(rows) == 0 {
		_ = b.sendHTML(ctx, api, chatID, "Nenhuma ação disponível por aqui.", nil)
		return
	}
	_ = b.sendKeyboard(ctx, api, chatID, "O que você quer fazer?", rows)
}

func (b *Bot) homeKeyboard() [][]Button {
	if !b.menuEnabled() {
		return nil
	}
	byName := map[string]assistant.Tool{}
	for _, t := range b.cfg.Tools.Tools() {
		byName[t.Name] = t
	}
	var rows [][]Button
	for _, name := range favouriteTools {
		if t, ok := byName[name]; ok && menuable(t) {
			rows = append(rows, []Button{{Text: "⭐ " + t.Title, CallbackData: "a:t:" + t.Name}})
		}
	}
	var row []Button
	for _, m := range menuModules {
		if len(b.menuTools(m.key)) == 0 {
			continue
		}
		row = append(row, Button{Text: m.label, CallbackData: "a:m:" + m.key})
		if len(row) == 2 {
			rows, row = append(rows, row), nil
		}
	}
	if len(row) > 0 {
		rows = append(rows, row)
	}
	return rows
}

func (b *Bot) menuTools(module string) []assistant.Tool {
	var out []assistant.Tool
	for _, t := range b.cfg.Tools.Tools(module) {
		if menuable(t) {
			out = append(out, t)
		}
	}
	return out
}

func (b *Bot) moduleKeyboard(module string) [][]Button {
	var rows [][]Button
	for _, t := range b.menuTools(module) {
		rows = append(rows, []Button{{Text: t.Title, CallbackData: "a:t:" + t.Name}})
	}
	return append(rows, []Button{{Text: "‹ Voltar", CallbackData: "a:home"}})
}

// handleMenuCallback deals with a tap on the menu; it reports whether the
// tap was one of ours.
func (b *Bot) handleMenuCallback(ctx context.Context, api API, chatID int64, cq *CallbackQuery) bool {
	data, ok := strings.CutPrefix(cq.Data, "a:")
	if !ok {
		return false
	}
	_ = api.AnswerCallbackQuery(ctx, cq.ID, "")
	messageID := int64(0)
	if cq.Message != nil {
		messageID = cq.Message.MessageID
	}
	switch {
	case !b.menuEnabled():
		_ = b.sendHTML(ctx, api, chatID, "Nenhuma ação disponível por aqui.", nil)
	case data == "home":
		b.setAction(nil)
		_ = b.editKeyboard(ctx, api, chatID, messageID, "O que você quer fazer?", b.homeKeyboard())
	case data == "cancel":
		b.setAction(nil)
		_ = b.sendHTML(ctx, api, chatID, "Cancelei.", nil)
	case data == "confirm":
		b.confirmAction(ctx, api, chatID)
	case strings.HasPrefix(data, "m:"):
		module := strings.TrimPrefix(data, "m:")
		_ = b.editKeyboard(ctx, api, chatID, messageID, esc(moduleLabel(module)), b.moduleKeyboard(module))
	case strings.HasPrefix(data, "t:"):
		b.startAction(ctx, api, chatID, strings.TrimPrefix(data, "t:"))
	case strings.HasPrefix(data, "v:"):
		b.menuAnswer(ctx, api, chatID, strings.TrimPrefix(data, "v:"))
	}
	return true
}

func moduleLabel(module string) string {
	for _, m := range menuModules {
		if m.key == module {
			return m.label
		}
	}
	return module
}

// startAction runs an action outright, or asks for what it needs first.
func (b *Bot) startAction(ctx context.Context, api API, chatID int64, name string) {
	var tool assistant.Tool
	for _, t := range b.cfg.Tools.Tools() {
		if t.Name == name {
			tool = t
			break
		}
	}
	fields, ok := fieldsFor(tool)
	if tool.Name == "" || !ok {
		_ = b.sendHTML(ctx, api, chatID, "Essa ação não está mais disponível. Mande /acoes de novo.", nil)
		return
	}
	action := &menuAction{tool: tool, fields: fields, answers: map[string]any{}}
	if len(fields) == 0 {
		b.setAction(nil)
		b.runAction(ctx, api, chatID, action)
		return
	}
	b.setAction(action)
	b.askField(ctx, api, chatID, action, "")
}

// ask puts the current question, with the choices as buttons when the field
// has a fixed set of values.
func (b *Bot) askField(ctx context.Context, api API, chatID int64, a *menuAction, problem string) {
	f := a.fields[a.next]
	text := "<b>" + esc(a.tool.Title) + "</b>\n" + esc(f.question)
	if problem != "" {
		text = esc(problem) + "\n\n" + text
	}
	if len(f.choices) > 0 {
		rows := make([][]Button, 0, len(f.choices)+1)
		for _, choice := range f.choices {
			rows = append(rows, []Button{{Text: choice, CallbackData: "a:v:" + choice}})
		}
		rows = append(rows, []Button{{Text: "Cancelar", CallbackData: "a:cancel"}})
		_ = b.sendKeyboard(ctx, api, chatID, text, rows)
		return
	}
	_ = b.sendHTML(ctx, api, chatID, text+"\n\n<i>/cancelar para sair</i>", nil)
}

// menuAnswer takes a message (or a tapped choice) as the answer to the open
// question; it reports whether there was one to answer.
func (b *Bot) menuAnswer(ctx context.Context, api API, chatID int64, text string) bool {
	a := b.action()
	if a == nil {
		return false
	}
	if a.confirming {
		_ = b.sendHTML(ctx, api, chatID, "Toque em Confirmar ou Cancelar.", nil)
		return true
	}
	field := a.fields[a.next]
	value, err := parseAnswer(field, text)
	if err != nil {
		b.askField(ctx, api, chatID, a, err.Error())
		return true
	}
	a.answers[field.name] = value
	a.next++
	b.setAction(a)
	switch {
	case a.next < len(a.fields):
		b.askField(ctx, api, chatID, a, "")
	case a.tool.Destructive:
		b.askConfirm(ctx, api, chatID, a)
	default:
		b.setAction(nil)
		b.runAction(ctx, api, chatID, a)
	}
	return true
}

func (b *Bot) askConfirm(ctx context.Context, api API, chatID int64, a *menuAction) {
	a.confirming = true
	b.setAction(a)
	var parts []string
	for _, f := range a.fields {
		parts = append(parts, fmt.Sprintf("%s: %v", f.name, a.answers[f.name]))
	}
	text := "<b>" + esc(a.tool.Title) + "</b>\n" + esc(strings.Join(parts, "\n")) + "\n\nIsto não tem volta."
	_ = b.sendKeyboard(ctx, api, chatID, text, [][]Button{{
		{Text: "Confirmar", CallbackData: "a:confirm"},
		{Text: "Cancelar", CallbackData: "a:cancel"},
	}})
}

func (b *Bot) confirmAction(ctx context.Context, api API, chatID int64) {
	a := b.action()
	if a == nil || !a.confirming {
		_ = b.sendHTML(ctx, api, chatID, "Não tem nada para confirmar. Mande /acoes.", nil)
		return
	}
	a.answers["confirm"] = true
	b.setAction(nil)
	b.runAction(ctx, api, chatID, a)
}

// runAction calls the tool and sends back what it answered.
func (b *Bot) runAction(ctx context.Context, api API, chatID int64, a *menuAction) {
	raw, err := json.Marshal(a.answers)
	if err != nil {
		_ = b.sendHTML(ctx, api, chatID, "Não consegui montar esse pedido.", nil)
		return
	}
	result, err := b.cfg.Tools.Call(ctx, a.tool.Name, raw)
	if err != nil {
		slog.Warn("telegram: menu action failed", "tool", a.tool.Name, "error", err)
		_ = b.sendHTML(ctx, api, chatID, "<b>"+esc(a.tool.Title)+"</b>\n"+esc(assistant.ToolErrorMessage(err)), nil)
		return
	}
	_ = b.sendLong(ctx, api, chatID, "<b>"+esc(a.tool.Title)+"</b>\n"+esc(renderResult(result)))
}

// fieldsFor turns a tool's schema into the questions to ask: its required
// scalar fields, in the order the tool declares them. A tool that needs a
// list or an object can't be filled in by chat, and stays out of the menu.
func fieldsFor(t assistant.Tool) ([]menuField, bool) {
	props, _ := t.Input["properties"].(map[string]any)
	var fields []menuField
	for _, name := range toStrings(t.Input["required"]) {
		if name == "confirm" {
			continue // asked as a Confirmar button instead
		}
		p, _ := props[name].(map[string]any)
		kind, _ := p["type"].(string)
		switch kind {
		case "string", "number", "integer", "boolean":
		default:
			return nil, false
		}
		question, _ := p["description"].(string)
		if question == "" {
			question = name
		}
		fields = append(fields, menuField{name: name, question: question, kind: kind, choices: toStrings(p["enum"])})
	}
	return fields, true
}

func menuable(t assistant.Tool) bool {
	_, ok := fieldsFor(t)
	return ok
}

// parseAnswer reads a typed answer, in the shape the tool's schema wants.
func parseAnswer(f menuField, text string) (any, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("Não entendi — responda em uma mensagem.")
	}
	if len(f.choices) > 0 {
		for _, choice := range f.choices {
			if strings.EqualFold(choice, text) {
				return choice, nil
			}
		}
		return nil, fmt.Errorf("Escolha uma das opções: %s.", strings.Join(f.choices, ", "))
	}
	switch f.kind {
	case "number", "integer":
		// People write money the Brazilian way: 1.234,56.
		clean := strings.ReplaceAll(strings.ReplaceAll(text, ".", ""), ",", ".")
		clean = strings.TrimPrefix(strings.Fields(clean)[0], "R$")
		value, err := strconv.ParseFloat(clean, 64)
		if err != nil {
			return nil, fmt.Errorf("Isso não parece um número. Escreva só o valor, ex.: 45,90.")
		}
		if f.kind == "integer" {
			return int(value), nil
		}
		return value, nil
	case "boolean":
		for _, yes := range []string{"sim", "s", "true", "1"} {
			if strings.EqualFold(text, yes) {
				return true, nil
			}
		}
		for _, no := range []string{"não", "nao", "n", "false", "0"} {
			if strings.EqualFold(text, no) {
				return false, nil
			}
		}
		return nil, fmt.Errorf("Responda sim ou não.")
	default:
		return text, nil
	}
}

func toStrings(v any) []string {
	switch list := v.(type) {
	case []string:
		return list
	case []any:
		out := make([]string, 0, len(list))
		for _, item := range list {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// renderResult turns a tool's answer into something readable in a chat.
func renderResult(v any) string {
	// Tool results are typed structs with Portuguese JSON names; going
	// through JSON gives one shape to walk.
	raw, err := json.Marshal(v)
	if err != nil {
		return "Feito ✅"
	}
	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return "Feito ✅"
	}
	text := strings.TrimSpace(strings.Join(renderValue(generic, 0), "\n"))
	if text == "" {
		return "Feito ✅"
	}
	return text
}

const maxRenderedItems = 20

func renderValue(v any, depth int) []string {
	switch value := v.(type) {
	case map[string]any:
		if depth > 2 {
			return nil
		}
		keys := make([]string, 0, len(value))
		for k := range value {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var lines []string
		for _, k := range keys {
			inner := renderValue(value[k], depth+1)
			switch {
			case len(inner) == 0:
			case len(inner) == 1 && !strings.HasPrefix(inner[0], "•"):
				lines = append(lines, k+": "+inner[0])
			default:
				lines = append(lines, k+":")
				lines = append(lines, inner...)
			}
		}
		return lines
	case []any:
		var lines []string
		for i, item := range value {
			if i == maxRenderedItems {
				lines = append(lines, fmt.Sprintf("… e mais %d", len(value)-maxRenderedItems))
				break
			}
			parts := renderValue(item, depth+1)
			if len(parts) == 0 {
				continue
			}
			lines = append(lines, "• "+strings.Join(parts, " · "))
		}
		return lines
	case string:
		if strings.TrimSpace(value) == "" {
			return nil
		}
		return []string{value}
	case bool:
		if !value {
			return nil
		}
		return []string{"sim"}
	case float64:
		return []string{strconv.FormatFloat(value, 'f', -1, 64)}
	case nil:
		return nil
	default:
		return []string{fmt.Sprint(value)}
	}
}
