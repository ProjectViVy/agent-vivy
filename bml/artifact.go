// Artifact admission owns exact source input, opaque allocations and its generated outputs.
package bml

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

const artifactSchemaVersion = "2.0.0"

var artifactOutputs = []string{"records.json", "rollback.json", "manifest.json"}

// MemoryMigrationManifest is the versioned, isolated Memory migration
// manifest.
type MemoryMigrationManifest struct {
	SchemaVersion   string             `json:"schema_version"`
	MigrationID     string             `json:"migration_id"`
	Input           []byte             `json:"input"`
	AdmittedRecords []Record           `json:"admitted_records"`
	State           string             `json:"state"`
	Allocations     []RecordAllocation `json:"allocations"`
	RecordCount     uint64             `json:"record_count"`
	CreatedAt       time.Time          `json:"created_at"`
	Outputs         []string           `json:"outputs"`
}

// MarshalJSON emits CreatedAt in chrono serde form and a nil Outputs as an
// empty array.
func (m MemoryMigrationManifest) MarshalJSON() ([]byte, error) {
	type wire struct {
		SchemaVersion   string             `json:"schema_version"`
		MigrationID     string             `json:"migration_id"`
		Input           []byte             `json:"input"`
		AdmittedRecords []Record           `json:"admitted_records"`
		State           string             `json:"state"`
		Allocations     []RecordAllocation `json:"allocations"`
		RecordCount     uint64             `json:"record_count"`
		CreatedAt       string             `json:"created_at"`
		Outputs         []string           `json:"outputs"`
	}
	outputs := m.Outputs
	if outputs == nil {
		outputs = []string{}
	}
	return marshalCanonical(wire{
		SchemaVersion:   m.SchemaVersion,
		MigrationID:     m.MigrationID,
		Input:           m.Input,
		AdmittedRecords: m.AdmittedRecords,
		State:           m.State,
		Allocations:     m.Allocations,
		RecordCount:     m.RecordCount,
		CreatedAt:       chronoSerdeTime(m.CreatedAt),
		Outputs:         outputs,
	})
}

