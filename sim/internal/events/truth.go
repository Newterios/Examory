package events

import (
	"bufio"
	"encoding/csv"
	"io"
	"math"
	"strconv"

	"github.com/Newterios/Examory/sim/internal/world"
)

// TruthColumns is the header of truth.csv.
var TruthColumns = []string{
	"world", "attempt_id", "user_id", "group_id", "attempt_index", "is_final",
	"slot", "mechanism", "is_anomaly", "role", "partner_user_id", "intensity_used",
	"items_affected", "theta", "tau", "disengage", "revise", "practice_gain",
	"theta_attempt", "mech_flag", "score", "max_score", "answers_changed",
	"duration_ms", "start_offset_ms", "proxy_theta", "sync_submit",
}

// WindowColumns are appended to TruthColumns when the quiz window is enabled.
var WindowColumns = []string{"auto_submitted", "answered_items", "natural_duration_ms", "natural_score"}

func ff(v float64) string {
	if math.IsNaN(v) {
		return ""
	}
	return strconv.FormatFloat(v, 'f', 6, 64)
}

// TruthRow returns the truth.csv fields of one record.
func TruthRow(seed int64, r *world.Record) []string {
	a := r.Attempt
	partner := ""
	if r.Partner >= 0 {
		partner = world.StudentID(r.Partner)
	}
	isAnomaly := "0"
	if r.IsAnomaly {
		isAnomaly = "1"
	}
	row := []string{
		r.World, AttemptID(seed, a.Student, a.Index), world.StudentID(a.Student), world.GroupID(a.Student),
		strconv.Itoa(a.Index), strconv.FormatBool(r.IsFinal), r.Slot, r.AnomalyType, isAnomaly, r.Role, partner,
		ff(r.Intensity), strconv.Itoa(r.ItemsAffected), ff(r.Person.Theta), ff(r.Person.Tau),
		ff(r.Person.Disengage), ff(r.Person.Revise), ff(r.Person.Gain), ff(r.Person.Ability(a.Index)),
		r.Flag, strconv.Itoa(a.Score()), strconv.Itoa(len(a.Items)), strconv.Itoa(a.Changes()),
		strconv.FormatInt(a.Duration(), 10), strconv.FormatInt(a.Offset, 10), ff(r.ProxyTheta), syncCell(r.SyncSubmit),
	}
	if r.Natural != nil {
		auto := "0"
		if a.AutoSubmit > 0 {
			auto = "1"
		}
		row = append(row, auto, strconv.Itoa(a.Answered()),
			strconv.FormatInt(r.Natural.Duration(), 10), strconv.Itoa(r.Natural.Score()))
	}
	return row
}

// WriteTruth writes truth.csv for the given worlds in order.
func WriteTruth(w io.Writer, seed int64, worlds []string, res *world.Result) error {
	bw := bufio.NewWriterSize(w, 1<<20)
	cw := csv.NewWriter(bw)
	cw.UseCRLF = true
	header := TruthColumns
	if res.Window {
		header = append(append([]string(nil), TruthColumns...), WindowColumns...)
	}
	if err := cw.Write(header); err != nil {
		return err
	}
	for _, name := range worlds {
		for _, r := range res.Worlds[name] {
			if err := cw.Write(TruthRow(seed, r)); err != nil {
				return err
			}
		}
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		return err
	}
	return bw.Flush()
}

// KeysOf returns the answer keys of every record, for Validate.
func KeysOf(seed int64, recs []*world.Record) Keys {
	keys := make(Keys, len(recs))
	for _, r := range recs {
		a := r.Attempt
		m := make(map[string]string, len(a.Items))
		for _, it := range a.Items {
			m[it.ID] = answerHashes[it.Key]
		}
		keys[AttemptID(seed, a.Student, a.Index)] = m
	}
	return keys
}

func syncCell(v int) string {
	if v < 0 {
		return ""
	}
	return strconv.Itoa(v)
}
