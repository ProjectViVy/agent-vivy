package app

// M4 real-provider smoke suite. Env-gated: it runs only when
// VIVY_REAL_SMOKE=1 and OPENAI_API_KEY are both set, so ordinary gates and
// CI stay offline and deterministic. The suite drives the full composed
// app over HTTP + SSE against a real OpenAI-compatible gateway
// (VIVY_API_BASE override) and asserts the deterministic acceptance
// anchors AS-1, AS-5 and AS-7. Tool-approval scenarios (AS-2..AS-4) depend
// on the live model honoring prompt inducement and stay with the manual
// walkthrough (docs/real-provider-smoke.md).

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"agent-vivy/internal/config"
	"agent-vivy/internal/runtime"
)

// realSmokeTimeout bounds one SSE consumption; live gateways vary.
const realSmokeTimeout = 3 * time.Minute

// pinnedEinoVersion mirrors the go.mod dependency pin: go test binaries
// lack embedded module metadata, so the checkpoint anchor is set from the
// version this suite was built against.
const pinnedEinoVersion = "v0.9.13"

type sseFrame struct {
	Seq   int64
	Event string
	Data  json.RawMessage
}

func isTerminalEvent(name string) bool {
	return name == "run.completed" || name == "run.failed" || name == "run.cancelled"
}

func TestRealProviderSmoke(t *testing.T) {
	if os.Getenv("VIVY_REAL_SMOKE") != "1" {
		t.Skip("set VIVY_REAL_SMOKE=1 (and OPENAI_API_KEY) to run the real provider smoke")
	}
	// LookupEnv (not Getenv): the E3 source audit pins credential Getenv
	// reads to internal/provider, and here only presence matters — the
	// value is consumed solely by the provider at request time (D-010).
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
		Server: config.Server{Addr: "127.0.0.1:0"}, // httptest owns the listener
		Storage: config.Storage{
			Backend: "sqlite",
			SQLite:  config.SQLite{Path: filepath.Join(t.TempDir(), "smoke.db")},
		},
		Providers: config.Providers{
			Active:    "openai",
			BundleDir: filepath.Join("..", "..", "fixtures", "provider"),
			OpenAI:    config.Provider{EnvKey: "OPENAI_API_KEY", DefaultModel: modelID},
			Anthropic: config.Provider{EnvKey: "ANTHROPIC_API_KEY", DefaultModel: "claude-sonnet-4-5"},
		},
		Runtime: config.Runtime{Mock: false, StreamBuffer: 256, MaxEventPayloadBytes: 65536},
		Tools: config.Tools{
			Enabled:  []string{"echo_info", "write_note"},
			Approval: config.Approval{Expiration: 5 * time.Minute},
		},
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

	// AS-1: session -> message -> streamed completion -> persisted log.
	sessID := createSession(t, ts.URL)
	runID := postMessage(t, ts.URL, sessID, "Answer in one short sentence: what is the capital of France?")
	frames := streamRunFrames(t, ts.URL, runID, nil)

	last := frames[len(frames)-1]
	if last.Event != "run.completed" {
		t.Fatalf("AS-1: terminal event = %s, want run.completed", last.Event)
	}
	for i, f := range frames {
		if f.Seq != int64(i+1) {
			t.Fatalf("AS-1: frame %d has seq %d, want %d (gaps break AS-7)", i, f.Seq, i+1)
		}
	}
	msgs := listMessages(t, ts.URL, sessID)
	if len(msgs) < 2 {
		t.Fatalf("AS-1: messages = %d, want user + assistant", len(msgs))
	}
	first, ok := msgs[0].(map[string]any)
	if !ok || first["role"] != "user" {
		t.Fatalf("AS-1: first message = %v, want the user turn", msgs[0])
	}
	reply, ok := msgs[len(msgs)-1].(map[string]any)
	if !ok || reply["role"] != "assistant" {
		t.Fatalf("AS-1: last message = %v, want the assistant turn", msgs[len(msgs)-1])
	}
	if strings.TrimSpace(fmt.Sprint(reply["content"])) == "" {
		t.Fatal("AS-1: assistant reply is empty")
	}
	t.Logf("AS-1: run completed with %d events, model %s", len(frames), modelID)

	// AS-7: reconnect with after_seq replays exactly the tail.
	tail := readSSEFrames(t, fmt.Sprintf("%s/api/runs/%s/events?after_seq=1", ts.URL, runID), nil)
	if len(tail) != len(frames)-1 {
		t.Fatalf("AS-7: reconnect frames = %d, want %d", len(tail), len(frames)-1)
	}
	for i, f := range tail {
		if f.Seq != frames[i+1].Seq || f.Event != frames[i+1].Event {
			t.Fatalf("AS-7: reconnect frame %d = %+v, want %+v", i, f, frames[i+1])
		}
	}

	// AS-5: cancelling mid-stream closes the run as cancelled with
	// exactly one terminal. The long-answer prompt keeps the stream open
	// long enough for the cancel to land between deltas.
	cancelRunID := postMessage(t, ts.URL, sessID,
		"Write a very long, detailed essay about the history of computing. Do not stop early.")
	var once sync.Once
	cancelFrames := streamRunFrames(t, ts.URL, cancelRunID, func(f sseFrame) {
		if f.Event == "model.delta" {
			once.Do(func() { cancelRun(t, ts.URL, cancelRunID) })
		}
	})
	terminals := 0
	for _, f := range cancelFrames {
		if isTerminalEvent(f.Event) {
			terminals++
		}
	}
	if terminals != 1 {
		t.Fatalf("AS-5: terminal events = %d, want exactly 1", terminals)
	}
	if last := cancelFrames[len(cancelFrames)-1]; last.Event != "run.cancelled" {
		t.Fatalf("AS-5: terminal event = %s, want run.cancelled", last.Event)
	}
}

