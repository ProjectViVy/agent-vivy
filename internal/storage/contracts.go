package storage

import (
	"context"
	"errors"
	"time"

	"agent-vivy/internal/domain"
)

// Sentinel errors shared by every backend.
var (
	// ErrVersionConflict is returned by SnapshotStore.Put when the stored
	// version differs from expectVersion (optimistic concurrency).
	ErrVersionConflict = errors.New("storage: snapshot version conflict")
	// ErrRunClosed is returned by Journal.Append once the run has a
	// terminal event (D-008 exactly-one-terminal).
	ErrRunClosed = errors.New("storage: run already has a terminal event")
	// ErrCommitInvalid is returned for commits that violate invariants
	// (empty, or containing more than one terminal event).
	ErrCommitInvalid = errors.New("storage: invalid commit")
	// ErrNotFound is returned when the requested session, message or run
	// does not exist.
	ErrNotFound = errors.New("storage: not found")
)

// Commit is one atomic batch of events for a single run. Events carry no
// Seq: the journal assigns a contiguous monotonic range on append.
type Commit struct {
	RunID  domain.RunID
	Events []domain.RunEvent
}

// Entry is one replayed journal record.
type Entry struct {
	Event domain.RunEvent
}

// Iterator walks replayed entries. Next advances and reports availability;
// Value returns the current entry; Err surfaces iteration failures; Close
// releases backend resources.
type Iterator[T any] interface {
	Next() bool
	Value() T
	Err() error
	Close() error
}

// Journal is the durable, append-only, ordered event log. Product history
// lives here, never in engine checkpoint bytes (D-028).
type Journal interface {
	// Append stores the commit atomically and returns the last assigned
	// seq. It rejects any append once the run holds a terminal event, and
	// rejects commits containing more than one terminal event (D-008).
	Append(ctx context.Context, commit Commit) (domain.EventSeq, error)
	// Replay streams the run's events with seq > after, in order.
	Replay(ctx context.Context, runID domain.RunID, after domain.EventSeq) (Iterator[Entry], error)
}

// SnapshotStore holds the latest consistent domain state per key. Version
// starts at 0 for absent keys.
type SnapshotStore interface {
	// Get returns the value and current version; absent keys yield
	// (nil, 0, nil).
	Get(ctx context.Context, key string) ([]byte, int64, error)
	// Put replaces the value only when the stored version equals
	// expectVersion, otherwise ErrVersionConflict.
	Put(ctx context.Context, key string, value []byte, expectVersion int64) error
}

// BlobStore stores opaque blobs (e.g. Eino checkpoint bytes) by id.
// Writes are generation-based (D-030): same-id writes append a new
// generation and flip the current pointer atomically; an existing
// generation is never mutated in place.
type BlobStore interface {
	Get(ctx context.Context, id string) ([]byte, bool, error)
	Put(ctx context.Context, id string, data []byte) error
	Delete(ctx context.Context, id string) error
}

// LeaseStore serializes exclusive work (session run lock, recovery).
type LeaseStore interface {
	// Acquire grants the lease when free or expired; acquired=false means
	// another owner holds it.
	Acquire(ctx context.Context, key, owner string, ttl time.Duration) (bool, error)
	// Release drops the lease only if the caller still owns it.
	Release(ctx context.Context, key, owner string) error
}

// SessionStore persists conversations. Deleting a session removes its
// messages, runs and journal events in one transaction.
type SessionStore interface {
	CreateSession(ctx context.Context, s domain.Session) error
	// ListSessions returns all sessions, newest first.
	ListSessions(ctx context.Context) ([]domain.Session, error)
	GetSession(ctx context.Context, id domain.SessionID) (domain.Session, error)
	RenameSession(ctx context.Context, id domain.SessionID, title string) error
	DeleteSession(ctx context.Context, id domain.SessionID) error
}

// MessageStore persists the append-only conversation turns (FR-2).
type MessageStore interface {
	AppendMessage(ctx context.Context, m domain.Message) error
	// ListMessages returns the session's messages in creation order.
	ListMessages(ctx context.Context, sessionID domain.SessionID) ([]domain.Message, error)
}

// NoteStore persists the user's notebook (MA-3). Notes are append-only;
// listing serves the newest entries first so digests stay bounded at the
// head.
type NoteStore interface {
	AppendNote(ctx context.Context, n domain.Note) error
	// ListNotes returns the notebook newest-first.
	ListNotes(ctx context.Context) ([]domain.Note, error)
	// GetNote fetches one entry by id; unknown ids yield ErrNotFound.
	GetNote(ctx context.Context, id string) (domain.Note, error)
}

// ErrConflict is returned when a first-writer-wins studio write loses.
var ErrConflict = errors.New("storage: conflict")

