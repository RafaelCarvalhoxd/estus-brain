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
		AllDay   bool   `json:"dia_inteiro,omitempty"`
		Location string `json:"local,omitempty"`
		Notes    string `json:"notas,omitempty"`
	}
	toRow := func(e domain.Event) row {
		loc := r.deps.Location
		return row{e.ID, e.Title, e.StartsAt.In(loc).Format("2006-01-02 15:04"), e.EndsAt.In(loc).Format("2006-01-02 15:04"), isAllDay(e, loc), e.Location, e.Notes}
	}

	r.add(Tool{
		Name: "agenda_list", Title: "Compromissos", Module: "agenda", ReadOnly: true,
		Description: "Lista compromissos entre duas datas (padrão: de hoje até 7 dias à frente), com id para editar ou excluir.",
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
		Description: "Marca um compromisso na agenda. Horários são opcionais: sem nenhum, ocupa o dia inteiro; só com início, dura 1 hora (ou duration_minutes); só com fim, começa 1 hora antes.",
		Input: object(map[string]any{
			"title":            str("O compromisso"),
			"date":             str("Dia AAAA-MM-DD ou hoje/amanhã; padrão hoje"),
			"start":            str("Hora de início HH:MM (ou AAAA-MM-DDTHH:MM); opcional"),
			"end":              str("Hora de fim HH:MM; opcional"),
			"duration_minutes": integer("Duração quando não há fim; padrão 60"),
			"location":         str("Local"),
			"notes":            str("Observações"),
		}, "title"),
		run: typed(func(ctx context.Context, in struct {
			Title    string `json:"title"`
			Date     string `json:"date"`
			Start    string `json:"start"`
			End      string `json:"end"`
			Duration int    `json:"duration_minutes"`
			Location string `json:"location"`
			Notes    string `json:"notes"`
		}) (any, error) {
			date, start := splitEventDate(in.Date, in.Start)
			day, err := r.parseDay(date)
			if err != nil {
				return nil, err
			}
			startsAt, endsAt, err := eventRange(day, start, in.End, in.Duration, r.deps.Location)
			if err != nil {
				return nil, err
			}
			e, err := d.Events.Create(ctx, service.NewEventInput{
				Title:    strings.TrimSpace(in.Title),
				Location: strings.TrimSpace(in.Location),
				Notes:    strings.TrimSpace(in.Notes),
				StartsAt: startsAt,
				EndsAt:   endsAt,
			})
			if err != nil {
				return nil, err
			}
			return toRow(e), nil
		}),
	})

	r.add(Tool{
		Name: "agenda_update", Title: "Editar compromisso", Module: "agenda",
		Description: "Altera um compromisso pelo id (de agenda_list). Só muda o que for enviado. Trocar só o início mantém a duração; mudar só o dia mantém os horários.",
		Input: object(map[string]any{
			"id":       str("Id do compromisso"),
			"title":    str("Novo título"),
			"date":     str("Novo dia AAAA-MM-DD ou hoje/amanhã"),
			"start":    str("Nova hora de início HH:MM (ou AAAA-MM-DDTHH:MM)"),
			"end":      str("Nova hora de fim HH:MM"),
			"all_day":  boolean("true para ocupar o dia inteiro (00:00–23:59)"),
			"location": str("Novo local; \"\" apaga"),
			"notes":    str("Novas observações; \"\" apaga"),
		}, "id"),
		run: typed(func(ctx context.Context, in struct {
			ID       string  `json:"id"`
			Title    *string `json:"title"`
			Date     string  `json:"date"`
			Start    string  `json:"start"`
			End      string  `json:"end"`
			AllDay   bool    `json:"all_day"`
			Location *string `json:"location"`
			Notes    *string `json:"notes"`
		}) (any, error) {
			e, err := r.findEvent(ctx, strings.TrimSpace(in.ID))
			if err != nil {
				return nil, err
			}
			var day *time.Time
			date, start := splitEventDate(in.Date, in.Start)
			if date != "" {
				parsed, err := r.parseDay(date)
				if err != nil {
					return nil, err
				}
				day = &parsed
			}
			startsAt, endsAt, err := moveEvent(e, day, start, in.End, in.AllDay, r.deps.Location)
			if err != nil {
				return nil, err
			}
			input := service.NewEventInput{Title: e.Title, Location: e.Location, Notes: e.Notes, StartsAt: startsAt, EndsAt: endsAt}
			if in.Title != nil {
				input.Title = strings.TrimSpace(*in.Title)
			}
			if in.Location != nil {
				input.Location = strings.TrimSpace(*in.Location)
			}
			if in.Notes != nil {
				input.Notes = strings.TrimSpace(*in.Notes)
			}
			updated, err := d.Events.Update(ctx, e.ID, input)
			if err != nil {
				return nil, err
			}
			return toRow(updated), nil
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

// findEvent looks the id up in a wide window: the service has no get-by-id,
// only range queries.
func (r *Registry) findEvent(ctx context.Context, id string) (domain.Event, error) {
	now := r.now()
	events, err := r.deps.Events.ListRange(ctx, now.AddDate(-2, 0, 0), now.AddDate(3, 0, 0))
	if err != nil {
		return domain.Event{}, err
	}
	for _, e := range events {
		if e.ID == id {
			return e, nil
		}
	}
	return domain.Event{}, invalid("compromisso %q não encontrado; use o id de agenda_list", id)
}

// splitEventDate unglues "AAAA-MM-DDTHH:MM" sent as either the date or the
// start: models (and the chat's datetime field) often send it that way.
func splitEventDate(date, start string) (string, string) {
	date, start = strings.TrimSpace(date), strings.TrimSpace(start)
	glued := func(s string) bool { return len(s) > 11 && (s[10] == 'T' || s[10] == ' ') }
	if glued(start) {
		if date == "" {
			date = start[:10]
		}
		start = start[11:]
	}
	if glued(date) {
		if start == "" {
			start = date[11:]
		}
		date = date[:10]
	}
	return date, start
}

// eventRange applies the web form's rule for optional times on a calendar
// day: none is the whole day, one alone gets an hour (or minutes, when
// positive), clamped to the day.
func eventRange(day time.Time, start, end string, minutes int, loc *time.Location) (time.Time, time.Time, error) {
	if minutes <= 0 {
		minutes = 60
	}
	startMin, hasStart, err := parseClock(start)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	endMin, hasEnd, err := parseClock(end)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	switch {
	case !hasStart && !hasEnd:
		startMin, endMin = 0, lastMinute
	case !hasEnd:
		endMin = min(startMin+minutes, lastMinute)
	case !hasStart:
		startMin = max(endMin-minutes, 0)
	}
	return clockRange(day, startMin, endMin, loc)
}

// moveEvent applies an edit to an existing event's times. Without a new day
// the event keeps its own; a new start alone keeps the duration.
func moveEvent(e domain.Event, day *time.Time, start, end string, allDay bool, loc *time.Location) (time.Time, time.Time, error) {
	oldStart, oldEnd := e.StartsAt.In(loc), e.EndsAt.In(loc)
	target := time.Date(oldStart.Year(), oldStart.Month(), oldStart.Day(), 0, 0, 0, 0, time.UTC)
	if day != nil {
		target = *day
	}
	if allDay {
		return clockRange(target, 0, lastMinute, loc)
	}
	startMin, hasStart, err := parseClock(start)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	endMin, hasEnd, err := parseClock(end)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	if !hasStart {
		startMin = oldStart.Hour()*60 + oldStart.Minute()
	}
	startsAt := time.Date(target.Year(), target.Month(), target.Day(), startMin/60, startMin%60, 0, 0, loc)
	if hasEnd {
		return clockRange(target, startMin, endMin, loc)
	}
	endsAt := startsAt.Add(oldEnd.Sub(oldStart))
	if hasStart {
		// A shifted start must not spill the event into the next day.
		dayEnd := time.Date(target.Year(), target.Month(), target.Day(), 23, 59, 0, 0, loc)
		if oldEnd.Sub(oldStart) < 24*time.Hour && endsAt.After(dayEnd) {
			endsAt = dayEnd
		}
	}
	if !endsAt.After(startsAt) {
		return time.Time{}, time.Time{}, invalid("o horário de fim deve ser depois do início")
	}
	return startsAt, endsAt, nil
}

const lastMinute = 23*60 + 59

func clockRange(day time.Time, startMin, endMin int, loc *time.Location) (time.Time, time.Time, error) {
	if endMin <= startMin {
		return time.Time{}, time.Time{}, invalid("o horário de fim deve ser depois do início")
	}
	at := func(m int) time.Time {
		return time.Date(day.Year(), day.Month(), day.Day(), m/60, m%60, 0, 0, loc)
	}
	return at(startMin), at(endMin), nil
}

// parseClock reads "HH:MM" (also "9h", "9h30") as minutes since midnight;
// empty reports false.
func parseClock(s string) (int, bool, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return 0, false, nil
	}
	for _, layout := range []string{"15:04", "15:04:05", "15h04", "15h"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Hour()*60 + t.Minute(), true, nil
		}
	}
	return 0, false, invalid("hora %q inválida, use HH:MM", s)
}

func isAllDay(e domain.Event, loc *time.Location) bool {
	s, en := e.StartsAt.In(loc), e.EndsAt.In(loc)
	return s.Hour() == 0 && s.Minute() == 0 && en.Hour() == 23 && en.Minute() == 59 && s.Format(dayLayout) == en.Format(dayLayout)
}
