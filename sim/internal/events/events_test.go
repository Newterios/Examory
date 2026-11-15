package events

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/Newterios/Examory/sim/internal/params"
	"github.com/Newterios/Examory/sim/internal/world"
)

// Reference values computed with Python's uuid module and the legacy
// generator's event_identifier / answer_digest.
func TestIdentifiersMatchPythonReference(t *testing.T) {
	if got := UUID5("examory-sim:42:attempt:0:0"); got != "3f82b32d-9a3c-53e2-b6e4-29aeb7bf9b71" {
		t.Fatalf("UUID5 %s", got)
	}
	if got := AttemptID(42, 0, 0); got != "3f82b32d-9a3c-53e2-b6e4-29aeb7bf9b71" {
		t.Fatalf("AttemptID %s", got)
	}
	if got := EventID(42, 1790845200123, 0, 1, "answer:0:0"); got != "01a0f6b1-26fb-7bfc-82d7-b708650e31ba" {
		t.Fatalf("EventID %s", got)
	}
	if AnswerHash(0) != "b73e891e5560b9299e68e06380fda3431052c6fc8eff74f6874be002fbe65bf7" ||
		AnswerHash(3) != "28b2d7c2366c61d54d70351b4defb14f3f26ae8150739f8097b19ca788330bd0" {
		t.Fatal("AnswerHash differs from legacy answer_digest")
	}
	if FormatTime(1790845200123) != "2026-10-01T09:00:00.123Z" {
		t.Fatal(FormatTime(1790845200123))
	}
}

func config(t *testing.T, students int, sampling string) *world.Config {
	t.Helper()
	m, _, err := params.LoadModel("../../params/default.json")
	if err != nil {
		t.Fatal(err)
	}
	mc, _, err := params.LoadMechanisms("../../params/mechanisms.json")
	if err != nil {
		t.Fatal(err)
	}
	return &world.Config{
		Students: students, Attempts: 3, Questions: 20, AnomalyRate: .12,
		Weights: [4]float64{.35, .25, .25, .15}, Seed: 42,
		StartAt: time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC), Model: m, Mech: mc,
		ItemSampling: sampling, Worlds: world.AllWorlds,
	}
}

func csvBytes(t *testing.T, seed int64, recs []*world.Record) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := WriteCSV(&buf, Rows(seed, recs)); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestGeneratedDatasetsValidate(t *testing.T) {
	for _, sampling := range []string{world.SamplingFixed, world.SamplingPerAttempt} {
		cfg := config(t, 400, sampling)
		res, err := world.Generate(cfg)
		if err != nil {
			t.Fatal(err)
		}
		for _, w := range world.AllWorlds {
			recs := res.Worlds[w]
			data := csvBytes(t, cfg.Seed, recs)
			sum, err := Validate(bytes.NewReader(data), KeysOf(cfg.Seed, recs))
			if err != nil {
				t.Fatalf("%s/%s: %v", sampling, w, err)
			}
			if sum.Attempts != 400*3 {
				t.Fatalf("%s/%s: %d attempts", sampling, w, sum.Attempts)
			}
			anomalous := 0
			for k, v := range sum.ByType {
				if k != "none" {
					anomalous += v
				}
			}
			if w == world.Honest && anomalous != 0 || w != world.Honest && anomalous != 144 {
				t.Fatalf("%s/%s: %d anomalous attempts", sampling, w, anomalous)
			}
		}
	}
}

