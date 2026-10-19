package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"testing"

	genassembly "agent-vivy/internal/generated/assembly"
)

func TestMemoryLoopReflectionProvenanceAfterRestart(t *testing.T) {
	probe := genassembly.BuildDefault()
	if !probe.HasCognitiveFactory() {
		t.Skip("DIVA integration overlay required")
	}
	f := newMemoryLoopFixture(t, memoryLoopOptions{ConfigPath: memoryLoopConfig(t), ModelMode: "reflection"})
	sessionID := createCognitiveSession(t, f.peer)
	memoryLoopEnable(t, f, sessionID)
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		t.Fatal(err)
	}
	runID := memoryLoopTurn(t, f, sessionID, "process-provenance="+hex.EncodeToString(nonce))
	before, err := f.Wait(context.Background(), "reflected", runID)
	if err != nil || len(before.CanonicalSources) == 0 {
		t.Fatalf("original canonical provenance missing: %+v %v", before, err)
	}
	if err := f.Restart(context.Background()); err != nil {
		t.Fatal(err)
	}
	params, _ := json.Marshal(map[string]any{"session_id": sessionID})
	if _, err := f.Call(context.Background(), "session/get", params); err != nil {
		t.Fatal(err)
	}
	after, err := f.Wait(context.Background(), "reflected", runID)
	if err != nil {
		t.Fatal(err)
	}
	if before.ProcessID == after.ProcessID || before.RecordID != after.RecordID || before.OperationID != after.OperationID || before.Revision != after.Revision || before.CanonicalBody != after.CanonicalBody || after.CanonicalCount != 2 || !reflect.DeepEqual(before.CanonicalSources, after.CanonicalSources) || before.PrimarySourceURI != after.PrimarySourceURI || before.PrimarySourceRevision != after.PrimarySourceRevision {
		t.Fatalf("process reopen changed ordinary memory/provenance: before=%+v after=%+v", before, after)
	}
	if len(f.ModelRequests()) != 0 {
		t.Fatal("reading settled provenance invoked a new model operation")
	}
}
