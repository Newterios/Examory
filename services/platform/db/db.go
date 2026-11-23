// Package db opens PostgreSQL pools with safe defaults and provides a
// transaction helper and error classification.
package db

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Config configures Open.
type Config struct {
	URL              string
	ApplicationName  string
	MaxConns         int32         // default 10
	ConnectTimeout   time.Duration // default 5s
	StatementTimeout time.Duration // default 5s, server-side per statement
}

// Open creates a pool, applies server-side timeouts to every connection
// (statement_timeout, idle_in_transaction_session_timeout, lock_timeout) and
// pings the database.
func Open(ctx context.Context, cfg Config) (*pgxpool.Pool, error) {
	pc, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		// pgx errors may echo the DSN; never wrap them.
		return nil, errors.New("db: invalid database URL")
	}
	if cfg.MaxConns <= 0 {
		cfg.MaxConns = 10
	}
	if cfg.ConnectTimeout <= 0 {
		cfg.ConnectTimeout = 5 * time.Second
	}
	if cfg.StatementTimeout <= 0 {
		cfg.StatementTimeout = 5 * time.Second
	}
	pc.MaxConns = cfg.MaxConns
	pc.MaxConnLifetime = 30 * time.Minute
	pc.MaxConnIdleTime = 5 * time.Minute
	pc.HealthCheckPeriod = 30 * time.Second
	pc.ConnConfig.ConnectTimeout = cfg.ConnectTimeout
	rp := pc.ConnConfig.RuntimeParams
	rp["application_name"] = cfg.ApplicationName
	rp["statement_timeout"] = strconv.FormatInt(cfg.StatementTimeout.Milliseconds(), 10)
	rp["lock_timeout"] = strconv.FormatInt(cfg.StatementTimeout.Milliseconds(), 10)
	rp["idle_in_transaction_session_timeout"] = "15000"
	rp["timezone"] = "UTC"
	pool, err := pgxpool.NewWithConfig(ctx, pc)
	if err != nil {
		return nil, fmt.Errorf("db: create pool: %w", err)
	}
	pctx, cancel := context.WithTimeout(ctx, cfg.ConnectTimeout)
	defer cancel()
	if err := pool.Ping(pctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: ping: %w", err)
	}
	return pool, nil
}

// TxBeginner is implemented by *pgxpool.Pool and *pgx.Conn.
type TxBeginner interface {
	BeginTx(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error)
}

// InTx runs fn in a READ COMMITTED transaction, committing on nil error and
// rolling back otherwise (also on panic, which is re-raised).
func InTx(ctx context.Context, b TxBeginner, fn func(pgx.Tx) error) (err error) {
	tx, err := b.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return fmt.Errorf("db: begin: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(context.WithoutCancel(ctx))
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback(context.WithoutCancel(ctx))
		}
	}()
	if err = fn(tx); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("db: commit: %w", err)
	}
	return nil
}

// Postgres SQLSTATE codes used by callers.
const (
	CodeUniqueViolation     = "23505"
	CodeForeignKeyViolation = "23503"
	CodeCheckViolation      = "23514"
	CodeRaiseException      = "P0001"
)

// ErrorCode returns the SQLSTATE and constraint name of a Postgres error.
func ErrorCode(err error) (code, constraint string, ok bool) {
	var pe *pgconn.PgError
	if errors.As(err, &pe) {
		return pe.Code, pe.ConstraintName, true
	}
	return "", "", false
}

// IsUniqueViolation reports a unique violation, optionally on a named constraint.
func IsUniqueViolation(err error, constraint string) bool {
	code, c, ok := ErrorCode(err)
	return ok && code == CodeUniqueViolation && (constraint == "" || c == constraint)
}
