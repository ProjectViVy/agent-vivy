package app

import (
	"context"
	"testing"
	"time"

	genassembly "agent-vivy/internal/generated/assembly"
	"github.com/dashimaki/garden/agentapi"
)

func TestMemoryLoopMissionChangeFencesPendingEffects(t *testing.T) {
	probe := genassembly.BuildDefault()
	if !probe.HasCognitiveFactory() {
		t.Skip("DIVA integration overlay required")
	}
	t.Run("unassigned-to-assigned", func(t *testing.T) { runMemoryLoopMissionFence(t, false) })
	t.Run("assigned-revision-change", func(t *testing.T) { runMemoryLoopMissionFence(t, true) })
}

func runMemoryLoopMissionFence(t *testing.T, initiallyAssigned bool) {
	t.Helper()
	f := newMemoryLoopFixture(t, memoryLoopOptions{ConfigPath: memoryLoopConfig(t), ModelMode: "mission-fence"})
	session := memoryLoopSession(t, f)
	if initiallyAssigned {
		memoryLoopAction(t, f, "diva.cognitive.persona.save", map[string]any{"session_id": session, "kind": "mission", "content": "synthetic initial mission", "base_revision": 0, "reason": "initial test pin"}, nil)
	}
	memoryLoopEnable(t, f, session)
	run := memoryLoopTurn(t, f, session, memoryLoopRandomFact(t))
	source, err := f.Wait(context.Background(), "canonical", run)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for len(f.ModelRequests()) < 3 {
		select {
		case <-ctx.Done():
			t.Fatal("actual reflect request did not reach response gate")
		case <-ticker.C:
		}
	}
	var before memoryLoopCognitionStatus
	memoryLoopAction(t, f, "diva.cognitive.status", map[string]any{"session_id": session}, &before)
	if before.Cognition.ActiveRunID == "" || before.Cognition.Watermark != 0 {
		t.Fatalf("pre-effect window identity: %+v", before)
	}
	var mission agentapi.PersonaDocument
	memoryLoopAction(t, f, "diva.cognitive.persona.read", map[string]any{"session_id": session, "kind": "mission"}, &mission)
	oldRevision := mission.Revision
	memoryLoopAction(t, f, "diva.cognitive.persona.save", map[string]any{"session_id": session, "kind": "mission", "content": "synthetic new mission after inference admission", "base_revision": oldRevision, "reason": "synthetic actual authority change"}, nil)
	memoryLoopAction(t, f, "diva.cognitive.persona.read", map[string]any{"session_id": session, "kind": "mission"}, &mission)
	if mission.Revision <= oldRevision {
		t.Fatal("actual Mission revision did not advance")
	}
	f.modelReleaseOnce.Do(func() { close(f.modelRelease) })
	var after memoryLoopCognitionStatus
	for {
		if err := f.cognitiveValue(ctx, "diva.cognitive.status", session, nil, &after); err != nil {
			t.Fatalf("post-release status: last=%+v requests=%d error=%v", after, len(f.ModelRequests()), err)
		}
		if after.Cognition.Phase == "blocked" {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatalf("stale Mission workflow did not stop: %+v", after)
		case <-ticker.C:
		}
	}
	if after.Cognition.Watermark != 0 || after.Cognition.ActiveRunID != before.Cognition.ActiveRunID || after.Cognition.BlockReason == "" {
		t.Fatalf("stale binding lost its original unresolved identity: before=%+v after=%+v", before, after)
	}
	var page struct {
		Items []struct {
			Kind   string `json:"kind"`
			Status string `json:"status"`
		} `json:"items"`
	}
	memoryLoopAction(t, f, "diva.cognitive.results.list", map[string]any{"session_id": session, "limit": 100}, &page)
	for _, item := range page.Items {
		if item.Kind == "memory_mutation" && item.Status == "applied" {
			t.Fatal("stale Mission allowed pending memory effect")
		}
	}
	canonical, err := f.Wait(context.Background(), "canonical", run)
	if err != nil || canonical.CanonicalCount != 1 || canonical.RecordID != source.RecordID {
		t.Fatalf("stale binding mutated actual canonical: %+v %v", canonical, err)
	}
	assertMemoryLoopNoExtraRequests(t, f, 3, 6*time.Second)
}
