// Package consumer runs Kafka consumer groups with idempotent processing
// by event_id (NFR-12).
//
// For every record the handler runs inside a database transaction that first
// inserts (consumer, event_id) into processed_events. A duplicate delivery
// hits the primary key, the handler is skipped and the offset is committed.
// Offsets are committed only after the transaction committed, so a crash
// re-delivers (at-least-once) and the dedup row makes the effect exactly-once.
package consumer

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/Newterios/Examory/services/platform/db"
	"github.com/Newterios/Examory/services/platform/events"
	"github.com/Newterios/Examory/services/platform/kafkax"
	"github.com/Newterios/Examory/services/platform/migrate"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Migrations returns the migration set creating processed_events.
func Migrations() migrate.Set {
	sub, err := fs.Sub(migrations, "migrations")
	if err != nil {
		panic(err)
	}
	return migrate.Set{Name: "platform_consumer", FS: sub}
}

// Handler applies one event within tx. Returning an error rolls back the
// transaction (including the dedup row) and the event is retried.
type Handler func(ctx context.Context, tx pgx.Tx, env events.Envelope) error

// ErrPermanent marks an error that retrying cannot fix; the event is skipped
// (logged) instead of blocking the partition.
var ErrPermanent = errors.New("permanent processing error")

// HandleOnce runs h for env unless consumer already processed env.EventID.
// It reports whether h ran.
func HandleOnce(ctx context.Context, pool *pgxpool.Pool, consumerName string, env events.Envelope, h Handler) (bool, error) {
	ran := false
	err := db.InTx(ctx, pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx,
			`INSERT INTO processed_events (consumer, event_id, event_type) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
			consumerName, env.EventID, env.EventType)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return nil // duplicate
		}
		ran = true
		return h(ctx, tx, env)
	})
	if err != nil {
		ran = false
	}
	return ran, err
}

// Config configures a Consumer.
type Config struct {
	Kafka  kafkax.Config
	Group  string   // consumer group id, also the dedup namespace
	Topics []string // topics to consume
	// MaxAttempts per event before a non-permanent error is treated as
	// permanent and skipped (default 10). Retries back off exponentially.
	MaxAttempts int
}

// Consumer consumes topics and calls Handler once per event_id.
type Consumer struct {
	cfg     Config
	pool    *pgxpool.Pool
	handler Handler
	log     *slog.Logger
	client  *kgo.Client
}

// New creates a consumer.
func New(cfg Config, pool *pgxpool.Pool, h Handler, logger *slog.Logger) (*Consumer, error) {
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 10
	}
	cl, err := kafkax.NewGroupClient(cfg.Kafka, cfg.Group, cfg.Topics...)
	if err != nil {
		return nil, err
	}
	return &Consumer{cfg: cfg, pool: pool, handler: h, log: logger, client: cl}, nil
}

// Run polls until ctx is cancelled, then closes the client (leaving the group).
func (c *Consumer) Run(ctx context.Context) error {
	defer c.client.Close()
	for {
		fetches := c.client.PollFetches(ctx)
		if ctx.Err() != nil {
			return nil
		}
		fetches.EachError(func(t string, p int32, err error) {
			c.log.Warn("kafka fetch error", slog.String("topic", t), slog.Int("partition", int(p)), slog.String("error", err.Error()))
		})
		var done []*kgo.Record
		stop := false
		fetches.EachPartition(func(ftp kgo.FetchTopicPartition) {
			for _, rec := range ftp.Records {
				if stop {
					return
				}
				if err := c.process(ctx, rec); err != nil {
					stop = true // shutting down; uncommitted records are redelivered
					return
				}
				done = append(done, rec)
			}
		})
		if len(done) > 0 {
			cctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
			if err := c.client.CommitRecords(cctx, done...); err != nil {
				c.log.Warn("kafka commit failed; records will be redelivered and deduplicated", slog.String("error", err.Error()))
			}
			cancel()
		}
		c.client.AllowRebalance()
		if stop {
			return nil
		}
	}
}

// process handles one record with retries. It returns an error only when
// ctx is cancelled.
func (c *Consumer) process(ctx context.Context, rec *kgo.Record) error {
	env, err := events.Parse(rec.Value)
	if err != nil {
		c.log.Error("skipping invalid event", slog.String("topic", rec.Topic), slog.Int64("offset", rec.Offset), slog.String("error", err.Error()))
		return nil
	}
	backoff := 100 * time.Millisecond
	for attempt := 1; ; attempt++ {
		ran, err := HandleOnce(ctx, c.pool, c.cfg.Group, env, c.handler)
		if err == nil {
			if !ran {
				c.log.Debug("duplicate event skipped", slog.String("event_id", env.EventID))
			}
			return nil
		}
		if errors.Is(err, ErrPermanent) || attempt >= c.cfg.MaxAttempts {
			c.log.Error("event processing failed permanently; skipped",
				slog.String("event_id", env.EventID), slog.String("event_type", env.EventType), slog.String("error", err.Error()))
			return nil
		}
		c.log.Warn("event processing failed; retrying", slog.String("event_id", env.EventID), slog.Int("attempt", attempt), slog.String("error", err.Error()))
		t := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			t.Stop()
			return fmt.Errorf("consumer stopping: %w", ctx.Err())
		case <-t.C:
		}
		backoff = min(backoff*2, 30*time.Second)
	}
}
