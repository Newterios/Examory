package model

import "fmt"

// Close applies a fixed quiz closing time to an attempt. closeAt is the
// closing instant (Unix ms) and jitter the delay of the server-side
// auto-submission after it.
//
// If the attempt's natural submission is not later than closeAt, a itself is
// returned. Otherwise Close returns a copy in which every answer event at or
// after closeAt is removed and which is submitted at closeAt + jitter. Events
// within an item are strictly increasing in time, so the kept events of an
// item are a prefix of its events (event indices, and hence event ids, are
// unchanged); items whose first answer falls after the close stay unanswered.
// Item display times, gaps and the review pause are kept, so the kept events
// occur at exactly their natural times. a is never mutated.
//
// An attempt that would start at or after the close, or that has no answer
// before it, cannot be represented in the event contract (a submission needs
// at least one answer) and is reported as an error.
func Close(a *Attempt, closeAt, jitter int64) (*Attempt, error) {
	start := a.Start()
	if start+a.NaturalDuration() <= closeAt {
		return a, nil
	}
	cut := closeAt - start // events at or after cut are dropped
	if cut <= 0 {
		return nil, fmt.Errorf("attempt starts %d ms after the quiz closes; increase close_after_ms", -cut)
	}
	c := a.Clone()
	shown := a.Shown()
	for q := range c.Results {
		r := &c.Results[q]
		n := 0
		for n < len(r.Events) && shown[q]+r.Events[n].SinceShown < cut {
			n++
		}
		r.Events = r.Events[:n]
	}
	if c.Answered() == 0 {
		return nil, fmt.Errorf("attempt has no answer before the quiz closes %d ms after its start; increase close_after_ms", cut)
	}
	c.AutoSubmit = cut + jitter
	return c, nil
}
