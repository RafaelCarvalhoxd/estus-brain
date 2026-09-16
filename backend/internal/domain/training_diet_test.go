package domain

import (
	"math"
	"reflect"
	"testing"
)

func TestWeekdayMaskRoundTrip(t *testing.T) {
	mask, err := DaysToMask([]int{1, 3, 5})
	if err != nil {
		t.Fatal(err)
	}
	if mask != 0b0101010 {
		t.Fatalf("mask = %b, want 101010", mask)
	}
	if got := MaskToDays(mask); !reflect.DeepEqual(got, []int{1, 3, 5}) {
		t.Fatalf("MaskToDays = %v", got)
	}
	if got := MaskToDays(0); got == nil || len(got) != 0 {
		t.Fatalf("empty mask should give an empty, non-nil slice, got %#v", got)
	}
	if mask, _ := DaysToMask([]int{0, 6, 6}); mask != 0b1000001 {
		t.Fatalf("duplicates and Sunday/Saturday: mask = %b", mask)
	}
	for _, bad := range [][]int{{-1}, {7}} {
		if _, err := DaysToMask(bad); err == nil {
			t.Fatalf("expected error for %v", bad)
		}
	}
}

func validWorkout() Workout {
	return Workout{
		Name:     "Treino A",
		Weekdays: 0b0000010,
		Exercises: []Exercise{
			{Name: "Supino reto", Sets: 4, Reps: "8-12", Weight: "30 kg", RestSeconds: 90},
		},
	}
}

func TestWorkoutValidate(t *testing.T) {
	if err := validWorkout().Validate(); err != nil {
		t.Fatalf("valid workout rejected: %v", err)
	}
	cases := map[string]func(*Workout){
		"no name":          func(w *Workout) { w.Name = "  " },
		"no day":           func(w *Workout) { w.Weekdays = 0 },
		"mask too big":     func(w *Workout) { w.Weekdays = 128 },
		"unnamed exercise": func(w *Workout) { w.Exercises[0].Name = "" },
		"zero sets":        func(w *Workout) { w.Exercises[0].Sets = 0 },
		"negative rest":    func(w *Workout) { w.Exercises[0].RestSeconds = -1 },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			w := validWorkout()
			mutate(&w)
			if err := w.Validate(); err == nil {
				t.Fatal("expected a validation error")
			}
		})
	}
	w := validWorkout()
	w.Exercises = nil
	if err := w.Validate(); err != nil {
		t.Fatalf("a workout may be saved before its exercises are listed: %v", err)
	}
}

func validMeal() Meal {
	return Meal{
		Name:     "Almoço",
		Time:     "12:30",
		Weekdays: allWeekdays,
		Items:    []MealItem{{Food: "Arroz", Quantity: "150 g", Kcal: 195, ProteinG: 4, CarbsG: 42, FatG: 0.4}},
	}
}

func TestMealValidate(t *testing.T) {
	if err := validMeal().Validate(); err != nil {
		t.Fatalf("valid meal rejected: %v", err)
	}
	cases := map[string]func(*Meal){
		"no name":        func(m *Meal) { m.Name = "" },
		"hour 24":        func(m *Meal) { m.Time = "24:00" },
		"no leading 0":   func(m *Meal) { m.Time = "7:30" },
		"no day":         func(m *Meal) { m.Weekdays = 0 },
		"unnamed food":   func(m *Meal) { m.Items[0].Food = " " },
		"negative carbs": func(m *Meal) { m.Items[0].CarbsG = -1 },
		"NaN kcal":       func(m *Meal) { m.Items[0].Kcal = math.NaN() },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			m := validMeal()
			mutate(&m)
			if err := m.Validate(); err == nil {
				t.Fatal("expected a validation error")
			}
		})
	}
}

func TestDietTargetsValidate(t *testing.T) {
	if err := (DietTargets{Kcal: 2200, ProteinG: 160}).Validate(); err != nil {
		t.Fatalf("valid targets rejected: %v", err)
	}
	if err := (DietTargets{FatG: -5}).Validate(); err == nil {
		t.Fatal("negative target accepted")
	}
}
