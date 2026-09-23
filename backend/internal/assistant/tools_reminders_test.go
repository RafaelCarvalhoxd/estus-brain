package assistant

import (
	"slices"
	"testing"
	"time"

	"github.com/rafael/estus-vault/backend/internal/domain"
)

func TestReminderEdit(t *testing.T) {
	r := testRegistry(t)
	due := time.Date(2026, 9, 22, 8, 0, 0, 0, r.deps.Location)
	weekly := domain.Reminder{Title: "Remédio", DueAt: &due, RepeatDays: []time.Weekday{time.Monday}}
	title := " Tomar remédio "
	zero, fifteen := 0, 15

	in, err := r.reminderEdit(weekly, &title, "", nil, nil)
	if err != nil || in.Title != "Tomar remédio" || in.DueAt != &due || !slices.Equal(in.RepeatDays, weekly.RepeatDays) {
		t.Fatalf("title only: %+v, %v", in, err)
	}

	in, _ = r.reminderEdit(weekly, nil, "", nil, &fifteen)
	if in.RepeatMonthDay != 15 || len(in.RepeatDays) != 0 {
		t.Errorf("monthly_day should replace weekdays: %+v", in)
	}

	in, _ = r.reminderEdit(domain.Reminder{Title: "x", DueAt: &due, RepeatMonthDay: 5}, nil, "", []string{"sex"}, nil)
	if in.RepeatMonthDay != 0 || !slices.Equal(in.RepeatDays, []time.Weekday{time.Friday}) {
		t.Errorf("repeat should replace monthly_day: %+v", in)
	}

	in, _ = r.reminderEdit(weekly, nil, "", []string{}, &zero)
	if len(in.RepeatDays) != 0 || in.RepeatMonthDay != 0 || in.DueAt != &due {
		t.Errorf("clearing the repeat: %+v", in)
	}

	in, _ = r.reminderEdit(domain.Reminder{Title: "x", DueAt: &due}, nil, "sem data", nil, nil)
	if in.DueAt != nil {
		t.Errorf("sem data kept a date: %v", in.DueAt)
	}

	in, _ = r.reminderEdit(domain.Reminder{Title: "x"}, nil, "", []string{"seg"}, nil)
	if in.DueAt == nil || in.DueAt.Hour() != 9 {
		t.Errorf("an undated reminder that starts repeating needs a date: %v", in.DueAt)
	}

	in, _ = r.reminderEdit(weekly, nil, "2026-10-01", nil, nil)
	if got := in.DueAt.Format("2006-01-02 15:04"); got != "2026-10-01 09:00" {
		t.Errorf("bare day = %s", got)
	}

	if _, err := r.reminderEdit(weekly, nil, "", []string{"feriado"}, nil); err == nil {
		t.Error("unknown weekday accepted")
	}
}
