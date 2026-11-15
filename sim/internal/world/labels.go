package world

import (
	"fmt"
	"math"
	"sort"

	"github.com/Newterios/Examory/sim/internal/rng"
)

// Slots are the cheater slot types, in legacy order.
var Slots = [4]string{"rapid_response", "excessive_changes", "collusion", "score_jump"}

// BLMechanism maps a slot to its behaviour-level mechanism.
var BLMechanism = map[string]string{
	"rapid_response":    "preknowledge",
	"excessive_changes": "lookup",
	"collusion":         "copy_peer",
	"score_jump":        "proxy",
}

// CohortSize is the number of consecutive students per cohort (legacy).
const CohortSize = 20

// Labels is the cheater assignment shared by the fi and bl worlds.
type Labels struct {
	Slot    []string // per student; "" for honest students
	Partner []int    // per student; -1 unless collusion
	Counts  map[string]int
}

// Quotas reproduces the legacy largest-remainder allocation of
// round(students*attempts*rate) anomalous attempts to slot types, with the
// collusion count made even.
func Quotas(students, attempts int, rate float64, weights [4]float64) (map[string]int, int) {
	requested := int(math.Floor(float64(students)*float64(attempts)*rate + 0.5))
	var total float64
	for _, w := range weights {
		total += w
	}
	quota := make([]float64, 4)
	counts := make([]int, 4)
	sum := 0
	for i := range Slots {
		quota[i] = float64(requested) * weights[i] / total
		counts[i] = int(quota[i])
		sum += counts[i]
	}
	order := []int{0, 1, 2, 3}
	sort.SliceStable(order, func(a, b int) bool {
		return quota[order[a]]-float64(counts[order[a]]) > quota[order[b]]-float64(counts[order[b]])
	})
	for _, i := range order[:requested-sum] {
		counts[i]++
	}
	const col = 2
	if counts[col]%2 == 1 {
		counts[col]--
		best := -1
		for i := range Slots {
			if i != col && weights[i] > 0 && (best < 0 || weights[i] > weights[best]) {
				best = i
			}
		}
		if best >= 0 {
			counts[best]++
		}
	}
	out := make(map[string]int, 4)
	for i, s := range Slots {
		out[s] = counts[i]
	}
	return out, requested
}

// AssignLabels mirrors legacy make_labels: collusion pairs are drawn within
// cohorts of 20 consecutive students, the remaining slot types from the
// remaining students. Only the final attempt of a cheater is anomalous.
func AssignLabels(seed int64, students, attempts int, rate float64, weights [4]float64) (*Labels, error) {
	counts, _ := Quotas(students, attempts, rate, weights)
	s := rng.New(seed, "labels")
	var candidates [][2]int
	for base := 0; base < students; base += CohortSize {
		end := base + CohortSize
		if end > students {
			end = students
		}
		members := make([]int, 0, end-base)
		for i := base; i < end; i++ {
			members = append(members, i)
		}
		s.Shuffle(len(members), func(i, j int) { members[i], members[j] = members[j], members[i] })
		for i := 0; i+1 < len(members); i += 2 {
			candidates = append(candidates, [2]int{members[i], members[i+1]})
		}
	}
	s.Shuffle(len(candidates), func(i, j int) { candidates[i], candidates[j] = candidates[j], candidates[i] })
	l := &Labels{Slot: make([]string, students), Partner: make([]int, students), Counts: counts}
	for i := range l.Partner {
		l.Partner[i] = -1
	}
	pairs := counts["collusion"] / 2
	if pairs > len(candidates) {
		return nil, fmt.Errorf("collusion needs %d within-cohort pairs but only %d exist", pairs, len(candidates))
	}
	for _, p := range candidates[:pairs] {
		l.Slot[p[0]], l.Slot[p[1]] = "collusion", "collusion"
		l.Partner[p[0]], l.Partner[p[1]] = p[1], p[0]
	}
	remaining := make([]int, 0, students)
	for i := 0; i < students; i++ {
		if l.Slot[i] == "" {
			remaining = append(remaining, i)
		}
	}
	s.Shuffle(len(remaining), func(i, j int) { remaining[i], remaining[j] = remaining[j], remaining[i] })
	for _, kind := range Slots {
		if kind == "collusion" {
			continue
		}
		for c := 0; c < counts[kind]; c++ {
			if len(remaining) == 0 {
				return nil, fmt.Errorf("not enough students for %d anomalous attempts", counts[kind])
			}
			last := remaining[len(remaining)-1]
			remaining = remaining[:len(remaining)-1]
			l.Slot[last] = kind
		}
	}
	return l, nil
}
