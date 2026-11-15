package model

import (
	"math"
	"testing"

	"github.com/Newterios/Examory/sim/internal/params"
	"github.com/Newterios/Examory/sim/internal/rng"
)

func loadModel(t *testing.T) *params.Model {
	t.Helper()
	m, _, err := params.LoadModel("../../params/default.json")
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func testItem() Item {
	it := Item{ID: "q", A: 1.2, B: 0.3, C: 0.2, Beta: 10, Sigma: 0.5, Key: 2}
	it.W = [4]float64{0.6, 0.3, 0, 0.1}
	return it
}

func TestAccuracyIncreasesWithAbility(t *testing.T) {
	m := loadModel(t)
	it := testItem()
	const n = 40000
	prev := -1.0
	for _, theta := range []float64{-2, -1, 0, 1, 2} {
		r := Responder{Ability: theta}
		correct := 0
		for i := 0; i < n; i++ {
			d := DrawItem(1, i, 0, int(theta*10)+50)
			res := Respond(m, r, it, &d, Override{ForceChanges: 0})
			if res.Events[0].Option == it.Key {
				correct++
			}
		}
		acc := float64(correct) / n
		want := it.C + (1-it.C)*Logistic(it.A*(theta-it.B))
		if math.Abs(acc-want) > 0.01 {
			t.Fatalf("theta %v: accuracy %v want %v", theta, acc, want)
		}
		if acc <= prev {
			t.Fatalf("accuracy not increasing in theta: %v after %v", acc, prev)
		}
		prev = acc
	}
}

func TestLogTimeTracksBetaMinusTau(t *testing.T) {
	m := loadModel(t)
	it := testItem()
	for _, tau := range []float64{-0.5, 0, 0.4} {
		r := Responder{Tau: tau}
		var s float64
		const n = 30000
		for i := 0; i < n; i++ {
			d := DrawItem(2, i, 1, 0)
			res := Respond(m, r, it, &d, NoOverride)
			s += math.Log(float64(res.Events[0].SinceShown))
		}
		if got, want := s/n, it.Beta-tau; math.Abs(got-want) > 0.01 {
			t.Fatalf("tau %v: mean log T %v want %v", tau, got, want)
		}
	}
}

func TestRapidGuesses(t *testing.T) {
	m := loadModel(t)
	it := testItem()
	r := Responder{Ability: 3, Disengage: 1, Revise: 1}
	correct := 0
	const n = 40000
	var logs float64
	for i := 0; i < n; i++ {
		d := DrawItem(3, i, 0, 0)
		res := Respond(m, r, it, &d, NoOverride)
		if !res.Rapid || res.Knows || len(res.Events) != 1 {
			t.Fatal("rapid guess must be a single unrevised answer")
		}
		if res.Events[0].Option == it.Key {
			correct++
		}
		logs += math.Log(float64(res.Events[0].SinceShown))
	}
	if acc := float64(correct) / n; math.Abs(acc-0.25) > 0.01 {
		t.Fatalf("rapid accuracy %v", acc)
	}
	if ml := logs / n; math.Abs(ml-m.RapidGuess.LogMsMean) > 0.01 {
		t.Fatalf("rapid mean log time %v", ml)
	}
}

func TestKnowsImpliesCorrectAndGuessRate(t *testing.T) {
	m := loadModel(t)
	it := testItem()
	r := Responder{Ability: 0}
	notKnow, guessCorrect := 0, 0
	for i := 0; i < 50000; i++ {
		d := DrawItem(4, i, 0, 0)
		res := Respond(m, r, it, &d, Override{ForceChanges: 0})
		if res.Knows && res.Events[0].Option != it.Key {
			t.Fatal("knows but wrong")
		}
		if !res.Knows {
			notKnow++
			if res.Events[0].Option == it.Key {
				guessCorrect++
			}
		}
	}
	if g := float64(guessCorrect) / float64(notKnow); math.Abs(g-it.C) > 0.01 {
		t.Fatalf("P(correct | not knows) = %v want %v", g, it.C)
	}
}

func TestRevisionProcess(t *testing.T) {
	m := loadModel(t)
	it := testItem()
	r := Responder{Ability: -1, Revise: 0.3}
	changedWrong, firstWrong := 0, 0
	for i := 0; i < 50000; i++ {
		d := DrawItem(5, i, 0, 0)
		res := Respond(m, r, it, &d, NoOverride)
		if len(res.Events) > MaxChanges+1 {
			t.Fatal("too many changes")
		}
		for j := 1; j < len(res.Events); j++ {
			e, p := res.Events[j], res.Events[j-1]
			if !e.IsChange || e.SinceShown <= p.SinceShown || e.Option == p.Option {
				t.Fatalf("bad revision sequence %+v", res.Events)
			}
			if p.Option == it.Key && e.Option == it.Key {
				t.Fatal("right to right")
			}
		}
		if res.Dwell != res.Events[len(res.Events)-1].SinceShown {
			t.Fatal("dwell must equal last event time")
		}
		if res.Events[0].Option != it.Key {
			firstWrong++
			if len(res.Events) > 1 {
				changedWrong++
			}
		}
	}
	want := math.Min(0.95, 0.3*m.Revision.ChangeMultWrong)
	if got := float64(changedWrong) / float64(firstWrong); math.Abs(got-want) > 0.015 {
		t.Fatalf("P(change | wrong) = %v want %v", got, want)
	}
}

func TestOverrideShiftKeepsCorrectAndTimes(t *testing.T) {
	m := loadModel(t)
	it := testItem()
	r := Responder{Ability: -0.5, Revise: 0.2, Disengage: 0.1}
	gained := 0
	for i := 0; i < 20000; i++ {
		d := DrawItem(6, i, 0, 0)
		h := Respond(m, r, it, &d, NoOverride)
		j := Respond(m, r, it, &d, Override{Shift: 0.3, Cap: 0.98, ForceChanges: h.Changes()})
		if len(h.Events) != len(j.Events) || h.Dwell != j.Dwell || h.Gap != j.Gap {
			t.Fatal("timings changed")
		}
		for k := range h.Events {
			if h.Events[k].SinceShown != j.Events[k].SinceShown {
				t.Fatal("event time changed")
			}
		}
		hc, jc := h.Events[0].Option == it.Key, j.Events[0].Option == it.Key
		if hc && !jc {
			t.Fatal("correct first answer became wrong")
		}
		if jc && !hc {
			gained++
		}
	}
	if gained == 0 {
		t.Fatal("shift had no effect")
	}
}

func TestPriorItemAndPool(t *testing.T) {
	m := loadModel(t)
	for i := 0; i < 200; i++ {
		it := PriorItem(rng.New(1, "item", i), "x", m.ItemPrior)
		var sum float64
		for o := 0; o < NumOptions; o++ {
			if o == it.Key && it.W[o] != 0 {
				t.Fatal("key has distractor weight")
			}
			sum += it.W[o]
		}
		if math.Abs(sum-1) > 1e-12 || it.A <= 0 || it.Sigma <= 0 || it.C != m.ItemPrior.C {
			t.Fatalf("bad item %+v", it)
		}
		r := it.Rarest()
		for o := 0; o < NumOptions; o++ {
			if o != it.Key && it.W[o] < it.W[r] {
				t.Fatal("Rarest is not minimal")
			}
		}
	}
	p := params.PoolItem{ID: "abc", A: 1, B: 0, C: 0.25, Beta: 10, Sigma: 0.4, DistractorWeights: []float64{0.7, 0.2, 0.1}}
	a, b := FromPool(9, p), FromPool(9, p)
	if a != b {
		t.Fatal("pool conversion not deterministic")
	}
}

func TestPickWeightedZeroWeights(t *testing.T) {
	it := Item{Key: 0, W: [4]float64{0, 1, 0, 0}}
	for _, u := range []float64{0, 0.3, 0.6, 0.9999} {
		o := PickWeighted(it, 1, u)
		if o == 0 || o == 1 {
			t.Fatalf("picked excluded option %d", o)
		}
	}
	if PickWeighted(it, -1, 0.99) != 1 {
		t.Fatal("weighted pick")
	}
}

func TestSimulateAttemptCheck(t *testing.T) {
	m := loadModel(t)
	items := make([]Item, 20)
	for q := range items {
		items[q] = PriorItem(rng.New(1, "item", q), "q", m.ItemPrior)
	}
	for i := 0; i < 300; i++ {
		p := DrawPerson(1, i, m.Person)
		a := Simulate(m, 1, i, 1, 0, ResponderOf(p, 1), items, nil)
		if err := a.Check(); err != nil {
			t.Fatal(err)
		}
		if a.Offset < 0 || a.Offset > OffsetMaxMs {
			t.Fatal("offset out of range")
		}
		b := Resimulate(m, a, ResponderOf(p, 1), nil)
		if b.Duration() != a.Duration() || b.Score() != a.Score() {
			t.Fatal("resimulation with same responder differs")
		}
	}
}

func TestPersonDistribution(t *testing.T) {
	m := loadModel(t)
	const n = 40000
	var st, stt, sxy, zeros float64
	for i := 0; i < n; i++ {
		p := DrawPerson(11, i, m.Person)
		st += p.Theta
		stt += p.Theta * p.Theta
		sxy += p.Theta * p.Tau
		if p.Disengage == 0 {
			zeros++
		}
		if p.Revise <= 0 || p.Revise >= 1 {
			t.Fatal("revise out of (0,1)")
		}
	}
	if sd := math.Sqrt(stt/n - (st/n)*(st/n)); math.Abs(sd-m.Person.ThetaSD) > 0.02 {
		t.Fatalf("theta sd %v", sd)
	}
	cov := sxy / n
	want := m.Person.ThetaTauCorr * m.Person.ThetaSD * m.Person.TauSD
	if math.Abs(cov-want) > 0.01 {
		t.Fatalf("cov(theta,tau) %v want %v", cov, want)
	}
	if z := zeros / n; math.Abs(z-m.Person.Disengage.PZero) > 0.01 {
		t.Fatalf("P(g=0) %v", z)
	}
}
