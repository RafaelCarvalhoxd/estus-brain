package telegram

import (
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/rafael/estus-vault/backend/internal/domain"
)

// Snapshot is what the reports read: the owner's data around now, loaded
// once per report. Now carries the owner's time zone.
type Snapshot struct {
	Now        time.Time
	Events     []domain.Event // today and tomorrow
	Reminders  []domain.Reminder
	Bills      []domain.Bill // open ones
	Workouts   []domain.Workout
	Habits     []domain.Habit
	SpentToday domain.Cents
	MonthTotal domain.Cents
}

var weekdaysPT = []string{"domingo", "segunda-feira", "terça-feira", "quarta-feira", "quinta-feira", "sexta-feira", "sábado"}

// calendarDay is t's calendar day as a UTC-midnight date, the way bills and
// habits keep bare dates.
func calendarDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func localDay(t time.Time, loc *time.Location) time.Time { return calendarDay(t.In(loc)) }

// allDay reports whether e fills whole days in loc: Google's all-day events
// arrive as midnight to midnight, and a time like 00:00 means nothing for them.
func allDay(e domain.Event, loc *time.Location) bool {
	start, end := e.StartsAt.In(loc), e.EndsAt.In(loc)
	return start.Equal(localMidnight(start)) && end.Equal(localMidnight(end)) && end.Sub(start) >= 24*time.Hour
}

func localMidnight(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func esc(s string) string { return html.EscapeString(s) }

func dayLabel(day time.Time) string {
	return fmt.Sprintf("%s, %s", weekdaysPT[day.Weekday()], day.Format("02/01"))
}

// section is a titled list, or nothing when the list is empty.
func section(title string, lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return title + "\n" + strings.Join(lines, "\n")
}

func joinSections(header string, sections ...string) string {
	out := []string{header}
	for _, s := range sections {
		if s != "" {
			out = append(out, s)
		}
	}
	return strings.Join(out, "\n\n")
}

func eventsOn(events []domain.Event, day time.Time, loc *time.Location) []string {
	var lines []string
	for _, e := range events {
		if !localDay(e.StartsAt, loc).Equal(day) {
			continue
		}
		line := "• " + e.StartsAt.In(loc).Format("15:04") + " " + esc(e.Title)
		if allDay(e, loc) {
			line = "• Dia todo: " + esc(e.Title)
		}
		if e.Location != "" {
			line += " · " + esc(e.Location)
		}
		lines = append(lines, line)
	}
	return lines
}

func habitsDue(habits []domain.Habit, day time.Time, onlyUndone bool) []string {
	var lines []string
	for _, h := range habits {
		if h.Archived || !h.ScheduledOn(day) || day.Before(h.StartDay) || (onlyUndone && h.DoneOn(day)) {
			continue
		}
		lines = append(lines, "• "+esc(h.Name))
	}
	return lines
}

func MorningReport(s Snapshot) string {
	loc := s.Now.Location()
	today := calendarDay(s.Now)

	var reminders []string
	for _, r := range s.Reminders {
		if r.Done || r.DueAt == nil {
			continue
		}
		switch due := localDay(*r.DueAt, loc); {
		case due.Equal(today):
			reminders = append(reminders, "• "+r.DueAt.In(loc).Format("15:04")+" "+esc(r.Title))
		case due.Before(today):
			reminders = append(reminders, "• ⚠️ "+esc(r.Title)+" (desde "+due.Format("02/01")+")")
		}
	}

	var bills []string
	for _, b := range s.Bills {
		if b.PaidAt != nil || b.Direction != domain.BillPayable || b.DueDate.After(today) {
			continue
		}
		line := "• " + esc(b.Description) + " · " + b.AmountCents.String()
		if b.DueDate.Before(today) {
			line += " (venceu " + b.DueDate.Format("02/01") + ")"
		}
		bills = append(bills, line)
	}

	var training []string
	for _, w := range s.Workouts {
		if w.Weekdays&(1<<int(today.Weekday())) == 0 {
			continue
		}
		line := "• " + esc(w.Name)
		if w.Focus != "" {
			line += " — " + esc(w.Focus)
		}
		training = append(training, line)
	}

	agenda := eventsOn(s.Events, today, loc)
	habits := habitsDue(s.Habits, today, false)
	header := "☀️ <b>Bom dia! Hoje é " + dayLabel(today) + "</b>"
	if len(agenda)+len(reminders)+len(bills)+len(training)+len(habits) == 0 {
		return joinSections(header, "Dia livre: nada na agenda, lembretes ou contas.")
	}
	return joinSections(header,
		section("📅 <b>Agenda</b>", agenda),
		section("🔔 <b>Lembretes</b>", reminders),
		section("💳 <b>Contas a pagar</b>", bills),
		section("🏋️ <b>Treino</b>", training),
		section("✅ <b>Hábitos</b>", habits),
	)
}

func EveningReport(s Snapshot) string {
	loc := s.Now.Location()
	today := calendarDay(s.Now)

	var reminders []string
	for _, r := range s.Reminders {
		if !r.Done && r.DueAt != nil && localDay(*r.DueAt, loc).Equal(today) {
			reminders = append(reminders, "• "+esc(r.Title))
		}
	}
	return joinSections("🌙 <b>Fechamento de "+dayLabel(today)+"</b>",
		section("💸 <b>Gastos</b>", []string{"• Hoje: " + s.SpentToday.String(), "• No mês: " + s.MonthTotal.String()}),
		section("⏳ <b>Hábitos que faltam</b>", habitsDue(s.Habits, today, true)),
		section("🔔 <b>Lembretes em aberto</b>", reminders),
		section("📅 <b>Amanhã</b>", eventsOn(s.Events, today.AddDate(0, 0, 1), loc)),
	)
}

func ReminderAlert(r domain.Reminder) string { return "🔔 <b>" + esc(r.Title) + "</b>" }

// ReminderDone replaces an alert once its button marked the reminder done.
func ReminderDone(r domain.Reminder) string { return "✅ <s>" + esc(r.Title) + "</s>" }

func EventAlert(e domain.Event, loc *time.Location) string {
	text := "📅 <b>" + esc(e.Title) + "</b> às " + e.StartsAt.In(loc).Format("15:04")
	if e.Location != "" {
		text += " · " + esc(e.Location)
	}
	return text
}
