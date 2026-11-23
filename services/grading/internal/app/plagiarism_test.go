package app_test

import (
	"github.com/Newterios/Examory/services/grading/internal/app"
	"github.com/Newterios/Examory/services/platform/events"
	"testing"
)

func TestPlagiarismCategoryPreservesSubmissionMatches(t *testing.T) {
	f := newFx(t)
	_, attempt, _ := f.seed()
	p := riskPayload(attempt)
	p["target_type"] = "submission"
	p["risk_id"] = "copy-1"
	p["evidence"] = map[string]any{"matches": []any{map[string]any{"submission_id": "original", "similarity": .9, "overlap_spans": []any{map[string]any{"target": []int{0, 14}, "source": []int{0, 14}, "token_count": 5}}}}}
	env, err := events.New("risk.plagiarism", 1, "integrity", events.Actor{UserID: "integrity", Role: "system"}, events.Context{CourseID: &f.course, GroupID: &f.group}, p, t0)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err = f.ingest(events.TopicRisk, env); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := f.svc.ListRisks(f.ctx, f.teacher, app.Filter{Category: "risk.plagiarism"}, 5, "")
	if err != nil || len(rows) != 1 || rows[0].Attempt != nil {
		t.Fatalf("%v %v rejects=%v", rows, err, f.store.Snapshot().Rejected)
	}
	row, err := f.svc.GetRisk(f.ctx, f.teacher, rows[0].Risk.ID)
	if err != nil || len(row.Risk.Evidence) == 0 || row.Attempt != nil {
		t.Fatal(row, err)
	}
	if _, err = f.svc.Review(f.ctx, f.teacher, row.Risk.ID, "dismissed", "Quoted source verified", 1, "review-copy-001"); err != nil {
		t.Fatal(err)
	}
	latest := f.store.Snapshot().Outbox
	var reviewed map[string]any
	if err = latest[len(latest)-1].Env.DecodePayload(&reviewed, false); err != nil || reviewed["target_type"] != "submission" {
		t.Fatal(reviewed, err)
	}
}
