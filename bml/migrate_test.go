package bml

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func migrationContext() *MemoryAdapterContext {
	session := "session-1"
	return &MemoryAdapterContext{
		TenantID:    "tenant-1",
		WorkspaceID: "workspace-1",
		SessionID:   &session,
		Correlation: AuditCorrelation{
			RequestID: "request-1",
			TurnID:    "turn-1",
			SessionID: "session-1",
			TraceID:   nil,
		},
		CapturedAt: ts(0),
	}
}

func migrationErrCode(t *testing.T, err error, code MigrationErrorCode) {
	t.Helper()
	var me *MigrationError
	if !errors.As(err, &me) {
		t.Fatalf("expected MigrationError %q, got %v", code, err)
	}
	if me.Code != code {
		t.Fatalf("expected code %q, got %q", code, me.Code)
	}
}

func TestLegacyAdapterDeterministicAuthorityExplicit(t *testing.T) {
	first := AdaptLegacyMarkdown("MEMORY.md", "hello", KindLongTerm, migrationContext(), true)
	second := AdaptLegacyMarkdown("MEMORY.md", "hello", KindLongTerm, migrationContext(), true)
	if !reflect.DeepEqual(first, second) {
		t.Fatal("adapt_legacy_markdown must be deterministic")
	}
	if first.Records[0].Trust != TrustAppliedAuthority {
		t.Fatalf("expected applied authority, got %q", first.Records[0].Trust)
	}
	if first.Records[0].Provenance.Source != ProvenanceSourceLegacyMarkdownOwner {
		t.Fatalf("expected legacy_markdown_owner, got %q", first.Records[0].Provenance.Source)
	}
}

func TestNonOwnedLaputaSectionsNotNormalized(t *testing.T) {
	modified := ts(0)
	section := &LaputaSection{
		Name:         LaputaSectionChangelog,
		Status:       SectionStatusTbd,
		Content:      json.RawMessage(`{"pending":"must not enter prompt"}`),
		Metadata:     json.RawMessage(`{}`),
		LastModified: &modified,
		Version:      "1.0.0",
	}
	raw := map[string]any{
		"name":          "changelog",
		"status":        "tbd",
		"content":       map[string]any{},
		"metadata":      map[string]any{},
		"last_modified": nil,
		"version":       "1.0.0",
		"future_field":  true,
	}
	output, err := AdaptLaputaSection(section, raw, migrationContext())
	if err != nil {
		t.Fatalf("adapt_laputa_section: %v", err)
	}
	if len(output.Records) != 0 {
		t.Fatalf("non-owned section must not produce records, got %d", len(output.Records))
	}
	if len(output.Findings) != 2 {
		t.Fatalf("expected 2 findings (unknown field + not owned), got %d", len(output.Findings))
	}
}

