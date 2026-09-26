// Package bml is the Go port of Diva's Basic Memory Layer: the stable
// normalized Memory v2 record, provenance, and integrity contracts.
package bml

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// MaxConfidenceBPS is the maximum confidence expressed as integer basis points.
const MaxConfidenceBPS uint16 = 10_000

// Kind is the normalized Memory record category.
type Kind string

const (
	KindIdentity          Kind = "identity"
	KindRelationship      Kind = "relationship"
	KindCommitment        Kind = "commitment"
	KindPreference        Kind = "preference"
	KindLongTerm          Kind = "long_term"
	KindHistory           Kind = "history"
	KindDaily             Kind = "daily"
	KindWeekly            Kind = "weekly"
	KindMonthly           Kind = "monthly"
	KindJournal           Kind = "journal"
	KindLearning          Kind = "learning"
	KindSessionCheckpoint Kind = "session_checkpoint"
	KindUnknown           Kind = "unknown"
)

var kindByName = map[string]Kind{
	"identity":           KindIdentity,
	"relationship":       KindRelationship,
	"commitment":         KindCommitment,
	"preference":         KindPreference,
	"long_term":          KindLongTerm,
	"history":            KindHistory,
	"daily":              KindDaily,
	"weekly":             KindWeekly,
	"monthly":            KindMonthly,
	"journal":            KindJournal,
	"learning":           KindLearning,
	"session_checkpoint": KindSessionCheckpoint,
	// Legacy serde alias.
	"working_memory": KindSessionCheckpoint,
	"unknown":        KindUnknown,
}

// UnmarshalJSON accepts the snake_case wire names, maps the retired
// "working_memory" alias to KindSessionCheckpoint, and falls back to
// KindUnknown for unrecognized values (serde `other` semantics).
func (k *Kind) UnmarshalJSON(data []byte) error {
	s, err := enumString(data)
	if err != nil {
		return err
	}
	if v, ok := kindByName[s]; ok {
		*k = v
	} else {
		*k = KindUnknown
	}
	return nil
}

// ProvenanceSource is the origin of normalized Memory content.
type ProvenanceSource string

const (
	ProvenanceSourceLaputaAppliedSection ProvenanceSource = "laputa_applied_section"
	ProvenanceSourceLegacyMarkdownOwner  ProvenanceSource = "legacy_markdown_owner"
	ProvenanceSourceUserInput            ProvenanceSource = "user_input"
	ProvenanceSourceToolResult           ProvenanceSource = "tool_result"
	ProvenanceSourceSessionSync          ProvenanceSource = "session_sync"
	ProvenanceSourceAutoDream            ProvenanceSource = "auto_dream"
	ProvenanceSourceContextCompaction    ProvenanceSource = "context_compaction"
	ProvenanceSourceFile                 ProvenanceSource = "file"
	ProvenanceSourceUnknown              ProvenanceSource = "unknown"
)

var provenanceSourceByName = map[string]ProvenanceSource{
	"laputa_applied_section": ProvenanceSourceLaputaAppliedSection,
	"legacy_markdown_owner":  ProvenanceSourceLegacyMarkdownOwner,
	"user_input":             ProvenanceSourceUserInput,
	"tool_result":            ProvenanceSourceToolResult,
	"session_sync":           ProvenanceSourceSessionSync,
	"auto_dream":             ProvenanceSourceAutoDream,
	"context_compaction":     ProvenanceSourceContextCompaction,
	"file":                   ProvenanceSourceFile,
	"unknown":                ProvenanceSourceUnknown,
}

func (s *ProvenanceSource) UnmarshalJSON(data []byte) error {
	v, err := enumString(data)
	if err != nil {
		return err
	}
	if mapped, ok := provenanceSourceByName[v]; ok {
		*s = mapped
	} else {
		*s = ProvenanceSourceUnknown
	}
	return nil
}

