// Package mechanism implements the cheating mechanisms of the behaviour-level
// (bl) world and the legacy feature-aligned injection recipes of the fi
// world. Every function takes the honest attempt (with its draws) and returns
// a new attempt; inputs are never mutated.
package mechanism

import (
	"math"

	"github.com/Newterios/Examory/sim/internal/model"
	"github.com/Newterios/Examory/sim/internal/params"
	"github.com/Newterios/Examory/sim/internal/rng"
)

// Info describes what a mechanism or recipe did to an attempt.
type Info struct {
	Role          string  // source/copier (copy_peer), leader/member (collusion)
	Intensity     float64 // coverage/propensity used (NaN when not applicable)
	ItemsAffected int
	Flag          string  // naive/disguising, verify/lookup_then_answer, ...
	ProxyTheta    float64 // NaN unless proxy
	// SyncSubmit is 1 if the copy_peer pair is sync-submitting, 0 if not,
	// -1 when not applicable.
	SyncSubmit int
	// SyncOffsetMs is the drawn signed offset (earlier finisher minus later
	// finisher) for a sync pair; SyncClamped reports that the target would
	// have shortened the earlier finisher's review pause, so it was kept.
	SyncOffsetMs int64
	SyncClamped  bool
}

func newInfo() Info { return Info{Intensity: math.NaN(), ProxyTheta: math.NaN(), SyncSubmit: -1} }

// Preknowledge simulates answering with leaked items.
func Preknowledge(m *params.Model, pk params.Preknowledge, seed int64, h *model.Attempt, coverage float64) (*model.Attempt, Info) {
	a := h.Clone()
	info := newInfo()
	info.Intensity = coverage
	naive := rng.New(seed, "preknowledge-person", h.Student).Float64() < pk.NaiveProb
	info.Flag = "disguising"
	if naive {
		info.Flag = "naive"
	}
	for q := range a.Items {
		s := rng.New(seed, "preknowledge-item", h.Student, h.Index, q)
		uComp, uRecall, uSpeed := s.Float64(), s.Float64(), s.Float64()
		if uComp >= coverage || uRecall >= pk.Recall {
			continue
		}
		r := &a.Results[q]
		t := r.Time
		if naive {
			f := pk.SpeedupLow + (pk.SpeedupHigh-pk.SpeedupLow)*uSpeed
			t = int64(math.Round(float64(r.Time) * f))
			if t < m.MinResponseMs {
				t = m.MinResponseMs
			}
		}
		r.Events = []model.Event{{SinceShown: t, Option: a.Items[q].Key}}
		r.Dwell = t
		r.Rapid = false
		info.ItemsAffected++
	}
	return a, info
}

// Lookup simulates consulting an external resource on unknown items.
func Lookup(m *params.Model, lk params.Lookup, seed int64, h *model.Attempt, propensity float64) (*model.Attempt, Info) {
	a := h.Clone()
	info := newInfo()
	info.Intensity = propensity
	verify := rng.New(seed, "lookup-person", h.Student).Float64() < lk.VerifyModeProb
	info.Flag = "lookup_then_answer"
	if verify {
		info.Flag = "verify"
	}
	for q, it := range a.Items {
		s := rng.New(seed, "lookup-item", h.Student, h.Index, q)
		uLook, eps, uSuccess, uOpt := s.Float64(), s.Norm(), s.Float64(), s.Float64()
		r := &a.Results[q]
		if r.Knows || r.Rapid || uLook >= propensity {
			continue
		}
		dur := rng.LogNormalMs(lk.DurationLogMsMean, lk.DurationLogSD, eps, 1)
		looked := it.Key
		if uSuccess >= lk.Success {
			looked = model.PickWeighted(it, -1, uOpt)
		}
		first := r.Events[0]
		if verify {
			r.Events = []model.Event{first}
			if looked != first.Option {
				r.Events = append(r.Events, model.Event{SinceShown: first.SinceShown + dur, Option: looked, IsChange: true})
			}
		} else {
			r.Events = []model.Event{{SinceShown: first.SinceShown + dur, Option: looked}}
		}
		r.Dwell = first.SinceShown + dur
		info.ItemsAffected++
	}
	return a, info
}

