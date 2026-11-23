package logging

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestRedactionAndRequestID(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, "test", "debug")
	ctx := WithRequestID(context.Background(), "req-1")
	l.InfoContext(ctx, "login",
		slog.String("password", "hunter2hunter2"),
		slog.String("refresh_token", "abc"),
		slog.Group("req", slog.String("Authorization", "Bearer xyz")),
		slog.String("answer_hash", "deadbeef"),
		slog.String("user_id", "u1"))
	out := buf.String()
	for _, leaked := range []string{"hunter2hunter2", "\"abc\"", "Bearer xyz"} {
		if strings.Contains(out, leaked) {
			t.Fatalf("leaked %q in %s", leaked, out)
		}
	}
	for _, want := range []string{`"request_id":"req-1"`, `"service":"test"`, `"answer_hash":"deadbeef"`, `"user_id":"u1"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %s in %s", want, out)
		}
	}
}
