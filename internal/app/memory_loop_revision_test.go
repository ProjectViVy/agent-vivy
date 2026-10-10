package app

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	genassembly "agent-vivy/internal/generated/assembly"
	"github.com/dashimaki/garden/memory"
	laputaevolution "github.com/dashimaki/laputa/evolution"
)

func TestMemoryLoopCorrectionAndDeletionInModelInput(t *testing.T) {
	probe := genassembly.BuildDefault()
	if !probe.HasCognitiveFactory() {
		t.Skip("DIVA integration overlay required")
	}
	f := newMemoryLoopFixture(t, memoryLoopOptions{ConfigPath: memoryLoopConfig(t), ModelMode: "recall"})
	a := memoryLoopSession(t, f)
	memoryLoopEnable(t, f, a)
	oldFact := memoryLoopRandomFact(t)
	runA := memoryLoopTurn(t, f, a, oldFact)
	original, err := f.Wait(context.Background(), "reflected", runA)
	if err != nil {
		t.Fatal(err)
	}
	var policy struct {
		Revision uint64 `json:"policy_revision"`
	}
	memoryLoopAction(t, f, "diva.cognitive.policy.get", map[string]any{"session_id": a}, &policy)
	memoryLoopAction(t, f, "diva.cognitive.policy.set", map[string]any{"session_id": a, "enabled": false, "min_interval_ms": 0, "base_revision": policy.Revision}, nil)
	newFact := memoryLoopRandomFact(t)
	mutation := memory.AuthorizedMutation{Operation: laputaevolution.MutationUpdate, RecordID: original.RecordID, ExpectedRevision: original.Revision, Body: newFact, Inference: laputaevolution.InferenceObserved, Sources: original.CanonicalSources}
	var correction memory.MutationReceipt
	memoryLoopAction(t, f, "diva.cognitive.memory.mutate", map[string]any{"session_id": a, "mutation": mutation}, &correction)
	if correction.Status != laputaevolution.StatusApplied || correction.TargetRef != original.RecordID || correction.Revision != original.Revision+1 {
		t.Fatalf("actual correction: %+v", correction)
	}
	if err := f.Restart(context.Background()); err != nil {
		t.Fatal(err)
	}
	b := memoryLoopSession(t, f)
	runB := memoryLoopTurn(t, f, b, "What synthetic user-only fact do you remember from previous conversations?")
	terminal, err := f.Wait(context.Background(), "terminal", runB)
	if err != nil || terminal.State != "completed" {
		t.Fatalf("corrected recall: %+v %v", terminal, err)
	}
	requests := f.ModelRequests()
	if len(requests) != 1 || !strings.Contains(string(requests[0]), newFact) || strings.Contains(string(requests[0]), oldFact) {
		t.Fatalf("corrected actual model input: %s", requests)
	}
	assertMemoryLoopAssistantAnswer(t, f, b, newFact)
	mutation = memory.AuthorizedMutation{Operation: laputaevolution.MutationTombstone, RecordID: original.RecordID, ExpectedRevision: correction.Revision, Inference: laputaevolution.InferenceObserved}
	var deleted memory.MutationReceipt
	memoryLoopAction(t, f, "diva.cognitive.memory.mutate", map[string]any{"session_id": b, "mutation": mutation}, &deleted)
	if deleted.Status != laputaevolution.StatusApplied || deleted.Revision != correction.Revision+1 {
		t.Fatalf("actual deletion: %+v", deleted)
	}
	if err := f.Restart(context.Background()); err != nil {
		t.Fatal(err)
	}
	c := memoryLoopSession(t, f)
	if c == a || c == b {
		t.Fatal("deleted control reused old transcript")
	}
	runC := memoryLoopTurn(t, f, c, "What synthetic user-only fact do you remember from previous conversations?")
	terminal, err = f.Wait(context.Background(), "terminal", runC)
	if err != nil || terminal.State != "completed" {
		t.Fatalf("deleted recall: %+v %v", terminal, err)
	}
	requests = f.ModelRequests()
	if len(requests) != 1 || strings.Contains(string(requests[0]), newFact) || strings.Contains(string(requests[0]), oldFact) || strings.Contains(string(requests[0]), "[context: vivy.memory.mentle/"+original.RecordID) {
		t.Fatalf("deleted fact/old revision resurrected in model input: %s", requests)
	}
	assertMemoryLoopAssistantAnswer(t, f, c, "未知")
	rawSource, err := f.Wait(context.Background(), "canonical", runA)
	if err != nil || !strings.Contains(rawSource.SourceBody, oldFact) {
		t.Fatalf("negative control erased original raw source: %+v %v", rawSource, err)
	}
}

