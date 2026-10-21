package conformance

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

func presentationMutation(sid domain.SessionID, run domain.RunID, item string, content []byte) storage.ContinuityMutation {
	body, _ := json.Marshal(map[string]any{"delivery_set": map[string]any{
		"id": "set-" + item, "session_id": sid, "run_id": run, "tool_call_id": "call-" + item, "created_at": 8, "status": "ok",
		"items": []map[string]any{{"id": item, "session_id": sid, "run_id": run, "workspace_id": "w", "path": item + ".txt", "name": item + ".txt", "size": len(content), "media_type": "text/plain", "captured_at": 8, "origin_tool_call_id": "call-" + item}},
	}})
	return storage.ContinuityMutation{SessionID: sid, RunID: run,
		Event:         domain.RunEvent{RunID: run, Type: domain.EventDeliverablesPresented, CreatedAt: 8, PayloadVersion: 1, Payload: body},
		Receipt:       domain.ContinuityReceipt{SessionID: sid, Operation: storage.ContinuityOperationPresent, RequestID: "call-" + item, Input: []byte(`{"path":"` + item + `"}`)},
		Presentations: []storage.DeliverablePresentation{{SessionID: sid, ItemID: item, Content: content}},
	}
}

func AssertDeliverablePresentations(t *testing.T, slot Slot) {
	t.Helper()
	ctx := context.Background()
	b := slot.Engine
	sid := domain.SessionID("presentation-owner")
	createHistorySession(t, b, sid)
	store := continuityStore(t, b)
	if _, err := store.CommitContinuityRun(ctx, continuityAdmission(sid, "presentation-run", "owning-message", "admit", "admit")); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range []struct {
		id      string
		content []byte
	}{{"full", []byte("private bytes")}, {"empty", []byte{}}} {
		mutation := presentationMutation(sid, "presentation-run", fixture.id, fixture.content)
		if _, err := store.CommitContinuityOperation(ctx, mutation); err != nil {
			t.Fatal(err)
		}
		got, err := store.ReadDeliverablePresentation(ctx, sid, fixture.id)
		if err != nil || got == nil || !bytes.Equal(got, fixture.content) {
			t.Fatalf("captured bytes unavailable: %q %v", got, err)
		}
		mutation.Presentations[0].Content = []byte("later filesystem capture")
		if result, err := store.CommitContinuityOperation(ctx, mutation); err != nil || result.NewlyCommitted {
			t.Fatalf("retry replaced capture: %v", err)
		}
		got, err = store.ReadDeliverablePresentation(ctx, sid, fixture.id)
		if err != nil || !bytes.Equal(got, fixture.content) {
			t.Fatalf("retry changed bytes: %q %v", got, err)
		}
	}
	if _, err := store.ReadDeliverablePresentation(ctx, "foreign", "full"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("foreign owner read: %v", err)
	}
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := slot.Reopen()
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	store = continuityStore(t, reopened)
	if got, err := store.ReadDeliverablePresentation(ctx, sid, "full"); err != nil || string(got) != "private bytes" {
		t.Fatalf("reopen lost bytes: %q %v", got, err)
	}
	if got, err := store.ReadDeliverablePresentation(ctx, sid, "empty"); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("reopen lost known empty: %v", err)
	}
	if err := reopened.DeleteSession(ctx, sid); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReadDeliverablePresentation(ctx, sid, "full"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("delete retained private bytes: %v", err)
	}
}

