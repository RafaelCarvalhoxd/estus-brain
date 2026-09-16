package assistant

import (
	"context"
	"fmt"
	"math"
	"strconv"
)

func fmtInt(n int) string { return strconv.Itoa(n) }

func round1(v float64) float64 { return math.Round(v*10) / 10 }

func (r *Registry) addHealth() {
	d := r.deps

	if d.Training != nil {
		r.add(Tool{
			Name: "training_day", Title: "Treino do dia", Module: "treino", ReadOnly: true,
			Description: "O treino previsto para um dia (padrão hoje), com os exercícios; ou o plano da semana inteira com week=true.",
			Input: object(map[string]any{
				"day":  str("Dia AAAA-MM-DD ou hoje/amanha; padrão hoje"),
				"week": boolean("Trazer o plano da semana inteira"),
			}),
			run: typed(func(ctx context.Context, in struct {
				Day  string `json:"day"`
				Week bool   `json:"week"`
			}) (any, error) {
				day, err := r.parseDay(in.Day)
				if err != nil {
					return nil, err
				}
				workouts, err := d.Training.List(ctx)
				if err != nil {
					return nil, err
				}
				type ex struct {
					Name   string `json:"exercicio"`
					Detail string `json:"series"`
					Weight string `json:"carga,omitempty"`
				}
				type wk struct {
					Name      string `json:"treino"`
					Focus     string `json:"foco,omitempty"`
					Days      []int  `json:"dias"`
					Exercises []ex   `json:"exercicios"`
				}
				var out []wk
				for _, w := range workouts {
					if !in.Week && w.Weekdays&(1<<int(day.Weekday())) == 0 {
						continue
					}
					item := wk{Name: w.Name, Focus: w.Focus}
					for dd := 0; dd < 7; dd++ {
						if w.Weekdays&(1<<dd) != 0 {
							item.Days = append(item.Days, dd)
						}
					}
					for _, e := range w.Exercises {
						item.Exercises = append(item.Exercises, ex{e.Name, fmt.Sprintf("%d×%s", e.Sets, e.Reps), e.Weight})
					}
					out = append(out, item)
				}
				res := map[string]any{"treinos": out, "dias_da_semana": "0=domingo … 6=sábado"}
				if !in.Week {
					res["dia"] = day.Format(dayLayout)
					if len(out) == 0 {
						res["descanso"] = true
					}
				}
				return res, nil
			}),
		})
	}

	if d.Diet != nil {
		r.add(Tool{
			Name: "diet_day", Title: "Dieta do dia", Module: "dieta", ReadOnly: true,
			Description: "As refeições previstas para um dia (padrão hoje), com calorias e macros somados contra as metas.",
			Input:       object(map[string]any{"day": str("Dia AAAA-MM-DD ou hoje/amanha; padrão hoje")}),
			run: typed(func(ctx context.Context, in struct {
				Day string `json:"day"`
			}) (any, error) {
				day, err := r.parseDay(in.Day)
				if err != nil {
					return nil, err
				}
				meals, err := d.Diet.List(ctx)
				if err != nil {
					return nil, err
				}
				targets, err := d.Diet.Targets(ctx)
				if err != nil {
					return nil, err
				}
				type meal struct {
					Name  string   `json:"refeicao"`
					Time  string   `json:"horario"`
					Foods []string `json:"alimentos"`
					Kcal  float64  `json:"kcal"`
				}
				var out []meal
				var kcal, protein, carbs, fat float64
				for _, m := range meals {
					if m.Weekdays&(1<<int(day.Weekday())) == 0 {
						continue
					}
					item := meal{Name: m.Name, Time: m.Time}
					for _, it := range m.Items {
						item.Foods = append(item.Foods, fmt.Sprintf("%s %s", it.Quantity, it.Food))
						item.Kcal += it.Kcal
						protein += it.ProteinG
						carbs += it.CarbsG
						fat += it.FatG
					}
					kcal += item.Kcal
					item.Kcal = round1(item.Kcal)
					out = append(out, item)
				}
				return map[string]any{
					"dia":       day.Format(dayLayout),
					"refeicoes": out,
					"totais":    map[string]float64{"kcal": round1(kcal), "proteina_g": round1(protein), "carboidratos_g": round1(carbs), "gordura_g": round1(fat)},
					"metas":     map[string]float64{"kcal": targets.Kcal, "proteina_g": targets.ProteinG, "carboidratos_g": targets.CarbsG, "gordura_g": targets.FatG},
				}, nil
			}),
		})
	}
}
