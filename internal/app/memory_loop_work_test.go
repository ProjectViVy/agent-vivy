package app

import (
	"context"
	"reflect"
	"strings"
	"testing"

	genassembly "agent-vivy/internal/generated/assembly"
	laputaevolution "github.com/dashimaki/laputa/evolution"
)

func TestMemoryLoopReconciliationReadsExistingWork(t *testing.T) {
	probe := genassembly.BuildDefault()
	if !probe.HasCognitiveFactory() {
		t.Skip("DIVA integration overlay required")
	}
	f := newMemoryLoopFixture(t, memoryLoopOptions{ConfigPath: memoryLoopConfig(t), ModelMode: "reflection"})
	session := memoryLoopSession(t, f)
	var activity laputaevolution.ActivityResult
	memoryLoopAction(t, f, "diva.cognitive.actmem.read", map[string]any{"session_id": session, "sections": []string{"work"}, "max_chars": 1200}, &activity)
	oldWork := memoryLoopRandomFact(t)
	patch := laputaevolution.WorkPatch{BaseRevision: activity.Revision, Changes: []laputaevolution.WorkChange{{Kind: laputaevolution.WorkChangeAdd, Field: laputaevolution.FieldGoal, Body: oldWork}}}
	memoryLoopAction(t, f, "diva.cognitive.actmem.work.patch", map[string]any{"session_id": session, "patch": patch}, &activity)
	if !activity.Changed || len(activity.Entries) != 1 {
		t.Fatalf("actual old Work write: %+v", activity)
	}
	memoryLoopEnable(t, f, session)
	run := memoryLoopTurn(t, f, session, memoryLoopRandomFact(t))
	if _, err := f.Wait(context.Background(), "reflected", run); err != nil {
		t.Fatal(err)
	}
	requests := f.ModelRequests()
	if len(requests) != 3 || strings.Contains(string(requests[0]), oldWork) {
		t.Fatal("old Work must remain absent from automatic foreground prompt")
	}
	if !strings.Contains(string(requests[1]), oldWork) || !strings.Contains(string(requests[1]), activity.Entries[0].ID) {
		t.Fatal("MEM-S05-02: reconcile model did not receive actual old scoped Work")
	}
	var after laputaevolution.ActivityResult
	memoryLoopAction(t, f, "diva.cognitive.actmem.read", map[string]any{"session_id": session, "sections": []string{"work"}, "max_chars": 1200}, &after)
	// Empty source slices are semantically identical across YAML/JSON reads.
	for i := range activity.Entries {
		if len(activity.Entries[i].Sources) == 0 {
			activity.Entries[i].Sources = nil
		}
	}
	for i := range after.Entries {
		if len(after.Entries[i].Sources) == 0 {
			after.Entries[i].Sources = nil
		}
	}
	// The terminal pair advances the profile-wide head once. Empty Work
	// reconciliation must preserve every Work field while using that new head.
	if !reflect.DeepEqual(after.Entries, activity.Entries) || after.Revision != activity.Revision+1 {
		t.Fatalf("empty reconciliation changed existing Work: before=%+v after=%+v", activity, after)
	}
}
