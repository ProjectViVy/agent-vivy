package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"agent-vivy/internal/cognitivecontract"
	"agent-vivy/internal/domain"
	genassembly "agent-vivy/internal/generated/assembly"
	"github.com/ProjectViVy/laputa/garden/agentapi"
)

// Controlled cursor rewind exercises the real host redelivery path. This is
// not a crash-cut recovery claim: all prior source/canonical commits exist.
func TestMemoryLoopCaptureReplayAndContentConflict(t *testing.T) {
	probe := genassembly.BuildDefault()
	if !probe.HasCognitiveFactory() {
		t.Skip("DIVA integration overlay required")
	}
	ctx := context.Background()
	f := newMemoryLoopFixture(t, memoryLoopOptions{ConfigPath: memoryLoopConfig(t), ModelMode: "reflection"})
	session := memoryLoopSession(t, f)
	runID := memoryLoopTurn(t, f, session, memoryLoopRandomFact(t))
	original, err := f.Wait(ctx, "canonical", runID)
	if err != nil {
		t.Fatal(err)
	}
	if f.app.observerHost == nil || f.app.cognitive == nil {
		t.Fatal("owned observer/cognitive composition missing")
	}
	iterator, err := f.app.backend.Replay(ctx, domain.RunID(runID), 0)
	if err != nil {
		t.Fatal(err)
	}
	var terminal domain.RunEvent
	for iterator.Next() {
		e := iterator.Value().Event
		if uint64(e.Seq) == original.EventSeq {
			terminal = e
		}
	}
	err = iterator.Err()
	_ = iterator.Close()
	if err != nil || terminal.Type != domain.EventRunCompleted {
		t.Fatalf("actual terminal event: %+v %v", terminal, err)
	}
	var payload struct {
		TenantID    string `json:"tenant_id"`
		WorkspaceID string `json:"workspace_id"`
	}
	if err := json.Unmarshal(terminal.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	cap := cognitivecontract.Capture{
		SubjectID: payload.TenantID, WorkspaceID: payload.WorkspaceID,
		SessionID: session, RunID: terminal.RunID,
		EventID: fmt.Sprintf("%s:%d", terminal.RunID, terminal.Seq),
		Phase:   "completed", Content: original.SourceBody, OccurredAt: terminal.CreatedAt,
	}
	replayed, err := f.app.cognitive.Sink().Capture(ctx, cap)
	if err != nil || replayed.IngestionID != original.IngestionID || replayed.Seq != original.CaptureSeq {
		t.Fatalf("same event/content did not rejoin original receipt: %+v %v", replayed, err)
	}
	cap.Content += " changed replay payload"
	_, err = f.app.cognitive.Sink().Capture(ctx, cap)
	var conflict *agentapi.Error
	if !errors.As(err, &conflict) || conflict.Code != "event_conflict" {
		t.Fatalf("changed event content not rejected: %v", err)
	}
	// Same key derivation as the observer host, test-only. Rewind the actual
	// task-owned snapshot with CAS, never a user profile or private SQL table.
	key := memoryLoopObserverCursorKey(runID)
	store := f.app.backend.Snapshot()
	deadline := time.Now().Add(5 * time.Second)
	for {
		value, _, err := store.Get(ctx, key)
		if err != nil {
			t.Fatal(err)
		}
		seq, _ := strconv.ParseUint(string(value), 10, 64)
		if seq >= original.EventSeq {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("original observer cursor did not commit")
		}
		time.Sleep(10 * time.Millisecond)
	}
	for i := 0; i < 3; i++ {
		_, version, err := store.Get(ctx, key)
		if err != nil {
			t.Fatal(err)
		}
		if err := store.Put(ctx, key, []byte("0"), version); err != nil {
			t.Fatal(err)
		}
		if err := f.app.observerHost.DeliverRun(ctx, domain.RunID(runID)); err != nil {
			t.Fatal(err)
		}
		value, _, err := store.Get(ctx, key)
		seq, parseErr := strconv.ParseUint(string(value), 10, 64)
		if err != nil || parseErr != nil || seq < original.EventSeq {
			t.Fatalf("redelivery did not acknowledge terminal: %q %v %v", value, err, parseErr)
		}
		assertMemoryLoopReplayUnchanged(t, f, runID, original)
	}
	if len(f.ModelRequests()) != 1 {
		t.Fatal("disabled reflection or capture replay invoked additional model requests")
	}
	if err := f.Restart(ctx); err != nil {
		t.Fatal(err)
	}
	assertMemoryLoopReplayUnchanged(t, f, runID, original)
	if len(f.ModelRequests()) != 0 {
		t.Fatal("source recovery invoked a model after restart")
	}
}

func assertMemoryLoopReplayUnchanged(t *testing.T, f *memoryLoopFixture, run string, want memoryLoopSnapshot) {
	t.Helper()
	got, err := f.Wait(context.Background(), "canonical", run)
	if err != nil || got.IngestionID != want.IngestionID || got.CaptureSeq != want.CaptureSeq || got.RecordID != want.RecordID || got.Revision != want.Revision || got.CanonicalCount != want.CanonicalCount || got.SourceHash != want.SourceHash || got.SourceBody != want.SourceBody || got.CanonicalBody != want.CanonicalBody {
		t.Fatalf("accepted source replay changed original commit: %+v %v", got, err)
	}
}
