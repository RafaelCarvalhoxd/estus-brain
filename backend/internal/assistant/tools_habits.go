package assistant

import (
	"context"
	"strings"

	"github.com/rafael/estus-vault/backend/internal/domain"
)

func (r *Registry) addHabits() {
	d := r.deps
	if d.Habits == nil {
		return
	}

	// find skips archived habits unless withArchived: only editing reaches them.
	find := func(ctx context.Context, query string, withArchived bool) (domain.Habit, error) {
		habits, err := d.Habits.List(ctx)
		if err != nil {
			return domain.Habit{}, err
		}
		var active []domain.Habit
		for _, h := range habits {
			if withArchived || !h.Archived {
				active = append(active, h)
			}
		}
		h, ok, names := match(active, query, func(h domain.Habit) string { return h.ID }, func(h domain.Habit) string { return h.Name })
		if !ok {
			return domain.Habit{}, invalid("hábito %q não encontrado; hábitos: %s", query, strings.Join(names, ", "))
		}
		return h, nil
	}

	r.add(Tool{
		Name: "habits_today", Title: "Hábitos de hoje", Module: "habitos", ReadOnly: true,
		Description: "Os hábitos previstos para um dia (padrão hoje): feito ou quanto falta, e a sequência de cada um.",
		Input:       object(map[string]any{"day": str("Dia AAAA-MM-DD ou hoje/ontem; padrão hoje")}),
		run: typed(func(ctx context.Context, in struct {
			Day string `json:"day"`
		}) (any, error) {
			day, err := r.parseDay(in.Day)
			if err != nil {
				return nil, err
			}
			habits, err := d.Habits.List(ctx)
			if err != nil {
				return nil, err
			}
			type row struct {
				Name     string `json:"habito"`
				Done     bool   `json:"feito"`
				Progress string `json:"progresso,omitempty"`
				Streak   int    `json:"sequencia"`
				Best     int    `json:"melhor_sequencia"`
			}
			out := []row{}
			done := 0
			for _, h := range habits {
				if h.Archived || !h.ScheduledOn(day) || day.Before(h.StartDay) {
					continue
				}
				cur, best := h.Streaks(r.today())
				item := row{Name: h.Name, Done: h.DoneOn(day), Streak: cur, Best: best}
				if h.Kind == domain.HabitCount {
					item.Progress = strings.TrimSpace(strings.Join([]string{itoa(h.Logs[day.Format(dayLayout)]), "de", itoa(h.Target), h.Unit}, " "))
				}
				if item.Done {
					done++
				}
				out = append(out, item)
			}
			return map[string]any{"dia": day.Format(dayLayout), "feitos": done, "total": len(out), "habitos": out}, nil
		}),
	})

	r.add(Tool{
		Name: "habits_log", Title: "Marcar hábito", Module: "habitos",
		Description: "Marca um hábito como feito num dia, ou registra uma quantidade (ex.: 2 copos de água). Use add para somar ao que já tem.",
		Input: object(map[string]any{
			"habit": str("Nome do hábito"),
			"day":   str("Dia AAAA-MM-DD ou hoje/ontem; padrão hoje"),
			"done":  boolean("Para hábitos de marcar: true feito, false desfaz; padrão true"),
			"count": integer("Para hábitos de contar: quantidade total do dia"),
			"add":   integer("Para hábitos de contar: quanto somar ao valor atual"),
		}, "habit"),
		run: typed(func(ctx context.Context, in struct {
			Habit string `json:"habit"`
			Day   string `json:"day"`
			Done  *bool  `json:"done"`
			Count *int   `json:"count"`
			Add   *int   `json:"add"`
		}) (any, error) {
			h, err := find(ctx, in.Habit, false)
			if err != nil {
				return nil, err
			}
			day, err := r.parseDay(in.Day)
			if err != nil {
				return nil, err
			}
			if day.After(r.today()) {
				return nil, invalid("não dá para marcar um dia que ainda não chegou")
			}
			current := h.Logs[day.Format(dayLayout)]
			next := h.Target
			switch {
			case in.Add != nil:
				next = max(0, current+*in.Add)
			case in.Count != nil:
				next = max(0, *in.Count)
			case in.Done != nil && !*in.Done:
				next = 0
			}
			if err := d.Habits.SetLog(ctx, h.ID, day, next); err != nil {
				return nil, err
			}
			return map[string]any{"habito": h.Name, "dia": day.Format(dayLayout), "valor": next, "meta": h.Target, "feito": next >= h.Target}, nil
		}),
	})

	r.add(Tool{
		Name: "habits_create", Title: "Novo hábito", Module: "habitos",
		Description: "Cria um hábito de marcar (feito ou não) ou de contar até uma meta diária.",
		Input: object(map[string]any{
			"name":   str("Nome, ex.: \"Beber água\""),
			"kind":   enum("check (feito ou não) ou count (contar até a meta); padrão check", "check", "count"),
			"target": integer("Meta diária, só para count"),
			"unit":   str("Unidade da meta, ex.: copos"),
			"days":   array("Dias da semana, 0=domingo … 6=sábado; padrão todos", integer("Dia da semana")),
		}, "name"),
		run: typed(func(ctx context.Context, in struct {
			Name   string `json:"name"`
			Kind   string `json:"kind"`
			Target int    `json:"target"`
			Unit   string `json:"unit"`
			Days   []int  `json:"days"`
		}) (any, error) {
			days := in.Days
			if len(days) == 0 {
				days = []int{0, 1, 2, 3, 4, 5, 6}
			}
			mask, err := domain.DaysToMask(days)
			if err != nil {
				return nil, err
			}
			kind := domain.HabitCheck
			if normalize(in.Kind) == "count" {
				kind = domain.HabitCount
			}
			h, err := d.Habits.Create(ctx, domain.Habit{Name: in.Name, Kind: kind, Target: in.Target, Unit: in.Unit, Weekdays: mask, Color: "#e2c23a", StartDay: r.today()})
			if err != nil {
				return nil, err
			}
			return map[string]any{"habito": h.Name, "tipo": string(h.Kind), "meta": h.Target}, nil
		}),
	})
	r.add(Tool{
		Name: "habits_list", Title: "Todos os hábitos", Module: "habitos", ReadOnly: true,
		Description: "Lista todos os hábitos, inclusive os arquivados, com tipo, meta, dias e cor.",
		Input:       object(map[string]any{}),
		run: typed(func(ctx context.Context, _ struct{}) (any, error) {
			habits, err := d.Habits.List(ctx)
			if err != nil {
				return nil, err
			}
			out := make([]map[string]any, len(habits))
			for i, h := range habits {
				out[i] = habitRow(h)
			}
			return map[string]any{"habitos": out}, nil
		}),
	})

	r.add(Tool{
		Name: "habits_update", Title: "Editar hábito", Module: "habitos",
		Description: "Altera um hábito (nome ou id): nome, tipo, meta, unidade, dias, cor, ou arquiva/desarquiva. Só muda o que for enviado.",
		Input: object(map[string]any{
			"habit":    str("Nome ou id do hábito"),
			"name":     str("Novo nome"),
			"kind":     enum("check (feito ou não) ou count (contar até a meta)", "check", "count"),
			"target":   integer("Nova meta diária, só para count (1-1000)"),
			"unit":     str("Nova unidade da meta, ex.: copos"),
			"days":     array("Novos dias da semana, 0=domingo … 6=sábado", integer("Dia da semana")),
			"color":    str("Nova cor #rrggbb"),
			"archived": boolean("true arquiva (some de hoje, guarda o histórico); false desarquiva"),
		}, "habit"),
		run: typed(func(ctx context.Context, in struct {
			Habit    string  `json:"habit"`
			Name     *string `json:"name"`
			Kind     *string `json:"kind"`
			Target   *int    `json:"target"`
			Unit     *string `json:"unit"`
			Days     []int   `json:"days"`
			Color    *string `json:"color"`
			Archived *bool   `json:"archived"`
		}) (any, error) {
			h, err := find(ctx, in.Habit, true)
			if err != nil {
				return nil, err
			}
			next, err := habitEdit(h, in.Name, in.Kind, in.Unit, in.Color, in.Target, in.Days, in.Archived)
			if err != nil {
				return nil, err
			}
			// Validate first: it normalizes (a check habit's target becomes 1) what we echo back.
			if err := next.Validate(); err != nil {
				return nil, err
			}
			if err := d.Habits.Update(ctx, h.ID, next); err != nil {
				return nil, err
			}
			return habitRow(next), nil
		}),
	})

	r.add(Tool{
		Name: "habits_delete", Title: "Excluir hábito", Module: "habitos", Destructive: true,
		Description: "Exclui um hábito e todo o seu histórico. Para só tirar da rotina, prefira arquivar com habits_update.",
		Input:       object(map[string]any{"habit": str("Nome ou id do hábito")}, "habit"),
		run: typed(func(ctx context.Context, in struct {
			Habit string `json:"habit"`
		}) (any, error) {
			h, err := find(ctx, in.Habit, true)
			if err != nil {
				return nil, err
			}
			if err := d.Habits.Delete(ctx, h.ID); err != nil {
				return nil, err
			}
			return map[string]any{"ok": true, "habito": h.Name}, nil
		}),
	})
}

