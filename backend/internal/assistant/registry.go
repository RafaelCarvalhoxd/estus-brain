// Package assistant is everything Estus Brain can do on request, written once:
// each Tool is a named action with a JSON Schema for its input. The chat's
// ready-made flows, the MCP server and every AI engine call the same tools,
// so a "lançar gasto" behaves identically however it was asked for.
//
// The password vault is deliberately absent: secrets never reach a model.
package assistant

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/rafael/estus-vault/backend/internal/domain"
	"github.com/rafael/estus-vault/backend/internal/service"
	"github.com/rafael/estus-vault/backend/internal/store/postgres"
)

// Schema is a JSON Schema object, built with the helpers in schema.go.
type Schema = map[string]any

type Tool struct {
	Name        string `json:"name"`
	Title       string `json:"title"`
	Description string `json:"description"`
	// Module groups tools the way the app does ("financeiro", "contas", …).
	Module string `json:"module"`
	Input  Schema `json:"input_schema"`
	// ReadOnly tools never change data; Destructive ones delete it and
	// require `confirm: true` in their input.
	ReadOnly    bool `json:"read_only"`
	Destructive bool `json:"destructive"`

	run func(ctx context.Context, raw json.RawMessage) (any, error)
}

// Deps are the services the tools act through. Optional modules may be nil;
// their tools are then left out.
type Deps struct {
	Categories     *postgres.CategoryRepo
	Cards          *postgres.CreditCardRepo
	Transactions   *service.TransactionService
	TransactionLog *postgres.TransactionRepo
	Dashboard      *service.DashboardService
	Bills          *service.BillService
	// CardSpending answers "quanto tá a fatura": invoices per card.
	CardSpending   *service.CardSpendingService
	Notes          *service.NoteService
	NoteCategories *service.NoteCategoryService
	Reminders      *service.ReminderService
	Events         *service.EventService
	Habits         *service.HabitService
	Training       *service.TrainingService
	Diet           *service.DietService
	Documents      *service.DocumentService
	Boards         *service.BoardService
	// Location is the owner's time zone: "today", "this month" and bare
	// dates and times in tool input are read in it.
	Location *time.Location
	// AssistantRepo and VaultKey let generate_image read the same stored
	// OpenAI key the chat settings screen writes — the same pair
	// Chat.apiKey reads with, but the registry is built before Chat exists.
	AssistantRepo *postgres.AssistantRepo
	VaultKey      *[32]byte
}

type Registry struct {
	deps   Deps
	tools  []Tool
	byName map[string]Tool
}

// ErrUnknownTool is returned by Call for a name no tool has.
var ErrUnknownTool = errors.New("unknown tool")

func New(deps Deps) *Registry {
	if deps.Location == nil {
		deps.Location = time.Local
	}
	r := &Registry{deps: deps, byName: map[string]Tool{}}
	r.addFinance()
	r.addBills()
	r.addNotes()
	r.addReminders()
	r.addAgenda()
	r.addHabits()
	r.addHealth()
	r.addFiles()
	r.addImages()
	r.addOverview()
	sort.SliceStable(r.tools, func(i, j int) bool { return r.tools[i].Module < r.tools[j].Module })
	return r
}

func (r *Registry) add(t Tool) {
	if t.Destructive {
		props := t.Input["properties"].(map[string]any)
		props["confirm"] = boolean("Precisa ser true. Só envie depois que a pessoa confirmar a exclusão.")
		t.Input["required"] = append(t.Input["required"].([]string), "confirm")
	}
	r.tools = append(r.tools, t)
	r.byName[t.Name] = t
}

// Tools lists every tool, optionally only those of some modules.
func (r *Registry) Tools(modules ...string) []Tool {
	if len(modules) == 0 {
		return r.tools
	}
	want := map[string]bool{}
	for _, m := range modules {
		want[m] = true
	}
	var out []Tool
	for _, t := range r.tools {
		if want[t.Module] {
			out = append(out, t)
		}
	}
	return out
}

func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.byName[name]
	return t, ok
}

// Call runs a tool with raw JSON input. Input problems come back wrapped in
// domain.ErrValidation with a message meant to be shown (or read by a model).
func (r *Registry) Call(ctx context.Context, name string, raw json.RawMessage) (any, error) {
	t, ok := r.byName[name]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownTool, name)
	}
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "null" {
		raw = json.RawMessage(`{}`)
	}
	if t.Destructive {
		var c struct {
			Confirm bool `json:"confirm"`
		}
		_ = json.Unmarshal(raw, &c)
		if !c.Confirm {
			return nil, invalid("confirmação necessária: pergunte à pessoa e chame de novo com confirm=true")
		}
	}
	return t.run(ctx, raw)
}

// typed adapts a function taking a decoded input struct into a tool runner.
func typed[T any](fn func(ctx context.Context, in T) (any, error)) func(context.Context, json.RawMessage) (any, error) {
	return func(ctx context.Context, raw json.RawMessage) (any, error) {
		var in T
		if err := json.Unmarshal(raw, &in); err != nil {
			return nil, invalid("entrada inválida: %v", err)
		}
		return fn(ctx, in)
	}
}

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", domain.ErrValidation, fmt.Sprintf(format, args...))
}
