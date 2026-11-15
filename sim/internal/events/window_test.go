package events

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"runtime"
	"strings"
	"testing"

	"github.com/Newterios/Examory/sim/internal/world"
)

// SHA-256 of the seed-42 default outputs (2000 students, default.json,
// mechanisms.json) of simulator version 0.2.0, produced on darwin/arm64 by
// the 0.2.0 binary. With the quiz window off (the default) the current
// simulator must reproduce them byte for byte.
var golden020 = map[string]map[string]string{
	world.SamplingFixed: {
		world.Honest: "1e705c2f612a3aa8085af10093ad057454b41657fa05729706507fee9caf5b15",
		world.FI:     "ebf392137b278ff8e7754a5d6e6bbeec6d7bed625d42ef7d66653563de49a074",
		world.BL:     "8d9a6294775bd5607f9f1f6c488eb597644b4a01221e9d8ca7f40c200107b760",
		"truth":      "a64f83df96938390bb2dd78c5aa0e9901c76597087c2becd7271cfb14dc30bff",
	},
	world.SamplingPerAttempt: {
		world.Honest: "4aa50d4fd4fd9b2dfc1ec56846f1024572d68d5f5601b23fb1bd350839697905",
		world.FI:     "8b033bf6bd3f85a422821907a04dc9701c933a763d7c584146b12aef6c87d6c2",
		world.BL:     "b602792a644e872ad2f3507d6c59f9aa9b502c76a0a4533695bbffbd57c599ce",
		"truth":      "334ab4b9d7e0814107b497320f08ad10872646c8f8117575529fc3b2285fc595",
	},
}

