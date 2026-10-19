package acp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	acp "github.com/eino-contrib/acp"
	acpconn "github.com/eino-contrib/acp/conn"
	acpstdio "github.com/eino-contrib/acp/transport/stdio"
)

// wireTee forwards the agent's stdout to the scripted client while recording
// decoded frames, so ordering assertions read the actual wire, not a
// test-only shortcut.
type wireTee struct {
	mu     sync.Mutex
	frames []map[string]any
}

func (w *wireTee) record(line []byte) {
	var frame map[string]any
	if json.Unmarshal(line, &frame) != nil {
		return
	}
	w.mu.Lock()
	w.frames = append(w.frames, frame)
	w.mu.Unlock()
}

func (w *wireTee) snapshot() []map[string]any {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make([]map[string]any, len(w.frames))
	copy(out, w.frames)
	return out
}

// firstIndex returns the index of the first frame matching method, or -1.
func (w *wireTee) firstIndex(method string) int {
	for i, frame := range w.snapshot() {
		if frame["method"] == method {
			return i
		}
	}
	return -1
}

func (w *wireTee) frame(t *testing.T, method string) map[string]any {
	t.Helper()
	for _, frame := range w.snapshot() {
		if frame["method"] == method {
			return frame
		}
	}
	t.Fatalf("no %q frame on the wire: %v", method, w.snapshot())
	return nil
}

// fakeClient answers the two reverse calls a prompt can raise.
type fakeClient struct {
	acp.BaseClient

	mu           sync.Mutex
	updates      []acp.SessionNotification
	permissions  []acp.RequestPermissionRequest
	elicitations []acp.CreateElicitationRequest

	asked     chan struct{}
	askedOnce sync.Once
	gate      chan struct{}

	permissionReply  acp.RequestPermissionResponse
	permissionErr    error
	elicitationReply acp.CreateElicitationResponse
}

func (c *fakeClient) SessionUpdate(_ context.Context, n acp.SessionNotification) error {
	c.mu.Lock()
	c.updates = append(c.updates, n)
	c.mu.Unlock()
	return nil
}

func (c *fakeClient) RequestPermission(ctx context.Context, params acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
	c.mu.Lock()
	c.permissions = append(c.permissions, params)
	c.mu.Unlock()
	c.askedOnce.Do(func() { close(c.asked) })
	if c.gate != nil {
		select {
		case <-c.gate:
		case <-ctx.Done():
			return acp.RequestPermissionResponse{}, ctx.Err()
		}
	}
	if c.permissionErr != nil {
		return acp.RequestPermissionResponse{}, c.permissionErr
	}
	return c.permissionReply, nil
}

func (c *fakeClient) UnstableCreateElicitation(_ context.Context, params acp.CreateElicitationRequest) (acp.CreateElicitationResponse, error) {
	c.mu.Lock()
	c.elicitations = append(c.elicitations, params)
	c.mu.Unlock()
	c.askedOnce.Do(func() { close(c.asked) })
	return c.elicitationReply, nil
}

func (c *fakeClient) lastPermission() acp.RequestPermissionRequest {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.permissions) == 0 {
		return acp.RequestPermissionRequest{}
	}
	return c.permissions[len(c.permissions)-1]
}

func (c *fakeClient) lastElicitation() acp.CreateElicitationRequest {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.elicitations) == 0 {
		return acp.CreateElicitationRequest{}
	}
	return c.elicitations[len(c.elicitations)-1]
}

type interactionHarness struct {
	agent  *agent
	host   *fakeHost
	client *fakeClient
	wire   *wireTee
	close  func()
}

