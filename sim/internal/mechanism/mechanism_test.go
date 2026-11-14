package mechanism

import (
	"math"
	"testing"

	"github.com/Newterios/Examory/sim/internal/model"
	"github.com/Newterios/Examory/sim/internal/params"
	"github.com/Newterios/Examory/sim/internal/rng"
)

type fixture struct {
	m     *params.Model
	mc    *params.Mechanisms
	items []model.Item
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	m, _, err := params.LoadModel("../../params/default.json")
	if err != nil {
		t.Fatal(err)
	}
	mc, _, err := params.LoadMechanisms("../../params/mechanisms.json")
	if err != nil {
		t.Fatal(err)
	}
	items := make([]model.Item, 20)
	for q := range items {
		items[q] = model.PriorItem(rng.New(42, "item", q), "q", m.ItemPrior)
		items[q].ID = string(rune('a'+q%26)) + string(rune('a'+q/26))
	}
	return &fixture{m: m, mc: mc, items: items}
}

const seed = 42
const final = 2

func (f *fixture) honest(i int) (*model.Attempt, model.Person) {
	p := model.DrawPerson(seed, i, f.m.Person)
	return model.Simulate(f.m, seed, i, final, 1_800_000_000_000, model.ResponderOf(p, final), f.items, nil), p
}

func sameEvents(a, b []model.Event) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func mustCheck(t *testing.T, a *model.Attempt) {
	t.Helper()
	if err := a.Check(); err != nil {
		t.Fatal(err)
	}
}

func TestPreknowledge(t *testing.T) {
	f := newFixture(t)
	pk := f.mc.Preknowledge
	var naive, recalled, total int
	for i := 0; i < 400; i++ {
		h, _ := f.honest(i)
		hc := h.Clone()
		a, info := Preknowledge(f.m, pk, seed, h, pk.Coverage)
		mustCheck(t, a)
		if !sameEvents(h.Results[0].Events, hc.Results[0].Events) {
			t.Fatal("input mutated")
		}
		if info.Flag == "naive" {
			naive++
		}
		n := 0
		for q, r := range a.Results {
			hr := h.Results[q]
			if sameEvents(r.Events, hr.Events) && r.Dwell == hr.Dwell {
				continue
			}
			n++
			if len(r.Events) != 1 || r.Events[0].Option != a.Items[q].Key || r.Rapid {
				t.Fatal("recalled item must be a single correct non-rapid answer")
			}
			tm := r.Events[0].SinceShown
			if info.Flag == "naive" {
				hi := int64(math.Round(float64(hr.Time) * pk.SpeedupHigh))
				if tm > hi || (tm < f.m.MinResponseMs) {
					t.Fatalf("naive time %d outside [floor, %d]", tm, hi)
				}
			} else if tm != hr.Time {
				t.Fatal("disguising cheater must keep the honest time draw")
			}
			if r.Gap != hr.Gap {
				t.Fatal("gap changed")
			}
		}
		// items whose honest answer already equalled the recalled one at the
		// same time are counted by info but indistinguishable here.
		if n > info.ItemsAffected {
			t.Fatalf("changed %d items but reported %d", n, info.ItemsAffected)
		}
		recalled += info.ItemsAffected
		total += len(a.Items)
		if a.Offset != h.Offset || a.Review != h.Review {
			t.Fatal("attempt-level draws changed")
		}
	}
	rate := float64(recalled) / float64(total)
	if want := pk.Coverage * pk.Recall; math.Abs(rate-want) > 0.02 {
		t.Fatalf("recalled fraction %v want %v", rate, want)
	}
	if fr := float64(naive) / 400; math.Abs(fr-pk.NaiveProb) > 0.08 {
		t.Fatalf("naive fraction %v", fr)
	}
	// Coverage 0 leaves the attempt unchanged.
	h, _ := f.honest(1)
	a, info := Preknowledge(f.m, pk, seed, h, 0)
	if info.ItemsAffected != 0 || a.Duration() != h.Duration() || a.Score() != h.Score() {
		t.Fatal("coverage 0 changed the attempt")
	}
}

