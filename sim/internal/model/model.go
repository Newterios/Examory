// Package model implements the honest latent response model: a
// three-parameter logistic IRT model for correctness, a lognormal
// response-time model (van der Linden, 2006), a zero-inflated rapid-guessing
// process, and an answer-revision process.
//
// All randomness for one item response is drawn up front into a Draws value
// from an independent stream keyed by (seed, student, attempt, position).
// Mechanisms reuse these draws (common random numbers), so a mechanism
// changes only the parts of a response it acts on.
package model

import (
	"fmt"
	"math"
	"sort"

	"github.com/Newterios/Examory/sim/internal/params"
	"github.com/Newterios/Examory/sim/internal/rng"
)

// NumOptions is the number of answer options per item.
const NumOptions = 4

// MaxChanges is the maximum number of honest answer changes per item.
const MaxChanges = 4

// OffsetMaxMs is the upper bound of the uniform attempt start offset (legacy).
const OffsetMaxMs = 600000

// Item holds the parameters of one multiple-choice item.
type Item struct {
	ID    string
	A     float64 // discrimination
	B     float64 // difficulty
	C     float64 // pseudo-guessing: P(correct | does not know)
	Beta  float64 // time intensity, natural log of ms
	Sigma float64 // residual log-time SD
	Key   int     // index of the correct option
	// W[o] is the probability of choosing wrong option o when guessing by
	// attraction; W[Key] == 0 and the wrong entries sum to 1.
	W [NumOptions]float64
}

// Rarest returns the wrong option with the smallest distractor weight
// (lowest index on ties).
func (it Item) Rarest() int {
	best := -1
	for o := 0; o < NumOptions; o++ {
		if o == it.Key {
			continue
		}
		if best < 0 || it.W[o] < it.W[best] {
			best = o
		}
	}
	return best
}

func wrongOptions(key int) [NumOptions - 1]int {
	var w [NumOptions - 1]int
	j := 0
	for o := 0; o < NumOptions; o++ {
		if o != key {
			w[j] = o
			j++
		}
	}
	return w
}

// PriorItem draws an item from the generating prior.
func PriorItem(s *rng.Stream, id string, ip params.ItemPrior) Item {
	it := Item{ID: id, C: ip.C}
	it.B = ip.BMean + ip.BSD*s.Norm()
	it.A = math.Exp(ip.LogAMean + ip.LogASD*s.Norm())
	it.Beta = ip.BetaMean + ip.BetaSD*s.Norm()
	it.Sigma = math.Exp(ip.LogSigmaMean + ip.LogSigmaSD*s.Norm())
	w := s.Dirichlet(ip.DistractorAlpha)
	sort.Sort(sort.Reverse(sort.Float64Slice(w)))
	it.Key = s.IntN(NumOptions)
	assignWeights(&it, w, s.Perm(NumOptions-1))
	return it
}

// FromPool converts a calibrated pool item. The key and the assignment of
// distractor weights to option positions are drawn from a stream keyed by
// the item id, so they are stable for a given seed across attempts.
func FromPool(seed int64, p params.PoolItem) Item {
	s := rng.NewString(seed, "pool-item", p.ID)
	it := Item{ID: p.ID, A: p.A, B: p.B, C: p.C, Beta: p.Beta, Sigma: p.Sigma}
	it.Key = s.IntN(NumOptions)
	assignWeights(&it, p.DistractorWeights, s.Perm(NumOptions-1))
	return it
}

func assignWeights(it *Item, w []float64, perm []int) {
	wrong := wrongOptions(it.Key)
	var sum float64
	for _, v := range w {
		sum += v
	}
	for i := range wrong {
		v := w[i]
		if sum > 0 {
			v /= sum
		} else {
			v = 1.0 / float64(len(wrong))
		}
		it.W[wrong[perm[i]]] = v
	}
}

