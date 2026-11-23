// Package testpg creates throwaway PostgreSQL databases for integration
// tests. It is test-only; production binaries must not import it.
//
// IT_PG_ADMIN_URL must point at a role allowed to CREATE DATABASE (the
// compose superuser), e.g. postgres://examory:...@127.0.0.1:5432/postgres.
package testpg

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// New creates a fresh database, registers its removal with t.Cleanup and
// returns its URL. It skips the test when IT_PG_ADMIN_URL is unset.
func New(t testing.TB) string {
	t.Helper()
	admin := os.Getenv("IT_PG_ADMIN_URL")
	if admin == "" {
		t.Skip("IT_PG_ADMIN_URL not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, admin)
	if err != nil {
		t.Fatalf("testpg: connect: %v", err)
	}
	defer conn.Close(ctx)
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	name := "it_" + hex.EncodeToString(b)
	// Identifier is generated locally from hex digits only.
	if _, err := conn.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()); err != nil {
		t.Fatalf("testpg: create database: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		c, err := pgx.Connect(ctx, admin)
		if err != nil {
			return
		}
		defer c.Close(ctx)
		_, _ = c.Exec(ctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)")
	})
	u, err := url.Parse(admin)
	if err != nil {
		t.Fatalf("testpg: bad admin url")
	}
	u.Path = "/" + name
	return u.String()
}
