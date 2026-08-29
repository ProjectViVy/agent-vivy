package app

// M4 real-provider smoke suite. Env-gated: it runs only when
// VIVY_REAL_SMOKE=1 and OPENAI_API_KEY are both set. The suite drives the
// composed app over its local JSON-RPC/WebSocket control plane and asserts
// the durable acceptance anchors AS-1, AS-5 and AS-7.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"agent-vivy/internal/config"
	"agent-vivy/internal/runtime"
)

const realSmokeTimeout = 3 * time.Minute
const pinnedEinoVersion = "v0.9.13"

type rpcSmokeClient struct {
	conn *websocket.Conn
	next int
}

type rpcSmokeMessage struct {
	ID     string          `json:"id,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
}

type smokeEvent struct {
	RunID string `json:"run_id"`
	Seq   int64  `json:"seq"`
	Type  string `json:"type"`
}

func TestRealProviderSmoke(t *testing.T) {
	if os.Getenv("VIVY_REAL_SMOKE") != "1" {
		t.Skip("set VIVY_REAL_SMOKE=1 (and OPENAI_API_KEY) to run the real provider smoke")
	}
	if _, keySet := os.LookupEnv("OPENAI_API_KEY"); !keySet {
		t.Skip("OPENAI_API_KEY is not set; the real provider smoke needs a live key")
	}
	modelID := os.Getenv("VIVY_REAL_MODEL")
	if modelID == "" {
		modelID = "step-3.7-flash"
	}
	runtime.SetEngineVersionOverride(pinnedEinoVersion)
	t.Cleanup(func() { runtime.SetEngineVersionOverride("") })

	ctx := context.Background()
	cfg := config.Config{
		Server:  config.Server{Addr: "127.0.0.1:0"},
		Storage: config.Storage{Backend: "sqlite", SQLite: config.SQLite{Path: filepath.Join(t.TempDir(), "smoke.db")}},
		Providers: config.Providers{
			Active: "openai", BundleDir: filepath.Join("..", "..", "fixtures", "provider"),
			OpenAI:    config.Provider{EnvKey: "OPENAI_API_KEY", DefaultModel: modelID},
			Anthropic: config.Provider{EnvKey: "ANTHROPIC_API_KEY", DefaultModel: "claude-sonnet-4-5"},
		},
		Runtime: config.Runtime{StreamBuffer: 256, MaxEventPayloadBytes: 65536},
		Tools:   config.Tools{Enabled: []string{"echo_info", "write_note"}, Approval: config.Approval{Expiration: 5 * time.Minute}},
	}
	a, err := New(ctx, cfg)
	if err != nil {
		t.Fatalf("compose app on the real provider: %v", err)
	}
	ts := httptest.NewServer(a.httpServer.Handler)
	t.Cleanup(func() {
		ts.Close()
		a.service.CancelAll()
		_ = a.backend.Close()
	})

	client := connectSmokeRPC(t, ts.URL, a.rpcToken)
	t.Cleanup(func() { _ = client.conn.Close() })
	callSmoke(t, client, "initialize", map[string]any{"protocol_version": "vivy.rpc.v1"})
	session := callSmoke(t, client, "session/create", map[string]any{"title": "real smoke"})
	var sess struct {
		ID string `json:"id"`
	}
	decodeSmoke(t, session, &sess)
	if sess.ID == "" {
		t.Fatal("session/create returned no id")
	}
	started := callSmoke(t, client, "turn/start", map[string]any{"session_id": sess.ID, "text": "Answer in one short sentence: what is the capital of France?"})
	var accepted struct {
		RunID string `json:"run_id"`
	}
	decodeSmoke(t, started, &accepted)
	frames := subscribeAndRead(t, client, accepted.RunID, 0, nil)
	if frames[len(frames)-1].Type != "run.completed" {
		t.Fatalf("AS-1: terminal event = %s, want run.completed", frames[len(frames)-1].Type)
	}
	for i, frame := range frames {
		if frame.Seq != int64(i+1) {
			t.Fatalf("AS-1: frame %d seq = %d, want %d", i, frame.Seq, i+1)
		}
	}
	messages := callSmoke(t, client, "session/messages", map[string]any{"session_id": sess.ID})
	var messageList struct {
		Messages []map[string]any `json:"messages"`
	}
	decodeSmoke(t, messages, &messageList)
	if len(messageList.Messages) < 2 || strings.TrimSpace(fmt.Sprint(messageList.Messages[len(messageList.Messages)-1]["content"])) == "" {
		t.Fatalf("AS-1: persisted messages = %+v", messageList.Messages)
	}

	tail := callSmoke(t, client, "run/log", map[string]any{"run_id": accepted.RunID, "after_seq": 1})
	var tailList struct {
		Events []smokeEvent `json:"events"`
	}
	decodeSmoke(t, tail, &tailList)
	if len(tailList.Events) != len(frames)-1 {
		t.Fatalf("AS-7: replay tail = %d, want %d", len(tailList.Events), len(frames)-1)
	}

	cancelled := callSmoke(t, client, "turn/start", map[string]any{"session_id": sess.ID, "text": "Write a very long, detailed essay about the history of computing. Do not stop early."})
	var cancelAccepted struct {
		RunID string `json:"run_id"`
	}
	decodeSmoke(t, cancelled, &cancelAccepted)
	var once sync.Once
	cancelFrames := subscribeAndRead(t, client, cancelAccepted.RunID, 0, func(event smokeEvent) {
		if event.Type == "model.delta" {
			once.Do(func() { sendSmoke(t, client, "run/cancel", map[string]any{"run_id": cancelAccepted.RunID}) })
		}
	})
	if cancelFrames[len(cancelFrames)-1].Type != "run.cancelled" {
		t.Fatalf("AS-5: terminal event = %s, want run.cancelled", cancelFrames[len(cancelFrames)-1].Type)
	}
	t.Logf("real RPC smoke passed with %d completion events, model %s", len(frames), modelID)
}

func connectSmokeRPC(t *testing.T, base, token string) *rpcSmokeClient {
	t.Helper()
	parsed, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	parsed.Scheme = "ws"
	parsed.Path = "/rpc"
	parsed.RawQuery = url.Values{"token": []string{token}}.Encode()
	conn, _, err := websocket.DefaultDialer.Dial(parsed.String(), nil)
	if err != nil {
		t.Fatalf("dial RPC: %v", err)
	}
	return &rpcSmokeClient{conn: conn}
}

func callSmoke(t *testing.T, client *rpcSmokeClient, method string, params any) json.RawMessage {
	t.Helper()
	id := fmt.Sprintf("%d", client.next+1)
	client.next++
	sendSmokeID(t, client, id, method, params)
	for {
		var message rpcSmokeMessage
		if err := client.conn.ReadJSON(&message); err != nil {
			t.Fatalf("read RPC %s: %v", method, err)
		}
		if message.ID != id {
			continue
		}
		if message.Error != nil {
			t.Fatalf("RPC %s failed: %d %s", method, message.Error.Code, message.Error.Message)
		}
		return message.Result
	}
}

func sendSmoke(t *testing.T, client *rpcSmokeClient, method string, params any) {
	t.Helper()
	client.next++
	sendSmokeID(t, client, fmt.Sprintf("%d", client.next), method, params)
}

func sendSmokeID(t *testing.T, client *rpcSmokeClient, id, method string, params any) {
	t.Helper()
	if err := client.conn.WriteJSON(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}); err != nil {
		t.Fatalf("send RPC %s: %v", method, err)
	}
}

func subscribeAndRead(t *testing.T, client *rpcSmokeClient, runID string, after int64, onEvent func(smokeEvent)) []smokeEvent {
	t.Helper()
	_ = callSmoke(t, client, "run/subscribe", map[string]any{"run_id": runID, "after_seq": after})
	ctx, cancel := context.WithTimeout(context.Background(), realSmokeTimeout)
	defer cancel()
	_ = client.conn.SetReadDeadline(time.Now().Add(realSmokeTimeout))
	var frames []smokeEvent
	for {
		var message rpcSmokeMessage
		if err := client.conn.ReadJSON(&message); err != nil {
			t.Fatalf("read run/event: %v", err)
		}
		if message.Method != "run/event" {
			continue
		}
		var envelope struct {
			Event smokeEvent `json:"event"`
		}
		if err := json.Unmarshal(message.Params, &envelope); err != nil {
			t.Fatal(err)
		}
		if envelope.Event.RunID != runID {
			continue
		}
		frames = append(frames, envelope.Event)
		if onEvent != nil {
			onEvent(envelope.Event)
		}
		if envelope.Event.Type == "run.completed" || envelope.Event.Type == "run.failed" || envelope.Event.Type == "run.cancelled" {
			return frames
		}
		select {
		case <-ctx.Done():
			t.Fatalf("run %s did not reach terminal event", runID)
		default:
		}
	}
}

func decodeSmoke(t *testing.T, raw json.RawMessage, target any) {
	t.Helper()
	if err := json.Unmarshal(raw, target); err != nil {
		t.Fatalf("decode RPC result: %v", err)
	}
}