func sha(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func truthBytes(t *testing.T, seed int64, res *world.Result) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := WriteTruth(&buf, seed, world.AllWorlds, res); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestWindowOffReproducesVersion020(t *testing.T) {
	if runtime.GOARCH != "arm64" {
		t.Skip("golden hashes were produced on arm64; float rounding may differ elsewhere")
	}
	if testing.Short() {
		t.Skip("full 2000-student runs")
	}
	for sampling, want := range golden020 {
		cfg := config(t, 2000, sampling)
		if cfg.Model.QuizWindow.CloseAfterMs != 0 {
			t.Fatal("default.json must not enable the quiz window")
		}
		cfg.CloseAfterMs, cfg.AutoSubmitJitterMs = 0, cfg.Model.QuizWindow.AutoSubmitJitterMs
		res, err := world.Generate(cfg)
		if err != nil {
			t.Fatal(err)
		}
		for _, w := range world.AllWorlds {
			if got := sha(csvBytes(t, cfg.Seed, res.Worlds[w])); got != want[w] {
				t.Fatalf("%s %s.csv: %s, want %s", sampling, w, got, want[w])
			}
		}
		if got := sha(truthBytes(t, cfg.Seed, res)); got != want["truth"] {
			t.Fatalf("%s truth.csv: %s, want %s", sampling, got, want["truth"])
		}
	}
}

// windowConfig closes every sitting 1 200 000 ms (20 min) after it opens,
// which auto-submits some but not all default-parameter attempts.
func windowConfig(t *testing.T, students int, sampling string) *world.Config {
	t.Helper()
	cfg := config(t, students, sampling)
	cfg.CloseAfterMs, cfg.AutoSubmitJitterMs = 1_200_000, 2000
	return cfg
}

func TestWindowDatasetsValidate(t *testing.T) {
	for _, sampling := range []string{world.SamplingFixed, world.SamplingPerAttempt} {
		cfg := windowConfig(t, 400, sampling)
		res, err := world.Generate(cfg)
		if err != nil {
			t.Fatal(err)
		}
		for _, w := range world.AllWorlds {
			recs := res.Worlds[w]
			auto, partial := 0, 0
			for _, r := range recs {
				if r.Attempt.AutoSubmit > 0 {
					auto++
					if MaxScore(r.Attempt) != r.Attempt.Answered() {
						t.Fatal("max_score of an auto-submitted attempt must count answered items")
					}
					if r.Attempt.Answered() < len(r.Attempt.Items) {
						partial++
					}
				} else if MaxScore(r.Attempt) != len(r.Attempt.Items) {
					t.Fatal("max_score of a natural submission must count all items")
				}
			}
			if auto == 0 || partial == 0 || auto == len(recs) {
				t.Fatalf("%s/%s: %d auto-submitted, %d with unanswered items", sampling, w, auto, partial)
			}
			// Validate re-implements read_attempts and, with keys, checks that
			// score equals the correct answers saved before the close.
			sum, err := Validate(bytes.NewReader(csvBytes(t, cfg.Seed, recs)), KeysOf(cfg.Seed, recs))
			if err != nil {
				t.Fatalf("%s/%s: %v", sampling, w, err)
			}
			if sum.Attempts != 400*3 {
				t.Fatalf("%s/%s: %d attempts", sampling, w, sum.Attempts)
			}
		}
	}
}

func TestWindowDeterministicAndTruthColumns(t *testing.T) {
	a, err := world.Generate(windowConfig(t, 200, world.SamplingFixed))
	if err != nil {
		t.Fatal(err)
	}
	b, err := world.Generate(windowConfig(t, 200, world.SamplingFixed))
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range world.AllWorlds {
		if !bytes.Equal(csvBytes(t, 42, a.Worlds[w]), csvBytes(t, 42, b.Worlds[w])) {
			t.Fatalf("world %s not deterministic with the window on", w)
		}
	}
	ta, tb := truthBytes(t, 42, a), truthBytes(t, 42, b)
	if !bytes.Equal(ta, tb) {
		t.Fatal("truth not deterministic with the window on")
	}
	header := strings.SplitN(string(ta), "\r\n", 2)[0]
	if want := strings.Join(append(append([]string(nil), TruthColumns...), WindowColumns...), ","); header != want {
		t.Fatalf("truth header %q, want %q", header, want)
	}
	off, err := world.Generate(config(t, 200, world.SamplingFixed))
	if err != nil {
		t.Fatal(err)
	}
	if header := strings.SplitN(string(truthBytes(t, 42, off)), "\r\n", 2)[0]; header != strings.Join(TruthColumns, ",") {
		t.Fatal("window columns must appear only when the window is enabled")
	}
	// The audit columns agree with the attempt: auto_submitted, answered
	// items and the natural duration/score.
	for _, r := range a.Worlds[world.BL] {
		row := TruthRow(42, r)
		n := len(TruthColumns)
		if (row[n] == "1") != (r.Attempt.AutoSubmit > 0) || (row[n] == "0" && row[n+2] != row[23]) {
			t.Fatalf("audit columns %v inconsistent with duration %s", row[n:], row[23])
		}
	}
}

func TestWindowNonCheaterRowsByteIdentical(t *testing.T) {
	cfg := windowConfig(t, 300, world.SamplingFixed)
	res, err := world.Generate(cfg)
	if err != nil {
		t.Fatal(err)
	}
	rowsOf := func(w string) map[string]string {
		out := map[string]string{}
		for _, r := range res.Worlds[w] {
			if r.Slot != "none" && r.IsFinal {
				continue
			}
			for _, row := range AppendAttempt(nil, cfg.Seed, r) {
				out[row.EventID] = strings.Join(row.Fields, "\x1f")
			}
		}
		return out
	}
	h, fi, bl := rowsOf(world.Honest), rowsOf(world.FI), rowsOf(world.BL)
	if len(h) == 0 || len(h) != len(fi) || len(h) != len(bl) {
		t.Fatalf("row counts %d %d %d", len(h), len(fi), len(bl))
	}
	for id, row := range h {
		if fi[id] != row || bl[id] != row {
			t.Fatalf("event %s differs across worlds", id)
		}
	}
}
