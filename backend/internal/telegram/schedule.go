package telegram

import (
	"time"

	"github.com/rafael/estus-vault/backend/internal/domain"
	"github.com/rafael/estus-vault/backend/internal/store/postgres"
)

type JobKind string

const (
	JobMorning  JobKind = "morning"
	JobEvening  JobKind = "evening"
	JobReminder JobKind = "reminder"
	JobEvent    JobKind = "event"
)

// Job is one message the scheduler should send.
type Job struct {
	Kind     JobKind
	Ref      string // reminders and events: the telegram_sent key
	Reminder domain.Reminder
	Event    domain.Event
}

const (
	// A morning summary missed while the Mac slept still goes out until noon;
	// after that it would be stale.
	morningCatchUpHour = 12
	reminderCatchUp    = 12 * time.Hour
)

// Due decides what to send at now, in the owner's time zone. It has no side
// effects: the scheduler still checks telegram_sent before sending an alert,
// so returning one again on the next minute is harmless.
func Due(now time.Time, s postgres.TelegramSettings, reminders []domain.Reminder, events []domain.Event) []Job {
	var jobs []Job
	today := calendarDay(now)
	clock := now.Format("15:04") // zero-padded, so it compares as a string
	if s.MorningEnabled && clock >= s.MorningTime && now.Hour() < morningCatchUpHour && !sentOn(s.LastMorningOn, today) {
		jobs = append(jobs, Job{Kind: JobMorning})
	}
	if s.EveningEnabled && clock >= s.EveningTime && !sentOn(s.LastEveningOn, today) {
		jobs = append(jobs, Job{Kind: JobEvening})
	}
	if s.RemindersEnabled {
		for _, r := range reminders {
			if r.Done || r.DueAt == nil || r.DueAt.After(now) || !r.DueAt.After(now.Add(-reminderCatchUp)) {
				continue
			}
			jobs = append(jobs, Job{Kind: JobReminder, Ref: r.ID + "@" + r.DueAt.UTC().Format(time.RFC3339), Reminder: r})
		}
	}
	if s.EventsEnabled {
		lead := time.Duration(s.EventsMinutesBefore) * time.Minute
		for _, e := range events {
			// An all-day event has no hour to warn about; the morning report lists it.
			if allDay(e, now.Location()) || !now.Before(e.StartsAt) || now.Before(e.StartsAt.Add(-lead)) {
				continue
			}
			jobs = append(jobs, Job{Kind: JobEvent, Ref: e.ID + "@" + e.StartsAt.UTC().Format(time.RFC3339), Event: e})
		}
	}
	return jobs
}

func sentOn(day *time.Time, today time.Time) bool {
	return day != nil && calendarDay(*day).Equal(today)
}
