// Package postgres implements the repository interfaces the service layer
// depends on, using pgx directly rather than an ORM: financial queries are
// small and hand-tunable, and hiding them behind a query builder makes the
// generated SQL harder to reason about than writing it.
package postgres

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
	Pool *pgxpool.Pool
}

func Connect(ctx context.Context, url string) (*DB, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("connect to postgres: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return &DB{Pool: pool}, nil
}

func (db *DB) Close() {
	db.Pool.Close()
}

// Migrate applies every .up.sql file under dir in filename order inside a
// single transaction per file, tracking what's already applied in a
// schema_migrations table. It's intentionally minimal — no down-migration
// runner, no CLI — because a single-operator app doesn't need one; restoring
// from a backup is the real rollback story for a personal ledger.
func (db *DB) Migrate(ctx context.Context, dir string) error {
	if _, err := db.Pool.Exec(ctx, `create table if not exists schema_migrations (name text primary key, applied_at timestamptz not null default now())`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	files, err := upMigrationFiles(dir)
	if err != nil {
		return err
	}

	for _, f := range files {
		var already bool
		err := db.Pool.QueryRow(ctx, `select exists(select 1 from schema_migrations where name = $1)`, f.name).Scan(&already)
		if err != nil {
			return fmt.Errorf("check migration %s: %w", f.name, err)
		}
		if already {
			continue
		}

		tx, err := db.Pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin migration %s: %w", f.name, err)
		}
		if _, err := tx.Exec(ctx, f.sql); err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("apply migration %s: %w", f.name, err)
		}
		if _, err := tx.Exec(ctx, `insert into schema_migrations (name) values ($1)`, f.name); err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("record migration %s: %w", f.name, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit migration %s: %w", f.name, err)
		}
	}
	return nil
}

type migrationFile struct {
	name string
	sql  string
}

func upMigrationFiles(dir string) ([]migrationFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read migrations dir %s: %w", dir, err)
	}
	var files []migrationFile
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".up.sql") {
			continue
		}
		content, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", e.Name(), err)
		}
		files = append(files, migrationFile{name: e.Name(), sql: string(content)})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].name < files[j].name })
	return files, nil
}
