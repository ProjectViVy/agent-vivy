package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

// HistoryStatus is the public outcome vocabulary for bounded continuity
// operations. It deliberately does not expose storage or policy internals.
type HistoryStatus string

const (
	HistoryStatusOK              = "ok"
	HistoryStatusPartial         = "partial"
	HistoryStatusForbidden       = "forbidden"
	HistoryStatusNotFound        = "not_found"
	HistoryStatusUnavailable     = "unavailable"
	HistoryStatusInvalidArgument = "invalid_argument"
	HistoryStatusConflict        = "conflict"
	HistoryStatusCancelled       = "cancelled"
)

// Valid reports whether a public history status is part of the wire contract.
func (s HistoryStatus) Valid() bool {
	switch s {
	case HistoryStatusOK, HistoryStatusPartial, HistoryStatusForbidden,
		HistoryStatusNotFound, HistoryStatusUnavailable, HistoryStatusInvalidArgument,
		HistoryStatusConflict, HistoryStatusCancelled:
		return true
	}
	return false
}

// SourceKind identifies a safe, model-visible history projection type.
type SourceKind string

const (
	SourceKindMessage    = "message"
	SourceKindToolCall   = "tool_call"
	SourceKindToolResult = "tool_result"
	SourceKindEvent      = "event"
	SourceKindSummary    = "summary"
	SourceKindFile       = "file"
	SourceKindDiff       = "diff"
)

// Valid reports whether a source kind is part of the bounded projection
// contract. System, credential, and reasoning payloads have no public kind.
func (k SourceKind) Valid() bool {
	switch k {
	case SourceKindMessage, SourceKindToolCall, SourceKindToolResult,
		SourceKindEvent, SourceKindSummary, SourceKindFile, SourceKindDiff:
		return true
	}
	return false
}

const (
	HistoryAuthorUser      = "user"
	HistoryAuthorAssistant = "assistant"
	HistoryAuthorTool      = "tool"
)

func validHistoryAuthor(author string) bool {
	switch author {
	case HistoryAuthorUser, HistoryAuthorAssistant, HistoryAuthorTool:
		return true
	}
	return false
}

// ContinuityLimits owns the SC-D4 finite defaults and their effective bounds.
// All byte values are UTF-8 bytes unless a field says otherwise.
type ContinuityLimits struct {
	SearchQueryBytes  int
	SearchPageDefault int
	SearchPageMax     int
	ReadPageDefault   int
	ReadPageMax       int

	CandidateRecords int
	CandidateBytes   int
	ResultItemBytes  int
	ResultPageBytes  int

	ReferencesPerTask    int
	ReferenceBytes       int
	ReferencesTotalBytes int
	SelectionRefs        int

	PresentPaths     int
	DescriptionBytes int
	PresentFileBytes int64
	PresentSetBytes  int64

	BinaryPageBytes  int
	OutstandingPages int
	ExplicitSessions int
	ExpandedSessions int
}

// DefaultContinuityLimits returns the immutable SC-D4 defaults. Callers may
// derive a smaller effective value, never a larger wire budget.
func DefaultContinuityLimits() ContinuityLimits {
	return ContinuityLimits{
		SearchQueryBytes: 512, SearchPageDefault: 20, SearchPageMax: 50,
		ReadPageDefault: 20, ReadPageMax: 100,
		CandidateRecords: 2000, CandidateBytes: 4 << 20,
		ResultItemBytes: 8 << 10, ResultPageBytes: 32 << 10,
		ReferencesPerTask: 8, ReferenceBytes: 16 << 10, ReferencesTotalBytes: 64 << 10, SelectionRefs: 100,
		PresentPaths: 20, DescriptionBytes: 512, PresentFileBytes: 32 << 20, PresentSetBytes: 128 << 20,
		BinaryPageBytes: 256 << 10, OutstandingPages: 1,
		ExplicitSessions: 20, ExpandedSessions: 100,
	}
}

