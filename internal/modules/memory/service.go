package memory

import (
	"context"
	"errors"
	"os"
	"sync/atomic"
	"time"

	"github.com/ProjectViVy/agent-vivy/bml"
)

// defaultRecallLimit caps list/search fan-out when the caller leaves the
// limit unset, matching the store's own search ceiling.
const defaultRecallLimit uint32 = 100

// Service wraps the machine-home BML facade. Every method resolves through
// the same Home handle owned by the package-level registry; a service left
// behind by Close reports the explicit bml_unavailable outcome rather than
// reopening the store.
type Service struct {
	home   *bml.Home
	closed atomic.Bool
}

func (s *Service) close() error {
	s.closed.Store(true)
	return s.home.Close()
}

// live reports whether the service can still reach the store.
func (s *Service) live() bool { return s != nil && !s.closed.Load() }

// StatusResponse is the vivy.memory.status wire DTO. RulesRevision is the
// current MEMRULES CAS token; empty when the document could not be read.
type StatusResponse struct {
	Available       bool   `json:"available"`
	Reason          string `json:"reason,omitempty"`
	StartupRevision uint64 `json:"startup_revision"`
	DatabasePresent bool   `json:"database_present"`
	RulesRevision   string `json:"rules_revision,omitempty"`
}

// List returns the visible long-term projection as a listed outcome.
func (s *Service) List(ctx context.Context, req bml.MemoryListRequest) bml.MemoryCrudOutcome {
	if !s.live() {
		return unavailableOutcome()
	}
	limit := defaultRecallLimit
	if req.Limit != nil {
		limit = *req.Limit
	}
	records, err := s.home.ListRecords(ctx, limit)
	if err != nil {
		return outcomeFromError(err)
	}
	entries := make([]bml.MemoryEntry, 0, len(records))
	for _, record := range records {
		entries = append(entries, entryFromRecord(record))
	}
	return bml.MemoryCrudOutcome{Status: bml.CrudOutcomeListed, Entries: entries}
}

// Get returns one visible record as a listed outcome, or failed
// memory_not_found when the id is absent or no longer visible.
func (s *Service) Get(ctx context.Context, req bml.MemoryGetRequest) bml.MemoryCrudOutcome {
	if !s.live() {
		return unavailableOutcome()
	}
	record, err := s.home.GetRecord(ctx, req.RecordID)
	if err != nil {
		return outcomeFromError(err)
	}
	if record == nil {
		return failedOutcome(bml.HomeCodeNotFound)
	}
	return bml.MemoryCrudOutcome{Status: bml.CrudOutcomeListed, Entries: []bml.MemoryEntry{entryFromRecord(*record)}}
}

// Search runs FTS5 recall through the store's own escaping and visibility
// semantics and returns the hits as a listed outcome.
func (s *Service) Search(ctx context.Context, req bml.MemorySearchRequest) bml.MemoryCrudOutcome {
	if !s.live() {
		return unavailableOutcome()
	}
	limit := defaultRecallLimit
	if req.Limit != nil {
		limit = *req.Limit
	}
	hits, err := s.home.SearchVisible(ctx, bml.SearchQuery{Text: req.Query, Limit: limit})
	if err != nil {
		return outcomeFromError(err)
	}
	entries := make([]bml.MemoryEntry, 0, len(hits))
	for _, hit := range hits {
		entries = append(entries, entryFromRecord(hit.Record))
	}
	return bml.MemoryCrudOutcome{Status: bml.CrudOutcomeListed, Entries: entries}
}

// Add applies a user-asserted long-term record and returns the applied
// outcome with the stored entry.
func (s *Service) Add(ctx context.Context, req bml.MemoryAddRequest) bml.MemoryCrudOutcome {
	if !s.live() {
		return unavailableOutcome()
	}
	stored, err := s.home.AddRecord(ctx, bml.KindLongTerm, req.Content, req.EvidenceRefs)
	if err != nil {
		return outcomeFromError(err)
	}
	entry := entryFromRecord(stored)
	return bml.MemoryCrudOutcome{Status: bml.CrudOutcomeApplied, Entry: &entry}
}

// Update applies a compare-and-swap content update under the caller's base
// revision; a stale revision returns failed memory_revision_conflict.
func (s *Service) Update(ctx context.Context, req bml.MemoryUpdateRequest) bml.MemoryCrudOutcome {
	if !s.live() {
		return unavailableOutcome()
	}
	stored, err := s.home.UpdateRecord(ctx, req.RecordID, req.Content, req.BaseRevision, req.EvidenceRefs)
	if err != nil {
		return outcomeFromError(err)
	}
	entry := entryFromRecord(stored)
	return bml.MemoryCrudOutcome{Status: bml.CrudOutcomeApplied, Entry: &entry}
}

