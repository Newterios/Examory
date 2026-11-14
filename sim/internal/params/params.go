// Package params loads and validates the simulator parameter files.
//
// Two files exist: the honest-model file (default.json, or an
// EdNet-calibrated file in the same schema) and the mechanism file
// (mechanisms.json). Unknown fields are tolerated for forward
// compatibility; every field declared here without `omitempty` is required,
// and every numeric value is range-checked.
package params

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"reflect"
	"strings"
)

// SchemaVersion is the only parameter schema version this simulator reads.
const SchemaVersion = 1

// MaxFileBytes bounds the size of a parameter file.
const MaxFileBytes = 64 << 20

// BetaDist parameterises a Beta distribution.
type BetaDist struct {
	BetaA float64 `json:"beta_a"`
	BetaB float64 `json:"beta_b"`
}

// Disengage is the zero-inflated Beta prior of the rapid-guess propensity.
type Disengage struct {
	PZero float64 `json:"p_zero"`
	BetaA float64 `json:"beta_a"`
	BetaB float64 `json:"beta_b"`
}

// Person holds the population distribution of person parameters.
type Person struct {
	ThetaSD          float64   `json:"theta_sd"`
	TauSD            float64   `json:"tau_sd"`
	ThetaTauCorr     float64   `json:"theta_tau_corr"`
	PracticeGainMean float64   `json:"practice_gain_mean"`
	PracticeGainSD   float64   `json:"practice_gain_sd"`
	Disengage        Disengage `json:"disengage"`
	Revise           BetaDist  `json:"revise"`
}

// ItemPrior is the generating distribution of item parameters, used when the
// item pool is empty.
type ItemPrior struct {
	BMean           float64   `json:"b_mean"`
	BSD             float64   `json:"b_sd"`
	LogAMean        float64   `json:"log_a_mean"`
	LogASD          float64   `json:"log_a_sd"`
	C               float64   `json:"c"`
	BetaMean        float64   `json:"beta_mean"`
	BetaSD          float64   `json:"beta_sd"`
	LogSigmaMean    float64   `json:"log_sigma_mean"`
	LogSigmaSD      float64   `json:"log_sigma_sd"`
	DistractorAlpha []float64 `json:"distractor_alpha"`
}

// PoolItem is one calibrated item.
type PoolItem struct {
	ID                string    `json:"id"`
	A                 float64   `json:"a"`
	B                 float64   `json:"b"`
	C                 float64   `json:"c"`
	Beta              float64   `json:"beta"`
	Sigma             float64   `json:"sigma"`
	DistractorWeights []float64 `json:"distractor_weights"`
}

// LogNormal parameterises log(ms) ~ N(LogMsMean, LogSD^2).
type LogNormal struct {
	LogMsMean float64 `json:"log_ms_mean"`
	LogSD     float64 `json:"log_sd"`
}

// Revision holds the answer-change process.
type Revision struct {
	ChangeMultWrong  float64 `json:"change_mult_wrong"`
	ChangeMultRight  float64 `json:"change_mult_right"`
	WrongToRight     float64 `json:"wrong_to_right"`
	DelayLogMsMean   float64 `json:"delay_log_ms_mean"`
	DelayLogSD       float64 `json:"delay_log_sd"`
	SecondChangeProb float64 `json:"second_change_prob"`
}

// Model is the honest-model parameter file.
type Model struct {
	SchemaVersion int             `json:"schema_version"`
	Source        json.RawMessage `json:"source,omitempty"`
	Person        Person          `json:"person"`
	ItemPrior     ItemPrior       `json:"item_prior"`
	Items         []PoolItem      `json:"items,omitempty"`
	RapidGuess    LogNormal       `json:"rapid_guess"`
	Revision      Revision        `json:"revision"`
	Gap           LogNormal       `json:"gap"`
	Review        LogNormal       `json:"review"`
	MinResponseMs int64           `json:"min_response_ms"`
}

