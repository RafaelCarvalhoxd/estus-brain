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

	find := func(ctx context.Context, query string) (domain.Habit, error) {
		habits, err := d.Habits.List(ctx)
		if err != nil {
			return domain.Habit{}, err
		}
		var active []domain.Habit
		for _, h := range habits {
			if !h.Archived {
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
			h, err := find(ctx, in.Habit)
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
}

func itoa(n int) string { return fmtInt(n) }