func newInteractionHarness(t *testing.T, host *fakeHost) *interactionHarness {
	t.Helper()
	clientToAgentR, clientToAgentW := io.Pipe()
	agentToClientR, agentToClientW := io.Pipe()

	tee := &wireTee{}
	clientInR, clientInW := io.Pipe()
	go func() {
		defer clientInW.Close()
		buf := make([]byte, 4096)
		var pending []byte
		for {
			n, err := agentToClientR.Read(buf)
			if n > 0 {
				pending = append(pending, buf[:n]...)
				for {
					idx := bytes.IndexByte(pending, '\n')
					if idx < 0 {
						break
					}
					line := pending[:idx]
					pending = pending[idx+1:]
					if len(bytes.TrimSpace(line)) > 0 {
						tee.record(line)
					}
				}
				if _, werr := clientInW.Write(buf[:n]); werr != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	client := &fakeClient{asked: make(chan struct{})}
	clientConn := acpconn.NewClientConnection(client, acpstdio.NewTransport(clientInR, clientToAgentW))
	ctx, cancel := context.WithCancel(context.Background())
	if err := clientConn.Start(ctx); err != nil {
		cancel()
		t.Fatalf("client conn start: %v", err)
	}

	a := newAgent(host)
	conn, err := newConnection(a, clientToAgentR, agentToClientW)
	if err != nil {
		cancel()
		t.Fatalf("newConnection: %v", err)
	}
	a.bindConnection(conn)
	if err := conn.Start(ctx); err != nil {
		cancel()
		t.Fatalf("agent conn start: %v", err)
	}

	closeFn := func() {
		cancel()
		_ = conn.Close()
		_ = clientConn.Close()
		_ = clientToAgentW.Close()
	}
	t.Cleanup(closeFn)
	return &interactionHarness{agent: a, host: host, client: client, wire: tee, close: closeFn}
}

func newTestPrompt(runID string) *promptState {
	return &promptState{
		scope:    promptScope{SessionID: "sess_test", RunID: runID, Generation: 1},
		result:   make(chan promptResult, 1),
		inbox:    make(chan committedEvent, 8),
		overflow: make(chan struct{}),
	}
}

func approvalEvent(t *testing.T, approvalID, toolCallID, toolName string, expiresIn time.Duration) committedEvent {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"approval_id":  approvalID,
		"tool_call_id": toolCallID,
		"tool_name":    toolName,
		"expires_at":   time.Now().Add(expiresIn).UnixMilli(),
	})
	if err != nil {
		t.Fatalf("marshal approval event: %v", err)
	}
	return committedEvent{Type: "tool.approval_required", Payload: payload, PayloadVersion: 1, Seq: 1}
}

func questionEvent(t *testing.T, questionID, toolCallID, prompt string, expiresIn time.Duration) committedEvent {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"question_id":  questionID,
		"tool_call_id": toolCallID,
		"prompt":       prompt,
		"expires_at":   time.Now().Add(expiresIn).UnixMilli(),
	})
	if err != nil {
		t.Fatalf("marshal question event: %v", err)
	}
	return committedEvent{Type: "user.question_required", Payload: payload, PayloadVersion: 1, Seq: 1}
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// approvalHost returns canned review/get + respond results.
func approvalHost(reviewJSON string, reviewErr error) *fakeHost {
	return &fakeHost{callFn: func(_ context.Context, method string, _ any) (json.RawMessage, error) {
		switch method {
		case "review/get":
			if reviewErr != nil {
				return nil, reviewErr
			}
			return json.RawMessage(reviewJSON), nil
		case "run/get":
			return json.RawMessage(`{"id":"run_1","status":"cancelled"}`), nil
		}
		return json.RawMessage(`{}`), nil
	}}
}

func approvalJSON(sessionID, runID string, withArguments string) string {
	return fmt.Sprintf(`{"id":"apr_1","kind":"approval","status":"pending","session_id":%q,"run_id":%q,"tool_call_id":"tc_1","tool_name":"write_file","action":"write","target":"main.go","preview":"+1 line","risk_findings":["overwrites content"],"expires_at":%d%s}`,
		sessionID, runID, time.Now().Add(time.Minute).UnixMilli(), withArguments)
}

