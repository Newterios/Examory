package world

import (
	"testing"
	"time"

	"github.com/Newterios/Examory/sim/internal/params"
)

var defaultWeights = [4]float64{.35, .25, .25, .15}

func TestQuotasMatchLegacy(t *testing.T) {
	// Expected values computed with ml/datasets/generate_quiz_behavior.py make_labels.
	cases := []struct {
		students, attempts int
		rate               float64
		w                  [4]float64
		want               [4]int
	}{
		{2000, 3, .12, defaultWeights, [4]int{252, 180, 180, 108}},
		{300, 3, .12, defaultWeights, [4]int{39, 27, 26, 16}},
		{2000, 3, .01, defaultWeights, [4]int{22, 15, 14, 9}},
		{2000, 3, .05, defaultWeights, [4]int{106, 75, 74, 45}},
		{137, 2, .31, [4]float64{1, 1, 1, 1}, [4]int{23, 21, 20, 21}},
		{500, 4, .2, [4]float64{0, 1, 1, 0}, [4]int{0, 200, 200, 0}},
		{999, 1, .5, [4]float64{.2, .3, .5, 0}, [4]int{100, 150, 250, 0}},
		{60, 3, .3, [4]float64{0, 0, 1, 0}, [4]int{0, 0, 54, 0}},
	}
	for _, c := range cases {
		q, _ := Quotas(c.students, c.attempts, c.rate, c.w)
		l, err := AssignLabels(7, c.students, c.attempts, c.rate, c.w)
		if err != nil {
			t.Fatal(err)
		}
		got := map[string]int{}
		for _, s := range l.Slot {
			if s != "" {
				got[s]++
			}
		}
		for i, s := range Slots {
			if q[s] != c.want[i] || got[s] != c.want[i] {
				t.Fatalf("%+v: slot %s quota %d assigned %d want %d", c, s, q[s], got[s], c.want[i])
			}
		}
	}
}

func TestCollusionPairsWithinCohort(t *testing.T) {
	l, err := AssignLabels(42, 2000, 3, .12, defaultWeights)
	if err != nil {
		t.Fatal(err)
	}
	for i, p := range l.Partner {
		if (p >= 0) != (l.Slot[i] == "collusion") {
			t.Fatal("partner iff collusion")
		}
		if p >= 0 && (l.Partner[p] != i || p/CohortSize != i/CohortSize || p == i) {
			t.Fatalf("bad pair %d-%d", i, p)
		}
	}
	// Cohorts of 20 are even, so floor(students/2) pairs always exist and an
	// all-collusion assignment at the maximal rate succeeds.
	for _, n := range []int{1, 20, 21, 41} {
		l, err := AssignLabels(1, n, 1, 1, [4]float64{0, 0, 1, 0})
		if err != nil {
			t.Fatal(err)
		}
		if got, want := l.Counts["collusion"], n/2*2; got != want {
			t.Fatalf("n=%d: collusion %d want %d", n, got, want)
		}
	}
}

func testConfig(t *testing.T, students int) *Config {
	t.Helper()
	m, _, err := params.LoadModel("../../params/default.json")
	if err != nil {
		t.Fatal(err)
	}
	mc, _, err := params.LoadMechanisms("../../params/mechanisms.json")
	if err != nil {
		t.Fatal(err)
	}
	return &Config{
		Students: students, Attempts: 3, Questions: 20, AnomalyRate: .12, Weights: defaultWeights,
		Seed: 42, StartAt: time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC), Model: m, Mech: mc,
		ItemSampling: SamplingFixed, Worlds: AllWorlds,
	}
}

func TestCheaterSetsIdenticalAcrossWorlds(t *testing.T) {
	res, err := Generate(testConfig(t, 400))
	if err != nil {
		t.Fatal(err)
	}
	type key struct{ s, k int }
	fi, bl := map[key]*Record{}, map[key]*Record{}
	for _, r := range res.Worlds[FI] {
		if r.IsAnomaly {
			fi[key{r.Attempt.Student, r.Attempt.Index}] = r
		}
	}
	for _, r := range res.Worlds[BL] {
		if r.IsAnomaly {
			bl[key{r.Attempt.Student, r.Attempt.Index}] = r
		}
	}
	if len(fi) != len(bl) || len(fi) == 0 {
		t.Fatalf("cheater counts differ: fi %d bl %d", len(fi), len(bl))
	}
	for k, r := range fi {
		b, ok := bl[k]
		if !ok || b.Slot != r.Slot || BLMechanism[r.AnomalyType] != b.AnomalyType {
			t.Fatalf("cheater %v differs between worlds", k)
		}
		if k.k != 2 || !r.IsFinal {
			t.Fatal("only final attempts may be anomalous")
		}
		if (r.Partner >= 0) != (b.Partner >= 0) || (r.Partner >= 0 && r.Partner != b.Partner) {
			t.Fatal("collusion partners differ")
		}
	}
	for _, r := range res.Worlds[Honest] {
		if r.IsAnomaly || r.AnomalyType != "none" {
			t.Fatal("honest world contains anomalies")
		}
	}
}