// Proxy simulates the final attempt being taken by a stronger person.
// gainScale (the intensity) scales the ability gain theta' - theta_i(k).
func Proxy(m *params.Model, px params.Proxy, seed int64, h *model.Attempt, own model.Person, gainScale float64) (*model.Attempt, Info) {
	info := newInfo()
	info.Intensity = gainScale
	s := rng.New(seed, "proxy", h.Student)
	u := 1 - s.Float64() // (0, 1]
	z2 := s.Norm()
	rRevise := s.Beta(m.Person.Revise.BetaA, m.Person.Revise.BetaB)
	base := own.Ability(h.Index)
	thetaP := rng.TruncNormalLower(px.ThetaMean, px.ThetaSD, base+px.MinGain, u)
	ability := base + gainScale*(thetaP-base)
	// Speed drawn from the population distribution conditional on ability.
	zt := 0.0
	if m.Person.ThetaSD > 0 {
		zt = ability / m.Person.ThetaSD
	}
	rho := m.Person.ThetaTauCorr
	tau := m.Person.TauSD * (rho*zt + math.Sqrt(1-rho*rho)*z2)
	r := model.Responder{Ability: ability, Tau: tau, Disengage: 0, Revise: rRevise}
	a := model.Resimulate(m, h, r, nil)
	info.ProxyTheta = ability
	info.ItemsAffected = len(a.Items)
	info.Flag = "proxy"
	return a, info
}

// CopyPeer simulates a copier copying answers from a source in the same
// cohort. src and cpy are the honest attempts of the two members on the
// same items. It returns the modified source and copier attempts.
func CopyPeer(m *params.Model, cp params.CopyPeer, seed int64, src, cpy *model.Attempt, coverage float64, pairKey int) (*model.Attempt, *model.Attempt, Info, Info) {
	n := len(src.Items)
	si, ci := newInfo(), newInfo()
	si.Role, ci.Role = "source", "copier"
	si.Intensity, ci.Intensity = coverage, coverage
	si.Flag, ci.Flag = "source", "copier"

	type itemDraw struct {
		uCopy, relayEps, entryEps, overheadEps float64
		follow                                 [model.MaxChanges + 1][2]float64
	}
	draws := make([]itemDraw, n)
	copyQ := make([]bool, n)
	for q := 0; q < n; q++ {
		s := rng.New(seed, "copy-item", cpy.Student, cpy.Index, q)
		d := itemDraw{uCopy: s.Float64(), relayEps: s.Norm(), entryEps: s.Norm(), overheadEps: s.Norm()}
		for j := range d.follow {
			d.follow[j] = [2]float64{s.Float64(), s.Norm()}
		}
		draws[q] = d
		copyQ[q] = d.uCopy < coverage
	}
	lag := rng.LogNormalMs(cp.StartLagLogMsMean, cp.StartLagLogSD, rng.New(seed, "copy-lag", pairKey, cpy.Index).Norm(), 1)

	// Source: honest, plus communication overhead after each copied item.
	sa := src.Clone()
	for q := 0; q < n; q++ {
		if !copyQ[q] {
			continue
		}
		oh := rng.LogNormalMs(cp.SourceOverheadLogMsMean, cp.SourceOverheadLogSD, draws[q].overheadEps, 1)
		if q < n-1 {
			sa.Results[q].Gap += oh
		} else {
			sa.Review += oh
		}
		si.ItemsAffected++
	}
	srcStart := sa.Start()
	srcShown := sa.Shown()

	ca := cpy.Clone()
	ca.Offset = src.Offset + lag
	ca.Base = src.Base
	cpyStart := ca.Start()

	type pending struct {
		q   int
		t   int64 // ms since copier attempt start
		opt int
	}
	var follows []pending
	shown := int64(0)
	for q := 0; q < n; q++ {
		r := &ca.Results[q]
		if copyQ[q] {
			ev := sa.Results[q].Events
			relay := rng.LogNormalMs(cp.RelayLogMsMean, cp.RelayLogSD, draws[q].relayEps, 1)
			entry := rng.LogNormalMs(cp.EntryLogMsMean, cp.EntryLogSD, draws[q].entryEps, 1)
			srcFirstWall := srcStart + srcShown[q] + ev[0].SinceShown
			avail := srcFirstWall + relay - cpyStart
			at := shown + entry
			if avail > at {
				at = avail
			}
			// Source state as known at the copier's answer time.
			cutoff := cpyStart + at - relay
			j := 0
			for j+1 < len(ev) && srcStart+srcShown[q]+ev[j+1].SinceShown <= cutoff {
				j++
			}
			r.Events = []model.Event{{SinceShown: at - shown, Option: ev[j].Option}}
			r.Dwell = at - shown
			r.Rapid, r.Knows = false, false
			cur, last := ev[j].Option, at
			for k := j + 1; k < len(ev) && k < len(draws[q].follow); k++ {
				fd := draws[q].follow[k]
				if fd[0] >= cp.FollowChangeProb {
					continue
				}
				d := rng.LogNormalMs(cp.RelayLogMsMean, cp.RelayLogSD, fd[1], 1)
				t := srcStart + srcShown[q] + ev[k].SinceShown + d - cpyStart
				if t <= last {
					t = last + 1
				}
				if ev[k].Option == cur {
					continue
				}
				follows = append(follows, pending{q: q, t: t, opt: ev[k].Option})
				cur, last = ev[k].Option, t
			}
			ci.ItemsAffected++
		}
		shown += r.Dwell + r.Gap
	}
	syncSubmit(cp, seed, sa, ca, &si, &ci, pairKey)
	dur := ca.Duration()
	shownC := ca.Shown()
	// Keep follow-up changes that land before submission, preserving the
	// alternation of options (a change must differ from the current answer).
	for _, f := range follows {
		if f.t >= dur {
			continue
		}
		r := &ca.Results[f.q]
		if r.Final() == f.opt {
			continue
		}
		r.Events = append(r.Events, model.Event{SinceShown: f.t - shownC[f.q], Option: f.opt, IsChange: true})
	}
	return sa, ca, si, ci
}

