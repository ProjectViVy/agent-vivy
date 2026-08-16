// Package studiocore is Vivy Studio's own lifecycle engine: the ledger
// (Worktree / Generation / EvalRun / Release / Install) and the operations
// that write it (pack via vivy-sdk, candidate eval, human-gated release,
// install, rollback). It lives in the vivy-studio binary, never in the
// daily vivy.exe. The ledger DB is Studio-owned (data/studio-home/) and is
// not the species Journal.
package studiocore

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"agent-vivy/internal/domain"
)

// ErrNotFound is returned when a ledger object does not exist.
var ErrNotFound = errors.New("studiocore: not found")

// ErrConflict is returned when a state transition is not allowed.
var ErrConflict = errors.New("studiocore: conflict")

// Ledger is the Studio-owned store for lifecycle objects.
type Ledger struct {
	db *sql.DB
}

// OpenLedger opens (or creates) the Studio ledger at path.
func OpenLedger(ctx context.Context, path string) (*Ledger, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("studiocore: ledger dir: %w", err)
	}
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		return nil, fmt.Errorf("studiocore: open ledger %s: %w", path, err)
	}
	db.SetMaxOpenConns(1)
	l := &Ledger{db: db}
	if err := l.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return l, nil
}

// Close releases the ledger handle.
func (l *Ledger) Close() error { return l.db.Close() }

func (l *Ledger) migrate(ctx context.Context) error {
	const schema = `
CREATE TABLE IF NOT EXISTS worktrees (
	id TEXT PRIMARY KEY,
	path TEXT NOT NULL,
	kind TEXT NOT NULL,
	dirty INTEGER NOT NULL DEFAULT 0,
	created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS generations (
	id TEXT PRIMARY KEY,
	parent_id TEXT NOT NULL DEFAULT '',
	artifact_sha256 TEXT NOT NULL,
	source_ref TEXT NOT NULL,
	recipe_json TEXT NOT NULL DEFAULT '{}',
	phase TEXT NOT NULL,
	created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS eval_runs (
	id TEXT PRIMARY KEY,
	candidate_id TEXT NOT NULL,
	baseline_id TEXT NOT NULL DEFAULT '',
	suite TEXT NOT NULL,
	verdict TEXT NOT NULL,
	journal_ref TEXT NOT NULL DEFAULT '',
	created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS releases (
	id TEXT PRIMARY KEY,
	generation_id TEXT NOT NULL,
	eval_id TEXT NOT NULL DEFAULT '',
	actor TEXT NOT NULL,
	phase TEXT NOT NULL,
	created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS installs (
	id TEXT PRIMARY KEY,
	release_id TEXT NOT NULL,
	target TEXT NOT NULL,
	phase TEXT NOT NULL,
	created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS studio_events (
	seq INTEGER PRIMARY KEY AUTOINCREMENT,
	type TEXT NOT NULL,
	object_id TEXT NOT NULL,
	created_at INTEGER NOT NULL,
	payload TEXT NOT NULL DEFAULT '{}'
);
`
	if _, err := l.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("studiocore: migrate ledger: %w", err)
	}
	return nil
}

// ---- Worktree ----

func (l *Ledger) CreateWorktree(ctx context.Context, w domain.Worktree) error {
	if w.ID == "" || w.Path == "" || w.Kind == "" {
		return fmt.Errorf("studiocore: invalid worktree")
	}
	if w.CreatedAt == 0 {
		w.CreatedAt = time.Now().UnixMilli()
	}
	_, err := l.db.ExecContext(ctx,
		`INSERT INTO worktrees (id, path, kind, dirty, created_at) VALUES (?, ?, ?, ?, ?)`,
		w.ID, w.Path, w.Kind, boolInt(w.Dirty), w.CreatedAt)
	if err != nil {
		return fmt.Errorf("studiocore: create worktree: %w", err)
	}
	return nil
}

