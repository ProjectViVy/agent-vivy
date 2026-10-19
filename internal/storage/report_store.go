package storage

import "context"

const (
	// OutcomePromoted marks a generation that became the document head.
	OutcomePromoted = "promoted"
	// OutcomeCandidate marks a generation kept on the revision chain while
	// a human (or newer) head stays authoritative.
	OutcomeCandidate = "candidate"
	// OutcomeDestinationDeleted marks a deleted target: provenance records
	// the outcome and the document is never recreated.
	OutcomeDestinationDeleted = "destination_deleted"
)

// ReportStore is the composite report authority discovered on the storage
// backend: settings (cron authority), immutable generations, and bounded
// source/feedback reads.
type ReportStore interface {
	ReportSettingsStore
	ReportGenerationStore
	ReportSourceStore
	ReportPublicationStore
}

// ReportPublicationInput is the sealed persist-effect DTO: admitted target
// identity plus the rendered document plus immutable provenance.
type ReportPublicationInput struct {
	Scope           string
	OperationKey    string
	RequestDigest   string
	SectionID       string
	EntryID         string // explicit target; empty resolves by series+window
	SeriesID        string
	WindowID        string
	AdmittedHead    string // snapshot head verified at persist time
	AdmittedVersion int64
	Title           string
	Markdown        string
	Actor           string
	Generation      ReportGeneration
}

// ReportPublicationReceipt is the output receipt persisted with the
// publication transaction; replayed persists return it unchanged.
type ReportPublicationReceipt struct {
	EntryID    string
	RevisionID string
	Version    int64
	Outcome    string // promoted | candidate | destination_deleted
}

// ReportPublicationStore commits one report publication atomically:
// revision + generation row + output receipt + conditional head update in
// a single transaction.
type ReportPublicationStore interface {
	CommitReportPublication(ctx context.Context, in ReportPublicationInput) (ReportPublicationReceipt, error)
}