// syncSubmit decides, with one pair-level Bernoulli from its own stream,
// whether the pair finishes together. If so, the member who would submit
// first lengthens its review pause so that it submits at (later submit ± d),
// d ~ LogNormal(sync_submit_log_ms_mean, sync_submit_log_sd), sign by a fair
// coin. A review pause is never shortened: if the target lies before the
// earlier finisher's own submission, that submission is kept. All three
// variates are always drawn, so the decision cannot shift other draws.
func syncSubmit(cp params.CopyPeer, seed int64, sa, ca *model.Attempt, si, ci *Info, pairKey int) {
	s := rng.New(seed, "copy-sync", pairKey, sa.Index)
	uSync, uSign, eps := s.Float64(), s.Float64(), s.Norm()
	si.SyncSubmit, ci.SyncSubmit = 0, 0
	if uSync >= cp.SyncSubmitProb {
		return
	}
	si.SyncSubmit, ci.SyncSubmit = 1, 1
	d := rng.LogNormalMs(cp.SyncSubmitLogMsMean, cp.SyncSubmitLogSD, eps, 1)
	if uSign < 0.5 {
		d = -d
	}
	si.SyncOffsetMs, ci.SyncOffsetMs = d, d
	endS, endC := sa.Start()+sa.Duration(), ca.Start()+ca.Duration()
	early, earlyEnd, lateEnd := sa, endS, endC
	if endC < endS {
		early, earlyEnd, lateEnd = ca, endC, endS
	}
	target := lateEnd + d
	if target < earlyEnd {
		si.SyncClamped, ci.SyncClamped = true, true
		return
	}
	early.Review += target - earlyEnd
}