func TestMigrationIdempotentConflictSafeReversible(t *testing.T) {
	temp := t.TempDir()
	original := filepath.Join(temp, "MEMORY.md")
	if err := os.WriteFile(original, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	adapter := AdaptLegacyMarkdown(original, "original", KindLongTerm, migrationContext(), true)
	migration := NewMemoryMigration(filepath.Join(temp, "artifacts"))
	plan, err := migration.DryRun("gmh-21", []byte("original"), adapter.Records, ts(0))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(plan.Directory); !os.IsNotExist(err) {
		t.Fatal("dry_run must not create the artifact directory")
	}
	first, err := migration.Execute(plan)
	if err != nil {
		t.Fatal(err)
	}
	again, err := migration.Execute(plan)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(*again, *first) {
		t.Fatal("execute must be idempotent")
	}
	restarted := NewMemoryMigration(filepath.Join(temp, "artifacts"))
	restartedManifest, err := restarted.Execute(plan)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(*restartedManifest, *first) {
		t.Fatal("restarted migration must return the existing manifest")
	}
	if data, err := os.ReadFile(original); err != nil || string(data) != "original" {
		t.Fatal("source file must not be modified")
	}

	conflicting, err := migration.DryRun("gmh-21", []byte("changed"), nil, ts(0))
	if err != nil {
		t.Fatal(err)
	}
	_, err = migration.Execute(conflicting)
	migrationErrCode(t, err, ErrMigrationConflict)

	if err := migration.Rollback("gmh-21"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(plan.Directory); !os.IsNotExist(err) {
		t.Fatal("rollback must remove the artifact directory")
	}
	if data, err := os.ReadFile(original); err != nil || string(data) != "original" {
		t.Fatal("source file must not be modified")
	}
}

func TestFailedMigrationCleansOnlyOwnedArtifacts(t *testing.T) {
	temp := t.TempDir()
	artifacts := filepath.Join(temp, "artifacts")
	if err := os.MkdirAll(artifacts, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(artifacts, "unrelated"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	migration := NewMemoryMigration(artifacts)
	plan, err := migration.DryRun("failed", []byte("input"), nil, ts(0))
	if err != nil {
		t.Fatal(err)
	}
	failure := MemoryMigrationTestFailureAfterRollbackManifest
	_, err = migration.ExecuteWithFailure(plan, &failure)
	migrationErrCode(t, err, ErrInjectedMigrationFailure)
	if _, err := os.Stat(plan.Directory); !os.IsNotExist(err) {
		t.Fatal("failed migration must remove its artifact directory")
	}
	if data, err := os.ReadFile(filepath.Join(artifacts, "unrelated")); err != nil || string(data) != "keep" {
		t.Fatal("failed migration must not touch unrelated files")
	}
}

func TestForgedArtifactPathsAndRollbackManifestsFailClosed(t *testing.T) {
	temp := t.TempDir()
	migration := NewMemoryMigration(filepath.Join(temp, "artifacts"))
	plan, err := migration.DryRun("safe", []byte("input"), nil, ts(0))
	if err != nil {
		t.Fatal(err)
	}
	plan.Directory = filepath.Join(temp, "outside")
	_, err = migration.Execute(plan)
	migrationErrCode(t, err, ErrMigrationConflict)

	valid, err := migration.DryRun("rollback", []byte("input"), nil, ts(0))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := migration.Execute(valid); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(temp, "outside.txt")
	if err := os.WriteFile(outside, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	forged := &MemoryRollbackManifest{
		SchemaVersion: "1.0.0",
		MigrationID:   "rollback",
		Outputs:       []string{"../outside.txt"},
	}
	if err := atomicWriteJSON(filepath.Join(valid.Directory, "rollback.json"), forged); err != nil {
		t.Fatal(err)
	}
	err = migration.Rollback("rollback")
	migrationErrCode(t, err, ErrMigrationConflict)
	if data, err := os.ReadFile(outside); err != nil || string(data) != "keep" {
		t.Fatal("forged rollback manifest must not delete files outside the artifact directory")
	}
}

func TestIntegrityReportDetectsDuplicatesAndBrokenLinks(t *testing.T) {
	record := AdaptLegacyMarkdown("HISTORY.md", "history", KindHistory, migrationContext(), true).Records[0]
	record.Supersedes = append(record.Supersedes, "missing")
	report, err := CompareNormalizedRecords([]byte("history"), []Record{record, record}, nil, ts(0))
	if err != nil {
		t.Fatal(err)
	}
	if len(report.DuplicateRecordIDs) != 1 {
		t.Fatalf("expected 1 duplicate id, got %v", report.DuplicateRecordIDs)
	}
	if !reflect.DeepEqual(report.BrokenSupersedes, []string{"missing"}) {
		t.Fatalf("expected broken supersedes [missing], got %v", report.BrokenSupersedes)
	}
	if len(report.Findings) != 2 {
		t.Fatalf("expected 2 findings, got %d", len(report.Findings))
	}
}

func TestMigrationManifestJSONProtocolStable(t *testing.T) {
	migration := NewMemoryMigration("artifacts")
	plan, err := migration.DryRun("gmh-21", []byte("input"), nil, ts(0))
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(plan.Manifest)
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := json.Unmarshal(encoded, &value); err != nil {
		t.Fatal(err)
	}
	if value["schema_version"] != "1.0.0" {
		t.Fatalf("schema_version: %v", value["schema_version"])
	}
	if value["migration_id"] != "gmh-21" {
		t.Fatalf("migration_id: %v", value["migration_id"])
	}
	digest, ok := value["input_digest"].(map[string]any)
	if !ok || digest["algorithm"] != "sha256" {
		t.Fatalf("input_digest: %v", value["input_digest"])
	}
	if !reflect.DeepEqual(value["outputs"], []any{"records.json", "rollback.json", "manifest.json"}) {
		t.Fatalf("outputs: %v", value["outputs"])
	}
	var decoded MemoryMigrationManifest
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, plan.Manifest) {
		t.Fatal("manifest JSON round-trip must preserve all fields")
	}
}
