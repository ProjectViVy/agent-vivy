package rpc

import (
	"agent-vivy/internal/domain"
	bus "agent-vivy/internal/events"
	"agent-vivy/internal/storage"
	stream "agent-vivy/sdk/tui/stream"
	"context"
	"encoding/json"
	"net"
	"strings"
	"testing"
	"time"
)

func TestRunSubscriptionProtocolFailure(t *testing.T) {
	for _, phase := range []string{"initial", "live", "catch-up"} {
		t.Run(phase, func(t *testing.T) {
			env := newControlTestEnv(t)
			h := env.handler.(*controlHandler)
			h.deps.Bus = bus.NewBus(1)
			ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
			defer cancel()
			if err := env.backend.CreateRun(ctx, domain.Run{ID: "r-bad", SessionID: "s-bad", Status: domain.RunActive, CreatedAt: 1}); err != nil {
				t.Fatal(err)
			}
			appendEvent := func(kind domain.EventType, version int, payload string, publish bool) {
				t.Helper()
				event := domain.RunEvent{RunID: "r-bad", Type: kind, PayloadVersion: version, Payload: json.RawMessage(payload)}
				seq, err := env.backend.Append(ctx, storage.Commit{RunID: "r-bad", Events: []domain.RunEvent{event}})
				if err != nil {
					t.Fatal(err)
				}
				if publish {
					event.Seq = seq
					h.deps.Bus.Publish(event)
				}
			}
			if phase == "initial" {
				appendEvent(domain.EventModelCompleted, 3, `{"byte_len":null}`, false)
			}
			left, right := net.Pipe()
			defer right.Close()
			peer := NewPeer(NewJSONLTransport(left, left, left.Close), nil, Options{OutgoingBuffer: 1})
			defer peer.Close()
			params, _ := json.Marshal(map[string]any{"run_id": "r-bad", "after_seq": 0})
			request := Request{JSONRPC: "2.0", ID: json.RawMessage(`"bad"`), Method: "run/subscribe", Params: params}
			if _, err := h.subscribe(ctx, peer, request); err != nil {
				t.Fatal(err)
			}
			peer.runAfterResponse(request.ID)
			if phase != "initial" {
				for h.deps.Bus.Subscribers("r-bad") == 0 {
					select {
					case <-ctx.Done():
						t.Fatal(ctx.Err())
					default:
						time.Sleep(time.Millisecond)
					}
				}
				if phase == "catch-up" {
					for i := 0; i < 8; i++ {
						appendEvent(domain.EventWorkflowNodeStarted, 1, `{}`, true)
					}
				}
				appendEvent(domain.EventModelCompleted, 3, `{"byte_len":null}`, phase == "live")
			}
			for {
				select {
				case raw := <-peer.out:
					var notice struct {
						Method string `json:"method"`
						Params struct {
							Retryable *bool  `json:"retryable"`
							Message   string `json:"message"`
						} `json:"params"`
					}
					if err := json.Unmarshal(raw, &notice); err != nil {
						t.Fatal(err)
					}
					if notice.Method == "run/stream_error" {
						if notice.Params.Retryable == nil || *notice.Params.Retryable || !strings.Contains(notice.Params.Message, "byte_len") {
							t.Fatalf("failure not classified: %s", raw)
						}
						run, err := env.backend.GetRun(ctx, "r-bad")
						if err != nil || run.Status != domain.RunActive {
							t.Fatalf("backend run changed: %+v %v", run, err)
						}
						return
					}
				case <-ctx.Done():
					t.Fatal("missing protocol failure notification")
				}
			}
		})
	}
}