// Developer storage/control proof. The complete S09 case still needs ordinary
// Agent recall and actual corrected/deleted model input, supplied by S08.
func TestMemoryLoopPublicCorrectionAndTombstoneAfterRestart(t *testing.T) {
	probe := genassembly.BuildDefault()
	if !probe.HasCognitiveFactory() {
		t.Skip("DIVA integration overlay required")
	}
	f := newMemoryLoopFixture(t, memoryLoopOptions{ConfigPath: memoryLoopConfig(t), ModelMode: "reflection"})
	session := memoryLoopSession(t, f)
	memoryLoopEnable(t, f, session)
	run := memoryLoopTurn(t, f, session, memoryLoopRandomFact(t))
	original, err := f.Wait(context.Background(), "reflected", run)
	if err != nil {
		t.Fatal(err)
	}
	var policy struct {
		Revision uint64 `json:"policy_revision"`
	}
	memoryLoopAction(t, f, "diva.cognitive.policy.get", map[string]any{"session_id": session}, &policy)
	memoryLoopAction(t, f, "diva.cognitive.policy.set", map[string]any{"session_id": session, "enabled": false, "min_interval_ms": 0, "base_revision": policy.Revision}, nil)
	corrected := memoryLoopRandomFact(t)
	mutation := memory.AuthorizedMutation{Operation: laputaevolution.MutationUpdate, RecordID: original.RecordID, ExpectedRevision: original.Revision, Body: corrected, Inference: laputaevolution.InferenceObserved, Sources: original.CanonicalSources}
	var update memory.MutationReceipt
	memoryLoopAction(t, f, "diva.cognitive.memory.mutate", map[string]any{"session_id": session, "mutation": mutation}, &update)
	if update.Status != laputaevolution.StatusApplied || update.TargetRef != original.RecordID || update.Revision != original.Revision+1 {
		t.Fatalf("actual correction receipt: %+v", update)
	}
	var cards memory.CardPage
	memoryLoopAction(t, f, "diva.cognitive.memory.search", map[string]any{"session_id": session, "query": corrected, "limit": 20, "budget_chars": 1200}, &cards)
	found := false
	for _, card := range cards.Items {
		if card.ID == original.RecordID && uint64(card.Revision) == update.Revision {
			found = true
		}
	}
	if !found {
		t.Fatalf("corrected current card missing: %+v", cards)
	}
	var evidence memory.EvidencePage
	memoryLoopAction(t, f, "diva.cognitive.memory.expand", map[string]any{"session_id": session, "card_id": original.RecordID, "expected_revision": update.Revision, "budget_chars": 1200}, &evidence)
	if len(evidence.Items) != 1 || evidence.Items[0].Excerpt != corrected || evidence.Items[0].Revision != update.Revision {
		t.Fatalf("current corrected evidence: %+v", evidence)
	}
	assertMemoryLoopExpansionDenied(t, f, session, original.RecordID, original.Revision, "revision_conflict")
	assertMemoryLoopMutationDenied(t, f, session, mutation)
	memoryLoopAction(t, f, "diva.cognitive.memory.expand", map[string]any{"session_id": session, "card_id": original.RecordID, "expected_revision": update.Revision, "budget_chars": 1200}, &evidence)
	if len(evidence.Items) != 1 || evidence.Items[0].Excerpt != corrected || evidence.Items[0].Revision != update.Revision {
		t.Fatalf("rejected stale CAS changed corrected evidence: %+v", evidence)
	}
	mutation = memory.AuthorizedMutation{Operation: laputaevolution.MutationTombstone, RecordID: original.RecordID, ExpectedRevision: update.Revision, Inference: laputaevolution.InferenceObserved}
	var deleted memory.MutationReceipt
	memoryLoopAction(t, f, "diva.cognitive.memory.mutate", map[string]any{"session_id": session, "mutation": mutation}, &deleted)
	if deleted.Status != laputaevolution.StatusApplied || deleted.Revision != update.Revision+1 {
		t.Fatalf("actual tombstone: %+v", deleted)
	}
	if err := f.Restart(context.Background()); err != nil {
		t.Fatal(err)
	}
	params, _ := json.Marshal(map[string]any{"session_id": session})
	if _, err := f.Call(context.Background(), "session/get", params); err != nil {
		t.Fatal(err)
	}
	memoryLoopAction(t, f, "diva.cognitive.memory.search", map[string]any{"session_id": session, "query": corrected, "limit": 20, "budget_chars": 1200}, &cards)
	for _, card := range cards.Items {
		if card.ID == original.RecordID {
			t.Fatal("deleted record returned as a current card after restart")
		}
	}
	assertMemoryLoopExpansionDenied(t, f, session, original.RecordID, update.Revision, "effect_not_found")
	for _, want := range []struct {
		id       string
		revision uint64
	}{{original.OperationID, original.Revision}, {update.OperationID, update.Revision}, {deleted.OperationID, deleted.Revision}} {
		var receipt memory.MutationReceipt
		memoryLoopAction(t, f, "diva.cognitive.memory.receipt", map[string]any{"session_id": session, "operation_id": want.id}, &receipt)
		if receipt.OperationID != want.id || receipt.Revision != want.revision || receipt.TargetRef != original.RecordID || receipt.Status != laputaevolution.StatusApplied {
			t.Fatalf("original committed receipt changed: %+v", receipt)
		}
	}
	mutation = memory.AuthorizedMutation{Operation: laputaevolution.MutationCreate, RecordID: original.RecordID, ExpectedAbsent: true, Body: original.CanonicalBody, Inference: laputaevolution.InferenceObserved, Sources: original.CanonicalSources}
	// The native facade mints create IDs. A fresh human create is a new
	// operation, not replay of the original record or its tombstone receipt.
	var fresh memory.MutationReceipt
	memoryLoopAction(t, f, "diva.cognitive.memory.mutate", map[string]any{"session_id": session, "mutation": mutation}, &fresh)
	if fresh.Status != laputaevolution.StatusApplied || fresh.TargetRef == "" || fresh.TargetRef == original.RecordID || fresh.OperationID == original.OperationID || fresh.OperationID == deleted.OperationID || fresh.Revision != 1 {
		t.Fatalf("fresh create did not preserve tombstoned identity: %+v", fresh)
	}
	assertMemoryLoopExpansionDenied(t, f, session, original.RecordID, deleted.Revision, "effect_not_found")
	if len(f.ModelRequests()) != 0 {
		t.Fatal("public lifecycle/receipt checks invoked a model after restart")
	}
}

