package telegram

import (
	"context"
	"log/slog"
	"time"

	"github.com/rafael/estus-vault/backend/internal/domain"
	"github.com/rafael/estus-vault/backend/internal/service"
	"github.com/rafael/estus-vault/backend/internal/store/postgres"
)

// ServiceData reads the owner's data through the same services the API and
// the assistant use.
type ServiceData struct {
	ReminderSvc     *service.ReminderService
	EventSvc        *service.EventService
	BillSvc         *service.BillService
	TrainingSvc     *service.TrainingService
	HabitSvc        *service.HabitService
	TransactionRepo *postgres.TransactionRepo
}

func (d ServiceData) Reminders(ctx context.Context) ([]domain.Reminder, error) {
	return d.ReminderSvc.List(ctx)
}

func (d ServiceData) Events(ctx context.Context, from, to time.Time) ([]domain.Event, error) {
	return d.EventSvc.ListRange(ctx, from, to)
}

func (d ServiceData) CompleteReminder(ctx context.Context, id string) (domain.Reminder, error) {
	return d.ReminderSvc.SetDone(ctx, id, true)
}

// Snapshot loads what the reports read. A part that fails to load is left
// empty and logged, so the report still goes out without that section.
func (d ServiceData) Snapshot(ctx context.Context, now time.Time) Snapshot {
	s := Snapshot{Now: now}
	warn := func(part string, err error) {
		if err != nil {
			slog.Warn("telegram: report data", "part", part, "error", err)
		}
	}
	var err error
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	s.Events, err = d.EventSvc.ListRange(ctx, start, start.AddDate(0, 0, 2))
	warn("agenda", err)
	s.Reminders, err = d.ReminderSvc.List(ctx)
	warn("lembretes", err)
	s.Bills, err = d.BillSvc.List(ctx, nil, true)
	warn("contas", err)
	s.Workouts, err = d.TrainingSvc.List(ctx)
	warn("treino", err)
	s.Habits, err = d.HabitSvc.List(ctx)
	warn("hábitos", err)
	s.SpentToday, err = d.TransactionRepo.SpentOn(ctx, calendarDay(now))
	warn("gastos do dia", err)
	s.MonthTotal, err = d.TransactionRepo.MonthTotal(ctx, domain.YearMonthOf(now))
	warn("gastos do mês", err)
	return s
}
