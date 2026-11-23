package outbox

import (
	"context"
	"errors"
	"log/slog"
	"math/rand/v2"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Newterios/Examory/services/platform/kafkax"
)

// RelayConfig tunes the relay. Zero values use the defaults in brackets.
type RelayConfig struct {
	BatchSize      int           // rows per tick [100]
	PollInterval   time.Duration // idle sleep [200ms]
	MinBackoff     time.Duration // first retry delay after a failure [200ms]
	MaxBackoff     time.Duration // cap [30s]
	PublishTimeout time.Duration // per wave [10s]
	Retention      time.Duration // published rows are deleted after [72h]
	// LockKey is the advisory lock id electing the single active relay of a
	// database. Each service has its own database, so the default is fine.
	LockKey int64
}

func (c *RelayConfig) defaults() {
	if c.BatchSize <= 0 {
		c.BatchSize = 100
	}
	if c.PollInterval <= 0 {
		c.PollInterval = 200 * time.Millisecond
	}
	if c.MinBackoff <= 0 {
		c.MinBackoff = 200 * time.Millisecond
	}
	if c.MaxBackoff <= 0 {
		c.MaxBackoff = 30 * time.Second
	}
	if c.PublishTimeout <= 0 {
		c.PublishTimeout = 10 * time.Second
	}
	if c.Retention <= 0 {
		c.Retention = 72 * time.Hour
	}
	if c.LockKey == 0 {
		c.LockKey = 0x45584d524f555458 // "EXMROUTX"
	}
}

// Row is a pending outbox row.
type Row struct {
	ID    int64
	Topic string
	Key   string
	Type  string
	Body  []byte
}

// store abstracts the database side of a relay tick (tests use a fake).
type store interface {
	Pending(ctx context.Context, limit int) ([]Row, error)
	MarkPublished(ctx context.Context, ids []int64) error
	MarkFailed(ctx context.Context, id int64, reason string) error
	Cleanup(ctx context.Context, olderThan time.Duration) error
}

// Relay publishes outbox rows. Create with NewRelay and run with Run.
type Relay struct {
	pool   *pgxpool.Pool
	pub    kafkax.Publisher
	log    *slog.Logger
	cfg    RelayConfig
	sleep  func(ctx context.Context, d time.Duration) bool
	isLead func() // test hook, called when leadership is acquired
}

// NewRelay returns a relay publishing rows of pool's outbox through pub.
func NewRelay(pool *pgxpool.Pool, pub kafkax.Publisher, logger *slog.Logger, cfg RelayConfig) *Relay {
	cfg.defaults()
	return &Relay{pool: pool, pub: pub, log: logger, cfg: cfg, sleep: sleepCtx}
}

func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// Run blocks until ctx is cancelled. It returns nil on graceful stop.
func (r *Relay) Run(ctx context.Context) error {
	for ctx.Err() == nil {
		if err := r.lead(ctx); err != nil && ctx.Err() == nil {
			r.log.Warn("outbox relay lost leadership or database", slog.String("error", err.Error()))
		}
		if !r.sleep(ctx, r.cfg.PollInterval*5) {
			break
		}
	}
	r.log.Info("outbox relay stopped")
	return nil
}

// lead acquires the advisory lock on a dedicated connection and publishes
// until ctx ends or the connection fails. All queries of the active relay
// run on that same connection, so losing it also stops publishing.
func (r *Relay) lead(ctx context.Context) error {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	var ok bool
	if err := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", r.cfg.LockKey).Scan(&ok); err != nil {
		return err
	}
	if !ok {
		return nil // another instance is active
	}
	defer func() {
		uctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer cancel()
		_, _ = conn.Exec(uctx, "SELECT pg_advisory_unlock($1)", r.cfg.LockKey)
	}()
	r.log.Info("outbox relay active")
	if r.isLead != nil {
		r.isLead()
	}
	return r.loop(ctx, &pgStore{conn: conn})
}

func (r *Relay) loop(ctx context.Context, st store) error {
	backoff := time.Duration(0)
	ticks := 0
	for ctx.Err() == nil {
		n, err := r.tick(ctx, st)
		ticks++
		if ticks%500 == 1 {
			if cerr := st.Cleanup(ctx, r.cfg.Retention); cerr != nil && ctx.Err() == nil {
				r.log.Warn("outbox cleanup failed", slog.String("error", cerr.Error()))
			}
		}
		switch {
		case err != nil:
			var dbErr *dbError
			if errors.As(err, &dbErr) {
				return err // re-elect on a fresh connection
			}
			backoff = nextBackoff(backoff, r.cfg.MinBackoff, r.cfg.MaxBackoff)
			r.log.Warn("outbox publish failed, backing off", slog.String("error", err.Error()), slog.Duration("backoff", backoff))
			if !r.sleep(ctx, backoff) {
				return nil
			}
		case n == 0:
			backoff = 0
			if !r.sleep(ctx, r.cfg.PollInterval) {
				return nil
			}
		default:
			backoff = 0
		}
	}
	return nil
}