// debugName returns the Rust `{:?}` variant spelling used inside
// escaped prompt blocks.
func (s ProvenanceSource) debugName() string {
	switch s {
	case ProvenanceSourceLaputaAppliedSection:
		return "LaputaAppliedSection"
	case ProvenanceSourceLegacyMarkdownOwner:
		return "LegacyMarkdownOwner"
	case ProvenanceSourceUserInput:
		return "UserInput"
	case ProvenanceSourceToolResult:
		return "ToolResult"
	case ProvenanceSourceSessionSync:
		return "SessionSync"
	case ProvenanceSourceAutoDream:
		return "AutoDream"
	case ProvenanceSourceContextCompaction:
		return "ContextCompaction"
	case ProvenanceSourceFile:
		return "File"
	default:
		return "Unknown"
	}
}

// Sensitivity is the boundary checked before recall or rendering.
type Sensitivity string

const (
	SensitivityPublic     Sensitivity = "public"
	SensitivityInternal   Sensitivity = "internal"
	SensitivityPrivate    Sensitivity = "private"
	SensitivityRestricted Sensitivity = "restricted"
	SensitivityUnknown    Sensitivity = "unknown"
)

var sensitivityByName = map[string]Sensitivity{
	"public":     SensitivityPublic,
	"internal":   SensitivityInternal,
	"private":    SensitivityPrivate,
	"restricted": SensitivityRestricted,
	"unknown":    SensitivityUnknown,
}

func (s *Sensitivity) UnmarshalJSON(data []byte) error {
	v, err := enumString(data)
	if err != nil {
		return err
	}
	if mapped, ok := sensitivityByName[v]; ok {
		*s = mapped
	} else {
		*s = SensitivityUnknown
	}
	return nil
}

// Trust is assigned to a record without changing its underlying provenance.
type Trust string

const (
	TrustAppliedAuthority Trust = "applied_authority"
	TrustUserAsserted     Trust = "user_asserted"
	TrustObserved         Trust = "observed"
	TrustInferred         Trust = "inferred"
	TrustUntrusted        Trust = "untrusted"
	TrustUnknown          Trust = "unknown"
)

var trustByName = map[string]Trust{
	"applied_authority": TrustAppliedAuthority,
	"user_asserted":     TrustUserAsserted,
	"observed":          TrustObserved,
	"inferred":          TrustInferred,
	"untrusted":         TrustUntrusted,
	"unknown":           TrustUnknown,
}

func (t *Trust) UnmarshalJSON(data []byte) error {
	v, err := enumString(data)
	if err != nil {
		return err
	}
	if mapped, ok := trustByName[v]; ok {
		*t = mapped
	} else {
		*t = TrustUnknown
	}
	return nil
}

// debugName returns the Rust `{:?}` variant spelling used inside escaped
// prompt blocks.
func (t Trust) debugName() string {
	switch t {
	case TrustAppliedAuthority:
		return "AppliedAuthority"
	case TrustUserAsserted:
		return "UserAsserted"
	case TrustObserved:
		return "Observed"
	case TrustInferred:
		return "Inferred"
	case TrustUntrusted:
		return "Untrusted"
	default:
		return "Unknown"
	}
}

// DigestAlgorithm identifies the algorithm used to bind content to a decision.
type DigestAlgorithm string

const (
	DigestAlgorithmSHA256  DigestAlgorithm = "sha256"
	DigestAlgorithmUnknown DigestAlgorithm = "unknown"
)

func (a *DigestAlgorithm) UnmarshalJSON(data []byte) error {
	v, err := enumString(data)
	if err != nil {
		return err
	}
	if v == string(DigestAlgorithmSHA256) {
		*a = DigestAlgorithmSHA256
	} else {
		*a = DigestAlgorithmUnknown
	}
	return nil
}

// ContentDigest is a non-secret digest of the exact content covered by a
// decision.
type ContentDigest struct {
	Algorithm DigestAlgorithm `json:"algorithm"`
	Value     string          `json:"value"`
}

// AuditCorrelation carries the identifiers used to correlate governance
// activity with runtime audit data.
type AuditCorrelation struct {
	RequestID string  `json:"request_id"`
	TurnID    string  `json:"turn_id"`
	SessionID string  `json:"session_id"`
	TraceID   *string `json:"trace_id"`
}

