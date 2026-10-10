package live

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"agent-vivy/sdk/tui/surface"
	"testing"
)

func TestLiveDequeuedTurnResendsCapturedOptionsAfterTextEdit(t *testing.T) {
	for _, method := range []string{"turn/start", "turn/steer", "turn/follow_up"} {
		busy := method != "turn/start"
		name := method
		t.Run(name, func(t *testing.T) {
			turn := map[string]any{
				"id": "q1", "session_id": "sess_1", "track": "follow_up", "text": "recall me",
				"mode": "normal", "thinking": "high", "face": "code", "policy_profile": "full_auto",
				"collaboration_mode": "plan", "collaboration_version": 1,
				"attachments":   []any{map[string]any{"name": "captured.png", "mime_type": "image/png", "data": "aW1hZ2U="}},
				"file_contexts": []any{map[string]any{"path": "source.go", "name": "source.go", "size": 8, "content": "c25hcHNob3Q="}},
				"continuity":    map[string]any{"request_id": "original-request", "history_scope": map[string]any{"workspace": true}},
			}
			env := &fakeEnv{script: map[string]func(json.RawMessage) (any, error){}}
			env.script["queue/dequeue"] = func(json.RawMessage) (any, error) {
				return map[string]any{"dequeued": true, "text": "recall me", "turn": turn}, nil
			}
			env.script["queue/state"] = func(json.RawMessage) (any, error) {
				return map[string]any{"steering": []any{}, "follow_up": []any{}}, nil
			}
			var sent map[string]any

			env.script[method] = func(raw json.RawMessage) (any, error) {
				if err := json.Unmarshal(raw, &sent); err != nil {
					t.Fatal(err)
				}
				if busy {
					return map[string]any{"queued": true, "queue_id": "q2", "track": "follow_up"}, nil
				}
				return map[string]any{"run_id": "r2", "status": "accepted"}, nil
			}
			live := newLive(context.Background(), newClient(env), Options{})
			defer live.Close()
			live.activeID = "sess_1"
			recalled := live.Dequeue()().(liveQueueMsg)
			restore := live.applyQueueMsg(recalled)
			if restore == nil {
				t.Fatal("no editor restore")
			}
			_ = restore()
			if live.ThinkingMode() != "high" {
				t.Fatalf("restored thinking = %q", live.ThinkingMode())
			}
			if got := live.PendingAttachments(); len(got) != 1 || got[0].Name != "captured.png" {
				t.Fatalf("restored attachment metadata = %+v", got)
			}
			live.busy = busy
			send := live.Send
			if method == "turn/follow_up" {
				send = live.SendFollowUp
			}
			cmd := send("edited recall")
			if cmd == nil {
				t.Fatal("no edited resend")
			}
			_ = live.Handle(cmd())
			if sent["text"] != "edited recall" {
				t.Fatalf("resend text = %v", sent["text"])
			}
			for _, field := range []string{"thinking", "face", "mode", "policy_profile", "collaboration_mode", "collaboration_version", "attachments", "file_contexts"} {
				want, _ := json.Marshal(turn[field])
				got, _ := json.Marshal(sent[field])
				if string(got) != string(want) {
					t.Errorf("resend %s = %s, want captured %s", field, got, want)
				}
			}
			if sent["request_id"] != "original-request" {
				t.Errorf("resend request_id = %v", sent["request_id"])
			}
			if env.saw("project-context/resolve") {
				t.Fatal("restored captured context was re-read from filesystem")
			}
		})
	}
}

func decodedRestoreTurn(t *testing.T, id, thinking string) queuedTurnView {
	t.Helper()
	var turn queuedTurnView
	raw := []byte(`{"id":"` + id + `","session_id":"sess_1","text":"` + id + `","mode":"plan","thinking":"` + thinking + `","face":"web","attachments":[{"name":"capture.png","mime_type":"image/png","data":"aW1hZ2U="}],"file_contexts":[{"path":"a.go","name":"a.go","size":8,"content":"c25hcHNob3Q="}],"continuity":{"request_id":"original"}}`)
	if err := json.Unmarshal(raw, &turn); err != nil {
		t.Fatal(err)
	}
	return turn
}

func TestLiveRecallPreflightsAndPinsDurableQueueID(t *testing.T) {
	for _, supported := range []bool{false, true} {
		t.Run(fmt.Sprint(supported), func(t *testing.T) {
			turn := decodedRestoreTurn(t, "q1", "high")
			if !supported {
				turn.Thinking = "future-effort"
			}
			env := &fakeEnv{script: map[string]func(json.RawMessage) (any, error){}}
			env.script["queue/state"] = func(json.RawMessage) (any, error) { return queueStateView{FollowUps: []queuedTurnView{turn}}, nil }
			env.script["queue/dequeue"] = func(raw json.RawMessage) (any, error) {
				var params map[string]any
				_ = json.Unmarshal(raw, &params)
				if params["queue_id"] != "q1" {
					t.Fatalf("unconditional dequeue: %s", raw)
				}
				// A newer unsupported turn raced the recall: leave both durable.
				return map[string]any{"dequeued": false}, nil
			}
			live := newLive(context.Background(), newClient(env), Options{})
			defer live.Close()
			live.activeID = "sess_1"
			msg := live.Dequeue()().(liveQueueMsg)
			if supported && msg.Err != nil {
				t.Fatal(msg.Err)
			}
			if !supported && (msg.Err == nil || env.saw("queue/dequeue")) {
				t.Fatal("unsupported turn consumed")
			}
			if msg.RestoreTurn != nil || msg.RestoreText != "" {
				t.Fatal("CAS mismatch restored unconsumed work")
			}
		})
	}
}

