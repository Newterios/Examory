package world

import (
	"sort"
	"strings"
	"testing"

	"github.com/Newterios/Examory/sim/internal/model"
)

// naturalCloseAt returns the q-quantile of the honest natural submission
// times relative to the sitting open, from a run without a window.
func naturalCloseAt(t *testing.T, cfg *Config, q float64) int64 {
	t.Helper()
	res, err := Generate(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var subs []int64
	for _, r := range res.Worlds[Honest] {
		subs = append(subs, r.Attempt.Offset+r.Attempt.Duration())
	}
	sort.Slice(subs, func(i, j int) bool { return subs[i] < subs[j] })
	return subs[int(q*float64(len(subs)-1))]
}

func sameAttempt(a, b *model.Attempt) bool {
	if a.Student != b.Student || a.Index != b.Index || a.Base != b.Base || a.Offset != b.Offset ||
		a.Review != b.Review || a.AutoSubmit != b.AutoSubmit || len(a.Results) != len(b.Results) {
		return false
	}
	for q := range a.Results {
		x, y := a.Results[q], b.Results[q]
		if x.Dwell != y.Dwell || x.Gap != y.Gap || len(x.Events) != len(y.Events) || a.Items[q].ID != b.Items[q].ID {
			return false
		}
		for j := range x.Events {
			if x.Events[j] != y.Events[j] {
				return false
			}
		}
	}
	return true
}

func TestWindowPairedWorldInvariants(t *testing.T) {
	for _, sampling := range []string{SamplingFixed, SamplingPerAttempt} {
		t.Run(sampling, func(t *testing.T) {
			off := testConfig(t, 400)
			off.ItemSampling = sampling
			natural, err := Generate(off)
			if err != nil {
				t.Fatal(err)
			}
			on := testConfig(t, 400)
			on.ItemSampling = sampling
			on.CloseAfterMs = naturalCloseAt(t, off, 0.75)
			on.AutoSubmitJitterMs = 2000
			res, err := Generate(on)
			if err != nil {
				t.Fatal(err)
			}
			if !res.Window || natural.Window {
				t.Fatal("Result.Window not set")
			}
			type key struct{ s, k int }
			autoAt := map[key]map[string]int64{}
			auto := 0
			for _, w := range AllWorlds {
				recs, nat := res.Worlds[w], natural.Worlds[w]
				if len(recs) != len(nat) {
					t.Fatal("record counts differ")
				}
				for i, r := range recs {
					a, n := r.Attempt, r.Natural
					// The window perturbs no draw: the natural attempt of every
					// record is the attempt generated without a window.
					if n == nil || !sameAttempt(n, nat[i].Attempt) {
						t.Fatalf("%s record %d: natural attempt differs from the window-off attempt", w, i)
					}
					if r.IsAnomaly != nat[i].IsAnomaly || r.AnomalyType != nat[i].AnomalyType || r.Partner != nat[i].Partner {
						t.Fatalf("%s record %d: labels changed by the window", w, i)
					}
					closeAt := a.Base + on.CloseAfterMs
					if n.Start()+n.Duration() <= closeAt {
						if a != n {
							t.Fatalf("%s record %d: natural submission before the close was modified", w, i)
						}
						continue
					}
					auto++
					sub := a.Start() + a.Duration()
					if a.AutoSubmit == 0 || sub < closeAt || sub > closeAt+on.AutoSubmitJitterMs {
						t.Fatalf("%s record %d: submitted at close%+d ms", w, i, sub-closeAt)
					}
					shown := n.Shown()
					for q, res := range a.Results {
						for j, e := range res.Events {
							if a.Start()+shown[q]+e.SinceShown >= closeAt || e != n.Results[q].Events[j] {
								t.Fatalf("%s record %d item %d: kept event is not a natural event before the close", w, i, q)
							}
						}
						if m := len(res.Events); m < len(n.Results[q].Events) &&
							n.Start()+shown[q]+n.Results[q].Events[m].SinceShown < closeAt {
							t.Fatalf("%s record %d item %d: an event before the close was dropped", w, i, q)
						}
					}
					k := key{a.Student, a.Index}
					if autoAt[k] == nil {
						autoAt[k] = map[string]int64{}
					}
					autoAt[k][w] = sub
				}
			}
			if auto == 0 {
				t.Fatal("no attempt was auto-submitted")
			}
			// Non-cheater attempts are the same object in all worlds.
			h := res.Worlds[Honest]
			for _, w := range []string{FI, BL} {
				for i, r := range res.Worlds[w] {
					if !r.IsAnomaly && r.Attempt != h[i].Attempt {
						t.Fatalf("world %s: non-cheater attempt %d is not shared", w, i)
					}
				}
			}
			// Common random numbers: an attempt auto-submitted in several
			// worlds is submitted at the same instant in each (one jitter
			// stream per student and attempt).
			for k, byWorld := range autoAt {
				var first int64 = -1
				for _, v := range byWorld {
					if first >= 0 && v != first {
						t.Fatalf("attempt %v: auto-submission times %v differ across worlds", k, byWorld)
					}
					first = v
				}
			}
		})
	}
}

func TestWindowValidation(t *testing.T) {
	bad := []func(*Config){
		func(c *Config) { c.CloseAfterMs = -1 },
		func(c *Config) { c.CloseAfterMs = model.OffsetMaxMs },
		func(c *Config) { c.CloseAfterMs = 86400001 },
		func(c *Config) { c.CloseAfterMs = 3600000; c.AutoSubmitJitterMs = -1 },
		func(c *Config) { c.CloseAfterMs = 3600000; c.AutoSubmitJitterMs = 3600001 },
	}
	for i, f := range bad {
		c := testConfig(t, 40)
		f(c)
		if err := c.Validate(); err == nil {
			t.Fatalf("case %d accepted", i)
		}
	}
	c := testConfig(t, 40)
	c.CloseAfterMs, c.AutoSubmitJitterMs = model.OffsetMaxMs+1, 0
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	// A window that closes right after the last possible start leaves late
	// starters without any answer, which the event contract cannot express.
	c = testConfig(t, 400)
	c.CloseAfterMs = model.OffsetMaxMs + 1
	if _, err := Generate(c); err == nil || !strings.Contains(err.Error(), "increase close_after_ms") {
		t.Fatalf("got %v, want an error asking for a later close", err)
	}
}
