//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Newterios/Examory/services/courses/internal/adapters/postgres"
	"github.com/Newterios/Examory/services/courses/internal/domain"
	"github.com/Newterios/Examory/services/platform/authn"
	"github.com/Newterios/Examory/services/platform/db"
	"github.com/Newterios/Examory/services/platform/events"
	"github.com/Newterios/Examory/services/platform/eventschema"
	"github.com/Newterios/Examory/services/platform/ids"
	"github.com/Newterios/Examory/services/platform/kafkax"
	"github.com/Newterios/Examory/services/platform/logging"
	"github.com/Newterios/Examory/services/platform/migrate"
	"github.com/Newterios/Examory/services/platform/outbox"
	"github.com/Newterios/Examory/services/platform/testpg"
	"github.com/twmb/franz-go/pkg/kgo"
	"os"
	"strings"
	"testing"
	"time"
)

func TestCourseLifecycleAndKafka(t *testing.T) {
	brokers := os.Getenv("IT_KAFKA_BROKERS")
	if brokers == "" {
		t.Skip("IT_KAFKA_BROKERS required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	pool, err := db.Open(ctx, db.Config{URL: testpg.New(t)})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err = migrate.Up(ctx, pool, logging.Discard(), postgres.Migrations()...); err != nil {
		t.Fatal(err)
	}
	s := postgres.New(pool)
	teacher := authn.Principal{UserID: ids.NewV7(), Role: authn.RoleTeacher}
	admin := authn.Principal{UserID: ids.NewV7(), Role: authn.RoleAdmin}
	student := authn.Principal{UserID: ids.NewV7(), Role: authn.RoleStudent, GroupID: ids.NewV7()}
	if _, err = s.Create(ctx, student, "Forbidden"); !errors.Is(err, domain.ErrForbidden) {
		t.Fatal(err)
	}
	c, err := s.Create(ctx, teacher, "Engineering")
	if err != nil {
		t.Fatal(err)
	}
	other := teacher
	other.UserID = ids.NewV7()
	if _, err = s.Update(ctx, other, c.ID, c); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal(err)
	}
	if err = s.Assign(ctx, admin, c.ID, "group", student.GroupID, true); err != nil {
		t.Fatal(err)
	}
	if err = s.Assign(ctx, admin, c.ID, "group", student.GroupID, true); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Get(ctx, student, c.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("draft visible", err)
	}
	if _, err = s.Update(ctx, teacher, c.ID, c); !errors.Is(err, domain.ErrConflict) {
		t.Fatal(err)
	}
	c, err = s.Get(ctx, teacher, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	c.Status = "published"
	c.Modules = []domain.Module{{Title: "Week one", Status: "published", Materials: []domain.Material{{Title: "Notes", Status: "published", Kind: "text", Content: "Read this"}, {Title: "Draft", Status: "draft", Kind: "text", Content: "Secret"}}}}
	c, err = s.Update(ctx, teacher, c.ID, c)
	if err != nil {
		t.Fatal(err)
	}
	items, err := s.List(ctx, student, "", 10)
	if err != nil || len(items) != 1 || len(items[0].Modules[0].Materials) != 1 {
		t.Fatal(items, err)
	}
	if _, err = s.View(ctx, student, c.ID, c.Modules[0].Materials[1].ID, 1, true); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal(err)
	}
	for _, requested := range []bool{true, false} {
		if done, e := s.View(ctx, student, c.ID, c.Modules[0].Materials[0].ID, 5, requested); e != nil || !done {
			t.Fatal(done, e)
		}
	}
	if err = s.Assign(ctx, admin, c.ID, "group", student.GroupID, false); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Get(ctx, student, c.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal(err)
	}
	var n int
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM material_completions").Scan(&n); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	pub, err := kafkax.NewProducer(kafkax.Config{Brokers: strings.Split(brokers, ",")})
	if err != nil {
		t.Fatal(err)
	}
	defer pub.Close()
	consumer, err := kgo.NewClient(kgo.SeedBrokers(strings.Split(brokers, ",")...), kgo.ConsumeTopics(events.TopicCourses), kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()))
	if err != nil {
		t.Fatal(err)
	}
	defer consumer.Close()
	rctx, stop := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = outbox.NewRelay(pool, pub, logging.Discard(), outbox.RelayConfig{}).Run(rctx)
	}()
	defer func() { stop(); <-done }()
	validator, err := eventschema.Load("../../../docs/03_architecture/schemas")
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	seen := map[string]bool{}
	for len(seen) < 6 {
		batch := consumer.PollFetches(ctx)
		if errs := batch.Errors(); len(errs) > 0 {
			t.Fatal(errs)
		}
		for it := batch.RecordIter(); !it.Done(); {
			rec := it.Next()
			if string(rec.Key) != c.ID {
				continue
			}
			if err = validator.Validate(rec.Value); err != nil {
				t.Fatal(err)
			}
			env, e := events.Parse(rec.Value)
			if e != nil {
				t.Fatal(e)
			}
			if seen[env.EventID] {
				continue
			}
			seen[env.EventID] = true
			counts[env.EventType]++
			if env.EventType == "material.viewed" {
				var p struct{ Completed bool }
				if json.Unmarshal(env.Payload, &p) != nil || !p.Completed {
					t.Fatal("completion lost")
				}
			}
		}
	}
	if counts["course.created"] != 1 || counts["course.updated"] != 3 || counts["material.viewed"] != 2 {
		t.Fatal(counts)
	}
}