func TestNonCheaterRowsByteIdentical(t *testing.T) {
	cfg := config(t, 300, world.SamplingFixed)
	res, err := world.Generate(cfg)
	if err != nil {
		t.Fatal(err)
	}
	cheater := map[[2]int]bool{}
	for _, r := range res.Worlds[world.FI] {
		if r.IsAnomaly {
			cheater[[2]int{r.Attempt.Student, r.Attempt.Index}] = true
		}
	}
	rowsOf := func(w string) map[string]string {
		out := map[string]string{}
		for _, r := range res.Worlds[w] {
			if cheater[[2]int{r.Attempt.Student, r.Attempt.Index}] {
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
	// The full honest CSV restricted to non-cheater attempts equals the fi
	// and bl CSVs restricted the same way (same order, same bytes).
	filter := func(data []byte, ids map[string]string) string {
		var keep []string
		for _, line := range strings.Split(string(data), "\r\n") {
			if len(line) > 36 {
				if _, ok := ids[line[:36]]; ok {
					keep = append(keep, line)
				}
			}
		}
		return strings.Join(keep, "\n")
	}
	hc := filter(csvBytes(t, cfg.Seed, res.Worlds[world.Honest]), h)
	if hc != filter(csvBytes(t, cfg.Seed, res.Worlds[world.FI]), h) || hc != filter(csvBytes(t, cfg.Seed, res.Worlds[world.BL]), h) {
		t.Fatal("non-cheater rows differ in bytes or order")
	}
}

func TestDeterministicBytes(t *testing.T) {
	a, err := world.Generate(config(t, 200, world.SamplingFixed))
	if err != nil {
		t.Fatal(err)
	}
	b, err := world.Generate(config(t, 200, world.SamplingFixed))
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range world.AllWorlds {
		if !bytes.Equal(csvBytes(t, 42, a.Worlds[w]), csvBytes(t, 42, b.Worlds[w])) {
			t.Fatalf("world %s not deterministic", w)
		}
	}
	var ta, tb bytes.Buffer
	if err := WriteTruth(&ta, 42, world.AllWorlds, a); err != nil {
		t.Fatal(err)
	}
	if err := WriteTruth(&tb, 42, world.AllWorlds, b); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(ta.Bytes(), tb.Bytes()) {
		t.Fatal("truth not deterministic")
	}
	c := config(t, 200, world.SamplingFixed)
	c.Seed = 43
	r, err := world.Generate(c)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(csvBytes(t, 43, r.Worlds[world.BL]), csvBytes(t, 42, a.Worlds[world.BL])) {
		t.Fatal("seed has no effect")
	}
}

func TestValidatorRejectsCorruption(t *testing.T) {
	cfg := config(t, 40, world.SamplingFixed)
	res, err := world.Generate(cfg)
	if err != nil {
		t.Fatal(err)
	}
	recs := res.Worlds[world.BL]
	good := string(csvBytes(t, cfg.Seed, recs))
	keys := KeysOf(cfg.Seed, recs)
	lines := strings.Split(good, "\r\n")
	// Find a submitted row and an answered row with a change.
	var sub, ans int
	for i, l := range lines {
		if strings.Contains(l, "quiz.submitted") && sub == 0 {
			sub = i
		}
		if strings.Contains(l, `""is_change"":false`) && ans == 0 {
			ans = i
		}
	}
	mut := map[string]func() string{
		"header": func() string { return strings.Replace(good, "anomaly_type", "kind", 1) },
		"score": func() string {
			l := append([]string(nil), lines...)
			l[sub] = strings.Replace(l[sub], `""score"":`, `""score"":1`, 1)
			return strings.Join(l, "\r\n")
		},
		"changes": func() string {
			l := append([]string(nil), lines...)
			l[sub] = strings.Replace(l[sub], `""answers_changed"":`, `""answers_changed"":9`, 1)
			return strings.Join(l, "\r\n")
		},
		"is_change": func() string {
			l := append([]string(nil), lines...)
			l[ans] = strings.Replace(l[ans], `""is_change"":false`, `""is_change"":true`, 1)
			return strings.Join(l, "\r\n")
		},
		"extra-key": func() string {
			l := append([]string(nil), lines...)
			l[ans] = strings.Replace(l[ans], `""is_change"":false`, `""is_change"":false,""x"":1`, 1)
			return strings.Join(l, "\r\n")
		},
		"label": func() string {
			l := append([]string(nil), lines...)
			l[ans] = strings.TrimSuffix(l[ans], ",0,none") + ",1,none"
			return strings.Join(l, "\r\n")
		},
		"drop-submit": func() string {
			l := append([]string(nil), lines[:sub]...)
			return strings.Join(append(l, lines[sub+1:]...), "\r\n")
		},
		"order": func() string {
			l := append([]string(nil), lines...)
			l[1], l[2] = l[2], l[1]
			return strings.Join(l, "\r\n")
		},
		"elapsed": func() string {
			l := append([]string(nil), lines...)
			l[ans] = strings.Replace(l[ans], `""time_since_attempt_start_ms"":`, `""time_since_attempt_start_ms"":1`, 1)
			return strings.Join(l, "\r\n")
		},
		"uuid-ts": func() string {
			l := append([]string(nil), lines...)
			l[ans] = "01a0f6b1-0000" + l[ans][13:]
			return strings.Join(l, "\r\n")
		},
	}
	if _, err := Validate(strings.NewReader(good), keys); err != nil {
		t.Fatal(err)
	}
	for name, f := range mut {
		t.Run(name, func(t *testing.T) {
			bad := f()
			if bad == good {
				t.Fatal("mutation did not apply")
			}
			if _, err := Validate(strings.NewReader(bad), keys); err == nil {
				t.Fatal("corruption accepted")
			}
		})
	}
}
