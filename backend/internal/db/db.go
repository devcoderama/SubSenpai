// Package db membungkus akses PostgreSQL (pgx) dan migrasi skema.
//
// Migrasi awal (0001_init.sql) adalah salinan persis database/database.sql
// di repo (skema final Fase 0, 52 tabel) dan tertanam (embed) di binary agar
// server tetap satu binary mandiri. Perubahan skema berikutnya hanya lewat
// berkas migrasi berversi baru di folder ini (O23), tidak mengubah 0001.
package db

import (
	"context"
	"embed"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func Connect(ctx context.Context, url string) (*pgxpool.Pool, error) {
	if url == "" {
		return nil, fmt.Errorf("DATABASE_URL kosong")
	}
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

// Migrate menerapkan migrasi yang belum pernah jalan, berurutan nama berkas,
// dan mencatatnya di tabel schema_migrations.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, `create table if not exists schema_migrations (
		version text primary key,
		applied_at timestamptz not null default now()
	)`); err != nil {
		return fmt.Errorf("buat tabel schema_migrations: %w", err)
	}
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		var done bool
		if err := pool.QueryRow(ctx,
			"select exists(select 1 from schema_migrations where version = $1)", name,
		).Scan(&done); err != nil {
			return err
		}
		if done {
			continue
		}
		sqlBytes, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return err
		}
		tx, err := pool.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, string(sqlBytes)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("migrasi %s gagal: %w", name, err)
		}
		if _, err := tx.Exec(ctx,
			"insert into schema_migrations (version) values ($1)", name); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
	}
	return nil
}
