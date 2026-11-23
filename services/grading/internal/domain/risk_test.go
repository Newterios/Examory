package domain

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestReviewStateMachine(t *testing.T) {
	r := &Risk{Status: StatusOpen, Version: 1}
	if err := r.ApplyReview(DecisionConfirmed, 2); !errors.Is(err, ErrVersionMismatch) {
		t.Fatalf("stale version accepted: %v", err)
	}
	if r.Status != StatusOpen || r.Version != 1 {
		t.Fatal("failed review mutated the risk")
	}
	if err := r.ApplyReview(DecisionConfirmed, 1); err != nil || r.Status != StatusConfirmed || r.Version != 2 {
		t.Fatalf("review: %v %+v", err, r)
	}
	for _, d := range []string{DecisionConfirmed, DecisionDismissed} {
		if err := r.ApplyReview(d, 2); !errors.Is(err, ErrAlreadyReviewed) {
			t.Fatalf("second review allowed: %v", err)
		}
	}
}

func TestValidateDecision(t *testing.T) {
	ok := []struct{ d, c string }{{"confirmed", "pattern matches"}, {"dismissed", strings.Repeat("я", 2000)}}
	for _, c := range ok {
		if err := ValidateDecision(c.d, c.c); err != nil {
			t.Errorf("%v rejected: %v", c, err)
		}
	}
	bad := []struct{ d, c string }{{"maybe", "x"}, {"confirmed", ""}, {"confirmed", "   "}, {"dismissed", strings.Repeat("a", 2001)}, {"confirmed", "\xff"}}
	for _, c := range bad {
		if err := ValidateDecision(c.d, c.c); err == nil {
			t.Errorf("%q/%d accepted", c.d, len(c.c))
		}
	}
}

func TestCheckPayload(t *testing.T) {
	good := RiskPayload{RiskID: "r1", TargetType: "attempt", TargetID: "a1", Score: .9, Level: "high", Explanation: "x", Evidence: json.RawMessage(`{"features":{}}`)}
	if err := good.CheckPayload(); err != nil {
		t.Fatal(err)
	}
	for name, mut := range map[string]func(*RiskPayload){
		"score": func(p *RiskPayload) { p.Score = 1.5 }, "level": func(p *RiskPayload) { p.Level = "severe" },
		"id": func(p *RiskPayload) { p.RiskID = "a b" }, "evidence": func(p *RiskPayload) { p.Evidence = nil },
		"explanation": func(p *RiskPayload) { p.Explanation = "" },
	} {
		p := good
		mut(&p)
		if p.CheckPayload() == nil {
			t.Errorf("%s accepted", name)
		}
	}
}