func TestLiveAbortRecallKeepsSeparateCompleteTurnsAndRejectsDraftMerge(t *testing.T) {
	env := &fakeEnv{script: map[string]func(json.RawMessage) (any, error){}}
	live := newLive(context.Background(), newClient(env), Options{})
	defer live.Close()
	live.activeID = "sess_1"
	for i, effort := range []string{"low", "high"} {
		turn := decodedRestoreTurn(t, fmt.Sprintf("q%d", i), effort)
		raw, _ := json.Marshal(turn.Payload)
		live.applyNotice(eventNotice{Kind: "queue_dequeued", QueueReason: "aborted", Message: turn.Text, QueueTurn: raw})
	}
	if len(live.returnedTurns["sess_1"]) != 2 {
		t.Fatal("returned DTOs merged")
	}
	if live.restoreDraftCmd() != nil {
		t.Fatal("returned turn automatically overwrote editor")
	}
	recalled := live.Dequeue()().(liveQueueMsg)
	cmd := live.applyQueueMsg(recalled)
	if !cmd().(surface.RestoreInputMsg).Recall || live.ThinkingMode() != "high" {
		t.Fatal("full recall missing")
	}
	if files := live.PendingFileContexts(); len(files) != 1 || files[0].Name != "a.go" {
		t.Fatalf("file metadata = %+v", files)
	}
	raw, _ := json.Marshal(live.PendingAttachments())
	if strings.Contains(string(raw), "aW1hZ2U=") {
		t.Fatal("raw image bytes leaked to surface")
	}
	live.Handle(surface.RecallRejectedMsg{})
	if len(live.returnedTurns["sess_1"]) != 2 || live.editorTurns["sess_1"] != nil {
		t.Fatal("in-flight user draft rejection lost recalled DTO")
	}
	if env.saw("turn/start") || env.saw("turn/steer") {
		t.Fatal("recovery automatically submitted")
	}
}

func TestLiveRestoredResendFailureRetainsCapturedBytesAndEditableOptions(t *testing.T) {
	for _, busy := range []bool{false, true} {
		t.Run(fmt.Sprint(busy), func(t *testing.T) {
			env := &fakeEnv{script: map[string]func(json.RawMessage) (any, error){}}
			turn := decodedRestoreTurn(t, "q1", "high")
			method := "turn/start"
			if busy {
				method = "turn/steer"
			}
			env.script[method] = func(json.RawMessage) (any, error) { return nil, errors.New("temporary rejection") }
			live := newLive(context.Background(), newClient(env), Options{})
			defer live.Close()
			live.activeID = "sess_1"
			_ = live.applyQueueMsg(liveQueueMsg{SessionID: "sess_1", RestoreText: turn.Text, RestoreTurn: &turn})
			live.busy = busy
			msg := live.Send("edited")()
			_ = live.Handle(msg)
			if live.editorTurns["sess_1"] == nil || len(live.PendingAttachments()) != 1 {
				t.Fatal("rejection discarded editor snapshot")
			}
			if err := live.SetRunMode("normal"); err != nil {
				t.Fatal(err)
			}
			live.executeImageCommand([]string{"clear"})
			var sent map[string]any
			env.script[method] = func(raw json.RawMessage) (any, error) {
				_ = json.Unmarshal(raw, &sent)
				return map[string]any{"run_id": "r2", "queued": busy}, nil
			}
			live.busy = busy
			_ = live.Send("retry")()
			if sent["mode"] != "normal" || sent["attachments"] != nil || sent["request_id"] != "original" || sent["file_contexts"] == nil {
				t.Fatalf("retry lost edits or captured state: %+v", sent)
			}
		})
	}
}

func TestLiveFailedQueuedRecallRetainsUserPreferenceAndAddedImageEdits(t *testing.T) {
	env := &fakeEnv{script: map[string]func(json.RawMessage) (any, error){"turn/steer": func(json.RawMessage) (any, error) { return nil, errors.New("temporary") }}}
	live := newLive(context.Background(), newClient(env), Options{})
	defer live.Close()
	live.activeID = "sess_1"
	turn := decodedRestoreTurn(t, "q1", "high")
	_ = live.applyQueueMsg(liveQueueMsg{SessionID: "sess_1", RestoreText: turn.Text, RestoreTurn: &turn})
	_ = live.SetRunMode("normal")
	live.drafts["sess_1"] = append(live.drafts["sess_1"], surface.Attachment{Path: "new.png", Name: "new.png"})
	live.busy = true
	_ = live.Handle(live.Send("edited")())
	if live.RunMode() != "normal" || len(live.PendingAttachments()) != 2 || live.PendingAttachments()[1].Path != "new.png" {
		t.Fatal("queued rejection discarded edits")
	}
}
