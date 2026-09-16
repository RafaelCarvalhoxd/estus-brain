package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/rafael/estus-vault/backend/internal/domain"
)

type HabitRepo struct{ db *DB }

func NewHabitRepo(db *DB) *HabitRepo { return &HabitRepo{db: db} }

func habitErr(op string, err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "22P02" {
		return domain.ErrNotFound
	}
	return fmt.Errorf("%s: %w", op, err)
}

const habitColumns = `id, name, kind, target, unit, weekdays, color, archived, start_day, created_at`

func scanHabit(row scanner, h *domain.Habit) error {
	var kind string
	if err := row.Scan(&h.ID, &h.Name, &kind, &h.Target, &h.Unit, &h.Weekdays, &h.Color, &h.Archived, &h.StartDay, &h.CreatedAt); err != nil {
		return err
	}
	h.Kind = domain.HabitKind(kind)
	return nil
}

// List returns every habit with its whole log, in two queries. Streaks need
// the full history, and a person's habits are a few rows a day at most.
func (r *HabitRepo) List(ctx context.Context) ([]domain.Habit, error) {
	rows, err := r.db.Pool.Query(ctx, `select `+habitColumns+` from habits order by archived, created_at`)
	if err != nil {
		return nil, fmt.Errorf("list habits: %w", err)
	}
	defer rows.Close()

	var out []domain.Habit
	index := map[string]int{}
	for rows.Next() {
		h := domain.Habit{Logs: map[string]int{}}
		if err := scanHabit(rows, &h); err != nil {
			return nil, fmt.Errorf("scan habit: %w", err)
		}
		index[h.ID] = len(out)
		out = append(out, h)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	lrows, err := r.db.Pool.Query(ctx, `select habit_id, day, count from habit_logs`)
	if err != nil {
		return nil, fmt.Errorf("list habit logs: %w", err)
	}
	defer lrows.Close()
	for lrows.Next() {
		var habitID string
		var day time.Time
		var count int
		if err := lrows.Scan(&habitID, &day, &count); err != nil {
			return nil, fmt.Errorf("scan habit log: %w", err)
		}
		if i, ok := index[habitID]; ok {
			out[i].Logs[day.Format(domain.DayLayout)] = count
		}
	}
	return out, lrows.Err()
}

func (r *HabitRepo) Create(ctx context.Context, h domain.Habit) (domain.Habit, error) {
	out := domain.Habit{Logs: map[string]int{}}
	err := scanHabit(r.db.Pool.QueryRow(ctx, `
		insert into habits (name, kind, target, unit, weekdays, color, start_day)
		values ($1, $2, $3, $4, $5, $6, $7)
		returning `+habitColumns,
		h.Name, string(h.Kind), h.Target, h.Unit, h.Weekdays, h.Color, h.StartDay,
	), &out)
	if err != nil {
		return domain.Habit{}, fmt.Errorf("create habit: %w", err)
	}
	return out, nil
}

// Update changes the habit itself; its start day and log are left alone.
func (r *HabitRepo) Update(ctx context.Context, id string, h domain.Habit) error {
	tag, err := r.db.Pool.Exec(ctx, `
		update habits
		set name = $2, kind = $3, target = $4, unit = $5, weekdays = $6, color = $7, archived = $8
		where id = $1`,
		id, h.Name, string(h.Kind), h.Target, h.Unit, h.Weekdays, h.Color, h.Archived,
	)
	if err != nil {
		return habitErr("update habit", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *HabitRepo) Delete(ctx context.Context, id string) error {
	tag, err := r.db.Pool.Exec(ctx, `delete from habits where id = $1`, id)
	if err != nil {
		return habitErr("delete habit", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// SetLog records how much of a habit was done on a day; zero clears the day.
func (r *HabitRepo) SetLog(ctx context.Context, id string, day time.Time, count int) error {
	var exists bool
	if err := r.db.Pool.QueryRow(ctx, `select exists(select 1 from habits where id = $1)`, id).Scan(&exists); err != nil {
		return habitErr("set habit log", err)
	}
	if !exists {
		return domain.ErrNotFound
	}
	if count == 0 {
		_, err := r.db.Pool.Exec(ctx, `delete from habit_logs where habit_id = $1 and day = $2`, id, day)
		if err != nil {
			return fmt.Errorf("clear habit log: %w", err)
		}
		return nil
	}
	_, err := r.db.Pool.Exec(ctx, `
		insert into habit_logs (habit_id, day, count) values ($1, $2, $3)
		on conflict (habit_id, day) do update set count = excluded.count`,
		id, day, count,
	)
	if err != nil {
		return fmt.Errorf("set habit log: %w", err)
	}
	return nil
}
