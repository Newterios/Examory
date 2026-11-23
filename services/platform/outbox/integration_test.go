//go:build integration

package outbox_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/Newterios/Examory/services/platform/consumer"
	"github.com/Newterios/Examory/services/platform/db"
	"github.com/Newterios/Examory/services/platform/events"
	"github.com/Newterios/Examory/services/platform/ids"
	"github.com/Newterios/Examory/services/platform/kafkax"
	"github.com/Newterios/Examory/services/platform/logging"
	"github.com/Newterios/Examory/services/platform/migrate"
	"github.com/Newterios/Examory/services/platform/outbox"
	"github.com/Newterios/Examory/services/platform/testpg"
)

func brokers(t *testing.T) []string {
	b := os.Getenv("IT_KAFKA_BROKERS")
	if b == "" {
		t.Skip("IT_KAFKA_BROKERS not set")
	}
	return strings.Split(b, ",")
}

func setup(t *testing.T) (context.Context, *pgxpool.Pool) {
	url := testpg.New(t)
	ctx := context.Background()
	pool, err := db.Open(ctx, db.Config{URL: url, ApplicationName: "it"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	sets := []migrate.Set{outbox.Migrations(), consumer.Migrations()}
	// Concurrent migrators: the advisory lock serialises them; both succeed.
	var wg sync.WaitGroup
	errs := make([]error, 3)
	for i := range errs {
		wg.Add(1)
		go func(i int) { defer wg.Done(); errs[i] = migrate.Up(ctx, pool, logging.Discard(), sets...) }(i)
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatalf("migrate: %v", err)
		}
	}
	return ctx, pool
}

func TestMigrateDetectsModifiedMigration(t *testing.T) {
	url := testpg.New(t)
	ctx := context.Background()
	pool, err := db.Open(ctx, db.Config{URL: url})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	set := migrate.Set{Name: "x", FS: fstest.MapFS{"0001_a.sql": {Data: []byte("CREATE TABLE a (id int);")}}}
	if err := migrate.Up(ctx, pool, logging.Discard(), set); err != nil {
		t.Fatal(err)
	}
	if err := migrate.Up(ctx, pool, logging.Discard(), set); err != nil {
		t.Fatalf("re-apply must be a no-op: %v", err)
	}
	set.FS = fstest.MapFS{"0001_a.sql": {Data: []byte("CREATE TABLE a (id bigint);")}}
	if err := migrate.Up(ctx, pool, logging.Discard(), set); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("modified migration not detected: %v", err)
	}
}

// enqueue writes n events for each of keys in interleaved order.
func enqueue(t *testing.T, ctx context.Context, pool *pgxpool.Pool, topic string, keys []string, n int) map[string][]string {
	want := map[string][]string{}
	for i := 0; i < n; i++ {
		for _, k := range keys {
			env, err := events.New("test.happened", 1, "it", events.Actor{UserID: k, Role: "student"}, events.Context{},
				map[string]any{"seq": i, "key": k}, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			if err := db.InTx(ctx, pool, func(tx pgx.Tx) error { return outbox.Enqueue(ctx, tx, topic, k, env) }); err != nil {
				t.Fatal(err)
			}
			want[k] = append(want[k], env.EventID)
		}
	}
	return want
}

type got struct {
	byKey map[string][]string
	all   int
}

func consumeAll(t *testing.T, br []string, topic string, total int, timeout time.Duration) got {
	cl, err := kgo.NewClient(kgo.SeedBrokers(br...), kgo.ConsumeTopics(topic), kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()))
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()
	g := got{byKey: map[string][]string{}}
	deadline := time.Now().Add(timeout)
	for g.all < total && time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		fs := cl.PollFetches(ctx)
		cancel()
		fs.EachRecord(func(r *kgo.Record) {
			env, err := events.Parse(r.Value)
			if err != nil {
				t.Errorf("invalid event on topic: %v", err)
				return
			}
			if string(r.Key) != env.Actor.UserID {
				t.Errorf("partition key %s != actor %s", r.Key, env.Actor.UserID)
			}
			g.byKey[string(r.Key)] = append(g.byKey[string(r.Key)], env.EventID)
			g.all++
		})
	}
	return g
}

