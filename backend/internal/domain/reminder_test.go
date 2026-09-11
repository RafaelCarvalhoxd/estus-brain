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
