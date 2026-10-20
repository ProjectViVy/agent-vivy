// migrate.go ports Diva's memory_records.rs: read-only normalization
// adapters (legacy Markdown owner files and applied Laputa sections) plus
// the isolated Memory v2 migration artifact writer with dry-run, conflict
// detection, and fail-closed rollback.
package bml

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// MigrationErrorCode is a stable, machine-matchable failure code carrying
// the LaputaError::code() string from the Rust contract.
type MigrationErrorCode string

const (
	ErrMigrationIO              MigrationErrorCode = "io"
	ErrMigrationJSON            MigrationErrorCode = "json"
	ErrInvalidMigrationID       MigrationErrorCode = "invalid_memory_migration_id"
	ErrMigrationConflict        MigrationErrorCode = "memory_migration_conflict"
	ErrInjectedMigrationFailure MigrationErrorCode = "injected_memory_migration_failure"
)

// MigrationError is a typed migration-adapter failure. Message never
// contains Memory content bytes.
type MigrationError struct {
	Code        MigrationErrorCode
	MigrationID string
	Path        string
	Message     string
	Err         error
}

func (e *MigrationError) Error() string { return e.Message }
func (e *MigrationError) Unwrap() error { return e.Err }

func migrationIOErr(path string, err error) *MigrationError {
	return &MigrationError{
		Code:    ErrMigrationIO,
		Path:    path,
		Message: fmt.Sprintf("I/O error at %s: %v", path, err),
		Err:     err,
	}
}

func migrationJSONErr(err error) *MigrationError {
	return &MigrationError{
		Code:    ErrMigrationJSON,
		Message: "JSON serialization error: " + err.Error(),
		Err:     err,
	}
}

func invalidMigrationIDErr(migrationID string) *MigrationError {
	return &MigrationError{
		Code:        ErrInvalidMigrationID,
		MigrationID: migrationID,
		Message:     "invalid migration id: " + migrationID,
	}
}

func migrationConflictErr(migrationID string) *MigrationError {
	return &MigrationError{
		Code:        ErrMigrationConflict,
		MigrationID: migrationID,
		Message:     "memory migration conflict: " + migrationID,
	}
}

func injectedMigrationFailureErr() *MigrationError {
	return &MigrationError{
		Code:    ErrInjectedMigrationFailure,
		Message: "injected memory migration failure",
	}
}

// SectionStatus is the lifecycle flag of a legacy `.laputa/sections/*.json`
// projection. Retired as production authority by the cognitive clean break;
// retained only as the import shape consumed by the offline migration tool.
type SectionStatus string

const (
	SectionStatusOwned SectionStatus = "owned"
	SectionStatusTbd   SectionStatus = "tbd"
)

// UnmarshalJSON rejects unrecognized values (serde enum semantics — no
// catch-all variant).
func (s *SectionStatus) UnmarshalJSON(data []byte) error {
	v, err := enumString(data)
	if err != nil {
		return err
	}
	if v != string(SectionStatusOwned) && v != string(SectionStatusTbd) {
		return fmt.Errorf("bml: unknown section status %q", v)
	}
	*s = SectionStatus(v)
	return nil
}

// LaputaSectionName is a canonical legacy Laputa section name, retired as
// production authority and retained only as the import vocabulary consumed
// by the offline migration tool.
type LaputaSectionName string

const (
	LaputaSectionIdentity     LaputaSectionName = "identity"
	LaputaSectionRelationship LaputaSectionName = "relationship"
	LaputaSectionCommitment   LaputaSectionName = "commitment"
	LaputaSectionPreferences  LaputaSectionName = "preferences"
	LaputaSectionMemoryMd     LaputaSectionName = "memory_md"
	LaputaSectionDaily        LaputaSectionName = "daily"
	LaputaSectionWeekly       LaputaSectionName = "weekly"
	LaputaSectionMonthly      LaputaSectionName = "monthly"
	LaputaSectionChangelog    LaputaSectionName = "changelog"
)