// Effective returns limits capped by a known transport wire-frame budget.
// A non-positive frame means no narrower transport budget was supplied.
func (l ContinuityLimits) Effective(wireFrameBytes int) ContinuityLimits {
	if l == (ContinuityLimits{}) {
		l = DefaultContinuityLimits()
	}
	if wireFrameBytes > 0 && wireFrameBytes < l.ResultPageBytes {
		l.ResultPageBytes = wireFrameBytes
	}
	if l.ResultItemBytes > l.ResultPageBytes {
		l.ResultItemBytes = l.ResultPageBytes
	}
	return l
}

// HistoryScope is caller selection only. An empty scope means the current
// destination session; it is not a permission database.
type HistoryScope struct {
	SessionIDs []SessionID `json:"session_ids,omitempty"`
	Workspace  bool        `json:"workspace,omitempty"`
}

// Validate checks local, side-effect-free caller selection bounds.
func (s HistoryScope) Validate(limits ContinuityLimits) error {
	limits = limits.Effective(0)
	if len(s.SessionIDs) > limits.ExplicitSessions {
		return fmt.Errorf("history scope has %d explicit sessions; maximum is %d", len(s.SessionIDs), limits.ExplicitSessions)
	}
	return validateSessionIDs(s.SessionIDs, "history scope")
}

// AcceptedHistoryScope is the host-resolved, canonical task scope.
type AcceptedHistoryScope struct {
	DestinationSessionID SessionID   `json:"destination_session_id"`
	SourceSessionIDs     []SessionID `json:"source_session_ids"`
	ScopeHash            string      `json:"scope_hash"`
}

// NewAcceptedHistoryScope canonicalizes source IDs and derives a stable hash
// binding them to the destination session.
func NewAcceptedHistoryScope(destination SessionID, sources []SessionID) (AcceptedHistoryScope, error) {
	if destination == "" || !utf8.ValidString(string(destination)) {
		return AcceptedHistoryScope{}, fmt.Errorf("destination session ID is invalid")
	}
	canonical, err := canonicalSessionIDs(sources, DefaultContinuityLimits().ExpandedSessions, "accepted history scope")
	if err != nil {
		return AcceptedHistoryScope{}, err
	}
	if len(canonical) == 0 {
		return AcceptedHistoryScope{}, fmt.Errorf("accepted history scope requires at least one source session")
	}
	encoded, err := json.Marshal(struct {
		DestinationSessionID SessionID   `json:"destination_session_id"`
		SourceSessionIDs     []SessionID `json:"source_session_ids"`
	}{destination, canonical})
	if err != nil {
		return AcceptedHistoryScope{}, fmt.Errorf("encode accepted history scope: %w", err)
	}
	digest := sha256.Sum256(encoded)
	return AcceptedHistoryScope{
		DestinationSessionID: destination,
		SourceSessionIDs:     canonical,
		ScopeHash:            hex.EncodeToString(digest[:]),
	}, nil
}

// Validate verifies the accepted scope has its canonical, destination-bound
// hash. It does not make an authorization decision.
func (s AcceptedHistoryScope) Validate() error {
	limits := DefaultContinuityLimits()
	if s.DestinationSessionID == "" || !utf8.ValidString(string(s.DestinationSessionID)) {
		return fmt.Errorf("accepted history scope destination session ID is invalid")
	}
	if len(s.SourceSessionIDs) == 0 || len(s.SourceSessionIDs) > limits.ExpandedSessions {
		return fmt.Errorf("accepted history scope has %d source sessions; maximum is %d", len(s.SourceSessionIDs), limits.ExpandedSessions)
	}
	for i, id := range s.SourceSessionIDs {
		if id == "" || !utf8.ValidString(string(id)) {
			return fmt.Errorf("accepted history scope has an invalid source session ID")
		}
		if i > 0 && s.SourceSessionIDs[i-1] >= id {
			return fmt.Errorf("accepted history scope source session IDs are not canonical")
		}
	}
	canonical, err := NewAcceptedHistoryScope(s.DestinationSessionID, s.SourceSessionIDs)
	if err != nil || s.ScopeHash != canonical.ScopeHash {
		return fmt.Errorf("accepted history scope hash does not match canonical scope")
	}
	return nil
}

