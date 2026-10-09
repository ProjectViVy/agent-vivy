package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	"agent-vivy/internal/domain"
	rc "agent-vivy/internal/reportcontract"
)

// ReportSettingsJobID is the deterministic CronJob identity for one
// (scope, period) settings row — one row is the persistent authority for
// manual and scheduled reports alike.
func ReportSettingsJobID(scope string, period rc.Period) string {
	sum := sha256.Sum256([]byte("vivy.report.settings\x00" + scope + "\x00" + string(period)))
	return "cron_report_" + hex.EncodeToString(sum[:12]) + "_" + string(period)
}

// ReportSettingsStore persists report settings on top of the CronJob
// authority: no second config table. R1 owns read/default materialization;
// R3 owns scheduled dispatch and writes.
type ReportSettingsStore interface {
	// EnsureReportSettings returns the durable settings row for the scope
	// and period, creating the disabled manual-only default on first read.
	EnsureReportSettings(ctx context.Context, scope string, period rc.Period, defaults rc.ReportSettings) (rc.ReportSettings, error)
	// GetReportSettings returns the row without creating it; unknown yields
	// ErrNotFound so callers can distinguish manual-only from configured.
	GetReportSettings(ctx context.Context, scope string, period rc.Period) (rc.ReportSettings, error)
	// WriteReportSettingsCAS replaces the settings payload under revision
	// CAS: a stale expected revision returns ErrRevisionConflict.
	WriteReportSettingsCAS(ctx context.Context, expected int64, settings rc.ReportSettings) (rc.ReportSettings, error)
}

// ReportGeneration is the durable provenance+receipt row for one admitted
// report run: immutable once committed, unique per (scope, run_id).
type ReportGeneration struct {
	Scope          string
	RunID          string
	Period         string
	SeriesID       string
	WindowID       string
	EntryID        string
	RevisionID     string
	ConfigRevision int64
	Timezone       string
	WindowStartMs  int64
	WindowEndMs    int64
	AsOfMs         int64
	InputDigest    string
	FactsDigest    string
	Provider       string
	ModelID        string
	OutcomeMode    string
	OutcomeReason  string
	FactsJSON      []byte
	FeedbackJSON   []byte
	PayloadJSON    []byte
	CreatedAt      int64
}

// ReportGenerationStore owns the immutable generation provenance and its
// output receipt. Insert happens inside the publication transaction;
// execution state never lands here.
type ReportGenerationStore interface {
	// InsertReportGeneration commits the provenance row; a duplicate run_id
	// returns ErrAlreadyExists so the persist effect replays its receipt.
	InsertReportGeneration(ctx context.Context, g ReportGeneration) error
	// GetReportGenerationByRun returns the committed row for one run.
	GetReportGenerationByRun(ctx context.Context, scope, runID string) (ReportGeneration, error)
	// ListReportGenerations returns committed rows of one series in a
	// window-id range, used by weekly/monthly source reuse.
	ListReportGenerations(ctx context.Context, scope, seriesID string, fromWindow, toWindow string) ([]ReportGeneration, error)
}

// ReportSourceRow is one authorized evidence message for report collection.
type ReportSourceRow struct {
	MessageID   string
	SessionID   string
	RunID       string
	Role        string
	Content     string
	CreatedAtMs int64
}

// ReportFeedbackRow is one active comment version admitted as user context.
type ReportFeedbackRow struct {
	CommentID string
	EntryID   string
	Version   int64
	Body      string
}

// ReportEntryHead is the publication race surface of a report target entry.
type ReportEntryHead struct {
	EntryID        string
	SectionID      string
	Title          string
	HeadRevisionID string
	Version        int64
	Deleted        bool
}

// ReportSourceStore is the read side of bounded collection: it never
// reports aggregate data outside the queried scope and window.
type ReportSourceStore interface {
	// ListReportSourceSessions returns non-hidden sessions authorized for
	// the scope: Home scope covers every non-hidden session, a workspace
	// scope covers sessions bound to that workspace path.
	ListReportSourceSessions(ctx context.Context, scope string) ([]domain.Session, error)
	// ListReportSourceMessages returns bounded non-tool messages of one
	// session inside [startMs,endMs), excluding ingest-protected content.
	ListReportSourceMessages(ctx context.Context, sessionID string, startMs, endMs int64, limit int) ([]ReportSourceRow, error)
	// ListReportFeedback returns active comment snapshots on the given
	// entries inside the scope.
	ListReportFeedback(ctx context.Context, scope string, entryIDs []string) ([]ReportFeedbackRow, error)
	// FindReportEntry returns the series identity's entry head for one
	// window, or ErrNotFound when the target doc does not exist yet.
	FindReportEntry(ctx context.Context, scope, seriesID, windowID string) (ReportEntryHead, error)
	// GetReportEntryHead returns a target entry's head by id.
	GetReportEntryHead(ctx context.Context, scope, entryID string) (ReportEntryHead, error)
	// ListReportHumanEdits returns non-generated human revision heads on
	// the given entries (revision id + title) for the human-edit context.
	ListReportHumanEdits(ctx context.Context, scope string, entryIDs []string, limit int) ([]ReportFeedbackRow, error)
}