var laputaSectionNameByName = map[string]LaputaSectionName{
	"identity":     LaputaSectionIdentity,
	"relationship": LaputaSectionRelationship,
	"commitment":   LaputaSectionCommitment,
	"preferences":  LaputaSectionPreferences,
	"memory_md":    LaputaSectionMemoryMd,
	"daily":        LaputaSectionDaily,
	"weekly":       LaputaSectionWeekly,
	"monthly":      LaputaSectionMonthly,
	"changelog":    LaputaSectionChangelog,
}

// UnmarshalJSON rejects unrecognized section names (serde enum semantics).
func (n *LaputaSectionName) UnmarshalJSON(data []byte) error {
	v, err := enumString(data)
	if err != nil {
		return err
	}
	if mapped, ok := laputaSectionNameByName[v]; ok {
		*n = mapped
		return nil
	}
	return fmt.Errorf("bml: unknown laputa section %q", v)
}

// LaputaSection is the legacy `.laputa/sections/*.json` section projection.
type LaputaSection struct {
	Name         LaputaSectionName `json:"name"`
	Status       SectionStatus     `json:"status"`
	Content      json.RawMessage   `json:"content"`
	Metadata     json.RawMessage   `json:"metadata"`
	LastModified *time.Time        `json:"last_modified"`
	Version      string            `json:"version"`
}

// MarshalJSON emits LastModified in chrono serde form.
func (s LaputaSection) MarshalJSON() ([]byte, error) {
	type wire struct {
		Name         LaputaSectionName `json:"name"`
		Status       SectionStatus     `json:"status"`
		Content      json.RawMessage   `json:"content"`
		Metadata     json.RawMessage   `json:"metadata"`
		LastModified *string           `json:"last_modified"`
		Version      string            `json:"version"`
	}
	w := wire{
		Name:     s.Name,
		Status:   s.Status,
		Content:  s.Content,
		Metadata: s.Metadata,
		Version:  s.Version,
	}
	if s.LastModified != nil {
		modified := chronoSerdeTime(*s.LastModified)
		w.LastModified = &modified
	}
	return marshalCanonical(w)
}

// UnmarshalJSON normalizes LastModified to UTC.
func (s *LaputaSection) UnmarshalJSON(data []byte) error {
	type plain LaputaSection
	var p plain
	if err := json.Unmarshal(data, &p); err != nil {
		return err
	}
	if p.LastModified != nil {
		modified := p.LastModified.UTC()
		p.LastModified = &modified
	}
	*s = LaputaSection(p)
	return nil
}

// MemoryAdapterContext carries the inputs required to assign ownership and
// audit correlation during adaptation.
type MemoryAdapterContext struct {
	TenantID    string
	WorkspaceID string
	SessionID   *string
	Correlation AuditCorrelation
	CapturedAt  time.Time
}

// MemoryAdapterOutput is the result of adapting one source without
// modifying it.
type MemoryAdapterOutput struct {
	Records  []Record
	Findings []IntegrityFinding
}

// AdaptLegacyMarkdown normalizes one legacy owner Markdown file
// (adapt_legacy_markdown in memory_records.rs).
func AdaptLegacyMarkdown(path, content string, kind Kind, context *MemoryAdapterContext, legacyOwnerSelected bool) MemoryAdapterOutput {
	sourceID := strings.ReplaceAll(path, "\\", "/")
	confidence := uint16(5_000)
	trust := TrustUntrusted
	if legacyOwnerSelected {
		confidence = MaxConfidenceBPS
		trust = TrustAppliedAuthority
	}
	record := Record{
		ID:      opaqueID("memory"),
		Kind:    kind,
		Content: content,
		Provenance: Provenance{
			Source:      ProvenanceSourceLegacyMarkdownOwner,
			SourceID:    sourceID,
			CapturedAt:  context.CapturedAt,
			Correlation: context.Correlation,
		},
		EvidenceRefs:  []EvidenceRef{},
		ConfidenceBPS: confidence,
		Sensitivity:   SensitivityPrivate,
		Trust:         trust,
		Scope: Scope{
			TenantID:    context.TenantID,
			WorkspaceID: context.WorkspaceID,
			SessionID:   context.SessionID,
		},
		CreatedAt:   context.CapturedAt,
		EffectiveAt: context.CapturedAt,
		ExpiresAt:   nil,
		Supersedes:  []string{},
		Tombstone:   nil,
	}
	return MemoryAdapterOutput{
		Records:  []Record{record},
		Findings: []IntegrityFinding{},
	}
}