// Person holds the latent parameters of one student.
type Person struct {
	Theta     float64 // ability at the first attempt
	Tau       float64 // speed (higher is faster)
	Gain      float64 // practice gain per attempt
	Disengage float64 // rapid-guess propensity g
	Revise    float64 // revision propensity r
}

// Ability returns theta + k * gain for zero-based attempt k.
func (p Person) Ability(k int) float64 { return p.Theta + float64(k)*p.Gain }

// DrawPerson draws the latent parameters of student i.
func DrawPerson(seed int64, student int, pp params.Person) Person {
	s := rng.New(seed, "person", student)
	z1, z2 := s.Norm(), s.Norm()
	rho := pp.ThetaTauCorr
	p := Person{
		Theta: pp.ThetaSD * z1,
		Tau:   pp.TauSD * (rho*z1 + math.Sqrt(1-rho*rho)*z2),
	}
	p.Gain = pp.PracticeGainMean + pp.PracticeGainSD*s.Norm()
	uZero := s.Float64()
	g := s.Beta(pp.Disengage.BetaA, pp.Disengage.BetaB)
	if uZero >= pp.Disengage.PZero {
		p.Disengage = g
	}
	p.Revise = s.Beta(pp.Revise.BetaA, pp.Revise.BetaB)
	return p
}

// Responder is the person-at-attempt state used to answer items.
type Responder struct {
	Ability   float64
	Tau       float64
	Disengage float64
	Revise    float64
}

// ResponderOf returns the responder of person p at attempt k.
func ResponderOf(p Person, k int) Responder {
	return Responder{Ability: p.Ability(k), Tau: p.Tau, Disengage: p.Disengage, Revise: p.Revise}
}

// RevDraw holds the variates of one potential answer change.
type RevDraw struct {
	U       float64 // change decision
	UTarget float64 // wrong -> right decision
	UOpt    float64 // choice among distractors
	Eps     float64 // delay normal
}

// Draws holds every variate used by one honest item response.
type Draws struct {
	URapid float64 // rapid-guess decision
	UCorr  float64 // correctness: knows iff UCorr < L, correct iff UCorr < P
	UOpt   float64 // choice of wrong option
	Eps    float64 // response-time normal (solution behaviour)
	ERapid float64 // response-time normal (rapid guess)
	GapEps float64 // inter-item gap normal
	Rev    [MaxChanges]RevDraw
}

// DrawItem draws the variates of item position q of attempt k of student i.
func DrawItem(seed int64, student, attempt, q int) Draws {
	s := rng.New(seed, "item-response", student, attempt, q)
	var d Draws
	d.URapid = s.Float64()
	d.UCorr = s.Float64()
	d.UOpt = s.Float64()
	d.Eps = s.Norm()
	d.ERapid = s.Norm()
	d.GapEps = s.Norm()
	for i := range d.Rev {
		d.Rev[i] = RevDraw{U: s.Float64(), UTarget: s.Float64(), UOpt: s.Float64(), Eps: s.Norm()}
	}
	return d
}

// Event is one saved answer within an item, relative to the item being shown.
type Event struct {
	SinceShown int64
	Option     int
	IsChange   bool
}

// ItemResult is the realised behaviour on one item.
type ItemResult struct {
	Events []Event // strictly increasing SinceShown; Events[0] is the first answer
	Dwell  int64   // time on the item before navigating away (ms since shown)
	Gap    int64   // navigation gap after leaving the item
	Rapid  bool    // response was a rapid guess
	Knows  bool    // latent knowledge state (false for rapid guesses)
	P      float64 // model probability of a correct first answer
	Time   int64   // solution-behaviour response-time draw T (even if rapid)
}

// Answered reports whether the item has at least one saved answer (false
// only for items left unanswered when the quiz window closed).
func (r ItemResult) Answered() bool { return len(r.Events) > 0 }

// Final returns the final option, or -1 for an unanswered item.
func (r ItemResult) Final() int {
	if len(r.Events) == 0 {
		return -1
	}
	return r.Events[len(r.Events)-1].Option
}

