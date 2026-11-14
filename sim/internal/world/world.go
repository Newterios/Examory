// Package world builds the three paired worlds (honest, fi, bl) from one set
// of honest attempts.
package world

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/Newterios/Examory/sim/internal/mechanism"
	"github.com/Newterios/Examory/sim/internal/model"
	"github.com/Newterios/Examory/sim/internal/params"
	"github.com/Newterios/Examory/sim/internal/rng"
)

// World names in canonical order.
const (
	Honest = "honest"
	FI     = "fi"
	BL     = "bl"
)

// AllWorlds lists the worlds in canonical order.
var AllWorlds = []string{Honest, FI, BL}

// Item sampling designs.
const (
	SamplingFixed      = "fixed"
	SamplingPerAttempt = "per-attempt"
)

// Config is the full specification of one simulation.
type Config struct {
	Students     int
	Attempts     int
	Questions    int
	AnomalyRate  float64
	Weights      [4]float64 // in Slots order
	Seed         int64
	StartAt      time.Time // truncated to milliseconds, UTC
	Model        *params.Model
	Mech         *params.Mechanisms
	ItemSampling string
	Intensity    *float64 // overrides mechanism coverage/propensity when set
	Worlds       []string
}

// Record is one attempt in one world with its ground truth.
type Record struct {
	World         string
	Attempt       *model.Attempt
	Person        model.Person
	Slot          string // the student's cheater slot ("none" for honest students)
	AnomalyType   string // value written to the anomaly_type column
	IsAnomaly     bool
	IsFinal       bool
	Role          string
	Partner       int // -1 when none
	Intensity     float64
	ItemsAffected int
	Flag          string
	ProxyTheta    float64
	SyncSubmit    int // 1/0 for copy_peer pairs, -1 otherwise
}

// Result holds every world.
type Result struct {
	Labels  *Labels
	Persons []model.Person
	Worlds  map[string][]*Record // records ordered by (attempt index, student)
}

// Validate checks a configuration independently of the CLI.
func (c *Config) Validate() error {
	switch {
	case c.Students < 1 || c.Students > 1_000_000:
		return errors.New("students must lie in [1, 1000000]")
	case c.Attempts < 1 || c.Attempts > 365:
		return errors.New("attempts must lie in [1, 365]")
	case c.Questions < 1 || c.Questions > 1000:
		return errors.New("questions must lie in [1, 1000]")
	case math.IsNaN(c.AnomalyRate) || c.AnomalyRate < 0 || c.AnomalyRate > 1/float64(c.Attempts):
		return errors.New("anomaly-rate must lie in [0, 1/attempts]")
	case c.Model == nil || c.Mech == nil:
		return errors.New("parameters missing")
	case c.ItemSampling != SamplingFixed && c.ItemSampling != SamplingPerAttempt:
		return fmt.Errorf("item-sampling must be %q or %q", SamplingFixed, SamplingPerAttempt)
	case c.Intensity != nil && (math.IsNaN(*c.Intensity) || *c.Intensity < 0 || *c.Intensity > 1):
		return errors.New("intensity must lie in [0, 1]")
	case len(c.Worlds) == 0:
		return errors.New("at least one world is required")
	}
	var total float64
	for _, w := range c.Weights {
		if math.IsNaN(w) || math.IsInf(w, 0) || w < 0 {
			return errors.New("anomaly weights must be finite and non-negative")
		}
		total += w
	}
	if total <= 0 {
		return errors.New("at least one anomaly weight must be positive")
	}
	if c.Attempts < 2 && c.AnomalyRate > 0 && c.Weights[3] > 0 {
		return errors.New("score_jump requires at least two attempts; set its weight to zero")
	}
	if n := len(c.Model.Items); n > 0 && n < c.Questions {
		return fmt.Errorf("item pool has %d items but %d questions were requested", n, c.Questions)
	}
	for _, w := range c.Worlds {
		if w != Honest && w != FI && w != BL {
			return fmt.Errorf("unknown world %q", w)
		}
	}
	return nil
}

// StudentID returns the legacy user id.
func StudentID(i int) string { return fmt.Sprintf("student_%05d", i+1) }

// GroupID returns the legacy cohort id.
func GroupID(i int) string { return fmt.Sprintf("group_%03d", i/CohortSize+1) }

type sim struct {
	cfg     *Config
	m       *params.Model
	mech    *params.Mechanisms
	fixed   []model.Item
	persons []model.Person
	honest  [][]*model.Attempt // [attempt][student]
}