func assertMemoryLoopMutationDenied(t *testing.T, f *memoryLoopFixture, session string, mutation memory.AuthorizedMutation) {
	t.Helper()
	args, _ := json.Marshal(map[string]any{"module_id": "vivy/diva-cognitive", "action_id": "diva.cognitive.memory.mutate", "input": map[string]any{"session_id": session, "mutation": mutation}})
	raw, err := f.Call(context.Background(), "module.action.invoke", args)
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Status string                 `json:"status"`
		Value  memory.MutationReceipt `json:"value"`
		Error  struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if json.Unmarshal(raw, &out) != nil || out.Status != "failed" || out.Value.Status == laputaevolution.StatusApplied || out.Error.Code != "revision_conflict" {
		t.Fatalf("invalid mutation did not fail closed: %s", raw)
	}
}

func assertMemoryLoopExpansionDenied(t *testing.T, f *memoryLoopFixture, session, record string, revision uint64, code string) {
	t.Helper()
	args, _ := json.Marshal(map[string]any{"module_id": "vivy/diva-cognitive", "action_id": "diva.cognitive.memory.expand", "input": map[string]any{"session_id": session, "card_id": record, "expected_revision": revision, "budget_chars": 1200}})
	raw, err := f.Call(context.Background(), "module.action.invoke", args)
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Status string              `json:"status"`
		Value  memory.EvidencePage `json:"value"`
		Error  struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if json.Unmarshal(raw, &out) != nil || out.Status != "failed" || len(out.Value.Items) != 0 || out.Error.Code != code {
		t.Fatalf("invalid/stale expansion did not fail closed: %s", raw)
	}
}
