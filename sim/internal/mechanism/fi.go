package mechanism

import (
	"math"

	"github.com/Newterios/Examory/sim/internal/model"
	"github.com/Newterios/Examory/sim/internal/params"
	"github.com/Newterios/Examory/sim/internal/rng"
)

// FIRapid ports the legacy rapid_response recipe: on each item with
// probability rapid_item_prob all event times within the item are scaled by
// U(rapid_scale_low, rapid_scale_high) and the first answer is floored at
// rapid_min_ms.
func FIRapid(fi params.FI, seed int64, h *model.Attempt) (*model.Attempt, Info) {
	a := h.Clone()
	info := newInfo()
	info.Flag = "fi"
	for q := range a.Results {
		s := rng.New(seed, "fi-rapid", h.Student, h.Index, q)
		uSel, uScale := s.Float64(), s.Float64()
		if uSel >= fi.RapidItemProb {
			continue
		}
		f := fi.RapidScaleLow + (fi.RapidScaleHigh-fi.RapidScaleLow)*uScale
		r := &a.Results[q]
		prev := int64(0)
		for j := range r.Events {
			t := int64(math.Round(float64(r.Events[j].SinceShown) * f))
			if j == 0 && t < fi.RapidMinMs {
				t = fi.RapidMinMs
			}
			if j > 0 && t <= prev {
				t = prev + 1
			}
			r.Events[j].SinceShown = t
			prev = t
		}
		r.Dwell = prev
		info.ItemsAffected++
	}
	return a, info
}

// FIChanges ports the legacy excessive_changes recipe: per item
// p ~ U(changes_prob_low, changes_prob_high); with probability p the item's
// events are replaced by a random option sequence of n+1 answers,
// n ~ U{1..changes_max_count}, ending in the honest final answer, with
// consecutive options different and evenly spaced over the honest
// first-answer time: since_shown = T*(j+1) div (n+1).
func FIChanges(fi params.FI, seed int64, h *model.Attempt) (*model.Attempt, Info) {
	a := h.Clone()
	info := newInfo()
	info.Flag = "fi"
	for q := range a.Results {
		s := rng.New(seed, "fi-changes", h.Student, h.Index, q)
		p := fi.ChangesProbLow + (fi.ChangesProbHigh-fi.ChangesProbLow)*s.Float64()
		uSel := s.Float64()
		count := int(s.IntRange(1, int64(fi.ChangesMaxCount)))
		if uSel >= p {
			continue
		}
		r := &a.Results[q]
		seq := []int{r.Final()}
		for i := 0; i < count; i++ {
			prev := seq[len(seq)-1]
			o := s.IntN(model.NumOptions - 1)
			if o >= prev {
				o++
			}
			seq = append(seq, o)
		}
		// Reverse so the honest final answer is last.
		for i, j := 0, len(seq)-1; i < j; i, j = i+1, j-1 {
			seq[i], seq[j] = seq[j], seq[i]
		}
		T := r.Events[0].SinceShown
		L := int64(len(seq))
		events := make([]model.Event, len(seq))
		prev := int64(0)
		for j, o := range seq {
			t := T * int64(j+1) / L
			if t <= prev {
				t = prev + 1
			}
			events[j] = model.Event{SinceShown: t, Option: o, IsChange: j > 0}
			prev = t
		}
		r.Events = events
		r.Dwell = prev
		r.Rapid = false
		info.ItemsAffected++
	}
	return a, info
}

// FIScoreJump ports the legacy score_jump recipe: with
// delta ~ U(score_jump_low, score_jump_high) per attempt, each item's
// probability of a correct first answer becomes max(P, min(cap, P+delta)),
// using the same uniform as the honest draw. Timings are unchanged: the
// first-answer times, the number of answer changes and their delays are held
// at their honest values.
func FIScoreJump(m *params.Model, fi params.FI, seed int64, h *model.Attempt, r model.Responder) (*model.Attempt, Info) {
	info := newInfo()
	info.Flag = "fi"
	delta := rng.New(seed, "fi-scorejump", h.Student, h.Index).Uniform(fi.ScoreJumpLow, fi.ScoreJumpHigh)
	a := model.Resimulate(m, h, r, func(q int) model.Override {
		return model.Override{Shift: delta, Cap: fi.ScoreProbCap, ForceChanges: h.Results[q].Changes()}
	})
	for q := range a.Results {
		if (a.Results[q].Final() == a.Items[q].Key) != (h.Results[q].Final() == h.Items[q].Key) {
			info.ItemsAffected++
		}
	}
	info.Intensity = delta
	return a, info
}

// FICollusion ports the legacy collusion recipe for one pair. leader is the
// member with the lower student index; both attempts are on the same items.
// Returns the leader and member attempts.
func FICollusion(fi params.FI, seed int64, leader, member *model.Attempt) (*model.Attempt, *model.Attempt, Info, Info) {
	n := len(leader.Items)
	ps := rng.New(seed, "fi-collusion-pair", leader.Student, leader.Index)
	nForced := int(math.RoundToEven(float64(n) * fi.CollusionForcedWrongFraction))
	if fi.CollusionForcedWrongFraction > 0 && nForced < 1 {
		nForced = 1
	}
	if nForced > n {
		nForced = n
	}
	forced := make([]bool, n)
	for _, q := range ps.Perm(n)[:nForced] {
		forced[q] = true
	}
	shared := make([]int, n)
	for q := range shared {
		if forced[q] {
			shared[q] = leader.Items[q].Rarest()
		} else {
			shared[q] = leader.Results[q].Final()
		}
	}
	pairOffset := leader.Offset
	j := fi.CollusionDurationJitter
	var target int64
	for q, r := range leader.Results {
		target += int64(math.Ceil(float64(r.Dwell) * (1 + j)))
		if q < n-1 {
			target += r.Gap
		}
	}
	target += fi.CollusionStartWindowMs + leader.Review

	build := func(own *model.Attempt, role string) (*model.Attempt, Info) {
		info := newInfo()
		info.Role = role
		info.Flag = "fi"
		info.Intensity = fi.CollusionCopyProb
		s := rng.New(seed, "fi-collusion-member", own.Student, own.Index)
		a := own.Clone()
		a.Offset = pairOffset + s.IntRange(0, fi.CollusionStartWindowMs)
		submitJitter := s.IntRange(0, fi.CollusionSubmitWindowMs)
		for q := range a.Results {
			uCopy, uJit := s.Float64(), s.Float64()
			opt := own.Results[q].Final()
			if forced[q] || uCopy < fi.CollusionCopyProb {
				opt = shared[q]
				info.ItemsAffected++
			}
			f := (1 - j) + 2*j*uJit
			d := int64(math.Round(float64(leader.Results[q].Dwell) * f))
			if d < 1 {
				d = 1
			}
			a.Results[q] = model.ItemResult{
				Events: []model.Event{{SinceShown: d, Option: opt}},
				Dwell:  d, Gap: leader.Results[q].Gap,
				Knows: own.Results[q].Knows, P: own.Results[q].P, Time: own.Results[q].Time,
			}
		}
		a.Review = 1
		end := a.Duration() - 1 // time the last item is left
		dur := target - (a.Offset - pairOffset) + submitJitter
		if dur-end >= 1 {
			a.Review = dur - end
		}
		return a, info
	}
	la, li := build(leader, "leader")
	ma, mi := build(member, "member")
	return la, ma, li, mi
}