func (s *sim) base(k int) int64 {
	return s.cfg.StartAt.UnixMilli() + int64(k)*86400000
}

func (s *sim) poolSample(st *rng.Stream, n int) []model.Item {
	pool := s.m.Items
	// Partial Fisher-Yates on a sparse map: O(n) for any pool size.
	swapped := make(map[int]int, n)
	at := func(i int) int {
		if v, ok := swapped[i]; ok {
			return v
		}
		return i
	}
	items := make([]model.Item, n)
	N := len(pool)
	for i := 0; i < n; i++ {
		j := i + st.IntN(N-i)
		vi, vj := at(i), at(j)
		swapped[i], swapped[j] = vj, vi
		items[i] = model.FromPool(s.cfg.Seed, pool[vj])
	}
	return items
}

func (s *sim) items(student, k int) []model.Item {
	if s.cfg.ItemSampling == SamplingFixed {
		return s.fixed
	}
	n := s.cfg.Questions
	if len(s.m.Items) > 0 {
		return s.poolSample(rng.New(s.cfg.Seed, "attempt-items", student, k), n)
	}
	items := make([]model.Item, n)
	for q := range items {
		id := fmt.Sprintf("gen_s%05d_a%02d_q%03d", student+1, k+1, q+1)
		items[q] = model.PriorItem(rng.New(s.cfg.Seed, "attempt-item", student, k, q), id, s.m.ItemPrior)
	}
	return items
}

func (s *sim) intensity(fileValue float64) float64 {
	if s.cfg.Intensity != nil {
		return *s.cfg.Intensity
	}
	return fileValue
}

// Generate runs the simulation.
func Generate(cfg *Config) (*Result, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if err := cfg.Model.Validate(); err != nil {
		return nil, err
	}
	if err := cfg.Mech.Validate(); err != nil {
		return nil, err
	}
	s := &sim{cfg: cfg, m: cfg.Model, mech: cfg.Mech}
	if cfg.ItemSampling == SamplingFixed {
		if len(s.m.Items) > 0 {
			s.fixed = s.poolSample(rng.New(cfg.Seed, "quiz"), cfg.Questions)
		} else {
			s.fixed = make([]model.Item, cfg.Questions)
			for q := range s.fixed {
				s.fixed[q] = model.PriorItem(rng.New(cfg.Seed, "item", q), fmt.Sprintf("question_%03d", q+1), s.m.ItemPrior)
			}
		}
	}
	labels, err := AssignLabels(cfg.Seed, cfg.Students, cfg.Attempts, cfg.AnomalyRate, cfg.Weights)
	if err != nil {
		return nil, err
	}
	s.persons = make([]model.Person, cfg.Students)
	for i := range s.persons {
		s.persons[i] = model.DrawPerson(cfg.Seed, i, s.m.Person)
	}
	s.honest = make([][]*model.Attempt, cfg.Attempts)
	for k := 0; k < cfg.Attempts; k++ {
		s.honest[k] = make([]*model.Attempt, cfg.Students)
		for i := 0; i < cfg.Students; i++ {
			s.honest[k][i] = model.Simulate(s.m, cfg.Seed, i, k, s.base(k), model.ResponderOf(s.persons[i], k), s.items(i, k), nil)
		}
	}
	final := cfg.Attempts - 1
	// Base attempts of cheaters' final attempts. Collusion pairs answer the
	// same items: in per-attempt mode both members are re-simulated on the
	// item set of the lower-index member (identical to honest in fixed mode).
	baseOf := func(i int) *model.Attempt {
		h := s.honest[final][i]
		p := labels.Partner[i]
		if p < 0 || p > i || cfg.ItemSampling == SamplingFixed {
			return h
		}
		lo := s.honest[final][p]
		return model.Simulate(s.m, cfg.Seed, i, final, s.base(final), model.ResponderOf(s.persons[i], final), lo.Items, nil)
	}

	res := &Result{Labels: labels, Persons: s.persons, Worlds: map[string][]*Record{}}
	for _, w := range cfg.Worlds {
		overrides := map[int]*Record{}
		switch w {
		case FI:
			if err := s.buildFI(labels, baseOf, overrides); err != nil {
				return nil, err
			}
		case BL:
			if err := s.buildBL(labels, baseOf, overrides); err != nil {
				return nil, err
			}
		}
		var recs []*Record
		for k := 0; k < cfg.Attempts; k++ {
			for i := 0; i < cfg.Students; i++ {
				slot := labels.Slot[i]
				if slot == "" {
					slot = "none"
				}
				if k == final {
					if r, ok := overrides[i]; ok {
						r.World, r.Person, r.Slot, r.IsFinal = w, s.persons[i], slot, true
						if err := r.Attempt.Check(); err != nil {
							return nil, fmt.Errorf("world %s: %w", w, err)
						}
						recs = append(recs, r)
						continue
					}
				}
				recs = append(recs, &Record{
					World: w, Attempt: s.honest[k][i], Person: s.persons[i], Slot: slot,
					AnomalyType: "none", IsFinal: k == final, Partner: -1,
					Intensity: math.NaN(), ProxyTheta: math.NaN(), SyncSubmit: -1,
				})
			}
		}
		res.Worlds[w] = recs
	}
	return res, nil
}

