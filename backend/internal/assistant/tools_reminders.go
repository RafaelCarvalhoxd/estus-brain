package assistant

import (
	"context"
	"fmt"
	"slices"
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
		// Repeat lists the weekdays it comes back on, e.g. "seg, qua".
		Repeat string `json:"repete,omitempty"`
	}
	toRow := func(rem domain.Reminder) row {
		out := row{ID: rem.ID, Title: rem.Title, Done: rem.Done}
		names := make([]string, len(rem.RepeatDays))
		for i, d := range rem.RepeatDays {
			names[i] = weekdayNames[d]
		}
		out.Repeat = strings.Join(names, ", ")
		if rem.RepeatMonthDay != 0 {
			out.Repeat = fmt.Sprintf("todo dia %d", rem.RepeatMonthDay)
		}
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
		Description: "Cria um lembrete, com data e hora opcionais. Pode repetir em dias da semana ou num dia do mês: concluir move para a próxima vez.",
		Input: object(map[string]any{
			"title":       str("Do que lembrar"),
			"when":        str("Quando: AAAA-MM-DDTHH:MM, ou só AAAA-MM-DD (vira 09:00); vazio = sem data (com repeat, vazio = hoje 09:00)"),
			"repeat":      array("Dias da semana em que repete: dom, seg, ter, qua, qui, sex, sab", str("Dia da semana")),
			"monthly_day": integer("Dia do mês em que repete (1-31), ex.: 15 para todo dia 15; não junte com repeat"),
		}, "title"),
		run: typed(func(ctx context.Context, in struct {
			Title      string   `json:"title"`
			When       string   `json:"when"`
			Repeat     []string `json:"repeat"`
			MonthlyDay int      `json:"monthly_day"`
		}) (any, error) {
			input := service.NewReminderInput{Title: strings.TrimSpace(in.Title), RepeatMonthDay: in.MonthlyDay}
			days, err := parseWeekdays(in.Repeat)
			if err != nil {
				return nil, err
			}
			input.RepeatDays = days
			if input.DueAt, err = r.parseWhen(in.When); err != nil {
				return nil, err
			}
			r.defaultRepeatDue(&input)
			rem, err := d.Reminders.Create(ctx, input)
			if err != nil {
				return nil, err
			}
			return toRow(rem), nil
		}),
	})

	r.add(Tool{
		Name: "reminders_set_done", Title: "Concluir lembrete", Module: "lembretes",
		Description: "Marca um lembrete como feito (ou desfaz), pelo id. Um lembrete que repete não fica feito: vai para a próxima data.",
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
		Name: "reminders_update", Title: "Editar lembrete", Module: "lembretes",
		Description: "Altera um lembrete pelo id. Só muda o que for enviado. Para não repetir mais, envie repeat: [] e monthly_day: 0.",
		Input: object(map[string]any{
			"id":          str("Id do lembrete"),
			"title":       str("Novo texto"),
			"when":        str("Nova data: AAAA-MM-DDTHH:MM, ou só AAAA-MM-DD (vira 09:00); \"sem data\" tira a data"),
			"repeat":      array("Novos dias da semana em que repete: dom, seg, ter, qua, qui, sex, sab; [] = não repete nos dias da semana", str("Dia da semana")),
			"monthly_day": integer("Novo dia do mês em que repete (1-31); 0 = não repete todo mês"),
		}, "id"),
		run: typed(func(ctx context.Context, in struct {
			ID         string   `json:"id"`
			Title      *string  `json:"title"`
			When       string   `json:"when"`
			Repeat     []string `json:"repeat"`
			MonthlyDay *int     `json:"monthly_day"`
		}) (any, error) {
			all, err := d.Reminders.List(ctx)
			if err != nil {
				return nil, err
			}
			id := strings.TrimSpace(in.ID)
			i := slices.IndexFunc(all, func(rem domain.Reminder) bool { return rem.ID == id })
			if i < 0 {
				return nil, invalid("lembrete %q não encontrado; use o id de reminders_list", id)
			}
			input, err := r.reminderEdit(all[i], in.Title, in.When, in.Repeat, in.MonthlyDay)
			if err != nil {
				return nil, err
			}
			rem, err := d.Reminders.Update(ctx, id, input)
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

var weekdayNames = [...]string{"dom", "seg", "ter", "qua", "qui", "sex", "sab"}

// parseWeekday reads "seg", "segunda", "Segunda-feira", "sábado"…
func parseWeekday(s string) (time.Weekday, bool) {
	n := normalize(s)
	for i, prefix := range weekdayNames {
		if strings.HasPrefix(n, prefix) {
			return time.Weekday(i), true
		}
	}
	return 0, false
}

func parseWeekdays(names []string) ([]time.Weekday, error) {
	var days []time.Weekday
	for _, name := range names {
		day, ok := parseWeekday(name)
		if !ok {
			return nil, fmt.Errorf("dia da semana desconhecido: %q (use dom, seg, ter, qua, qui, sex, sab)", name)
		}
		days = append(days, day)
	}
	return days, nil
}

// parseWhen reads a reminder's date: a bare day becomes 09:00; empty is nil.
func (r *Registry) parseWhen(s string) (*time.Time, error) {
	w := strings.TrimSpace(s)
	if w == "" {
		return nil, nil
	}
	if len(w) <= 10 {
		day, err := r.parseDay(w)
		if err != nil {
			return nil, err
		}
		at := time.Date(day.Year(), day.Month(), day.Day(), 9, 0, 0, 0, r.deps.Location)
		return &at, nil
	}
	at, err := r.parseLocalTime(w)
	if err != nil {
		return nil, err
	}
	return &at, nil
}

// defaultRepeatDue dates an undated repeating reminder today at 09:00, since
// repeating needs a date to count from.
func (r *Registry) defaultRepeatDue(in *service.NewReminderInput) {
	if in.DueAt != nil || (len(in.RepeatDays) == 0 && in.RepeatMonthDay == 0) {
		return
	}
	today := r.today()
	at := time.Date(today.Year(), today.Month(), today.Day(), 9, 0, 0, 0, r.deps.Location)
	in.DueAt = &at
}

// reminderEdit merges the sent fields over the current reminder. repeat and
// monthly_day are mutually exclusive, so setting one clears the other.
func (r *Registry) reminderEdit(cur domain.Reminder, title *string, when string, repeat []string, monthlyDay *int) (service.NewReminderInput, error) {
	in := service.NewReminderInput{Title: cur.Title, DueAt: cur.DueAt, RepeatDays: cur.RepeatDays, RepeatMonthDay: cur.RepeatMonthDay}
	if title != nil {
		in.Title = strings.TrimSpace(*title)
	}
	if repeat != nil {
		days, err := parseWeekdays(repeat)
		if err != nil {
			return in, err
		}
		in.RepeatDays = days
		if len(days) > 0 && monthlyDay == nil {
			in.RepeatMonthDay = 0
		}
	}
	if monthlyDay != nil {
		in.RepeatMonthDay = *monthlyDay
		if *monthlyDay != 0 && repeat == nil {
			in.RepeatDays = nil
		}
	}
	switch normalize(when) {
	case "":
	case "sem data", "nenhuma", "nenhum":
		in.DueAt = nil
	default:
		at, err := r.parseWhen(when)
		if err != nil {
			return in, err
		}
		in.DueAt = at
	}
	r.defaultRepeatDue(&in)
	return in, nil
}