// UnmarshalJSON requires every serde field to be present and normalizes
// CreatedAt to UTC.
func (m *MemoryMigrationManifest) UnmarshalJSON(data []byte) error {
	var version struct {
		SchemaVersion string `json:"schema_version"`
	}
	if err := json.Unmarshal(data, &version); err != nil {
		return err
	}
	if version.SchemaVersion == "1.0.0" {
		return formatError(ErrUpgradeRequired)
	}
	if version.SchemaVersion != artifactSchemaVersion {
		return formatError(ErrUnsupportedSourceFormat)
	}
	if err := requireJSONFields(data, "schema_version", "migration_id", "input",
		"admitted_records", "allocations", "state", "record_count", "created_at", "outputs"); err != nil {
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
// RecordAllocation binds an opaque record ID to its original source identity.
type RecordAllocation struct {
	Source   ProvenanceSource `json:"source"`
	SourceID string           `json:"source_id"`
	RecordID string           `json:"record_id"`
}

func (m *MemoryMigration) DryRun(migrationID string, inputBytes []byte, records []Record, createdAt time.Time) (*MemoryMigrationPlan, error) {
	if !validMigrationID(migrationID) {
		return nil, invalidMigrationIDErr(migrationID)
	}
	encoded, err := recordsJSON(records)
	if err != nil {
		return nil, err
	}
	var admitted []Record
	if err := json.Unmarshal(encoded, &admitted); err != nil {
		return nil, migrationJSONErr(err)
	}
	directory := filepath.Join(m.artifactRoot, migrationID)
	existing, err := readMigrationManifest(directory)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if existing != nil {
		if (existing.State != "allocated" && existing.State != "complete") || !bytes.Equal(inputBytes, existing.Input) {
			return nil, migrationConflictErr(migrationID)
		}
		if err := reuseAllocations(admitted, existing.Allocations); err != nil {
			return nil, migrationConflictErr(migrationID)
		}
		old, err := recordsJSON(existing.AdmittedRecords)
		if err != nil {
			return nil, err
		}
		normalized, err := recordsJSON(admitted)
		if err != nil {
			return nil, err
		}
		if !bytes.Equal(old, normalized) {
			return nil, migrationConflictErr(migrationID)
		}
		createdAt = existing.CreatedAt
	}
	allocations := make([]RecordAllocation, 0, len(admitted))
	seen := map[string]bool{}
	for _, r := range admitted {
		key := string(r.Provenance.Source) + "\x00" + r.Provenance.SourceID
		if seen[key] {
			return nil, migrationConflictErr(migrationID)
		}
		seen[key] = true
		allocations = append(allocations, RecordAllocation{r.Provenance.Source, r.Provenance.SourceID, r.ID})
	}
	manifest := MemoryMigrationManifest{SchemaVersion: artifactSchemaVersion, MigrationID: migrationID, Input: append([]byte{}, inputBytes...), AdmittedRecords: admitted, Allocations: allocations, State: "complete", RecordCount: uint64(len(admitted)), CreatedAt: createdAt, Outputs: expectedOutputs()}
	return &MemoryMigrationPlan{Directory: directory, Manifest: manifest, Rollback: MemoryRollbackManifest{SchemaVersion: artifactSchemaVersion, MigrationID: migrationID, Outputs: expectedOutputs()}, Records: admitted}, nil
}

func readMigrationManifest(directory string) (*MemoryMigrationManifest, error) {
	data, err := os.ReadFile(filepath.Join(directory, "manifest.json"))
	if err != nil {
		return nil, err
	}
	var manifest MemoryMigrationManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}
	return &manifest, nil
}

func reuseAllocations(records []Record, allocations []RecordAllocation) error {
	if len(records) != len(allocations) {
		return migrationConflictErr("")
	}
	bySource := map[string]string{}
	for _, a := range allocations {
		bySource[string(a.Source)+"\x00"+a.SourceID] = a.RecordID
	}
	ids := map[string]string{}
	for i := range records {
		r := &records[i]
		id, ok := bySource[string(r.Provenance.Source)+"\x00"+r.Provenance.SourceID]
		if !ok {
			return migrationConflictErr("")
		}
		ids[r.ID] = id
		r.ID = id
	}
	for i := range records {
		r := &records[i]
		for j, id := range r.Supersedes {
			if mapped, ok := ids[id]; ok {
				r.Supersedes[j] = mapped
			}
		}
		if r.Tombstone != nil {
			if mapped, ok := ids[r.Tombstone.TargetRecordID]; ok {
				r.Tombstone.TargetRecordID = mapped
			}
		}
	}
	return nil
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
	if err := os.MkdirAll(m.artifactRoot, 0o755); err != nil {
		return nil, migrationIOErr(m.artifactRoot, err)
	}
	fresh := false
	if err := os.Mkdir(plan.Directory, 0o700); err == nil {
		fresh = true
	} else if !errors.Is(err, os.ErrExist) {
		return nil, migrationIOErr(plan.Directory, err)
	}
	if info, err := os.Lstat(plan.Directory); err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, migrationConflictErr(plan.Manifest.MigrationID)
	}
	manifestPath := filepath.Join(plan.Directory, "manifest.json")
	if !fresh {
		if _, err := os.Stat(manifestPath); err != nil {
			return nil, migrationConflictErr(plan.Manifest.MigrationID)
		}
	}
	if !fresh {
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			return nil, migrationIOErr(manifestPath, err)
		}
		var existing MemoryMigrationManifest
		if err := json.Unmarshal(data, &existing); err != nil {
			return nil, migrationJSONErr(err)
		}
		expected := existing
		expected.State = "complete"
		if manifestsEqual(expected, plan.Manifest) {
			if existing.State == "complete" {
				if err := verifyMigrationOutputs(plan); err != nil {
					return nil, err
				}
				return &existing, nil
			}
			if existing.State != "allocated" {
				return nil, migrationConflictErr(plan.Manifest.MigrationID)
			}
		} else {
			return nil, migrationConflictErr(plan.Manifest.MigrationID)
		}
	}

	allocated := plan.Manifest
	allocated.State = "allocated"
	if err := atomicWriteJSON(manifestPath, &allocated); err != nil {
		return nil, err
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
		// Keep the admitted inputs/allocation map for an exact restart.
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
	if info, err := os.Lstat(directory); err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return migrationConflictErr(migrationID)
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return migrationIOErr(directory, err)
	}
	// An interrupted rollback can leave only an empty directory after deleting
	// its last receipt. Remove only that empty directory, never recursively.
	if len(entries) == 0 {
		if err := os.Remove(directory); err != nil {
			return migrationIOErr(directory, err)
		}
		return nil
	}
	for _, entry := range entries {
		if !slices.Contains(expectedOutputs(), entry.Name()) || !entry.Type().IsRegular() {
			return migrationConflictErr(migrationID)
		}
	}
	manifest, err := readMigrationManifest(directory)
	if err != nil {
		return err
	}
	if manifest.MigrationID != migrationID || !slices.Equal(manifest.Outputs, expectedOutputs()) || (manifest.State != "allocated" && manifest.State != "complete" && manifest.State != "unverifiable" && manifest.State != "rolling_back") {
		return migrationConflictErr(migrationID)
	}
	rollbackPath := filepath.Join(directory, "rollback.json")
	raw, err := os.ReadFile(rollbackPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return migrationIOErr(rollbackPath, err)
	}
	if err == nil {
		var rollback MemoryRollbackManifest
		if err := json.Unmarshal(raw, &rollback); err != nil {
			return migrationJSONErr(err)
		}
		if rollback.SchemaVersion != artifactSchemaVersion || rollback.MigrationID != migrationID || !slices.Equal(rollback.Outputs, expectedOutputs()) {
			return migrationConflictErr(migrationID)
		}
	}
	manifest.State = "rolling_back"
	if err := atomicWriteJSON(filepath.Join(directory, "manifest.json"), manifest); err != nil {
		return err
	}
	// Keep durable authority until every other owned file has been removed.
	for _, name := range []string{"records.json", "rollback.json", "manifest.json"} {
		path := filepath.Join(directory, name)
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return migrationIOErr(path, err)
		}
	}
	if err := os.Remove(directory); err != nil {
		return migrationIOErr(directory, err)
	}
	return nil
}