func AssertPresentationRollbackAndValidation(t *testing.T, slot Slot) {
	t.Helper()
	ctx := context.Background()
	b := slot.Engine
	store := continuityStore(t, b)
	sid := domain.SessionID("presentation-rollback")
	createHistorySession(t, b, sid)
	if _, err := store.CommitContinuityRun(ctx, continuityAdmission(sid, "rollback-run", "rollback-message", "admit", "admit")); err != nil {
		t.Fatal(err)
	}
	for _, step := range []string{"events", "presentations", "receipt"} {
		m := presentationMutation(sid, "rollback-run", step, []byte("private canary"))
		m.TestFaultHook = func(actual string) error {
			if actual == step {
				return errors.New("injected")
			}
			return nil
		}
		if _, err := store.CommitContinuityOperation(ctx, m); err == nil {
			t.Fatal("injected failure committed")
		}
		if _, err := store.ReadDeliverablePresentation(ctx, sid, step); !errors.Is(err, storage.ErrNotFound) {
			t.Fatalf("fault retained bytes: %v", err)
		}
		if _, found, err := store.FindContinuityReceipt(ctx, sid, m.Receipt.Operation, m.Receipt.RequestID); err != nil || found {
			t.Fatalf("fault retained receipt: %v", err)
		}
	}
	if events := replayAll(t, b, "rollback-run", 0); len(events) != 2 {
		t.Fatalf("fault retained presentation event: %d", len(events))
	}
	for _, wrong := range []string{"owner", "item", "size", "missing", "unexpected-event", "metadata-owner", "oversize", "aggregate"} {
		m := presentationMutation(sid, "rollback-run", wrong, []byte("expected"))
		switch wrong {
		case "owner":
			m.Presentations[0].SessionID = "foreign"
		case "item":
			m.Presentations[0].ItemID = "another"
		case "size":
			m.Presentations[0].Content = []byte("changed-size")
		case "missing":
			m.Presentations = nil
		case "unexpected-event":
			m.Event.Type = domain.EventContextReferenceAttached
		case "metadata-owner":
			var body map[string]any
			_ = json.Unmarshal(m.Event.Payload, &body)
			body["delivery_set"].(map[string]any)["session_id"] = "foreign"
			m.Event.Payload, _ = json.Marshal(body)
		case "oversize", "aggregate":
			var body map[string]any
			_ = json.Unmarshal(m.Event.Payload, &body)
			set := body["delivery_set"].(map[string]any)
			items := set["items"].([]any)
			item := items[0].(map[string]any)
			if wrong == "oversize" {
				item["size"] = int64(32<<20) + 1
			} else {
				out := []any{}
				for i := 0; i < 5; i++ {
					copy := map[string]any{}
					for k, v := range item {
						copy[k] = v
					}
					copy["id"] = fmt.Sprintf("over-%d", i)
					copy["path"] = fmt.Sprintf("over-%d", i)
					copy["size"] = 32 << 20
					out = append(out, copy)
				}
				set["items"] = out
			}
			m.Event.Payload, _ = json.Marshal(body)
		}
		if _, err := store.CommitContinuityOperation(ctx, m); err == nil {
			t.Fatalf("invalid %s presentation committed", wrong)
		}
	}
	if events := replayAll(t, b, "rollback-run", 0); len(events) != 2 {
		t.Fatalf("invalid bytes leaked events: %d", len(events))
	}
}

func AssertPresentationVisibility(t *testing.T, slot Slot) {
	t.Helper()
	ctx := context.Background()
	b := slot.Engine
	store := continuityStore(t, b)
	sid := domain.SessionID("presentation-visibility")
	createHistorySession(t, b, sid)
	if _, err := store.CommitContinuityRun(ctx, continuityAdmission(sid, "hidden-run", "owning-message", "admit", "admit")); err != nil {
		t.Fatal(err)
	}
	if err := b.CreateRun(ctx, domain.Run{ID: "unowned-run", SessionID: sid, Kind: domain.RunKindWorkflow, Status: domain.RunActive, CreatedAt: 9}); err != nil {
		t.Fatal(err)
	}
	for _, run := range []domain.RunID{"hidden-run", "unowned-run"} {
		if _, err := store.CommitContinuityOperation(ctx, presentationMutation(sid, run, string(run), []byte("captured"))); err != nil {
			t.Fatal(err)
		}
	}
	// These backdated anchors bracket the owner in ListMessages order despite
	// being inserted later; presentation retention follows the existing fold.
	for _, message := range []domain.Message{{ID: "cut", SessionID: sid, Role: domain.RoleAssistant, CreatedAt: 6, Content: "cut"}, {ID: "tail", SessionID: sid, Role: domain.RoleAssistant, CreatedAt: 8, Content: "tail"}} {
		if err := b.AppendMessage(ctx, message); err != nil {
			t.Fatal(err)
		}
	}
	marker := storage.SessionTruncation{SessionID: sid, CutoffMessageID: "cut", TailMessageID: "tail", Reason: storage.TruncationFork, CreatedAt: 10}
	if err := b.RecordSessionTruncation(ctx, marker); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReadDeliverablePresentation(ctx, sid, "hidden-run"); err != nil {
		t.Fatal("fork marker pruned bytes")
	}
	marker.Reason = storage.TruncationRewind
	if err := b.RecordSessionTruncation(ctx, marker); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReadDeliverablePresentation(ctx, sid, "hidden-run"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("rewind retained bytes: %v", err)
	}
	if _, err := store.ReadDeliverablePresentation(ctx, sid, "unowned-run"); err != nil {
		t.Fatalf("unowned run was pruned: %v", err)
	}
	// Existing receipts replay without recreating bytes; new hidden presentations refuse.
	if result, err := store.CommitContinuityOperation(ctx, presentationMutation(sid, "hidden-run", "hidden-run", []byte("captured"))); err != nil || result.NewlyCommitted {
		t.Fatalf("hidden receipt replay: %v", err)
	}
	if _, err := store.ReadDeliverablePresentation(ctx, sid, "hidden-run"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatal("replay resurrected bytes")
	}
	if _, err := store.CommitContinuityOperation(ctx, presentationMutation(sid, "hidden-run", "repopulate", []byte("new"))); !errors.Is(err, storage.ErrSourceChanged) {
		t.Fatalf("hidden run repopulated bytes: %v", err)
	}
	failed := presentationMutation(sid, "hidden-run", "failed-hidden", nil)
	var body map[string]any
	_ = json.Unmarshal(failed.Event.Payload, &body)
	set := body["delivery_set"].(map[string]any)
	set["items"] = []any{}
	set["status"] = "failed"
	set["failures"] = []map[string]string{{"path": "missing.txt", "reason": "not_found"}}
	failed.Event.Payload, _ = json.Marshal(body)
	failed.Presentations = nil
	if _, err := store.CommitContinuityOperation(ctx, failed); !errors.Is(err, storage.ErrSourceChanged) {
		t.Fatalf("hidden failure presentation committed: %v", err)
	}
}