// EvidenceSource is the origin of a governance evidence pointer. Unlike the
// other enums it has no `unknown` catch-all: unrecognized wire values are
// rejected, matching serde behavior for the Rust enum.
type EvidenceSource string

const (
	EvidenceSourceSession           EvidenceSource = "session"
	EvidenceSourceReport            EvidenceSource = "report"
	EvidenceSourceAutoDreamRun      EvidenceSource = "auto_dream_run"
	EvidenceSourceUserInput         EvidenceSource = "user_input"
	EvidenceSourceFile              EvidenceSource = "file"
	EvidenceSourceContextCompaction EvidenceSource = "context_compaction"
	EvidenceSourceExperienceJournal EvidenceSource = "experience_journal"
	EvidenceSourceRecallFeedback    EvidenceSource = "recall_feedback"
)

var evidenceSourceByName = map[string]EvidenceSource{
	"session":            EvidenceSourceSession,
	"report":             EvidenceSourceReport,
	"auto_dream_run":     EvidenceSourceAutoDreamRun,
	"user_input":         EvidenceSourceUserInput,
	"file":               EvidenceSourceFile,
	"context_compaction": EvidenceSourceContextCompaction,
	"experience_journal": EvidenceSourceExperienceJournal,
	"recall_feedback":    EvidenceSourceRecallFeedback,
}

func (s *EvidenceSource) UnmarshalJSON(data []byte) error {
	v, err := enumString(data)
	if err != nil {
		return err
	}
	mapped, ok := evidenceSourceByName[v]
	if !ok {
		return fmt.Errorf("bml: unknown evidence source %q", v)
	}
	*s = mapped
	return nil
}

// EvidenceRef is a typed pointer to bounded evidence used by governance
// review.
type EvidenceRef struct {
	ID        string         `json:"id"`
	Source    EvidenceSource `json:"source"`
	URI       string         `json:"uri"`
	Excerpt   *string        `json:"excerpt"`
	Hash      *string        `json:"hash"`
	CreatedAt time.Time      `json:"created_at"`
}

// MarshalJSON emits CreatedAt as RFC3339Nano UTC text.
func (e EvidenceRef) MarshalJSON() ([]byte, error) {
	type plain EvidenceRef
	p := plain(e)
	p.CreatedAt = e.CreatedAt.UTC()
	return json.Marshal(p)
}

// UnmarshalJSON normalizes CreatedAt to UTC (chrono DateTime<Utc> semantics).
func (e *EvidenceRef) UnmarshalJSON(data []byte) error {
	type plain EvidenceRef
	var p plain
	if err := json.Unmarshal(data, &p); err != nil {
		return err
	}
	p.CreatedAt = p.CreatedAt.UTC()
	*e = EvidenceRef(p)
	return nil
}

// IsPrimaryGovernanceEvidence reports whether evidence can act as primary
// authority evidence. Context compaction is session-local prompt survival: it
// can support review as secondary evidence but cannot be the sole authority
// behind durable changes.
func IsPrimaryGovernanceEvidence(e EvidenceRef) bool {
	return e.Source != EvidenceSourceContextCompaction
}

// Scope is the tenant and workspace ownership of a normalized record.
type Scope struct {
	TenantID    string  `json:"tenant_id"`
	WorkspaceID string  `json:"workspace_id"`
	SessionID   *string `json:"session_id"`
}

// Provenance is the immutable origin information for normalized Memory
// content.
type Provenance struct {
	Source        ProvenanceSource `json:"source"`
	SourceID      string           `json:"source_id"`
	ContentDigest ContentDigest    `json:"content_digest"`
	CapturedAt    time.Time        `json:"captured_at"`
	Correlation   AuditCorrelation `json:"correlation"`
}

// MarshalJSON emits CapturedAt as RFC3339Nano UTC text.
func (p Provenance) MarshalJSON() ([]byte, error) {
	type plain Provenance
	q := plain(p)
	q.CapturedAt = p.CapturedAt.UTC()
	return json.Marshal(q)
}

