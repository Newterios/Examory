// Package events serialises simulated attempts into the Examory event CSV
// (docs/03_architecture/events.md) accepted by ml/anomaly/features.py.
package events

import (
	"bufio"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/binary"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"io"
	"sort"
	"strconv"
	"time"

	"github.com/Newterios/Examory/sim/internal/model"
	"github.com/Newterios/Examory/sim/internal/world"
)

// Columns is the exact CSV header.
var Columns = []string{"event_id", "event_type", "version", "occurred_at", "producer", "actor", "context", "payload", "is_anomaly", "anomaly_type"}

// Fixed identifiers (legacy).
const (
	CourseID  = "course_001"
	QuizID    = "quiz_001"
	VariantID = "variant_001"
	Producer  = "quizzes"
	idPrefix  = "examory-sim"
)

// namespaceURL is the RFC 4122 URL namespace.
var namespaceURL = [16]byte{0x6b, 0xa7, 0xb8, 0x11, 0x9d, 0xad, 0x11, 0xd1, 0x80, 0xb4, 0x00, 0xc0, 0x4f, 0xd4, 0x30, 0xc8}

func formatUUID(b [16]byte) string {
	var dst [36]byte
	hex.Encode(dst[0:8], b[0:4])
	dst[8] = '-'
	hex.Encode(dst[9:13], b[4:6])
	dst[13] = '-'
	hex.Encode(dst[14:18], b[6:8])
	dst[18] = '-'
	hex.Encode(dst[19:23], b[8:10])
	dst[23] = '-'
	hex.Encode(dst[24:], b[10:16])
	return string(dst[:])
}

// UUID5 returns the RFC 4122 version-5 UUID of name in the URL namespace.
func UUID5(name string) string {
	h := sha1.New()
	h.Write(namespaceURL[:])
	h.Write([]byte(name))
	var b [16]byte
	copy(b[:], h.Sum(nil))
	b[6] = (b[6] & 0x0f) | 0x50
	b[8] = (b[8] & 0x3f) | 0x80
	return formatUUID(b)
}

// AttemptID returns the deterministic attempt id.
func AttemptID(seed int64, student, attempt int) string {
	return UUID5(fmt.Sprintf("%s:%d:attempt:%d:%d", idPrefix, seed, student, attempt))
}

// SessionID returns the deterministic exam-session id.
func SessionID(seed int64, student, attempt int) string {
	return UUID5(fmt.Sprintf("%s:%d:exam_session:%d:%d", idPrefix, seed, student, attempt))
}

// EventID returns a deterministic UUIDv7 whose 48-bit timestamp is ms and
// whose 74 random bits come from SHA-256 of the identifying material.
func EventID(seed int64, ms int64, student, attempt int, suffix string) string {
	material := fmt.Sprintf(`[%q,%d,%d,%d,%q]`, idPrefix, seed, student, attempt, suffix)
	h := sha256.Sum256([]byte(material))
	// Low 74 bits of the digest interpreted big-endian (as legacy).
	hi := binary.BigEndian.Uint64(h[16:24])
	lo := binary.BigEndian.Uint64(h[24:32])
	randA := hi & 0x3ff // bits 64..73 of the 74-bit value
	randA = (randA << 2) | (lo >> 62)
	randA &= 0xfff
	randB := lo & ((1 << 62) - 1)
	var b [16]byte
	binary.BigEndian.PutUint64(b[0:8], uint64(ms)<<16|7<<12|randA)
	binary.BigEndian.PutUint64(b[8:16], 2<<62|randB)
	return formatUUID(b)
}

// AnswerHash returns the lowercase hex SHA-256 of {"option":N}.
func AnswerHash(option int) string {
	h := sha256.Sum256([]byte(`{"option":` + strconv.Itoa(option) + `}`))
	return hex.EncodeToString(h[:])
}

var answerHashes = func() [model.NumOptions]string {
	var out [model.NumOptions]string
	for i := range out {
		out[i] = AnswerHash(i)
	}
	return out
}()

// FormatTime formats Unix ms as RFC 3339 UTC with millisecond precision.
func FormatTime(ms int64) string {
	return time.UnixMilli(ms).UTC().Format("2006-01-02T15:04:05.000Z")
}

