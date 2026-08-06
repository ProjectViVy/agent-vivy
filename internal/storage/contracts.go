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

// RunStore tracks run lifecycle rows. Status transitions themselves are
// validated by the domain state machine; the store only persists them.
type RunStore interface {
	CreateRun(ctx context.Context, r domain.Run) error
	GetRun(ctx context.Context, id domain.RunID) (domain.Run, error)
	SetRunStatus(ctx context.Context, id domain.RunID, status domain.RunStatus) error
	// ListActiveRuns enumerates non-terminal runs (restart recovery, E2).
	ListActiveRuns(ctx context.Context) ([]domain.Run, error)
}