func TestPermissionOnceOnly(t *testing.T) {
	host := approvalHost(approvalJSON("sess_test", "run_1", `,"arguments":{"secret":"never-present"}`), nil)
	h := newInteractionHarness(t, host)
	client := h.client
	client.permissionReply = acp.RequestPermissionResponse{Outcome: acp.RequestPermissionOutcome{
		Selected: &acp.RequestPermissionOutcomeSelected{
			SelectedPermissionOutcome: acp.SelectedPermissionOutcome{OptionID: optionAllowOnce},
		},
	}}

	p := newTestPrompt("run_1")
	proj := newRunProjection(h.agent.nonce, p.scope)
	h.agent.startInteraction(context.Background(), p, proj, approvalEvent(t, "apr_1", "tc_1", "write_file", time.Minute))

	select {
	case <-client.asked:
	case <-time.After(10 * time.Second):
		t.Fatal("client was never asked for permission")
	}
	waitFor(t, "approval/respond", func() bool { return host.callCount("approval/respond") == 1 })

	var responded struct {
		ApprovalID string `json:"approval_id"`
		Decision   string `json:"decision"`
		Reason     string `json:"reason"`
	}
	host.callParams(t, "approval/respond", &responded)
	if responded.ApprovalID != "apr_1" || responded.Decision != decisionApproved {
		t.Fatalf("approval response = %+v, want approved for apr_1", responded)
	}
	if host.callCount("approval/respond") != 1 {
		t.Fatalf("approval responses = %d, want exactly one", host.callCount("approval/respond"))
	}

	request := client.lastPermission()
	if string(request.SessionID) != "sess_test" {
		t.Fatalf("permission sessionId = %q", request.SessionID)
	}
	wantToolCallID := opaqueToolID(h.agent.nonce, p.scope, "tc_1")
	if string(request.ToolCall.ToolCallID) != wantToolCallID {
		t.Fatalf("permission toolCallId = %q, want %q", request.ToolCall.ToolCallID, wantToolCallID)
	}
	if len(request.Options) != 2 {
		t.Fatalf("permission options = %+v, want exactly two", request.Options)
	}
	if string(request.Options[0].OptionID) != optionAllowOnce || request.Options[0].Kind != acp.PermissionOptionKindAllowOnce {
		t.Fatalf("allow option = %+v", request.Options[0])
	}
	if string(request.Options[1].OptionID) != optionRejectOnce || request.Options[1].Kind != acp.PermissionOptionKindRejectOnce {
		t.Fatalf("reject option = %+v", request.Options[1])
	}
	if request.ToolCall.Title != "write_file" {
		t.Fatalf("permission title = %q", request.ToolCall.Title)
	}
	var text strings.Builder
	for _, block := range request.ToolCall.Content {
		if block.Content != nil && block.Content.Content.Content.Text != nil {
			text.WriteString(block.Content.Content.Content.Text.Text)
		}
	}
	body := text.String()
	for _, want := range []string{"Target: main.go", "Preview:", "Risk: overwrites content"} {
		if !strings.Contains(body, want) {
			t.Fatalf("permission body %q lacks %q", body, want)
		}
	}
	if strings.Contains(body, "never-present") {
		t.Fatalf("permission body leaked raw arguments: %q", body)
	}
	if toolCall, request := h.wire.firstIndex("session/update"), h.wire.firstIndex("session/request_permission"); toolCall < 0 || request < 0 || toolCall > request {
		t.Fatalf("wire order tool_call=%d request_permission=%d, want the tool_call first", toolCall, request)
	}
}

func TestPermissionLateReplyAfterCancelHasNoEffect(t *testing.T) {
	host := approvalHost(approvalJSON("sess_test", "run_1", ""), nil)
	h := newInteractionHarness(t, host)
	h.client.gate = make(chan struct{})
	p := newTestPrompt("run_1")
	proj := newRunProjection(h.agent.nonce, p.scope)
	h.agent.startInteraction(context.Background(), p, proj, approvalEvent(t, "apr_1", "tc_1", "write_file", time.Minute))

	select {
	case <-h.client.asked:
	case <-time.After(10 * time.Second):
		t.Fatal("client was never asked for permission")
	}
	h.agent.cancelPromptInteractions(p)
	close(h.client.gate)
	time.Sleep(200 * time.Millisecond)
	if n := host.callCount("approval/respond"); n != 0 {
		t.Fatalf("late reply produced %d approval response(s), want zero", n)
	}
}

func TestPermissionPreviewFailsClosed(t *testing.T) {
	host := approvalHost("", errors.New("control unavailable"))
	h := newInteractionHarness(t, host)
	p := newTestPrompt("run_1")
	proj := newRunProjection(h.agent.nonce, p.scope)
	h.agent.startInteraction(context.Background(), p, proj, approvalEvent(t, "apr_1", "tc_1", "write_file", time.Minute))

	waitFor(t, "deny response", func() bool { return host.callCount("approval/respond") == 1 })
	var responded struct {
		Decision string `json:"decision"`
		Reason   string `json:"reason"`
	}
	host.callParams(t, "approval/respond", &responded)
	if responded.Decision != decisionDenied || responded.Reason == "" {
		t.Fatalf("fail-closed response = %+v, want denied with a reason", responded)
	}
}

