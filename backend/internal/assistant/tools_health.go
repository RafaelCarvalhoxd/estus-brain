package assistant

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/rafael/estus-vault/backend/internal/domain"
)

func fmtInt(n int) string { return strconv.Itoa(n) }

func round1(v float64) float64 { return math.Round(v*10) / 10 }

func (r *Registry) addHealth() {
	d := r.deps

	if d.Training != nil {
		r.add(Tool{
			Name: "training_day", Title: "Treino do dia", Module: "treino", ReadOnly: true,
			Description: "O treino previsto para um dia (padrão hoje), com os exercícios; ou o plano da semana inteira com week=true. Traz o id de cada treino, para training_update e training_delete.",
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
					Rest   int    `json:"descanso_s,omitempty"`
					Notes  string `json:"obs,omitempty"`
				}
				type wk struct {
					ID        string `json:"id"`
					Name      string `json:"treino"`
					Focus     string `json:"foco,omitempty"`
					Days      []int  `json:"dias"`
					Notes     string `json:"obs,omitempty"`
					Exercises []ex   `json:"exercicios"`
				}
				var out []wk
				for _, w := range workouts {
					if !in.Week && w.Weekdays&(1<<int(day.Weekday())) == 0 {
						continue
					}
					item := wk{ID: w.ID, Name: w.Name, Focus: w.Focus, Days: domain.MaskToDays(w.Weekdays), Notes: w.Notes}
					for _, e := range w.Exercises {
						item.Exercises = append(item.Exercises, ex{e.Name, fmt.Sprintf("%d×%s", e.Sets, e.Reps), e.Weight, e.RestSeconds, e.Notes})
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
			Description: "As refeições previstas para um dia (padrão hoje), com calorias e macros somados contra as metas; ou o plano da semana inteira com week=true. Traz o id de cada refeição, para diet_update e diet_delete.",
			Input: object(map[string]any{
				"day":  str("Dia AAAA-MM-DD ou hoje/amanha; padrão hoje"),
				"week": boolean("Trazer todas as refeições da semana, sem totais do dia"),
			}),
			run: typed(func(ctx context.Context, in struct {
				Day  string `json:"day"`
				Week bool   `json:"week"`
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
				type food struct {
					Food     string  `json:"alimento"`
					Quantity string  `json:"quantidade,omitempty"`
					Kcal     float64 `json:"kcal"`
					ProteinG float64 `json:"proteina_g"`
					CarbsG   float64 `json:"carboidratos_g"`
					FatG     float64 `json:"gordura_g"`
				}
				type meal struct {
					ID    string  `json:"id"`
					Name  string  `json:"refeicao"`
					Time  string  `json:"horario"`
					Days  []int   `json:"dias,omitempty"`
					Notes string  `json:"obs,omitempty"`
					Foods []food  `json:"alimentos"`
					Kcal  float64 `json:"kcal"`
				}
				var out []meal
				var kcal, protein, carbs, fat float64
				for _, m := range meals {
					if !in.Week && m.Weekdays&(1<<int(day.Weekday())) == 0 {
						continue
					}
					item := meal{ID: m.ID, Name: m.Name, Time: m.Time, Notes: m.Notes}
					if in.Week {
						item.Days = domain.MaskToDays(m.Weekdays)
					}
					for _, it := range m.Items {
						item.Foods = append(item.Foods, food{it.Food, it.Quantity, it.Kcal, it.ProteinG, it.CarbsG, it.FatG})
						item.Kcal += it.Kcal
						protein += it.ProteinG
						carbs += it.CarbsG
						fat += it.FatG
					}
					kcal += item.Kcal
					item.Kcal = round1(item.Kcal)
					out = append(out, item)
				}
				metas := map[string]float64{"kcal": targets.Kcal, "proteina_g": targets.ProteinG, "carboidratos_g": targets.CarbsG, "gordura_g": targets.FatG}
				if in.Week {
					return map[string]any{"refeicoes": out, "metas_diarias": metas, "dias_da_semana": "0=domingo … 6=sábado"}, nil
				}
				return map[string]any{
					"dia":       day.Format(dayLayout),
					"refeicoes": out,
					"totais":    map[string]float64{"kcal": round1(kcal), "proteina_g": round1(protein), "carboidratos_g": round1(carbs), "gordura_g": round1(fat)},
					"metas":     metas,
				}, nil
			}),
		})
	}

	r.addHealthWrites()
}

type exerciseIn struct {
	Name        string `json:"name"`
	Sets        int    `json:"sets"`
	Reps        string `json:"reps"`
	Weight      string `json:"weight"`
	RestSeconds int    `json:"rest_seconds"`
	Notes       string `json:"notes"`
}

type foodIn struct {
	Food     string  `json:"food"`
	Quantity string  `json:"quantity"`
	Kcal     float64 `json:"kcal"`
	ProteinG float64 `json:"protein_g"`
	CarbsG   float64 `json:"carbs_g"`
	FatG     float64 `json:"fat_g"`
}

// Update inputs are patches: a field left out keeps its current value.
// Exercises and foods are pointers to slices so "left out" (keep) differs
// from an empty list (clear).
type workoutIn struct {
	ID        string        `json:"id"`
	Name      *string       `json:"name"`
	Focus     *string       `json:"focus"`
	Days      *[]int        `json:"days"`
	Notes     *string       `json:"notes"`
	Exercises *[]exerciseIn `json:"exercises"`
}

type mealIn struct {
	ID    string    `json:"id"`
	Name  *string   `json:"name"`
	Time  *string   `json:"time"`
	Days  *[]int    `json:"days"`
	Notes *string   `json:"notes"`
	Foods *[]foodIn `json:"foods"`
}

func (in workoutIn) apply(w domain.Workout) (domain.Workout, error) {
	if in.Name != nil {
		w.Name = strings.TrimSpace(*in.Name)
	}
	if in.Focus != nil {
		w.Focus = strings.TrimSpace(*in.Focus)
	}
	if in.Notes != nil {
		w.Notes = strings.TrimSpace(*in.Notes)
	}
	if in.Days != nil {
		mask, err := domain.DaysToMask(*in.Days)
		if err != nil {
			return w, err
		}
		w.Weekdays = mask
	}
	if in.Exercises != nil {
		w.Exercises = make([]domain.Exercise, len(*in.Exercises))
		for i, e := range *in.Exercises {
			w.Exercises[i] = domain.Exercise{Name: strings.TrimSpace(e.Name), Sets: e.Sets, Reps: strings.TrimSpace(e.Reps), Weight: strings.TrimSpace(e.Weight), RestSeconds: e.RestSeconds, Notes: strings.TrimSpace(e.Notes)}
		}
	}
	return w, nil
}

func (in mealIn) apply(m domain.Meal) (domain.Meal, error) {
	if in.Name != nil {
		m.Name = strings.TrimSpace(*in.Name)
	}
	if in.Time != nil {
		m.Time = strings.TrimSpace(*in.Time)
	}
	if in.Notes != nil {
		m.Notes = strings.TrimSpace(*in.Notes)
	}
	if in.Days != nil {
		mask, err := domain.DaysToMask(*in.Days)
		if err != nil {
			return m, err
		}
		m.Weekdays = mask
	}
	if in.Foods != nil {
		m.Items = make([]domain.MealItem, len(*in.Foods))
		for i, f := range *in.Foods {
			m.Items[i] = domain.MealItem{Food: strings.TrimSpace(f.Food), Quantity: strings.TrimSpace(f.Quantity), Kcal: f.Kcal, ProteinG: f.ProteinG, CarbsG: f.CarbsG, FatG: f.FatG}
		}
	}
	return m, nil
}

func daysSchema() map[string]any {
	return array("Dias da semana, 0=domingo … 6=sábado", integer("dia da semana"))
}

func exercisesSchema() map[string]any {
	return array("Exercícios, na ordem; numa edição, a lista substitui a atual inteira", object(map[string]any{
		"name":         str("Exercício"),
		"sets":         integer("Séries (1 a 50)"),
		"reps":         str("Repetições, texto livre: 8-12, até a falha"),
		"weight":       str("Carga, texto livre: 20 kg, peso corporal"),
		"rest_seconds": integer("Descanso em segundos"),
		"notes":        str("Observação"),
	}, "name", "sets"))
}

func foodsSchema() map[string]any {
	return array("Alimentos da refeição; numa edição, a lista substitui a atual inteira", object(map[string]any{
		"food":      str("Alimento"),
		"quantity":  str("Quantidade, texto livre: 100 g, 2 ovos"),
		"kcal":      number("Calorias da porção"),
		"protein_g": number("Proteína em gramas"),
		"carbs_g":   number("Carboidratos em gramas"),
		"fat_g":     number("Gordura em gramas"),
	}, "food"))
}

func (r *Registry) addHealthWrites() {
	d := r.deps

	if d.Training != nil {
		findWorkout := func(ctx context.Context, id string) (domain.Workout, error) {
			workouts, err := d.Training.List(ctx)
			if err != nil {
				return domain.Workout{}, err
			}
			for _, w := range workouts {
				if w.ID == strings.TrimSpace(id) {
					return w, nil
				}
			}
			return domain.Workout{}, invalid("treino %q não encontrado; use o id de training_day com week=true", id)
		}

		r.add(Tool{
			Name: "training_create", Title: "Novo treino", Module: "treino",
			Description: "Cria um treino no plano semanal, com os dias em que ele cai e os exercícios.",
			Input: object(map[string]any{
				"name":      str("Nome do treino, ex.: Treino A"),
				"focus":     str("Foco, ex.: peito e tríceps"),
				"days":      daysSchema(),
				"notes":     str("Observações"),
				"exercises": exercisesSchema(),
			}, "name", "days"),
			run: typed(func(ctx context.Context, in workoutIn) (any, error) {
				w, err := in.apply(domain.Workout{})
				if err != nil {
					return nil, err
				}
				saved, err := d.Training.Create(ctx, w)
				if err != nil {
					return nil, err
				}
				return map[string]any{"id": saved.ID, "treino": saved.Name, "dias": domain.MaskToDays(saved.Weekdays), "exercicios": len(saved.Exercises)}, nil
			}),
		})

		r.add(Tool{
			Name: "training_update", Title: "Editar treino", Module: "treino",
			Description: "Altera um treino pelo id. Só os campos enviados mudam; exercises, se enviado, substitui a lista inteira.",
			Input: object(map[string]any{
				"id":        str("Id do treino (de training_day)"),
				"name":      str("Novo nome"),
				"focus":     str("Novo foco"),
				"days":      daysSchema(),
				"notes":     str("Novas observações"),
				"exercises": exercisesSchema(),
			}, "id"),
			run: typed(func(ctx context.Context, in workoutIn) (any, error) {
				current, err := findWorkout(ctx, in.ID)
				if err != nil {
					return nil, err
				}
				w, err := in.apply(current)
				if err != nil {
					return nil, err
				}
				saved, err := d.Training.Update(ctx, current.ID, w)
				if err != nil {
					return nil, err
				}
				return map[string]any{"id": saved.ID, "treino": saved.Name, "dias": domain.MaskToDays(saved.Weekdays), "exercicios": len(saved.Exercises)}, nil
			}),
		})

		r.add(Tool{
			Name: "training_delete", Title: "Excluir treino", Module: "treino", Destructive: true,
			Description: "Exclui um treino do plano pelo id, com todos os exercícios.",
			Input:       object(map[string]any{"id": str("Id do treino (de training_day)")}, "id"),
			run: typed(func(ctx context.Context, in struct {
				ID string `json:"id"`
			}) (any, error) {
				return map[string]any{"ok": true}, d.Training.Delete(ctx, strings.TrimSpace(in.ID))
			}),
		})
	}

	if d.Diet != nil {
		findMeal := func(ctx context.Context, id string) (domain.Meal, error) {
			meals, err := d.Diet.List(ctx)
			if err != nil {
				return domain.Meal{}, err
			}
			for _, m := range meals {
				if m.ID == strings.TrimSpace(id) {
					return m, nil
				}
			}
			return domain.Meal{}, invalid("refeição %q não encontrada; use o id de diet_day com week=true", id)
		}

		r.add(Tool{
			Name: "diet_create", Title: "Nova refeição", Module: "dieta",
			Description: "Cria uma refeição no plano de dieta, com horário, dias e alimentos com seus macros.",
			Input: object(map[string]any{
				"name":  str("Nome, ex.: Almoço"),
				"time":  str("Horário HH:MM"),
				"days":  array("Dias da semana, 0=domingo … 6=sábado; padrão todos", integer("dia da semana")),
				"notes": str("Observações"),
				"foods": foodsSchema(),
			}, "name", "time"),
			run: typed(func(ctx context.Context, in mealIn) (any, error) {
				if in.Days == nil || len(*in.Days) == 0 {
					in.Days = &[]int{0, 1, 2, 3, 4, 5, 6}
				}
				m, err := in.apply(domain.Meal{})
				if err != nil {
					return nil, err
				}
				saved, err := d.Diet.Create(ctx, m)
				if err != nil {
					return nil, err
				}
				return map[string]any{"id": saved.ID, "refeicao": saved.Name, "horario": saved.Time, "dias": domain.MaskToDays(saved.Weekdays), "alimentos": len(saved.Items)}, nil
			}),
		})

		r.add(Tool{
			Name: "diet_update", Title: "Editar refeição", Module: "dieta",
			Description: "Altera uma refeição pelo id. Só os campos enviados mudam; foods, se enviado, substitui a lista inteira.",
			Input: object(map[string]any{
				"id":    str("Id da refeição (de diet_day)"),
				"name":  str("Novo nome"),
				"time":  str("Novo horário HH:MM"),
				"days":  daysSchema(),
				"notes": str("Novas observações"),
				"foods": foodsSchema(),
			}, "id"),
			run: typed(func(ctx context.Context, in mealIn) (any, error) {
				current, err := findMeal(ctx, in.ID)
				if err != nil {
					return nil, err
				}
				m, err := in.apply(current)
				if err != nil {
					return nil, err
				}
				saved, err := d.Diet.Update(ctx, current.ID, m)
				if err != nil {
					return nil, err
				}
				return map[string]any{"id": saved.ID, "refeicao": saved.Name, "horario": saved.Time, "dias": domain.MaskToDays(saved.Weekdays), "alimentos": len(saved.Items)}, nil
			}),
		})

		r.add(Tool{
			Name: "diet_delete", Title: "Excluir refeição", Module: "dieta", Destructive: true,
			Description: "Exclui uma refeição do plano pelo id.",
			Input:       object(map[string]any{"id": str("Id da refeição (de diet_day)")}, "id"),
			run: typed(func(ctx context.Context, in struct {
				ID string `json:"id"`
			}) (any, error) {
				return map[string]any{"ok": true}, d.Diet.Delete(ctx, strings.TrimSpace(in.ID))
			}),
		})

		targetsOut := func(t domain.DietTargets) map[string]float64 {
			return map[string]float64{"kcal": t.Kcal, "proteina_g": t.ProteinG, "carboidratos_g": t.CarbsG, "gordura_g": t.FatG}
		}

		r.add(Tool{
			Name: "diet_targets", Title: "Metas da dieta", Module: "dieta", ReadOnly: true,
			Description: "As metas diárias de calorias e macros. Zero é sem meta.",
			Input:       object(map[string]any{}),
			run: typed(func(ctx context.Context, _ struct{}) (any, error) {
				t, err := d.Diet.Targets(ctx)
				if err != nil {
					return nil, err
				}
				return map[string]any{"metas": targetsOut(t)}, nil
			}),
		})

		r.add(Tool{
			Name: "diet_set_targets", Title: "Definir metas da dieta", Module: "dieta",
			Description: "Define as metas diárias de calorias e macros. Só os valores enviados mudam; 0 tira a meta.",
			Input: object(map[string]any{
				"kcal":      number("Calorias por dia"),
				"protein_g": number("Proteína em gramas por dia"),
				"carbs_g":   number("Carboidratos em gramas por dia"),
				"fat_g":     number("Gordura em gramas por dia"),
			}),
			run: typed(func(ctx context.Context, in struct {
				Kcal     *float64 `json:"kcal"`
				ProteinG *float64 `json:"protein_g"`
				CarbsG   *float64 `json:"carbs_g"`
				FatG     *float64 `json:"fat_g"`
			}) (any, error) {
				t, err := d.Diet.Targets(ctx)
				if err != nil {
					return nil, err
				}
				for _, f := range []struct {
					in  *float64
					out *float64
				}{{in.Kcal, &t.Kcal}, {in.ProteinG, &t.ProteinG}, {in.CarbsG, &t.CarbsG}, {in.FatG, &t.FatG}} {
					if f.in != nil {
						*f.out = *f.in
					}
				}
				saved, err := d.Diet.SetTargets(ctx, t)
				if err != nil {
					return nil, err
				}
				return map[string]any{"metas": targetsOut(saved)}, nil
			}),
		})
	}
}
