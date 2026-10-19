package bml

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"unicode/utf8"
)

// BoundCapabilities describes this opened native handle, including read-only
// and legacy-schema limits. It is not a promise about a future adapter.
type BoundCapabilities struct{ Search, Export, Mutate, MutationLookup bool }

func (s *Store) BoundCapabilities() BoundCapabilities {
	if s.schema < 2 {
		return BoundCapabilities{}
	}
	return BoundCapabilities{Search: true, Export: true, Mutate: !s.readOnly, MutationLookup: true}
}

func (s *Store) legacyPredicate(alias string) string {
	if s.schema < 2 {
		return "1=1"
	}
	return `NOT EXISTS (SELECT 1 FROM memory_ownership own WHERE own.memory_id=` + alias + `.memory_id)`
}

type legacyReader interface {
	rowQuerier
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

// A schema-1 handle cannot use its cached version as a data visibility grant:
// another supported writer may have upgraded the database since open. Pin the
// version check and legacy query to one SQLite snapshot, or require reopening.
func (s *Store) legacyRead(ctx context.Context) (legacyReader, func(), error) {
	if s.schema != 1 {
		return s.db, func() {}, nil
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, nil, persistenceErr(s.path, err)
	}
	var version, revision int64
	var workspace string
	if err := tx.QueryRowContext(ctx, selectMetaSQL, component).Scan(&version, &revision, &workspace); err != nil {
		tx.Rollback()
		return nil, nil, persistenceErr(s.path, err)
	}
	if err := checkMetaRow(version, workspace, s.workspaceID); err != nil {
		tx.Rollback()
		return nil, nil, err
	}
	if version != 1 {
		tx.Rollback()
		return nil, nil, boundError("schema_changed", "legacy read-only schema changed; reopen store")
	}
	return tx, func() { tx.Rollback() }, nil
}

// GetBound returns the visible exact native row version under its stored
// ownership. Neither an unknown legacy row nor a newer revision is substituted.
func (s *Store) GetBound(ctx context.Context, binding OwnershipBinding, id string, expectedRevision int64) (StoredRecord, error) {
	if err := binding.validate(); err != nil {
		return StoredRecord{}, err
	}
	if !utf8.ValidString(id) || strings.TrimSpace(id) == "" || expectedRevision <= 0 {
		return StoredRecord{}, boundError("invalid_mutation", "exact native source identity/revision required")
	}
	if !s.BoundCapabilities().Export {
		return StoredRecord{}, boundUnavailable()
	}
	owner, err := marshalCanonical(binding)
	if err != nil {
		return StoredRecord{}, err
	}
	var revision int64
	var raw string
	err = s.db.QueryRowContext(ctx, `SELECT r.record_revision,r.record_json FROM memory_records r JOIN memory_ownership own ON own.memory_id=r.memory_id
	 WHERE r.memory_id=? AND own.binding_json=? AND r.tombstone=0
	 AND NOT EXISTS (SELECT 1 FROM memory_supersedes edge JOIN memory_records marker ON marker.memory_id=edge.memory_id WHERE edge.superseded_id=r.memory_id AND marker.tombstone=1)`, id, string(owner)).Scan(&revision, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return StoredRecord{}, boundError("source_unavailable", "native source unavailable for binding")
	}
	if err != nil {
		return StoredRecord{}, persistenceErr(s.path, err)
	}
	if revision != expectedRevision {
		return StoredRecord{}, recordRevisionConflictErr(id, &expectedRevision, &revision)
	}
	return decodeStored(revision, raw)
}

// SearchBound adds an exact durable ownership predicate to native FTS before
// ranking/LIMIT. SQLite still owns escaping, visibility and the returned rank.
func (s *Store) SearchBound(ctx context.Context, binding OwnershipBinding, q SearchQuery) ([]SearchHit, error) {
	if err := binding.validate(); err != nil {
		return nil, err
	}
	if !s.BoundCapabilities().Search {
		return nil, boundUnavailable()
	}
	if !utf8.ValidString(q.Text) {
		return nil, boundError("invalid_mutation", "search text must be UTF-8")
	}
	return s.search(ctx, q, searchVisibleSQL, 1, &binding)
}

func (h *Home) MutateBound(ctx context.Context, binding OwnershipBinding, m BoundMutation) (MutationOutcome, error) {
	if err := binding.validate(); err != nil {
		return MutationOutcome{}, err
	}
	s, err := h.writableStore(ctx)
	if err != nil {
		return MutationOutcome{}, err
	}
	out, err := s.MutateBound(ctx, binding, m)
	if err != nil {
		return MutationOutcome{}, err
	}
	// Home owns the derived L1 lifecycle. A refresh failure never erases the
	// committed native receipt; retry replays it and can repair projection.
	if err := h.refreshStartupIndex(ctx); err != nil {
		return MutationOutcome{}, err
	}
	return out, nil
}

func (h *Home) LookupBoundMutation(ctx context.Context, binding OwnershipBinding, operationID, digest string) (*MutationOutcome, error) {
	if err := binding.validate(); err != nil {
		return nil, err
	}
	s, err := h.existingStore(ctx)
	if err != nil {
		return nil, err
	}
	if s == nil {
		return nil, boundUnavailable()
	}
	return s.LookupBoundMutation(ctx, binding, operationID, digest)
}

func (h *Home) SearchBound(ctx context.Context, binding OwnershipBinding, q SearchQuery) ([]SearchHit, error) {
	if err := binding.validate(); err != nil {
		return nil, err
	}
	s, err := h.existingStore(ctx)
	if err != nil {
		return nil, err
	}
	if s == nil {
		return nil, boundUnavailable()
	}
	q.Scope = machineScope(q.Scope.SessionID)
	return s.SearchBound(ctx, binding, q)
}

// ExportBoundSource is an exact-version BML-owned source export. Authorization
// of core originals remains with the host; BML never reconstructs those bodies.
func (h *Home) ExportBoundSource(ctx context.Context, binding OwnershipBinding, id string, revision int64) (StoredRecord, error) {
	if err := binding.validate(); err != nil {
		return StoredRecord{}, err
	}
	s, err := h.existingStore(ctx)
	if err != nil {
		return StoredRecord{}, err
	}
	if s == nil {
		return StoredRecord{}, boundUnavailable()
	}
	stored, err := s.GetBound(ctx, binding, id, revision)
	if err != nil {
		return StoredRecord{}, err
	}
	if stored.Record.Kind != KindLongTerm && stored.Record.Kind != KindHistory {
		return StoredRecord{}, boundError("source_unavailable", "record is not historical/long-term material")
	}
	return stored, nil
}

// ExportSource is the existing machine/legacy view, never an ownership grant.
func (h *Home) ExportSource(ctx context.Context, id string, revision int64) (StoredRecord, error) {
	if !utf8.ValidString(id) || strings.TrimSpace(id) == "" || revision <= 0 {
		return StoredRecord{}, homeInvalid("exact source identity/revision required")
	}
	s, err := h.existingStore(ctx)
	if err != nil {
		return StoredRecord{}, err
	}
	if s == nil {
		return StoredRecord{}, homeNotFound(id)
	}
	stored, err := s.Get(ctx, id)
	if err != nil {
		return StoredRecord{}, err
	}
	if stored == nil || stored.Record.Tombstone != nil || (stored.Record.Kind != KindHistory && stored.Record.Kind != KindLongTerm) {
		return StoredRecord{}, homeNotFound(id)
	}
	superseded, err := s.SupersededTargetIDs(ctx)
	if err != nil {
		return StoredRecord{}, err
	}
	if _, found := superseded[id]; found {
		return StoredRecord{}, homeNotFound(id)
	}
	if stored.Revision != revision {
		return StoredRecord{}, homeRevisionConflict(revision, &stored.Revision)
	}
	return *stored, nil
}

// LookupBoundOperation returns the retained native result for an exact owner
// and operation ID, including its original digest. This read-only status API
// never weakens LookupBoundMutation's required digest or mutation replay checks.
func (s *Store) LookupBoundOperation(ctx context.Context, binding OwnershipBinding, operationID string) (*MutationOutcome, error) {
	if err := binding.validate(); err != nil {
		return nil, err
	}
	if !utf8.ValidString(operationID) || strings.TrimSpace(operationID) == "" {
		return nil, boundError("invalid_mutation", "native operation identity required")
	}
	if s.schema < 2 {
		return nil, boundUnavailable()
	}
	raw, err := marshalCanonical(binding)
	if err != nil {
		return nil, err
	}
	return s.lookupBound(ctx, s.db, string(raw), operationID, nil, "")
}

func (h *Home) LookupBoundOperation(ctx context.Context, binding OwnershipBinding, operationID string) (*MutationOutcome, error) {
	if err := binding.validate(); err != nil {
		return nil, err
	}
	s, err := h.existingStore(ctx)
	if err != nil {
		return nil, err
	}
	if s == nil {
		return nil, boundUnavailable()
	}
	return s.LookupBoundOperation(ctx, binding, operationID)
}