// AdaptLaputaSection normalizes an applied Laputa section and preserves
// unknown top-level keys as warnings (adapt_laputa_section in
// memory_records.rs). Non-owned sections produce no records.
func AdaptLaputaSection(section *LaputaSection, rawSection any, context *MemoryAdapterContext) (*MemoryAdapterOutput, error) {
	findings := unknownSectionFindings(rawSection, string(section.Name))
	if section.Status != SectionStatusOwned {
		sourceID := string(section.Name)
		findings = append(findings, IntegrityFinding{
			Code:     "laputa_section_not_owned",
			Severity: IntegritySeverityWarning,
			RecordID: nil,
			SourceID: &sourceID,
		})
		return &MemoryAdapterOutput{Records: []Record{}, Findings: findings}, nil
	}

	content, err := sectionContentString(section.Content)
	if err != nil {
		return nil, err
	}
	sourceID := string(section.Name)
	createdAt := context.CapturedAt
	if section.LastModified != nil {
		createdAt = *section.LastModified
	}
	record := Record{
		ID:      opaqueID("memory"),
		Kind:    recordKindForSection(section.Name),
		Content: content,
		Provenance: Provenance{
			Source:      ProvenanceSourceLaputaAppliedSection,
			SourceID:    sourceID,
			CapturedAt:  context.CapturedAt,
			Correlation: context.Correlation,
		},
		EvidenceRefs:  []EvidenceRef{},
		ConfidenceBPS: MaxConfidenceBPS,
		Sensitivity:   SensitivityPrivate,
		Trust:         TrustAppliedAuthority,
		Scope: Scope{
			TenantID:    context.TenantID,
			WorkspaceID: context.WorkspaceID,
			SessionID:   context.SessionID,
		},
		CreatedAt:   createdAt,
		EffectiveAt: createdAt,
		ExpiresAt:   nil,
		Supersedes:  []string{},
		Tombstone:   nil,
	}
	return &MemoryAdapterOutput{
		Records:  []Record{record},
		Findings: findings,
	}, nil
}

// sectionContentString extracts the section content: a JSON string decodes
// to its value; any other JSON value re-serializes in serde compact form.
func sectionContentString(content json.RawMessage) (string, error) {
	var text string
	if err := json.Unmarshal(content, &text); err == nil {
		return text, nil
	}
	var value any
	if err := json.Unmarshal(content, &value); err != nil {
		return "", migrationJSONErr(err)
	}
	encoded, err := marshalCanonical(value)
	if err != nil {
		return "", migrationJSONErr(err)
	}
	return string(encoded), nil
}