// UnmarshalJSON normalizes CapturedAt to UTC.
func (p *Provenance) UnmarshalJSON(data []byte) error {
	type plain Provenance
	var q plain
	if err := json.Unmarshal(data, &q); err != nil {
		return err
	}
	q.CapturedAt = q.CapturedAt.UTC()
	*p = Provenance(q)
	return nil
}

// Tombstone is a content-free marker that removes a previous record from
// active use.
type Tombstone struct {
	TargetRecordID string        `json:"target_record_id"`
	ReasonDigest   ContentDigest `json:"reason_digest"`
	ActorID        string        `json:"actor_id"`
	CreatedAt      time.Time     `json:"created_at"`
}

// MarshalJSON emits CreatedAt as RFC3339Nano UTC text.
func (t Tombstone) MarshalJSON() ([]byte, error) {
	type plain Tombstone
	p := plain(t)
	p.CreatedAt = t.CreatedAt.UTC()
	return json.Marshal(p)
}

// UnmarshalJSON normalizes CreatedAt to UTC.
func (t *Tombstone) UnmarshalJSON(data []byte) error {
	type plain Tombstone
	var p plain
	if err := json.Unmarshal(data, &p); err != nil {
		return err
	}
	p.CreatedAt = p.CreatedAt.UTC()
	*t = Tombstone(p)
	return nil
}

// Record is the stable normalized Memory v2 record.
type Record struct {
	ID            string        `json:"id"`
	Kind          Kind          `json:"kind"`
	Content       string        `json:"content"`
	Provenance    Provenance    `json:"provenance"`
	EvidenceRefs  []EvidenceRef `json:"evidence_refs"`
	ConfidenceBPS uint16        `json:"confidence_bps"`
	Sensitivity   Sensitivity   `json:"sensitivity"`
	Trust         Trust         `json:"trust"`
	Scope         Scope         `json:"scope"`
	CreatedAt     time.Time     `json:"created_at"`
	EffectiveAt   time.Time     `json:"effective_at"`
	ExpiresAt     *time.Time    `json:"expires_at"`
	Supersedes    []string      `json:"supersedes"`
	Tombstone     *Tombstone    `json:"tombstone"`
}

// MarshalJSON emits timestamps as RFC3339Nano UTC text and nil slices as
// empty arrays, matching the serde wire contract.
func (r Record) MarshalJSON() ([]byte, error) {
	type plain Record
	p := plain(r)
	p.CreatedAt = r.CreatedAt.UTC()
	p.EffectiveAt = r.EffectiveAt.UTC()
	if r.ExpiresAt != nil {
		expires := r.ExpiresAt.UTC()
		p.ExpiresAt = &expires
	}
	if p.EvidenceRefs == nil {
		p.EvidenceRefs = []EvidenceRef{}
	}
	if p.Supersedes == nil {
		p.Supersedes = []string{}
	}
	return json.Marshal(p)
}

// UnmarshalJSON normalizes all timestamps to UTC.
func (r *Record) UnmarshalJSON(data []byte) error {
	type plain Record
	var p plain
	if err := json.Unmarshal(data, &p); err != nil {
		return err
	}
	p.CreatedAt = p.CreatedAt.UTC()
	p.EffectiveAt = p.EffectiveAt.UTC()
	if p.ExpiresAt != nil {
		expires := p.ExpiresAt.UTC()
		p.ExpiresAt = &expires
	}
	*r = Record(p)
	return nil
}

// ValidationError is a stable, machine-matchable reason a normalized Memory
// record is rejected. Code is the snake_case variant name from the Rust
// contract; Field is set only for missing_required_field.
type ValidationError struct {
	Code  string
	Field string
}