// SourceRef preserves the exact identity and safe projection kind of source
// material. RunID is absent for original user messages; MessageID is absent
// for non-message events.
type SourceRef struct {
	SessionID SessionID `json:"session_id"`
	RunID     RunID     `json:"run_id,omitempty"`
	MessageID string    `json:"message_id,omitempty"`
	EventSeq  EventSeq  `json:"event_seq,omitempty"`
	Kind      string    `json:"kind"`
	CreatedAt int64     `json:"created_at"`
}

// Validate checks that a public source reference names a supported kind.
func (r SourceRef) Validate() error {
	if r.SessionID == "" || !utf8.ValidString(string(r.SessionID)) {
		return fmt.Errorf("source reference session ID is invalid")
	}
	if !SourceKind(r.Kind).Valid() {
		return fmt.Errorf("source reference kind %q is unknown", r.Kind)
	}
	if r.EventSeq < 0 || r.CreatedAt < 0 {
		return fmt.Errorf("source reference has negative bounds")
	}
	return validateUTF8Strings("source reference", string(r.RunID), r.MessageID)
}

// HistoryItem is a redacted, bounded projection. Redacted and Truncated are
// independent because an item may have both properties.
type HistoryItem struct {
	Ref        SourceRef   `json:"ref"`
	Author     string      `json:"author"`
	Text       string      `json:"text"`
	SourceRefs []SourceRef `json:"source_refs,omitempty"`
	Redacted   bool        `json:"redacted"`
	Truncated  bool        `json:"truncated"`
}

// Validate checks a bounded, public projection and its bounded provenance.
func (i HistoryItem) Validate(limits ContinuityLimits) error {
	limits = limits.Effective(0)
	if err := i.Ref.Validate(); err != nil {
		return err
	}
	if !validHistoryAuthor(i.Author) {
		return fmt.Errorf("history item author %q is unknown", i.Author)
	}
	if err := validateUTF8Bounded("history item text", i.Text, limits.ResultItemBytes); err != nil {
		return err
	}
	if len(i.SourceRefs) > limits.SelectionRefs {
		return fmt.Errorf("history item has %d source refs; maximum is %d", len(i.SourceRefs), limits.SelectionRefs)
	}
	for _, ref := range i.SourceRefs {
		if err := ref.Validate(); err != nil {
			return err
		}
	}
	return validateJSONBytes("history item", i, limits.ResultItemBytes)
}

// HistoryRunRange is the event-range alternative in HistorySelection.
type HistoryRunRange struct {
	RunID   RunID    `json:"run_id"`
	FromSeq EventSeq `json:"from_seq"`
	ToSeq   EventSeq `json:"to_seq"`
}

// RunRange is retained as a concise alias for consumers of the ledger's run
// range shape.
type RunRange = HistoryRunRange

// HistorySelection names exactly one source session and exactly one selector.
type HistorySelection struct {
	SourceSessionID SessionID        `json:"source_session_id"`
	Refs            []SourceRef      `json:"refs,omitempty"`
	RunRange        *HistoryRunRange `json:"run_range,omitempty"`
}

// Validate rejects ambiguous, partial, or oversized selectors before any
// source inspection can occur.
func (s HistorySelection) Validate(limits ContinuityLimits) error {
	limits = limits.Effective(0)
	if s.SourceSessionID == "" || !utf8.ValidString(string(s.SourceSessionID)) {
		return fmt.Errorf("history selection source session ID is invalid")
	}
	hasRefs := len(s.Refs) > 0
	hasRange := s.RunRange != nil
	if hasRefs == hasRange {
		return fmt.Errorf("history selection requires exactly one of refs or run range")
	}
	if len(s.Refs) > limits.SelectionRefs {
		return fmt.Errorf("history selection has %d refs; maximum is %d", len(s.Refs), limits.SelectionRefs)
	}
	for _, ref := range s.Refs {
		if err := ref.Validate(); err != nil {
			return err
		}
		if ref.SessionID != s.SourceSessionID {
			return fmt.Errorf("history selection ref session differs from source session")
		}
	}
	if s.RunRange != nil {
		if s.RunRange.RunID == "" || s.RunRange.FromSeq <= 0 || s.RunRange.ToSeq <= 0 || s.RunRange.ToSeq < s.RunRange.FromSeq {
			return fmt.Errorf("history selection run range is invalid")
		}
	}
	return nil
}

