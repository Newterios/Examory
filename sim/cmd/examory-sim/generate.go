package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/Newterios/Examory/sim/internal/events"
	"github.com/Newterios/Examory/sim/internal/params"
	"github.com/Newterios/Examory/sim/internal/world"
)

// Version is the simulator version recorded in every manifest.
const Version = "0.2.0"

var errHelp = errors.New("help requested")

const defaultWeights = "rapid_response=.35,excessive_changes=.25,collusion=.25,score_jump=.15"

// parseWeights accepts exactly the four slot names with finite non-negative
// weights, at least one positive (legacy weights_arg).
func parseWeights(v string) ([4]float64, error) {
	var w [4]float64
	bad := errors.New("anomaly-weights: provide all four distinct name=nonnegative-weight entries " +
		"(rapid_response, excessive_changes, collusion, score_jump); at least one must be positive")
	parts := strings.Split(v, ",")
	if len(parts) != 4 {
		return w, bad
	}
	seen := map[string]bool{}
	var total float64
	for _, p := range parts {
		kv := strings.Split(strings.TrimSpace(p), "=")
		if len(kv) != 2 {
			return w, bad
		}
		idx := -1
		for i, s := range world.Slots {
			if s == strings.TrimSpace(kv[0]) {
				idx = i
			}
		}
		if idx < 0 || seen[kv[0]] {
			return w, bad
		}
		seen[kv[0]] = true
		x, err := strconv.ParseFloat(strings.TrimSpace(kv[1]), 64)
		if err != nil || math.IsNaN(x) || math.IsInf(x, 0) || x < 0 {
			return w, bad
		}
		w[idx] = x
		total += x
	}
	if total <= 0 || math.IsInf(total, 0) {
		return w, bad
	}
	return w, nil
}

func parseWorlds(v string) ([]string, error) {
	want := map[string]bool{}
	for _, p := range strings.Split(v, ",") {
		p = strings.TrimSpace(p)
		if p != world.Honest && p != world.FI && p != world.BL {
			return nil, fmt.Errorf("worlds: unknown world %q (use honest, fi, bl)", p)
		}
		if want[p] {
			return nil, fmt.Errorf("worlds: %q listed twice", p)
		}
		want[p] = true
	}
	var out []string
	for _, w := range world.AllWorlds {
		if want[w] {
			out = append(out, w)
		}
	}
	return out, nil
}

func parseStart(v string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339Nano, v)
	if err != nil {
		return t, fmt.Errorf("start-at: use an RFC 3339 datetime with timezone, e.g. 2026-10-01T09:00:00Z")
	}
	t = t.UTC().Truncate(time.Millisecond)
	if t.Before(time.Unix(0, 0)) || t.Year() > 9000 {
		return t, errors.New("start-at must lie between 1970-01-01 and 9000-01-01 UTC")
	}
	return t, nil
}

// outDirReady creates dir, or refuses a non-empty existing dir without force.
func outDirReady(dir string, force bool) error {
	info, err := os.Stat(dir)
	switch {
	case err == nil && !info.IsDir():
		return fmt.Errorf("out-dir %s exists and is not a directory", dir)
	case err == nil:
		f, err := os.Open(dir)
		if err != nil {
			return err
		}
		names, err := f.Readdirnames(1)
		f.Close()
		if err != nil && err != io.EOF {
			return err
		}
		if len(names) > 0 && !force {
			return fmt.Errorf("out-dir %s is not empty; pass --force to overwrite", dir)
		}
		return nil
	case errors.Is(err, os.ErrNotExist):
		return os.MkdirAll(dir, 0o755)
	default:
		return err
	}
}

