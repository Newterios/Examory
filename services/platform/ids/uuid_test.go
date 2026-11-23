package ids

import (
	"regexp"
	"testing"
	"time"
)

var v7re = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestNewV7AtFormatAndTimestamp(t *testing.T) {
	ts := time.Date(2026, 11, 3, 10, 15, 42, 317_900_000, time.UTC)
	for i := 0; i < 1000; i++ {
		id := NewV7At(ts)
		if !v7re.MatchString(id) {
			t.Fatalf("bad format %q", id)
		}
		got, err := V7Time(id)
		if err != nil {
			t.Fatal(err)
		}
		if !got.Equal(ts.Truncate(time.Millisecond)) {
			t.Fatalf("timestamp %v != %v", got, ts.Truncate(time.Millisecond))
		}
	}
}

func TestUnique(t *testing.T) {
	seen := map[string]bool{}
	now := time.Now()
	for i := 0; i < 10000; i++ {
		id := NewV7At(now)
		if seen[id] {
			t.Fatal("duplicate id")
		}
		seen[id] = true
	}
}

func TestParse(t *testing.T) {
	cases := []struct {
		in string
		ok bool
	}{
		{"018f3a2e-7c1b-7d6e-9a3f-4d2c1b0a9e8f", true},
		{"018F3A2E-7C1B-7D6E-9A3F-4D2C1B0A9E8F", true},
		{"018f3a2e7c1b7d6e9a3f4d2c1b0a9e8f", false},
		{"018f3a2e-7c1b-7d6e-9a3f-4d2c1b0a9e8", false},
		{"zz8f3a2e-7c1b-7d6e-9a3f-4d2c1b0a9e8f", false},
		{"", false},
		{"' OR 1=1 --", false},
	}
	for _, c := range cases {
		if Valid(c.in) != c.ok {
			t.Errorf("Valid(%q) = %v, want %v", c.in, !c.ok, c.ok)
		}
	}
	got, err := Canonical("018F3A2E-7C1B-7D6E-9A3F-4D2C1B0A9E8F")
	if err != nil || got != "018f3a2e-7c1b-7d6e-9a3f-4d2c1b0a9e8f" {
		t.Fatalf("Canonical = %q, %v", got, err)
	}
	if _, err := V7Time("018f3a2e-7c1b-4d6e-9a3f-4d2c1b0a9e8f"); err == nil {
		t.Fatal("expected error for v4")
	}
}
