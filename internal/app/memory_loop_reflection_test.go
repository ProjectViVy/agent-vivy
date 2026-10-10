package app

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	genassembly "agent-vivy/internal/generated/assembly"
)

func memoryLoopAction(t *testing.T, f *memoryLoopFixture, action string, input map[string]any, dst any) {
	t.Helper()
	args, err := json.Marshal(map[string]any{"module_id": "vivy/diva-cognitive", "action_id": action, "input": input})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	raw, err := f.Call(ctx, "module.action.invoke", args)
	if err != nil {
		t.Fatalf("%s transport: %v", action, err)
	}
	var out struct {
		Status string          `json:"status"`
		Value  json.RawMessage `json:"value"`
		Error  json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || out.Status != "ok" {
		t.Fatalf("%s outcome: %s (%v)", action, raw, err)
	}
	if dst != nil {
		if err := json.Unmarshal(out.Value, dst); err != nil {
			t.Fatalf("%s value: %s (%v)", action, out.Value, err)
		}
	}
}

func memoryLoopEnable(t *testing.T, f *memoryLoopFixture, sessionID string) {
	t.Helper()
	var policy struct {
		Revision uint64 `json:"policy_revision"`
	}
	memoryLoopAction(t, f, "diva.cognitive.policy.get", map[string]any{"session_id": sessionID}, &policy)
	memoryLoopAction(t, f, "diva.cognitive.policy.set", map[string]any{"session_id": sessionID, "enabled": true, "min_interval_ms": 0, "base_revision": policy.Revision}, nil)
}

func memoryLoopTurn(t *testing.T, f *memoryLoopFixture, sessionID, text string) string {
	t.Helper()
	params, err := json.Marshal(map[string]any{"session_id": sessionID, "text": text})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := f.Call(context.Background(), "turn/start", params)
	if err != nil {
		t.Fatal(err)
	}
	var run struct {
		ID string `json:"run_id"`
	}
	if err := json.Unmarshal(raw, &run); err != nil || run.ID == "" {
		t.Fatalf("turn: %s %v", raw, err)
	}
	return run.ID
}

func TestMemoryLoopAutomaticReflection(t *testing.T) {
	memoryLoopAutomaticReflection(t, "")
}

func TestMemoryLoopAutomaticReflectionLargeSource(t *testing.T) {
	memoryLoopAutomaticReflection(t, strings.Repeat("长", 1500))
}

func memoryLoopAutomaticReflection(t *testing.T, extraSource string) {
	t.Helper()
	probe := genassembly.BuildDefault()
	if !probe.HasCognitiveFactory() {
		t.Skip("DIVA integration overlay required")
	}
	f := newMemoryLoopFixture(t, memoryLoopOptions{ConfigPath: memoryLoopConfig(t), ModelMode: "reflection"})
	t.Cleanup(func() {
		if !t.Failed() {
			return
		}
		db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(filepath.Join(f.dataRoot, "vivy-test.db"))+"?mode=ro")
		if err != nil {
			return
		}
		defer db.Close()
		rows, err := db.Query(`SELECT type,payload FROM run_events WHERE type LIKE '%failed%' LIMIT 12`)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var kind string
				var payload []byte
				if rows.Scan(&kind, &payload) == nil {
					t.Logf("actual failure %s: %s", kind, payload)
				}
			}
		}
		for _, req := range f.ModelRequests() {
			if strings.Contains(string(req), "[cognitive-infer") {
				t.Logf("actual inference request: %s", req)
				break
			}
		}
	})
	sessionID := createCognitiveSession(t, f.peer)
	memoryLoopEnable(t, f, sessionID)
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		t.Fatal(err)
	}
	fact := "memory-fact=" + hex.EncodeToString(nonce) + extraSource
	runID := memoryLoopTurn(t, f, sessionID, fact)
	canonical, err := f.Wait(context.Background(), "canonical", runID)
	if err != nil {
		t.Fatal(err)
	}
	reflected, err := f.Wait(context.Background(), "reflected", runID)
	if err != nil {
		t.Fatal(err)
	}
	if reflected.OperationID == "" || reflected.RecordID == "" || reflected.Revision == 0 || reflected.ProcessedThrough < canonical.CaptureSeq || !strings.Contains(reflected.CanonicalBody, fact) {
		t.Fatalf("reflection not durably applied from actual source: %+v", reflected)
	}
	if reflected.CanonicalCount != 2 {
		t.Fatalf("canonical count=%d; want raw source plus one memory effect", reflected.CanonicalCount)
	}
	stages := map[string]bool{}
	for _, req := range f.ModelRequests() {
		for _, stage := range []string{"reconcile", "reflect"} {
			if strings.Contains(string(req), "[cognitive-infer stage="+stage+"]") && strings.Contains(string(req), fact) {
				stages[stage] = true
			}
		}
	}
	if !stages["reconcile"] || !stages["reflect"] {
		t.Fatalf("actual inference requests missing source: %v", stages)
	}
}
