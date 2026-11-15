package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func baseArgs(dir string) []string {
	return []string{"generate", "--students", "120", "--seed", "7",
		"--params", "../../params/default.json", "--mechanisms", "../../params/mechanisms.json",
		"--out-dir", dir}
}

func runCLI(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := run(args, &out, &errb)
	return code, out.String(), errb.String()
}

func TestGenerateDeterministicAndManifest(t *testing.T) {
	d1, d2 := filepath.Join(t.TempDir(), "a"), filepath.Join(t.TempDir(), "b")
	for _, d := range []string{d1, d2} {
		if code, _, stderr := runCLI(baseArgs(d)...); code != 0 {
			t.Fatalf("exit %d: %s", code, stderr)
		}
	}
	for _, name := range []string{"honest.csv", "fi.csv", "bl.csv", "truth.csv", "manifest.json"} {
		a, err := os.ReadFile(filepath.Join(d1, name))
		if err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(filepath.Join(d2, name))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(a, b) {
			t.Fatalf("%s differs between identical runs", name)
		}
	}
	if code, out, stderr := runCLI("validate", filepath.Join(d1, "fi.csv"), filepath.Join(d1, "bl.csv")); code != 0 || !strings.Contains(out, `"attempts":360`) {
		t.Fatalf("validate failed: %d %s %s", code, out, stderr)
	}
	var man Manifest
	raw, _ := os.ReadFile(filepath.Join(d1, "manifest.json"))
	if err := json.Unmarshal(raw, &man); err != nil {
		t.Fatal(err)
	}
	for key, ref := range man.Outputs {
		data, err := os.ReadFile(filepath.Join(d1, ref.Path))
		if err != nil {
			t.Fatal(err)
		}
		h := sha256.Sum256(data)
		if hex.EncodeToString(h[:]) != ref.SHA256 {
			t.Fatalf("manifest hash of %s is wrong", key)
		}
	}
	if pairs := man.Counts["bl"].AttemptsByType["copy_peer"] / 2; man.SyncSubmit["sync"]+man.SyncSubmit["nosync"] != pairs || pairs == 0 {
		t.Fatalf("sync_submit pair counts %v for %d pairs", man.SyncSubmit, pairs)
	}
	if man.Params["params"].SHA256 == "" || man.Version != Version || man.Counts["bl"].AnomalousAttempts != 43 {
		t.Fatalf("manifest incomplete: %+v", man)
	}
}

func TestRefusesNonEmptyOutDir(t *testing.T) {
	d := t.TempDir()
	if err := os.WriteFile(filepath.Join(d, "keep.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := runCLI("validate", filepath.Join(d, "keep.txt")); code != 2 {
		t.Fatal("validate must reject a non-CSV file")
	}
	if code, _, stderr := runCLI(baseArgs(d)...); code != 2 || !strings.Contains(stderr, "--force") {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if code, _, stderr := runCLI(append(baseArgs(d), "--force")...); code != 0 {
		t.Fatalf("exit %d with --force: %s", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(d, "keep.txt")); err != nil {
		t.Fatal("unrelated file removed")
	}
}

func TestBadInputExitCode2(t *testing.T) {
	d := t.TempDir()
	badParams := filepath.Join(d, "bad.json")
	os.WriteFile(badParams, []byte(`{"schema_version": 1}`), 0o644)
	cases := [][]string{
		{"--students", "0"},
		{"--students", "abc"},
		{"--attempts", "0"},
		{"--questions", "-1"},
		{"--anomaly-rate", "0.5"},
		{"--anomaly-rate", "NaN"},
		{"--anomaly-weights", "rapid_response=1"},
		{"--anomaly-weights", "rapid_response=1,excessive_changes=1,collusion=1,collusion=1"},
		{"--anomaly-weights", "rapid_response=-1,excessive_changes=1,collusion=1,score_jump=1"},
		{"--anomaly-weights", "rapid_response=0,excessive_changes=0,collusion=0,score_jump=0"},
		{"--start-at", "2026-10-01 09:00"},
		{"--start-at", "2026-10-01T09:00:00"},
		{"--item-sampling", "random"},
		{"--intensity", "2"},
		{"--intensity", "x"},
		{"--worlds", "honest,honest"},
		{"--worlds", "real"},
		{"--params", filepath.Join(d, "missing.json")},
		{"--params", badParams},
		{"--attempts", "1", "--anomaly-rate", "0.1"},
		{"--unknown-flag"},
		{"extra-positional"},
	}
	for _, c := range cases {
		out := filepath.Join(t.TempDir(), "o")
		args := append(baseArgs(out), c...)
		if code, _, stderr := runCLI(args...); code != 2 || stderr == "" {
			t.Fatalf("%v: exit %d stderr %q", c, code, stderr)
		}
		if _, err := os.Stat(out); err == nil {
			t.Fatalf("%v: output directory created despite invalid input", c)
		}
	}
	if code, _, _ := runCLI("generate"); code != 2 {
		t.Fatal("missing --out-dir must fail")
	}
	if code, _, _ := runCLI(); code != 2 {
		t.Fatal("no command must fail")
	}
	if code, _, _ := runCLI("frobnicate"); code != 2 {
		t.Fatal("unknown command must fail")
	}
	if code, _, _ := runCLI("generate", "-h"); code != 0 {
		t.Fatal("help must succeed")
	}
}

func TestWorldSubset(t *testing.T) {
	d := filepath.Join(t.TempDir(), "o")
	if code, _, stderr := runCLI(append(baseArgs(d), "--worlds", "bl", "--item-sampling", "per-attempt", "--intensity", "0.3")...); code != 0 {
		t.Fatal(stderr)
	}
	if _, err := os.Stat(filepath.Join(d, "honest.csv")); err == nil {
		t.Fatal("unrequested world written")
	}
	if _, err := os.Stat(filepath.Join(d, "bl.csv")); err != nil {
		t.Fatal(err)
	}
}

func readManifest(t *testing.T, dir string) Manifest {
	t.Helper()
	var man Manifest
	raw, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &man); err != nil {
		t.Fatal(err)
	}
	return man
}

func truthHeader(t *testing.T, dir string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "truth.csv"))
	if err != nil {
		t.Fatal(err)
	}
	return strings.SplitN(string(raw), "\r\n", 2)[0]
}