func TestNonCheaterAttemptsShared(t *testing.T) {
	res, err := Generate(testConfig(t, 200))
	if err != nil {
		t.Fatal(err)
	}
	h := res.Worlds[Honest]
	for _, w := range []string{FI, BL} {
		recs := res.Worlds[w]
		if len(recs) != len(h) {
			t.Fatal("record counts differ")
		}
		for i := range recs {
			if !recs[i].IsAnomaly && recs[i].Attempt != h[i].Attempt {
				t.Fatalf("world %s: non-cheater attempt %d is not the honest attempt", w, i)
			}
		}
	}
}

func TestPerAttemptSampling(t *testing.T) {
	cfg := testConfig(t, 100)
	cfg.ItemSampling = SamplingPerAttempt
	res, err := Generate(cfg)
	if err != nil {
		t.Fatal(err)
	}
	a0 := res.Worlds[Honest][0].Attempt
	a1 := res.Worlds[Honest][1].Attempt
	if a0.Items[0].ID == a1.Items[0].ID {
		t.Fatal("per-attempt items should differ between attempts")
	}
	// With a pool.
	cfg.Model.Items = nil
	for i := 0; i < 60; i++ {
		cfg.Model.Items = append(cfg.Model.Items, params.PoolItem{
			ID: "pool_" + string(rune('A'+i%26)) + string(rune('A'+i/26)), A: 1, B: float64(i%7) - 3, C: .2, Beta: 10, Sigma: .4,
			DistractorWeights: []float64{.5, .3, .2},
		})
	}
	res, err = Generate(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range res.Worlds[BL] {
		ids := map[string]bool{}
		for _, it := range r.Attempt.Items {
			if ids[it.ID] {
				t.Fatal("pool item sampled twice within an attempt")
			}
			ids[it.ID] = true
		}
		if r.Partner >= 0 && r.IsAnomaly {
			// Both pair members answer the same items.
			var other *Record
			for _, o := range res.Worlds[BL] {
				if o.Attempt.Student == r.Partner && o.IsFinal {
					other = o
				}
			}
			for q := range r.Attempt.Items {
				if r.Attempt.Items[q].ID != other.Attempt.Items[q].ID {
					t.Fatal("pair members answer different items")
				}
			}
		}
	}
	cfg.Questions = 61
	if _, err := Generate(cfg); err == nil {
		t.Fatal("pool smaller than questions must fail")
	}
}

func TestIntensityZero(t *testing.T) {
	cfg := testConfig(t, 200)
	zero := 0.0
	cfg.Intensity = &zero
	res, err := Generate(cfg)
	if err != nil {
		t.Fatal(err)
	}
	h := res.Worlds[Honest]
	for i, r := range res.Worlds[BL] {
		if !r.IsAnomaly {
			continue
		}
		switch r.AnomalyType {
		case "preknowledge", "lookup":
			if r.ItemsAffected != 0 || r.Attempt.Duration() != h[i].Attempt.Duration() || r.Attempt.Score() != h[i].Attempt.Score() {
				t.Fatalf("%s with intensity 0 changed the attempt", r.AnomalyType)
			}
		case "copy_peer":
			if r.ItemsAffected != 0 {
				t.Fatal("copy_peer with coverage 0 copied items")
			}
		case "proxy":
			if r.ProxyTheta != r.Person.Ability(2) {
				t.Fatal("proxy with scale 0 must keep ability")
			}
		}
		if r.Intensity != 0 {
			t.Fatal("intensity_used not recorded")
		}
	}
}

func TestConfigValidation(t *testing.T) {
	bad := []func(*Config){
		func(c *Config) { c.Students = 0 },
		func(c *Config) { c.Attempts = 0 },
		func(c *Config) { c.Questions = 0 },
		func(c *Config) { c.AnomalyRate = 0.5 },
		func(c *Config) { c.AnomalyRate = -0.1 },
		func(c *Config) { c.Weights = [4]float64{} },
		func(c *Config) { c.Weights[0] = -1 },
		func(c *Config) { c.ItemSampling = "random" },
		func(c *Config) { x := 1.5; c.Intensity = &x },
		func(c *Config) { c.Worlds = nil },
		func(c *Config) { c.Worlds = []string{"x"} },
		func(c *Config) { c.Attempts = 1; c.AnomalyRate = 0.1 },
	}
	for i, f := range bad {
		c := testConfig(t, 100)
		f(c)
		if _, err := Generate(c); err == nil {
			t.Fatalf("case %d accepted", i)
		}
	}
}
