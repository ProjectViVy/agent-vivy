package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	genassembly "agent-vivy/internal/generated/assembly"
	laputaevolution "github.com/dashimaki/laputa/evolution"
)

func TestMemoryLoopActivityContinuity(t *testing.T) {
	probe := genassembly.BuildDefault()
	if !probe.HasCognitiveFactory() {
		t.Skip("DIVA integration overlay required")
	}
	f := newMemoryLoopFixture(t, memoryLoopOptions{ConfigPath: memoryLoopConfig(t), ModelMode: "ack"})
	session := memoryLoopSession(t, f)
	facts := []string{memoryLoopRandomFact(t), memoryLoopRandomFact(t)}
	for _, fact := range facts {
		run := memoryLoopTurn(t, f, session, fact)
		source, err := f.Wait(context.Background(), "canonical", run)
		if err != nil {
			t.Fatal(err)
		}
		awaitMemoryLoopActivitySource(t, f, source.CaptureSeq)
	}
	var activity laputaevolution.ActivityResult
	memoryLoopAction(t, f, "diva.cognitive.actmem.read", map[string]any{"session_id": session, "sections": []string{"pulse", "recap"}, "max_chars": 1200}, &activity)
	counts := map[laputaevolution.EntrySection]int{}
	for _, entry := range activity.Entries {
		if entry.SessionID != session || entry.EventID == "" || len(entry.Sources) == 0 {
			t.Fatalf("activity source identity missing: %+v", entry)
		}
		counts[entry.Section]++
	}
	if counts[laputaevolution.SectionPulse] == 0 || counts[laputaevolution.SectionRecap] == 0 {
		t.Fatalf("MEM-S05-01: real completed turns left actual ACTMEM empty: revision=%d pulse=%d recap=%d (ingest rows are not Markdown activity)", activity.Revision, counts[laputaevolution.SectionPulse], counts[laputaevolution.SectionRecap])
	}
	for _, fact := range facts {
		found := false
		for _, entry := range activity.Entries {
			if entry.Section == laputaevolution.SectionRecap && strings.Contains(entry.Body, fact) {
				found = true
			}
		}
		if !found {
			t.Fatal("actual recap lost a short user-only fact")
		}
	}
	// Owned Close/Restart launches a distinct process; entry identities and
	// complete metadata must survive, with no source replay duplication.
	if err := f.Restart(context.Background()); err != nil {
		t.Fatal(err)
	}
	args, _ := json.Marshal(map[string]any{"session_id": session})
	if _, err := f.Call(context.Background(), "session/get", args); err != nil {
		t.Fatal(err)
	}
	var reopened laputaevolution.ActivityResult
	memoryLoopAction(t, f, "diva.cognitive.actmem.read", map[string]any{"session_id": session, "sections": []string{"pulse", "recap"}, "max_chars": 1200}, &reopened)
	if activity.Revision != reopened.Revision || !reflect.DeepEqual(activity.Entries, reopened.Entries) {
		t.Fatal("actual activity Restart changed original IDs, revision or provenance")
	}

}

func TestMemoryLoopActivityArchivesOnSessionDelete(t *testing.T) {
	probe := genassembly.BuildDefault()
	if !probe.HasCognitiveFactory() {
		t.Skip("DIVA integration overlay required")
	}
	f := newMemoryLoopFixture(t, memoryLoopOptions{ConfigPath: memoryLoopConfig(t), ModelMode: "ack"})
	session := memoryLoopSession(t, f)
	run := memoryLoopTurn(t, f, session, memoryLoopRandomFact(t))
	source, err := f.Wait(context.Background(), "canonical", run)
	if err != nil {
		t.Fatal(err)
	}
	awaitMemoryLoopActivitySource(t, f, source.CaptureSeq)
	var before laputaevolution.ActivityResult
	memoryLoopAction(t, f, "diva.cognitive.actmem.read", map[string]any{"session_id": session, "sections": []string{"pulse", "recap"}, "max_chars": 1200}, &before)
	if len(before.Entries) != 2 {
		t.Fatalf("expected actual terminal activity, got %d", len(before.Entries))
	}
	args, _ := json.Marshal(map[string]any{"session_id": session})
	if _, err := f.Call(context.Background(), "session/delete", args); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(f.dataRoot, "garden", "persona", "actmem", "ACTMEM.MD"))
	if err != nil {
		t.Fatal(err)
	}
	after, err := laputaevolution.ParseActmemDocument(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Sections[laputaevolution.SectionPulse])+len(after.Sections[laputaevolution.SectionRecap]) != 0 {
		t.Fatal("MEM-S05-02: deleted Session left unarchived head entries")
	}
	files, err := filepath.Glob(filepath.Join(f.dataRoot, "garden", "persona", "actmem", "capsules", "fold-*.md"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no actual session archive: %v %v", files, err)
	}
	for _, entry := range before.Entries {
		found := false
		for _, file := range files {
			body, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(body), entry.ID) && strings.Contains(string(body), entry.Body) {
				found = true
			}
		}
		if !found {
			t.Fatalf("archive lost original entry %s", entry.ID)
		}
	}
}

