package domain

import (
	"testing"
	"time"
)

func TestReminderBucket(t *testing.T) {
	now := time.Date(2026, time.September, 11, 12, 0, 0, 0, time.UTC)

	past := now.Add(-2 * time.Hour)
	laterToday := now.Add(2 * time.Hour)
	future := now.AddDate(0, 0, 5)

	cases := []struct {
		name string
		r    Reminder
		want string
	}{
		{"done takes priority over everything", Reminder{Done: true, DueAt: &past}, "concluido"},
		{"done with no due date", Reminder{Done: true}, "concluido"},
		{"overdue, not done", Reminder{DueAt: &past}, "atrasado"},
		{"due later today, not done", Reminder{DueAt: &laterToday}, "hoje"},
		{"due in the future, not done", Reminder{DueAt: &future}, "proximo"},
		{"no due date, not done", Reminder{DueAt: nil}, "proximo"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := c.r.Bucket(now)
			if got != c.want {
				t.Errorf("Bucket() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestReminderValidate(t *testing.T) {
	if err := (Reminder{Title: ""}).Validate(); err == nil {
		t.Error("expected error for empty title")
	}
	if err := (Reminder{Title: "Pagar aluguel"}).Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestReminderRepeat(t *testing.T) {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Fatal(err)
	}
	// Tuesday 2026-09-22, 08:00 local.
	tue := time.Date(2026, time.September, 22, 8, 0, 0, 0, loc)
	monWedFri := []time.Weekday{time.Monday, time.Wednesday, time.Friday}

	t.Run("first due snaps to the next repeat day", func(t *testing.T) {
		r := Reminder{DueAt: &tue, RepeatDays: monWedFri}
		want := time.Date(2026, time.September, 23, 8, 0, 0, 0, loc)
		if got := r.FirstDue(loc); !got.Equal(want) {
			t.Fatalf("got %v, want %v", got, want)
		}
	})

	t.Run("first due keeps a date already on a repeat day", func(t *testing.T) {
		r := Reminder{DueAt: &tue, RepeatDays: []time.Weekday{time.Tuesday}}
		if got := r.FirstDue(loc); !got.Equal(tue) {
			t.Fatalf("got %v, want %v", got, tue)
		}
	})

	t.Run("next due is the following repeat day", func(t *testing.T) {
		wed := time.Date(2026, time.September, 23, 8, 0, 0, 0, loc)
		r := Reminder{DueAt: &wed, RepeatDays: monWedFri}
		want := time.Date(2026, time.September, 25, 8, 0, 0, 0, loc)
		if got := r.NextDue(wed.Add(-time.Hour), loc); !got.Equal(want) {
			t.Fatalf("got %v, want %v", got, want)
		}
	})

	t.Run("next due skips days already missed", func(t *testing.T) {
		r := Reminder{DueAt: &tue, RepeatDays: monWedFri}
		now := time.Date(2026, time.September, 28, 9, 0, 0, 0, loc) // Monday, after 08:00
		want := time.Date(2026, time.September, 30, 8, 0, 0, 0, loc)
		if got := r.NextDue(now, loc); !got.Equal(want) {
			t.Fatalf("got %v, want %v", got, want)
		}
	})

	t.Run("repeating needs a date", func(t *testing.T) {
		if err := (Reminder{Title: "x", RepeatDays: monWedFri}).Validate(); err == nil {
			t.Fatal("expected validation error")
		}
	})
}

func TestReminderRepeatMonthly(t *testing.T) {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Fatal(err)
	}
	at := func(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 9, 0, 0, 0, loc) }

	cases := []struct {
		name     string
		day      int
		due, now time.Time
		first    time.Time
		next     time.Time
	}{
		{"day 15, set on the 15th", 15, at(2026, 9, 15), at(2026, 9, 15), at(2026, 9, 15), at(2026, 10, 15)},
		{"day 15, set after the 15th starts next month", 15, at(2026, 9, 22), at(2026, 9, 22), at(2026, 10, 15), at(2026, 10, 15)},
		{"day 31 in a 30-day month uses the 30th", 31, at(2026, 9, 1), at(2026, 9, 1), at(2026, 9, 30), at(2026, 10, 31)},
		{"late completion skips missed months", 15, at(2026, 9, 15), at(2026, 11, 20), at(2026, 9, 15), at(2026, 12, 15)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := Reminder{DueAt: &c.due, RepeatMonthDay: c.day}
			if got := r.FirstDue(loc); !got.Equal(c.first) {
				t.Errorf("first = %v, want %v", got, c.first)
			}
			if got := r.NextDue(c.now, loc); !got.Equal(c.next) {
				t.Errorf("next = %v, want %v", got, c.next)
			}
		})
	}

	t.Run("feb uses its last day and march goes back to 31", func(t *testing.T) {
		jan := at(2026, 1, 31)
		r := Reminder{DueAt: &jan, RepeatMonthDay: 31}
		feb := r.NextDue(jan, loc)
		if want := at(2026, 2, 28); !feb.Equal(want) {
			t.Fatalf("feb = %v, want %v", feb, want)
		}
		r.DueAt = &feb
		if got, want := r.NextDue(feb, loc), at(2026, 3, 31); !got.Equal(want) {
			t.Fatalf("mar = %v, want %v", got, want)
		}
	})

	t.Run("weekdays and month day together are refused", func(t *testing.T) {
		due := at(2026, 9, 15)
		r := Reminder{Title: "x", DueAt: &due, RepeatMonthDay: 15, RepeatDays: []time.Weekday{time.Monday}}
		if err := r.Validate(); err == nil {
			t.Fatal("expected validation error")
		}
	})
}
