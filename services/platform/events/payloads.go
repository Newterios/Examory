package events

// Event types produced by the platform core in stage 2.
const (
	TypeUserCreated   = "user.created"
	TypeUserLoggedIn  = "user.logged_in"
	TypeUserLoggedOut = "user.logged_out"
	TypeGroupUpdated  = "group.updated"

	TypeQuizStarted   = "quiz.started"
	TypeQuizAnswered  = "quiz.answered"
	TypeQuizSubmitted = "quiz.submitted"
)

// QuizStarted is the quiz.started v1 payload.
type QuizStarted struct {
	AttemptID     string  `json:"attempt_id"`
	QuizID        string  `json:"quiz_id"`
	VariantID     string  `json:"variant_id"`
	QuestionCount int     `json:"question_count"`
	TimeLimitSec  int     `json:"time_limit_sec"`
	ExamSessionID *string `json:"exam_session_id"`
}

// QuizAnswered is the quiz.answered v1 payload (counting conventions in
// events.md §3).
type QuizAnswered struct {
	AttemptID               string `json:"attempt_id"`
	QuestionID              string `json:"question_id"`
	QuestionIndex           int    `json:"question_index"`
	AnswerHash              string `json:"answer_hash"`
	IsChange                bool   `json:"is_change"`
	TimeSinceShownMs        int64  `json:"time_since_shown_ms"`
	TimeSinceAttemptStartMs int64  `json:"time_since_attempt_start_ms"`
}

// Finalization reasons of quiz.submitted (optional additive field "reason").
const (
	ReasonSubmitted = "submitted"
	ReasonTimedOut  = "timed_out"
)

// QuizSubmitted is the quiz.submitted v1 payload. Reason is an optional
// additive field proposed in LMS-38 (events.md §5 allows additive fields).
type QuizSubmitted struct {
	AttemptID      string `json:"attempt_id"`
	QuizID         string `json:"quiz_id"`
	VariantID      string `json:"variant_id"`
	Score          int    `json:"score"`
	MaxScore       int    `json:"max_score"`
	DurationMs     int64  `json:"duration_ms"`
	AnswersChanged int    `json:"answers_changed"`
	Reason         string `json:"reason,omitempty"`
}

// UserCreated is the user.created v1 payload. It intentionally carries no
// e-mail or name (NFR-9: only necessary identifiers).
type UserCreated struct {
	UserID    string `json:"user_id"`
	Role      string `json:"role"`
	CreatedBy string `json:"created_by"`
}

// UserLoggedIn is the user.logged_in v1 payload.
type UserLoggedIn struct {
	UserID string `json:"user_id"`
	Method string `json:"method"` // "password"
}

// Logout reasons.
const (
	LogoutUser        = "logout"
	LogoutTokenReuse  = "token_reuse"
	LogoutDeactivated = "deactivated"
)

// UserLoggedOut is the user.logged_out v1 payload.
type UserLoggedOut struct {
	UserID string `json:"user_id"`
	Reason string `json:"reason"`
}

// Group change kinds.
const (
	GroupCreated       = "created"
	GroupMemberAdded   = "member_added"
	GroupMemberRemoved = "member_removed"
)

// GroupUpdated is the group.updated v1 payload.
type GroupUpdated struct {
	GroupID      string  `json:"group_id"`
	Name         string  `json:"name"`
	CourseID     *string `json:"course_id"`
	Change       string  `json:"change"`
	MemberUserID *string `json:"member_user_id"`
	MemberCount  int     `json:"member_count"`
}
