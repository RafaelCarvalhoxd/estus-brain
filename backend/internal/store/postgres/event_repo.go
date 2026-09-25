package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/rafael/estus-vault/backend/internal/domain"
)

type EventRepo struct{ db *DB }

func NewEventRepo(db *DB) *EventRepo { return &EventRepo{db: db} }

func (r *EventRepo) Create(ctx context.Context, e domain.Event) (domain.Event, error) {
	err := r.db.Pool.QueryRow(ctx, `
		insert into events (id, title, location, notes, starts_at, ends_at)
		values (gen_random_uuid(), $1, $2, $3, $4, $5)
		returning id, created_at`,
		e.Title, e.Location, e.Notes, e.StartsAt, e.EndsAt,
	).Scan(&e.ID, &e.CreatedAt)
	if err != nil {
		return domain.Event{}, fmt.Errorf("create event: %w", err)
	}
	return e, nil
}

// ListRange returns events overlapping [from, to), ordered by start time.
func (r *EventRepo) ListRange(ctx context.Context, from, to time.Time) ([]domain.Event, error) {
	rows, err := r.db.Pool.Query(ctx, `
		select id, title, location, notes, starts_at, ends_at, created_at
		from events
		where starts_at < $2 and ends_at > $1
		order by starts_at asc`, from, to)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	defer rows.Close()

	var out []domain.Event
	for rows.Next() {
		var e domain.Event
		if err := rows.Scan(&e.ID, &e.Title, &e.Location, &e.Notes, &e.StartsAt, &e.EndsAt, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *EventRepo) Update(ctx context.Context, e domain.Event) (domain.Event, error) {
	err := r.db.Pool.QueryRow(ctx, `
		update events set title = $2, location = $3, notes = $4, starts_at = $5, ends_at = $6
		where id = $1
		returning created_at`,
		e.ID, e.Title, e.Location, e.Notes, e.StartsAt, e.EndsAt,
	).Scan(&e.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Event{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Event{}, fmt.Errorf("update event %s: %w", e.ID, err)
	}
	return e, nil
}

func (r *EventRepo) Delete(ctx context.Context, id string) error {
	tag, err := r.db.Pool.Exec(ctx, `delete from events where id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete event: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}
