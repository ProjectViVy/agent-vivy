// migrate.go ports Diva's memory_records.rs: read-only normalization
// adapters (legacy Markdown owner files and applied Laputa sections) plus
// the isolated Memory v2 migration artifact writer with dry-run, conflict
// detection, and fail-closed rollback.
package bml

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"
)

const artifactSchemaVersion = "1.0.0"

var artifactOutputs = []string{"records.json", "rollback.json", "manifest.json"}

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
	digest := MemoryContentDigest([]byte(content))
	sourceID := strings.ReplaceAll(path, "\\", "/")
	confidence := uint16(5_000)
	trust := TrustUntrusted
	if legacyOwnerSelected {
		confidence = MaxConfidenceBPS
		trust = TrustAppliedAuthority
	}
	record := Record{
		ID:      deterministicRecordID("legacy", sourceID, digest),
		Kind:    kind,
		Content: content,
		Provenance: Provenance{
			Source:        ProvenanceSourceLegacyMarkdownOwner,
			SourceID:      sourceID,
			ContentDigest: digest,
			CapturedAt:    context.CapturedAt,
			Correlation:   context.Correlation,
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
	digest := MemoryContentDigest([]byte(content))
	sourceID := string(section.Name)
	createdAt := context.CapturedAt
	if section.LastModified != nil {
		createdAt = *section.LastModified
	}
	record := Record{
		ID:      deterministicRecordID("laputa", sourceID, digest),
		Kind:    recordKindForSection(section.Name),
		Content: content,
		Provenance: Provenance{
			Source:        ProvenanceSourceLaputaAppliedSection,
			SourceID:      sourceID,
			ContentDigest: digest,
			CapturedAt:    context.CapturedAt,
			Correlation:   context.Correlation,
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
	normalizedBytes, err := recordsJSON(records)
	if err != nil {
		return nil, err
	}
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
		SourceDigest:          MemoryContentDigest(sourceBytes),
		NormalizedDigest:      MemoryContentDigest(normalizedBytes),
		SourceRecordCount:     sourceRecordCount,
		NormalizedRecordCount: uint64(len(records)),
		DuplicateRecordIDs:    duplicateIDs,
		BrokenSupersedes:      brokenIDs,
		ExpiredRecordCount:    expired,
		TombstoneRecordCount:  tombstoned,
		Findings:              findings,
	}, nil
}

// MemoryMigrationManifest is the versioned, isolated Memory migration
// manifest.
type MemoryMigrationManifest struct {
	SchemaVersion string        `json:"schema_version"`
	MigrationID   string        `json:"migration_id"`
	InputDigest   ContentDigest `json:"input_digest"`
	RecordsDigest ContentDigest `json:"records_digest"`
	RecordCount   uint64        `json:"record_count"`
	CreatedAt     time.Time     `json:"created_at"`
	Outputs       []string      `json:"outputs"`
}

// MarshalJSON emits CreatedAt in chrono serde form and a nil Outputs as an
// empty array.
func (m MemoryMigrationManifest) MarshalJSON() ([]byte, error) {
	type wire struct {
		SchemaVersion string        `json:"schema_version"`
		MigrationID   string        `json:"migration_id"`
		InputDigest   ContentDigest `json:"input_digest"`
		RecordsDigest ContentDigest `json:"records_digest"`
		RecordCount   uint64        `json:"record_count"`
		CreatedAt     string        `json:"created_at"`
		Outputs       []string      `json:"outputs"`
	}
	outputs := m.Outputs
	if outputs == nil {
		outputs = []string{}
	}
	return marshalCanonical(wire{
		SchemaVersion: m.SchemaVersion,
		MigrationID:   m.MigrationID,
		InputDigest:   m.InputDigest,
		RecordsDigest: m.RecordsDigest,
		RecordCount:   m.RecordCount,
		CreatedAt:     chronoSerdeTime(m.CreatedAt),
		Outputs:       outputs,
	})
}

// UnmarshalJSON requires every serde field to be present and normalizes
// CreatedAt to UTC.
func (m *MemoryMigrationManifest) UnmarshalJSON(data []byte) error {
	if err := requireJSONFields(data, "schema_version", "migration_id", "input_digest",
		"records_digest", "record_count", "created_at", "outputs"); err != nil {
		return err
	}
	type plain MemoryMigrationManifest
	var p plain
	if err := json.Unmarshal(data, &p); err != nil {
		return err
	}
	p.CreatedAt = p.CreatedAt.UTC()
	*m = MemoryMigrationManifest(p)
	return nil
}

// MemoryRollbackManifest is the content-free rollback manifest listing only
// files owned by one migration.
type MemoryRollbackManifest struct {
	SchemaVersion string   `json:"schema_version"`
	MigrationID   string   `json:"migration_id"`
	Outputs       []string `json:"outputs"`
}

// MarshalJSON emits a nil Outputs as an empty array.
func (m MemoryRollbackManifest) MarshalJSON() ([]byte, error) {
	type plain MemoryRollbackManifest
	p := plain(m)
	if p.Outputs == nil {
		p.Outputs = []string{}
	}
	return marshalCanonical(p)
}

// UnmarshalJSON requires every serde field to be present.
func (m *MemoryRollbackManifest) UnmarshalJSON(data []byte) error {
	if err := requireJSONFields(data, "schema_version", "migration_id", "outputs"); err != nil {
		return err
	}
	type plain MemoryRollbackManifest
	var p plain
	if err := json.Unmarshal(data, &p); err != nil {
		return err
	}
	*m = MemoryRollbackManifest(p)
	return nil
}

// MemoryMigrationPlan is the dry-run result that can be reviewed before any
// artifact is written.
type MemoryMigrationPlan struct {
	Directory string
	Manifest  MemoryMigrationManifest
	Rollback  MemoryRollbackManifest
	Records   []Record
}

// MemoryMigrationTestFailure is the test-only fault boundary used to prove
// artifact cleanup.
type MemoryMigrationTestFailure int

const (
	MemoryMigrationTestFailureAfterRecords MemoryMigrationTestFailure = iota + 1
	MemoryMigrationTestFailureAfterRollbackManifest
)

// MemoryMigration writes isolated Memory v2 migration artifacts
// (MemoryRecordMigration in memory_records.rs).
type MemoryMigration struct {
	artifactRoot string
}

// NewMemoryMigration creates a migration writer rooted at artifactRoot.
func NewMemoryMigration(artifactRoot string) *MemoryMigration {
	return &MemoryMigration{artifactRoot: artifactRoot}
}

// DryRun builds the migration plan without writing any artifact.
func (m *MemoryMigration) DryRun(migrationID string, inputBytes []byte, records []Record, createdAt time.Time) (*MemoryMigrationPlan, error) {
	if !validMigrationID(migrationID) {
		return nil, invalidMigrationIDErr(migrationID)
	}
	recordsBytes, err := recordsJSON(records)
	if err != nil {
		return nil, err
	}
	outputs := expectedOutputs()
	manifest := MemoryMigrationManifest{
		SchemaVersion: artifactSchemaVersion,
		MigrationID:   migrationID,
		InputDigest:   MemoryContentDigest(inputBytes),
		RecordsDigest: MemoryContentDigest(recordsBytes),
		RecordCount:   uint64(len(records)),
		CreatedAt:     createdAt,
		Outputs:       outputs,
	}
	return &MemoryMigrationPlan{
		Directory: filepath.Join(m.artifactRoot, migrationID),
		Manifest:  manifest,
		Rollback: MemoryRollbackManifest{
			SchemaVersion: artifactSchemaVersion,
			MigrationID:   migrationID,
			Outputs:       slices.Clone(outputs),
		},
		Records: records,
	}, nil
}

// Execute writes the planned artifacts, returning the existing manifest when
// this exact migration already ran.
func (m *MemoryMigration) Execute(plan *MemoryMigrationPlan) (*MemoryMigrationManifest, error) {
	return m.ExecuteWithFailure(plan, nil)
}

// ExecuteWithFailure is Execute with an optional test-only fault injection.
func (m *MemoryMigration) ExecuteWithFailure(plan *MemoryMigrationPlan, testFailure *MemoryMigrationTestFailure) (*MemoryMigrationManifest, error) {
	if err := m.validatePlan(plan); err != nil {
		return nil, err
	}
	manifestPath := filepath.Join(plan.Directory, "manifest.json")
	if _, err := os.Stat(manifestPath); err == nil {
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			return nil, migrationIOErr(manifestPath, err)
		}
		var existing MemoryMigrationManifest
		if err := json.Unmarshal(data, &existing); err != nil {
			return nil, migrationJSONErr(err)
		}
		if manifestsEqual(existing, plan.Manifest) {
			return &existing, nil
		}
		return nil, migrationConflictErr(plan.Manifest.MigrationID)
	}

	if err := os.MkdirAll(plan.Directory, 0o755); err != nil {
		return nil, migrationIOErr(plan.Directory, err)
	}
	manifest, err := func() (*MemoryMigrationManifest, error) {
		if err := atomicWriteJSON(filepath.Join(plan.Directory, "records.json"), recordsForJSON(plan.Records)); err != nil {
			return nil, err
		}
		if testFailure != nil && *testFailure == MemoryMigrationTestFailureAfterRecords {
			return nil, injectedMigrationFailureErr()
		}
		if err := atomicWriteJSON(filepath.Join(plan.Directory, "rollback.json"), &plan.Rollback); err != nil {
			return nil, err
		}
		if testFailure != nil && *testFailure == MemoryMigrationTestFailureAfterRollbackManifest {
			return nil, injectedMigrationFailureErr()
		}
		if err := atomicWriteJSON(manifestPath, &plan.Manifest); err != nil {
			return nil, err
		}
		return &plan.Manifest, nil
	}()
	if err != nil {
		removeOwnedOutputs(plan.Directory, plan.Rollback.Outputs)
		return nil, err
	}
	return manifest, nil
}

// Rollback deletes the files owned by one migration, verifying the rollback
// manifest before touching anything.
func (m *MemoryMigration) Rollback(migrationID string) error {
	if !validMigrationID(migrationID) {
		return invalidMigrationIDErr(migrationID)
	}
	directory := filepath.Join(m.artifactRoot, migrationID)
	rollbackPath := filepath.Join(directory, "rollback.json")
	data, err := os.ReadFile(rollbackPath)
	if err != nil {
		return migrationIOErr(rollbackPath, err)
	}
	var rollback MemoryRollbackManifest
	if err := json.Unmarshal(data, &rollback); err != nil {
		return migrationJSONErr(err)
	}
	if rollback.SchemaVersion != artifactSchemaVersion ||
		rollback.MigrationID != migrationID ||
		!slices.Equal(rollback.Outputs, expectedOutputs()) {
		return migrationConflictErr(migrationID)
	}
	for _, output := range rollback.Outputs {
		path := filepath.Join(directory, output)
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return migrationIOErr(path, err)
		}
	}
	if err := os.Remove(directory); err != nil && !errors.Is(err, os.ErrNotExist) {
		return migrationIOErr(directory, err)
	}
	return nil
}

