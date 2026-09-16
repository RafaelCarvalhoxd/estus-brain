package assistant

import (
	"context"
	"strings"
	"time"

	"github.com/rafael/estus-vault/backend/internal/domain"
	"github.com/rafael/estus-vault/backend/internal/service"
)

func (r *Registry) addAgenda() {
	d := r.deps
	if d.Events == nil {
		return
	}
	type row struct {
		ID       string `json:"id"`
		Title    string `json:"titulo"`
		Start    string `json:"inicio"`
		End      string `json:"fim"`
		Location string `json:"local,omitempty"`
	}
	toRow := func(e domain.Event) row {
		return row{e.ID, e.Title, e.StartsAt.In(r.deps.Location).Format("2006-01-02 15:04"), e.EndsAt.In(r.deps.Location).Format("2006-01-02 15:04"), e.Location}
	}

	r.add(Tool{
		Name: "agenda_list", Title: "Compromissos", Module: "agenda", ReadOnly: true,
		Description: "Lista compromissos entre duas datas (padrão: de hoje até 7 dias à frente).",
		Input: object(map[string]any{
			"from": str("Início AAAA-MM-DD; padrão hoje"),
			"days": integer("Quantos dias a partir do início; padrão 7, máximo 92"),
		}),
		run: typed(func(ctx context.Context, in struct {
			From string `json:"from"`
			Days int    `json:"days"`
		}) (any, error) {
			day, err := r.parseDay(in.From)
			if err != nil {
				return nil, err
			}
			days := in.Days
			if days <= 0 {
				days = 7
			}
			days = min(days, 92)
			from := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, r.deps.Location)
			events, err := d.Events.ListRange(ctx, from, from.AddDate(0, 0, days))
			if err != nil {
				return nil, err
			}
			out := make([]row, len(events))
			for i, e := range events {
				out[i] = toRow(e)
			}
			return map[string]any{"de": from.Format(dayLayout), "ate": from.AddDate(0, 0, days-1).Format(dayLayout), "compromissos": out}, nil
		}),
	})

	r.add(Tool{
		Name: "agenda_create", Title: "Novo compromisso", Module: "agenda",
		Description: "Marca um compromisso na agenda.",
		Input: object(map[string]any{
			"title":            str("O compromisso"),
			"start":            str("Início AAAA-MM-DDTHH:MM"),
			"duration_minutes": integer("Duração em minutos; padrão 60"),
			"location":         str("Local"),
			"notes":            str("Observações"),
		}, "title", "start"),
		run: typed(func(ctx context.Context, in struct {
			Title    string `json:"title"`
			Start    string `json:"start"`
			Duration int    `json:"duration_minutes"`
			Location string `json:"location"`
			Notes    string `json:"notes"`
		}) (any, error) {
			start, err := r.parseLocalTime(in.Start)
			if err != nil {
				return nil, err
			}
			minutes := in.Duration
			if minutes <= 0 {
				minutes = 60
			}
			e, err := d.Events.Create(ctx, service.NewEventInput{
				Title:    strings.TrimSpace(in.Title),
				Location: strings.TrimSpace(in.Location),
				Notes:    strings.TrimSpace(in.Notes),
				StartsAt: start,
				EndsAt:   start.Add(time.Duration(minutes) * time.Minute),
			})
			if err != nil {
				return nil, err
			}
			return toRow(e), nil
		}),
	})

	r.add(Tool{
		Name: "agenda_delete", Title: "Excluir compromisso", Module: "agenda", Destructive: true,
		Description: "Exclui um compromisso pelo id.",
		Input:       object(map[string]any{"id": str("Id do compromisso")}, "id"),
		run: typed(func(ctx context.Context, in struct {
			ID string `json:"id"`
		}) (any, error) {
			return map[string]any{"ok": true}, d.Events.Delete(ctx, in.ID)
		}),
	})
}