// Row is one serialised event.
type Row struct {
	Ms      int64
	EventID string
	Fields  []string
}

func jsonString(s string) string { return strconv.Quote(s) } // ids are ASCII without escapes

// Rows serialises every record of one world, sorted by (occurred_at, event_id).
func Rows(seed int64, recs []*world.Record) []Row {
	rows := make([]Row, 0, len(recs)*24)
	for _, r := range recs {
		rows = AppendAttempt(rows, seed, r)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Ms != rows[j].Ms {
			return rows[i].Ms < rows[j].Ms
		}
		return rows[i].EventID < rows[j].EventID
	})
	return rows
}

// AppendAttempt appends the events of one attempt.
func AppendAttempt(rows []Row, seed int64, r *world.Record) []Row {
	a := r.Attempt
	student, k := a.Student, a.Index
	attemptID := AttemptID(seed, student, k)
	actor := `{"user_id":` + jsonString(world.StudentID(student)) + `,"role":"student"}`
	context := `{"course_id":` + jsonString(CourseID) + `,"group_id":` + jsonString(world.GroupID(student)) +
		`,"session_id":` + jsonString(SessionID(seed, student, k)) + `}`
	isAnomaly, kind := "0", "none"
	if r.IsAnomaly {
		isAnomaly, kind = "1", r.AnomalyType
	}
	start := a.Start()
	shown := a.Shown()
	emit := func(elapsed int64, typ, payload, suffix string) {
		ms := start + elapsed
		id := EventID(seed, ms, student, k, suffix)
		rows = append(rows, Row{Ms: ms, EventID: id, Fields: []string{
			id, typ, "1", FormatTime(ms), Producer, actor, context, payload, isAnomaly, kind,
		}})
	}
	for q, res := range a.Results {
		qid := jsonString(a.Items[q].ID)
		for j, e := range res.Events {
			elapsed := shown[q] + e.SinceShown
			payload := `{"attempt_id":"` + attemptID + `","question_id":` + qid +
				`,"question_index":` + strconv.Itoa(q) +
				`,"time_since_shown_ms":` + strconv.FormatInt(e.SinceShown, 10) +
				`,"is_change":` + strconv.FormatBool(e.IsChange) +
				`,"time_since_attempt_start_ms":` + strconv.FormatInt(elapsed, 10) +
				`,"answer_hash":"` + answerHashes[e.Option] + `"}`
			emit(elapsed, "quiz.answered", payload, "answer:"+strconv.Itoa(q)+":"+strconv.Itoa(j))
		}
	}
	dur := a.Duration()
	payload := `{"attempt_id":"` + attemptID + `","quiz_id":"` + QuizID + `","variant_id":"` + VariantID +
		`","score":` + strconv.Itoa(a.Score()) + `,"max_score":` + strconv.Itoa(MaxScore(a)) +
		`,"duration_ms":` + strconv.FormatInt(dur, 10) + `,"answers_changed":` + strconv.Itoa(a.Changes()) + `}`
	emit(dur, "quiz.submitted", payload, "submitted")
	return rows
}

// MaxScore returns the max_score written for an attempt: the number of
// questions, except for an attempt auto-submitted by the quiz window, where it
// is the number of answered questions. The frozen read_attempts (and
// Validate) require max_score to equal the number of questions with events,
// so an unanswered question cannot be represented as a zero-point question;
// score still counts only correct answers saved before the close.
func MaxScore(a *model.Attempt) int {
	if a.AutoSubmit > 0 {
		return a.Answered()
	}
	return len(a.Items)
}

// WriteCSV writes rows with the legacy CSV dialect (CRLF, minimal quoting).
func WriteCSV(w io.Writer, rows []Row) error {
	bw := bufio.NewWriterSize(w, 1<<20)
	cw := csv.NewWriter(bw)
	cw.UseCRLF = true
	if err := cw.Write(Columns); err != nil {
		return err
	}
	for _, r := range rows {
		if err := cw.Write(r.Fields); err != nil {
			return err
		}
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		return err
	}
	return bw.Flush()
}
