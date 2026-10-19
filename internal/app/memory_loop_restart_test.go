package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"

	genassembly "agent-vivy/internal/generated/assembly"
)

func TestMemoryLoopFixtureRestart(t *testing.T) {
	probe := genassembly.BuildDefault()
	if !probe.HasCognitiveFactory() {
		t.Skip("DIVA integration overlay required")
	}
	f := newMemoryLoopFixture(t, memoryLoopOptions{ConfigPath: memoryLoopConfig(t), ModelMode: "ack"})
	session := createCognitiveSession(t, f.peer)
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		t.Fatal(err)
	}
	fact := "restart-fact=" + hex.EncodeToString(nonce)
	run := memoryLoopTurn(t, f, session, fact)
	before, err := f.Wait(context.Background(), "canonical", run)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Restart(context.Background()); err != nil {
		t.Fatal(err)
	}
	// Rebind the new serving peer through the existing retained-session
	// lookup before effectful controls; never forge an origin/session claim.
	params, _ := json.Marshal(map[string]any{"session_id": session})
	if _, err := f.Call(context.Background(), "session/get", params); err != nil {
		t.Fatal(err)
	}
	after, err := f.Wait(context.Background(), "canonical", run)
	if err != nil {
		t.Fatal(err)
	}
	if before.RecordID != after.RecordID || before.IngestionID != after.IngestionID || before.Revision != after.Revision || after.CanonicalCount != 1 || !strings.Contains(after.CanonicalBody, fact) {
		t.Fatalf("source changed across fixture restart: before=%+v after=%+v", before, after)
	}
	if before.ProcessID <= 0 || after.ProcessID <= 0 || before.ProcessID == after.ProcessID {
		t.Fatalf("restart reused process: before=%d after=%d", before.ProcessID, after.ProcessID)
	}
	t.Logf("fixture process restart: %d -> %d", before.ProcessID, after.ProcessID)
	var policy struct {
		Enabled bool `json:"enabled"`
	}
	memoryLoopAction(t, f, "diva.cognitive.policy.get", map[string]any{"session_id": session}, &policy)
	if policy.Enabled {
		t.Fatal("restarting changed the operator's disabled cognitive policy")
	}
	if len(f.ModelRequests()) != 0 {
		t.Fatal("model request counter did not reset with fresh process")
	}
}

func TestMemoryLoopFixtureCloseAfterRestartIsIdempotent(t *testing.T) {
	probe := genassembly.BuildDefault()
	if !probe.HasCognitiveFactory() {
		t.Skip("DIVA integration overlay required")
	}
	f := newMemoryLoopFixture(t, memoryLoopOptions{ConfigPath: memoryLoopConfig(t), ModelMode: "ack"})
	if err := f.Restart(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := f.Close(ctx); err != nil {
		t.Fatalf("first close: %v", err)
	}
	if err := f.Close(ctx); err != nil {
		t.Fatalf("repeated close: %v", err)
	}
}