// ReferenceSelection supplies a server-validated selector plus its expected
// sanitized-snapshot digest; no browser content is accepted.
type ReferenceSelection struct {
	Selection      HistorySelection `json:"selection"`
	ExpectedDigest string           `json:"expected_digest"`
}

func (s ReferenceSelection) Validate(limits ContinuityLimits) error {
	if err := s.Selection.Validate(limits); err != nil {
		return err
	}
	if s.ExpectedDigest == "" || !utf8.ValidString(s.ExpectedDigest) {
		return fmt.Errorf("reference selection expected digest is invalid")
	}
	return nil
}

type HistorySearchRequest struct {
	Query      string      `json:"query"`
	SessionIDs []SessionID `json:"session_ids,omitempty"`
	From       int64       `json:"from,omitempty"`
	To         int64       `json:"to,omitempty"`
	Kinds      []string    `json:"kinds,omitempty"`
	ArtifactID string      `json:"artifact_id,omitempty"`
	TaskID     string      `json:"task_id,omitempty"`
	Cursor     string      `json:"cursor,omitempty"`
	Limit      int         `json:"limit,omitempty"`
}

func (r HistorySearchRequest) Validate(limits ContinuityLimits) error {
	limits = limits.Effective(0)
	if err := validateUTF8Bounded("history search query", r.Query, limits.SearchQueryBytes); err != nil {
		return err
	}
	if err := validateSessionIDs(r.SessionIDs, "history search"); err != nil {
		return err
	}
	if len(r.SessionIDs) > limits.ExplicitSessions {
		return fmt.Errorf("history search has %d explicit sessions; maximum is %d", len(r.SessionIDs), limits.ExplicitSessions)
	}
	if r.From < 0 || r.To < 0 || (r.To != 0 && r.From != 0 && r.To < r.From) {
		return fmt.Errorf("history search time bounds are invalid")
	}
	if r.Limit < 0 || r.Limit > limits.SearchPageMax {
		return fmt.Errorf("history search limit is invalid")
	}
	for _, kind := range r.Kinds {
		if !SourceKind(kind).Valid() {
			return fmt.Errorf("history search kind %q is unknown", kind)
		}
	}
	return validateUTF8Strings("history search", r.ArtifactID, r.TaskID, r.Cursor)
}

type HistoryReadRequest struct {
	Selection   *HistorySelection `json:"selection,omitempty"`
	ReferenceID string            `json:"reference_id,omitempty"`
	Cursor      string            `json:"cursor,omitempty"`
	Limit       int               `json:"limit,omitempty"`
}

func (r HistoryReadRequest) Validate(limits ContinuityLimits) error {
	limits = limits.Effective(0)
	if (r.Selection != nil) == (r.ReferenceID != "") {
		return fmt.Errorf("history read requires exactly one of selection or reference ID")
	}
	if r.Selection != nil {
		if err := r.Selection.Validate(limits); err != nil {
			return err
		}
	}
	if r.Limit < 0 || r.Limit > limits.ReadPageMax {
		return fmt.Errorf("history read limit is invalid")
	}
	return validateUTF8Strings("history read", r.ReferenceID, r.Cursor)
}

type HistoryTraceRequest struct {
	SourceRef     *SourceRef `json:"source_ref,omitempty"`
	ReferenceID   string     `json:"reference_id,omitempty"`
	DeliverableID string     `json:"deliverable_id,omitempty"`
}

func (r HistoryTraceRequest) Validate(limits ContinuityLimits) error {
	selectors := 0
	if r.SourceRef != nil {
		selectors++
		if err := r.SourceRef.Validate(); err != nil {
			return err
		}
	}
	if r.ReferenceID != "" {
		selectors++
	}
	if r.DeliverableID != "" {
		selectors++
	}
	if selectors != 1 {
		return fmt.Errorf("history trace requires exactly one selector")
	}
	return validateUTF8Strings("history trace", r.ReferenceID, r.DeliverableID)
}

