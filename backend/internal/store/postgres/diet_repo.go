package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/rafael/estus-vault/backend/internal/domain"
)

type MealRepo struct{ db *DB }

func NewMealRepo(db *DB) *MealRepo { return &MealRepo{db: db} }

const mealColumns = `id, name, time_of_day, weekdays, notes, created_at, updated_at`

func scanMeal(row scanner, m *domain.Meal) error {
	return row.Scan(&m.ID, &m.Name, &m.Time, &m.Weekdays, &m.Notes, &m.CreatedAt, &m.UpdatedAt)
}

// List returns every meal with its foods, ordered by time of day — the order
// the diet screen reads them in.
func (r *MealRepo) List(ctx context.Context) ([]domain.Meal, error) {
	rows, err := r.db.Pool.Query(ctx, `select `+mealColumns+` from meals order by time_of_day, created_at`)
	if err != nil {
		return nil, fmt.Errorf("list meals: %w", err)
	}
	defer rows.Close()

	var out []domain.Meal
	index := map[string]int{}
	for rows.Next() {
		var m domain.Meal
		if err := scanMeal(rows, &m); err != nil {
			return nil, fmt.Errorf("scan meal: %w", err)
		}
		index[m.ID] = len(out)
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	irows, err := r.db.Pool.Query(ctx, `
		select meal_id, id, food, quantity, kcal, protein_g, carbs_g, fat_g
		from meal_items
		order by meal_id, position`)
	if err != nil {
		return nil, fmt.Errorf("list meal items: %w", err)
	}
	defer irows.Close()
	for irows.Next() {
		var mealID string
		var it domain.MealItem
		if err := irows.Scan(&mealID, &it.ID, &it.Food, &it.Quantity, &it.Kcal, &it.ProteinG, &it.CarbsG, &it.FatG); err != nil {
			return nil, fmt.Errorf("scan meal item: %w", err)
		}
		if i, ok := index[mealID]; ok {
			out[i].Items = append(out[i].Items, it)
		}
	}
	return out, irows.Err()
}

func (r *MealRepo) Get(ctx context.Context, id string) (domain.Meal, error) {
	var m domain.Meal
	if err := scanMeal(r.db.Pool.QueryRow(ctx, `select `+mealColumns+` from meals where id = $1`, id), &m); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Meal{}, domain.ErrNotFound
		}
		return domain.Meal{}, fmt.Errorf("get meal: %w", err)
	}
	rows, err := r.db.Pool.Query(ctx, `
		select id, food, quantity, kcal, protein_g, carbs_g, fat_g
		from meal_items where meal_id = $1 order by position`, id)
	if err != nil {
		return domain.Meal{}, fmt.Errorf("get meal items: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var it domain.MealItem
		if err := rows.Scan(&it.ID, &it.Food, &it.Quantity, &it.Kcal, &it.ProteinG, &it.CarbsG, &it.FatG); err != nil {
			return domain.Meal{}, fmt.Errorf("scan meal item: %w", err)
		}
		m.Items = append(m.Items, it)
	}
	return m, rows.Err()
}

func insertMealItems(ctx context.Context, tx pgx.Tx, mealID string, items []domain.MealItem) error {
	for i, it := range items {
		_, err := tx.Exec(ctx, `
			insert into meal_items (meal_id, position, food, quantity, kcal, protein_g, carbs_g, fat_g)
			values ($1, $2, $3, $4, $5, $6, $7, $8)`,
			mealID, i, it.Food, it.Quantity, it.Kcal, it.ProteinG, it.CarbsG, it.FatG,
		)
		if err != nil {
			return fmt.Errorf("insert meal item %d: %w", i+1, err)
		}
	}
	return nil
}

func (r *MealRepo) Create(ctx context.Context, m domain.Meal) (domain.Meal, error) {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return domain.Meal{}, fmt.Errorf("begin create meal: %w", err)
	}
	defer tx.Rollback(ctx)

	var id string
	err = tx.QueryRow(ctx, `
		insert into meals (name, time_of_day, weekdays, notes)
		values ($1, $2, $3, $4) returning id`,
		m.Name, m.Time, m.Weekdays, m.Notes,
	).Scan(&id)
	if err != nil {
		return domain.Meal{}, fmt.Errorf("create meal: %w", err)
	}
	if err := insertMealItems(ctx, tx, id, m.Items); err != nil {
		return domain.Meal{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Meal{}, fmt.Errorf("commit create meal: %w", err)
	}
	return r.Get(ctx, id)
}

// Update rewrites the meal and replaces its food list whole.
func (r *MealRepo) Update(ctx context.Context, id string, m domain.Meal) (domain.Meal, error) {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return domain.Meal{}, fmt.Errorf("begin update meal: %w", err)
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `
		update meals set name = $2, time_of_day = $3, weekdays = $4, notes = $5, updated_at = now()
		where id = $1`,
		id, m.Name, m.Time, m.Weekdays, m.Notes,
	)
	if err != nil {
		return domain.Meal{}, fmt.Errorf("update meal: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.Meal{}, domain.ErrNotFound
	}
	if _, err := tx.Exec(ctx, `delete from meal_items where meal_id = $1`, id); err != nil {
		return domain.Meal{}, fmt.Errorf("clear meal items: %w", err)
	}
	if err := insertMealItems(ctx, tx, id, m.Items); err != nil {
		return domain.Meal{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Meal{}, fmt.Errorf("commit update meal: %w", err)
	}
	return r.Get(ctx, id)
}

func (r *MealRepo) Delete(ctx context.Context, id string) error {
	tag, err := r.db.Pool.Exec(ctx, `delete from meals where id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete meal: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *MealRepo) Targets(ctx context.Context) (domain.DietTargets, error) {
	var t domain.DietTargets
	err := r.db.Pool.QueryRow(ctx, `select kcal, protein_g, carbs_g, fat_g from diet_targets where id = 1`).
		Scan(&t.Kcal, &t.ProteinG, &t.CarbsG, &t.FatG)
	if err != nil {
		return domain.DietTargets{}, fmt.Errorf("get diet targets: %w", err)
	}
	return t, nil
}

func (r *MealRepo) SetTargets(ctx context.Context, t domain.DietTargets) (domain.DietTargets, error) {
	_, err := r.db.Pool.Exec(ctx, `
		insert into diet_targets (id, kcal, protein_g, carbs_g, fat_g, updated_at)
		values (1, $1, $2, $3, $4, now())
		on conflict (id) do update
		set kcal = excluded.kcal, protein_g = excluded.protein_g, carbs_g = excluded.carbs_g,
		    fat_g = excluded.fat_g, updated_at = now()`,
		t.Kcal, t.ProteinG, t.CarbsG, t.FatG,
	)
	if err != nil {
		return domain.DietTargets{}, fmt.Errorf("set diet targets: %w", err)
	}
	return t, nil
}