// writeFile writes atomically (temp file + rename) and returns the SHA-256.
func writeFile(dir, name string, fill func(io.Writer) error) (string, error) {
	tmp, err := os.CreateTemp(dir, "."+name+".tmp-*")
	if err != nil {
		return "", err
	}
	h := sha256.New()
	ferr := tmp.Chmod(0o644)
	if ferr == nil {
		ferr = fill(io.MultiWriter(tmp, h))
	}
	cerr := tmp.Close()
	if ferr != nil || cerr != nil {
		os.Remove(tmp.Name())
		if ferr != nil {
			return "", ferr
		}
		return "", cerr
	}
	if err := os.Rename(tmp.Name(), filepath.Join(dir, name)); err != nil {
		os.Remove(tmp.Name())
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Manifest documents one generation run.
type Manifest struct {
	Simulator  string                    `json:"simulator"`
	Version    string                    `json:"version"`
	GoVersion  string                    `json:"go_version"`
	GOARCH     string                    `json:"goarch"`
	Args       map[string]any            `json:"args"`
	Params     map[string]FileRef        `json:"params"`
	Outputs    map[string]FileRef        `json:"outputs"`
	Quotas     map[string]int            `json:"quotas"`
	Counts     map[string]WorldCounts    `json:"counts"`
	Mechanisms map[string]map[string]int `json:"mechanism_flags"`
	// SyncSubmit counts copy_peer pairs by sync_submit (bl world only).
	SyncSubmit map[string]int `json:"copy_peer_sync_submit_pairs,omitempty"`
}

// FileRef names a file and its SHA-256.
type FileRef struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// WorldCounts summarises one world.
type WorldCounts struct {
	Events            int            `json:"events"`
	Attempts          int            `json:"attempts"`
	AttemptsByType    map[string]int `json:"attempts_by_type"`
	AnomalousAttempts int            `json:"anomalous_attempts"`
}

func generate(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("generate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	students := fs.Int("students", 2000, "number of students (cohorts of 20 consecutive students)")
	attempts := fs.Int("attempts", 3, "attempts per student (one per day)")
	questions := fs.Int("questions", 20, "questions per attempt")
	rate := fs.Float64("anomaly-rate", 0.12, "fraction of ALL attempts that are anomalous; at most 1/attempts")
	weightsArg := fs.String("anomaly-weights", defaultWeights, "relative slot weights")
	seed := fs.Int64("seed", 42, "random seed")
	startArg := fs.String("start-at", "2026-10-01T09:00:00Z", "RFC 3339 start of attempt 0")
	paramsPath := fs.String("params", "sim/params/default.json", "honest-model parameter file")
	mechPath := fs.String("mechanisms", "sim/params/mechanisms.json", "mechanism parameter file")
	sampling := fs.String("item-sampling", world.SamplingFixed, "fixed | per-attempt")
	intensityArg := fs.String("intensity", "", "optional override in [0,1] of every mechanism's per-item coverage/propensity (proxy: ability-gain scale)")
	worldsArg := fs.String("worlds", "honest,fi,bl", "comma-separated subset of honest,fi,bl")
	outDir := fs.String("out-dir", "", "output directory (required)")
	force := fs.Bool("force", false, "allow writing into a non-empty out-dir")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return errHelp
		}
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected arguments: %v", fs.Args())
	}
	if *outDir == "" {
		return errors.New("--out-dir is required")
	}
	weights, err := parseWeights(*weightsArg)
	if err != nil {
		return err
	}
	worlds, err := parseWorlds(*worldsArg)
	if err != nil {
		return err
	}
	start, err := parseStart(*startArg)
	if err != nil {
		return err
	}
	var intensity *float64
	if *intensityArg != "" {
		x, err := strconv.ParseFloat(*intensityArg, 64)
		if err != nil || math.IsNaN(x) || x < 0 || x > 1 {
			return errors.New("intensity must be a number in [0, 1]")
		}
		intensity = &x
	}
	m, mSum, err := params.LoadModel(*paramsPath)
	if err != nil {
		return err
	}
	mech, mechSum, err := params.LoadMechanisms(*mechPath)
	if err != nil {
		return err
	}
	cfg := &world.Config{
		Students: *students, Attempts: *attempts, Questions: *questions,
		AnomalyRate: *rate, Weights: weights, Seed: *seed, StartAt: start,
		Model: m, Mech: mech, ItemSampling: *sampling, Intensity: intensity, Worlds: worlds,
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	if err := outDirReady(*outDir, *force); err != nil {
		return err
	}
	res, err := world.Generate(cfg)
	if err != nil {
		return err
	}
	quotas, _ := world.Quotas(cfg.Students, cfg.Attempts, cfg.AnomalyRate, cfg.Weights)
	man := Manifest{
		Simulator: "examory-sim", Version: Version, GoVersion: runtime.Version(), GOARCH: runtime.GOARCH,
		Args: map[string]any{
			"students": cfg.Students, "attempts": cfg.Attempts, "questions": cfg.Questions,
			"anomaly_rate": cfg.AnomalyRate, "anomaly_weights": map[string]float64{
				world.Slots[0]: weights[0], world.Slots[1]: weights[1], world.Slots[2]: weights[2], world.Slots[3]: weights[3]},
			"seed": cfg.Seed, "start_at": events.FormatTime(start.UnixMilli()),
			"params": *paramsPath, "mechanisms": *mechPath, "item_sampling": cfg.ItemSampling,
			"intensity": intensity, "worlds": worlds,
		},
		Params: map[string]FileRef{
			"params":     {Path: *paramsPath, SHA256: mSum},
			"mechanisms": {Path: *mechPath, SHA256: mechSum},
		},
		Outputs: map[string]FileRef{}, Quotas: quotas,
		Counts: map[string]WorldCounts{}, Mechanisms: map[string]map[string]int{},
	}
	for _, w := range worlds {
		recs := res.Worlds[w]
		rows := events.Rows(cfg.Seed, recs)
		name := w + ".csv"
		sum, err := writeFile(*outDir, name, func(out io.Writer) error { return events.WriteCSV(out, rows) })
		if err != nil {
			return err
		}
		man.Outputs[w] = FileRef{Path: name, SHA256: sum}
		wc := WorldCounts{Events: len(rows), Attempts: len(recs), AttemptsByType: map[string]int{}}
		flags := map[string]int{}
		for _, r := range recs {
			kind := "none"
			if r.IsAnomaly {
				kind = r.AnomalyType
				wc.AnomalousAttempts++
				flags[r.AnomalyType+":"+r.Flag]++
				if r.SyncSubmit >= 0 && r.Role == "source" {
					if man.SyncSubmit == nil {
						man.SyncSubmit = map[string]int{"sync": 0, "nosync": 0}
					}
					if r.SyncSubmit == 1 {
						man.SyncSubmit["sync"]++
					} else {
						man.SyncSubmit["nosync"]++
					}
				}
			}
			wc.AttemptsByType[kind]++
		}
		man.Counts[w] = wc
		man.Mechanisms[w] = flags
	}
	sum, err := writeFile(*outDir, "truth.csv", func(out io.Writer) error {
		return events.WriteTruth(out, cfg.Seed, worlds, res)
	})
	if err != nil {
		return err
	}
	man.Outputs["truth"] = FileRef{Path: "truth.csv", SHA256: sum}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(man); err != nil {
		return err
	}
	if _, err := writeFile(*outDir, "manifest.json", func(out io.Writer) error {
		_, err := out.Write(buf.Bytes())
		return err
	}); err != nil {
		return err
	}
	b, err := json.Marshal(map[string]any{"out_dir": *outDir, "counts": man.Counts})
	if err != nil {
		return err
	}
	fmt.Fprintln(stdout, string(b))
	return nil
}
