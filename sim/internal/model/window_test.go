package model

import (
	"testing"

	"github.com/Newterios/Examory/sim/internal/rng"
)

func windowAttempts(t *testing.T, n int) []*Attempt {
	t.Helper()
	m := loadModel(t)
	items := make([]Item, 20)
	for q := range items {
		items[q] = PriorItem(rng.New(3, "item", q), "q", m.ItemPrior)
	}
	out := make([]*Attempt, n)
	for i := range out {
		p := DrawPerson(3, i, m.Person)
		out[i] = Simulate(m, 3, i, 0, 1_800_000_000_000, ResponderOf(p, 0), items, nil)
	}
	return out
}

func TestCloseKeepsNaturalSubmissions(t *testing.T) {
	for _, a := range windowAttempts(t, 50) {
		end := a.Start() + a.Duration()
		for _, closeAt := range []int64{end, end + 1, end + 1_000_000} {
			c, err := Close(a, closeAt, 1234)
			if err != nil || c != a {
				t.Fatalf("submission at %d with close %d: got %p (%v), want the attempt itself", end, closeAt, c, err)
			}
		}
	}
}

func TestCloseTruncatesAndRescores(t *testing.T) {
	truncated := 0
	for _, a := range windowAttempts(t, 200) {
		before := a.Clone()
		shown := a.Shown()
		// Close at a point inside the attempt, after its first answer.
		first := a.Start() + a.Results[0].Events[0].SinceShown
		end := a.Start() + a.Duration()
		closeAt := first + 1 + (end-first)*int64(a.Student%7+1)/9
		const jitter = 777
		c, err := Close(a, closeAt, jitter)
		if err != nil {
			t.Fatal(err)
		}
		if c == a || c.AutoSubmit != closeAt-a.Start()+jitter || c.Duration() != c.AutoSubmit {
			t.Fatalf("auto-submission at %d, want %d", c.AutoSubmit, closeAt-a.Start()+jitter)
		}
		if c.NaturalDuration() != a.Duration() {
			t.Fatal("natural duration changed")
		}
		if err := c.Check(); err != nil {
			t.Fatal(err)
		}
		if !equalAttempt(a, before) {
			t.Fatal("Close mutated its input")
		}
		score, changes, answered := 0, 0, 0
		for q, r := range c.Results {
			nat := a.Results[q].Events
			// Kept events are exactly the natural events before the close.
			n := 0
			for n < len(nat) && a.Start()+shown[q]+nat[n].SinceShown < closeAt {
				n++
			}
			if len(r.Events) != n {
				t.Fatalf("item %d: kept %d events, want %d", q, len(r.Events), n)
			}
			for j := range r.Events {
				if r.Events[j] != nat[j] {
					t.Fatalf("item %d event %d differs from the natural event", q, j)
				}
			}
			if r.Dwell != a.Results[q].Dwell || r.Gap != a.Results[q].Gap {
				t.Fatal("item timeline changed")
			}
			if n == 0 {
				if r.Final() != -1 || r.Changes() != 0 || r.Answered() {
					t.Fatal("unanswered item must have no final option and no changes")
				}
				// Answers are given in item order, so no later item is answered.
				for _, later := range c.Results[q:] {
					if later.Answered() {
						t.Fatal("answered items are not a prefix")
					}
				}
				continue
			}
			answered++
			changes += n - 1
			if nat[n-1].Option == c.Items[q].Key {
				score++
			}
		}
		if c.Score() != score || c.Changes() != changes || c.Answered() != answered {
			t.Fatalf("score/changes/answered %d/%d/%d, want %d/%d/%d", c.Score(), c.Changes(), c.Answered(), score, changes, answered)
		}
		if answered < len(c.Items) {
			truncated++
		}
	}
	if truncated == 0 {
		t.Fatal("no attempt lost an item; the test does not exercise truncation")
	}
}

func TestCloseRejectsUnrepresentableAttempts(t *testing.T) {
	a := windowAttempts(t, 1)[0]
	if _, err := Close(a, a.Start(), 0); err == nil {
		t.Fatal("an attempt starting at the close must be rejected")
	}
	if _, err := Close(a, a.Start()+a.Results[0].Events[0].SinceShown, 0); err == nil {
		t.Fatal("an attempt without an answer before the close must be rejected")
	}
	c, err := Close(a, a.Start()+a.Results[0].Events[0].SinceShown+1, 0)
	if err != nil || c.Answered() != 1 {
		t.Fatalf("one answer before the close: %v, answered %d", err, c.Answered())
	}
}

func TestScoreSkipsUnansweredItems(t *testing.T) {
	it := testItem()
	a := &Attempt{
		Items: []Item{it, it, it},
		Results: []ItemResult{
			{Events: []Event{{SinceShown: 10, Option: it.Key}}, Dwell: 10, Gap: 1},
			{Events: []Event{{SinceShown: 10, Option: 0}, {SinceShown: 20, Option: it.Key, IsChange: true}}, Dwell: 20, Gap: 1},
			{Dwell: 10},
		},
		AutoSubmit: 40,
	}
	if a.Score() != 2 || a.Answered() != 2 || a.Changes() != 1 || a.Duration() != 40 {
		t.Fatalf("score %d answered %d changes %d duration %d", a.Score(), a.Answered(), a.Changes(), a.Duration())
	}
	if err := a.Check(); err != nil {
		t.Fatal(err)
	}
	a.AutoSubmit = 0
	if err := a.Check(); err == nil {
		t.Fatal("an unanswered item is only valid in an auto-submitted attempt")
	}
	a.AutoSubmit = 31 // the change at 31 ms since start is not before submission
	if err := a.Check(); err == nil {
		t.Fatal("an answer at or after the auto-submission must be rejected")
	}
}

func equalAttempt(a, b *Attempt) bool {
	if a.Offset != b.Offset || a.Review != b.Review || a.AutoSubmit != b.AutoSubmit || len(a.Results) != len(b.Results) {
		return false
	}
	for q := range a.Results {
		x, y := a.Results[q], b.Results[q]
		if x.Dwell != y.Dwell || x.Gap != y.Gap || len(x.Events) != len(y.Events) {
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
