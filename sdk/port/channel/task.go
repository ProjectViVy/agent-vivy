package channel

import (
	"context"
	"time"
)

// task.go declares the OPTIONAL native task contract for Channel Modules
// (design doc: docs/superpowers/specs/2026-10-07-a2a-server-design.md §5).
// A Channel Module serving a task protocol — e.g. the A2A adapter — asserts
// the bound Host against TaskHost and TaskServiceInfoHost instead of
// widening the base Host interface; a Host that does not wire the
// capability simply fails the type assertion.
//
// Identity: all IDs below are opaque product identifiers owned by the Host's
// native session/Run authority, never handles to storage rows. The Host
// validates, deduplicates, persists and authorizes; this package carries
// values only — no HTTP request, authorization token, arbitrary metadata map
// or storage callback may ride on these types. Implementations copy slices
// on ingress and egress.
//
// Authentication is established by a Host-private request binding installed
// by the listener; calls without that binding are rejected even when the
// plugin manufactures message metadata. No public principal setter exists.

// TaskHost is the optional five-method task surface a Channel Host may
// provide. SubmitTask commits before answering; CancelTask reports the
// observed committed state, not an intent to cancel.
type TaskHost interface {
	SubmitTask(context.Context, TaskRequest) (TaskRef, error)
	GetTask(context.Context, TaskQuery) (TaskSnapshot, error)
	ListTasks(context.Context, TaskListQuery) (TaskPage, error)
	CancelTask(context.Context, TaskQuery) (TaskSnapshot, error)
	SubscribeTask(context.Context, TaskSubscription) (TaskStream, error)
}

// TaskServiceInfoHost is the small optional reader for the safe discovery
// view (public identity, endpoint, skills, enabled feature flags). Its only
// consumer is a task-protocol adapter such as A2A; keeping it separate from
// TaskHost prevents card construction from importing Inspect internals.
type TaskServiceInfoHost interface {
	TaskServiceInfo(context.Context) (TaskServiceInfo, error)
}

// TaskTextPart is one text part of a caller or task message.
type TaskTextPart struct{ Text string }

// TaskRequest is a caller message. MessageID is the caller's deduplication
// key. TaskID selects an existing task for ordinary pending-input
// continuation — never a new Run and never an approval. ContextID may be
// empty (allocated) or inferred from TaskID.
type TaskRequest struct {
	MessageID string
	ContextID string
	TaskID    string
	Parts     []TaskTextPart
}

// TaskRef is returned only after commit. Replayed reports a deduplicated
// acceptance: the same MessageID already committed, so the receipt was
// replayed rather than a new task created.
type TaskRef struct {
	TaskID    string
	ContextID string
	Replayed  bool
}

// TaskQuery names one task; ContextID is an optional consistency assertion
// that must match the owned task, and HistoryLimit bounds the projected
// history when set.
type TaskQuery struct {
	TaskID       string
	ContextID    string
	HistoryLimit *int
}

// TaskState is the frozen lifecycle vocabulary. Only completed, failed and
// canceled are terminal.
type TaskState string

const (
	TaskStateSubmitted             TaskState = "submitted"
	TaskStateWorking               TaskState = "working"
	TaskStateInputRequired         TaskState = "input_required"
	TaskStateAuthorizationRequired TaskState = "authorization_required"
	TaskStateCompleted             TaskState = "completed"
	TaskStateFailed                TaskState = "failed"
	TaskStateCanceled              TaskState = "canceled"
)

// TaskMessage is a projected task-local message; Role is "user" or "agent".
type TaskMessage struct {
	ID    string
	Role  string
	Parts []TaskTextPart
}

// TaskStatus carries the projected state, its commit timestamp and an
// optional safe status message.
type TaskStatus struct {
	State     TaskState
	UpdatedAt time.Time
	Message   *TaskMessage
}

// TaskArtifact is a projected task-local artifact.
type TaskArtifact struct {
	ID    string
	Name  string
	Parts []TaskTextPart
}

// TaskSnapshot is the projected task view. Revision is the Host-issued
// projection watermark used for internal convergence, not a wire extension.
// Ref.Replayed is always false here — it describes a send outcome only when
// returned by SubmitTask.
type TaskSnapshot struct {
	Ref       TaskRef
	Status    TaskStatus
	History   []TaskMessage
	Artifacts []TaskArtifact
	Revision  string
}

// TaskListQuery filters by context/state/update time with opaque paging.
type TaskListQuery struct {
	ContextID        string
	State            TaskState
	UpdatedAfter     *time.Time
	PageSize         int
	PageToken        string
	HistoryLimit     *int
	IncludeArtifacts bool
}

// TaskPage is one authorized-scope page. TotalSize counts the authorized
// scope only; NextPageToken is an opaque Host cursor.
type TaskPage struct {
	Tasks         []TaskSnapshot
	TotalSize     int
	PageSize      int
	NextPageToken string
}

// TaskSubscription requests a task stream. After is a Host-issued catch-up
// cursor; wire adapters leave it empty — no remote ownership assertions.
type TaskSubscription struct {
	TaskID string
	After  string
}

// TaskUpdate is one ordered stream record; exactly one variant field is
// nonnil. Artifact is a full replacement, not a diff. No raw event payload
// is exposed.
type TaskUpdate struct {
	Cursor   string
	Snapshot *TaskSnapshot
	Status   *TaskStatus
	Artifact *TaskArtifact
}

// TaskStream delivers a snapshot first when no cursor was supplied, then
// ordered safe updates; it ends with io.EOF after a terminal or interrupted
// state is delivered. Close is idempotent, may interrupt a blocked Next,
// and Next returns io.EOF after Close. Concurrent Next calls are
// unsupported.
type TaskStream interface {
	Next(context.Context) (TaskUpdate, error)
	Close() error
}

// TaskErrorCode is the frozen error vocabulary. Busy and transient
// unavailable errors may set Retryable; malformed input, ownership denial,
// conflicting input and corruption never do.
type TaskErrorCode string

const (
	TaskErrInvalid       TaskErrorCode = "invalid"
	TaskErrDenied        TaskErrorCode = "denied"
	TaskErrNotFound      TaskErrorCode = "not_found"
	TaskErrConflict      TaskErrorCode = "conflict"
	TaskErrBusy          TaskErrorCode = "busy"
	TaskErrUnsupported   TaskErrorCode = "unsupported"
	TaskErrNotCancelable TaskErrorCode = "not_cancelable"
	TaskErrLimit         TaskErrorCode = "limit"
	TaskErrUnavailable   TaskErrorCode = "unavailable"
	TaskErrCorrupt       TaskErrorCode = "corrupt"
	TaskErrCursorInvalid TaskErrorCode = "cursor_invalid"
)

// TaskError is the seam's typed failure. Message is the only wire-safe
// text; diagnostic causes are logged locally with a request correlation ID.
type TaskError struct {
	Code       TaskErrorCode
	Message    string
	Retryable  bool
	RetryAfter time.Duration // zero unless retry is appropriate
}

// Error returns only the safe message.
func (e *TaskError) Error() string { return e.Message }

// TaskServiceInfo is the safe discovery projection built from enabled
// Generation capabilities — never a copy of effective limits or settings.
type TaskServiceInfo struct {
	Name              string
	Description       string
	Version           string
	PublicEndpoint    string
	Streaming         bool
	InputContinuation bool
	Skills            []TaskSkill
}

// TaskSkill describes one public skill on the discovery card; descriptions
// are explicit projections, not raw tool schemas or SKILL.md contents.
type TaskSkill struct {
	ID          string
	Name        string
	Description string
	Tags        []string
	Examples    []string
}
