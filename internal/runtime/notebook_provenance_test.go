package runtime

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/sqlite"
)

// TestNotebookProvenanceSurvivesCompaction proves the N2 exclusion chain:
// a notebook tool result is marked at durable ToolOperation admission,
// the derived projected message inherits the mark, earlier plain rows are
// untouched, and a compaction summary folded over excluded content is
// conservatively tainted — so BML/cognitive ingest never receives
// notebook-derived content after replay or compaction.
func TestNotebookProvenanceSurvivesCompaction(t *testing.T) {
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "provenance.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backend.Close() })

	const sessionID = domain.SessionID("sess-nb-prov")
	const runID = domain.RunID("run-nb-prov")
	if err := backend.CreateSession(ctx, domain.Session{ID: sessionID, Title: "prov", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := backend.CreateRun(ctx, domain.Run{ID: runID, SessionID: sessionID, Status: domain.RunActive, CreatedAt: 2}); err != nil {
		t.Fatal(err)
	}

	svc := &Service{deps: ServiceDeps{
		Journal:        backend,
		Sessions:       backend,
		Runs:           backend,
		ToolOperations: backend,
		Compactions:    backend,
	}}
	svc.engine = &Engine{cfg: EngineConfig{MaxEventPayloadBytes: 64 << 10}}

	// Admission: a notebook tool stamps provenance; a plain tool does not.
	coordinator := serviceToolOperationCoordinator{service: svc, runID: runID, sessionID: sessionID}
	noteOp, err := coordinator.Admit(ctx, "call-note-1", "write_note", []byte(`{"content":"UNIQUE_NOTEBOOK_MARKER"}`), []byte(`{"content":"UNIQUE_NOTEBOOK_MARKER"}`), []byte(`{"content":"UNIQUE_NOTEBOOK_MARKER"}`))
	if err != nil {
		t.Fatalf("notebook admit: %v", err)
	}
	if noteOp.ContentOrigin != domain.ContentOriginNotebook || !noteOp.ExcludeAutomaticIngest {
		t.Fatalf("admitted op missing provenance: %+v", noteOp)
	}
	stored, err := backend.GetToolOperation(ctx, runID, "call-note-1")
	if err != nil {
		t.Fatalf("read op: %v", err)
	}
	if stored.ContentOrigin != domain.ContentOriginNotebook || !stored.ExcludeAutomaticIngest {
		t.Fatalf("durable op lost provenance: %+v", stored)
	}
	plainOp, err := coordinator.Admit(ctx, "call-plain-1", "bash", []byte(`{"command":"ls"}`), []byte(`{"command":"ls"}`), []byte(`{"command":"ls"}`))
	if err != nil {
		t.Fatalf("plain admit: %v", err)
	}
	if plainOp.ContentOrigin != "" || plainOp.ExcludeAutomaticIngest {
		t.Fatalf("plain op tainted: %+v", plainOp)
	}
	if excluded, err := backend.HasExcludedToolOperations(ctx, runID); err != nil || !excluded {
		t.Fatalf("exclusion marker unreadable: %v %v", excluded, err)
	}

	// Durable operation replay: a second admit with the same key returns the
	// recorded operation — provenance survives the replay path.
	replayedOp, err := coordinator.Admit(ctx, "call-note-1", "write_note", []byte(`{"content":"UNIQUE_NOTEBOOK_MARKER"}`), []byte(`{"content":"UNIQUE_NOTEBOOK_MARKER"}`), []byte(`{"content":"UNIQUE_NOTEBOOK_MARKER"}`))
	if err != nil {
		t.Fatalf("replay admit: %v", err)
	}
	if replayedOp.ContentOrigin != domain.ContentOriginNotebook || !replayedOp.ExcludeAutomaticIngest {
		t.Fatalf("replayed op lost provenance: %+v", replayedOp)
	}

	// Projection: journal a tool.finished for the note tool and project the
	// model feed — the derived message carries the exclusion.
	mapper := newEventMapper(runID, 0)
	var events []domain.RunEvent
	events = append(events, mapper.build(domain.EventToolFinished, payloadToolFinished{
		ToolCallID: "call-note-1", ToolName: "write_note", Result: `{"status":"ok"}`,
	}))
	events = append(events, mapper.build(domain.EventToolFinished, payloadToolFinished{
		ToolCallID: "call-plain-1", ToolName: "bash", Result: "file.txt",
	}))
	if _, err := backend.Append(ctx, storage.Commit{RunID: runID, Events: events}); err != nil {
		t.Fatal(err)
	}
	projected, err := svc.projectedMessages(ctx, sessionID, runID)
	if err != nil {
		t.Fatalf("project messages: %v", err)
	}
	var noteMsg, plainMsg *domain.Message
	for i := range projected {
		switch projected[i].ToolName {
		case "write_note":
			noteMsg = &projected[i]
		case "bash":
			plainMsg = &projected[i]
		}
	}
	if noteMsg == nil || plainMsg == nil {
		t.Fatalf("projection missing tool rows: %d", len(projected))
	}
	if noteMsg.ContentOrigin != domain.ContentOriginNotebook || !noteMsg.ExcludeAutomaticIngest {
		t.Fatalf("projected message lost provenance: %+v", noteMsg)
	}
	if plainMsg.ExcludeAutomaticIngest {
		t.Fatalf("retroactive taint on plain row: %+v", plainMsg)
	}

	// Compaction fold: a summary covering excluded content is tainted, while
	// a summary over only plain rows stays clean.
	storedHistory := []domain.Message{
		{ID: "m1", SessionID: sessionID, Role: domain.RoleTool, CreatedAt: 3, Content: "note result", ContentOrigin: domain.ContentOriginNotebook, ExcludeAutomaticIngest: true, ToolName: "write_note"},
		{ID: "m2", SessionID: sessionID, Role: domain.RoleTool, CreatedAt: 4, Content: "plain result", ToolName: "bash"},
		{ID: "m3", SessionID: sessionID, Role: domain.RoleUser, CreatedAt: 5, Content: "later turn"},
	}
	if svc.deps.Compactions != nil {
		if err := svc.deps.Compactions.SaveSessionCompaction(ctx, storage.SessionCompaction{SessionID: sessionID, RunID: runID, Summary: "earlier context", TailFrom: 4, CreatedAt: 10}); err != nil {
			t.Fatalf("save compaction: %v", err)
		}
		folded, ok := svc.foldSessionHistory(ctx, sessionID, storedHistory)
		if !ok {
			t.Fatal("fold skipped")
		}
		if len(folded) != 2 || !folded[0].ExcludeAutomaticIngest || folded[0].ContentOrigin != domain.ContentOriginNotebook {
			t.Fatalf("summary not tainted: %+v", folded)
		}
		if folded[1].Content != "later turn" {
			t.Fatalf("tail lost: %+v", folded)
		}
		// Clean range folds untainted.
		clean := []domain.Message{storedHistory[1], storedHistory[2]}
		foldedClean, ok := svc.foldSessionHistory(ctx, sessionID, clean)
		if ok && len(foldedClean) > 0 && foldedClean[0].ExcludeAutomaticIngest {
			t.Fatalf("clean summary tainted: %+v", foldedClean)
		}
	}
}

// JSON marshaling check for the envelope: durable op serialization carries
// the markers (used by journal payloads/exports).
func TestNotebookToolOperationSerializesProvenance(t *testing.T) {
	op := domain.ToolOperation{RunID: "r", OperationID: "o", ToolName: "write_note", CreatedAt: time.Now().UnixMilli(), UpdatedAt: time.Now().UnixMilli(), ContentOrigin: domain.ContentOriginNotebook, ExcludeAutomaticIngest: true}
	raw, err := json.Marshal(op)
	if err != nil {
		t.Fatal(err)
	}
	var back domain.ToolOperation
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if !back.ExcludeAutomaticIngest || back.ContentOrigin != domain.ContentOriginNotebook {
		t.Fatalf("provenance lost through JSON: %+v", back)
	}
}