// Changes returns the number of change events.
func (r ItemResult) Changes() int {
	if len(r.Events) == 0 {
		return 0
	}
	return len(r.Events) - 1
}

// Clone deep-copies the result.
func (r ItemResult) Clone() ItemResult {
	c := r
	c.Events = append([]Event(nil), r.Events...)
	return c
}

// Override modifies the honest response process.
type Override struct {
	// Shift raises the probability of a correct first answer to
	// max(P, min(Cap, P+Shift)) when Shift > 0 (same uniform UCorr).
	Shift, Cap float64
	// ForceChanges, when >= 0, fixes the number of answer changes instead of
	// drawing the change decisions (delays and targets still use the draws).
	ForceChanges int
}

// NoOverride is the honest process.
var NoOverride = Override{ForceChanges: -1}

// Logistic returns 1 / (1 + exp(-x)).
func Logistic(x float64) float64 { return 1 / (1 + math.Exp(-x)) }

// SolutionTime returns T = max(min, round(exp(beta - tau + sigma*eps))).
func SolutionTime(m *params.Model, r Responder, it Item, eps float64) int64 {
	return rng.LogNormalMs(it.Beta-r.Tau, it.Sigma, eps, m.MinResponseMs)
}

// PickWeighted chooses a wrong option by distractor weight, excluding the
// key and (if >= 0) the option `exclude`. If the remaining weights are all
// zero it chooses uniformly among the remaining options.
func PickWeighted(it Item, exclude int, u float64) int {
	var cand [NumOptions]int
	var w [NumOptions]float64
	n := 0
	var total float64
	for o := 0; o < NumOptions; o++ {
		if o == it.Key || o == exclude {
			continue
		}
		cand[n], w[n] = o, it.W[o]
		total += it.W[o]
		n++
	}
	if total <= 0 {
		i := int(u * float64(n))
		if i >= n {
			i = n - 1
		}
		return cand[i]
	}
	x := u * total
	for i := 0; i < n; i++ {
		if x < w[i] {
			return cand[i]
		}
		x -= w[i]
	}
	// Floating-point remainder: last option with positive weight.
	for i := n - 1; i >= 0; i-- {
		if w[i] > 0 {
			return cand[i]
		}
	}
	return cand[n-1]
}

// Respond realises one honest item response.
func Respond(m *params.Model, r Responder, it Item, d *Draws, ov Override) ItemResult {
	res := ItemResult{Time: SolutionTime(m, r, it, d.Eps)}
	res.Gap = rng.LogNormalMs(m.Gap.LogMsMean, m.Gap.LogSD, d.GapEps, 1)
	var first int
	var t int64
	if d.URapid < r.Disengage {
		res.Rapid = true
		res.P = 1.0 / NumOptions
		res.P = shifted(res.P, ov)
		if d.UCorr < res.P {
			first = it.Key
		} else {
			wrong := wrongOptions(it.Key)
			i := int(d.UOpt * float64(len(wrong)))
			if i >= len(wrong) {
				i = len(wrong) - 1
			}
			first = wrong[i]
		}
		t = rng.LogNormalMs(m.RapidGuess.LogMsMean, m.RapidGuess.LogSD, d.ERapid, 1)
		res.Events = []Event{{SinceShown: t, Option: first}}
		res.Dwell = t
		return res
	}
	L := Logistic(it.A * (r.Ability - it.B))
	res.Knows = d.UCorr < L
	res.P = shifted(it.C+(1-it.C)*L, ov)
	if d.UCorr < res.P {
		first = it.Key
	} else {
		first = PickWeighted(it, -1, d.UOpt)
	}
	t = res.Time
	res.Events = []Event{{SinceShown: t, Option: first}}
	res.Events = append(res.Events, revisions(m, r, it, d, first, t, ov.ForceChanges)...)
	res.Dwell = res.Events[len(res.Events)-1].SinceShown
	return res
}