func (l *Ledger) ListWorktrees(ctx context.Context) ([]domain.Worktree, error) {
	rows, err := l.db.QueryContext(ctx, `SELECT id, path, kind, dirty, created_at FROM worktrees ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("studiocore: list worktrees: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []domain.Worktree{}
	for rows.Next() {
		var w domain.Worktree
		var dirty int
		if err := rows.Scan(&w.ID, &w.Path, &w.Kind, &dirty, &w.CreatedAt); err != nil {
			return nil, err
		}
		w.Dirty = dirty != 0
		out = append(out, w)
	}
	return out, rows.Err()
}

// ---- Generation ----

func (l *Ledger) CreateGeneration(ctx context.Context, g domain.Generation) error {
	if g.ID == "" || g.ArtifactSHA256 == "" || !g.Phase.Valid() {
		return fmt.Errorf("studiocore: invalid generation")
	}
	if g.CreatedAt == 0 {
		g.CreatedAt = time.Now().UnixMilli()
	}
	recipe, err := json.Marshal(g.Recipe)
	if err != nil {
		return fmt.Errorf("studiocore: encode generation recipe: %w", err)
	}
	_, err = l.db.ExecContext(ctx,
		`INSERT INTO generations (id, parent_id, artifact_sha256, source_ref, recipe_json, phase, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		g.ID, g.ParentID, g.ArtifactSHA256, g.SourceRef, string(recipe), string(g.Phase), g.CreatedAt)
	if err != nil {
		return fmt.Errorf("studiocore: create generation: %w", err)
	}
	return nil
}

func (l *Ledger) GetGeneration(ctx context.Context, id string) (domain.Generation, error) {
	var g domain.Generation
	var recipe string
	var phase string
	err := l.db.QueryRowContext(ctx,
		`SELECT id, parent_id, artifact_sha256, source_ref, recipe_json, phase, created_at
		 FROM generations WHERE id = ?`, id).
		Scan(&g.ID, &g.ParentID, &g.ArtifactSHA256, &g.SourceRef, &recipe, &phase, &g.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Generation{}, ErrNotFound
	}
	if err != nil {
		return domain.Generation{}, fmt.Errorf("studiocore: get generation %s: %w", id, err)
	}
	g.Phase = domain.GenerationPhase(phase)
	if recipe != "" {
		_ = json.Unmarshal([]byte(recipe), &g.Recipe)
	}
	return g, nil
}

func (l *Ledger) ListGenerations(ctx context.Context) ([]domain.Generation, error) {
	rows, err := l.db.QueryContext(ctx,
		`SELECT id, parent_id, artifact_sha256, source_ref, recipe_json, phase, created_at
		 FROM generations ORDER BY created_at DESC, id DESC`)
	if err != nil {
		return nil, fmt.Errorf("studiocore: list generations: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []domain.Generation{}
	for rows.Next() {
		var g domain.Generation
		var recipe string
		var phase string
		if err := rows.Scan(&g.ID, &g.ParentID, &g.ArtifactSHA256, &g.SourceRef, &recipe, &phase, &g.CreatedAt); err != nil {
			return nil, err
		}
		g.Phase = domain.GenerationPhase(phase)
		if recipe != "" {
			_ = json.Unmarshal([]byte(recipe), &g.Recipe)
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (l *Ledger) UpdateGenerationPhase(ctx context.Context, id string, phase domain.GenerationPhase) error {
	if !phase.Valid() {
		return fmt.Errorf("studiocore: invalid phase")
	}
	res, err := l.db.ExecContext(ctx, `UPDATE generations SET phase = ? WHERE id = ?`, string(phase), id)
	if err != nil {
		return fmt.Errorf("studiocore: update generation phase %s: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ---- EvalRun ----

func (l *Ledger) CreateEvalRun(ctx context.Context, e domain.EvalRun) error {
	if e.ID == "" || e.CandidateID == "" || !e.Verdict.Valid() {
		return fmt.Errorf("studiocore: invalid eval run")
	}
	if e.CreatedAt == 0 {
		e.CreatedAt = time.Now().UnixMilli()
	}
	_, err := l.db.ExecContext(ctx,
		`INSERT INTO eval_runs (id, candidate_id, baseline_id, suite, verdict, journal_ref, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.CandidateID, e.BaselineID, e.Suite, string(e.Verdict), e.JournalRef, e.CreatedAt)
	if err != nil {
		return fmt.Errorf("studiocore: create eval run: %w", err)
	}
	return nil
}

func (l *Ledger) GetEvalRun(ctx context.Context, id string) (domain.EvalRun, error) {
	var e domain.EvalRun
	var verdict string
	err := l.db.QueryRowContext(ctx,
		`SELECT id, candidate_id, baseline_id, suite, verdict, journal_ref, created_at
		 FROM eval_runs WHERE id = ?`, id).
		Scan(&e.ID, &e.CandidateID, &e.BaselineID, &e.Suite, &verdict, &e.JournalRef, &e.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.EvalRun{}, ErrNotFound
	}
	if err != nil {
		return domain.EvalRun{}, fmt.Errorf("studiocore: get eval run %s: %w", id, err)
	}
	e.Verdict = domain.EvalVerdict(verdict)
	return e, nil
}

func (l *Ledger) ListEvalRuns(ctx context.Context) ([]domain.EvalRun, error) {
	rows, err := l.db.QueryContext(ctx,
		`SELECT id, candidate_id, baseline_id, suite, verdict, journal_ref, created_at
		 FROM eval_runs ORDER BY created_at DESC, id DESC`)
	if err != nil {
		return nil, fmt.Errorf("studiocore: list eval runs: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []domain.EvalRun{}
	for rows.Next() {
		var e domain.EvalRun
		var verdict string
		if err := rows.Scan(&e.ID, &e.CandidateID, &e.BaselineID, &e.Suite, &verdict, &e.JournalRef, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.Verdict = domain.EvalVerdict(verdict)
		out = append(out, e)
	}
	return out, rows.Err()
}

func (l *Ledger) ListEvalRunsFor(ctx context.Context, generationID string) ([]domain.EvalRun, error) {
	rows, err := l.db.QueryContext(ctx,
		`SELECT id, candidate_id, baseline_id, suite, verdict, journal_ref, created_at
		 FROM eval_runs WHERE candidate_id = ? ORDER BY created_at DESC, id DESC`, generationID)
	if err != nil {
		return nil, fmt.Errorf("studiocore: list eval runs for %s: %w", generationID, err)
	}
	defer func() { _ = rows.Close() }()
	out := []domain.EvalRun{}
	for rows.Next() {
		var e domain.EvalRun
		var verdict string
		if err := rows.Scan(&e.ID, &e.CandidateID, &e.BaselineID, &e.Suite, &verdict, &e.JournalRef, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.Verdict = domain.EvalVerdict(verdict)
		out = append(out, e)
	}
	return out, rows.Err()
}

// ---- Release ----

func (l *Ledger) CreateRelease(ctx context.Context, r domain.Release) error {
	if r.ID == "" || r.GenerationID == "" || !r.Phase.Valid() {
		return fmt.Errorf("studiocore: invalid release")
	}
	if r.CreatedAt == 0 {
		r.CreatedAt = time.Now().UnixMilli()
	}
	_, err := l.db.ExecContext(ctx,
		`INSERT INTO releases (id, generation_id, eval_id, actor, phase, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		r.ID, r.GenerationID, r.EvalID, r.Actor, string(r.Phase), r.CreatedAt)
	if err != nil {
		return fmt.Errorf("studiocore: create release: %w", err)
	}
	return nil
}

func (l *Ledger) GetRelease(ctx context.Context, id string) (domain.Release, error) {
	var r domain.Release
	var phase string
	err := l.db.QueryRowContext(ctx,
		`SELECT id, generation_id, eval_id, actor, phase, created_at FROM releases WHERE id = ?`, id).
		Scan(&r.ID, &r.GenerationID, &r.EvalID, &r.Actor, &phase, &r.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Release{}, ErrNotFound
	}
	if err != nil {
		return domain.Release{}, fmt.Errorf("studiocore: get release %s: %w", id, err)
	}
	r.Phase = domain.ReleasePhase(phase)
	return r, nil
}

func (l *Ledger) ListReleases(ctx context.Context) ([]domain.Release, error) {
	rows, err := l.db.QueryContext(ctx,
		`SELECT id, generation_id, eval_id, actor, phase, created_at FROM releases ORDER BY created_at DESC, id DESC`)
	if err != nil {
		return nil, fmt.Errorf("studiocore: list releases: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []domain.Release{}
	for rows.Next() {
		var r domain.Release
		var phase string
		if err := rows.Scan(&r.ID, &r.GenerationID, &r.EvalID, &r.Actor, &phase, &r.CreatedAt); err != nil {
			return nil, err
		}
		r.Phase = domain.ReleasePhase(phase)
		out = append(out, r)
	}
	return out, rows.Err()
}

// ---- Install ----

func (l *Ledger) CreateInstall(ctx context.Context, in domain.Install) error {
	if in.ID == "" || in.ReleaseID == "" || !in.Phase.Valid() {
		return fmt.Errorf("studiocore: invalid install")
	}
	if in.CreatedAt == 0 {
		in.CreatedAt = time.Now().UnixMilli()
	}
	_, err := l.db.ExecContext(ctx,
		`INSERT INTO installs (id, release_id, target, phase, created_at) VALUES (?, ?, ?, ?, ?)`,
		in.ID, in.ReleaseID, in.Target, string(in.Phase), in.CreatedAt)
	if err != nil {
		return fmt.Errorf("studiocore: create install: %w", err)
	}
	return nil
}

func (l *Ledger) GetInstall(ctx context.Context, id string) (domain.Install, error) {
	var in domain.Install
	var phase string
	err := l.db.QueryRowContext(ctx,
		`SELECT id, release_id, target, phase, created_at FROM installs WHERE id = ?`, id).
		Scan(&in.ID, &in.ReleaseID, &in.Target, &phase, &in.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Install{}, ErrNotFound
	}
	if err != nil {
		return domain.Install{}, fmt.Errorf("studiocore: get install %s: %w", id, err)
	}
	in.Phase = domain.InstallPhase(phase)
	return in, nil
}

func (l *Ledger) ListInstalls(ctx context.Context) ([]domain.Install, error) {
	rows, err := l.db.QueryContext(ctx,
		`SELECT id, release_id, target, phase, created_at FROM installs ORDER BY created_at DESC, id DESC`)
	if err != nil {
		return nil, fmt.Errorf("studiocore: list installs: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []domain.Install{}
	for rows.Next() {
		var in domain.Install
		var phase string
		if err := rows.Scan(&in.ID, &in.ReleaseID, &in.Target, &phase, &in.CreatedAt); err != nil {
			return nil, err
		}
		in.Phase = domain.InstallPhase(phase)
		out = append(out, in)
	}
	return out, rows.Err()
}

// CurrentInstall returns the newest install row with phase current for the
// target, or ErrNotFound.
func (l *Ledger) CurrentInstall(ctx context.Context, target string) (domain.Install, error) {
	var in domain.Install
	var phase string
	err := l.db.QueryRowContext(ctx,
		`SELECT id, release_id, target, phase, created_at FROM installs
		 WHERE target = ? AND phase = 'current' ORDER BY created_at DESC, id DESC LIMIT 1`, target).
		Scan(&in.ID, &in.ReleaseID, &in.Target, &phase, &in.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Install{}, ErrNotFound
	}
	if err != nil {
		return domain.Install{}, fmt.Errorf("studiocore: current install %s: %w", target, err)
	}
	in.Phase = domain.InstallPhase(phase)
	return in, nil
}

func (l *Ledger) UpdateInstallPhase(ctx context.Context, id string, phase domain.InstallPhase) error {
	if !phase.Valid() {
		return fmt.Errorf("studiocore: invalid install phase")
	}
	res, err := l.db.ExecContext(ctx, `UPDATE installs SET phase = ? WHERE id = ?`, string(phase), id)
	if err != nil {
		return fmt.Errorf("studiocore: update install phase %s: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ---- Events ----

func (l *Ledger) AppendStudioEvent(ctx context.Context, ev domain.StudioEvent) error {
	if !ev.Type.Valid() || ev.ObjectID == "" {
		return fmt.Errorf("studiocore: invalid studio event")
	}
	if ev.CreatedAt == 0 {
		ev.CreatedAt = time.Now().UnixMilli()
	}
	payload := string(ev.Payload)
	if payload == "" {
		payload = "{}"
	}
	_, err := l.db.ExecContext(ctx,
		`INSERT INTO studio_events (type, object_id, created_at, payload) VALUES (?, ?, ?, ?)`,
		string(ev.Type), ev.ObjectID, ev.CreatedAt, payload)
	if err != nil {
		return fmt.Errorf("studiocore: append studio event: %w", err)
	}
	return nil
}

func (l *Ledger) ListStudioEvents(ctx context.Context) ([]domain.StudioEvent, error) {
	rows, err := l.db.QueryContext(ctx,
		`SELECT seq, type, object_id, created_at, payload FROM studio_events ORDER BY seq ASC`)
	if err != nil {
		return nil, fmt.Errorf("studiocore: list studio events: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []domain.StudioEvent{}
	for rows.Next() {
		var ev domain.StudioEvent
		var typ string
		if err := rows.Scan(&ev.Seq, &typ, &ev.ObjectID, &ev.CreatedAt, &ev.Payload); err != nil {
			return nil, err
		}
		ev.Type = domain.StudioEventType(typ)
		out = append(out, ev)
	}
	return out, rows.Err()
}

// NewID returns a fresh Studio-owned object id with the given prefix.
func NewID(prefix string) string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%s%x", prefix, time.Now().UnixNano())
	}
	return prefix + hex.EncodeToString(b[:])
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
