package params

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func readRepoFile(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile("../../params/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestLoadShippedFiles(t *testing.T) {
	m, sum, err := LoadModel("../../params/default.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(sum) != 64 || m.MinResponseMs != 500 || m.Person.ThetaSD != 1.0 || len(m.ItemPrior.DistractorAlpha) != 3 {
		t.Fatalf("unexpected model %+v sum %s", m, sum)
	}
	mc, _, err := LoadMechanisms("../../params/mechanisms.json")
	if err != nil {
		t.Fatal(err)
	}
	if mc.FI.ChangesMaxCount != 4 || mc.CopyPeer.Coverage != 0.8 || mc.FI.RapidMinMs != 500 {
		t.Fatalf("unexpected mechanisms %+v", mc)
	}
}

// mutate decodes the JSON file, applies f to the generic tree and re-encodes.
func mutate(t *testing.T, raw []byte, f func(map[string]any)) []byte {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	f(m)
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func obj(m map[string]any, key string) map[string]any { return m[key].(map[string]any) }

func TestModelRejects(t *testing.T) {
	raw := readRepoFile(t, "default.json")
	cases := map[string]struct {
		f    func(map[string]any)
		want string
	}{
		"schema":        {func(m map[string]any) { m["schema_version"] = 2 }, "schema_version"},
		"schema-string": {func(m map[string]any) { m["schema_version"] = "1" }, "schema_version"},
		"no-schema":     {func(m map[string]any) { delete(m, "schema_version") }, "schema_version"},
		"neg-sd":        {func(m map[string]any) { obj(m, "person")["theta_sd"] = -0.1 }, "theta_sd"},
		"neg-gap-sd":    {func(m map[string]any) { obj(m, "gap")["log_sd"] = -1 }, "gap.log_sd"},
		"corr":          {func(m map[string]any) { obj(m, "person")["theta_tau_corr"] = 1.0 }, "theta_tau_corr"},
		"pzero":         {func(m map[string]any) { obj(obj(m, "person"), "disengage")["p_zero"] = 1.5 }, "p_zero"},
		"beta-zero":     {func(m map[string]any) { obj(obj(m, "person"), "revise")["beta_a"] = 0 }, "revise.beta_a"},
		"alpha-len":     {func(m map[string]any) { obj(m, "item_prior")["distractor_alpha"] = []any{1, 1} }, "distractor_alpha"},
		"missing":       {func(m map[string]any) { delete(obj(m, "revision"), "wrong_to_right") }, "revision.wrong_to_right"},
		"missing-obj":   {func(m map[string]any) { delete(m, "review") }, "review"},
		"null":          {func(m map[string]any) { m["gap"] = nil }, "gap"},
		"wrong-type":    {func(m map[string]any) { m["gap"] = 3 }, "gap"},
		"min-ms":        {func(m map[string]any) { m["min_response_ms"] = 0 }, "min_response_ms"},
		"min-ms-frac":   {func(m map[string]any) { m["min_response_ms"] = 1.5 }, "min_response_ms"},
		"pool-weights": {func(m map[string]any) {
			m["items"] = []any{map[string]any{"id": "x", "a": 1, "b": 0, "c": 0.2, "beta": 10, "sigma": 0.5, "distractor_weights": []any{0.5, 0.3, 0.1}}}
		}, "sum to 1"},
		"pool-neg-weight": {func(m map[string]any) {
			m["items"] = []any{map[string]any{"id": "x", "a": 1, "b": 0, "c": 0.2, "beta": 10, "sigma": 0.5, "distractor_weights": []any{1.2, -0.2, 0}}}
		}, "distractor_weights"},
		"pool-dup": {func(m map[string]any) {
			it := map[string]any{"id": "x", "a": 1, "b": 0, "c": 0.2, "beta": 10, "sigma": 0.5, "distractor_weights": []any{0.5, 0.3, 0.2}}
			m["items"] = []any{it, it}
		}, "duplicated"},
		"pool-missing": {func(m map[string]any) {
			m["items"] = []any{map[string]any{"id": "x", "a": 1, "b": 0, "c": 0.2, "beta": 10, "distractor_weights": []any{0.5, 0.3, 0.2}}}
		}, "items[0].sigma"},
		"pool-a": {func(m map[string]any) {
			m["items"] = []any{map[string]any{"id": "x", "a": 0, "b": 0, "c": 0.2, "beta": 10, "sigma": 0.5, "distractor_weights": []any{0.5, 0.3, 0.2}}}
		}, "items[0].a"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := ParseModel(mutate(t, raw, tc.f))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestRejectsNonFiniteAndMalformedJSON(t *testing.T) {
	raw := string(readRepoFile(t, "default.json"))
	for name, text := range map[string]string{
		"nan":      strings.Replace(raw, `"theta_sd": 1.0`, `"theta_sd": NaN`, 1),
		"inf":      strings.Replace(raw, `"theta_sd": 1.0`, `"theta_sd": Infinity`, 1),
		"overflow": strings.Replace(raw, `"theta_sd": 1.0`, `"theta_sd": 1e999`, 1),
		"trailing": raw + "{}",
		"array":    "[]",
		"empty":    "",
	} {
		t.Run(name, func(t *testing.T) {
			if text == raw {
				t.Fatal("replacement did not apply")
			}
			if _, err := ParseModel([]byte(text)); err == nil {
				t.Fatal("expected error")
			}
		})
	}
	// Programmatic NaN is rejected by Validate.
	m, err := ParseModel([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	m.Revision.WrongToRight = nan()
	if err := m.Validate(); err == nil || !strings.Contains(err.Error(), "finite") {
		t.Fatalf("NaN accepted: %v", err)
	}
}

func nan() float64 { var z float64; return z / z }

func TestUnknownFieldsAndValidPool(t *testing.T) {
	raw := readRepoFile(t, "default.json")
	out := mutate(t, raw, func(m map[string]any) {
		m["future_field"] = map[string]any{"x": 1}
		obj(m, "person")["another"] = 3
		m["items"] = []any{map[string]any{"id": "q1", "a": 1.1, "b": -0.2, "c": 0.25, "beta": 10.3, "sigma": 0.45,
			"distractor_weights": []any{0.6, 0.3, 0.1}, "n_obs": 1234}}
	})
	m, err := ParseModel(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Items) != 1 || m.Items[0].ID != "q1" {
		t.Fatalf("pool not loaded: %+v", m.Items)
	}
}

func TestMechanismsRejects(t *testing.T) {
	raw := readRepoFile(t, "mechanisms.json")
	cases := map[string]struct {
		f    func(map[string]any)
		want string
	}{
		"schema":      {func(m map[string]any) { m["schema_version"] = 0 }, "schema_version"},
		"coverage":    {func(m map[string]any) { obj(m, "preknowledge")["coverage"] = 1.2 }, "coverage"},
		"speed-order": {func(m map[string]any) { obj(m, "preknowledge")["speedup_low"] = 0.9 }, "speedup_low"},
		"neg-sd":      {func(m map[string]any) { obj(m, "copy_peer")["relay_log_sd"] = -0.5 }, "relay_log_sd"},
		"proxy-sd":    {func(m map[string]any) { obj(m, "proxy")["theta_sd"] = -1 }, "proxy.theta_sd"},
		"max-count":   {func(m map[string]any) { obj(m, "fi")["changes_max_count"] = 0 }, "changes_max_count"},
		"jump-order":  {func(m map[string]any) { obj(m, "fi")["score_jump_low"] = 0.5 }, "score_jump_low"},
		"window":      {func(m map[string]any) { obj(m, "fi")["collusion_submit_window_ms"] = -1 }, "collusion_submit_window_ms"},
		"missing":     {func(m map[string]any) { delete(obj(m, "lookup"), "success") }, "lookup.success"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := ParseMechanisms(mutate(t, raw, tc.f))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestSyncSubmitOptionalFields(t *testing.T) {
	raw := readRepoFile(t, "mechanisms.json")
	absent := mutate(t, raw, func(m map[string]any) {
		cp := obj(m, "copy_peer")
		delete(cp, "sync_submit_prob")
		delete(cp, "sync_submit_log_ms_mean")
		delete(cp, "sync_submit_log_sd")
	})
	m, err := ParseMechanisms(absent)
	if err != nil {
		t.Fatal(err)
	}
	if m.CopyPeer.SyncSubmitProb != DefaultSyncSubmitProb || m.CopyPeer.SyncSubmitLogMsMean != DefaultSyncSubmitLogMsMean ||
		m.CopyPeer.SyncSubmitLogSD != DefaultSyncSubmitLogSD {
		t.Fatalf("defaults not applied: %+v", m.CopyPeer)
	}
	zero, err := ParseMechanisms(mutate(t, raw, func(m map[string]any) { obj(m, "copy_peer")["sync_submit_prob"] = 0 }))
	if err != nil || zero.CopyPeer.SyncSubmitProb != 0 {
		t.Fatalf("explicit zero not honoured: %v %+v", err, zero)
	}
	for name, f := range map[string]func(map[string]any){
		"prob": func(m map[string]any) { obj(m, "copy_peer")["sync_submit_prob"] = 1.5 },
		"sd":   func(m map[string]any) { obj(m, "copy_peer")["sync_submit_log_sd"] = -1 },
	} {
		if _, err := ParseMechanisms(mutate(t, raw, f)); err == nil || !strings.Contains(err.Error(), "sync_submit") {
			t.Fatalf("%s: want sync_submit error, got %v", name, err)
		}
	}
}

func TestQuizWindowOptionalFields(t *testing.T) {
	raw := readRepoFile(t, "default.json")
	m, err := ParseModel(raw)
	if err != nil {
		t.Fatal(err)
	}
	if m.QuizWindow.CloseAfterMs != 0 || m.QuizWindow.AutoSubmitJitterMs != DefaultAutoSubmitJitterMs {
		t.Fatalf("absent quiz_window: %+v, want disabled with default jitter", m.QuizWindow)
	}
	set := func(w map[string]any) []byte {
		return mutate(t, raw, func(m map[string]any) { m["quiz_window"] = w })
	}
	m, err = ParseModel(set(map[string]any{"close_after_ms": 1800000}))
	if err != nil || m.QuizWindow.CloseAfterMs != 1800000 || m.QuizWindow.AutoSubmitJitterMs != DefaultAutoSubmitJitterMs {
		t.Fatalf("close only: %v %+v", err, m)
	}
	m, err = ParseModel(set(map[string]any{"close_after_ms": 1800000, "auto_submit_jitter_ms": 0}))
	if err != nil || m.QuizWindow.AutoSubmitJitterMs != 0 {
		t.Fatalf("explicit zero jitter not honoured: %v %+v", err, m)
	}
	for name, w := range map[string]map[string]any{
		"missing-close": {"auto_submit_jitter_ms": 100},
		"neg-close":     {"close_after_ms": -1},
		"huge-close":    {"close_after_ms": 86400001},
		"frac-close":    {"close_after_ms": 1.5},
		"neg-jitter":    {"close_after_ms": 1800000, "auto_submit_jitter_ms": -1},
		"huge-jitter":   {"close_after_ms": 1800000, "auto_submit_jitter_ms": 3600001},
	} {
		if _, err := ParseModel(set(w)); err == nil || !strings.Contains(err.Error(), "_ms") {
			t.Fatalf("%s: want a quiz_window error, got %v", name, err)
		}
	}
}