func shifted(p float64, ov Override) float64 {
	if ov.Shift <= 0 {
		return p
	}
	return math.Max(p, math.Min(ov.Cap, p+ov.Shift))
}

// revisions runs the answer-change process after a first answer at time t.
func revisions(m *params.Model, r Responder, it Item, d *Draws, cur int, t int64, force int) []Event {
	var out []Event
	rv := m.Revision
	for k := 0; k < MaxChanges; k++ {
		var p float64
		switch {
		case k > 0:
			p = rv.SecondChangeProb
		case cur != it.Key:
			p = math.Min(0.95, r.Revise*rv.ChangeMultWrong)
		default:
			p = math.Min(0.95, r.Revise*rv.ChangeMultRight)
		}
		rd := d.Rev[k]
		change := rd.U < p
		if force >= 0 {
			change = k < force
		}
		if !change {
			break
		}
		t += rng.LogNormalMs(rv.DelayLogMsMean, rv.DelayLogSD, rd.Eps, 1)
		if cur == it.Key {
			cur = PickWeighted(it, -1, rd.UOpt)
		} else if rd.UTarget < rv.WrongToRight {
			cur = it.Key
		} else {
			cur = PickWeighted(it, cur, rd.UOpt)
		}
		out = append(out, Event{SinceShown: t, Option: cur, IsChange: true})
	}
	return out
}

// Attempt is one realised quiz attempt.
type Attempt struct {
	Student int
	Index   int   // zero-based attempt index
	Base    int64 // Unix ms of start-at + Index days
	Offset  int64 // start offset (ms) relative to Base
	Items   []Item
	Results []ItemResult
	Draws   []Draws
	Review  int64 // pause between leaving the last item and submitting
	// AutoSubmit, when > 0, is the submission time (ms since start) imposed
	// by the quiz window (see Close); answers at or after the close have been
	// removed, so trailing items may be unanswered. 0 means the attempt was
	// submitted naturally.
	AutoSubmit int64
}

// Start returns the Unix ms of the attempt start.
func (a *Attempt) Start() int64 { return a.Base + a.Offset }

// Shown returns, for each item, ms since attempt start when it was shown.
func (a *Attempt) Shown() []int64 {
	shown := make([]int64, len(a.Results))
	var t int64
	for q, r := range a.Results {
		shown[q] = t
		t += r.Dwell + r.Gap
	}
	return shown
}

// Duration returns ms from attempt start to submission.
func (a *Attempt) Duration() int64 {
	if a.AutoSubmit > 0 {
		return a.AutoSubmit
	}
	return a.NaturalDuration()
}

// NaturalDuration returns ms from attempt start to the submission the
// student's own timeline implies, ignoring any quiz window.
func (a *Attempt) NaturalDuration() int64 {
	n := len(a.Results)
	shown := a.Shown()
	return shown[n-1] + a.Results[n-1].Dwell + a.Review
}

// Answered returns the number of items with at least one saved answer.
func (a *Attempt) Answered() int {
	n := 0
	for _, r := range a.Results {
		if r.Answered() {
			n++
		}
	}
	return n
}

// Score returns the number of items whose final answer is the key
// (unanswered items score zero).
func (a *Attempt) Score() int {
	s := 0
	for q, r := range a.Results {
		if r.Final() == a.Items[q].Key {
			s++
		}
	}
	return s
}

// Changes returns the number of change events.
func (a *Attempt) Changes() int {
	c := 0
	for _, r := range a.Results {
		c += r.Changes()
	}
	return c
}

// Clone deep-copies the attempt (items and draws are shared read-only).
func (a *Attempt) Clone() *Attempt {
	c := *a
	c.Results = make([]ItemResult, len(a.Results))
	for i, r := range a.Results {
		c.Results[i] = r.Clone()
	}
	return &c
}