func TestPermissionStaleReviewIsUntouched(t *testing.T) {
	host := approvalHost("", codedError{code: -32004})
	h := newInteractionHarness(t, host)
	p := newTestPrompt("run_1")
	proj := newRunProjection(h.agent.nonce, p.scope)
	h.agent.startInteraction(context.Background(), p, proj, approvalEvent(t, "apr_1", "tc_1", "write_file", time.Minute))

	time.Sleep(200 * time.Millisecond)
	if n := host.callCount("approval/respond"); n != 0 {
		t.Fatalf("stale review produced %d response(s), want zero", n)
	}
	if n := len(h.client.snapshotPermissions()); n != 0 {
		t.Fatalf("stale review produced %d permission request(s), want zero", n)
	}
}

func TestPermissionForeignApprovalIsUntouched(t *testing.T) {
	host := approvalHost(approvalJSON("sess_test", "run_other", ""), nil)
	h := newInteractionHarness(t, host)
	p := newTestPrompt("run_1")
	proj := newRunProjection(h.agent.nonce, p.scope)
	h.agent.startInteraction(context.Background(), p, proj, approvalEvent(t, "apr_1", "tc_1", "write_file", time.Minute))

	time.Sleep(200 * time.Millisecond)
	if n := host.callCount("approval/respond"); n != 0 {
		t.Fatalf("foreign approval produced %d response(s), want zero", n)
	}
}

func (c *fakeClient) snapshotPermissions() []acp.RequestPermissionRequest {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]acp.RequestPermissionRequest, len(c.permissions))
	copy(out, c.permissions)
	return out
}

type codedError struct{ code int }

func (e codedError) Error() string     { return "coded control error" }
func (e codedError) RPCErrorCode() int { return e.code }

func TestQuestionFormAnswer(t *testing.T) {
	host := &fakeHost{callFn: func(context.Context, string, any) (json.RawMessage, error) {
		return json.RawMessage(`{}`), nil
	}}
	h := newInteractionHarness(t, host)
	h.client.elicitationReply = acp.CreateElicitationResponse{
		Accept: &acp.CreateElicitationResponseAccept{
			ElicitationAcceptAction: acp.ElicitationAcceptAction{
				Content: map[string]acp.ElicitationContentValue{"answer": acp.NewElicitationContentValueString("42")},
			},
		},
	}
	p := newTestPrompt("run_1")
	proj := newRunProjection(h.agent.nonce, p.scope)
	h.agent.startInteraction(context.Background(), p, proj, questionEvent(t, "que_1", "tc_1", "Pick a number", time.Minute))

	waitFor(t, "question/respond", func() bool { return host.callCount("question/respond") == 1 })
	var answered struct {
		QuestionID string `json:"question_id"`
		Answer     string `json:"answer"`
	}
	host.callParams(t, "question/respond", &answered)
	if answered.QuestionID != "que_1" || answered.Answer != "42" {
		t.Fatalf("question response = %+v", answered)
	}
	if host.callCount("review/respond") != 0 {
		t.Fatalf("accepted answer also cancelled the question")
	}

	request := h.client.lastElicitation()
	if request.Form == nil {
		t.Fatal("elicitation is not a form request")
	}
	scope := request.Form.ElicitationSessionScope
	if scope == nil {
		t.Fatal("elicitation lacks the session scope")
	}
	if string(scope.SessionID) != "sess_test" {
		t.Fatalf("elicitation sessionId = %q", scope.SessionID)
	}
	wantToolCallID := acp.ToolCallID(opaqueToolID(h.agent.nonce, p.scope, "tc_1"))
	if scope.ToolCallID == nil || *scope.ToolCallID != wantToolCallID {
		t.Fatalf("elicitation toolCallId = %v, want %q", scope.ToolCallID, wantToolCallID)
	}
	schema := scope.RequestedSchema
	if schema.Type == nil || *schema.Type != acp.ElicitationSchemaTypeObject {
		t.Fatalf("schema type = %v", schema.Type)
	}
	if len(schema.Required) != 1 || schema.Required[0] != "answer" {
		t.Fatalf("schema required = %v", schema.Required)
	}
	prop, ok := schema.Properties["answer"]
	if !ok || prop.String == nil {
		t.Fatalf("schema answer property = %+v", prop)
	}
	if request.Form.Message == nil || *request.Form.Message != "Pick a number" {
		t.Fatalf("elicitation message = %v", request.Form.Message)
	}
	// The wire frame carries mode=form, a single string answer property and
	// the required list, not only the typed SDK view.
	frame := h.wire.frame(t, "elicitation/create")
	params, _ := frame["params"].(map[string]any)
	if params["mode"] != "form" {
		t.Fatalf("elicitation mode = %v", params["mode"])
	}
	wireSchema, _ := params["requestedSchema"].(map[string]any)
	if wireSchema["type"] != "object" {
		t.Fatalf("wire schema type = %v", wireSchema["type"])
	}
	wireProps, _ := wireSchema["properties"].(map[string]any)
	wireAnswer, _ := wireProps["answer"].(map[string]any)
	if wireAnswer["type"] != "string" {
		t.Fatalf("wire answer property = %v", wireAnswer)
	}
	required, _ := wireSchema["required"].([]any)
	if len(required) != 1 || required[0] != "answer" {
		t.Fatalf("wire required = %v", required)
	}
}

