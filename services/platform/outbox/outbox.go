// Package outbox implements the transactional outbox pattern.
//
// Producers call Enqueue inside the same database transaction as the state
// change, so an event exists if and only if the change committed. A Relay
// publishes pending rows to Kafka:
//
//   - at-least-once: a row is marked published only after the broker acked
//     it; a crash between ack and mark re-publishes the same bytes (same
//     event_id), which consumers deduplicate (NFR-12);
//   - ordered per partition key: only one relay instance is active (Postgres
//     advisory lock), rows are read in id order, and the n-th pending row of a
//     key is sent only after the (n-1)-th was acknowledged ("waves"). After a
//     failure no later row of that key is sent until the failed row succeeds;
//   - exponential backoff with jitter on broker errors, graceful stop on
//     context cancellation.
//
// Ordering between rows depends on id order matching commit order for the
// same key. Producers guarantee that by locking the aggregate row (e.g. the
// quiz attempt) before enqueueing, so transactions touching one key commit
// serially.
package outbox

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Newterios/Examory/services/platform/events"
	"github.com/Newterios/Examory/services/platform/migrate"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Migrations returns the migration set creating the outbox table. Services
// apply it with migrate.Up next to their own sets.
func Migrations() migrate.Set {
	sub, err := fs.Sub(migrations, "migrations")
	if err != nil {
		panic(err) // embedded path is static
	}
	return migrate.Set{Name: "platform_outbox", FS: sub}
}

// Execer is satisfied by pgx.Tx (and *pgxpool.Pool, but always pass the tx).
type Execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Enqueue validates env and inserts it into the outbox within tx.
func Enqueue(ctx context.Context, tx Execer, topic, partitionKey string, env events.Envelope) error {
	if err := env.Validate(); err != nil {
		return fmt.Errorf("outbox: invalid envelope: %w", err)
	}
	if partitionKey == "" {
		return fmt.Errorf("outbox: empty partition key")
	}
	body, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("outbox: marshal: %w", err)
	}
	_, err = tx.Exec(ctx,
		`INSERT INTO outbox (event_id, topic, partition_key, event_type, envelope) VALUES ($1, $2, $3, $4, $5)`,
		env.EventID, topic, partitionKey, env.EventType, body)
	if err != nil {
		return fmt.Errorf("outbox: insert: %w", err)
	}
	return nil
}
