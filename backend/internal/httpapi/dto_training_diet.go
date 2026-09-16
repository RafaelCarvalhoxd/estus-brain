package httpapi

import (
	"time"

	"github.com/rafael/estus-vault/backend/internal/domain"
)

// Days cross the API as a list of weekdays (0 = Sunday … 6 = Saturday); the
// mask they're stored as stays a storage detail.

type exerciseDTO struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name"`
	Sets        int    `json:"sets"`
	Reps        string `json:"reps"`
	Weight      string `json:"weight"`
	RestSeconds int    `json:"rest_seconds"`
	Notes       string `json:"notes"`
}

type workoutDTO struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	Focus     string        `json:"focus"`
	Days      []int         `json:"days"`
	Notes     string        `json:"notes"`
	Exercises []exerciseDTO `json:"exercises"`
	CreatedAt string        `json:"created_at"`
	UpdatedAt string        `json:"updated_at"`
}

func toWorkoutDTO(w domain.Workout) workoutDTO {
	exercises := make([]exerciseDTO, len(w.Exercises))
	for i, e := range w.Exercises {
		exercises[i] = exerciseDTO{
			ID:          e.ID,
			Name:        e.Name,
			Sets:        e.Sets,
			Reps:        e.Reps,
			Weight:      e.Weight,
			RestSeconds: e.RestSeconds,
			Notes:       e.Notes,
		}
	}
	return workoutDTO{
		ID:        w.ID,
		Name:      w.Name,
		Focus:     w.Focus,
		Days:      domain.MaskToDays(w.Weekdays),
		Notes:     w.Notes,
		Exercises: exercises,
		CreatedAt: w.CreatedAt.Format(time.RFC3339),
		UpdatedAt: w.UpdatedAt.Format(time.RFC3339),
	}
}

type workoutRequest struct {
	Name      string        `json:"name"`
	Focus     string        `json:"focus"`
	Days      []int         `json:"days"`
	Notes     string        `json:"notes"`
	Exercises []exerciseDTO `json:"exercises"`
}

func (req workoutRequest) toDomain() (domain.Workout, error) {
	mask, err := domain.DaysToMask(req.Days)
	if err != nil {
		return domain.Workout{}, err
	}
	w := domain.Workout{Name: req.Name, Focus: req.Focus, Weekdays: mask, Notes: req.Notes}
	for _, e := range req.Exercises {
		w.Exercises = append(w.Exercises, domain.Exercise{
			Name:        e.Name,
			Sets:        e.Sets,
			Reps:        e.Reps,
			Weight:      e.Weight,
			RestSeconds: e.RestSeconds,
			Notes:       e.Notes,
		})
	}
	return w, nil
}

type mealItemDTO struct {
	ID       string  `json:"id,omitempty"`
	Food     string  `json:"food"`
	Quantity string  `json:"quantity"`
	Kcal     float64 `json:"kcal"`
	ProteinG float64 `json:"protein_g"`
	CarbsG   float64 `json:"carbs_g"`
	FatG     float64 `json:"fat_g"`
}

type mealDTO struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	Time      string        `json:"time"`
	Days      []int         `json:"days"`
	Notes     string        `json:"notes"`
	Items     []mealItemDTO `json:"items"`
	CreatedAt string        `json:"created_at"`
	UpdatedAt string        `json:"updated_at"`
}

func toMealDTO(m domain.Meal) mealDTO {
	items := make([]mealItemDTO, len(m.Items))
	for i, it := range m.Items {
		items[i] = mealItemDTO{
			ID:       it.ID,
			Food:     it.Food,
			Quantity: it.Quantity,
			Kcal:     it.Kcal,
			ProteinG: it.ProteinG,
			CarbsG:   it.CarbsG,
			FatG:     it.FatG,
		}
	}
	return mealDTO{
		ID:        m.ID,
		Name:      m.Name,
		Time:      m.Time,
		Days:      domain.MaskToDays(m.Weekdays),
		Notes:     m.Notes,
		Items:     items,
		CreatedAt: m.CreatedAt.Format(time.RFC3339),
		UpdatedAt: m.UpdatedAt.Format(time.RFC3339),
	}
}

type mealRequest struct {
	Name  string        `json:"name"`
	Time  string        `json:"time"`
	Days  []int         `json:"days"`
	Notes string        `json:"notes"`
	Items []mealItemDTO `json:"items"`
}

func (req mealRequest) toDomain() (domain.Meal, error) {
	mask, err := domain.DaysToMask(req.Days)
	if err != nil {
		return domain.Meal{}, err
	}
	m := domain.Meal{Name: req.Name, Time: req.Time, Weekdays: mask, Notes: req.Notes}
	for _, it := range req.Items {
		m.Items = append(m.Items, domain.MealItem{
			Food:     it.Food,
			Quantity: it.Quantity,
			Kcal:     it.Kcal,
			ProteinG: it.ProteinG,
			CarbsG:   it.CarbsG,
			FatG:     it.FatG,
		})
	}
	return m, nil
}

type dietTargetsDTO struct {
	Kcal     float64 `json:"kcal"`
	ProteinG float64 `json:"protein_g"`
	CarbsG   float64 `json:"carbs_g"`
	FatG     float64 `json:"fat_g"`
}