func AssertPresentationHistoryTransactions(t *testing.T, slot Slot) {
	t.Helper()
	ctx := context.Background()
	b := slot.Engine
	store := continuityStore(t, b)
	for _, path := range []string{"direct-edit", "rewind-transaction", "edit-transaction", "edit-admission"} {
		sid := domain.SessionID(path)
		run := domain.RunID(path + "-run")
		item := path + "-item"
		createHistorySession(t, b, sid)
		if _, err := store.CommitContinuityRun(ctx, continuityAdmission(sid, run, path+"-message", "admit", "admit")); err != nil {
			t.Fatal(err)
		}
		if _, err := store.CommitContinuityOperation(ctx, presentationMutation(sid, run, item, []byte("retained until commit"))); err != nil {
			t.Fatal(err)
		}
		marker := storage.SessionTruncation{SessionID: sid, CutoffMessageID: path + "-message", TailMessageID: path + "-message", Reason: storage.TruncationEdit, CreatedAt: 10}
		editedRun := domain.Run{ID: domain.RunID(path + "-replacement"), SessionID: sid, Status: domain.RunActive, CreatedAt: 20}
		message := domain.Message{ID: path + "-new-message", SessionID: sid, RunID: editedRun.ID, Role: domain.RoleUser, CreatedAt: 20, Content: "replacement"}
		event := domain.RunEvent{RunID: editedRun.ID, Type: domain.EventRunStarted, CreatedAt: 20, PayloadVersion: 1, Payload: []byte(`{}`)}
		if path == "edit-admission" {
			admission := storage.RunAdmission{Message: message, Run: editedRun, Started: event, Edit: &marker}
			admissionStore := b.(storage.RunAdmissionStore)
			// A preexisting Journal position injects a failure after marker/prune writes.
			failed := admission
			failed.Run.ID = domain.RunID(path + "-fault")
			failed.Message.ID = path + "-fault-message"
			failed.Message.RunID = failed.Run.ID
			failed.Started.RunID = failed.Run.ID
			if _, err := b.Append(ctx, storage.Commit{RunID: failed.Run.ID, Events: []domain.RunEvent{failed.Started}}); err != nil {
				t.Fatal(err)
			}
			if _, err := admissionStore.CommitRunAdmission(ctx, failed); err == nil {
				t.Fatal("expected Journal position failure")
			}
			if _, err := store.ReadDeliverablePresentation(ctx, sid, item); err != nil {
				t.Fatalf("failed edit pruned bytes: %v", err)
			}
			markers, err := b.ListViewTruncations(ctx, sid)
			if err != nil || len(markers) != 0 {
				t.Fatalf("failed edit retained marker: %v", err)
			}
			if err := b.SetRunStatus(ctx, run, domain.RunCompleted); err != nil {
				t.Fatal(err)
			}
			if _, err := admissionStore.CommitRunAdmission(ctx, admission); err != nil {
				t.Fatal(err)
			}
		} else if path == "direct-edit" {
			if err := b.RecordSessionTruncation(ctx, marker); err != nil {
				t.Fatal(err)
			}
		} else {
			history := b.(storage.HistoryMutationStore)
			if path == "rewind-transaction" {
				marker.Reason = storage.TruncationRewind
				event.Type = domain.EventSessionTruncated
				if _, err := history.CommitSessionRewind(ctx, marker, event); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := b.SetRunStatus(ctx, run, domain.RunCompleted); err != nil {
					t.Fatal(err)
				}
				if _, err := history.CommitSessionEdit(ctx, marker, message, editedRun, event); err != nil {
					t.Fatal(err)
				}
			}
		}
		if _, err := store.ReadDeliverablePresentation(ctx, sid, item); !errors.Is(err, storage.ErrNotFound) {
			t.Fatalf("%s retained hidden bytes: %v", path, err)
		}
	}
}
