package memory

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"math"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/ProjectViVy/agent-vivy/bml"
	gardenmemory "github.com/ProjectViVy/laputa/garden/memory"
	"github.com/ProjectViVy/laputa/laputa/evolution"
)

// BoundBackend borrows one App-owned Home. It serves historical material under
// a complete trusted binding, never the legacy machine view or raw ingest port.
// C2 source eligibility remains the host receiver's responsibility.
type BoundBackend struct {
	service *Service
	binding bml.OwnershipBinding
	scope   evolution.Scope
	cursor  cipher.AEAD
}

var _ gardenmemory.Backend = (*BoundBackend)(nil)

func backendFailure(code evolution.Code, message string) error {
	return &evolution.ContractError{Code: code, Message: message}
}

func NewBoundBackend(service *Service, binding bml.OwnershipBinding) (*BoundBackend, error) {
	scope := evolution.Scope{SubjectID: binding.SubjectID, Kind: evolution.ScopeKind(binding.ScopeKind), WorkspaceID: binding.WorkspaceID}
	if !rawUTF8(reflect.ValueOf(binding)) || strings.TrimSpace(binding.SubjectID) == "" || (binding.ScopeKind == "workspace" && strings.TrimSpace(binding.WorkspaceID) == "") || strings.TrimSpace(binding.DomainID) == "" || strings.TrimSpace(binding.DestinationID) == "" {
		return nil, backendFailure(evolution.ErrInvalidScope, "complete native owner/destination required")
	}
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if !service.live() {
		return nil, errUnavailable
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	cursor, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &BoundBackend{service: service, binding: binding, scope: scope, cursor: cursor}, nil
}
func (b *BoundBackend) Capabilities() gardenmemory.Capabilities {
	live := b != nil && b.service.live()
	return gardenmemory.Capabilities{Search: live, Expand: live, Mutate: live, MutationLookup: live}
}
func (b *BoundBackend) live() error {
	if b == nil || !b.service.live() {
		return errUnavailable
	}
	return nil
}
func (b *BoundBackend) authorizeScopes(scopes []evolution.Scope) error {
	for _, scope := range scopes {
		if !scope.Equal(b.scope) {
			return backendFailure(evolution.ErrAuthorityDenied, "scope outside exact native owner")
		}
	}
	return nil
}
func (b *BoundBackend) Search(ctx context.Context, req gardenmemory.AuthorizedSearch) (gardenmemory.CardPage, error) {
	if err := b.live(); err != nil {
		return gardenmemory.CardPage{}, err
	}
	if !rawUTF8(reflect.ValueOf(req)) {
		return gardenmemory.CardPage{}, backendFailure(evolution.ErrInvalidSchema, "search must be UTF-8")
	}
	if err := req.Validate(); err != nil {
		return gardenmemory.CardPage{}, err
	}
	if err := b.authorizeScopes(req.Scopes); err != nil {
		return gardenmemory.CardPage{}, err
	}
	if req.Collection != "" && req.Collection != "history" {
		return gardenmemory.CardPage{}, backendFailure(evolution.ErrCapabilityUnavailable, "native historical collection only")
	}
	limit := req.Limit
	if limit == 0 {
		limit = 20
	}
	if limit > 100 {
		return gardenmemory.CardPage{}, backendFailure(evolution.ErrInvalidSchema, "native search limit exceeds 100")
	}
	offset, err := b.readCursor(req)
	if err != nil {
		return gardenmemory.CardPage{}, err
	}
	// Native FTS owns rank and visibility. Pagination slices its bounded ordered
	// result; no adapter scoring, secondary search or authority body is generated.
	hits, err := b.service.home.SearchBound(ctx, b.binding, bml.SearchQuery{Text: req.Query, Limit: 100})
	if err != nil {
		return gardenmemory.CardPage{}, err
	}
	if offset > len(hits) {
		offset = len(hits)
	}
	end := min(offset+limit, len(hits))
	out := gardenmemory.CardPage{Items: []gardenmemory.MemoryCard{}}
	for _, hit := range hits[offset:end] {
		r := hit.Record
		out.Items = append(out.Items, gardenmemory.MemoryCard{ID: r.Record.ID, Kind: string(r.Record.Kind), Collection: "history", Scope: gardenmemory.EncodeScope(b.scope), Revision: int(r.Revision), Status: "active", SourceRef: r.Record.Provenance.SourceID, ValidFrom: r.Record.EffectiveAt, ValidTo: r.Record.ExpiresAt, CandidateScore: hit.Rank})
	}
	if end < len(hits) {
		out.NextCursor, err = b.writeCursor(req, end)
	}
	return out, err
}
func (b *BoundBackend) cursorContext(req gardenmemory.AuthorizedSearch) []byte {
	req.Cursor = ""
	raw, _ := json.Marshal(req)
	return raw
}
func (b *BoundBackend) writeCursor(req gardenmemory.AuthorizedSearch, offset int) (string, error) {
	nonce := make([]byte, b.cursor.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	raw := make([]byte, 8)
	binary.BigEndian.PutUint64(raw, uint64(offset))
	return base64.RawURLEncoding.EncodeToString(b.cursor.Seal(nonce, nonce, raw, b.cursorContext(req))), nil
}
func (b *BoundBackend) readCursor(req gardenmemory.AuthorizedSearch) (int, error) {
	if req.Cursor == "" {
		return 0, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(req.Cursor)
	if err != nil || len(raw) < b.cursor.NonceSize() {
		return 0, backendFailure(evolution.ErrInvalidScope, "invalid native search continuation")
	}
	plain, err := b.cursor.Open(nil, raw[:b.cursor.NonceSize()], raw[b.cursor.NonceSize():], b.cursorContext(req))
	if err != nil || len(plain) != 8 {
		return 0, backendFailure(evolution.ErrInvalidScope, "continuation belongs to another host or request")
	}
	offset := binary.BigEndian.Uint64(plain)
	if offset >= 100 {
		return 0, backendFailure(evolution.ErrInvalidSchema, "continuation outside native search bound")
	}
	return int(offset), nil
}
func (b *BoundBackend) Expand(ctx context.Context, req gardenmemory.AuthorizedExpansion) (gardenmemory.EvidencePage, error) {
	if err := b.live(); err != nil {
		return gardenmemory.EvidencePage{}, err
	}
	if !rawUTF8(reflect.ValueOf(req)) {
		return gardenmemory.EvidencePage{}, backendFailure(evolution.ErrInvalidSchema, "expansion must be UTF-8")
	}
	if err := req.Validate(); err != nil {
		return gardenmemory.EvidencePage{}, err
	}
	if err := b.authorizeScopes(req.Scopes); err != nil {
		return gardenmemory.EvidencePage{}, err
	}
	if req.ExpectedRevision > math.MaxInt64 || req.BudgetChars < 0 || req.BudgetChars > 4000 {
		return gardenmemory.EvidencePage{}, backendFailure(evolution.ErrInvalidSchema, "native expansion bound exceeded")
	}
	stored, err := b.service.home.ExportBoundSource(ctx, b.binding, req.CardID, int64(req.ExpectedRevision))
	if err != nil {
		return gardenmemory.EvidencePage{}, err
	}
	budget := req.BudgetChars
	if budget == 0 {
		budget = 1000
	}
	runes := []rune(stored.Record.Content)
	end := min(len(runes), budget)
	refs := make([]string, len(stored.Record.EvidenceRefs))
	for i, ref := range stored.Record.EvidenceRefs {
		refs[i] = ref.URI
	}
	return gardenmemory.EvidencePage{Items: []gardenmemory.EvidenceFragment{{CardID: stored.Record.ID, MaterialRef: stored.Record.ID, SourceURI: stored.Record.Provenance.SourceID, SourceRev: strconv.FormatInt(stored.Revision, 10), Scope: gardenmemory.EncodeScope(b.scope), Revision: uint64(stored.Revision), Status: "active", Excerpt: string(runes[:end]), StartOffset: 0, EndOffset: end, ContentHash: stored.Record.Provenance.ContentDigest.Value, Validity: "active", EvidenceRefs: refs}}, Truncated: end < len(runes)}, nil
}
func (b *BoundBackend) Mutate(ctx context.Context, req gardenmemory.AuthorizedMutation) (gardenmemory.MutationReceipt, error) {
	if err := b.live(); err != nil {
		return gardenmemory.MutationReceipt{}, err
	}
	if !rawUTF8(reflect.ValueOf(req)) {
		return gardenmemory.MutationReceipt{}, backendFailure(evolution.ErrInvalidSchema, "native request must be UTF-8")
	}
	if err := req.Validate(); err != nil {
		return gardenmemory.MutationReceipt{}, err
	}
	if err := req.MatchesWriter(b.scope, b.binding.DestinationID); err != nil {
		return gardenmemory.MutationReceipt{}, err
	}
	if req.ExpectedRevision > math.MaxInt64 {
		return gardenmemory.MutationReceipt{}, backendFailure(evolution.ErrInvalidSchema, "native revision overflow")
	}
	for _, source := range req.Sources {
		if !source.Scope.Equal(b.scope) {
			return gardenmemory.MutationReceipt{}, backendFailure(evolution.ErrAuthorityDenied, "source outside exact native owner")
		}
	}
	// Garden has no source clock. Zero times explicitly mean unspecified, not
	// observed capture time. Pure complete-request mapping keeps retries stable
	// after updates/tombstones/reopen without reading today's canonical row.
	raw, err := json.Marshal(req)
	if err != nil {
		return gardenmemory.MutationReceipt{}, err
	}
	digest := sha256.Sum256(raw)
	trust := bml.TrustObserved
	if req.Inference == evolution.InferenceInferred {
		trust = bml.TrustInferred
	}
	refs := make([]bml.EvidenceRef, len(req.Sources))
	for i, source := range req.Sources {
		encoded, _ := json.Marshal(source)
		refs[i] = bml.EvidenceRef{ID: source.RecordID, Source: bml.EvidenceSourceExperienceJournal, URI: "garden-source:v1:" + base64.RawURLEncoding.EncodeToString(encoded)}
	}
	record := bml.Record{ID: req.RecordID, Kind: bml.KindHistory, Content: req.Body, Provenance: bml.Provenance{Source: bml.ProvenanceSourceSessionSync, SourceID: req.OperationID + ":" + hex.EncodeToString(digest[:]), ContentDigest: bml.MemoryContentDigest([]byte(req.Body)), Correlation: bml.AuditCorrelation{RequestID: req.OperationID, TurnID: string(req.Operation), SessionID: "operation:" + req.OperationID}}, EvidenceRefs: refs, ConfidenceBPS: bml.MaxConfidenceBPS, Sensitivity: bml.SensitivityInternal, Trust: trust, Scope: bml.Scope{TenantID: "local", WorkspaceID: "machine-memory-home"}, Supersedes: []string{}}
	mutation := bml.BoundMutation{Operation: string(req.Operation), OperationID: req.OperationID, PayloadDigest: req.PayloadDigest, Record: record}
	revision := int64(req.ExpectedRevision)
	switch req.Operation {
	case evolution.MutationUpdate:
		mutation.ExpectedRecordRevision = &revision
	case evolution.MutationTombstone:
		marker := sha256.Sum256([]byte(req.OperationID))
		mutation.Record.ID = "tomb-" + hex.EncodeToString(marker[:])
		mutation.Record.Content = ""
		mutation.Record.Supersedes = []string{req.RecordID}
		mutation.Record.Tombstone = &bml.Tombstone{TargetRecordID: req.RecordID, ReasonDigest: bml.MemoryContentDigest([]byte(req.Body)), ActorID: b.binding.SubjectID}
		mutation.ExpectedTargetRevision = &revision
	}
	out, err := b.service.home.MutateBound(ctx, b.binding, mutation)
	if err != nil {
		return gardenmemory.MutationReceipt{}, err
	}
	return nativeReceipt(out), nil
}
func nativeReceipt(out bml.MutationOutcome) gardenmemory.MutationReceipt {
	return gardenmemory.MutationReceipt{EffectReceipt: evolution.EffectReceipt{OperationID: out.OperationID, PayloadDigest: out.PayloadDigest, Status: evolution.StatusApplied, TargetRef: out.RecordID, Revision: uint64(out.RecordRevision)}, CanonicalStatus: gardenmemory.CanonicalStatus(out.CanonicalStatus), IndexStatus: gardenmemory.IndexStatus(out.IndexStatus)}
}
func (b *BoundBackend) MutationStatus(ctx context.Context, operationID string) (gardenmemory.MutationReceipt, error) {
	if err := b.live(); err != nil {
		return gardenmemory.MutationReceipt{}, err
	}
	out, err := b.service.home.LookupBoundOperation(ctx, b.binding, operationID)
	if err != nil {
		return gardenmemory.MutationReceipt{}, err
	}
	if out == nil {
		return gardenmemory.MutationReceipt{}, backendFailure(evolution.ErrEffectNotFound, "native operation absent")
	}
	return nativeReceipt(*out), nil
}
func (b *BoundBackend) Health(ctx context.Context) (gardenmemory.Health, error) {
	if err := ctx.Err(); err != nil {
		return gardenmemory.Health{}, err
	}
	if err := b.live(); err != nil {
		return gardenmemory.Health{Status: gardenmemory.HealthUnavailable, ReasonCode: "closed", DerivedIndexState: "unavailable"}, nil
	}
	return gardenmemory.Health{Status: gardenmemory.HealthAvailable, DerivedIndexState: "native_fts"}, nil
}

// Close does not own the shared Home. App releases its exact Service instance.
func (*BoundBackend) Close() error { return nil }
func rawUTF8(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.String:
		return utf8.ValidString(v.String())
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			if !rawUTF8(v.Index(i)) {
				return false
			}
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if v.Type().Field(i).IsExported() && !rawUTF8(v.Field(i)) {
				return false
			}
		}
	}
	return true
}
