package acpcompat

// Wire-surface probe for eino-contrib/acp v0.0.4.
// Asserts the pinned observations the pilot design depends on:
//   initialize result.protocolVersion == 1
//   session/new accepts [] and returns the test-owned sessionId
//   session/update is written before the session/prompt result
//   permission selected optionId is preserved exactly
//   elicitation/create uses mode=form; accept/decline/cancel decode distinctly
//   unknown form action never increments answered or approved counters

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	acp "github.com/eino-contrib/acp"
)

func TestWireRoundTrip(t *testing.T) {
	h := newWireHarness(t, 1<<20)
	defer h.close()
	cases := readCaseFile(t, "testdata/wire-cases.json")

	var params map[string]json.RawMessage
	if err := json.Unmarshal(cases["initialize"], &params); err != nil {
		t.Fatal(err)
	}

	// initialize -> protocolVersion == 1
	init := h.peer.call(t, "initialize", params)
	var initResult struct {
		ProtocolVersion int64 `json:"protocolVersion"`
	}
	if err := json.Unmarshal(init.Result, &initResult); err != nil {
		t.Fatalf("initialize result not decodable: %v (raw %s)", err, init.Result)
	}
	if initResult.ProtocolVersion != 1 {
		t.Fatalf("protocolVersion = %d, want 1", initResult.ProtocolVersion)
	}

	// session/new accepts [] and returns the test-owned sessionId
	if err := json.Unmarshal(cases["sessionNew"], &params); err != nil {
		t.Fatal(err)
	}
	ns := h.peer.call(t, "session/new", params)
	var nsResult struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(ns.Result, &nsResult); err != nil {
		t.Fatalf("session/new result not decodable: %v (raw %s)", err, ns.Result)
	}
	if nsResult.SessionID != "probe-session-1" {
		t.Fatalf("sessionId = %q, want probe-session-1", nsResult.SessionID)
	}

	// During prompt: agent writes session/update, then asks permission, then
	// returns the prompt result. Peer answers permission with a selected id.
	var permReply map[string]json.RawMessage
	if err := json.Unmarshal(cases["permissionReply"], &permReply); err != nil {
		t.Fatal(err)
	}
	h.peer.onRequest = func(method string, params json.RawMessage) (json.RawMessage, *acp.RPCError) {
		switch method {
		case "session/request_permission":
			return mustJSON(permReply), nil
		default:
			return nil, acp.ErrMethodNotFound(method)
		}
	}

	h.agent.promptHook = func(ctx context.Context, req acp.PromptRequest) (acp.PromptResponse, error) {
		// session/update must be written before the prompt result.
		if err := h.conn.SessionUpdate(ctx, acp.SessionNotification{
			SessionID: req.SessionID,
			Update: acp.SessionUpdate{
				AgentMessageChunk: &acp.SessionUpdateAgentMessageChunk{
					ContentChunk: acp.ContentChunk{
						Content: acp.ContentBlock{
							Text: &acp.ContentBlockText{
								TextContent: acp.TextContent{Text: "chunk-1"},
								Type:        "text",
							},
						},
					},
				},
			},
		}); err != nil {
			t.Errorf("SessionUpdate: %v", err)
		}

		pr, err := h.conn.RequestPermission(ctx, acp.RequestPermissionRequest{
			SessionID: req.SessionID,
			ToolCall:  acp.ToolCallUpdate{ToolCallID: "tc-1", Title: "probe tool"},
			Options: []acp.PermissionOption{
				{Kind: "allow_once", Name: "Allow once", OptionID: "allow-once"},
				{Kind: "reject_once", Name: "Reject once", OptionID: "reject-once"},
			},
		})
		if err != nil {
			t.Errorf("RequestPermission: %v", err)
			return acp.PromptResponse{StopReason: "end_turn"}, nil
		}
		if pr.Outcome.Selected != nil && pr.Outcome.Selected.OptionID == "allow-once" {
			// preserved exactly
			h.agent.approved++
		} else {
			t.Errorf("permission outcome not preserved: %+v", pr.Outcome)
		}
		return acp.PromptResponse{StopReason: "end_turn"}, nil
	}

	if err := json.Unmarshal(cases["sessionPrompt"], &params); err != nil {
		t.Fatal(err)
	}
	promptResp := h.peer.call(t, "session/prompt", params)
	var prResult struct {
		StopReason string `json:"stopReason"`
	}
	if err := json.Unmarshal(promptResp.Result, &prResult); err != nil {
		t.Fatalf("prompt result not decodable: %v (raw %s)", err, promptResp.Result)
	}
	if prResult.StopReason != "end_turn" {
		t.Fatalf("stopReason = %q, want end_turn", prResult.StopReason)
	}
	if h.agent.approved != 1 {
		t.Fatalf("approved counter = %d, want 1", h.agent.approved)
	}

	// Order: the session/update notification must appear on the wire before
	// the session/prompt response.
	var envs []wireEnvelope
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		envs = h.peer.drainLog()
		updateIdx, respIdx := -1, -1
		for i, e := range envs {
			if e.Method == "session/update" && updateIdx < 0 {
				updateIdx = i
			}
			if e.Method == "" && string(e.ID) == string(promptResp.ID) && respIdx < 0 {
				respIdx = i
			}
		}
		if updateIdx >= 0 && respIdx >= 0 {
			if updateIdx > respIdx {
				t.Fatalf("session/update arrived after prompt result (update=%d resp=%d)", updateIdx, respIdx)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("missing envelopes for order check: %d logged", len(envs))
}

func TestUnknownElicitationActionFailsClosed(t *testing.T) {
	h := newWireHarness(t, 1<<20)
	defer h.close()
	cases := readCaseFile(t, "testdata/wire-cases.json")

	var init map[string]json.RawMessage
	if err := json.Unmarshal(cases["initialize"], &init); err != nil {
		t.Fatal(err)
	}
	h.peer.call(t, "initialize", init)

	var ns map[string]json.RawMessage
	if err := json.Unmarshal(cases["sessionNew"], &ns); err != nil {
		t.Fatal(err)
	}
	h.peer.call(t, "session/new", ns)

	var replies map[string]map[string]json.RawMessage
	if err := json.Unmarshal(cases["elicitationReplies"], &replies); err != nil {
		t.Fatal(err)
	}

	type outcome struct {
		kind    string // accept | decline | cancel | error
		content string
	}
	var got outcome
	h.agent.promptHook = func(ctx context.Context, req acp.PromptRequest) (acp.PromptResponse, error) {
		msg := "answer the question"
		schemaType := acp.ElicitationSchemaType("object")
		resp, err := h.conn.UnstableCreateElicitation(ctx, acp.CreateElicitationRequest{
			Form: &acp.CreateElicitationRequestForm{
				ElicitationFormMode: acp.ElicitationFormMode{
					ElicitationSessionScope: &acp.ElicitationFormModeElicitationSessionScope{
						ElicitationSessionScope: acp.ElicitationSessionScope{SessionID: req.SessionID},
						RequestedSchema: acp.ElicitationSchema{
							Type:       &schemaType,
							Required:   []string{"answer"},
							Properties: map[string]acp.ElicitationPropertySchema{},
						},
					},
				},
				Message: &msg,
			},
		})
		if err != nil {
			got = outcome{kind: "error"}
			return acp.PromptResponse{StopReason: "end_turn"}, nil
		}
		switch {
		case resp.Accept != nil:
			got = outcome{kind: "accept"}
			if v, ok := resp.Accept.Content["answer"]; ok && v.String != nil {
				got.content = string(*v.String)
			}
			h.agent.answered++
		case resp.Decline != nil:
			got = outcome{kind: "decline"}
		case resp.Cancel != nil:
			got = outcome{kind: "cancel"}
		default:
			got = outcome{kind: "empty"}
		}
		return acp.PromptResponse{StopReason: "end_turn"}, nil
	}

	var promptParams map[string]json.RawMessage
	if err := json.Unmarshal(cases["sessionPrompt"], &promptParams); err != nil {
		t.Fatal(err)
	}

	run := func(t *testing.T, replyKey string) outcome {
		t.Helper()
		got = outcome{}
		reply, ok := replies[replyKey]
		if !ok {
			t.Fatalf("fixture missing reply %q", replyKey)
		}
		h.peer.onRequest = func(method string, params json.RawMessage) (json.RawMessage, *acp.RPCError) {
			switch method {
			case "elicitation/create":
				// assert wire method + mode=form at the boundary
				var p struct {
					Mode string `json:"mode"`
				}
				if err := json.Unmarshal(params, &p); err != nil || p.Mode != "form" {
					t.Errorf("elicitation/create mode = %q, want form", p.Mode)
				}
				return mustJSON(reply), nil
			default:
				return nil, acp.ErrMethodNotFound(method)
			}
		}
		h.peer.call(t, "session/prompt", promptParams)
		return got
	}

	if o := run(t, "accept"); o.kind != "accept" || o.content != "yes, proceed" {
		t.Fatalf("accept case: got %+v, want accept with content", o)
	}
	if o := run(t, "decline"); o.kind != "decline" {
		t.Fatalf("decline case: got %+v", o)
	}
	if o := run(t, "cancel"); o.kind != "cancel" {
		t.Fatalf("cancel case: got %+v", o)
	}
	answeredBefore := h.agent.answered
	if o := run(t, "unknownAction"); o.kind != "error" {
		t.Fatalf("unknown action: got %+v, want decode error", o)
	}
	if h.agent.answered != answeredBefore {
		t.Fatalf("unknown action incremented answered: %d -> %d", answeredBefore, h.agent.answered)
	}
	if o := run(t, "missingAction"); o.kind != "error" {
		t.Fatalf("missing action: got %+v, want decode error", o)
	}
	if h.agent.answered != answeredBefore || h.agent.approved != 0 {
		t.Fatalf("counters moved on invalid replies: answered=%d approved=%d", h.agent.answered, h.agent.approved)
	}
}