// --- HTTP helpers ---

func doJSON(t *testing.T, method, url string, body any) map[string]any {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request: %v", err)
		}
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		t.Fatalf("%s %s: status %d", method, url, resp.StatusCode)
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("%s %s: decode response: %v", method, url, err)
	}
	return out
}

func createSession(t *testing.T, base string) string {
	t.Helper()
	out := doJSON(t, http.MethodPost, base+"/api/sessions", map[string]any{"title": "real smoke"})
	id, _ := out["id"].(string)
	if id == "" {
		t.Fatalf("create session returned no id: %v", out)
	}
	return id
}

func postMessage(t *testing.T, base, sessID, text string) string {
	t.Helper()
	out := doJSON(t, http.MethodPost, base+"/api/sessions/"+sessID+"/messages", map[string]any{"text": text})
	runID, _ := out["run_id"].(string)
	if runID == "" {
		t.Fatalf("post message returned no run_id: %v", out)
	}
	return runID
}

func cancelRun(t *testing.T, base, runID string) {
	t.Helper()
	doJSON(t, http.MethodPost, base+"/api/runs/"+runID+"/cancel", nil)
}

func listMessages(t *testing.T, base, sessID string) []any {
	t.Helper()
	out := doJSON(t, http.MethodGet, base+"/api/sessions/"+sessID+"/messages", nil)
	msgs, _ := out["messages"].([]any)
	return msgs
}

// --- SSE helpers ---

// streamRunFrames consumes one run's SSE stream until its terminal event,
// invoking onFrame (if non-nil) for every frame received.
func streamRunFrames(t *testing.T, base, runID string, onFrame func(sseFrame)) []sseFrame {
	t.Helper()
	url := fmt.Sprintf("%s/api/runs/%s/events", base, runID)
	return readSSEFrames(t, url, onFrame)
}

func readSSEFrames(t *testing.T, url string, onFrame func(sseFrame)) []sseFrame {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), realSmokeTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("new sse request: %v", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("open sse stream %s: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("sse stream %s: status %d", url, resp.StatusCode)
	}

	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 64<<10), 1<<20)
	var frames []sseFrame
	var cur sseFrame
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "id: "):
			cur.Seq, _ = strconv.ParseInt(strings.TrimPrefix(line, "id: "), 10, 64)
		case strings.HasPrefix(line, "event: "):
			cur.Event = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			cur.Data = json.RawMessage(strings.TrimPrefix(line, "data: "))
		case line == "":
			if cur.Event == "" {
				continue
			}
			frames = append(frames, cur)
			if onFrame != nil {
				onFrame(cur)
			}
			if isTerminalEvent(cur.Event) {
				return frames
			}
			cur = sseFrame{}
		}
	}
	if err := sc.Err(); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("scan sse stream %s: %v", url, err)
	}
	if len(frames) == 0 || !isTerminalEvent(frames[len(frames)-1].Event) {
		t.Fatalf("sse stream %s ended without a terminal event (%d frames)", url, len(frames))
	}
	return frames
}
