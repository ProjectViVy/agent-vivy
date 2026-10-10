package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	genassembly "agent-vivy/internal/generated/assembly"
)

// This recorder exports observed synthetic-profile artifacts, not a formal
// pass/candidate. Frozen baseline identity and case acceptance belong to the
// external evidence checker, which must not infer them from a test counter.
func saveMemoryLoopDevelopmentEvidence(t *testing.T, f *memoryLoopFixture, session, phase string, snapshots ...memoryLoopSnapshot) string {
	t.Helper()
	root := os.Getenv("VIVY_MEMORY_LOOP_EVIDENCE_ROOT")
	if root == "" {
		return ""
	}
	if !filepath.IsAbs(root) {
		t.Fatal("memory-loop evidence root must be absolute")
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	name := strings.NewReplacer("/", "_", "\\", "_").Replace(t.Name() + "-" + phase + "-")
	dir, err := os.MkdirTemp(root, name)
	if err != nil {
		t.Fatal(err)
	}
	type artifact struct {
		Path   string `json:"path"`
		SHA256 string `json:"sha256"`
		Bytes  int    `json:"bytes"`
	}
	artifacts := []artifact{}
	write := func(name string, raw []byte) {
		if err := os.WriteFile(filepath.Join(dir, name), raw, 0600); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(raw)
		artifacts = append(artifacts, artifact{name, hex.EncodeToString(sum[:]), len(raw)})
	}
	writeJSON := func(name string, value any) {
		raw, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		write(name, append(raw, '\n'))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var status json.RawMessage
	if err := f.cognitiveValue(ctx, "diva.cognitive.status", session, nil, &status); err != nil {
		t.Fatal(err)
	}
	writeJSON("status.json", status)
	requests := f.ModelRequests()
	responses := f.ModelResponses()
	writeJSON("model-requests.json", requests)
	writeJSON("model-responses.json", responses)
	writeJSON("model-counts.json", map[string]any{"requests": len(requests), "completed_response_handlers": len(responses), "requests_without_completed_response": len(requests) - len(responses), "client_consumption_asserted": false})
	writeJSON("native-snapshots.json", snapshots)
	for i, snapshot := range snapshots {
		write(fmt.Sprintf("source-%d.txt", i+1), []byte(snapshot.SourceBody))
	}
	var generation any
	pid := os.Getpid()
	if f.remote != nil {
		pid = f.remote.pid
	} else if f.app != nil {
		generation = runtimeGenerationID(*f.app.assembly)
	}
	writeJSON("identity.json", map[string]any{"process_id": pid, "generation_id": generation, "session_id": session})
	manifest := map[string]any{"kind": "developer-observation", "test": t.Name(), "phase": phase, "recorded_at": time.Now().UTC().Format(time.RFC3339Nano), "formal_acceptance": false, "candidate_id": nil, "artifacts": artifacts, "limitation": "final source/artifact identity must be bound externally; no pass status inferred"}
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), append(raw, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	t.Logf("actual development artifacts: %s (%d hashed files)", dir, len(artifacts))
	return dir
}

func TestMemoryLoopEvidenceExportsActualLargeUserSource(t *testing.T) {
	probe := genassembly.BuildDefault()
	if !probe.HasCognitiveFactory() {
		t.Skip("DIVA integration overlay required")
	}
	if os.Getenv("VIVY_MEMORY_LOOP_EVIDENCE_ROOT") == "" {
		t.Setenv("VIVY_MEMORY_LOOP_EVIDENCE_ROOT", t.TempDir())
	}
	f := newMemoryLoopFixture(t, memoryLoopOptions{ConfigPath: memoryLoopConfig(t), ModelMode: "ack"})
	session := memoryLoopSession(t, f)
	fact := strings.Repeat("合成用户来源 ", 300) + memoryLoopRandomFact(t)
	run := memoryLoopTurn(t, f, session, fact)
	snapshot, err := f.Wait(context.Background(), "canonical", run)
	if err != nil {
		t.Fatal(err)
	}
	dir := saveMemoryLoopDevelopmentEvidence(t, f, session, "captured", snapshot)
	source, err := os.ReadFile(filepath.Join(dir, "source-1.txt"))
	if err != nil || len(source) <= 4096 || !strings.Contains(string(source), fact) {
		t.Fatalf("actual large admitted user source was truncated: bytes=%d %v", len(source), err)
	}
	requests, err := os.ReadFile(filepath.Join(dir, "model-requests.json"))
	if err != nil || !strings.Contains(string(requests), fact) {
		t.Fatalf("actual recorded model request lost original source: %v", err)
	}
}