// Preknowledge parameterises the leaked-item mechanism.
type Preknowledge struct {
	Coverage    float64 `json:"coverage"`
	Recall      float64 `json:"recall"`
	NaiveProb   float64 `json:"naive_prob"`
	SpeedupLow  float64 `json:"speedup_low"`
	SpeedupHigh float64 `json:"speedup_high"`
}

// Lookup parameterises the external-resource mechanism.
type Lookup struct {
	Propensity        float64 `json:"propensity"`
	Success           float64 `json:"success"`
	DurationLogMsMean float64 `json:"duration_log_ms_mean"`
	DurationLogSD     float64 `json:"duration_log_sd"`
	VerifyModeProb    float64 `json:"verify_mode_prob"`
}

// CopyPeer parameterises the copy-from-peer mechanism.
type CopyPeer struct {
	Coverage                float64 `json:"coverage"`
	RelayLogMsMean          float64 `json:"relay_log_ms_mean"`
	RelayLogSD              float64 `json:"relay_log_sd"`
	EntryLogMsMean          float64 `json:"entry_log_ms_mean"`
	EntryLogSD              float64 `json:"entry_log_sd"`
	StartLagLogMsMean       float64 `json:"start_lag_log_ms_mean"`
	StartLagLogSD           float64 `json:"start_lag_log_sd"`
	FollowChangeProb        float64 `json:"follow_change_prob"`
	StrongerIsSourceProb    float64 `json:"stronger_is_source_prob"`
	SourceOverheadLogMsMean float64 `json:"source_overhead_log_ms_mean"`
	SourceOverheadLogSD     float64 `json:"source_overhead_log_sd"`
	// Optional (added after the seed-42 pilot): pair-level probability that
	// the pair submits close together, and the lognormal gap between the
	// two submissions. Defaults apply when the fields are absent.
	SyncSubmitProb      float64 `json:"sync_submit_prob,omitempty"`
	SyncSubmitLogMsMean float64 `json:"sync_submit_log_ms_mean,omitempty"`
	SyncSubmitLogSD     float64 `json:"sync_submit_log_sd,omitempty"`
}

// Defaults of optional mechanism fields.
const (
	DefaultSyncSubmitProb      = 0.5
	DefaultSyncSubmitLogMsMean = 7.6
	DefaultSyncSubmitLogSD     = 0.7
)

// Proxy parameterises impersonation.
type Proxy struct {
	ThetaMean float64 `json:"theta_mean"`
	ThetaSD   float64 `json:"theta_sd"`
	MinGain   float64 `json:"min_gain"`
}

// FI holds the constants of the legacy feature-aligned injection recipes.
type FI struct {
	RapidItemProb                float64 `json:"rapid_item_prob"`
	RapidScaleLow                float64 `json:"rapid_scale_low"`
	RapidScaleHigh               float64 `json:"rapid_scale_high"`
	RapidMinMs                   int64   `json:"rapid_min_ms"`
	ChangesProbLow               float64 `json:"changes_prob_low"`
	ChangesProbHigh              float64 `json:"changes_prob_high"`
	ChangesMaxCount              int     `json:"changes_max_count"`
	CollusionForcedWrongFraction float64 `json:"collusion_forced_wrong_fraction"`
	CollusionCopyProb            float64 `json:"collusion_copy_prob"`
	CollusionDurationJitter      float64 `json:"collusion_duration_jitter"`
	CollusionStartWindowMs       int64   `json:"collusion_start_window_ms"`
	CollusionSubmitWindowMs      int64   `json:"collusion_submit_window_ms"`
	ScoreJumpLow                 float64 `json:"score_jump_low"`
	ScoreJumpHigh                float64 `json:"score_jump_high"`
	ScoreProbCap                 float64 `json:"score_prob_cap"`
}

// Mechanisms is the mechanism parameter file.
type Mechanisms struct {
	SchemaVersion int          `json:"schema_version"`
	Preknowledge  Preknowledge `json:"preknowledge"`
	Lookup        Lookup       `json:"lookup"`
	CopyPeer      CopyPeer     `json:"copy_peer"`
	Proxy         Proxy        `json:"proxy"`
	FI            FI           `json:"fi"`
}

