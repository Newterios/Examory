package config

import (
	"strings"
	"testing"
	"time"
)

func TestEnv(t *testing.T) {
	e := FromMap(map[string]string{
		"A":   " hello ",
		"N":   "42",
		"BAD": "x",
		"D":   "5m",
		"L":   "a, b,,c",
		"S":   "supersecretvalue",
		"E":   "nope",
		"U":   "ftp://x",
	})
	if e.String("A", "") != "hello" {
		t.Fatal("trim")
	}
	if e.Int("N", 0, IntRange{0, 100}) != 42 {
		t.Fatal("int")
	}
	e.Int("BAD", 1, IntRange{0, 10})
	if e.Duration("D", 0, DurationRange{Min: time.Second}) != 5*time.Minute {
		t.Fatal("duration")
	}
	if got := e.List("L", nil); strings.Join(got, "|") != "a|b|c" {
		t.Fatalf("list %v", got)
	}
	_ = e.Secret("S")
	_ = e.Secret("MISSING_SECRET")
	e.Enum("E", "x", "x", "y")
	e.URL("U", "")
	err := e.Err()
	if err == nil {
		t.Fatal("expected errors")
	}
	msg := err.Error()
	for _, want := range []string{"BAD", "MISSING_SECRET", "E:", "U:"} {
		if !strings.Contains(msg, want) {
			t.Errorf("missing %q in %q", want, msg)
		}
	}
	if strings.Contains(msg, "supersecretvalue") {
		t.Fatal("secret leaked into error")
	}
}
