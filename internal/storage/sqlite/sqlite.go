// Package sqlite is the V0 reference backend for the Vivy storage
// contracts (D-026, D-031), built on modernc.org/sqlite (pure Go, no
// CGO). SQLite specifics never leak above the contracts (D-027).
package sqlite

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"sync"
	"time"

	_ "modernc.org/sqlite"

	"agent-vivy/internal/storage"
)

const (
	organismLeaseKey = "vivy/organism"
	leaseTTL         = 30 * time.Second
	leaseHeartbeat   = 10 * time.Second
)

// migrations apply in order; each runs inside its own transaction and
// records itself in schema_migrations. Failure aborts startup (FR-8).
var migrations = []struct {
	version int64
	sql     string
}{
	{1, migration001},
	{2, migration002},
	{3, migration003},
	{4, migration004},
	{5, migration005},
	{6, migration006},
	{7, migration007},
	{8, migration008},
	{9, migration009},
	{10, migration010},
	{11, migration011},
	{12, migration012},
	{13, migration013},
	{14, migration014},
	{15, migration015},
	{16, migration016},
	{17, migration017},
	{18, migration018},
	{19, migration019},
	{20, migration020},
	{21, migration021},
	{22, migration022},
}

// Open opens (or creates) the database at path and applies all pending
// migrations. The returned Backend implements the four storage contracts.
func Open(ctx context.Context, path string) (*Backend, error) {
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		return nil, fmt.Errorf("storage: open sqlite %s: %w", path, err)
	}
	// Single writer keeps SQLite contention trivial without leaking WAL or
	// locking knobs through the contracts (D-027).
	db.SetMaxOpenConns(1)

	b := &Backend{db: db}
	if err := b.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return b, nil
}

// Backend bundles the application contracts over one database handle. Snapshot
// and blob accessors are separate handles because their Get/Put signatures
// differ; all of them share the same underlying database.
type Backend struct {
	db         *sql.DB
	leaseOwner string
	stopLease  context.CancelFunc
	leaseDone  chan struct{}
	closeOnce  sync.Once
}

// Compile-time proof that every contract is satisfied.
var (
	_ storage.Journal            = (*Backend)(nil)
	_ storage.LeaseStore         = (*Backend)(nil)
	_ storage.ApprovalStore      = (*Backend)(nil)
	_ storage.QuestionStore      = (*Backend)(nil)
	_ storage.SkillRevisionStore = (*Backend)(nil)
	_ storage.TodoStore          = (*Backend)(nil)
	_ storage.ReviewStore        = (*Backend)(nil)
	_ storage.NoteStore          = (*Backend)(nil)
	_ storage.StudioStore        = (*Backend)(nil)
	_ storage.SnapshotStore      = (*Snapshot)(nil)
	_ storage.BlobStore          = (*Blobs)(nil)
	_ storage.Engine             = (*Backend)(nil)
	_ storage.CheckpointOrphaner = (*Backend)(nil)
)

// Snapshot returns the snapshot handle over this database.
func (b *Backend) Snapshot() storage.SnapshotStore { return &Snapshot{db: b.db} }

// Blobs returns the blob (checkpoint) handle over this database.
func (b *Backend) Blobs() storage.BlobStore { return &Blobs{db: b.db} }

// TakeOrganismLease claims the shared-workspace exclusive lease. A second
// process on the same Journal returns storage.ErrLeaseHeld. Tests that
// call Open without this remain concurrent-safe on distinct files.
func (b *Backend) TakeOrganismLease(ctx context.Context) error {
	owner, err := instanceOwner()
	if err != nil {
		return err
	}
	ok, err := b.Acquire(ctx, organismLeaseKey, owner, leaseTTL)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w: the shared Vivy workspace is already in use by another process", storage.ErrLeaseHeld)
	}
	b.leaseOwner = owner
	leaseCtx, cancel := context.WithCancel(context.Background())
	b.stopLease = cancel
	b.leaseDone = make(chan struct{})
	go b.heartbeat(leaseCtx)
	return nil
}

func instanceOwner() (string, error) {
	host, _ := os.Hostname()
	var nonce [8]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", fmt.Errorf("storage: lease nonce: %w", err)
	}
	return fmt.Sprintf("%s:%d:%s", host, os.Getpid(), hex.EncodeToString(nonce[:])), nil
}

