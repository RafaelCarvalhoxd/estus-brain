package assistant

import (
	"context"
	"strings"
	"time"

	"github.com/rafael/estus-vault/backend/internal/domain"
	"github.com/rafael/estus-vault/backend/internal/service"
)

func (r *Registry) addReminders() {
	d := r.deps
	if d.Reminders == nil {
		return
	}

	type row struct {
		ID    string `json:"id"`
		Title string `json:"titulo"`
		When  string `json:"quando,omitempty"`
		Done  bool   `json:"feito"`
		Late  bool   `json:"atrasado,omitempty"`
	}
	toRow := func(rem domain.Reminder) row {
		out := row{ID: rem.ID, Title: rem.Title, Done: rem.Done}
		if rem.DueAt != nil {
			out.When = rem.DueAt.In(r.deps.Location).Format("2006-01-02 15:04")
			out.Late = !rem.Done && rem.DueAt.Before(r.now())
		}
		return out
	}

	r.add(Tool{
		Name: "reminders_list", Title: "Lembretes", Module: "lembretes", ReadOnly: true,
		Description: "Lista lembretes: pendentes, só os de hoje, atrasados, feitos ou todos.",
		Input:       object(map[string]any{"filter": enum("padrão pendentes", "pendentes", "hoje", "atrasados", "feitos", "todos")}),
		run: typed(func(ctx context.Context, in struct {
			Filter string `json:"filter"`
		}) (any, error) {
			all, err := d.Reminders.List(ctx)
			if err != nil {
				return nil, err
			}
			filter := normalize(in.Filter)
			if filter == "" {
				filter = "pendentes"
			}
			now := r.now()
			endOfDay := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, r.deps.Location)
			out := []row{}
			for _, rem := range all {
				switch filter {
				case "pendentes":
					if rem.Done {
						continue
					}
				case "hoje":
					if rem.Done || rem.DueAt == nil || rem.DueAt.After(endOfDay) {
						continue
					}
				case "atrasados":
					if rem.Done || rem.DueAt == nil || !rem.DueAt.Before(now) {
						continue
					}
				case "feitos":
					if !rem.Done {
						continue
					}
				}
				out = append(out, toRow(rem))
			}
			return map[string]any{"lembretes": out}, nil
		}),
	})

	r.add(Tool{
		Name: "reminders_create", Title: "Novo lembrete", Module: "lembretes",
		Description: "Cria um lembrete, com data e hora opcionais.",
		Input: object(map[string]any{
			"title": str("Do que lembrar"),
			"when":  str("Quando: AAAA-MM-DDTHH:MM, ou só AAAA-MM-DD (vira 09:00); vazio = sem data"),
		}, "title"),
		run: typed(func(ctx context.Context, in struct {
			Title string `json:"title"`
			When  string `json:"when"`
		}) (any, error) {
			input := service.NewReminderInput{Title: strings.TrimSpace(in.Title)}
			if w := strings.TrimSpace(in.When); w != "" {
				var at time.Time
				var err error
				if len(w) <= 10 {
					day, derr := r.parseDay(w)
					if derr != nil {
						return nil, derr
					}
					at = time.Date(day.Year(), day.Month(), day.Day(), 9, 0, 0, 0, r.deps.Location)
				} else if at, err = r.parseLocalTime(w); err != nil {
					return nil, err
				}
				input.DueAt = &at
			}
			rem, err := d.Reminders.Create(ctx, input)
			if err != nil {
				return nil, err
			}
			return toRow(rem), nil
		}),
	})

	r.add(Tool{
		Name: "reminders_set_done", Title: "Concluir lembrete", Module: "lembretes",
		Description: "Marca um lembrete como feito (ou desfaz), pelo id.",
		Input: object(map[string]any{
			"id":   str("Id do lembrete"),
			"done": boolean("true para feito; padrão true"),
		}, "id"),
		run: typed(func(ctx context.Context, in struct {
			ID   string `json:"id"`
			Done *bool  `json:"done"`
		}) (any, error) {
			done := in.Done == nil || *in.Done
			rem, err := d.Reminders.SetDone(ctx, in.ID, done)
			if err != nil {
				return nil, err
			}
			return toRow(rem), nil
		}),
	})

	r.add(Tool{
		Name: "reminders_delete", Title: "Excluir lembrete", Module: "lembretes", Destructive: true,
		Description: "Exclui um lembrete pelo id.",
		Input:       object(map[string]any{"id": str("Id do lembrete")}, "id"),
		run: typed(func(ctx context.Context, in struct {
			ID string `json:"id"`
		}) (any, error) {
			return map[string]any{"ok": true}, d.Reminders.Delete(ctx, in.ID)
		}),
	})
}
