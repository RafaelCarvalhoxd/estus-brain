package telegram

import (
	"testing"
	"time"

	"github.com/rafael/estus-vault/backend/internal/domain"
	"github.com/rafael/estus-vault/backend/internal/store/postgres"
)

func scheduleSettings() postgres.TelegramSettings {
	return postgres.TelegramSettings{
		MorningEnabled: true, MorningTime: "07:00",
		EveningEnabled: true, EveningTime: "21:00",
		RemindersEnabled: true, EventsEnabled: true, EventsMinutesBefore: 30,
	}
}

func kinds(jobs []Job) []JobKind {
	var out []JobKind
	for _, j := range jobs {
		out = append(out, j.Kind)
	}
	return out
}

func TestDueReports(t *testing.T) {
	loc := saoPaulo(t)
	at := func(h, m int) time.Time { return time.Date(2026, time.September, 15, h, m, 0, 0, loc) }
	today, yesterday := utcDay(2026, 9, 15), utcDay(2026, 9, 14)
	cases := []struct {
		name  string
		now   time.Time
		tweak func(*postgres.TelegramSettings)
		want  []JobKind
	}{
		{"morning at its time", at(7, 0), nil, []JobKind{JobMorning}},
		{"not yet", at(6, 59), nil, nil},
		{"missed morning caught up before noon", at(11, 30), nil, []JobKind{JobMorning}},
		{"missed morning dropped at noon", at(12, 0), nil, nil},
		{"morning already sent today", at(8, 0), func(s *postgres.TelegramSettings) { s.LastMorningOn = &today }, nil},
		{"sent yesterday counts as not sent", at(8, 0), func(s *postgres.TelegramSettings) { s.LastMorningOn = &yesterday }, []JobKind{JobMorning}},
		{"morning disabled", at(8, 0), func(s *postgres.TelegramSettings) { s.MorningEnabled = false }, nil},
		{"evening at its time", at(21, 5), nil, []JobKind{JobEvening}},
		{"evening caught up until midnight", at(23, 59), nil, []JobKind{JobEvening}},
		{"evening not yet", at(20, 59), nil, nil},
		{"evening already sent", at(22, 0), func(s *postgres.TelegramSettings) { s.LastEveningOn = &today }, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := scheduleSettings()
			if tc.tweak != nil {
				tc.tweak(&s)
			}
			got := kinds(Due(tc.now, s, nil, nil))
			if len(got) != len(tc.want) || (len(got) == 1 && got[0] != tc.want[0]) {
				t.Fatalf("Due = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDueReminders(t *testing.T) {
	loc := saoPaulo(t)
	now := time.Date(2026, time.September, 15, 15, 0, 0, 0, loc)
	ago := func(d time.Duration) *time.Time { t := now.Add(-d); return &t }
	s := scheduleSettings()
	s.MorningEnabled, s.EveningEnabled = false, false

	reminders := []domain.Reminder{
		{ID: "due", Title: "a", DueAt: ago(10 * time.Minute)},
		{ID: "future", Title: "b", DueAt: ago(-5 * time.Minute)},
		{ID: "late", Title: "c", DueAt: ago(11 * time.Hour)},
		{ID: "too-late", Title: "d", DueAt: ago(13 * time.Hour)},
		{ID: "done", Title: "e", DueAt: ago(time.Minute), Done: true},
		{ID: "no-date", Title: "f"},
	}
	jobs := Due(now, s, reminders, nil)
	if len(jobs) != 2 || jobs[0].Reminder.ID != "due" || jobs[1].Reminder.ID != "late" {
		t.Fatalf("jobs = %+v", jobs)
	}
	if want := "due@" + reminders[0].DueAt.UTC().Format(time.RFC3339); jobs[0].Ref != want || jobs[0].Kind != JobReminder {
		t.Fatalf("job = %+v, want ref %q", jobs[0], want)
	}
	s.RemindersEnabled = false
	if jobs := Due(now, s, reminders, nil); len(jobs) != 0 {
		t.Fatalf("reminders disabled, jobs = %+v", jobs)
	}
}

func TestDueEvents(t *testing.T) {
	loc := saoPaulo(t)
	now := time.Date(2026, time.September, 15, 15, 0, 0, 0, loc)
	s := scheduleSettings()
	s.MorningEnabled, s.EveningEnabled = false, false
	event := func(id string, in time.Duration) domain.Event {
		return domain.Event{ID: id, Title: id, StartsAt: now.Add(in), EndsAt: now.Add(in + time.Hour)}
	}
	jobs := Due(now, s, nil, []domain.Event{event("soon", 20*time.Minute), event("later", 31*time.Minute), event("started", -time.Minute)})
	if len(jobs) != 1 || jobs[0].Event.ID != "soon" || jobs[0].Kind != JobEvent {
		t.Fatalf("jobs = %+v", jobs)
	}
	moved := Due(now, s, nil, []domain.Event{event("soon", 25*time.Minute)})
	if len(moved) != 1 || moved[0].Ref == jobs[0].Ref {
		t.Fatalf("a rescheduled event needs a new ref: %q vs %q", moved[0].Ref, jobs[0].Ref)
	}
}

func TestDueSkipsAllDayEvents(t *testing.T) {
	loc := saoPaulo(t)
	now := time.Date(2026, time.September, 14, 23, 40, 0, 0, loc)
	s := scheduleSettings()
	s.MorningEnabled, s.EveningEnabled = false, false
	midnight := time.Date(2026, time.September, 15, 0, 0, 0, 0, loc)
	events := []domain.Event{
		{ID: "holiday", Title: "Feriado", StartsAt: midnight, EndsAt: midnight.AddDate(0, 0, 1)},
		{ID: "trip", Title: "Viagem", StartsAt: midnight, EndsAt: midnight.AddDate(0, 0, 3)},
		{ID: "late", Title: "Plantão", StartsAt: midnight, EndsAt: midnight.Add(time.Hour)},
	}
	jobs := Due(now, s, nil, events)
	if len(jobs) != 1 || jobs[0].Event.ID != "late" {
		t.Fatalf("jobs = %+v", jobs)
	}
}