func (b *Backend) heartbeat(ctx context.Context) {
	defer close(b.leaseDone)
	ticker := time.NewTicker(leaseHeartbeat)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ok, err := b.Acquire(ctx, organismLeaseKey, b.leaseOwner, leaseTTL)
			if err != nil || !ok {
				return
			}
		}
	}
}

// Close releases the organism lease (when taken) and the database handle.
func (b *Backend) Close() error {
	var err error
	b.closeOnce.Do(func() {
		if b.stopLease != nil {
			b.stopLease()
		}
		if b.leaseDone != nil {
			select {
			case <-b.leaseDone:
			case <-time.After(leaseHeartbeat + time.Second):
			}
		}
		if b.leaseOwner != "" {
			_ = b.Release(context.Background(), organismLeaseKey, b.leaseOwner)
		}
		err = b.db.Close()
	})
	return err
}

// DropCheckpointPointer is the CN-11 crash-shape hook.
func (b *Backend) DropCheckpointPointer(ctx context.Context, id string) error {
	_, err := b.db.ExecContext(ctx, `DELETE FROM checkpoints WHERE id = ?`, id)
	return err
}

// CountCheckpointGenerations is the CN-11 crash-shape hook.
func (b *Backend) CountCheckpointGenerations(ctx context.Context, id string) (int, error) {
	var n int
	err := b.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM checkpoint_generations WHERE id = ?`, id).Scan(&n)
	return n, err
}

// DeleteCheckpointGenerations is the CN-11 crash-shape hook.
func (b *Backend) DeleteCheckpointGenerations(ctx context.Context, id string) error {
	_, err := b.db.ExecContext(ctx, `DELETE FROM checkpoint_generations WHERE id = ?`, id)
	return err
}

func (b *Backend) migrate(ctx context.Context) error {
	if _, err := b.db.ExecContext(ctx,
		`CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at INTEGER NOT NULL
		)`); err != nil {
		return fmt.Errorf("storage: init schema_migrations: %w", err)
	}

	for _, m := range migrations {
		var n int
		if err := b.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM schema_migrations WHERE version = ?`, m.version).Scan(&n); err != nil {
			return fmt.Errorf("storage: check migration %d: %w", m.version, err)
		}
		if n > 0 {
			continue
		}

		tx, err := b.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("storage: begin migration %d: %w", m.version, err)
		}
		if _, err := tx.ExecContext(ctx, m.sql); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("storage: apply migration %d: %w", m.version, err)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`,
			m.version, time.Now().UnixMilli()); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("storage: record migration %d: %w", m.version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("storage: commit migration %d: %w", m.version, err)
		}
	}
	return nil
}

// migration001 creates the full V0 schema (§5.2). snapshots extends the
// sketch: SnapshotStore is one of the four contracts (D-026).
const migration001 = `
CREATE TABLE sessions (
	id TEXT PRIMARY KEY,
	title TEXT NOT NULL,
	created_at INTEGER NOT NULL
);

CREATE TABLE messages (
	id TEXT PRIMARY KEY,
	session_id TEXT NOT NULL,
	run_id TEXT NOT NULL DEFAULT '',
	role TEXT NOT NULL,
	created_at INTEGER NOT NULL,
	content BLOB NOT NULL,
	FOREIGN KEY(session_id) REFERENCES sessions(id)
);

CREATE TABLE runs (
	id TEXT PRIMARY KEY,
	session_id TEXT NOT NULL,
	status TEXT NOT NULL,
	created_at INTEGER NOT NULL,
	FOREIGN KEY(session_id) REFERENCES sessions(id)
);

CREATE TABLE run_events (
	run_id TEXT NOT NULL,
	seq INTEGER NOT NULL,
	type TEXT NOT NULL,
	created_at INTEGER NOT NULL,
	payload_version INTEGER NOT NULL,
	payload BLOB NOT NULL,
	PRIMARY KEY(run_id, seq)
);

CREATE TABLE approvals (
	id TEXT PRIMARY KEY,
	run_id TEXT NOT NULL,
	tool_call_id TEXT NOT NULL,
	decision TEXT NOT NULL,
	expires_at INTEGER NOT NULL
);

CREATE TABLE snapshots (
	key TEXT PRIMARY KEY,
	value BLOB NOT NULL,
	version INTEGER NOT NULL
);

