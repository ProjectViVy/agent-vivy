package app

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	genassembly "agent-vivy/internal/generated/assembly"
)

func TestMemoryLoopOperatorRecoveryDoesNotInterruptOwnedRun(t *testing.T) {
	probe := genassembly.BuildDefault()
	if !probe.HasCognitiveFactory() {
		t.Skip("DIVA integration overlay required")
	}
	f := newMemoryLoopFixture(t, memoryLoopOptions{ConfigPath: memoryLoopConfig(t), ModelMode: "busy"})
	session := memoryLoopSession(t, f)
	primary := memoryLoopTurn(t, f, session, "[hold-foreground] "+memoryLoopRandomFact(t))
	deadline := time.Now().Add(5 * time.Second)
	for len(f.ModelRequests()) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("actual owned foreground response not held")
		}
		time.Sleep(time.Millisecond)
	}
	raw, err := f.Call(context.Background(), "background/recover", json.RawMessage(`{}`))
	if err == nil {
		t.Fatalf("operator recovery accepted while owned primary is executing: %s", raw)
	}
	run, err := f.app.backend.GetRun(context.Background(), domain.RunID(primary))
	if err != nil || run.Status.Terminal() {
		t.Fatalf("refused recovery interrupted owned run: %+v %v", run, err)
	}
	f.modelReleaseOnce.Do(func() { close(f.modelRelease) })
	source, err := f.Wait(context.Background(), "canonical", primary)
	if err != nil || source.State != "completed" || source.CanonicalCount != 1 {
		t.Fatalf("original owned primary did not complete normally: %+v %v", source, err)
	}
	awaitMemoryLoopActivitySource(t, f, source.CaptureSeq)
	raw, err = f.Call(context.Background(), "background/recover", json.RawMessage(`{}`))
	var result struct {
		Recovered bool `json:"recovered"`
	}
	if err != nil || json.Unmarshal(raw, &result) != nil || !result.Recovered {
		t.Fatalf("idle operator recovery failed: %s %v", raw, err)
	}
	after, err := f.Wait(context.Background(), "canonical", primary)
	if err != nil || source.IngestionID != after.IngestionID || source.RecordID != after.RecordID || source.Revision != after.Revision || after.CanonicalCount != 1 || len(f.ModelRequests()) != 1 {
		t.Fatalf("idle recovery changed original source or reran inference: %+v %v", after, err)
	}
	saveMemoryLoopDevelopmentEvidence(t, f, session, "recovered", source, after)
}
