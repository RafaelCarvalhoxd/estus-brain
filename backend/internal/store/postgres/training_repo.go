package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/rafael/estus-vault/backend/internal/domain"
)

type WorkoutRepo struct{ db *DB }

func NewWorkoutRepo(db *DB) *WorkoutRepo { return &WorkoutRepo{db: db} }

const workoutColumns = `id, name, focus, weekdays, notes, created_at, updated_at`

func scanWorkout(row scanner, w *domain.Workout) error {
	return row.Scan(&w.ID, &w.Name, &w.Focus, &w.Weekdays, &w.Notes, &w.CreatedAt, &w.UpdatedAt)
}

// List returns every workout with its exercises, in two queries rather than
// one per workout — a plan is a handful of workouts, read all at once.
func (r *WorkoutRepo) List(ctx context.Context) ([]domain.Workout, error) {
	rows, err := r.db.Pool.Query(ctx, `select `+workoutColumns+` from workouts order by created_at`)
	if err != nil {
		return nil, fmt.Errorf("list workouts: %w", err)
	}
	defer rows.Close()

	var out []domain.Workout
	index := map[string]int{}
	for rows.Next() {
		var w domain.Workout
		if err := scanWorkout(rows, &w); err != nil {
			return nil, fmt.Errorf("scan workout: %w", err)
		}
		index[w.ID] = len(out)
		out = append(out, w)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	erows, err := r.db.Pool.Query(ctx, `
		select workout_id, id, name, sets, reps, weight, rest_seconds, notes
		from workout_exercises
		order by workout_id, position`)
	if err != nil {
		return nil, fmt.Errorf("list exercises: %w", err)
	}
	defer erows.Close()
	for erows.Next() {
		var workoutID string
		var e domain.Exercise
		if err := erows.Scan(&workoutID, &e.ID, &e.Name, &e.Sets, &e.Reps, &e.Weight, &e.RestSeconds, &e.Notes); err != nil {
			return nil, fmt.Errorf("scan exercise: %w", err)
		}
		if i, ok := index[workoutID]; ok {
			out[i].Exercises = append(out[i].Exercises, e)
		}
	}
	return out, erows.Err()
}

func (r *WorkoutRepo) Get(ctx context.Context, id string) (domain.Workout, error) {
	var w domain.Workout
	if err := scanWorkout(r.db.Pool.QueryRow(ctx, `select `+workoutColumns+` from workouts where id = $1`, id), &w); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Workout{}, domain.ErrNotFound
		}
		return domain.Workout{}, fmt.Errorf("get workout: %w", err)
	}
	rows, err := r.db.Pool.Query(ctx, `
		select id, name, sets, reps, weight, rest_seconds, notes
		from workout_exercises where workout_id = $1 order by position`, id)
	if err != nil {
		return domain.Workout{}, fmt.Errorf("get exercises: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var e domain.Exercise
		if err := rows.Scan(&e.ID, &e.Name, &e.Sets, &e.Reps, &e.Weight, &e.RestSeconds, &e.Notes); err != nil {
			return domain.Workout{}, fmt.Errorf("scan exercise: %w", err)
		}
		w.Exercises = append(w.Exercises, e)
	}
	return w, rows.Err()
}

func insertExercises(ctx context.Context, tx pgx.Tx, workoutID string, exercises []domain.Exercise) error {
	for i, e := range exercises {
		_, err := tx.Exec(ctx, `
			insert into workout_exercises (workout_id, position, name, sets, reps, weight, rest_seconds, notes)
			values ($1, $2, $3, $4, $5, $6, $7, $8)`,
			workoutID, i, e.Name, e.Sets, e.Reps, e.Weight, e.RestSeconds, e.Notes,
		)
		if err != nil {
			return fmt.Errorf("insert exercise %d: %w", i+1, err)
		}
	}
	return nil
}

// Create writes the workout and its exercises in one transaction, so a
// failure halfway never leaves a workout with half its exercises.
func (r *WorkoutRepo) Create(ctx context.Context, w domain.Workout) (domain.Workout, error) {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return domain.Workout{}, fmt.Errorf("begin create workout: %w", err)
	}
	defer tx.Rollback(ctx)

	var id string
	err = tx.QueryRow(ctx, `
		insert into workouts (name, focus, weekdays, notes)
		values ($1, $2, $3, $4) returning id`,
		w.Name, w.Focus, w.Weekdays, w.Notes,
	).Scan(&id)
	if err != nil {
		return domain.Workout{}, fmt.Errorf("create workout: %w", err)
	}
	if err := insertExercises(ctx, tx, id, w.Exercises); err != nil {
		return domain.Workout{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Workout{}, fmt.Errorf("commit create workout: %w", err)
	}
	return r.Get(ctx, id)
}

// Update rewrites the workout and replaces its exercise list whole — the
// edit form always sends the full list, in order.
func (r *WorkoutRepo) Update(ctx context.Context, id string, w domain.Workout) (domain.Workout, error) {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return domain.Workout{}, fmt.Errorf("begin update workout: %w", err)
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `
		update workouts set name = $2, focus = $3, weekdays = $4, notes = $5, updated_at = now()
		where id = $1`,
		id, w.Name, w.Focus, w.Weekdays, w.Notes,
	)
	if err != nil {
		return domain.Workout{}, fmt.Errorf("update workout: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.Workout{}, domain.ErrNotFound
	}
	if _, err := tx.Exec(ctx, `delete from workout_exercises where workout_id = $1`, id); err != nil {
		return domain.Workout{}, fmt.Errorf("clear exercises: %w", err)
	}
	if err := insertExercises(ctx, tx, id, w.Exercises); err != nil {
		return domain.Workout{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Workout{}, fmt.Errorf("commit update workout: %w", err)
	}
	return r.Get(ctx, id)
}

func (r *WorkoutRepo) Delete(ctx context.Context, id string) error {
	tag, err := r.db.Pool.Exec(ctx, `delete from workouts where id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete workout: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}