func rec(a *model.Attempt, kind string, info mechanism.Info, partner int) *Record {
	return &Record{
		Attempt: a, AnomalyType: kind, IsAnomaly: true, Role: info.Role, Partner: partner,
		Intensity: info.Intensity, ItemsAffected: info.ItemsAffected, Flag: info.Flag, ProxyTheta: info.ProxyTheta,
		SyncSubmit: info.SyncSubmit,
	}
}

func (s *sim) buildFI(l *Labels, baseOf func(int) *model.Attempt, out map[int]*Record) error {
	fi := s.mech.FI
	final := s.cfg.Attempts - 1
	for i, slot := range l.Slot {
		switch slot {
		case "rapid_response":
			a, info := mechanism.FIRapid(fi, s.cfg.Seed, baseOf(i))
			out[i] = rec(a, slot, info, -1)
		case "excessive_changes":
			a, info := mechanism.FIChanges(fi, s.cfg.Seed, baseOf(i))
			out[i] = rec(a, slot, info, -1)
		case "score_jump":
			a, info := mechanism.FIScoreJump(s.m, fi, s.cfg.Seed, baseOf(i), model.ResponderOf(s.persons[i], final))
			out[i] = rec(a, slot, info, -1)
		case "collusion":
			p := l.Partner[i]
			if p < i {
				continue
			}
			la, ma, li, mi := mechanism.FICollusion(fi, s.cfg.Seed, baseOf(i), baseOf(p))
			out[i] = rec(la, slot, li, p)
			out[p] = rec(ma, slot, mi, i)
		}
	}
	return nil
}

func (s *sim) buildBL(l *Labels, baseOf func(int) *model.Attempt, out map[int]*Record) error {
	mc := s.mech
	final := s.cfg.Attempts - 1
	for i, slot := range l.Slot {
		kind := BLMechanism[slot]
		switch slot {
		case "rapid_response":
			a, info := mechanism.Preknowledge(s.m, mc.Preknowledge, s.cfg.Seed, baseOf(i), s.intensity(mc.Preknowledge.Coverage))
			out[i] = rec(a, kind, info, -1)
		case "excessive_changes":
			a, info := mechanism.Lookup(s.m, mc.Lookup, s.cfg.Seed, baseOf(i), s.intensity(mc.Lookup.Propensity))
			out[i] = rec(a, kind, info, -1)
		case "score_jump":
			a, info := mechanism.Proxy(s.m, mc.Proxy, s.cfg.Seed, baseOf(i), s.persons[i], s.intensity(1))
			out[i] = rec(a, kind, info, -1)
		case "collusion":
			p := l.Partner[i]
			if p < i {
				continue
			}
			// i is the lower index. The stronger member is the source with
			// probability stronger_is_source_prob.
			u := rng.New(s.cfg.Seed, "copy-pair", i, final).Float64()
			strong, weak := i, p
			if s.persons[p].Ability(final) > s.persons[i].Ability(final) {
				strong, weak = p, i
			}
			src, cpy := strong, weak
			if u >= mc.CopyPeer.StrongerIsSourceProb {
				src, cpy = weak, strong
			}
			sa, ca, si, ci := mechanism.CopyPeer(s.m, mc.CopyPeer, s.cfg.Seed, baseOf(src), baseOf(cpy), s.intensity(mc.CopyPeer.Coverage), i)
			out[src] = rec(sa, kind, si, cpy)
			out[cpy] = rec(ca, kind, ci, src)
		}
	}
	return nil
}
