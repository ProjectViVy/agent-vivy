package httpapi

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/events"
	"agent-vivy/internal/provider"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/tools"
)

// testEnv wires the full D1 stack over a throwaway SQLite file.
type testEnv struct {
	backend *sqlite.Backend
	bus     *events.Bus
	svc     *runtime.Service
	handler http.Handler
}

func newTestEnv(t *testing.T, model domain.ChatModel) *testEnv {
	t.Helper()
	ctx := context.Background()

	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "vivy.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })

	ts, err := tools.Builtin().Resolve([]string{tools.EchoInfoName})
	if err != nil {
		t.Fatalf("resolve tools: %v", err)
	}
	eng, err := runtime.NewEngine(ctx, runtime.WrapModel(model), ts, runtime.EngineConfig{
		StreamBuffer:         64,
		MaxEventPayloadBytes: 64 << 10,
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	bus := events.NewBus(64)
	svc := runtime.NewService(eng, "mock", "mock", runtime.ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Sink: bus,
	})
	h, err := New(Deps{
		Sessions: backend, Messages: backend, Runs: backend,
		Journal: backend, Bus: bus, Service: svc,
	})
	if err != nil {
		t.Fatalf("httpapi.New: %v", err)
	}
	return &testEnv{backend: backend, bus: bus, svc: svc, handler: h}
}

// blockingModel parks until its context is cancelled (cancel-path tests).
type blockingModel struct{}

func (blockingModel) Stream(ctx context.Context, _ []*domain.Message) (domain.Stream[*domain.Message], error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

// --- JSON helpers ---

func doJSON(t *testing.T, h http.Handler, method, target string, body string) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, target, nil)
	} else {
		r = httptest.NewRequest(method, target, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func decodeBody(t *testing.T, w *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.Unmarshal(w.Body.Bytes(), v); err != nil {
		t.Fatalf("decode body %q: %v", w.Body.String(), err)
	}
}

// assertAPIError checks the FR-11 envelope shape and code.
func assertAPIError(t *testing.T, w *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if w.Code != status {
		t.Fatalf("status = %d, want %d (body %s)", w.Code, status, w.Body.String())
	}
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	decodeBody(t, w, &body)
	if body.Error.Code != code {
		t.Fatalf("error code = %q, want %q", body.Error.Code, code)
	}
	if body.Error.Message == "" {
		t.Fatal("error message must not be empty")
	}
}

func createSession(t *testing.T, h http.Handler, title string) string {
	t.Helper()
	body := ""
	if title != "" {
		body = fmt.Sprintf(`{"title":%q}`, title)
	}
	w := doJSON(t, h, http.MethodPost, "/api/sessions", body)
	if w.Code != http.StatusCreated {
		t.Fatalf("create session: status %d body %s", w.Code, w.Body.String())
	}
	var sess struct {
		ID string `json:"id"`
	}
	decodeBody(t, w, &sess)
	if !strings.HasPrefix(sess.ID, "sess_") {
		t.Fatalf("session id %q must start with sess_", sess.ID)
	}
	return sess.ID
}

func postMessage(t *testing.T, h http.Handler, sessionID, text string) *httptest.ResponseRecorder {
	t.Helper()
	return doJSON(t, h, http.MethodPost, "/api/sessions/"+sessionID+"/messages",
		fmt.Sprintf(`{"text":%q}`, text))
}

// waitForRunStatus polls GET /api/runs/{id} until status equals want.
func waitForRunStatus(t *testing.T, h http.Handler, runID, want string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		w := doJSON(t, h, http.MethodGet, "/api/runs/"+runID, "")
		if w.Code == http.StatusOK {
			var run struct {
				Status string `json:"status"`
			}
			decodeBody(t, w, &run)
			if run.Status == want {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("run %s never reached status %s", runID, want)
}

// --- SSE helpers ---

type sseFrame struct {
	ID      string
	Event   string
	Payload map[string]any
}

// readSSE streams the endpoint until the connection closes and returns the
// parsed frames.
func readSSE(t *testing.T, h http.Handler, target string) []sseFrame {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, target, nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("sse status = %d, body %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("content-type = %q, want text/event-stream", ct)
	}

	var frames []sseFrame
	var cur sseFrame
	sc := bufio.NewScanner(w.Body)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case line == "":
			if cur.ID != "" || cur.Event != "" {
				frames = append(frames, cur)
			}
			cur = sseFrame{}
		case strings.HasPrefix(line, "id: "):
			cur.ID = strings.TrimPrefix(line, "id: ")
		case strings.HasPrefix(line, "event: "):
			cur.Event = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &cur.Payload); err != nil {
				t.Fatalf("decode sse data %q: %v", line, err)
			}
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan sse: %v", err)
	}
	return frames
}

// assertEnvelope checks the A3 envelope fields of one frame.
func assertEnvelope(t *testing.T, f sseFrame, runID string, seq int, evType string) {
	t.Helper()
	if f.ID != strconv.Itoa(seq) {
		t.Fatalf("frame id = %q, want %d", f.ID, seq)
	}
	if f.Event != evType {
		t.Fatalf("frame event = %q, want %q", f.Event, evType)
	}
	if got, _ := f.Payload["run_id"].(string); got != runID {
		t.Fatalf("envelope run_id = %q, want %q", got, runID)
	}
	if got, _ := f.Payload["seq"].(float64); int(got) != seq {
		t.Fatalf("envelope seq = %v, want %d", f.Payload["seq"], seq)
	}
	if got, _ := f.Payload["type"].(string); got != evType {
		t.Fatalf("envelope type = %q, want %q", got, evType)
	}
	if _, ok := f.Payload["payload"]; !ok {
		t.Fatal("envelope must carry a payload object")
	}
	if got, _ := f.Payload["payload_version"].(float64); int(got) != 1 {
		t.Fatalf("payload_version = %v, want 1", f.Payload["payload_version"])
	}
}

// --- tests ---

// TestFullFlowSessionMessageSSE drives the M1 vertical slice: session ->
// message -> run -> persisted events -> SSE replay (FR-1, FR-5, AS-7).
func TestFullFlowSessionMessageSSE(t *testing.T) {
	env := newTestEnv(t, provider.NewMock())
	h := env.handler

	sessID := createSession(t, h, "hello session")
	w := postMessage(t, h, sessID, "hello vivy")
	if w.Code != http.StatusAccepted {
		t.Fatalf("post message: status %d body %s", w.Code, w.Body.String())
	}
	var accepted struct {
		RunID  string `json:"run_id"`
		Status string `json:"status"`
	}
	decodeBody(t, w, &accepted)
	if accepted.Status != "accepted" || !strings.HasPrefix(accepted.RunID, "run_") {
		t.Fatalf("accepted body = %+v", accepted)
	}
	waitForRunStatus(t, h, accepted.RunID, "completed")

	frames := readSSE(t, h, "/api/runs/"+accepted.RunID+"/events")
	if len(frames) < 4 {
		t.Fatalf("frames = %d, want at least 4", len(frames))
	}
	assertEnvelope(t, frames[0], accepted.RunID, 1, "run.started")
	startedPayload, _ := frames[0].Payload["payload"].(map[string]any)
	if startedPayload["provider"] != "mock" || startedPayload["model"] != "mock" {
		t.Fatalf("run.started payload = %v", startedPayload)
	}
	for i := 1; i < len(frames)-1; i++ {
		if frames[i].Event != "model.delta" && frames[i].Event != "model.completed" {
			t.Fatalf("unexpected mid-run frame %q", frames[i].Event)
		}
		assertEnvelope(t, frames[i], accepted.RunID, i+1, frames[i].Event)
	}
	assertEnvelope(t, frames[len(frames)-1], accepted.RunID, len(frames), "run.completed")

	// after_seq reconnect: only the tail, no duplicates, no gaps.
	tail := readSSE(t, h, fmt.Sprintf("/api/runs/%s/events?after_seq=2", accepted.RunID))
	if len(tail) != len(frames)-2 {
		t.Fatalf("reconnect frames = %d, want %d", len(tail), len(frames)-2)
	}
	for i, f := range tail {
		assertEnvelope(t, f, accepted.RunID, i+3, frames[i+2].Event)
	}

	// The conversation log carries the user turn and the assistant reply.
	w = doJSON(t, h, http.MethodGet, "/api/sessions/"+sessID+"/messages", "")
	if w.Code != http.StatusOK {
		t.Fatalf("list messages: status %d", w.Code)
	}
	var msgs struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	decodeBody(t, w, &msgs)
	if len(msgs.Messages) != 2 ||
		msgs.Messages[0].Role != "user" || msgs.Messages[0].Content != "hello vivy" ||
		msgs.Messages[1].Role != "assistant" || msgs.Messages[1].Content != "mock reply to: hello vivy" {
		t.Fatalf("messages = %+v", msgs.Messages)
	}
}

// TestCancelEndpoint drives the cancel endpoint against a parked run and
// asserts the run closes as cancelled.
func TestCancelEndpoint(t *testing.T) {
	env := newTestEnv(t, blockingModel{})
	h := env.handler

	sessID := createSession(t, h, "")
	w := postMessage(t, h, sessID, "never finishes")
	if w.Code != http.StatusAccepted {
		t.Fatalf("post message: status %d body %s", w.Code, w.Body.String())
	}
	var accepted struct {
		RunID string `json:"run_id"`
	}
	decodeBody(t, w, &accepted)
	waitForRunStatus(t, h, accepted.RunID, "active")

	w = doJSON(t, h, http.MethodPost, "/api/runs/"+accepted.RunID+"/cancel", "")
	if w.Code != http.StatusAccepted {
		t.Fatalf("cancel: status %d body %s", w.Code, w.Body.String())
	}
	waitForRunStatus(t, h, accepted.RunID, "cancelled")

	// Cancelling an already-terminal run is a conflict.
	assertAPIError(t, doJSON(t, h, http.MethodPost, "/api/runs/"+accepted.RunID+"/cancel", ""),
		http.StatusConflict, codeConflict)
	// The terminal close is visible over SSE replay.
	frames := readSSE(t, h, "/api/runs/"+accepted.RunID+"/events")
	if last := frames[len(frames)-1]; last.Event != "run.cancelled" {
		t.Fatalf("last frame = %q, want run.cancelled", last.Event)
	}
}

// TestErrorShapes pins the FR-11 error envelope across the endpoints.
func TestErrorShapes(t *testing.T) {
	env := newTestEnv(t, provider.NewMock())
	h := env.handler

	assertAPIError(t, doJSON(t, h, http.MethodGet, "/api/sessions/sess-nope", ""),
		http.StatusNotFound, codeNotFound)
	assertAPIError(t, doJSON(t, h, http.MethodGet, "/api/runs/run-nope", ""),
		http.StatusNotFound, codeNotFound)
	assertAPIError(t, doJSON(t, h, http.MethodPost, "/api/runs/run-nope/cancel", ""),
		http.StatusNotFound, codeNotFound)
	assertAPIError(t, doJSON(t, h, http.MethodGet, "/api/runs/run-nope/events", ""),
		http.StatusNotFound, codeNotFound)

	sessID := createSession(t, h, "")
	assertAPIError(t, postMessage(t, h, sessID, "   "),
		http.StatusBadRequest, codeInvalidRequest)
	assertAPIError(t, postMessage(t, h, "sess-nope", "hi"),
		http.StatusNotFound, codeNotFound)
	assertAPIError(t, doJSON(t, h, http.MethodGet, "/api/runs/"+
		createRun(t, env, sessID)+"/events?after_seq=abc", ""),
		http.StatusBadRequest, codeInvalidRequest)
	assertAPIError(t, doJSON(t, h, http.MethodGet, "/api/runs/"+
		createRun(t, env, sessID)+"/events?after_seq=-1", ""),
		http.StatusBadRequest, codeInvalidRequest)
}

// createRun submits one message directly through the service so the error
// tests can address a real run row.
func createRun(t *testing.T, env *testEnv, sessID string) string {
	t.Helper()
	runID, err := env.svc.Run(context.Background(), domain.SessionID(sessID), "ping")
	if err != nil {
		t.Fatalf("svc.Run: %v", err)
	}
	return string(runID)
}

// TestSessionCRUD walks the session endpoints incl. cascade delete.
func TestSessionCRUD(t *testing.T) {
	env := newTestEnv(t, provider.NewMock())
	h := env.handler

	sessID := createSession(t, h, "")
	w := doJSON(t, h, http.MethodGet, "/api/sessions/"+sessID, "")
	var sess struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}
	decodeBody(t, w, &sess)
	if sess.Title != "New session" {
		t.Fatalf("default title = %q, want %q", sess.Title, "New session")
	}

	w = doJSON(t, h, http.MethodPatch, "/api/sessions/"+sessID, `{"title":"renamed"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("patch: status %d body %s", w.Code, w.Body.String())
	}
	decodeBody(t, w, &sess)
	if sess.Title != "renamed" {
		t.Fatalf("renamed title = %q", sess.Title)
	}
	assertAPIError(t, doJSON(t, h, http.MethodPatch, "/api/sessions/"+sessID, `{"title":""}`),
		http.StatusBadRequest, codeInvalidRequest)

	// Seed a finished run so the cascade has rows to delete.
	postMessage(t, h, sessID, "hello vivy")
	w = doJSON(t, h, http.MethodGet, "/api/sessions", "")
	var list struct {
		Sessions []struct {
			ID string `json:"id"`
		} `json:"sessions"`
	}
	decodeBody(t, w, &list)
	if len(list.Sessions) != 1 || list.Sessions[0].ID != sessID {
		t.Fatalf("sessions list = %+v", list.Sessions)
	}

	w = doJSON(t, h, http.MethodDelete, "/api/sessions/"+sessID, "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete: status %d body %s", w.Code, w.Body.String())
	}
	assertAPIError(t, doJSON(t, h, http.MethodGet, "/api/sessions/"+sessID, ""),
		http.StatusNotFound, codeNotFound)
	assertAPIError(t, doJSON(t, h, http.MethodDelete, "/api/sessions/"+sessID, ""),
		http.StatusNotFound, codeNotFound)
}
