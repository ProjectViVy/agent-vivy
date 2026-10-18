package runtime

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/mcphost"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/conformance"
	"agent-vivy/internal/tools"
	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

func TestIssue58RefusalCrashCutReplayKeepsNotExecuted(t *testing.T) {
	ctx := context.Background()
	tool := &classifierStubTool{class: tools.InvocationDenied, findings: []string{"synthetic preexecution refusal"}}
	f := newIssue58Wire(t, tool)
	source := conformance.Issue58Source("main-reader", true)
	admission := conformance.Issue58Admission(source, "refusal-crashcut")
	if _, err := f.b.CommitRunAdmission(ctx, admission); err != nil {
		t.Fatal(err)
	}
	args := `{"command":"synthetic"}`
	call := issue58ToolReply(tool.Spec().Name, args)
	mapper := newEventMapper(admission.Run.ID, 64<<10)
	request := mapper.build(domain.EventModelRequest, map[string]any{"source": "main", "input_provenance": storage.InputProvenance{ContentOrigin: "original"}})
	request.PayloadVersion = 4
	requested := mapper.build(domain.EventToolRequested, payloadToolRequested{ToolCallID: "fixture-call", ToolName: tool.Spec().Name, OriginalArguments: args, Args: map[string]any{"command": "synthetic"}})
	if _, err := f.b.Append(ctx, storage.Commit{RunID: admission.Run.ID, Events: []domain.RunEvent{request, requested}}); err != nil {
		t.Fatal(err)
	}
	invoke := func(svc *Service) ([]*schema.Message, *nudgeState) {
		t.Helper()
		state := newNudgeState()
		c := svc.newToolOperationCoordinator(admission.Run.ID, source.SessionID)
		runCtx := withRunID(withSessionID(withPolicyProfile(withSelectedTools(withNudgeState(ctx, state), []string{tool.Spec().Name}), domain.PolicyProfileDefault), source.SessionID), admission.Run.ID)
		runCtx = withApprovalPolicy(runCtx, domain.ApprovalPolicyAuto)
		runCtx = svc.withRecapRunPolicy(runCtx, admission.Run.ID, source.SessionID, "fixture", "fixture")
		runCtx = withToolOperationCoordinator(runCtx, c)
		node, err := compose.NewToolNode(runCtx, &compose.ToolsNodeConfig{Tools: []einotool.BaseTool{newEnhancedToolAdapter(newToolAdapter(tool, 128, nil, nil, nil))}})
		if err != nil {
			t.Fatal(err)
		}
		result, err := node.Invoke(runCtx, call)
		if err != nil {
			t.Fatal(err)
		}
		return result, state
	}
	_, firstState := invoke(f.svc)
	first, ok := firstState.Failure("fixture-call")
	if !ok || first.Status != toolFailureStatusRefused || first.Effects != toolEffectsNotExecuted || tool.calls != 0 {
		t.Fatal("first attempt was not a typed nonexecution")
	}
	op, err := f.b.GetToolOperation(ctx, admission.Run.ID, "fixture-call")
	if err != nil || op.State != domain.ToolOperationCompleted {
		t.Fatalf("no durable logical outcome %+v %v", op, err)
	}
	for _, e := range replayAll(t, f.b, admission.Run.ID) {
		if e.Type == domain.EventToolFinished {
			t.Fatal("crash cut occurred after tool.finished")
		}
	}
	// Stop after operation completion, before Service maps the native result.
	// Reopen the exact DB and owner without fabricating a checkpoint/terminal.
	f.reopen(t, false)
	tool.class = tools.InvocationSafe
	tool.findings = nil
	replayed, state := invoke(f.svc)
	failure, ok := state.Failure("fixture-call")
	if !ok || failure.Status != toolFailureStatusRefused || failure.Effects != toolEffectsNotExecuted || failure.Reason != toolFailureReasonPolicyDenied {
		t.Fatalf("completed refusal reinterpreted by current classifier: %+v present=%v", failure, ok)
	}
	if tool.calls != 0 {
		t.Fatal("replay executed previously refused tool")
	}
	after, err := f.b.GetToolOperation(ctx, admission.Run.ID, "fixture-call")
	if err != nil || after.Result != op.Result || after.State != op.State {
		t.Fatal("replay changed durable operation")
	}
	mapper = newEventMapper(admission.Run.ID, 64<<10)
	mapper.setNudgeState(state)
	events, err := mapper.toolResultEventsParts(tool.Spec().Name, "fixture-call", "retained refusal", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	var finish payloadToolFinished
	json.Unmarshal(events[1].Payload, &finish)
	if finish.Outcome != toolFailureStatusRefused || finish.Effects != toolEffectsNotExecuted {
		t.Fatal("native finish lost restored typed nonexecution")
	}
	messages, _, _, err := f.svc.runMessagesForRunWithCollaboration(withRunID(ctx, admission.Run.ID), source.SessionID, admission.Message.Content, f.svc.engine, domain.FaceWeb, "", domain.CollaborationModeNone)
	if err != nil {
		t.Fatal(err)
	}
	stampInputProvenance(call, storage.InputProvenance{ContentOrigin: "original"})
	messages = append(messages, call)
	messages = append(messages, replayed...)
	mapper.setUsageRoutes("fixture", "fixture", "")
	observed := f.svc.withLiveModelStreamObserver(withRunID(ctx, admission.Run.ID), mapper, source.SessionID, nil, nil, false, nil)
	stream, err := observeChatModel(f.svc.engine.chatModel).Stream(observed, messages)
	if err != nil {
		t.Fatal(err)
	}
	for {
		_, err = stream.Recv()
		if err != nil {
			break
		}
	}
	stream.Close()
	requests := f.capture()
	if len(requests) != 1 || !strings.Contains(requests[0], "did not run: synthetic preexecution refusal") {
		t.Fatal("actual provider did not receive exact restored refusal")
	}
	t.Logf("HTTP_CAPTURE %s", requests[0])
	through, err := f.b.CaptureSubjectHistory(ctx, source.SubjectHistoryBinding)
	if err != nil || through != 0 {
		t.Fatal("logical refusal replay fabricated terminal coverage")
	}
	retained, err := f.b.GetRun(ctx, admission.Run.ID)
	if err != nil || retained.Status != domain.RunActive {
		t.Fatal("crash-cut replay fabricated a terminal run")
	}
}

func TestIssue58LegacyToolOutcomeRequiresExactHostProof(t *testing.T) {
	for _, kind := range []string{"absent", "prior-refusal", "prior-success", "wrong-call", "malformed", "null-unit", "partial-unit", "null-result", "null-error"} {
		t.Run(kind, func(t *testing.T) {
			ctx := context.Background()
			f := newIssue58Wire(t)
			source := conformance.Issue58Source("main-reader", true)
			admission := conformance.Issue58Admission(source, "legacy-outcome")
			if _, err := f.b.CommitRunAdmission(ctx, admission); err != nil {
				t.Fatal(err)
			}
			request := domain.RunEvent{RunID: admission.Run.ID, Type: domain.EventModelRequest, PayloadVersion: 4, Payload: []byte(`{"source":"main","input_provenance":{"content_origin":"original","exclude_automatic_ingest":false}}`)}
			if _, err := f.b.Append(ctx, storage.Commit{RunID: admission.Run.ID, Events: []domain.RunEvent{request}}); err != nil {
				t.Fatal(err)
			}
			c := f.svc.newToolOperationCoordinator(admission.Run.ID, source.SessionID)
			args := []byte(`{}`)
			op, err := c.Admit(ctx, "legacy-call", "legacy-tool", args, args, args)
			if err != nil {
				t.Fatal(err)
			}
			// Construct a real immutable legacy artifact with no typed outcome tuple.
			evidence := storage.ToolEvidence{Result: "did not run: this text is not outcome proof", ModelProjection: "legacy projection"}
			raw, err := json.Marshal(evidence)
			if err != nil {
				t.Fatal(err)
			}
			ref := storage.NewToolEvidenceReference(op, raw, "")
			ref.Projection = evidence.ModelProjection + "\n[exact tool result: " + ref.ID + " sha256=" + ref.SHA256 + "]"
			if err = f.b.Blobs().Put(ctx, ref.ID, raw); err != nil {
				t.Fatal(err)
			}
			encoded, _ := json.Marshal(ref)
			if _, _, _, err = f.b.ClaimToolOperation(ctx, op.RunID, op.OperationID, "owner"); err != nil {
				t.Fatal(err)
			}
			op, _, err = f.b.CompleteToolOperation(ctx, op.RunID, op.OperationID, "owner", string(encoded), "")
			if err != nil {
				t.Fatal(err)
			}
			if kind != "absent" {
				payload := payloadToolFinished{ToolCallID: op.OperationID, ToolName: op.ToolName, Result: "arbitrary old body"}
				switch kind {
				case "prior-refusal":
					payload.Outcome = toolFailureStatusRefused
					payload.Reason = toolFailureReasonPolicyDenied
					payload.Effects = toolEffectsNotExecuted
					payload.Error = "exact old refusal"
				case "wrong-call":
					payload.ToolCallID = "different-call"
				case "malformed":
					payload.Outcome = "future"
					payload.Effects = "unknown"
				}
				p, _ := json.Marshal(payload)
				switch kind {
				case "null-unit":
					p = []byte(`{"tool_call_id":"legacy-call","tool_name":"legacy-tool","result":"old body","outcome":null,"reason":null,"effects":null}`)
				case "partial-unit":
					p = []byte(`{"tool_call_id":"legacy-call","tool_name":"legacy-tool","result":"old body","reason":""}`)
				case "null-result":
					p = []byte(`{"tool_call_id":"legacy-call","tool_name":"legacy-tool","result":null}`)
				case "null-error":
					p = []byte(`{"tool_call_id":"legacy-call","tool_name":"legacy-tool","result":"old body","error":null}`)
				}
				if _, err = f.b.Append(ctx, storage.Commit{RunID: op.RunID, Events: []domain.RunEvent{{RunID: op.RunID, Type: domain.EventToolFinished, PayloadVersion: 1, Payload: p}}}); err != nil {
					t.Fatal(err)
				}
			}
			f.reopen(t, false)
			state := newNudgeState()
			c = f.svc.newToolOperationCoordinator(op.RunID, source.SessionID)
			result, err := c.Execute(withNudgeState(ctx, state), op, func(context.Context) (string, error) { t.Fatal("legacy replay invoked tool"); return "", nil })
			switch kind {
			case "prior-refusal":
				failure, ok := state.Failure(op.OperationID)
				if err != nil || !ok || failure.Status != toolFailureStatusRefused || failure.Effects != toolEffectsNotExecuted || !strings.Contains(result, evidence.Result) {
					t.Fatalf("host refusal not restored %+v %v", failure, err)
				}
			case "prior-success":
				if err != nil || !strings.Contains(result, evidence.Result) {
					t.Fatalf("exact prior host success unavailable %v", err)
				}
				if _, ok := state.Failure(op.OperationID); ok {
					t.Fatal("free text recertified as refusal")
				}
			default:
				if err == nil {
					t.Fatalf("uncertified legacy outcome accepted kind=%s", kind)
				}
				if _, ok := state.Failure(op.OperationID); ok {
					t.Fatal("unavailable artifact invented failure")
				}
			}
			after, err := f.b.GetToolOperation(ctx, op.RunID, op.OperationID)
			if err != nil || after.Result != op.Result {
				t.Fatal("legacy read rewrote receipt")
			}
		})
	}
}

type issue58EmptyRemoteErrorTool struct{ remoteErrTool }

func (t *issue58EmptyRemoteErrorTool) InvokableRun(context.Context, json.RawMessage) (string, error) {
	t.calls++
	return "", &mcphost.ToolExecutionError{}
}

func TestIssue58EmptyNativeDiagnosticRetainsTypedOutcome(t *testing.T) {
	tool := &issue58EmptyRemoteErrorTool{}
	f := newIssue58Wire(t, tool)
	f.reply = func(i int) *schema.Message {
		if i == 0 {
			return schema.AssistantMessage("", []schema.ToolCall{{ID: "empty-diagnostic", Type: "function", Function: schema.FunctionCall{Name: tool.Spec().Name, Arguments: `{"q":"fixture"}`}}})
		}
		return schema.AssistantMessage("complete", nil)
	}
	run := f.run(t, "use the remote tool")
	if run.Status != domain.RunCompleted || tool.calls != 1 || len(f.capture()) != 2 {
		t.Fatalf("native empty diagnostic did not complete: status=%s calls=%d requests=%d", run.Status, tool.calls, len(f.capture()))
	}
	var finished bool
	for _, event := range replayAll(t, f.b, run.ID) {
		if event.Type != domain.EventToolFinished {
			continue
		}
		var p payloadToolFinished
		if err := json.Unmarshal(event.Payload, &p); err != nil {
			t.Fatal(err)
		}
		if p.Outcome != toolFailureStatusRecoverable || p.Reason != toolFailureReasonRemoteTool || p.Effects != toolEffectsUnknown || p.Error != "" {
			t.Fatalf("native outcome changed: %+v", p)
		}
		finished = true
	}
	if !finished {
		t.Fatal("missing native tool.finished")
	}
	f.restart(t)
	op, err := f.b.GetToolOperation(context.Background(), run.ID, "empty-diagnostic")
	if err != nil || op.State != domain.ToolOperationCompleted {
		t.Fatal("empty diagnostic outcome not durable", err)
	}
	var ref storage.ToolEvidenceReference
	if err := json.Unmarshal([]byte(op.Result), &ref); err != nil {
		t.Fatal(err)
	}
	raw, found, err := f.b.Blobs().Get(context.Background(), ref.ID)
	if err != nil || !found {
		t.Fatal("missing retained outcome", err)
	}
	body, err := storage.VerifyToolEvidence(ref, raw)
	if err != nil || body.Outcome == nil || body.Outcome.Outcome != toolFailureStatusRecoverable || body.Outcome.Diagnostic != "" {
		t.Fatalf("empty diagnostic tuple lost after reopen: %+v %v", body.Outcome, err)
	}
}