// Check verifies the internal timing invariants of an attempt.
func (a *Attempt) Check() error {
	if len(a.Results) == 0 || len(a.Results) != len(a.Items) {
		return fmt.Errorf("attempt %d/%d: %d results for %d items", a.Student, a.Index, len(a.Results), len(a.Items))
	}
	dur := a.Duration()
	shown := a.Shown()
	if a.AutoSubmit > 0 && a.Answered() == 0 {
		return fmt.Errorf("attempt %d/%d: auto-submitted without any answer", a.Student, a.Index)
	}
	for q, r := range a.Results {
		if len(r.Events) == 0 && a.AutoSubmit > 0 {
			continue // left unanswered when the quiz closed
		}
		if len(r.Events) == 0 || r.Events[0].IsChange {
			return fmt.Errorf("attempt %d/%d item %d: missing first answer", a.Student, a.Index, q)
		}
		if r.Dwell < 0 || r.Gap < 0 {
			return fmt.Errorf("attempt %d/%d item %d: negative dwell or gap", a.Student, a.Index, q)
		}
		prev := int64(0)
		for j, e := range r.Events {
			if j > 0 && (!e.IsChange || e.SinceShown <= prev) {
				return fmt.Errorf("attempt %d/%d item %d: events not strictly increasing changes", a.Student, a.Index, q)
			}
			if e.Option < 0 || e.Option >= NumOptions {
				return fmt.Errorf("attempt %d/%d item %d: option out of range", a.Student, a.Index, q)
			}
			if j > 0 && e.Option == r.Events[j-1].Option {
				return fmt.Errorf("attempt %d/%d item %d: change to the same option", a.Student, a.Index, q)
			}
			if e.SinceShown < 0 || shown[q]+e.SinceShown > dur || (a.AutoSubmit > 0 && shown[q]+e.SinceShown >= dur) {
				return fmt.Errorf("attempt %d/%d item %d: event outside attempt", a.Student, a.Index, q)
			}
			prev = e.SinceShown
		}
	}
	if a.Review < 0 || dur <= 0 {
		return fmt.Errorf("attempt %d/%d: invalid review or duration", a.Student, a.Index)
	}
	return nil
}

// AttemptDraws holds attempt-level variates.
type AttemptDraws struct {
	Offset    int64
	ReviewEps float64
}

// DrawAttempt draws the start offset and review pause normal.
func DrawAttempt(seed int64, student, attempt int) AttemptDraws {
	s := rng.New(seed, "attempt", student, attempt)
	return AttemptDraws{Offset: s.IntRange(0, OffsetMaxMs), ReviewEps: s.Norm()}
}

// Simulate realises an attempt for responder r on items with the honest
// process (optionally overridden per item).
func Simulate(m *params.Model, seed int64, student, k int, base int64, r Responder, items []Item, ov func(q int) Override) *Attempt {
	ad := DrawAttempt(seed, student, k)
	a := &Attempt{
		Student: student, Index: k, Base: base, Offset: ad.Offset,
		Items:   items,
		Results: make([]ItemResult, len(items)),
		Draws:   make([]Draws, len(items)),
		Review:  rng.LogNormalMs(m.Review.LogMsMean, m.Review.LogSD, ad.ReviewEps, 1),
	}
	for q := range items {
		a.Draws[q] = DrawItem(seed, student, k, q)
		o := NoOverride
		if ov != nil {
			o = ov(q)
		}
		a.Results[q] = Respond(m, r, items[q], &a.Draws[q], o)
	}
	return a
}

// Resimulate re-runs an attempt with the same draws but a different
// responder and/or override. Offset and review are kept.
func Resimulate(m *params.Model, a *Attempt, r Responder, ov func(q int) Override) *Attempt {
	c := *a
	c.Results = make([]ItemResult, len(a.Items))
	for q := range a.Items {
		o := NoOverride
		if ov != nil {
			o = ov(q)
		}
		c.Results[q] = Respond(m, r, a.Items[q], &a.Draws[q], o)
	}
	return &c
}
