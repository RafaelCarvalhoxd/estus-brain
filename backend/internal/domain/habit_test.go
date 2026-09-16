package domain

import (
	"testing"
	"time"
)

func day(s string) time.Time {
	d, err := time.Parse(DayLayout, s)
	if err != nil {
		panic(err)
	}
	return d
}

func TestHabitValidate(t *testing.T) {
	h := Habit{Name: " Ler ", Kind: HabitCheck, Target: 9, Unit: "x", Weekdays: 127, Color: "#aabbcc"}
	if err := h.Validate(); err != nil {
		t.Fatalf("valid habit rejected: %v", err)
	}
	if h.Name != "Ler" || h.Target != 1 || h.Unit != "" {
		t.Fatalf("check habit not normalized: %+v", h)
	}

	bad := []Habit{
		{Name: "", Kind: HabitCheck, Weekdays: 127, Color: "#aabbcc"},
		{Name: "Água", Kind: HabitCount, Target: 0, Weekdays: 127, Color: "#aabbcc"},
		{Name: "Água", Kind: "other", Weekdays: 127, Color: "#aabbcc"},
		{Name: "Água", Kind: HabitCheck, Weekdays: 0, Color: "#aabbcc"},
		{Name: "Água", Kind: HabitCheck, Weekdays: 127, Color: "red"},
	}
	for i, b := range bad {
		if err := b.Validate(); err == nil {
			t.Fatalf("case %d accepted", i)
		}
	}
}

func TestHabitStreaks(t *testing.T) {
	// 2026-09-07 is a Monday.
	daily := Habit{Kind: HabitCheck, Target: 1, Weekdays: 127, StartDay: day("2026-09-07")}

	cases := []struct {
		name          string
		habit         Habit
		logs          []string
		today         string
		current, best int
	}{
		{"nothing yet", daily, nil, "2026-09-07", 0, 0},
		{"today pending keeps the run", daily, []string{"2026-09-07", "2026-09-08"}, "2026-09-09", 2, 2},
		{"today done counts", daily, []string{"2026-09-07", "2026-09-08", "2026-09-09"}, "2026-09-09", 3, 3},
		{"a missed day breaks it", daily, []string{"2026-09-07", "2026-09-08", "2026-09-10"}, "2026-09-11", 1, 2},
		{
			"days off the schedule are skipped",
			// Monday, Wednesday, Friday only.
			Habit{Kind: HabitCheck, Target: 1, Weekdays: 1<<1 | 1<<3 | 1<<5, StartDay: day("2026-09-07")},
			[]string{"2026-09-07", "2026-09-09", "2026-09-11"},
			"2026-09-13",
			3, 3,
		},
		{"logs before the start still count", daily, []string{"2026-09-05", "2026-09-06", "2026-09-07"}, "2026-09-07", 3, 3},
	}
	for _, c := range cases {
		h := c.habit
		h.Logs = map[string]int{}
		for _, l := range c.logs {
			h.Logs[l] = 1
		}
		cur, best := h.Streaks(day(c.today))
		if cur != c.current || best != c.best {
			t.Errorf("%s: got %d/%d, want %d/%d", c.name, cur, best, c.current, c.best)
		}
	}

	count := Habit{Kind: HabitCount, Target: 8, Weekdays: 127, StartDay: day("2026-09-07"), Logs: map[string]int{"2026-09-07": 8, "2026-09-08": 5}}
	if cur, _ := count.Streaks(day("2026-09-09")); cur != 0 {
		t.Errorf("partial count should break the run, got %d", cur)
	}
}