var validationMessages = map[string]string{
	"unknown_record_kind":        "unknown record kind",
	"unknown_provenance_source":  "unknown provenance source",
	"unknown_sensitivity":        "unknown sensitivity",
	"unknown_trust":              "unknown trust",
	"unknown_digest_algorithm":   "unknown digest algorithm",
	"content_digest_mismatch":    "record content does not match its provenance digest",
	"confidence_out_of_range":    "confidence basis points exceed 10000",
	"created_in_future":          "record creation time exceeds the allowed clock skew",
	"effective_before_creation":  "effective time is earlier than creation time",
	"invalid_expiry":             "expiry must be later than effective time",
	"self_supersedes":            "record cannot supersede itself",
	"tombstone_contains_content": "tombstone records must not contain content",
	"tombstone_target_mismatch":  "tombstone target must match one superseded record",
	"invalid_authority_source":   "provenance source cannot be applied authority",
	"invalid_authority_evidence": "evidence source cannot be applied authority",
	"workspace_mismatch":         "record belongs to a different workspace",
}

func (e *ValidationError) Error() string {
	if e.Code == "missing_required_field" {
		return "missing required field: " + e.Field
	}
	if msg, ok := validationMessages[e.Code]; ok {
		return msg
	}
	return e.Code
}

// MarshalJSON mirrors serde's externally tagged enum encoding: unit variants
// serialize as the code string, while missing_required_field carries its field
// name as `{"missing_required_field": "<field>"}`.
func (e ValidationError) MarshalJSON() ([]byte, error) {
	if e.Code == "missing_required_field" {
		return json.Marshal(map[string]string{e.Code: e.Field})
	}
	return json.Marshal(e.Code)
}

