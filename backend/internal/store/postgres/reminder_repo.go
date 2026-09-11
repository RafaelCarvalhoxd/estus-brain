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
		insert into reminders (id, title, due_at, done)
		values (gen_random_uuid(), $1, $2, $3)
		returning id, created_at`,
		rem.Title, rem.DueAt, rem.Done,
	).Scan(&rem.ID, &rem.CreatedAt)
	if err != nil {
		return domain.Reminder{}, fmt.Errorf("create reminder: %w", err)
	}
	return rem, nil
}

func (r *ReminderRepo) List(ctx context.Context) ([]domain.Reminder, error) {
	rows, err := r.db.Pool.Query(ctx, `
		select id, title, due_at, done, created_at
		from reminders
		order by done asc, due_at asc nulls last`)
	if err != nil {
		return nil, fmt.Errorf("list reminders: %w", err)
	}
	defer rows.Close()

	var out []domain.Reminder
	for rows.Next() {
		var rem domain.Reminder
		if err := rows.Scan(&rem.ID, &rem.Title, &rem.DueAt, &rem.Done, &rem.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan reminder: %w", err)
		}
		out = append(out, rem)
	}
	return out, rows.Err()
}

func (r *ReminderRepo) SetDone(ctx context.Context, id string, done bool) (domain.Reminder, error) {
	var rem domain.Reminder
	err := r.db.Pool.QueryRow(ctx, `
		update reminders set done = $2
		where id = $1
		returning id, title, due_at, done, created_at`,
		id, done,
	).Scan(&rem.ID, &rem.Title, &rem.DueAt, &rem.Done, &rem.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Reminder{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Reminder{}, fmt.Errorf("set reminder done: %w", err)
	}
	return rem, nil
}

func (r *ReminderRepo) Update(ctx context.Context, id, title string, dueAt *time.Time) (domain.Reminder, error) {
	var rem domain.Reminder
	err := r.db.Pool.QueryRow(ctx, `
		update reminders set title = $2, due_at = $3
		where id = $1
		returning id, title, due_at, done, created_at`,
		id, title, dueAt,
	).Scan(&rem.ID, &rem.Title, &rem.DueAt, &rem.Done, &rem.CreatedAt)
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