CREATE TABLE checkpoints (
	id TEXT PRIMARY KEY,
	generation INTEGER NOT NULL,
	checksum TEXT NOT NULL,
	created_at INTEGER NOT NULL
);

CREATE TABLE checkpoint_generations (
	id TEXT NOT NULL,
	generation INTEGER NOT NULL,
	blob BLOB NOT NULL,
	PRIMARY KEY(id, generation)
);

CREATE TABLE leases (
	key TEXT PRIMARY KEY,
	owner TEXT NOT NULL,
	expires_at INTEGER NOT NULL
);
`

// migration002 extends approvals with the eino resume target so a
// decision can feed ResumeWithParams without re-deriving it (C6).
const migration002 = `
ALTER TABLE approvals ADD COLUMN resume_target TEXT NOT NULL DEFAULT '';
`

// migration003 adds the notebook (MA-3): write_note persists here and
// the read-only notes tools list/read from it.
const migration003 = `
CREATE TABLE notes (
	id TEXT PRIMARY KEY,
	content BLOB NOT NULL,
	created_at INTEGER NOT NULL
);
`

// migration004 adds durable ask_user interactions without changing the
// approval schema or conflating the two suspension types.
const migration004 = `
CREATE TABLE questions (
	id TEXT PRIMARY KEY,
	run_id TEXT NOT NULL,
	tool_call_id TEXT NOT NULL,
	prompt BLOB NOT NULL,
	answer BLOB NOT NULL DEFAULT '',
	status TEXT NOT NULL,
	expires_at INTEGER NOT NULL,
	resume_target TEXT NOT NULL DEFAULT '',
	FOREIGN KEY(run_id) REFERENCES runs(id)
);
`

// migration005 adds the durable parent/child run tree and identifies child
// approvals without changing the existing approval decision semantics.
const migration005 = `
ALTER TABLE runs ADD COLUMN kind TEXT NOT NULL DEFAULT 'primary';
ALTER TABLE runs ADD COLUMN parent_run_id TEXT NOT NULL DEFAULT '';
ALTER TABLE runs ADD COLUMN root_run_id TEXT NOT NULL DEFAULT '';
ALTER TABLE runs ADD COLUMN depth INTEGER NOT NULL DEFAULT 0;
UPDATE runs SET root_run_id = id WHERE root_run_id = '';
CREATE INDEX runs_parent_idx ON runs(parent_run_id, created_at, id);
CREATE INDEX runs_root_idx ON runs(root_run_id, created_at, id);
ALTER TABLE approvals ADD COLUMN kind TEXT NOT NULL DEFAULT 'run';
`

// migration006 adds restart-safe staged Skill mutations. The payload remains
// backend-owned JSON so storage does not become coupled to Skill semantics.
const migration006 = `
CREATE TABLE skill_revisions (
	 id TEXT PRIMARY KEY,
	 run_id TEXT NOT NULL,
	 skill_name TEXT NOT NULL,
	 action TEXT NOT NULL,
	 target_path TEXT NOT NULL,
	 payload BLOB NOT NULL,
	 base_hash TEXT NOT NULL,
	 content_hash TEXT NOT NULL,
	 preview BLOB NOT NULL,
	 warnings_json BLOB NOT NULL,
	 status TEXT NOT NULL,
	 created_at INTEGER NOT NULL,
	 applied_at INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX skill_revisions_status_idx ON skill_revisions(status, created_at, id);
`

// migration007 extends approvals with the reviewable mutation proposal. All
// columns have safe defaults so existing pending rows remain recoverable.
const migration007 = `
ALTER TABLE approvals ADD COLUMN action TEXT NOT NULL DEFAULT '';
ALTER TABLE approvals ADD COLUMN target TEXT NOT NULL DEFAULT '';
ALTER TABLE approvals ADD COLUMN precondition_hash TEXT NOT NULL DEFAULT '';
ALTER TABLE approvals ADD COLUMN preview BLOB NOT NULL DEFAULT '';
ALTER TABLE approvals ADD COLUMN risk_findings_json BLOB NOT NULL DEFAULT '[]';
ALTER TABLE approvals ADD COLUMN proposal_data BLOB NOT NULL DEFAULT '';
`

// migration008 adds the session-scoped durable plantask projection.
const migration008 = `
CREATE TABLE todos (
	 id TEXT NOT NULL,
	 session_id TEXT NOT NULL,
	 subject TEXT NOT NULL,
	 description TEXT NOT NULL,
	 status TEXT NOT NULL,
	 blocks_json BLOB NOT NULL,
	 blocked_by_json BLOB NOT NULL,
	 active_form TEXT NOT NULL,
	 owner TEXT NOT NULL,
	 metadata_json BLOB NOT NULL,
	 position INTEGER NOT NULL,
	 created_at INTEGER NOT NULL,
	 updated_at INTEGER NOT NULL,
	 PRIMARY KEY(session_id, id),
	 FOREIGN KEY(session_id) REFERENCES sessions(id)
);
CREATE INDEX todos_session_position_idx ON todos(session_id, position, id);
`

// migration009 stores the exact pre-mutation bytes needed for an approved
// Skill rollback. Older revisions remain non-rollbackable by design.
const migration009 = `
ALTER TABLE skill_revisions ADD COLUMN before_payload BLOB NOT NULL DEFAULT '';
`

// migration010 makes human interaction rows self-describing and queryable
// after restart. Defaults keep all pre-Review-Center rows readable.
const migration010 = `
ALTER TABLE approvals ADD COLUMN tool_name TEXT NOT NULL DEFAULT '';
ALTER TABLE approvals ADD COLUMN created_at INTEGER NOT NULL DEFAULT 0;
ALTER TABLE approvals ADD COLUMN decided_at INTEGER NOT NULL DEFAULT 0;
ALTER TABLE approvals ADD COLUMN actor TEXT NOT NULL DEFAULT '';
ALTER TABLE approvals ADD COLUMN decision_reason BLOB NOT NULL DEFAULT '';
ALTER TABLE approvals ADD COLUMN stale_reason BLOB NOT NULL DEFAULT '';
ALTER TABLE questions ADD COLUMN created_at INTEGER NOT NULL DEFAULT 0;
ALTER TABLE questions ADD COLUMN answered_at INTEGER NOT NULL DEFAULT 0;
ALTER TABLE questions ADD COLUMN actor TEXT NOT NULL DEFAULT '';
ALTER TABLE questions ADD COLUMN decision_reason BLOB NOT NULL DEFAULT '';
CREATE INDEX approvals_status_expiry_idx ON approvals(decision, expires_at, id);
CREATE INDEX questions_status_expiry_idx ON questions(status, expires_at, id);
`

// migration011 projects model-visible tool turns onto the message log
// (ADR-010). Empty defaults keep existing text rows valid.
const migration011 = `
ALTER TABLE messages ADD COLUMN tool_call_id TEXT NOT NULL DEFAULT '';
ALTER TABLE messages ADD COLUMN tool_name TEXT NOT NULL DEFAULT '';
ALTER TABLE messages ADD COLUMN tool_args BLOB NOT NULL DEFAULT '';
`

// migration012 is the studio object plane (ADR-011). Studio events are
// not run journal rows.
const migration012 = `
CREATE TABLE generations (
	id TEXT PRIMARY KEY,
	parent_id TEXT NOT NULL DEFAULT '',
	artifact_sha256 TEXT NOT NULL,
	source_ref TEXT NOT NULL DEFAULT '',
	recipe_json BLOB NOT NULL,
	phase TEXT NOT NULL,
	created_at INTEGER NOT NULL
);
CREATE TABLE eval_runs (
	id TEXT PRIMARY KEY,
	candidate_id TEXT NOT NULL,
	baseline_id TEXT NOT NULL DEFAULT '',
	suite TEXT NOT NULL,
	verdict TEXT NOT NULL,
	journal_ref TEXT NOT NULL DEFAULT '',
	created_at INTEGER NOT NULL,
	FOREIGN KEY(candidate_id) REFERENCES generations(id)
);
CREATE INDEX eval_runs_candidate_idx ON eval_runs(candidate_id, created_at, id);
CREATE TABLE promotions (
	id TEXT PRIMARY KEY,
	from_id TEXT NOT NULL,
	to_id TEXT NOT NULL,
	eval_id TEXT NOT NULL,
	actor TEXT NOT NULL,
	phase TEXT NOT NULL,
	applies_at TEXT NOT NULL,
	created_at INTEGER NOT NULL,
	FOREIGN KEY(from_id) REFERENCES generations(id),
	FOREIGN KEY(to_id) REFERENCES generations(id)
);
CREATE UNIQUE INDEX promotions_from_accepted_idx ON promotions(from_id) WHERE phase = 'accepted';
CREATE TABLE studio_events (
	seq INTEGER PRIMARY KEY AUTOINCREMENT,
	type TEXT NOT NULL,
	object_id TEXT NOT NULL,
	created_at INTEGER NOT NULL,
	payload BLOB NOT NULL
);
`

// migration013 adds sandbox mode and approval policy columns for
// configurable permission boundaries (D-021).
const migration013 = `
ALTER TABLE approvals ADD COLUMN sandbox_mode TEXT NOT NULL DEFAULT 'workspace_write';
ALTER TABLE approvals ADD COLUMN approval_policy TEXT NOT NULL DEFAULT 'ask';
ALTER TABLE approvals ADD COLUMN timeout_at INTEGER NOT NULL DEFAULT 0;
ALTER TABLE sessions ADD COLUMN sandbox_mode TEXT NOT NULL DEFAULT 'workspace_write';
ALTER TABLE sessions ADD COLUMN approval_policy TEXT NOT NULL DEFAULT 'ask';
`

// migration014 adds an index on run_events(type, created_at) to support
// cross-run token usage queries without full table scans.
const migration014 = `
CREATE INDEX IF NOT EXISTS run_events_type_created_idx ON run_events(type, created_at);
`

// migration015 adds the session-level context-compaction projection
// (context/compact + durable summary folding in the feed builder). The
// journal remains the append-only source of truth; this table only records
// which stored rows were folded into a summary and what replaces them.
const migration015 = `
CREATE TABLE session_compactions (
	session_id TEXT NOT NULL,
	run_id TEXT NOT NULL,
	summary BLOB NOT NULL,
	tail_from INTEGER NOT NULL,
	dropped_count INTEGER NOT NULL,
	created_at INTEGER NOT NULL,
	PRIMARY KEY(session_id, created_at, run_id),
	FOREIGN KEY(session_id) REFERENCES sessions(id)
);
CREATE INDEX session_compactions_session_idx ON session_compactions(session_id, created_at DESC);
`

// migration016 projects channel provenance onto the message log and adds
// the control plane's scheduled jobs. Schedule and payload stay backend-
// owned JSON columns so the domain shapes can evolve without schema churn.
const migration016 = `
	ALTER TABLE messages ADD COLUMN source TEXT NOT NULL DEFAULT '';
	ALTER TABLE messages ADD COLUMN channel TEXT NOT NULL DEFAULT '';
	ALTER TABLE messages ADD COLUMN chat_id TEXT NOT NULL DEFAULT '';
	ALTER TABLE messages ADD COLUMN channel_message_id TEXT NOT NULL DEFAULT '';
	CREATE TABLE cron_jobs (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		enabled INTEGER NOT NULL,
		schedule_json BLOB NOT NULL,
		payload_json BLOB NOT NULL,
		session_id TEXT NOT NULL DEFAULT '',
		next_run_at_ms INTEGER NOT NULL DEFAULT 0,
		last_run_at_ms INTEGER NOT NULL DEFAULT 0,
		last_status TEXT NOT NULL DEFAULT '',
		last_error TEXT NOT NULL DEFAULT '',
		delete_after_run INTEGER NOT NULL DEFAULT 0,
		created_at_ms INTEGER NOT NULL,
		updated_at_ms INTEGER NOT NULL
	);
	CREATE INDEX cron_jobs_next_run_idx ON cron_jobs(enabled, next_run_at_ms);
`

// migration017 repairs databases that recorded migration016 before the
// channel/cron branches were reconciled. Those databases already have the
// version-16 marker but may not have received cron_jobs at all. IF NOT EXISTS
// keeps the repair safe for databases that got the merged migration016.
const migration017 = `
	CREATE TABLE IF NOT EXISTS cron_jobs (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		enabled INTEGER NOT NULL,
		schedule_json BLOB NOT NULL,
		payload_json BLOB NOT NULL,
		session_id TEXT NOT NULL DEFAULT '',
		next_run_at_ms INTEGER NOT NULL DEFAULT 0,
		last_run_at_ms INTEGER NOT NULL DEFAULT 0,
		last_status TEXT NOT NULL DEFAULT '',
		last_error TEXT NOT NULL DEFAULT '',
		delete_after_run INTEGER NOT NULL DEFAULT 0,
		created_at_ms INTEGER NOT NULL,
		updated_at_ms INTEGER NOT NULL
	);
	CREATE INDEX IF NOT EXISTS cron_jobs_next_run_idx ON cron_jobs(enabled, next_run_at_ms);
`

// migration018 adds message_attachments: raw image bytes persisted next to
// their user message row (VC-1g-2). IF NOT EXISTS keeps the migration safe
// to re-run on databases that already carry the table.
const migration018 = `
	CREATE TABLE IF NOT EXISTS message_attachments (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		message_id TEXT NOT NULL,
		position INTEGER NOT NULL,
		name TEXT NOT NULL DEFAULT '',
		mime_type TEXT NOT NULL DEFAULT '',
		data BLOB NOT NULL
	);
	CREATE INDEX IF NOT EXISTS message_attachments_message_idx ON message_attachments(message_id);
`

// migration019 adds the session file-version chain (RB-1 record side):
// pre/post-mutation snapshots per (session, path) with hash dedupe and a
// 20-version retention, plus the file_reads marker table behind the
// stale-read guard. Restore consumers stay deferred (RB-L2-DEFER). No
// foreign keys: session deletion cascades through explicit DELETEs (the
// compactions FK showed the delete-order hazard that path creates).
const migration019 = `
	CREATE TABLE IF NOT EXISTS file_versions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id TEXT NOT NULL,
		run_id TEXT NOT NULL DEFAULT '',
		path TEXT NOT NULL,
		version INTEGER NOT NULL,
		content_hash TEXT NOT NULL,
		content BLOB NOT NULL,
		created_at INTEGER NOT NULL,
		UNIQUE(session_id, path, version)
	);
	CREATE INDEX IF NOT EXISTS file_versions_session_path_idx ON file_versions(session_id, path, version DESC);
	CREATE TABLE IF NOT EXISTS file_reads (
		session_id TEXT NOT NULL,
		path TEXT NOT NULL,
		read_at INTEGER NOT NULL,
		PRIMARY KEY(session_id, path)
	);
