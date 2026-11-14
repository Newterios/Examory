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