func TestQuizWindowFlagAndParams(t *testing.T) {
	off := filepath.Join(t.TempDir(), "off")
	if code, _, stderr := runCLI(baseArgs(off)...); code != 0 {
		t.Fatal(stderr)
	}
	if man := readManifest(t, off); man.QuizWindow != nil || man.Args["close_after_ms"] != float64(0) {
		t.Fatalf("window recorded although off: %+v %v", man.QuizWindow, man.Args["close_after_ms"])
	}
	if strings.Contains(truthHeader(t, off), "auto_submitted") {
		t.Fatal("window columns written although the window is off")
	}

	flag := filepath.Join(t.TempDir(), "flag")
	if code, _, stderr := runCLI(append(baseArgs(flag), "--close-after-ms", "1200000", "--auto-submit-jitter-ms", "500")...); code != 0 {
		t.Fatal(stderr)
	}
	man := readManifest(t, flag)
	if w := man.QuizWindow; w == nil || w.CloseAfterMs != 1200000 || w.AutoSubmitJitterMs != 500 || w.Source != "flag" ||
		w.AutoSubmitted["honest"] == 0 || w.AutoSubmitted["bl"] == 0 {
		t.Fatalf("manifest quiz_window %+v", man.QuizWindow)
	}
	if !strings.HasSuffix(truthHeader(t, flag), ",auto_submitted,answered_items,natural_duration_ms,natural_score") {
		t.Fatalf("truth header %q", truthHeader(t, flag))
	}
	if code, out, stderr := runCLI("validate", filepath.Join(flag, "honest.csv"), filepath.Join(flag, "bl.csv")); code != 0 || !strings.Contains(out, `"attempts":360`) {
		t.Fatalf("validate failed: %d %s %s", code, out, stderr)
	}

	// The same window from the parameter file gives the same outputs; the
	// flag overrides the file, and 0 disables the window.
	raw, err := os.ReadFile("../../params/default.json")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	m["quiz_window"] = map[string]any{"close_after_ms": 1200000, "auto_submit_jitter_ms": 500}
	b, _ := json.Marshal(m)
	pfile := filepath.Join(t.TempDir(), "window.json")
	if err := os.WriteFile(pfile, b, 0o644); err != nil {
		t.Fatal(err)
	}
	withParams := func(dir string, extra ...string) []string {
		args := baseArgs(dir)
		for i := range args {
			if args[i] == "--params" {
				args[i+1] = pfile
			}
		}
		return append(args, extra...)
	}
	fromFile := filepath.Join(t.TempDir(), "file")
	if code, _, stderr := runCLI(withParams(fromFile)...); code != 0 {
		t.Fatal(stderr)
	}
	if readManifest(t, fromFile).QuizWindow.Source != "params" {
		t.Fatal("window source not recorded")
	}
	for _, name := range []string{"honest.csv", "fi.csv", "bl.csv", "truth.csv"} {
		a, _ := os.ReadFile(filepath.Join(flag, name))
		c, _ := os.ReadFile(filepath.Join(fromFile, name))
		if !bytes.Equal(a, c) {
			t.Fatalf("%s differs between flag and parameter-file window", name)
		}
	}
	disabled := filepath.Join(t.TempDir(), "disabled")
	if code, _, stderr := runCLI(withParams(disabled, "--close-after-ms", "0")...); code != 0 {
		t.Fatal(stderr)
	}
	for _, name := range []string{"honest.csv", "fi.csv", "bl.csv", "truth.csv"} {
		a, _ := os.ReadFile(filepath.Join(off, name))
		c, _ := os.ReadFile(filepath.Join(disabled, name))
		if !bytes.Equal(a, c) {
			t.Fatalf("%s: --close-after-ms 0 does not reproduce the window-off output", name)
		}
	}

	for _, c := range [][]string{
		{"--close-after-ms", "-1"},
		{"--close-after-ms", "x"},
		{"--close-after-ms", "1.5"},
		{"--close-after-ms", "600000"},
		{"--close-after-ms", "86400001"},
		{"--close-after-ms", "1200000", "--auto-submit-jitter-ms", "-5"},
		{"--close-after-ms", "1200000", "--auto-submit-jitter-ms", "3600001"},
	} {
		out := filepath.Join(t.TempDir(), "o")
		if code, _, stderr := runCLI(append(baseArgs(out), c...)...); code != 2 || stderr == "" {
			t.Fatalf("%v: exit %d stderr %q", c, code, stderr)
		}
		if _, err := os.Stat(out); err == nil {
			t.Fatalf("%v: output directory created despite invalid input", c)
		}
	}
}