func TestLookup(t *testing.T) {
	f := newFixture(t)
	lk := f.mc.Lookup
	for i := 0; i < 400; i++ {
		h, _ := f.honest(i)
		a, info := Lookup(f.m, lk, seed, h, lk.Propensity)
		mustCheck(t, a)
		affected := 0
		for q, r := range a.Results {
			hr := h.Results[q]
			if sameEvents(r.Events, hr.Events) && r.Dwell == hr.Dwell {
				continue
			}
			affected++
			if hr.Knows || hr.Rapid {
				t.Fatal("lookup on a known or rapid-guessed item")
			}
			switch info.Flag {
			case "verify":
				if r.Events[0] != hr.Events[0] {
					t.Fatal("verify mode must keep the honest first answer and time")
				}
				if len(r.Events) > 2 {
					t.Fatal("verify mode produces at most one change")
				}
				if r.Dwell <= hr.Events[0].SinceShown {
					t.Fatal("verify mode must spend the lookup time")
				}
			case "lookup_then_answer":
				if len(r.Events) != 1 || r.Events[0].SinceShown <= hr.Events[0].SinceShown {
					t.Fatal("lookup-then-answer: single first answer after the lookup")
				}
			default:
				t.Fatalf("unknown flag %q", info.Flag)
			}
		}
		if affected != info.ItemsAffected {
			t.Fatalf("affected %d reported %d", affected, info.ItemsAffected)
		}
	}
}

func TestProxy(t *testing.T) {
	f := newFixture(t)
	px := f.mc.Proxy
	var gain float64
	for i := 0; i < 400; i++ {
		h, p := f.honest(i)
		a, info := Proxy(f.m, px, seed, h, p, 1)
		mustCheck(t, a)
		if info.ProxyTheta < p.Ability(final)+px.MinGain-1e-9 {
			t.Fatalf("proxy ability %v below %v", info.ProxyTheta, p.Ability(final)+px.MinGain)
		}
		for _, r := range a.Results {
			if r.Rapid {
				t.Fatal("proxy must not rapid-guess")
			}
		}
		if a.Offset != h.Offset || a.Review != h.Review {
			t.Fatal("attempt-level draws changed")
		}
		gain += float64(a.Score() - h.Score())
		// Scale 0 keeps the student's own ability.
		z, zi := Proxy(f.m, px, seed, h, p, 0)
		if math.Abs(zi.ProxyTheta-p.Ability(final)) > 1e-12 || len(z.Results) != len(h.Results) {
			t.Fatal("scale 0 must keep ability")
		}
	}
	if gain <= 0 {
		t.Fatal("proxy did not raise scores on average")
	}
}

func wall(a *model.Attempt) [][]int64 {
	shown := a.Shown()
	out := make([][]int64, len(a.Results))
	for q, r := range a.Results {
		for _, e := range r.Events {
			out[q] = append(out[q], a.Start()+shown[q]+e.SinceShown)
		}
	}
	return out
}

func TestCopyPeerCausality(t *testing.T) {
	f := newFixture(t)
	cp := f.mc.CopyPeer
	copied, follows := 0, 0
	for i := 0; i < 300; i++ {
		src, _ := f.honest(2 * i)
		cpy, _ := f.honest(2*i + 1)
		sa, ca, si, ci := CopyPeer(f.m, cp, seed, src, cpy, cp.Coverage, 2*i)
		mustCheck(t, sa)
		mustCheck(t, ca)
		if si.Role != "source" || ci.Role != "copier" {
			t.Fatal("roles")
		}
		if ca.Start() <= sa.Start() {
			t.Fatal("copier must start after the source")
		}
		sw, cw := wall(sa), wall(ca)
		affected := 0
		for q := range ca.Results {
			hr := cpy.Results[q]
			r := ca.Results[q]
			if sameEvents(r.Events, hr.Events) && r.Dwell == hr.Dwell {
				continue
			}
			affected++
			copied++
			// Every copier event reproduces a source answer given strictly
			// earlier on the same item.
			for j, e := range r.Events {
				ok := false
				for k, se := range sa.Results[q].Events {
					if se.Option == e.Option && sw[q][k] < cw[q][j] {
						ok = true
					}
				}
				if !ok {
					t.Fatalf("pair %d item %d: copied answer precedes its source", i, q)
				}
				if j > 0 {
					follows++
				}
			}
			if cw[q][0] <= sw[q][0] {
				t.Fatal("copied first answer precedes the source's first answer")
			}
		}
		if affected > ci.ItemsAffected {
			t.Fatal("copied item count")
		}
		// Source timing differs only by overhead added to gaps/review.
		for q := range sa.Results {
			if !sameEvents(sa.Results[q].Events, src.Results[q].Events) || sa.Results[q].Gap < src.Results[q].Gap {
				t.Fatal("source answers must stay honest")
			}
		}
	}
	if copied == 0 || follows == 0 {
		t.Fatalf("degenerate test: copied %d follows %d", copied, follows)
	}
}