func TestQuestionInvalidAnswerCancels(t *testing.T) {
	for name, content := range map[string]map[string]acp.ElicitationContentValue{
		"blank":      {"answer": acp.NewElicitationContentValueString("   ")},
		"extra key":  {"answer": acp.NewElicitationContentValueString("ok"), "other": acp.NewElicitationContentValueString("x")},
		"non-string": {"answer": acp.NewElicitationContentValueInt64(7)},
		"missing":    {},
		"oversized":  {"answer": acp.NewElicitationContentValueString(strings.Repeat("a", maxAnswerBytes+1))},
	} {
		t.Run(name, func(t *testing.T) {
			host := &fakeHost{callFn: func(context.Context, string, any) (json.RawMessage, error) {
				return json.RawMessage(`{}`), nil
			}}
			h := newInteractionHarness(t, host)
			h.client.elicitationReply = acp.CreateElicitationResponse{
				Accept: &acp.CreateElicitationResponseAccept{
					ElicitationAcceptAction: acp.ElicitationAcceptAction{Content: content},
				},
			}
			p := newTestPrompt("run_1")
			proj := newRunProjection(h.agent.nonce, p.scope)
			h.agent.startInteraction(context.Background(), p, proj, questionEvent(t, "que_1", "tc_1", "Pick a number", time.Minute))

			waitFor(t, "review/respond cancel", func() bool { return host.callCount("review/respond") == 1 })
			var cancelled struct {
				ReviewID string `json:"review_id"`
				Action   string `json:"action"`
				Reason   string `json:"reason"`
			}
			host.callParams(t, "review/respond", &cancelled)
			if cancelled.ReviewID != "que_1" || cancelled.Action != "cancel" || cancelled.Reason == "" {
				t.Fatalf("cancel = %+v", cancelled)
			}
			if n := host.callCount("question/respond"); n != 0 {
				t.Fatalf("invalid answer produced %d question response(s)", n)
			}
		})
	}
}

func TestQuestionDeclineCancels(t *testing.T) {
	host := &fakeHost{callFn: func(context.Context, string, any) (json.RawMessage, error) {
		return json.RawMessage(`{}`), nil
	}}
	h := newInteractionHarness(t, host)
	h.client.elicitationReply = acp.CreateElicitationResponse{
		Decline: &acp.CreateElicitationResponseDecline{},
	}
	p := newTestPrompt("run_1")
	proj := newRunProjection(h.agent.nonce, p.scope)
	h.agent.startInteraction(context.Background(), p, proj, questionEvent(t, "que_1", "tc_1", "Pick a number", time.Minute))

	waitFor(t, "review/respond", func() bool { return host.callCount("review/respond") == 1 })
	if host.callCount("question/respond") != 0 {
		t.Fatal("declined question was answered")
	}
}

func TestInteractionCapacityFailsClosed(t *testing.T) {
	host := approvalHost(approvalJSON("sess_test", "run_1", ""), nil)
	h := newInteractionHarness(t, host)
	p := newTestPrompt("run_1")
	for i := 0; i < maxPendingInteractions; i++ {
		h.agent.interactions[&interaction{p: p, alive: true, cancel: func() {}}] = struct{}{}
	}
	proj := newRunProjection(h.agent.nonce, p.scope)
	h.agent.startInteraction(context.Background(), p, proj, approvalEvent(t, "apr_17", "tc_17", "write_file", time.Minute))

	waitFor(t, "run/cancel", func() bool { return host.callCount("run/cancel") == 1 })
	select {
	case out := <-p.result:
		if out.err == nil || out.err.Code != -32603 {
			t.Fatalf("capacity failure result = %+v", out)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("capacity failure did not decide the prompt")
	}
	if n := len(h.client.snapshotPermissions()); n != 0 {
		t.Fatalf("capacity failure presented %d dialog(s), want zero", n)
	}
}