func TestMemoryLoopActivityDeleteDrainsLiveCancellation(t *testing.T) {
	probe := genassembly.BuildDefault()
	if !probe.HasCognitiveFactory() {
		t.Skip("DIVA integration overlay required")
	}
	f := newMemoryLoopFixture(t, memoryLoopOptions{ConfigPath: memoryLoopConfig(t), ModelMode: "wait-cancel"})
	session := memoryLoopSession(t, f)
	fact := memoryLoopRandomFact(t)
	_ = memoryLoopTurn(t, f, session, fact)
	deadline := time.Now().Add(10 * time.Second)
	for len(f.ModelRequests()) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("actual model request not admitted")
		}
		time.Sleep(time.Millisecond)
	}
	args, _ := json.Marshal(map[string]any{"session_id": session})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := f.Call(ctx, "session/delete", args); err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob(filepath.Join(f.dataRoot, "garden", "persona", "actmem", "capsules", "fold-*.md"))
	if err != nil || len(files) == 0 {
		t.Fatalf("MEM-S05-02: live deletion lost terminal archive: %v %v", files, err)
	}
	var archived strings.Builder
	for _, file := range files {
		body, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		archived.Write(body)
	}
	if !strings.Contains(archived.String(), fact) || !strings.Contains(archived.String(), "canceled") {
		t.Fatalf("live deletion archive lost admitted user or canceled terminal: fact=%q archive=%s", fact, archived.String())
	}
}

func TestMemoryLoopActivityKeepsStaleWorkPatchConflicted(t *testing.T) {
	probe := genassembly.BuildDefault()
	if !probe.HasCognitiveFactory() {
		t.Skip("DIVA integration overlay required")
	}
	f := newMemoryLoopFixture(t, memoryLoopOptions{ConfigPath: memoryLoopConfig(t), ModelMode: "ack"})
	session := memoryLoopSession(t, f)
	var before laputaevolution.ActivityResult
	memoryLoopAction(t, f, "diva.cognitive.actmem.read", map[string]any{"session_id": session, "sections": []string{"work"}, "max_chars": 1200}, &before)
	old := memoryLoopRandomFact(t)
	patch := laputaevolution.WorkPatch{BaseRevision: before.Revision, Changes: []laputaevolution.WorkChange{{Kind: laputaevolution.WorkChangeAdd, Field: laputaevolution.FieldGoal, Body: old}}}
	memoryLoopAction(t, f, "diva.cognitive.actmem.work.patch", map[string]any{"session_id": session, "patch": patch}, &before)
	run := memoryLoopTurn(t, f, session, memoryLoopRandomFact(t))
	source, err := f.Wait(context.Background(), "canonical", run)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	for {
		high, err := f.app.cognitive.Source().HighWatermark(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if high >= source.CaptureSeq {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(time.Millisecond):
		}
	}
	patch = laputaevolution.WorkPatch{BaseRevision: before.Revision, Changes: []laputaevolution.WorkChange{{Kind: laputaevolution.WorkChangeReplace, EntryID: before.Entries[0].ID, Field: laputaevolution.FieldGoal, Body: "stale replacement must not win"}}}
	args, _ := json.Marshal(map[string]any{"module_id": "vivy/diva-cognitive", "action_id": "diva.cognitive.actmem.work.patch", "input": map[string]any{"session_id": session, "patch": patch}})
	raw, err := f.Call(ctx, "module.action.invoke", args)
	if err != nil {
		t.Fatal(err)
	}
	var outcome struct {
		Status string `json:"status"`
		Error  *struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &outcome); err != nil || outcome.Status != "failed" || outcome.Error == nil || outcome.Error.Code != "actmem_revision_conflict" {
		t.Fatalf("actual stale Work outcome: %s %v", raw, err)
	}
	var after laputaevolution.ActivityResult
	memoryLoopAction(t, f, "diva.cognitive.actmem.read", map[string]any{"session_id": session, "sections": []string{"work"}, "max_chars": 1200}, &after)
	if len(after.Entries) != 1 || after.Entries[0].ID != before.Entries[0].ID || after.Entries[0].Body != old || after.Revision != before.Revision+1 {
		t.Fatalf("stale patch overwrote original Work/capture: %+v", after)
	}
}

// Canonical raw input and completed ACTMEM projection are separate durable
// boundaries. Await the actual owned source prefix; never infer readiness from
// an elapsed delay or fill a missing activity receipt in the fixture.
func awaitMemoryLoopActivitySource(t *testing.T, f *memoryLoopFixture, through uint64) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		high, err := f.app.cognitive.Source().HighWatermark(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if high >= through {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("native activity source prefix did not complete: high=%d want=%d", high, through)
		case <-ticker.C:
		}
	}
}
