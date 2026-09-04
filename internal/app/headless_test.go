package app

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/tools"
)

// newHeadlessTestService wires the same stack the runtime approval tests
// use: sqlite over a temp dir, checkpoint bridge, scripted model, and the
// headless sink writing into capture buffers.
func newHeadlessTestService(t *testing.T, script []*schema.Message, toolNames ...string) (*runtime.Service, *sqlite.Backend, *headlessSink, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	ctx := context.Background()

	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "headless.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })

	ts, err := tools.Builtin(backend).Resolve(toolNames)
	if err != nil {
		t.Fatalf("resolve tools: %v", err)
	}
	checkpoints, err := runtime.NewVersionedCheckpointStore(backend.Blobs(), "test-engine")
	if err != nil {
		t.Fatalf("checkpoint store: %v", err)
	}
	var m model.ToolCallingChatModel = runtime.NewScriptedModel(script...)
	eng, err := runtime.NewEngine(ctx, m, ts, runtime.EngineConfig{
		StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10, Checkpoints: checkpoints,
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	var out, errw bytes.Buffer
	sink := newHeadlessSink(&out, &errw)
	svc := runtime.NewService(eng, "scripted", "scripted-v0", runtime.ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Notes: backend,
		Approvals: backend, Questions: backend, ApprovalExpiration: 5 * time.Minute,
		Sink: sink, Sessions: backend,
	})
	return svc, backend, sink, &out, &errw
}

func replayEvents(t *testing.T, j storage.Journal, runID domain.RunID) []domain.RunEvent {
	t.Helper()
	it, err := j.Replay(context.Background(), runID, 0)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	defer func() { _ = it.Close() }()
	var events []domain.RunEvent
	for it.Next() {
		events = append(events, it.Value().Event)
	}
	if err := it.Err(); err != nil {
		t.Fatalf("replay iterate: %v", err)
	}
	return events
}

func countHeadlessTerminals(events []domain.RunEvent) int {
	n := 0
	for _, ev := range events {
		if ev.Type == domain.EventRunCompleted || ev.Type == domain.EventRunFailed || ev.Type == domain.EventRunCancelled {
			n++
		}
	}
	return n
}

func TestHeadlessSinkRendersStream(t *testing.T) {
	var out, errw bytes.Buffer
	sink := newHeadlessSink(&out, &errw)
	sink.Publish(domain.RunEvent{Type: domain.EventModelDelta, Payload: []byte(`{"delta":"Hel"}`)})
	sink.Publish(domain.RunEvent{Type: domain.EventModelDelta, Payload: []byte(`{"delta":"lo"}`)})
	sink.Publish(domain.RunEvent{Type: domain.EventModelCompleted, PayloadVersion: 2, Payload: []byte(`{"content_sha256":"185f8db32271fe25f561a6fc938b2e264306ec304eda518007d1764826381969","byte_len":5}`)})
	// A legacy v1 provider that did not stream prints its authoritative
	// completed content once.
	sink.Publish(domain.RunEvent{Type: domain.EventModelCompleted, PayloadVersion: 1, Payload: []byte(`{"content":"Second turn"}`)})
	sink.Publish(domain.RunEvent{Type: domain.EventModelCompleted, PayloadVersion: 2, Payload: []byte(`{"content":"illegal","content_sha256":"0000000000000000000000000000000000000000000000000000000000000000","byte_len":7}`)})
	if got := out.String(); got != "Hello\nSecond turn\n" {
		t.Fatalf("stdout = %q, want streamed text with message breaks", got)
	}
	if !strings.Contains(errw.String(), "model.completed v2") {
		t.Fatalf("stderr = %q, want invalid v2 diagnostic", errw.String())
	}
	sink.Publish(domain.RunEvent{Type: domain.EventToolStarted, Payload: []byte(`{"tool_name":"bash"}`)})
	sink.Publish(domain.RunEvent{Type: domain.EventToolFinished, Payload: []byte(`{"tool_name":"bash","error":"exit status 1"}`)})
	if !strings.Contains(errw.String(), "bash") || !strings.Contains(errw.String(), "exit status 1") {
		t.Fatalf("stderr = %q, want tool notice with failure", errw.String())
	}
	sink.Publish(domain.RunEvent{Type: domain.EventRunFailed, Payload: []byte(`{"cause_category":"provider_error","message":"boom"}`)})
	select {
	case term := <-sink.terminal:
		if term.status != domain.RunFailed || term.failCause != "provider_error" || term.failMessage != "boom" {
			t.Fatalf("terminal = %+v, want failed/provider_error/boom", term)
		}
	default:
		t.Fatal("terminal not captured")
	}
}

// A plain scripted turn streams to stdout, stamps the headless face and
// provenance, and lands as a completed run on the shared Journal.
func TestHeadlessTurnCompletesWithScriptedModel(t *testing.T) {
	svc, backend, sink, out, _ := newHeadlessTestService(t,
		[]*schema.Message{schema.AssistantMessage("All done.", nil)},
		tools.EchoInfoName)
	ctx := context.Background()
	if err := backend.CreateSession(ctx, domain.Session{ID: "sess_hl_ok", Title: "t", CreatedAt: time.Now().UnixMilli()}); err != nil {
		t.Fatalf("create session: %v", err)
	}

	result, err := runHeadlessTurn(ctx, svc, "sess_hl_ok", "say done", sink)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.Status != domain.RunCompleted {
		t.Fatalf("status = %s, want completed", result.Status)
	}
	if !strings.Contains(out.String(), "All done.") {
		t.Fatalf("stdout = %q, want the assistant text", out.String())
	}

	events := replayEvents(t, backend, result.RunID)
	if len(events) == 0 || events[0].Type != domain.EventRunStarted {
		t.Fatalf("first event = %v, want run.started", events)
	}
	var started struct {
		Face string `json:"face"`
	}
	if err := json.Unmarshal(events[0].Payload, &started); err != nil {
		t.Fatalf("decode run.started: %v", err)
	}
	if started.Face != string(domain.FaceHeadless) {
		t.Fatalf("run.started face = %q, want headless", started.Face)
	}
	var completed []domain.RunEvent
	for _, event := range events {
		if event.Type == domain.EventModelCompleted {
			completed = append(completed, event)
		}
	}
	if len(completed) != 1 || completed[0].PayloadVersion != 2 {
		t.Fatalf("model.completed events = %+v, want one v2 boundary", completed)
	}
	var metadata struct {
		Content       string `json:"content"`
		ContentSHA256 string `json:"content_sha256"`
		ByteLen       int    `json:"byte_len"`
	}
	if err := json.Unmarshal(completed[0].Payload, &metadata); err != nil {
		t.Fatalf("decode model.completed: %v", err)
	}
	if metadata.Content != "" || metadata.ByteLen != len([]byte("All done.")) || metadata.ContentSHA256 == "" {
		t.Fatalf("model.completed metadata = %+v, want metadata-only v2 content", metadata)
	}
	msgs, err := backend.ListMessages(ctx, "sess_hl_ok")
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	userSource := ""
	for _, m := range msgs {
		if m.Role == domain.RoleUser {
			userSource = m.EffectiveSource()
		}
	}
	if userSource != "headless" {
		t.Fatalf("user message source = %q, want headless", userSource)
	}
}

// An approval-needed tool call cannot proceed headless: the run is
// cancelled loudly, the suspension is closed durably, and no assistant
// text leaks to stdout.
func TestHeadlessTurnApprovalFailsLoudly(t *testing.T) {
	svc, backend, sink, out, errw := newHeadlessTestService(t,
		[]*schema.Message{
			schema.AssistantMessage("", []schema.ToolCall{{
				ID:       "call-hl-apr",
				Function: schema.FunctionCall{Name: tools.WriteNoteName, Arguments: `{"content":"buy milk"}`},
			}}),
			schema.AssistantMessage("must never stream", nil),
		},
		tools.WriteNoteName)
	ctx := context.Background()
	if err := backend.CreateSession(ctx, domain.Session{ID: "sess_hl_apr", Title: "t", CreatedAt: time.Now().UnixMilli()}); err != nil {
		t.Fatalf("create session: %v", err)
	}

	result, err := runHeadlessTurn(ctx, svc, "sess_hl_apr", "note this", sink)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.Status != domain.RunCancelled {
		t.Fatalf("status = %s, want cancelled", result.Status)
	}
	if out.String() != "" {
		t.Fatalf("stdout = %q, want no assistant text", out.String())
	}
	if !strings.Contains(errw.String(), "approval") {
		t.Fatalf("stderr = %q, want a loud blocking notice", errw.String())
	}
	rows, err := backend.ListPendingApprovals(ctx)
	if err != nil {
		t.Fatalf("list pending approvals: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("pending approvals = %d, want the suspension closed", len(rows))
	}
	events := replayEvents(t, backend, result.RunID)
	if n := countHeadlessTerminals(events); n != 1 {
		t.Fatalf("terminal events = %d, want exactly 1", n)
	}
	if i := indexHeadless(events, domain.EventToolApprovalRequired); i < 0 {
		t.Fatal("approval_required event missing from the journal")
	}
}

// indexHeadless finds the first event of the given type, or -1.
func indexHeadless(events []domain.RunEvent, want domain.EventType) int {
	for i, ev := range events {
		if ev.Type == want {
			return i
		}
	}
	return -1
}

// TestHeadlessTurnQuestionFailsLoudly mirrors the approval path for
// user questions: the scripted model asks instead of acting, the headless
// sink reports the block on stderr, the run is cancelled, and exactly one
// terminal lands in the journal.
func TestHeadlessTurnQuestionFailsLoudly(t *testing.T) {
	svc, backend, sink, out, _ := newHeadlessTestService(t, []*schema.Message{
		{Role: schema.Assistant, Content: "", ToolCalls: []schema.ToolCall{{
			ID: "call-hl-que",
			Function: schema.FunctionCall{
				Name:      tools.AskUserName,
				Arguments: `{"question":"Which color?"}`,
			},
		}}},
		{Role: schema.Assistant, Content: "must never stream"},
	}, tools.AskUserName)
	ctx := context.Background()

	sessionID := domain.SessionID("sess_hl_que")
	if err := backend.CreateSession(ctx, domain.Session{ID: sessionID, Title: "headless question", CreatedAt: time.Now().UnixMilli()}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	result, err := runHeadlessTurn(ctx, svc, sessionID, "pick one", sink)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.Status != domain.RunCancelled {
		t.Fatalf("status = %s, want cancelled", result.Status)
	}
	if out.String() != "" {
		t.Fatalf("stdout = %q, want empty", out.String())
	}
	if !strings.Contains(sink.errw.(*bytes.Buffer).String(), "asked") {
		t.Fatalf("stderr %q missing question notice", sink.errw.(*bytes.Buffer).String())
	}
	if pending, err := backend.ListPendingQuestions(ctx); err == nil && len(pending) != 0 {
		t.Fatalf("pending questions = %d, want 0 (durable cancel path)", len(pending))
	}
	events := replayEvents(t, backend, result.RunID)
	if n := countHeadlessTerminals(events); n != 1 {
		t.Fatalf("terminal events = %d, want exactly 1", n)
	}
	if i := indexHeadless(events, domain.EventUserQuestionRequired); i < 0 {
		t.Fatal("question_required event missing from the journal")
	}
}

// TestResolveHeadlessSession covers --continue resolution (newest wins,
// empty board errors) and fresh-session minting with the truncated title.
func TestResolveHeadlessSession(t *testing.T) {
	_, backend, _, _, _ := newHeadlessTestService(t, nil)
	ctx := context.Background()

	if _, err := resolveHeadlessSession(ctx, backend, HeadlessOptions{ContinueNewest: true}); err == nil {
		t.Fatal("continue with no sessions must error")
	}

	if err := backend.CreateSession(ctx, domain.Session{ID: "sess_old", Title: "old", CreatedAt: time.Now().Add(-2 * time.Minute).UnixMilli()}); err != nil {
		t.Fatalf("create old: %v", err)
	}
	if err := backend.CreateSession(ctx, domain.Session{ID: "sess_new", Title: "new", CreatedAt: time.Now().Add(-1 * time.Minute).UnixMilli()}); err != nil {
		t.Fatalf("create new: %v", err)
	}
	got, err := resolveHeadlessSession(ctx, backend, HeadlessOptions{ContinueNewest: true})
	if err != nil {
		t.Fatalf("continue: %v", err)
	}
	if got != "sess_new" {
		t.Fatalf("continue picked %q, want sess_new", got)
	}

	long := strings.Repeat("v", 80)
	fresh, err := resolveHeadlessSession(ctx, backend, HeadlessOptions{Prompt: long})
	if err != nil {
		t.Fatalf("fresh: %v", err)
	}
	row, err := backend.GetSession(ctx, fresh)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	wantTitle := long[:60] + "..."
	if row.Title != wantTitle {
		t.Fatalf("title = %d runes, want 60 runes + ellipsis", len([]rune(row.Title)))
	}
}
