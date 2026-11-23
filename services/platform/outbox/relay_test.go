package outbox

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/Newterios/Examory/services/platform/kafkax"
	"github.com/Newterios/Examory/services/platform/logging"
)

type fakeStore struct {
	mu        sync.Mutex
	rows      []Row
	published map[int64]bool
	failures  map[int64]int
}

func (s *fakeStore) Pending(_ context.Context, limit int) ([]Row, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Row
	for _, r := range s.rows {
		if !s.published[r.ID] && len(out) < limit {
			out = append(out, r)
		}
	}
	return out, nil
}
func (s *fakeStore) MarkPublished(_ context.Context, ids []int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, id := range ids {
		s.published[id] = true
	}
	return nil
}
func (s *fakeStore) MarkFailed(_ context.Context, id int64, _ string) error {
	s.mu.Lock()
	s.failures[id]++
	s.mu.Unlock()
	return nil
}
func (s *fakeStore) Cleanup(context.Context, time.Duration) error { return nil }

// fakePub records delivered messages and fails selected ones.
type fakePub struct {
	mu        sync.Mutex
	delivered []kafkax.Message
	failOnce  map[string]bool // body -> fail the first time
	down      bool
}

func (p *fakePub) Publish(_ context.Context, msgs []kafkax.Message) []error {
	p.mu.Lock()
	defer p.mu.Unlock()
	errs := make([]error, len(msgs))
	for i, m := range msgs {
		if p.down {
			errs[i] = errors.New("broker unavailable")
			continue
		}
		if p.failOnce[string(m.Value)] {
			delete(p.failOnce, string(m.Value))
			errs[i] = errors.New("transient")
			continue
		}
		p.delivered = append(p.delivered, m)
	}
	return errs
}

func newFixture(keys map[string]int) *fakeStore {
	st := &fakeStore{published: map[int64]bool{}, failures: map[int64]int{}}
	id := int64(0)
	names := make([]string, 0, len(keys))
	for k := range keys {
		names = append(names, k)
	}
	sort.Strings(names)
	// interleave keys like concurrent students would
	for round := 0; ; round++ {
		added := false
		for _, k := range names {
			if round < keys[k] {
				id++
				st.rows = append(st.rows, Row{ID: id, Topic: "t", Key: k, Type: "x.y", Body: []byte(fmt.Sprintf("%s-%d", k, round))})
				added = true
			}
		}
		if !added {
			return st
		}
	}
}

func testRelay(pub kafkax.Publisher) *Relay {
	r := NewRelay(nil, pub, logging.Discard(), RelayConfig{BatchSize: 7, MinBackoff: time.Millisecond, MaxBackoff: 2 * time.Millisecond})
	r.sleep = func(ctx context.Context, d time.Duration) bool { return ctx.Err() == nil }
	return r
}

func TestPlanWaves(t *testing.T) {
	rows := []Row{{ID: 1, Key: "a"}, {ID: 2, Key: "b"}, {ID: 3, Key: "a"}, {ID: 4, Key: "a"}, {ID: 5, Key: "c"}}
	got := fmt.Sprint(planWaves(rows))
	if got != "[[0 1 4] [2] [3]]" {
		t.Fatalf("waves %s", got)
	}
}

func TestRelayOrderingWithFailures(t *testing.T) {
	st := newFixture(map[string]int{"alice": 6, "bob": 4, "carol": 5})
	pub := &fakePub{failOnce: map[string]bool{"alice-1": true, "bob-0": true, "carol-3": true}}
	r := testRelay(pub)
	ctx := context.Background()
	for i := 0; i < 50; i++ {
		if _, err := r.tick(ctx, st); err != nil {
			continue
		}
	}
	if len(st.published) != 15 {
		t.Fatalf("published %d of 15", len(st.published))
	}
	// Per key, delivery order must equal enqueue order, with no gaps.
	next := map[string]int{}
	for _, m := range pub.delivered {
		want := fmt.Sprintf("%s-%d", m.Key, next[m.Key])
		if string(m.Value) != want {
			t.Fatalf("out of order for %s: got %s want %s", m.Key, m.Value, want)
		}
		next[m.Key]++
	}
	if len(pub.delivered) != 15 {
		t.Fatalf("delivered %d (duplicates or loss)", len(pub.delivered))
	}
	if st.failures[2] != 1 { // bob-0 has id 2
		t.Fatalf("failure not recorded: %v", st.failures)
	}
}

func TestRelayOutageThenRecovery(t *testing.T) {
	st := newFixture(map[string]int{"k": 3})
	pub := &fakePub{down: true, failOnce: map[string]bool{}}
	r := testRelay(pub)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = r.loop(ctx, st); close(done) }()
	time.Sleep(20 * time.Millisecond)
	st.mu.Lock()
	if len(st.published) != 0 {
		t.Fatal("published during outage")
	}
	st.mu.Unlock()
	pub.mu.Lock()
	pub.down = false
	pub.mu.Unlock()
	deadline := time.Now().Add(2 * time.Second)
	for {
		st.mu.Lock()
		n := len(st.published)
		st.mu.Unlock()
		if n == 3 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("not delivered after recovery")
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	<-done
}

func TestBackoff(t *testing.T) {
	b := time.Duration(0)
	for i := 0; i < 20; i++ {
		b = nextBackoff(b, 100*time.Millisecond, time.Second)
		if b <= 0 || b > time.Second {
			t.Fatalf("backoff %v out of range", b)
		}
	}
}