// Remove tombstones a record under the caller's base revision and returns
// the deposed entry.
func (s *Service) Remove(ctx context.Context, req bml.MemoryRemoveRequest) bml.MemoryCrudOutcome {
	if !s.live() {
		return unavailableOutcome()
	}
	stored, err := s.home.RemoveRecord(ctx, req.RecordID, req.Reason, req.BaseRevision)
	if err != nil {
		return outcomeFromError(err)
	}
	entry := entryFromRecord(stored)
	return bml.MemoryCrudOutcome{Status: bml.CrudOutcomeApplied, Entry: &entry}
}

// RulesView is the rules.read payload: the handbook plus the revision token
// WriteRules requires as its CAS base.
type RulesView struct {
	Content  string `json:"content"`
	Source   string `json:"source"`
	Revision string `json:"revision"`
}

// rulesRevision is the MEMRULES CAS token: a stable sha256 digest of the
// document's current content, so a stale base detects interleaved writes
// and external edits, and survives restarts.
func rulesRevision(content string) string {
	return bml.MemoryContentDigest([]byte(content)).Value
}

// Rules reads MEMRULES.MD, falling back to the built-in rulebook.
func (s *Service) Rules(_ context.Context) (RulesView, bml.MemoryCrudOutcome) {
	if !s.live() {
		return RulesView{}, unavailableOutcome()
	}
	doc, err := s.home.ReadMemRules()
	if err != nil {
		return RulesView{}, outcomeFromError(err)
	}
	return RulesView{
		Content:  doc.Content,
		Source:   string(doc.Source),
		Revision: rulesRevision(doc.Content),
	}, bml.MemoryCrudOutcome{Status: bml.CrudOutcomeListed}
}

// WriteRules atomically replaces MEMRULES.MD under content-digest CAS:
// baseRevision must equal the digest of the current MEMRULES content (from
// Rules or Status), else the outcome is failed memory_revision_conflict —
// never a forced write.
func (s *Service) WriteRules(_ context.Context, content string, baseRevision string) bml.MemoryCrudOutcome {
	if !s.live() {
		return unavailableOutcome()
	}
	current, err := s.home.ReadMemRules()
	if err != nil {
		return outcomeFromError(err)
	}
	if baseRevision != rulesRevision(current.Content) {
		return failedOutcome(bml.HomeCodeRevisionConflict)
	}
	if _, err := s.home.WriteMemRules(content); err != nil {
		return outcomeFromError(err)
	}
	return bml.MemoryCrudOutcome{Status: bml.CrudOutcomeApplied}
}

// Status reports the service's own availability without fabricating store
// state it cannot see.
func (s *Service) Status(_ context.Context) StatusResponse {
	if !s.live() {
		return StatusResponse{Available: false, Reason: bml.HomeCodeBmlUnavailable}
	}
	status := StatusResponse{Available: true, StartupRevision: s.home.StartupRevision()}
	_, err := os.Stat(s.home.DatabasePath())
	status.DatabasePresent = err == nil
	if doc, err := s.home.ReadMemRules(); err == nil {
		status.RulesRevision = rulesRevision(doc.Content)
	}
	return status
}

// Home exposes the wrapped facade for composition-side callers that need
// store paths; mutation still goes through the Service methods.
func (s *Service) Home() *bml.Home { return s.home }

// unavailableOutcome is the explicit failure for a closed or never-opened
// service — never a fabricated success.
func unavailableOutcome() bml.MemoryCrudOutcome {
	return failedOutcome(bml.HomeCodeBmlUnavailable)
}

// failedOutcome reports a stable, content-free reason.
func failedOutcome(reason string) bml.MemoryCrudOutcome {
	return bml.MemoryCrudOutcome{Status: bml.CrudOutcomeFailed, Reason: reason}
}

// outcomeFromError maps a BML facade failure onto the four-state outcome
// vocabulary. The stable HomeError code becomes the reason so record
// Content never leaks into an error string; non-facade errors collapse to
// bml_unavailable.
func outcomeFromError(err error) bml.MemoryCrudOutcome {
	var homeErr *bml.HomeError
	if errors.As(err, &homeErr) {
		return failedOutcome(homeErr.Code())
	}
	return unavailableOutcome()
}

// entryFromRecord projects a stored record into the contract MemoryEntry:
// trust and provenance keep their serde wire spellings, and EffectiveAt is
// the last-mutated timestamp bml maintains under CAS.
func entryFromRecord(stored bml.StoredRecord) bml.MemoryEntry {
	provenance := string(stored.Record.Provenance.Source)
	return bml.MemoryEntry{
		ID:           stored.Record.ID,
		Content:      stored.Record.Content,
		Trust:        string(stored.Record.Trust),
		Provenance:   &provenance,
		EvidenceRefs: stored.Record.EvidenceRefs,
		Revision:     stored.Revision,
		CreatedAt:    stored.Record.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:    stored.Record.EffectiveAt.UTC().Format(time.RFC3339),
	}
}