type HistoryPage struct {
	Status          string        `json:"status"`
	Items           []HistoryItem `json:"items"`
	NextCursor      string        `json:"next_cursor"`
	Truncated       bool          `json:"truncated"`
	Redacted        bool          `json:"redacted"`
	Warnings        []string      `json:"warnings"`
	Reason          *string       `json:"reason,omitempty"`
	SelectionDigest string        `json:"selection_digest,omitempty"`
}

func (p HistoryPage) Validate(limits ContinuityLimits) error {
	limits = limits.Effective(0)
	if !HistoryStatus(p.Status).Valid() {
		return fmt.Errorf("history page status %q is unknown", p.Status)
	}
	if len(p.Items) > limits.ReadPageMax {
		return fmt.Errorf("history page has %d items; maximum is %d", len(p.Items), limits.ReadPageMax)
	}
	for _, item := range p.Items {
		if err := item.Validate(limits); err != nil {
			return err
		}
	}
	if err := validateUTF8Strings("history page", p.NextCursor, p.SelectionDigest); err != nil {
		return err
	}
	for _, warning := range p.Warnings {
		if !utf8.ValidString(warning) {
			return fmt.Errorf("history page warning is not valid UTF-8")
		}
	}
	if p.Reason != nil && !utf8.ValidString(*p.Reason) {
		return fmt.Errorf("history page reason is not valid UTF-8")
	}
	return validateJSONBytes("history page", p, limits.ResultPageBytes)
}

type ReferencePreview struct {
	Selection    HistorySelection `json:"selection"`
	Items        []HistoryItem    `json:"items"`
	Digest       string           `json:"digest"`
	CapturedAt   int64            `json:"captured_at"`
	ByteCount    int              `json:"byte_count"`
	SourceStatus string           `json:"source_status"`
}

// Validate checks a preview is complete, bounded, and independently safe to
// compare with an attachment request.
func (p ReferencePreview) Validate(limits ContinuityLimits) error {
	limits = limits.Effective(0)
	if err := p.Selection.Validate(limits); err != nil {
		return err
	}
	if p.Digest == "" || !utf8.ValidString(p.Digest) || p.CapturedAt < 0 || p.ByteCount < 0 || p.ByteCount > limits.ReferenceBytes {
		return fmt.Errorf("reference preview metadata is invalid")
	}
	if !HistoryStatus(p.SourceStatus).Valid() {
		return fmt.Errorf("reference preview source status %q is unknown", p.SourceStatus)
	}
	if err := validateHistoryItems(p.Items, limits, limits.ReferenceBytes); err != nil {
		return err
	}
	return validateJSONBytes("reference preview", p, limits.ReferenceBytes)
}

type ContinuityInput struct {
	RequestID    string               `json:"request_id"`
	References   []ReferenceSelection `json:"references,omitempty"`
	HistoryScope HistoryScope         `json:"history_scope"`
}

func (i ContinuityInput) Validate(limits ContinuityLimits) error {
	limits = limits.Effective(0)
	if i.RequestID == "" || !utf8.ValidString(i.RequestID) {
		return fmt.Errorf("continuity request ID is invalid")
	}
	if len(i.References) > limits.ReferencesPerTask {
		return fmt.Errorf("continuity input has %d references; maximum is %d", len(i.References), limits.ReferencesPerTask)
	}
	if err := i.HistoryScope.Validate(limits); err != nil {
		return err
	}
	for _, reference := range i.References {
		if err := reference.Validate(limits); err != nil {
			return err
		}
	}
	return nil
}