func TestFIRapid(t *testing.T) {
	f := newFixture(t)
	fi := f.mc.FI
	for i := 0; i < 200; i++ {
		h, _ := f.honest(i)
		a, info := FIRapid(fi, seed, h)
		mustCheck(t, a)
		n := 0
		for q, r := range a.Results {
			hr := h.Results[q]
			if sameEvents(r.Events, hr.Events) {
				continue
			}
			n++
			for j, e := range r.Events {
				if e.Option != hr.Events[j].Option {
					t.Fatal("rapid recipe must not change answers")
				}
			}
			hi := int64(math.Round(float64(hr.Events[0].SinceShown)*fi.RapidScaleHigh)) + 1
			if r.Events[0].SinceShown > max(hi, fi.RapidMinMs) || r.Events[0].SinceShown < fi.RapidMinMs {
				t.Fatal("scaled first answer outside range")
			}
		}
		if n > info.ItemsAffected {
			t.Fatal("affected count")
		}
	}
}

func TestFIChanges(t *testing.T) {
	f := newFixture(t)
	fi := f.mc.FI
	for i := 0; i < 200; i++ {
		h, _ := f.honest(i)
		a, info := FIChanges(fi, seed, h)
		mustCheck(t, a)
		if a.Score() != h.Score() {
			t.Fatal("excessive changes must keep final answers")
		}
		n := 0
		for q, r := range a.Results {
			hr := h.Results[q]
			if r.Final() != hr.Final() {
				t.Fatal("final answer changed")
			}
			if sameEvents(r.Events, hr.Events) {
				continue
			}
			n++
			if c := r.Changes(); c < 1 || c > fi.ChangesMaxCount {
				t.Fatalf("change count %d", c)
			}
			T := hr.Events[0].SinceShown
			if r.Dwell != max(T, int64(len(r.Events))) {
				t.Fatalf("events must span the honest first-answer time: %d vs %d", r.Dwell, T)
			}
		}
		if n > info.ItemsAffected {
			t.Fatal("affected count")
		}
	}
}

func TestFIScoreJump(t *testing.T) {
	f := newFixture(t)
	fi := f.mc.FI
	var up int
	for i := 0; i < 300; i++ {
		h, p := f.honest(i)
		a, info := FIScoreJump(f.m, fi, seed, h, model.ResponderOf(p, final))
		mustCheck(t, a)
		if a.Duration() != h.Duration() {
			t.Fatal("score jump must not change timings")
		}
		if info.Intensity < fi.ScoreJumpLow || info.Intensity > fi.ScoreJumpHigh {
			t.Fatal("delta outside range")
		}
		for q := range a.Results {
			hr, r := h.Results[q], a.Results[q]
			if hr.Events[0].Option == h.Items[q].Key && r.Events[0].Option != h.Items[q].Key {
				t.Fatal("correct first answer became wrong")
			}
			for j := range hr.Events {
				if hr.Events[j].SinceShown != r.Events[j].SinceShown {
					t.Fatal("event time changed")
				}
			}
		}
		up += a.Score() - h.Score()
	}
	if up <= 0 {
		t.Fatal("score jump did not raise scores")
	}
}