// UnmarshalJSON accepts both wire forms emitted by MarshalJSON.
func (e *ValidationError) UnmarshalJSON(data []byte) error {
	var code string
	if err := json.Unmarshal(data, &code); err == nil {
		e.Code = code
		e.Field = ""
		return nil
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	for code, field := range m {
		e.Code = code
		e.Field = field
		return nil
	}
	return fmt.Errorf("bml: invalid validation error encoding: %s", data)
}

// ValidateAt validates this record at a caller-supplied clock boundary,
// returning the first violation found or nil.
func (r *Record) ValidateAt(now time.Time, skew time.Duration) *ValidationError {
	if err := required("id", r.ID); err != nil {
		return err
	}
	if err := required("scope.tenant_id", r.Scope.TenantID); err != nil {
		return err
	}
	if err := required("scope.workspace_id", r.Scope.WorkspaceID); err != nil {
		return err
	}
	if r.Scope.SessionID != nil {
		if err := required("scope.session_id", *r.Scope.SessionID); err != nil {
			return err
		}
	}
	if err := required("provenance.source_id", r.Provenance.SourceID); err != nil {
		return err
	}
	if err := validateCorrelation(r.Provenance.Correlation); err != nil {
		return err
	}
	if err := validateDigest(r.Provenance.ContentDigest); err != nil {
		return err
	}
	if r.Tombstone == nil && r.Provenance.ContentDigest != MemoryContentDigest([]byte(r.Content)) {
		return &ValidationError{Code: "content_digest_mismatch"}
	}
	if r.Kind == KindUnknown {
		return &ValidationError{Code: "unknown_record_kind"}
	}
	if r.Provenance.Source == ProvenanceSourceUnknown {
		return &ValidationError{Code: "unknown_provenance_source"}
	}
	if r.Sensitivity == SensitivityUnknown {
		return &ValidationError{Code: "unknown_sensitivity"}
	}
	if r.Trust == TrustUnknown {
		return &ValidationError{Code: "unknown_trust"}
	}
	if r.ConfidenceBPS > MaxConfidenceBPS {
		return &ValidationError{Code: "confidence_out_of_range"}
	}
	if r.CreatedAt.After(now.Add(skew)) {
		return &ValidationError{Code: "created_in_future"}
	}
	if r.EffectiveAt.Before(r.CreatedAt) {
		return &ValidationError{Code: "effective_before_creation"}
	}
	if r.ExpiresAt != nil && !r.ExpiresAt.After(r.EffectiveAt) {
		return &ValidationError{Code: "invalid_expiry"}
	}
	for _, id := range r.Supersedes {
		if id == r.ID {
			return &ValidationError{Code: "self_supersedes"}
		}
	}
	if r.Tombstone != nil {
		if err := validateTombstone(*r.Tombstone); err != nil {
			return err
		}
		if r.Content != "" {
			return &ValidationError{Code: "tombstone_contains_content"}
		}
		found := false
		for _, id := range r.Supersedes {
			if id == r.Tombstone.TargetRecordID {
				found = true
				break
			}
		}
		if !found {
			return &ValidationError{Code: "tombstone_target_mismatch"}
		}
	}
	if r.Trust == TrustAppliedAuthority {
		switch r.Provenance.Source {
		case ProvenanceSourceLaputaAppliedSection,
			ProvenanceSourceLegacyMarkdownOwner,
			ProvenanceSourceAutoDream:
		default:
			return &ValidationError{Code: "invalid_authority_source"}
		}
		for _, evidence := range r.EvidenceRefs {
			if evidence.Source == EvidenceSourceAutoDreamRun ||
				evidence.Source == EvidenceSourceContextCompaction {
				return &ValidationError{Code: "invalid_authority_evidence"}
			}
		}
	}
	return nil
}

// ValidateWorkspace fails when a caller tries to mix records from another
// workspace.
func (r *Record) ValidateWorkspace(workspaceID string) *ValidationError {
	if err := required("workspace_id", workspaceID); err != nil {
		return err
	}
	if r.Scope.WorkspaceID != workspaceID {
		return &ValidationError{Code: "workspace_mismatch"}
	}
	return nil
}

// MemoryContentDigest computes the canonical SHA-256 digest used by Memory
// adapters (memory_content_digest in Diva core).
func MemoryContentDigest(content []byte) ContentDigest {
	sum := sha256.Sum256(content)
	return ContentDigest{
		Algorithm: DigestAlgorithmSHA256,
		Value:     hex.EncodeToString(sum[:]),
	}
}

// EscapeMemoryForPrompt wraps untrusted content in a non-executable,
// length-delimited prompt block.
func EscapeMemoryForPrompt(r *Record) string {
	content := strings.ReplaceAll(r.Content, "</memory-data>", "&lt;/memory-data&gt;")
	return fmt.Sprintf(
		"<memory-data id=\"%s\" source=\"%s\" trust=\"%s\" bytes=\"%d\">\n%s\n</memory-data>",
		escapeAttribute(r.ID),
		r.Provenance.Source.debugName(),
		r.Trust.debugName(),
		len(r.Content),
		content,
	)
}

// IntegritySeverity is the severity of a normalized Memory integrity finding.
// There is no `unknown` catch-all; unrecognized wire values are rejected.
type IntegritySeverity string

const (
	IntegritySeverityWarning IntegritySeverity = "warning"
	IntegritySeverityError   IntegritySeverity = "error"
)

func (s *IntegritySeverity) UnmarshalJSON(data []byte) error {
	v, err := enumString(data)
	if err != nil {
		return err
	}
	if v != string(IntegritySeverityWarning) && v != string(IntegritySeverityError) {
		return fmt.Errorf("bml: unknown integrity severity %q", v)
	}
	*s = IntegritySeverity(v)
	return nil
}

// IntegrityFinding is a stable, machine-matchable integrity finding.
type IntegrityFinding struct {
	Code     string            `json:"code"`
	Severity IntegritySeverity `json:"severity"`
	RecordID *string           `json:"record_id"`
	SourceID *string           `json:"source_id"`
}

// IntegrityReport is the comparison and migration integrity summary.
type IntegrityReport struct {
	SourceDigest          ContentDigest      `json:"source_digest"`
	NormalizedDigest      ContentDigest      `json:"normalized_digest"`
	SourceRecordCount     uint64             `json:"source_record_count"`
	NormalizedRecordCount uint64             `json:"normalized_record_count"`
	DuplicateRecordIDs    []string           `json:"duplicate_record_ids"`
	BrokenSupersedes      []string           `json:"broken_supersedes"`
	ExpiredRecordCount    uint64             `json:"expired_record_count"`
	TombstoneRecordCount  uint64             `json:"tombstone_record_count"`
	Findings              []IntegrityFinding `json:"findings"`
}

// MarshalJSON emits nil slices as empty arrays, matching the serde contract.
func (r IntegrityReport) MarshalJSON() ([]byte, error) {
	type plain IntegrityReport
	p := plain(r)
	if p.DuplicateRecordIDs == nil {
		p.DuplicateRecordIDs = []string{}
	}
	if p.BrokenSupersedes == nil {
		p.BrokenSupersedes = []string{}
	}
	if p.Findings == nil {
		p.Findings = []IntegrityFinding{}
	}
	return json.Marshal(p)
}

// DefaultL1IndexLines is the default L1 startup index budget (B2): maximum
// index lines injected into the system prompt. Full entries are never
// injected; retrieval goes through memory_search / memory_list.
const DefaultL1IndexLines = 30

// l1PreviewChars is the maximum number of characters of the content preview
// in one L1 index line.
const l1PreviewChars = 80

// L1IndexEntry is one (id, content) pair rendered into the L1 startup index.
type L1IndexEntry struct {
	ID      string
	Content string
}

// RenderL1IndexLine renders a single L1 index line:
// "- [id] <first line of content, truncated>".
func RenderL1IndexLine(id, content string) string {
	firstLine := strings.TrimSpace(strings.SplitN(content, "\n", 2)[0])
	runes := []rune(firstLine)
	preview := firstLine
	if len(runes) > l1PreviewChars {
		preview = string(runes[:l1PreviewChars]) + "…"
	}
	return "- [" + escapeAttribute(id) + "] " + preview
}

// RenderL1IndexBlock renders the bounded L1 startup index block with a
// retrieval pointer hint. The block never contains full entries; the model
// must use memory_search or memory_list with a record id to retrieve details.
// Empty entries or a zero budget render nothing.
func RenderL1IndexBlock(entries []L1IndexEntry, maxLines int) string {
	if len(entries) == 0 || maxLines == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("## Long-term Memory Index\n\n")
	b.WriteString("Full entries are not injected; use memory_search or memory_list with a record id to retrieve details.\n\n")
	for _, entry := range entries[:min(maxLines, len(entries))] {
		b.WriteString(RenderL1IndexLine(entry.ID, entry.Content))
		b.WriteByte('\n')
	}
	return b.String()
}

func validateTombstone(t Tombstone) *ValidationError {
	if err := required("tombstone.target_record_id", t.TargetRecordID); err != nil {
		return err
	}
	if err := required("tombstone.actor_id", t.ActorID); err != nil {
		return err
	}
	return validateDigest(t.ReasonDigest)
}

func validateCorrelation(c AuditCorrelation) *ValidationError {
	if err := required("correlation.request_id", c.RequestID); err != nil {
		return err
	}
	if err := required("correlation.turn_id", c.TurnID); err != nil {
		return err
	}
	return required("correlation.session_id", c.SessionID)
}

func validateDigest(d ContentDigest) *ValidationError {
	if d.Algorithm == DigestAlgorithmUnknown {
		return &ValidationError{Code: "unknown_digest_algorithm"}
	}
	return required("digest.value", d.Value)
}

func required(field, value string) *ValidationError {
	if strings.TrimSpace(value) == "" {
		return &ValidationError{Code: "missing_required_field", Field: field}
	}
	return nil
}

var attributeEscaper = strings.NewReplacer(
	"&", "&amp;",
	`"`, "&quot;",
	"<", "&lt;",
	">", "&gt;",
)

func escapeAttribute(value string) string {
	return attributeEscaper.Replace(value)
}

func enumString(data []byte) (string, error) {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return "", err
	}
	// serde accepts only strings for these enums; null decodes as "" above.
	if s == "" && strings.TrimSpace(string(data)) != `""` {
		return "", fmt.Errorf("bml: expected string enum value, got %s", data)
	}
	return s, nil
}