func TestLegacyCompletionOnlyReadsPersistedMessages(t *testing.T) {
	env := newControlTestEnv(t)
	ctx := context.Background()
	if err := env.backend.CreateSession(ctx, domain.Session{ID: "s-v1", Title: "legacy", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := env.backend.CreateRun(ctx, domain.Run{ID: "r-v1", SessionID: "s-v1", Status: domain.RunCompleted, CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := env.backend.AppendMessage(ctx, domain.Message{ID: "old-message", SessionID: "s-v1", RunID: "r-v1", Role: domain.RoleAssistant, Content: "persisted legacy answer", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := env.backend.Append(ctx, storage.Commit{RunID: "r-v1", Events: []domain.RunEvent{{Type: domain.EventModelCompleted, PayloadVersion: 1, Payload: json.RawMessage(`{"content":"must not reconstruct"}`)}}}); err != nil {
		t.Fatal(err)
	}
	result, rpcErr := callControl(t, env.handler, "session/messages", map[string]string{"session_id": "s-v1"})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	raw, _ := json.Marshal(result)
	if !strings.Contains(string(raw), "persisted legacy answer") || strings.Contains(string(raw), "must not reconstruct") {
		t.Fatalf("legacy read: %s", raw)
	}
	if _, err := env.handler.(*controlHandler).replayEvents(ctx, "r-v1", 0); err == nil || !strings.Contains(err.Error(), "unsupported model.completed") {
		t.Fatalf("legacy replay: %v", err)
	}
}

func TestReplayNormalizesHistoricalCompletion(t *testing.T) {
	env := newControlTestEnv(t)
	ctx := context.Background()
	if err := env.backend.CreateSession(ctx, domain.Session{ID: "s-v2", Title: "history", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := env.backend.CreateRun(ctx, domain.Run{ID: "r-v2", SessionID: "s-v2", Status: domain.RunCompleted, CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	raw := json.RawMessage(`{"byte_len":3,"content_sha256":"wrong"}`)
	if _, err := env.backend.Append(ctx, storage.Commit{RunID: "r-v2", Events: []domain.RunEvent{
		{Type: domain.EventModelDelta, PayloadVersion: 1, Payload: json.RawMessage(`{"delta":"界"}`), CreatedAt: 2},
		{Type: domain.EventModelCompleted, PayloadVersion: 2, Payload: raw, CreatedAt: 3},
	}}); err != nil {
		t.Fatal(err)
	}
	got, err := env.handler.(*controlHandler).replayEvents(ctx, "r-v2", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[1].PayloadVersion != 3 || string(got[1].Payload) != `{"byte_len":3}` || got[1].Seq != 2 || got[1].CreatedAt != 3 {
		t.Fatalf("normalized: %+v", got)
	}
	it, err := env.backend.Replay(ctx, "r-v2", 1)
	if err != nil {
		t.Fatal(err)
	}
	defer it.Close()
	if !it.Next() || string(it.Value().Event.Payload) != string(raw) || it.Value().Event.PayloadVersion != 2 {
		t.Fatal("historical Journal changed")
	}
}

func TestSubscriptionNormalizesHistoricalCompletion(t *testing.T) {
	env := newControlTestEnv(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := env.backend.CreateRun(ctx, domain.Run{ID: "r-old", SessionID: "s-old", Status: domain.RunCompleted, CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := env.backend.Append(ctx, storage.Commit{RunID: "r-old", Events: []domain.RunEvent{
		{Type: domain.EventModelDelta, PayloadVersion: 1, Payload: json.RawMessage(`{"delta":"界"}`)},
		{Type: domain.EventModelCompleted, PayloadVersion: 2, Payload: json.RawMessage(`{"byte_len":3}`)},
		{Type: domain.EventRunCompleted, PayloadVersion: 1, Payload: json.RawMessage(`{}`)},
	}}); err != nil {
		t.Fatal(err)
	}
	left, right := net.Pipe()
	defer right.Close()
	peer := NewPeer(NewJSONLTransport(left, left, left.Close), nil, Options{OutgoingBuffer: 8})
	defer peer.Close()
	params, _ := json.Marshal(map[string]any{"run_id": "r-old", "after_seq": 0})
	request := Request{JSONRPC: "2.0", ID: json.RawMessage(`"history"`), Method: "run/subscribe", Params: params}
	h := env.handler.(*controlHandler)
	if _, err := h.subscribe(ctx, peer, request); err != nil {
		t.Fatal(err)
	}
	peer.runAfterResponse(request.ID)
	var tracker stream.ModelCompletionTracker
	for seq := 1; seq <= 3; seq++ {
		select {
		case raw := <-peer.out:
			var notice struct {
				Method string          `json:"method"`
				Params json.RawMessage `json:"params"`
			}
			if err := json.Unmarshal(raw, &notice); err != nil {
				t.Fatal(err)
			}
			event, ok := stream.Decode(notice.Params)
			if !ok || notice.Method != "run/event" || event.Seq != seq {
				t.Fatalf("notification: %s", raw)
			}
			if err := tracker.ObserveEvent(event); err != nil {
				t.Fatal(err)
			}
			if event.Type == "model.completed" && event.PayloadVersion != 3 {
				t.Fatal("old live completion escaped")
			}
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	waitForControlSubscriptionCount(t, h, 0)
}