// CompareNormalizedRecords compares source bytes and normalized records
// without changing either side (compare_normalized_records in
// memory_records.rs).
func CompareNormalizedRecords(sourceBytes []byte, records []Record, findings []IntegrityFinding, now time.Time) (*IntegrityReport, error) {
	seen := make(map[string]bool, len(records))
	duplicates := make(map[string]bool)
	for _, record := range records {
		if seen[record.ID] {
			duplicates[record.ID] = true
		} else {
			seen[record.ID] = true
		}
	}
	broken := make(map[string]bool)
	for _, record := range records {
		for _, id := range record.Supersedes {
			if !seen[id] {
				broken[id] = true
			}
		}
	}
	duplicateIDs := sortedKeys(duplicates)
	brokenIDs := sortedKeys(broken)
	for _, id := range duplicateIDs {
		findings = append(findings, IntegrityFinding{
			Code:     "duplicate_record_id",
			Severity: IntegritySeverityError,
			RecordID: &id,
			SourceID: nil,
		})
	}
	for _, id := range brokenIDs {
		findings = append(findings, IntegrityFinding{
			Code:     "broken_supersedes",
			Severity: IntegritySeverityError,
			RecordID: &id,
			SourceID: nil,
		})
	}
	sourceRecordCount := uint64(0)
	if len(sourceBytes) > 0 {
		sourceRecordCount = 1
	}
	var expired, tombstoned uint64
	for _, record := range records {
		if record.ExpiresAt != nil && !record.ExpiresAt.After(now) {
			expired++
		}
		if record.Tombstone != nil {
			tombstoned++
		}
	}
	return &IntegrityReport{
		SourceRecordCount:     sourceRecordCount,
		NormalizedRecordCount: uint64(len(records)),
		DuplicateRecordIDs:    duplicateIDs,
		BrokenSupersedes:      brokenIDs,
		ExpiredRecordCount:    expired,
		TombstoneRecordCount:  tombstoned,
		Findings:              findings,
	}, nil
}

// recordKindForSection maps a section to its record kind. Daily, Weekly, and
// Monthly are report surfaces, not memories: reports are never normalized
// into typed memory records.
func recordKindForSection(section LaputaSectionName) Kind {
	switch section {
	case LaputaSectionIdentity:
		return KindIdentity
	case LaputaSectionRelationship:
		return KindRelationship
	case LaputaSectionCommitment:
		return KindCommitment
	case LaputaSectionPreferences:
		return KindPreference
	case LaputaSectionMemoryMd:
		return KindLongTerm
	default:
		return KindUnknown
	}
}

var knownSectionFields = map[string]bool{
	"name":          true,
	"status":        true,
	"content":       true,
	"metadata":      true,
	"last_modified": true,
	"version":       true,
}

// unknownSectionFindings warns about unrecognized top-level keys of the raw
// section document (unknown_section_findings in memory_records.rs).
func unknownSectionFindings(rawSection any, sourceID string) []IntegrityFinding {
	var keys []string
	switch raw := rawSection.(type) {
	case map[string]any:
		for key := range raw {
			keys = append(keys, key)
		}
	case json.RawMessage:
		var object map[string]any
		if err := json.Unmarshal(raw, &object); err == nil {
			for key := range object {
				keys = append(keys, key)
			}
		}
	case []byte:
		var object map[string]any
		if err := json.Unmarshal(raw, &object); err == nil {
			for key := range object {
				keys = append(keys, key)
			}
		}
	}
	sort.Strings(keys)
	findings := []IntegrityFinding{}
	for _, key := range keys {
		if knownSectionFields[key] {
			continue
		}
		keySourceID := sourceID
		findings = append(findings, IntegrityFinding{
			Code:     "unknown_laputa_field:" + key,
			Severity: IntegritySeverityWarning,
			RecordID: nil,
			SourceID: &keySourceID,
		})
	}
	return findings
}

// recordsJSON serializes records as serde_json::to_vec does: compact
// canonical bytes with an empty slice rendering as "[]", not "null".
func recordsJSON(records []Record) ([]byte, error) {
	if records == nil {
		records = []Record{}
	}
	encoded, err := marshalCanonical(records)
	if err != nil {
		return nil, migrationJSONErr(err)
	}
	return encoded, nil
}

// recordsForJSON keeps nil slices serializing as [] through atomicWriteJSON.
func recordsForJSON(records []Record) []Record {
	if records == nil {
		return []Record{}
	}
	return records
}

func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// requireJSONFields enforces serde's missing-field rejection: every named
// key must be present in the object.
func requireJSONFields(data []byte, fields ...string) error {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		return err
	}
	for _, field := range fields {
		if _, ok := object[field]; !ok {
			return fmt.Errorf("missing field `%s`", field)
		}
	}
	return nil
}
