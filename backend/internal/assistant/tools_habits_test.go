package assistant

import (
	"testing"

	"github.com/rafael/estus-vault/backend/internal/domain"
)

func TestHabitEdit(t *testing.T) {
	h := domain.Habit{ID: "1", Name: "Ler", Kind: domain.HabitCheck, Target: 1, Weekdays: 127, Color: "#e2c23a"}
	name, kind, unit := "Água", "count", "copos"
	target, archived := 8, true

	got, err := habitEdit(h, nil, nil, nil, nil, nil, nil, nil)
	if err != nil || got.Name != h.Name || got.Weekdays != h.Weekdays {
		t.Fatalf("no fields changed the habit: %+v, %v", got, err)
	}

	got, err = habitEdit(h, &name, &kind, &unit, nil, &target, []int{1, 3}, &archived)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Água" || got.Kind != domain.HabitCount || got.Target != 8 || got.Unit != "copos" || got.Weekdays != 1<<1|1<<3 || !got.Archived || got.Color != h.Color {
		t.Errorf("edit = %+v", got)
	}

	bad := "diario"
	if _, err := habitEdit(h, nil, &bad, nil, nil, nil, nil, nil); err == nil {
		t.Error("unknown kind accepted")
	}
	if _, err := habitEdit(h, nil, nil, nil, nil, nil, []int{7}, nil); err == nil {
		t.Error("weekday 7 accepted")
	}

	row := habitRow(got)
	if row["dias"] != "seg, qua" || row["meta"] != 8 || row["arquivado"] != true {
		t.Errorf("habitRow = %v", row)
	}
}