// LoadModel reads, decodes and validates an honest-model file. It returns
// the parsed model and the SHA-256 of the file bytes.
func LoadModel(path string) (*Model, string, error) {
	raw, sum, err := readFile(path)
	if err != nil {
		return nil, "", err
	}
	m, err := ParseModel(raw)
	if err != nil {
		return nil, "", fmt.Errorf("%s: %w", path, err)
	}
	return m, sum, nil
}

// LoadMechanisms reads, decodes and validates a mechanism file.
func LoadMechanisms(path string) (*Mechanisms, string, error) {
	raw, sum, err := readFile(path)
	if err != nil {
		return nil, "", err
	}
	m, err := ParseMechanisms(raw)
	if err != nil {
		return nil, "", fmt.Errorf("%s: %w", path, err)
	}
	return m, sum, nil
}

func readFile(path string) ([]byte, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, MaxFileBytes+1))
	if err != nil {
		return nil, "", err
	}
	if len(raw) > MaxFileBytes {
		return nil, "", fmt.Errorf("%s: file larger than %d bytes", path, MaxFileBytes)
	}
	h := sha256.Sum256(raw)
	return raw, hex.EncodeToString(h[:]), nil
}

// ParseModel decodes and validates honest-model JSON.
func ParseModel(raw []byte) (*Model, error) {
	var m Model
	if err := decode(raw, &m); err != nil {
		return nil, err
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

// ParseMechanisms decodes and validates mechanism JSON.
func ParseMechanisms(raw []byte) (*Mechanisms, error) {
	// Optional fields keep these values when absent from the file
	// (json.Unmarshal leaves absent fields untouched).
	m := Mechanisms{CopyPeer: CopyPeer{
		SyncSubmitProb:      DefaultSyncSubmitProb,
		SyncSubmitLogMsMean: DefaultSyncSubmitLogMsMean,
		SyncSubmitLogSD:     DefaultSyncSubmitLogSD,
	}}
	if err := decode(raw, &m); err != nil {
		return nil, err
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

func decode(raw []byte, dst any) error {
	var generic any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&generic); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if dec.More() {
		return errors.New("invalid JSON: trailing data after top-level value")
	}
	obj, ok := generic.(map[string]any)
	if !ok {
		return errors.New("top-level JSON value must be an object")
	}
	if err := checkSchemaVersion(obj); err != nil {
		return err
	}
	if err := requireFields(reflect.TypeOf(dst).Elem(), obj, ""); err != nil {
		return err
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return fmt.Errorf("invalid parameter value: %w", err)
	}
	return nil
}

func checkSchemaVersion(obj map[string]any) error {
	v, ok := obj["schema_version"]
	if !ok {
		return errors.New("missing field schema_version")
	}
	n, ok := v.(json.Number)
	if !ok || n.String() != fmt.Sprint(SchemaVersion) {
		return fmt.Errorf("unsupported schema_version %v (want %d)", v, SchemaVersion)
	}
	return nil
}

// requireFields checks that every non-omitempty JSON field of t is present
// in obj, recursing into nested structs and slices of structs.
func requireFields(t reflect.Type, obj map[string]any, prefix string) error {
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag := f.Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		parts := strings.Split(tag, ",")
		name, optional := parts[0], false
		for _, p := range parts[1:] {
			if p == "omitempty" {
				optional = true
			}
		}
		path := prefix + name
		v, present := obj[name]
		if !present || v == nil {
			if optional {
				continue
			}
			return fmt.Errorf("missing field %s", path)
		}
		switch f.Type.Kind() {
		case reflect.Struct:
			sub, ok := v.(map[string]any)
			if !ok {
				return fmt.Errorf("field %s must be an object", path)
			}
			if err := requireFields(f.Type, sub, path+"."); err != nil {
				return err
			}
		case reflect.Slice:
			arr, ok := v.([]any)
			if f.Type.Elem().Kind() == reflect.Uint8 { // json.RawMessage
				continue
			}
			if !ok {
				return fmt.Errorf("field %s must be an array", path)
			}
			if f.Type.Elem().Kind() == reflect.Struct {
				for j, e := range arr {
					sub, ok := e.(map[string]any)
					if !ok {
						return fmt.Errorf("field %s[%d] must be an object", path, j)
					}
					if err := requireFields(f.Type.Elem(), sub, fmt.Sprintf("%s[%d].", path, j)); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

// checker accumulates the first validation error.
type checker struct{ err error }

func (c *checker) fail(format string, a ...any) {
	if c.err == nil {
		c.err = fmt.Errorf(format, a...)
	}
}

func (c *checker) finite(name string, v float64) bool {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		c.fail("%s must be finite, got %v", name, v)
		return false
	}
	return true
}

// between checks the closed range [lo, hi].
func (c *checker) between(name string, v, lo, hi float64) {
	if c.finite(name, v) && (v < lo || v > hi) {
		c.fail("%s must lie in [%v, %v], got %v", name, lo, hi, v)
	}
}

func (c *checker) nonneg(name string, v float64) {
	if c.finite(name, v) && v < 0 {
		c.fail("%s must be >= 0, got %v", name, v)
	}
}

func (c *checker) positive(name string, v float64) {
	if c.finite(name, v) && v <= 0 {
		c.fail("%s must be > 0, got %v", name, v)
	}
}

func (c *checker) prob(name string, v float64) { c.between(name, v, 0, 1) }

// logMs checks a log-millisecond location parameter (exp must stay < ~1 year).
func (c *checker) logMs(name string, v float64) { c.between(name, v, -10, 25) }

func (c *checker) lowHigh(lo, hi string, a, b float64) {
	if c.err == nil && a > b {
		c.fail("%s (%v) must not exceed %s (%v)", lo, a, hi, b)
	}
}

// Validate range-checks the honest model.
func (m *Model) Validate() error {
	c := &checker{}
	if m.SchemaVersion != SchemaVersion {
		c.fail("unsupported schema_version %d", m.SchemaVersion)
	}
	p := m.Person
	c.nonneg("person.theta_sd", p.ThetaSD)
	c.nonneg("person.tau_sd", p.TauSD)
	c.between("person.theta_tau_corr", p.ThetaTauCorr, -0.999, 0.999)
	c.between("person.practice_gain_mean", p.PracticeGainMean, -5, 5)
	c.nonneg("person.practice_gain_sd", p.PracticeGainSD)
	c.prob("person.disengage.p_zero", p.Disengage.PZero)
	c.positive("person.disengage.beta_a", p.Disengage.BetaA)
	c.positive("person.disengage.beta_b", p.Disengage.BetaB)
	c.positive("person.revise.beta_a", p.Revise.BetaA)
	c.positive("person.revise.beta_b", p.Revise.BetaB)
	ip := m.ItemPrior
	c.between("item_prior.b_mean", ip.BMean, -10, 10)
	c.nonneg("item_prior.b_sd", ip.BSD)
	c.between("item_prior.log_a_mean", ip.LogAMean, -5, 5)
	c.nonneg("item_prior.log_a_sd", ip.LogASD)
	c.between("item_prior.c", ip.C, 0, 0.99)
	c.logMs("item_prior.beta_mean", ip.BetaMean)
	c.nonneg("item_prior.beta_sd", ip.BetaSD)
	c.between("item_prior.log_sigma_mean", ip.LogSigmaMean, -10, 3)
	c.nonneg("item_prior.log_sigma_sd", ip.LogSigmaSD)
	if len(ip.DistractorAlpha) != 3 {
		c.fail("item_prior.distractor_alpha must have exactly 3 entries, got %d", len(ip.DistractorAlpha))
	}
	for i, a := range ip.DistractorAlpha {
		c.positive(fmt.Sprintf("item_prior.distractor_alpha[%d]", i), a)
	}
	seen := make(map[string]bool, len(m.Items))
	for i, it := range m.Items {
		name := fmt.Sprintf("items[%d]", i)
		if it.ID == "" || len(it.ID) > 128 || strings.ContainsAny(it.ID, "\"\\\x00\r\n") {
			c.fail("%s.id must be a non-empty string of at most 128 characters without quotes, backslashes or control characters", name)
		}
		if seen[it.ID] {
			c.fail("%s.id %q is duplicated", name, it.ID)
		}
		seen[it.ID] = true
		c.positive(name+".a", it.A)
		c.between(name+".a", it.A, 0, 20)
		c.between(name+".b", it.B, -10, 10)
		c.between(name+".c", it.C, 0, 0.99)
		c.logMs(name+".beta", it.Beta)
		c.positive(name+".sigma", it.Sigma)
		c.between(name+".sigma", it.Sigma, 0, 10)
		if len(it.DistractorWeights) != 3 {
			c.fail("%s.distractor_weights must have exactly 3 entries", name)
			continue
		}
		var sum float64
		for j, w := range it.DistractorWeights {
			c.nonneg(fmt.Sprintf("%s.distractor_weights[%d]", name, j), w)
			sum += w
		}
		if c.err == nil && math.Abs(sum-1) > 1e-6 {
			c.fail("%s.distractor_weights must sum to 1, got %v", name, sum)
		}
	}
	c.logMs("rapid_guess.log_ms_mean", m.RapidGuess.LogMsMean)
	c.nonneg("rapid_guess.log_sd", m.RapidGuess.LogSD)
	r := m.Revision
	c.between("revision.change_mult_wrong", r.ChangeMultWrong, 0, 1000)
	c.between("revision.change_mult_right", r.ChangeMultRight, 0, 1000)
	c.prob("revision.wrong_to_right", r.WrongToRight)
	c.logMs("revision.delay_log_ms_mean", r.DelayLogMsMean)
	c.nonneg("revision.delay_log_sd", r.DelayLogSD)
	c.prob("revision.second_change_prob", r.SecondChangeProb)
	c.logMs("gap.log_ms_mean", m.Gap.LogMsMean)
	c.nonneg("gap.log_sd", m.Gap.LogSD)
	c.logMs("review.log_ms_mean", m.Review.LogMsMean)
	c.nonneg("review.log_sd", m.Review.LogSD)
	if m.MinResponseMs < 1 || m.MinResponseMs > 3600000 {
		c.fail("min_response_ms must lie in [1, 3600000], got %d", m.MinResponseMs)
	}
	for _, sd := range []struct {
		n string
		v float64
	}{{"rapid_guess.log_sd", m.RapidGuess.LogSD}, {"revision.delay_log_sd", r.DelayLogSD}, {"gap.log_sd", m.Gap.LogSD}, {"review.log_sd", m.Review.LogSD}, {"item_prior.beta_sd", ip.BetaSD}, {"item_prior.log_sigma_sd", ip.LogSigmaSD}, {"item_prior.log_a_sd", ip.LogASD}} {
		if sd.v > 5 {
			c.fail("%s must be <= 5, got %v", sd.n, sd.v)
		}
	}
	return c.err
}

// Validate range-checks the mechanism parameters.
func (m *Mechanisms) Validate() error {
	c := &checker{}
	if m.SchemaVersion != SchemaVersion {
		c.fail("unsupported schema_version %d", m.SchemaVersion)
	}
	pk := m.Preknowledge
	c.prob("preknowledge.coverage", pk.Coverage)
	c.prob("preknowledge.recall", pk.Recall)
	c.prob("preknowledge.naive_prob", pk.NaiveProb)
	c.between("preknowledge.speedup_low", pk.SpeedupLow, 1e-6, 10)
	c.between("preknowledge.speedup_high", pk.SpeedupHigh, 1e-6, 10)
	c.lowHigh("preknowledge.speedup_low", "preknowledge.speedup_high", pk.SpeedupLow, pk.SpeedupHigh)
	lk := m.Lookup
	c.prob("lookup.propensity", lk.Propensity)
	c.prob("lookup.success", lk.Success)
	c.logMs("lookup.duration_log_ms_mean", lk.DurationLogMsMean)
	c.between("lookup.duration_log_sd", lk.DurationLogSD, 0, 5)
	c.prob("lookup.verify_mode_prob", lk.VerifyModeProb)
	cp := m.CopyPeer
	c.prob("copy_peer.coverage", cp.Coverage)
	c.logMs("copy_peer.relay_log_ms_mean", cp.RelayLogMsMean)
	c.between("copy_peer.relay_log_sd", cp.RelayLogSD, 0, 5)
	c.logMs("copy_peer.entry_log_ms_mean", cp.EntryLogMsMean)
	c.between("copy_peer.entry_log_sd", cp.EntryLogSD, 0, 5)
	c.logMs("copy_peer.start_lag_log_ms_mean", cp.StartLagLogMsMean)
	c.between("copy_peer.start_lag_log_sd", cp.StartLagLogSD, 0, 5)
	c.prob("copy_peer.follow_change_prob", cp.FollowChangeProb)
	c.prob("copy_peer.stronger_is_source_prob", cp.StrongerIsSourceProb)
	c.logMs("copy_peer.source_overhead_log_ms_mean", cp.SourceOverheadLogMsMean)
	c.between("copy_peer.source_overhead_log_sd", cp.SourceOverheadLogSD, 0, 5)
	c.prob("copy_peer.sync_submit_prob", cp.SyncSubmitProb)
	c.logMs("copy_peer.sync_submit_log_ms_mean", cp.SyncSubmitLogMsMean)
	c.between("copy_peer.sync_submit_log_sd", cp.SyncSubmitLogSD, 0, 5)
	px := m.Proxy
	c.between("proxy.theta_mean", px.ThetaMean, -10, 10)
	c.between("proxy.theta_sd", px.ThetaSD, 0, 10)
	c.between("proxy.min_gain", px.MinGain, 0, 10)
	fi := m.FI
	c.prob("fi.rapid_item_prob", fi.RapidItemProb)
	c.between("fi.rapid_scale_low", fi.RapidScaleLow, 1e-6, 10)
	c.between("fi.rapid_scale_high", fi.RapidScaleHigh, 1e-6, 10)
	c.lowHigh("fi.rapid_scale_low", "fi.rapid_scale_high", fi.RapidScaleLow, fi.RapidScaleHigh)
	if fi.RapidMinMs < 1 || fi.RapidMinMs > 3600000 {
		c.fail("fi.rapid_min_ms must lie in [1, 3600000], got %d", fi.RapidMinMs)
	}
	c.prob("fi.changes_prob_low", fi.ChangesProbLow)
	c.prob("fi.changes_prob_high", fi.ChangesProbHigh)
	c.lowHigh("fi.changes_prob_low", "fi.changes_prob_high", fi.ChangesProbLow, fi.ChangesProbHigh)
	if fi.ChangesMaxCount < 1 || fi.ChangesMaxCount > 50 {
		c.fail("fi.changes_max_count must lie in [1, 50], got %d", fi.ChangesMaxCount)
	}
	c.prob("fi.collusion_forced_wrong_fraction", fi.CollusionForcedWrongFraction)
	c.prob("fi.collusion_copy_prob", fi.CollusionCopyProb)
	c.between("fi.collusion_duration_jitter", fi.CollusionDurationJitter, 0, 0.99)
	if fi.CollusionStartWindowMs < 0 || fi.CollusionStartWindowMs > 86400000 {
		c.fail("fi.collusion_start_window_ms must lie in [0, 86400000], got %d", fi.CollusionStartWindowMs)
	}
	if fi.CollusionSubmitWindowMs < 0 || fi.CollusionSubmitWindowMs > 86400000 {
		c.fail("fi.collusion_submit_window_ms must lie in [0, 86400000], got %d", fi.CollusionSubmitWindowMs)
	}
	c.prob("fi.score_jump_low", fi.ScoreJumpLow)
	c.prob("fi.score_jump_high", fi.ScoreJumpHigh)
	c.lowHigh("fi.score_jump_low", "fi.score_jump_high", fi.ScoreJumpLow, fi.ScoreJumpHigh)
	c.prob("fi.score_prob_cap", fi.ScoreProbCap)
	return c.err
}
