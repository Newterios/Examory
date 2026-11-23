// Package migrate applies embedded SQL migrations under a PostgreSQL
// advisory lock so that several replicas starting at once never race.
//
// A Set is a named fs.FS whose top-level *.sql files are applied in
// lexicographic order (use 0001_name.sql). Each file runs in its own
// transaction and is recorded in schema_migrations with its SHA-256; a file
// that changed after being applied aborts startup.
package migrate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"path"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Set is a named group of migrations.
type Set struct {
	Name string
	FS   fs.FS
}

// lockKey is the advisory lock id used for migrations ("EXMRMIGR").
const lockKey int64 = 0x45584d524d494752

// Up applies pending migrations of all sets in order.
func Up(ctx context.Context, pool *pgxpool.Pool, logger *slog.Logger, sets ...Set) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("migrate: acquire: %w", err)
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", lockKey); err != nil {
		return fmt.Errorf("migrate: lock: %w", err)
	}
	defer func() {
		_, _ = conn.Exec(context.WithoutCancel(ctx), "SELECT pg_advisory_unlock($1)", lockKey)
	}()
	if _, err := conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		set_name   text NOT NULL,
		version    text NOT NULL,
		checksum   text NOT NULL,
		applied_at timestamptz NOT NULL DEFAULT now(),
		PRIMARY KEY (set_name, version))`); err != nil {
		return fmt.Errorf("migrate: bootstrap: %w", err)
	}
	for _, s := range sets {
		if err := applySet(ctx, conn.Conn(), logger, s); err != nil {
			return err
		}
	}
	return nil
}

func applySet(ctx context.Context, conn *pgx.Conn, logger *slog.Logger, s Set) error {
	entries, err := fs.ReadDir(s.FS, ".")
	if err != nil {
		return fmt.Errorf("migrate %s: read dir: %w", s.Name, err)
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)
	for _, f := range files {
		body, err := fs.ReadFile(s.FS, f)
		if err != nil {
			return fmt.Errorf("migrate %s/%s: %w", s.Name, f, err)
		}
		sum := sha256.Sum256(body)
		checksum := hex.EncodeToString(sum[:])
		version := strings.TrimSuffix(path.Base(f), ".sql")
		var existing string
		err = conn.QueryRow(ctx, "SELECT checksum FROM schema_migrations WHERE set_name=$1 AND version=$2", s.Name, version).Scan(&existing)
		switch {
		case err == nil:
			if existing != checksum {
				return fmt.Errorf("migrate %s/%s: applied migration was modified (checksum mismatch)", s.Name, version)
			}
			continue
		case !errors.Is(err, pgx.ErrNoRows):
			return fmt.Errorf("migrate %s/%s: %w", s.Name, version, err)
		}
		tx, err := conn.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, string(body)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("migrate %s/%s: %w", s.Name, version, err)
		}
		if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (set_name, version, checksum) VALUES ($1,$2,$3)", s.Name, version, checksum); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("migrate %s/%s: record: %w", s.Name, version, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("migrate %s/%s: commit: %w", s.Name, version, err)
		}
		logger.Info("migration applied", slog.String("set", s.Name), slog.String("version", version))
	}
	return nil
}