func TestFICollusion(t *testing.T) {
	f := newFixture(t)
	fi := f.mc.FI
	for i := 0; i < 200; i++ {
		l, _ := f.honest(2 * i)
		m, _ := f.honest(2*i + 1)
		la, ma, li, mi := FICollusion(fi, seed, l, m)
		mustCheck(t, la)
		mustCheck(t, ma)
		if li.Role != "leader" || mi.Role != "member" {
			t.Fatal("roles")
		}
		for _, a := range []*model.Attempt{la, ma} {
			if a.Changes() != 0 {
				t.Fatal("collusion attempts have no revisions")
			}
			if a.Offset < l.Offset || a.Offset > l.Offset+fi.CollusionStartWindowMs {
				t.Fatal("start outside the pair window")
			}
		}
		ls, ms := la.Start()+la.Duration(), ma.Start()+ma.Duration()
		if d := ls - ms; d > fi.CollusionSubmitWindowMs || -d > fi.CollusionSubmitWindowMs {
			t.Fatalf("submissions %d ms apart", d)
		}
		forcedBoth := 0
		for q := range la.Results {
			if la.Results[q].Final() == la.Items[q].Rarest() && ma.Results[q].Final() == la.Items[q].Rarest() {
				forcedBoth++
			}
			if la.Results[q].Gap != l.Results[q].Gap || ma.Results[q].Gap != l.Results[q].Gap {
				t.Fatal("gaps must be the leader's")
			}
		}
		if forcedBoth < int(math.RoundToEven(20*fi.CollusionForcedWrongFraction)) {
			t.Fatalf("only %d shared rare wrong answers", forcedBoth)
		}
	}
}

func submitAt(a *model.Attempt) int64 { return a.Start() + a.Duration() }

func lastEventAt(a *model.Attempt) int64 {
	var last int64
	for _, w := range wall(a) {
		for _, t := range w {
			last = max(last, t)
		}
	}
	return last
}

func equalAttempts(a, b *model.Attempt) bool {
	if a.Offset != b.Offset || a.Review != b.Review || len(a.Results) != len(b.Results) {
		return false
	}
	for q := range a.Results {
		if !sameEvents(a.Results[q].Events, b.Results[q].Events) || a.Results[q].Dwell != b.Results[q].Dwell || a.Results[q].Gap != b.Results[q].Gap {
			return false
		}
	}
	return true
}

func TestCopyPeerSyncSubmit(t *testing.T) {
	f := newFixture(t)
	off := f.mc.CopyPeer
	off.SyncSubmitProb = 0
	on := f.mc.CopyPeer
	sync, clamped := 0, 0
	const pairs = 400
	for i := 0; i < pairs; i++ {
		src, _ := f.honest(2 * i)
		cpy, _ := f.honest(2*i + 1)
		s0, c0, si0, _ := CopyPeer(f.m, off, seed, src, cpy, off.Coverage, 2*i)
		s1, c1, si1, ci1 := CopyPeer(f.m, on, seed, src, cpy, on.Coverage, 2*i)
		mustCheck(t, s1)
		mustCheck(t, c1)
		if si0.SyncSubmit != 0 || si1.SyncSubmit != ci1.SyncSubmit || si1.SyncSubmit < 0 {
			t.Fatal("sync flag")
		}
		// No review pause is ever shortened.
		if s1.Review < s0.Review || c1.Review < c0.Review {
			t.Fatal("review pause shortened")
		}
		for _, a := range []*model.Attempt{s1, c1} {
			if submitAt(a) <= lastEventAt(a) {
				t.Fatal("submission not after the last answer")
			}
		}
		if si1.SyncSubmit == 0 {
			// Non-sync pairs are identical to the mechanism without sync.
			if !equalAttempts(s0, s1) || !equalAttempts(c0, c1) {
				t.Fatal("non-sync pair changed")
			}
			continue
		}
		sync++
		d := submitAt(s1) - submitAt(c1)
		if d < 0 {
			d = -d
		}
		want := si1.SyncOffsetMs
		if want < 0 {
			want = -want
		}
		if si1.SyncClamped {
			clamped++
			if !equalAttempts(s0, s1) || d >= want {
				t.Fatal("clamped pair must be unchanged and closer than the draw")
			}
			continue
		}
		if d-want > 1 || want-d > 1 {
			t.Fatalf("pair %d: submissions %d ms apart, drawn %d", i, d, want)
		}
		// Only the earlier finisher moved, and only the review pause.
		moved := 0
		if s1.Review != s0.Review {
			moved++
		}
		if c1.Review != c0.Review {
			moved++
		}
		if moved > 1 {
			t.Fatal("both members moved")
		}
	}
	if fr := float64(sync) / pairs; math.Abs(fr-on.SyncSubmitProb) > 0.08 {
		t.Fatalf("sync fraction %v", fr)
	}
	t.Logf("sync pairs %d/%d, clamped %d", sync, pairs, clamped)
}