func pending(t *testing.T, ctx context.Context, pool *pgxpool.Pool) int {
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM outbox WHERE published_at IS NULL`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func runRelay(t *testing.T, pool *pgxpool.Pool, pub kafkax.Publisher) (stop func()) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	r := outbox.NewRelay(pool, pub, logging.Discard(), outbox.RelayConfig{PollInterval: 50 * time.Millisecond, MaxBackoff: time.Second, PublishTimeout: 3 * time.Second})
	go func() { _ = r.Run(ctx); close(done) }()
	return func() {
		cancel()
		select {
		case <-done:
		case <-time.After(15 * time.Second):
			t.Fatal("relay did not stop")
		}
	}
}

func waitPending(t *testing.T, ctx context.Context, pool *pgxpool.Pool, want int, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for pending(t, ctx, pool) != want {
		if time.Now().After(deadline) {
			t.Fatalf("pending=%d, want %d", pending(t, ctx, pool), want)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// TestRelayOutageRecoveryAndDuplicates: (1) broker unreachable: nothing is
// marked published, nothing lost; (2) relay restarted against the real
// broker delivers everything in per-key order; (3) a crash between the ack
// and the mark re-publishes duplicates that carry the same event_id.
func TestRelayOutageRecoveryAndDuplicates(t *testing.T) {
	br := brokers(t)
	ctx, pool := setup(t)
	topic := "it.outbox." + strings.ReplaceAll(ids.NewV7()[:8], "-", "")
	keys := []string{ids.NewV7(), ids.NewV7(), ids.NewV7()}
	want := enqueue(t, ctx, pool, topic, keys, 10)

	// (1) Outage: a producer pointed at a closed port.
	down, err := kafkax.NewProducer(kafkax.Config{Brokers: []string{"127.0.0.1:1"}, ClientID: "it-down", DeliveryTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	stop := runRelay(t, pool, down)
	time.Sleep(3 * time.Second)
	stop()
	down.Close()
	if p := pending(t, ctx, pool); p != 30 {
		t.Fatalf("during outage pending=%d, want 30 (nothing marked, nothing lost)", p)
	}
	var attempts int
	_ = pool.QueryRow(ctx, `SELECT coalesce(max(attempts),0) FROM outbox`).Scan(&attempts)
	if attempts == 0 {
		t.Fatal("failed publishes were not recorded")
	}

	// (2) Restart against the real broker.
	up, err := kafkax.NewProducer(kafkax.Config{Brokers: br, ClientID: "it-up", AllowAutoTopicCreation: true})
	if err != nil {
		t.Fatal(err)
	}
	defer up.Close()
	stop = runRelay(t, pool, up)
	waitPending(t, ctx, pool, 0, 30*time.Second)
	stop()
	g := consumeAll(t, br, topic, 30, 30*time.Second)
	for _, k := range keys {
		if fmt.Sprint(g.byKey[k]) != fmt.Sprint(want[k]) {
			t.Fatalf("key %s: order/loss mismatch\n got %v\nwant %v", k, g.byKey[k], want[k])
		}
	}

	// (3) Simulated crash after publish, before mark: rows become pending
	// again and are re-published with identical event ids.
	if _, err := pool.Exec(ctx, `UPDATE outbox SET published_at = NULL WHERE id IN (SELECT id FROM outbox ORDER BY id LIMIT 5)`); err != nil {
		t.Fatal(err)
	}
	stop = runRelay(t, pool, up)
	waitPending(t, ctx, pool, 0, 30*time.Second)
	stop()
	g = consumeAll(t, br, topic, 35, 30*time.Second)
	if g.all != 35 {
		t.Fatalf("expected 35 records (30 + 5 duplicates), got %d", g.all)
	}
	seen := map[string]int{}
	for _, list := range g.byKey {
		for _, id := range list {
			seen[id]++
		}
	}
	dups := 0
	for _, n := range seen {
		if n == 2 {
			dups++
		}
	}
	if len(seen) != 30 || dups != 5 {
		t.Fatalf("distinct=%d dups=%d: duplicates must reuse event_id", len(seen), dups)
	}
}

// TestRelayDockerPauseOutage pauses the real Kafka container (opt-in:
// IT_DOCKER_KAFKA=<container name>) while events are being produced.
func TestRelayDockerPauseOutage(t *testing.T) {
	container := os.Getenv("IT_DOCKER_KAFKA")
	if container == "" {
		t.Skip("IT_DOCKER_KAFKA not set")
	}
	br := brokers(t)
	ctx, pool := setup(t)
	topic := "it.pause." + strings.ReplaceAll(ids.NewV7()[:8], "-", "")
	key := ids.NewV7()
	up, err := kafkax.NewProducer(kafkax.Config{Brokers: br, ClientID: "it-pause", AllowAutoTopicCreation: true, DeliveryTimeout: 2 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer up.Close()
	want := enqueue(t, ctx, pool, topic, []string{key}, 5)
	stop := runRelay(t, pool, up)
	waitPending(t, ctx, pool, 0, 30*time.Second)
	if out, err := exec.Command("docker", "pause", container).CombinedOutput(); err != nil {
		t.Fatalf("docker pause: %v %s", err, out)
	}
	unpaused := false
	defer func() {
		if !unpaused {
			_ = exec.Command("docker", "unpause", container).Run()
		}
	}()
	more := enqueue(t, ctx, pool, topic, []string{key}, 5)
	time.Sleep(6 * time.Second)
	if p := pending(t, ctx, pool); p != 5 {
		t.Fatalf("during pause pending=%d, want 5", p)
	}
	if out, err := exec.Command("docker", "unpause", container).CombinedOutput(); err != nil {
		t.Fatalf("docker unpause: %v %s", err, out)
	}
	unpaused = true
	waitPending(t, ctx, pool, 0, 60*time.Second)
	stop()
	g := consumeAll(t, br, topic, 10, 30*time.Second)
	// Order must hold; duplicates (same event_id) are allowed after an outage.
	var dedup []string
	seen := map[string]bool{}
	for _, id := range g.byKey[key] {
		if !seen[id] {
			seen[id] = true
			dedup = append(dedup, id)
		}
	}
	if fmt.Sprint(dedup) != fmt.Sprint(append(want[key], more[key]...)) {
		t.Fatalf("order after pause:\n got %v\nwant %v", dedup, append(want[key], more[key]...))
	}
	t.Logf("delivered %d records (%d distinct) across a broker pause", g.all, len(dedup))
}

func TestHandleOnceDeduplicates(t *testing.T) {
	ctx, pool := setup(t)
	if _, err := pool.Exec(ctx, `CREATE TABLE effects (event_id uuid PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	env, _ := events.New("test.happened", 1, "it", events.Actor{UserID: "u", Role: "student"}, events.Context{}, map[string]int{"x": 1}, time.Now())
	h := func(ctx context.Context, tx pgx.Tx, e events.Envelope) error {
		_, err := tx.Exec(ctx, `INSERT INTO effects VALUES ($1)`, e.EventID)
		return err
	}
	var wg sync.WaitGroup
	ran := make([]bool, 8)
	for i := range ran {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r, err := consumer.HandleOnce(ctx, pool, "grading", env, h)
			if err != nil && !db.IsUniqueViolation(err, "") {
				t.Errorf("handle: %v", err)
			}
			ran[i] = r
		}(i)
	}
	wg.Wait()
	n := 0
	for _, r := range ran {
		if r {
			n++
		}
	}
	var effects int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM effects`).Scan(&effects)
	if n != 1 || effects != 1 {
		t.Fatalf("handler ran %d times, %d effects; want exactly 1", n, effects)
	}
	// Another consumer group processes the same event independently.
	if r, err := consumer.HandleOnce(ctx, pool, "analytics", env, func(context.Context, pgx.Tx, events.Envelope) error { return nil }); err != nil || !r {
		t.Fatalf("second group: %v %v", r, err)
	}
	// A failing handler rolls back the dedup row, so the event is retried.
	env2, _ := events.New("test.happened", 1, "it", events.Actor{UserID: "u", Role: "student"}, events.Context{}, map[string]int{}, time.Now())
	if _, err := consumer.HandleOnce(ctx, pool, "grading", env2, func(context.Context, pgx.Tx, events.Envelope) error { return fmt.Errorf("boom") }); err == nil {
		t.Fatal("expected error")
	}
	if r, err := consumer.HandleOnce(ctx, pool, "grading", env2, h); err != nil || !r {
		t.Fatalf("retry after failure: %v %v", r, err)
	}
}