`

// migration020 adds the session truncation markers behind logical
// rewind/edit/fork (JOURNAL-REWIND-AND-FORK): one marker row per user
// action, newest wins; messages and run_events stay append-only and are
// filtered at read time. The fold hides the closed id range
// [cutoff_message_id, tail_message_id] — tail is the session's last message
// at marker time, so turns appended after a rewind stay visible. No foreign
// keys, same delete-order rationale as migration019 — session deletion
// cascades through explicit DELETEs.
const migration020 = `
	CREATE TABLE IF NOT EXISTS session_truncations (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id TEXT NOT NULL,
		cutoff_message_id TEXT NOT NULL,
		tail_message_id TEXT NOT NULL DEFAULT '',
		reason TEXT NOT NULL,
		fork_session_id TEXT NOT NULL DEFAULT '',
		created_at INTEGER NOT NULL
	);
	CREATE INDEX IF NOT EXISTS session_truncations_session_idx ON session_truncations(session_id, id DESC);
`

// migration021 adds independent durable project-file snapshots next to each
// message. The body is the bounded text captured at turn acceptance; RPC
// history projections expose only path/name/size metadata. No foreign key is
// used so the existing explicit session-delete order remains safe.
const migration021 = `
	CREATE TABLE IF NOT EXISTS message_file_contexts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		message_id TEXT NOT NULL,
		position INTEGER NOT NULL,
		path TEXT NOT NULL,
		name TEXT NOT NULL DEFAULT '',
		size INTEGER NOT NULL,
		content BLOB NOT NULL
	);
	CREATE INDEX IF NOT EXISTS message_file_contexts_message_idx ON message_file_contexts(message_id);
`

// migration022 adds the durable session activity timestamp used by the
// narrow sidebar projection. Existing rows inherit creation time so legacy
// databases never expose a fabricated zero timestamp.
const migration022 = `
	ALTER TABLE sessions ADD COLUMN updated_at INTEGER NOT NULL DEFAULT 0;
	UPDATE sessions SET updated_at = created_at WHERE updated_at = 0;
`