func (m *MemoryMigration) validatePlan(plan *MemoryMigrationPlan) error {
	if plan == nil {
		return migrationConflictErr("")
	}
	recordsBytes, err := recordsJSON(plan.Records)
	if err != nil {
		return err
	}
	admittedBytes, err := recordsJSON(plan.Manifest.AdmittedRecords)
	if err != nil {
		return err
	}
	if plan.Manifest.State != "complete" || !validMigrationID(plan.Manifest.MigrationID) ||
		plan.Directory != filepath.Join(m.artifactRoot, plan.Manifest.MigrationID) ||
		plan.Manifest.SchemaVersion != artifactSchemaVersion ||
		!slices.Equal(plan.Manifest.Outputs, expectedOutputs()) ||
		plan.Manifest.RecordCount != uint64(len(plan.Records)) ||
		!bytes.Equal(recordsBytes, admittedBytes) ||
		plan.Rollback.SchemaVersion != artifactSchemaVersion ||
		plan.Rollback.MigrationID != plan.Manifest.MigrationID ||
		!slices.Equal(plan.Rollback.Outputs, expectedOutputs()) {
		return migrationConflictErr(plan.Manifest.MigrationID)
	}
	if len(plan.Manifest.Allocations) != len(plan.Records) {
		return migrationConflictErr(plan.Manifest.MigrationID)
	}
	seenIDs, seenSources := map[string]bool{}, map[string]bool{}
	for i, r := range plan.Records {
		a := plan.Manifest.Allocations[i]
		key := string(a.Source) + "\x00" + a.SourceID
		if a.RecordID != r.ID || a.Source != r.Provenance.Source || a.SourceID != r.Provenance.SourceID || strings.TrimSpace(r.ID) == "" || seenIDs[r.ID] || seenSources[key] {
			return migrationConflictErr(plan.Manifest.MigrationID)
		}
		seenIDs[r.ID], seenSources[key] = true, true
	}
	return nil
}

func verifyMigrationOutputs(plan *MemoryMigrationPlan) error {
	for _, name := range expectedOutputs() {
		if info, err := os.Lstat(filepath.Join(plan.Directory, name)); err != nil || !info.Mode().IsRegular() {
			return migrationConflictErr(plan.Manifest.MigrationID)
		}
	}
	rollbackBytes, err := os.ReadFile(filepath.Join(plan.Directory, "rollback.json"))
	if err != nil {
		return migrationIOErr(plan.Directory, err)
	}
	var rollback MemoryRollbackManifest
	if err := json.Unmarshal(rollbackBytes, &rollback); err != nil {
		return migrationJSONErr(err)
	}
	if rollback.SchemaVersion != artifactSchemaVersion || rollback.MigrationID != plan.Manifest.MigrationID || !slices.Equal(rollback.Outputs, expectedOutputs()) {
		return migrationConflictErr(plan.Manifest.MigrationID)
	}
	raw, err := os.ReadFile(filepath.Join(plan.Directory, "records.json"))
	if err != nil {
		return migrationIOErr(plan.Directory, err)
	}
	expected, err := recordsJSON(plan.Manifest.AdmittedRecords)
	if err != nil {
		return err
	}
	if !bytes.Equal(bytes.TrimSpace(raw), bytes.TrimSpace(expected)) {
		// atomicWriteJSON formats JSON; compare decoded canonical values.
		var records []Record
		if err := json.Unmarshal(raw, &records); err != nil {
			return migrationJSONErr(err)
		}
		actual, err := recordsJSON(records)
		if err != nil {
			return err
		}
		if !bytes.Equal(actual, expected) {
			return migrationConflictErr(plan.Manifest.MigrationID)
		}
	}
	return nil
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