func nextBackoff(cur, minB, maxB time.Duration) time.Duration {
	if cur == 0 {
		cur = minB
	} else {
		cur *= 2
	}
	if cur > maxB {
		cur = maxB
	}
	// full jitter in [cur/2, cur]
	return cur/2 + time.Duration(rand.Int64N(int64(cur/2)+1)) // #nosec G404 -- backoff jitter, not security-sensitive
}

type dbError struct{ err error }

func (e *dbError) Error() string { return "outbox db: " + e.err.Error() }
func (e *dbError) Unwrap() error { return e.err }

// tick publishes one batch. It returns how many rows were published and the
// first publish error, if any.
func (r *Relay) tick(ctx context.Context, st store) (int, error) {
	rows, err := st.Pending(ctx, r.cfg.BatchSize)
	if err != nil {
		return 0, &dbError{err}
	}
	if len(rows) == 0 {
		return 0, nil
	}
	waves := planWaves(rows)
	failed := map[string]bool{}
	var published []int64
	var firstErr error
	for _, wave := range waves {
		var msgs []kafkax.Message
		var idx []int
		for _, i := range wave {
			if failed[rows[i].Key] {
				continue
			}
			msgs = append(msgs, kafkax.Message{Topic: rows[i].Topic, Key: rows[i].Key, Value: rows[i].Body,
				Headers: map[string]string{"event_type": rows[i].Type}})
			idx = append(idx, i)
		}
		if len(msgs) == 0 {
			break
		}
		pctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), r.cfg.PublishTimeout)
		errs := r.pub.Publish(pctx, msgs)
		cancel()
		for j, e := range errs {
			row := rows[idx[j]]
			if e != nil {
				failed[row.Key] = true
				if firstErr == nil {
					firstErr = e
				}
				reason := e.Error()
				if len(reason) > 500 {
					reason = reason[:500]
				}
				if merr := st.MarkFailed(context.WithoutCancel(ctx), row.ID, reason); merr != nil {
					return len(published), &dbError{merr}
				}
				continue
			}
			published = append(published, row.ID)
		}
		if ctx.Err() != nil {
			break // stop after the current wave on shutdown
		}
	}
	if len(published) > 0 {
		if err := st.MarkPublished(context.WithoutCancel(ctx), published); err != nil {
			return 0, &dbError{err}
		}
	}
	return len(published), firstErr
}

// planWaves groups row indexes so that wave w contains the w-th row of
// every key, preserving id order inside a key.
func planWaves(rows []Row) [][]int {
	perKey := map[string][]int{}
	var order []string
	for i, r := range rows {
		if _, ok := perKey[r.Key]; !ok {
			order = append(order, r.Key)
		}
		perKey[r.Key] = append(perKey[r.Key], i)
	}
	var waves [][]int
	for w := 0; ; w++ {
		var wave []int
		for _, k := range order {
			if l := perKey[k]; w < len(l) {
				wave = append(wave, l[w])
			}
		}
		if len(wave) == 0 {
			return waves
		}
		waves = append(waves, wave)
	}
}

type pgStore struct{ conn *pgxpool.Conn }

func (s *pgStore) Pending(ctx context.Context, limit int) ([]Row, error) {
	rs, err := s.conn.Query(ctx,
		`SELECT id, topic, partition_key, event_type, envelope FROM outbox WHERE published_at IS NULL ORDER BY id LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out []Row
	for rs.Next() {
		var r Row
		if err := rs.Scan(&r.ID, &r.Topic, &r.Key, &r.Type, &r.Body); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rs.Err()
}

func (s *pgStore) MarkPublished(ctx context.Context, ids []int64) error {
	_, err := s.conn.Exec(ctx, `UPDATE outbox SET published_at = now(), last_error = NULL WHERE id = ANY($1)`, ids)
	return err
}

func (s *pgStore) MarkFailed(ctx context.Context, id int64, reason string) error {
	_, err := s.conn.Exec(ctx, `UPDATE outbox SET attempts = attempts + 1, last_error = $2 WHERE id = $1`, id, reason)
	return err
}

func (s *pgStore) Cleanup(ctx context.Context, olderThan time.Duration) error {
	_, err := s.conn.Exec(ctx,
		`DELETE FROM outbox WHERE id IN (SELECT id FROM outbox WHERE published_at < now() - make_interval(secs => $1) LIMIT 1000)`,
		olderThan.Seconds())
	return err
}
