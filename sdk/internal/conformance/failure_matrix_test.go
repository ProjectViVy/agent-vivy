package conformance

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

type failureMatrix struct {
	SchemaVersion string              `json:"schemaVersion"`
	Cases         []failureMatrixCase `json:"cases"`
}

type failureMatrixCase struct {
	ID         string `json:"id"`
	Layer      string `json:"layer"`
	EvidenceID string `json:"evidenceId"`
	Diagnostic string `json:"diagnostic"`
}

func TestGenerationFailureMatrixEvidence(t *testing.T) {
	raw, err := os.ReadFile("testdata/failure-matrix.json")
	if err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var matrix failureMatrix
	if err := decoder.Decode(&matrix); err != nil {
		t.Fatal(err)
	}
	if matrix.SchemaVersion != "vivy.conformance/failure-matrix-v1" {
		t.Fatalf("schemaVersion = %q", matrix.SchemaVersion)
	}
	wantCases := []string{
		"bad-source-hash", "catalog-byte-limit", "catalog-duplicate-key", "catalog-message-limit",
		"catalog-missing-english", "catalog-placeholder-drift", "catalog-placeholder-limit",
		"catalog-symlink-escape", "catalog-unit-limit", "catalog-unknown-field", "catalog-wrong-owner",
		"dependency-cycle", "duplicate-exclusive-provider", "grant-denied", "legacy-v0-input",
		"middleware-timeout", "missing-provider", "module-conflict", "no-formal-artifact",
		"protected-tool-override", "startup-rollback", "ui-root-conflict",
		"unsupported-eino-capability", "unused-provider", "valid-deterministic-rebuild",
	}
	gotCases := make([]string, 0, len(matrix.Cases))
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	for _, testCase := range matrix.Cases {
		gotCases = append(gotCases, testCase.ID)
		path, anchor, ok := strings.Cut(testCase.EvidenceID, "#")
		if !ok || path == "" || anchor == "" || filepath.IsAbs(path) || !filepath.IsLocal(path) {
			t.Fatalf("%s has unsafe evidence ID %q", testCase.ID, testCase.EvidenceID)
		}
		body, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if readErr != nil {
			t.Fatalf("%s evidence: %v", testCase.ID, readErr)
		}
		if !bytes.Contains(body, []byte(anchor)) {
			t.Fatalf("%s evidence anchor %s is absent from %s", testCase.ID, anchor, path)
		}
		if strings.TrimSpace(testCase.Layer) == "" || strings.TrimSpace(testCase.Diagnostic) == "" {
			t.Fatalf("%s omits its layer or stable diagnostic", testCase.ID)
		}
	}
	if !sort.StringsAreSorted(gotCases) {
		t.Fatalf("failure matrix is not canonical: %v", gotCases)
	}
	if !equalStrings(gotCases, wantCases) {
		t.Fatalf("failure matrix cases = %v, want %v", gotCases, wantCases)
	}
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