// StudioStore persists Generation / EvalRun / Promotion rows and the
// append-only studio event log. These are not run journal events.
type StudioStore interface {
	CreateGeneration(ctx context.Context, g domain.Generation) error
	GetGeneration(ctx context.Context, id string) (domain.Generation, error)
	ListGenerations(ctx context.Context) ([]domain.Generation, error)
	UpdateGenerationPhase(ctx context.Context, id string, phase domain.GenerationPhase) error

	CreateEvalRun(ctx context.Context, e domain.EvalRun) error
	GetEvalRun(ctx context.Context, id string) (domain.EvalRun, error)
	ListEvalRuns(ctx context.Context) ([]domain.EvalRun, error)
	ListEvalRunsFor(ctx context.Context, generationID string) ([]domain.EvalRun, error)

	CreatePromotion(ctx context.Context, p domain.Promotion) error
	ListPromotions(ctx context.Context) ([]domain.Promotion, error)
	ListPromotionsFrom(ctx context.Context, fromID string) ([]domain.Promotion, error)

	AppendStudioEvent(ctx context.Context, ev domain.StudioEvent) (int64, error)
	ListStudioEvents(ctx context.Context) ([]domain.StudioEvent, error)
}

// RunStore tracks run lifecycle rows. Status transitions themselves are
// validated by the domain state machine; the store only persists them.
type RunStore interface {
	CreateRun(ctx context.Context, r domain.Run) error
	GetRun(ctx context.Context, id domain.RunID) (domain.Run, error)
	SetRunStatus(ctx context.Context, id domain.RunID, status domain.RunStatus) error
	// ListActiveRuns enumerates non-terminal runs (restart recovery, E2).
	ListActiveRuns(ctx context.Context) ([]domain.Run, error)
	// ListChildRuns returns direct children in creation order.
	ListChildRuns(ctx context.Context, parentID domain.RunID) ([]domain.Run, error)
	// ListRunTree returns all descendants of a root in creation order.
	ListRunTree(ctx context.Context, rootID domain.RunID) ([]domain.Run, error)
}

// ApprovalStore persists server-side approval decisions for effectful
// tool calls (D-009, FR-6). Rows are created pending and settle exactly
// once: DecideApproval is first-writer-wins.
type ApprovalStore interface {
	CreateApproval(ctx context.Context, a domain.Approval) error
	// GetApproval returns the row; absent ids yield ErrNotFound.
	GetApproval(ctx context.Context, id string) (domain.Approval, error)
	// ListPendingApprovals returns pending rows, latest expiry first.
	ListPendingApprovals(ctx context.Context) ([]domain.Approval, error)
	// DecideApproval settles a pending row. decided=false means no
	// pending row matched (already decided, or unknown id); the first
	// writer wins.
	DecideApproval(ctx context.Context, id, decision string) (bool, error)
}

// ApprovalLifecycleStore is an optional extension implemented by durable
// backends. Keeping it separate preserves compatibility with small test or
// embedding stores that only implement the original approval contract.
type ApprovalLifecycleStore interface {
	DecideApprovalWithMetadata(context.Context, string, string, string, string) (bool, error)
	ExpireApproval(context.Context, string, string) (bool, error)
	CancelApproval(context.Context, string, string, string) (bool, error)
	MarkApprovalStale(context.Context, string, string) (bool, error)
}

// QuestionStore persists ask_user suspensions separately from approvals.
// Answers resume the interrupted run but never authorize a side effect.
type QuestionStore interface {
	CreateQuestion(ctx context.Context, q domain.Question) error
	GetQuestion(ctx context.Context, id string) (domain.Question, error)
	ListPendingQuestions(ctx context.Context) ([]domain.Question, error)
	AnswerQuestion(ctx context.Context, id, answer string) (bool, error)
	CancelQuestion(ctx context.Context, id string) error
}

// QuestionLifecycleStore carries reviewer metadata and durable expiry while
// retaining QuestionStore's compatibility methods.
type QuestionLifecycleStore interface {
	AnswerQuestionWithMetadata(context.Context, string, string, string, string) (bool, error)
	CancelQuestionWithMetadata(context.Context, string, string, string) (bool, error)
	ExpireQuestion(context.Context, string, string) (bool, error)
}

// ReviewFilter selects the unified ReviewItem projection. Zero values mean
// all kinds/statuses; callers that need the live queue pass pending.
type ReviewFilter struct {
	Kind      domain.ReviewKind
	Status    domain.ReviewStatus
	SessionID domain.SessionID
	Limit     int
}

// ReviewStore serves the durable read model consumed by Review Center.
type ReviewStore interface {
	ListReviews(context.Context, ReviewFilter) ([]domain.ReviewItem, error)
	GetReview(context.Context, string) (domain.ReviewItem, error)
}

// SkillRevisionStore persists staged Skill mutations. A revision is created
// before HITL approval and transitions exactly once to an applied/rejected
// terminal status.
type SkillRevisionStore interface {
	CreateSkillRevision(context.Context, domain.SkillRevision) error
	GetSkillRevision(context.Context, string) (domain.SkillRevision, error)
	ListPendingSkillRevisions(context.Context) ([]domain.SkillRevision, error)
	SetSkillRevisionStatus(context.Context, string, domain.SkillRevisionStatus, int64) error
}

// TodoStore persists the session-scoped plantask projection.
type TodoStore interface {
	CreateTodo(context.Context, domain.Todo) error
	GetTodo(context.Context, domain.SessionID, string) (domain.Todo, error)
	ListTodos(context.Context, domain.SessionID) ([]domain.Todo, error)
	UpdateTodo(context.Context, domain.Todo) error
}
