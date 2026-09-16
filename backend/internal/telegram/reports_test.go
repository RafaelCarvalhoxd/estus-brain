package telegram

import (
	"strings"
	"testing"
	"time"

	"github.com/rafael/estus-vault/backend/internal/domain"
)

func saoPaulo(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

func utcDay(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

func assertContains(t *testing.T, text string, want, not []string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(text, w) {
			t.Errorf("missing %q in:\n%s", w, text)
		}
	}
	for _, n := range not {
		if strings.Contains(text, n) {
			t.Errorf("unexpected %q in:\n%s", n, text)
		}
	}
}

func TestMorningReport(t *testing.T) {
	loc := saoPaulo(t)
	now := time.Date(2026, time.September, 15, 7, 0, 0, 0, loc) // a Tuesday
	today := time.Date(2026, time.September, 15, 14, 0, 0, 0, loc)
	old := time.Date(2026, time.September, 12, 9, 0, 0, 0, loc)
	s := Snapshot{
		Now: now,
		Events: []domain.Event{
			{Title: "Dentista", Location: "Rua X", StartsAt: time.Date(2026, 9, 15, 15, 0, 0, 0, loc), EndsAt: time.Date(2026, 9, 15, 16, 0, 0, 0, loc)},
			{Title: "Reunião amanhã", StartsAt: time.Date(2026, 9, 16, 10, 0, 0, 0, loc), EndsAt: time.Date(2026, 9, 16, 11, 0, 0, 0, loc)},
		},
		Reminders: []domain.Reminder{
			{Title: "Ligar <mãe>", DueAt: &today},
			{Title: "Pagar IPVA", DueAt: &old},
			{Title: "Já feito", DueAt: &today, Done: true},
		},
		Bills: []domain.Bill{
			{Description: "Aluguel", AmountCents: 150000, DueDate: utcDay(2026, 9, 10), Direction: domain.BillPayable},
			{Description: "Salário", AmountCents: 900000, DueDate: utcDay(2026, 9, 15), Direction: domain.BillReceivable},
			{Description: "Internet", AmountCents: 10000, DueDate: utcDay(2026, 9, 20), Direction: domain.BillPayable},
		},
		Workouts: []domain.Workout{{Name: "Treino A", Focus: "Peito", Weekdays: 1 << 2}, {Name: "Treino B", Weekdays: 1 << 3}},
		Habits:   []domain.Habit{{Name: "Ler", Weekdays: 127, Target: 1, StartDay: utcDay(2026, 9, 1)}, {Name: "Arquivado", Weekdays: 127, Archived: true}},
	}
	assertContains(t, MorningReport(s),
		[]string{"terça-feira, 15/09", "• 15:00 Dentista · Rua X", "• 14:00 Ligar &lt;mãe&gt;", "• ⚠️ Pagar IPVA (desde 12/09)", "• Aluguel · R$ 1.500,00 (venceu 10/09)", "• Treino A — Peito", "• Ler"},
		[]string{"Reunião amanhã", "Já feito", "Salário", "Internet", "Treino B", "Arquivado", "Dia livre"})
}

func TestMorningReportFreeDay(t *testing.T) {
	now := time.Date(2026, time.September, 15, 7, 0, 0, 0, saoPaulo(t))
	assertContains(t, MorningReport(Snapshot{Now: now}), []string{"Bom dia", "Dia livre"}, []string{"Agenda"})
}

func TestEveningReport(t *testing.T) {
	loc := saoPaulo(t)
	now := time.Date(2026, time.September, 15, 21, 0, 0, 0, loc)
	due := time.Date(2026, time.September, 15, 18, 0, 0, 0, loc)
	s := Snapshot{
		Now:        now,
		SpentToday: 4500,
		MonthTotal: 164500,
		Habits: []domain.Habit{
			{Name: "Ler", Weekdays: 127, Target: 1, Logs: map[string]int{"2026-09-15": 1}},
			{Name: "Correr", Weekdays: 127, Target: 1, Logs: map[string]int{}},
		},
		Reminders: []domain.Reminder{{Title: "Comprar pão", DueAt: &due}, {Title: "Feito hoje", DueAt: &due, Done: true}},
		Events:    []domain.Event{{Title: "Reunião", StartsAt: time.Date(2026, 9, 16, 9, 30, 0, 0, loc), EndsAt: time.Date(2026, 9, 16, 10, 0, 0, 0, loc)}},
	}
	assertContains(t, EveningReport(s),
		[]string{"Fechamento de terça-feira, 15/09", "• Hoje: R$ 45,00", "• No mês: R$ 1.645,00", "• Correr", "• Comprar pão", "• 09:30 Reunião"},
		[]string{"• Ler", "Feito hoje"})
}

func TestAlerts(t *testing.T) {
	loc := saoPaulo(t)
	if got := ReminderAlert(domain.Reminder{Title: "A & B"}); got != "🔔 <b>A &amp; B</b>" {
		t.Errorf("ReminderAlert = %q", got)
	}
	if got := ReminderDone(domain.Reminder{Title: "Pagar luz"}); got != "✅ <s>Pagar luz</s>" {
		t.Errorf("ReminderDone = %q", got)
	}
	e := domain.Event{Title: "Dentista", Location: "Rua X", StartsAt: time.Date(2026, 9, 15, 15, 0, 0, 0, loc)}
	if got := EventAlert(e, loc); got != "📅 <b>Dentista</b> às 15:00 · Rua X" {
		t.Errorf("EventAlert = %q", got)
	}
}

func TestReportsListAllDayEventsWithoutATime(t *testing.T) {
	loc := saoPaulo(t)
	now := time.Date(2026, time.September, 15, 7, 0, 0, 0, loc)
	today := time.Date(2026, time.September, 15, 0, 0, 0, 0, loc)
	tomorrow := today.AddDate(0, 0, 1)
	s := Snapshot{Now: now, Events: []domain.Event{
		{Title: "Feriado", StartsAt: today, EndsAt: tomorrow},
		{Title: "Congresso", Location: "Centro", StartsAt: tomorrow, EndsAt: tomorrow.AddDate(0, 0, 2)},
	}}
	assertContains(t, MorningReport(s), []string{"• Dia todo: Feriado"}, []string{"00:00"})
	s.Now = time.Date(2026, time.September, 15, 21, 0, 0, 0, loc)
	assertContains(t, EveningReport(s), []string{"• Dia todo: Congresso · Centro"}, []string{"00:00"})
}
