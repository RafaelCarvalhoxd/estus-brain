package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/rafael/estus-vault/backend/internal/domain"
)

type ReminderRepo struct{ db *DB }

func NewReminderRepo(db *DB) *ReminderRepo { return &ReminderRepo{db: db} }

func (r *ReminderRepo) Create(ctx context.Context, rem domain.Reminder) (domain.Reminder, error) {
	err := r.db.Pool.QueryRow(ctx, `
		insert into reminders (id, title, due_at, done, repeat_days, repeat_month_day)
		values (gen_random_uuid(), $1, $2, $3, $4, $5)
		returning id, created_at`,
		rem.Title, rem.DueAt, rem.Done, toDBDays(rem.RepeatDays), toDBMonthDay(rem.RepeatMonthDay),
	).Scan(&rem.ID, &rem.CreatedAt)
	if err != nil {
		return domain.Reminder{}, fmt.Errorf("create reminder: %w", err)
	}
	return rem, nil
}

const reminderColumns = `id, title, due_at, done, repeat_days, repeat_month_day, created_at`

func scanReminder(row pgx.Row) (domain.Reminder, error) {
	var rem domain.Reminder
	var days []int16
	var monthDay *int16
	if err := row.Scan(&rem.ID, &rem.Title, &rem.DueAt, &rem.Done, &days, &monthDay, &rem.CreatedAt); err != nil {
		return domain.Reminder{}, err
	}
	if monthDay != nil {
		rem.RepeatMonthDay = int(*monthDay)
	}
	for _, d := range days {
		rem.RepeatDays = append(rem.RepeatDays, time.Weekday(d))
	}
	return rem, nil
}

func toDBMonthDay(day int) *int16 {
	if day == 0 {
		return nil
	}
	d := int16(day)
	return &d
}

func toDBDays(days []time.Weekday) []int16 {
	out := make([]int16, len(days))
	for i, d := range days {
		out[i] = int16(d)
	}
	return out
}

func (r *ReminderRepo) Get(ctx context.Context, id string) (domain.Reminder, error) {
	rem, err := scanReminder(r.db.Pool.QueryRow(ctx, `select `+reminderColumns+` from reminders where id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Reminder{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Reminder{}, fmt.Errorf("get reminder %s: %w", id, err)
	}
	return rem, nil
}

func (r *ReminderRepo) List(ctx context.Context) ([]domain.Reminder, error) {
	rows, err := r.db.Pool.Query(ctx, `
		select `+reminderColumns+`
		from reminders
		order by done asc, due_at asc nulls last`)
	if err != nil {
		return nil, fmt.Errorf("list reminders: %w", err)
	}
	defer rows.Close()

	var out []domain.Reminder
	for rows.Next() {
		rem, err := scanReminder(rows)
		if err != nil {
			return nil, fmt.Errorf("scan reminder: %w", err)
		}
		out = append(out, rem)
	}
	return out, rows.Err()
}

func (r *ReminderRepo) SetDone(ctx context.Context, id string, done bool) (domain.Reminder, error) {
	rem, err := scanReminder(r.db.Pool.QueryRow(ctx, `
		update reminders set done = $2
		where id = $1
		returning `+reminderColumns,
		id, done,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Reminder{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Reminder{}, fmt.Errorf("set reminder done: %w", err)
	}
	return rem, nil
}

func (r *ReminderRepo) Update(ctx context.Context, id string, rem domain.Reminder) (domain.Reminder, error) {
	rem, err := scanReminder(r.db.Pool.QueryRow(ctx, `
		update reminders set title = $2, due_at = $3, repeat_days = $4, repeat_month_day = $5
		where id = $1
		returning `+reminderColumns,
		id, rem.Title, rem.DueAt, toDBDays(rem.RepeatDays), toDBMonthDay(rem.RepeatMonthDay),
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Reminder{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Reminder{}, fmt.Errorf("update reminder %s: %w", id, err)
	}
	return rem, nil
}

func (r *ReminderRepo) Delete(ctx context.Context, id string) error {
	tag, err := r.db.Pool.Exec(ctx, `delete from reminders where id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete reminder: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}