func (m *MemoryMigration) validatePlan(plan *MemoryMigrationPlan) error {
	recordsBytes, err := recordsJSON(plan.Records)
	if err != nil {
		return err
	}
	if !validMigrationID(plan.Manifest.MigrationID) ||
		plan.Directory != filepath.Join(m.artifactRoot, plan.Manifest.MigrationID) ||
		plan.Manifest.SchemaVersion != artifactSchemaVersion ||
		!slices.Equal(plan.Manifest.Outputs, expectedOutputs()) ||
		plan.Manifest.RecordCount != uint64(len(plan.Records)) ||
		plan.Manifest.RecordsDigest != MemoryContentDigest(recordsBytes) ||
		plan.Rollback.SchemaVersion != artifactSchemaVersion ||
		plan.Rollback.MigrationID != plan.Manifest.MigrationID ||
		!slices.Equal(plan.Rollback.Outputs, expectedOutputs()) {
		return migrationConflictErr(plan.Manifest.MigrationID)
	}
	return nil
}

// deterministicRecordID binds a record id to its source and content digest
// (deterministic_record_id in memory_records.rs).
func deterministicRecordID(prefix, sourceID string, digest ContentDigest) string {
	binding := prefix + "\x00" + sourceID + "\x00" + digest.Value
	return "memory-" + prefix + "-" + MemoryContentDigest([]byte(binding)).Value
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

// removeOwnedOutputs deletes only files listed in the migration's own
// outputs plus the (then empty) artifact directory.
func removeOwnedOutputs(directory string, outputs []string) {
	for _, output := range outputs {
		_ = os.Remove(filepath.Join(directory, output))
	}
	_ = os.Remove(directory)
}

func expectedOutputs() []string {
	return slices.Clone(artifactOutputs)
}

func validMigrationID(migrationID string) bool {
	return strings.TrimSpace(migrationID) != "" &&
		!strings.Contains(migrationID, "/") &&
		!strings.Contains(migrationID, "\\") &&
		migrationID != "." &&
		migrationID != ".."
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

// manifestsEqual compares manifests by their canonical serialized form.
func manifestsEqual(a, b MemoryMigrationManifest) bool {
	aBytes, err := marshalCanonical(a)
	if err != nil {
		return false
	}
	bBytes, err := marshalCanonical(b)
	if err != nil {
		return false
	}
	return bytes.Equal(aBytes, bBytes)
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