// ValidateContextReferences enforces both per-reference and aggregate
// persisted-snapshot budgets without inspecting source storage.
func ValidateContextReferences(references []ContextReference, limits ContinuityLimits) error {
	limits = limits.Effective(0)
	if len(references) > limits.ReferencesPerTask {
		return fmt.Errorf("context reference set has %d references; maximum is %d", len(references), limits.ReferencesPerTask)
	}
	total := 0
	for _, reference := range references {
		if err := reference.Validate(limits); err != nil {
			return err
		}
		encoded, err := json.Marshal(reference)
		if err != nil {
			return fmt.Errorf("encode context reference: %w", err)
		}
		total += len(encoded)
		if total > limits.ReferencesTotalBytes {
			return fmt.Errorf("context reference set exceeds %d bytes", limits.ReferencesTotalBytes)
		}
	}
	return nil
}

func validateSessionIDs(ids []SessionID, label string) error {
	seen := make(map[SessionID]struct{}, len(ids))
	for _, id := range ids {
		if id == "" || !utf8.ValidString(string(id)) {
			return fmt.Errorf("%s has an invalid session ID", label)
		}
		if _, duplicate := seen[id]; duplicate {
			return fmt.Errorf("%s has a duplicate session ID", label)
		}
		seen[id] = struct{}{}
	}
	return nil
}

func canonicalSessionIDs(ids []SessionID, maximum int, label string) ([]SessionID, error) {
	if len(ids) > maximum {
		return nil, fmt.Errorf("%s has %d sessions; maximum is %d", label, len(ids), maximum)
	}
	canonical := append([]SessionID(nil), ids...)
	for _, id := range canonical {
		if id == "" || !utf8.ValidString(string(id)) {
			return nil, fmt.Errorf("%s has an invalid session ID", label)
		}
	}
	sort.Slice(canonical, func(i, j int) bool { return canonical[i] < canonical[j] })
	unique := canonical[:0]
	for _, id := range canonical {
		if len(unique) == 0 || unique[len(unique)-1] != id {
			unique = append(unique, id)
		}
	}
	if len(unique) > maximum {
		return nil, fmt.Errorf("%s has %d sessions; maximum is %d", label, len(unique), maximum)
	}
	return unique, nil
}

func validateUTF8Bounded(label, value string, maximum int) error {
	if !utf8.ValidString(value) {
		return fmt.Errorf("%s is not valid UTF-8", label)
	}
	if len(value) > maximum {
		return fmt.Errorf("%s exceeds %d bytes", label, maximum)
	}
	return nil
}

func validateUTF8Strings(label string, values ...string) error {
	for _, value := range values {
		if !utf8.ValidString(value) {
			return fmt.Errorf("%s contains invalid UTF-8", label)
		}
	}
	return nil
}

func validateHistoryItems(items []HistoryItem, limits ContinuityLimits, maximumBytes int) error {
	if len(items) == 0 || len(items) > limits.SelectionRefs {
		return fmt.Errorf("history item collection has %d items; maximum is %d", len(items), limits.SelectionRefs)
	}
	for _, item := range items {
		if err := item.Validate(limits); err != nil {
			return err
		}
	}
	return validateJSONBytes("history item collection", items, maximumBytes)
}

func validateJSONBytes(label string, value any, maximum int) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode %s: %w", label, err)
	}
	if len(encoded) > maximum {
		return fmt.Errorf("%s exceeds %d bytes", label, maximum)
	}
	return nil
}

// CanonicalSessionIDs exposes the stable ID order used in accepted scope
// hashes without turning a caller selection into an authorization decision.
func CanonicalSessionIDs(ids []SessionID) []SessionID {
	canonical, err := canonicalSessionIDs(ids, DefaultContinuityLimits().ExpandedSessions, "history scope")
	if err != nil {
		return nil
	}
	return canonical
}

// ScopeHash is a convenience for consumers which already have canonical IDs.
func ScopeHash(destination SessionID, sources []SessionID) (string, error) {
	accepted, err := NewAcceptedHistoryScope(destination, sources)
	if err != nil {
		return "", err
	}
	return accepted.ScopeHash, nil
}

// IsPublicHistoryKind reports whether a string is a public source kind.
func IsPublicHistoryKind(kind string) bool { return SourceKind(strings.TrimSpace(kind)).Valid() }
