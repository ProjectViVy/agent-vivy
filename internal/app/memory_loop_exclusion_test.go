package app

import (
	"context"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	genassembly "agent-vivy/internal/generated/assembly"
)

func TestMemoryLoopExcludesDerivedEvidence(t *testing.T) {
	probe := genassembly.BuildDefault()
	if !probe.HasCognitiveFactory() {
		t.Skip("DIVA integration overlay required")
	}
	ctx := context.Background()
	f := newMemoryLoopFixture(t, memoryLoopOptions{ConfigPath: memoryLoopConfig(t), ModelMode: "reflection"})
	session := memoryLoopSession(t, f)
	memoryLoopEnable(t, f, session)
	run := memoryLoopTurn(t, f, session, memoryLoopRandomFact(t))
	source, err := f.Wait(ctx, "canonical", run)
	if err != nil {
		t.Fatal(err)
	}
	reflected, err := f.Wait(ctx, "reflected", run)
	if err != nil {
		t.Fatal(err)
	}
	if reflected.CanonicalCount != 2 || reflected.RecordID == source.RecordID || reflected.OperationID == "" {
		t.Fatalf("no real derived memory effect: %+v", reflected)
	}
	sessions, err := f.app.backend.ListSessions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[domain.RunKind]int{}
	supervisor := 0
	for _, session := range sessions {
		runs, err := f.app.backend.ListRunsBySession(ctx, session.ID)
		if err != nil {
			t.Fatal(err)
		}
		for _, run := range runs {
			kinds[run.Kind]++
			if run.SessionID == "sess_cognitive_supervisor" {
				supervisor++
			}
		}
	}
	if kinds[domain.RunKindWorkflow] < 1 || kinds[domain.RunKindChild] < 2 || supervisor < 1 {
		t.Fatalf("real derived runs not present: kinds=%v supervisor=%d", kinds, supervisor)
	}
	// Observe two actual default wake intervals, then inspect the native ledger.
	// Workflow/children/supervisor and canonical reflection output must not be
	// admitted as a new conversation source or cause another inference cycle.
	assertMemoryLoopNoExtraRequests(t, f, 3, 11*time.Second)
	db, err := f.readOnlyDB("garden", "garden.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM ingestions`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	high, err := f.app.cognitive.Source().HighWatermark(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var state struct {
		Cognition struct {
			Watermark      uint64 `json:"watermark"`
			PendingThrough uint64 `json:"pending_through"`
			Phase          string `json:"phase"`
		} `json:"cognition"`
	}
	memoryLoopAction(t, f, "diva.cognitive.status", map[string]any{"session_id": session}, &state)
	if count != 1 || high != source.CaptureSeq || state.Cognition.Watermark != source.CaptureSeq || state.Cognition.PendingThrough != source.CaptureSeq || state.Cognition.Phase != "idle" {
		t.Fatalf("derived activity reentered actual source: rows=%d high=%d source=%d state=%+v", count, high, source.CaptureSeq, state)
	}
	saveMemoryLoopDevelopmentEvidence(t, f, session, "settled", source, reflected)
}