func habitRow(h domain.Habit) map[string]any {
	days := domain.MaskToDays(h.Weekdays)
	names := make([]string, len(days))
	for i, d := range days {
		names[i] = weekdayNames[d]
	}
	out := map[string]any{"id": h.ID, "habito": h.Name, "tipo": string(h.Kind), "dias": strings.Join(names, ", "), "cor": h.Color}
	if h.Kind == domain.HabitCount {
		out["meta"] = h.Target
		out["unidade"] = h.Unit
	}
	if h.Archived {
		out["arquivado"] = true
	}
	return out
}

// habitEdit merges the sent fields over the current habit.
func habitEdit(h domain.Habit, name, kind, unit, color *string, target *int, days []int, archived *bool) (domain.Habit, error) {
	if name != nil {
		h.Name = *name
	}
	if kind != nil {
		switch normalize(*kind) {
		case "check":
			h.Kind = domain.HabitCheck
		case "count":
			h.Kind = domain.HabitCount
		default:
			return h, invalid("tipo %q inválido, use check ou count", *kind)
		}
	}
	if target != nil {
		h.Target = *target
	}
	if unit != nil {
		h.Unit = *unit
	}
	if color != nil {
		h.Color = strings.TrimSpace(*color)
	}
	if days != nil {
		mask, err := domain.DaysToMask(days)
		if err != nil {
			return h, err
		}
		h.Weekdays = mask
	}
	if archived != nil {
		h.Archived = *archived
	}
	return h, nil
}

func itoa(n int) string { return fmtInt(n) }
