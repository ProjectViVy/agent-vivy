package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/tools"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

func TestToolOperationTypedReplayAndLiveCause(t *testing.T) {
	for _, status := range []string{domain.ToolOutcomeSuccess, domain.ToolOutcomeRecoverable, domain.ToolOutcomeRefused, domain.ToolOutcomeFatal} {
		t.Run(status, func(t *testing.T) {
			svc, b, run := newBrokerOperationService(t)
			ctx := context.Background()
			c := svc.newToolOperationCoordinator(run, "session-broker-operations")
			raw := []byte(`{"a":1}`)
			op, err := c.Admit(ctx, "typed", "test", raw, raw, raw)
			if err != nil {
				t.Fatal(err)
			}
			want := domain.ToolOperationOutcome{Result: "visible", Status: status, Effects: domain.ToolEffectsUnknown}
			var live error
			switch status {
			case domain.ToolOutcomeRecoverable:
				want.Reason = domain.ToolReasonCommandFailed
				want.Failure = "command exited"
			case domain.ToolOutcomeRefused:
				want.Reason = domain.ToolReasonUserDenied
				want.Effects = domain.ToolEffectsNotExecuted
				want.Failure = "denied"
			case domain.ToolOutcomeFatal:
				want.Result = ""
				live = errors.New("storage sentinel")
				want.Failure = live.Error()
			}
			calls := 0
			result, err := c.Execute(ctx, op, func(context.Context) (domain.ToolOperationOutcome, error) { calls++; return want, live })
			if live != nil && !errors.Is(err, live) {
				t.Fatalf("live cause lost: %v", err)
			}
			if live == nil && (err != nil || result != want.Result) {
				t.Fatalf("live=%q/%v", result, err)
			}
			stored, err := b.GetToolOperation(ctx, run, "typed")
			if err != nil || stored.Outcome() != want {
				t.Fatalf("stored=%+v err=%v", stored, err)
			}
			result, err = c.Execute(ctx, op, func(context.Context) (domain.ToolOperationOutcome, error) {
				calls++
				return successToolOutcome("WRONG"), nil
			})
			if calls != 1 {
				t.Fatal("completed invocation replayed")
			}
			if (status == domain.ToolOutcomeFatal) != (err != nil) {
				t.Fatalf("replay error=%v", err)
			}
			if status != domain.ToolOutcomeFatal && result != want.Result {
				t.Fatal(result)
			}
		})
	}
}

func TestToolOperationLiveErrorsRedactAndKeepCause(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		t.Run(map[bool]string{false: "fatal", true: "cancelled"}[cancelled], func(t *testing.T) {
			svc, backend, run := newBrokerOperationService(t)
			ctx := context.Background()
			coordinator := svc.newToolOperationCoordinator(run, "session-broker-operations")
			raw := []byte(`{}`)
			op, err := coordinator.Admit(ctx, "secret-error", "test", raw, raw, raw)
			if err != nil {
				t.Fatal(err)
			}
			const canary = "sk-live-d03privatebrokerabcdef"
			sentinel := errors.New("provider rejected " + canary)
			var cause error = sentinel
			if cancelled {
				cause = errors.Join(sentinel, context.Canceled)
			}
			_, err = coordinator.Execute(ctx, op, func(context.Context) (domain.ToolOperationOutcome, error) {
				return domain.ToolOperationOutcome{}, cause
			})
			if !errors.Is(err, sentinel) || strings.Contains(err.Error(), canary) {
				t.Fatalf("live error lost cause or leaked: %v", err)
			}
			stored, readErr := backend.GetToolOperation(ctx, run, op.OperationID)
			if readErr != nil || strings.Contains(stored.Failure, canary) {
				t.Fatalf("stored error leaked: %+v, %v", stored, readErr)
			}
			if cancelled {
				if !errors.Is(err, context.Canceled) || stored.State != domain.ToolOperationClaimed {
					t.Fatalf("cancellation lost its fence: %+v %v", stored, err)
				}
			} else {
				_, replayErr := coordinator.Execute(ctx, op, func(context.Context) (domain.ToolOperationOutcome, error) {
					t.Fatal("fatal operation invoked twice")
					return domain.ToolOperationOutcome{}, nil
				})
				if replayErr == nil || strings.Contains(replayErr.Error(), canary) {
					t.Fatalf("replayed error leaked: %v", replayErr)
				}
			}
			for _, event := range replayAll(t, backend, run) {
				if strings.Contains(string(event.Payload), canary) {
					t.Fatal("Journal leaked error canary")
				}
			}
		})
	}
}

// Drive the native producer to its checkpoint while deliberately omitting the
// queued sibling result from the test consumer's Journal. No production hook
// or fake checkpoint is involved: Eino retains ExecutedEnhancedTools itself.
func TestToolOperationEnhancedCachedSiblingBeforeFinished(t *testing.T) {
	for _, mode := range []string{"arguments", "command", "media"} {
		t.Run(mode, func(t *testing.T) {
			command := mode == "command"
			name := contractToolName
			if command {
				name = tools.ExecuteName
			}
			sibling := newContractTool(func(context.Context, json.RawMessage) (string, error) {
				if mode == "media" {
					return `{"parts":[{"type":"text","text":"cached media"},{"type":"image","url":"https://example.invalid/image.png","mime_type":"image/png"},{"type":"audio","base64data":"YXVkaW8=","mime_type":"audio/wav"},{"type":"video","url":"https://example.invalid/video.mp4","mime_type":"video/mp4"},{"type":"file","base64data":"ZmlsZQ==","mime_type":"application/pdf"}]}`, nil
				}
				if command {
					return `{"exit_code":7,"stderr":"expected failure"}`, nil
				}
				return "", &tools.ArgError{Field: "text", Reason: "expected invalid value"}
			})
			sibling.spec.Name = name
			writer := newContractTool(func(context.Context, json.RawMessage) (string, error) { return "approved", nil })
			writer.spec.Name = tools.WriteNoteName
			writer.spec.Readonly = false
			script := []*schema.AgenticMessage{testToolCalls(testCall("cached", name, `{"text":"bad"}`), testCall("approval", tools.WriteNoteName, `{"text":"write"}`)), testAssistant("continued")}
			h := newContractHarness(t, script, contractHarnessOpts{enhanced: true, streaming: true, checkpoints: true, noBarrier: true, prodNudge: true, extraTools: []tools.Tool{sibling, writer}})
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			const run domain.RunID = "run-cached-gap"
			const session domain.SessionID = "session-cached-gap"
			if err := h.backend.CreateSession(ctx, domain.Session{ID: session, CreatedAt: 1}); err != nil {
				t.Fatal(err)
			}
			prompt, err := buildPromptSnapshot(PromptInput{Policy: defaultPolicyForTest(t, domain.PolicyProfileDefault), RunID: run, GenerationID: h.svc.deps.GenerationID, Capture: promptCapture(nil), Face: domain.FaceWeb})
			if err != nil {
				t.Fatal(err)
			}
			m := newEventMapper(run, 64<<10)
			m.setRunScope("", "", string(session))
			start := m.build(domain.EventRunStarted, payloadRunStarted{Provider: "test", Model: "test", Mode: string(domain.RunModeNormal), PolicyProfile: string(domain.PolicyProfileDefault), PromptSchema: prompt.SchemaVersion})
			if _, err := h.backend.CommitRunAdmission(ctx, storage.RunAdmission{
				Run:     domain.Run{ID: run, SessionID: session, CreatedAt: 2},
				Message: domain.Message{ID: "cached-gap-input", SessionID: session, RunID: run, Role: domain.RoleUser, Content: "exercise cached sibling"},
				Started: start, Prompt: &prompt,
			}); err != nil {
				t.Fatal(err)
			}
			ctx = withRunPrompt(ctx, prompt)
			ctx = withSelectedTools(withSessionID(withRunID(ctx, run), session), []string{name, tools.WriteNoteName})
			ctx = withSessionSandbox(ctx, domain.SandboxModeWorkspaceWrite, domain.ApprovalPolicyAsk)
			ctx = withNudgeState(ctx, newNudgeState())
			ctx = tools.WithMountedTools(ctx, tools.NewMountedTools())
			ctx = withToolOperationCoordinator(ctx, h.svc.newToolOperationCoordinator(run, session))
			it := h.svc.engine.Query(ctx, "exercise cached sibling", adk.WithCheckPointID(checkpointIDFor(run)))
			interrupted := false
			for {
				ev, ok := it.Next()
				if !ok {
					break
				}
				events, err := m.onEvent(ev)
				if errors.Is(err, errRunInterrupted) {
					interrupted = true
					h.svc.handleInterrupt(ctx, m, session, []string{name, tools.WriteNoteName}, domain.RunModeNormal)
					continue
				}
				if err != nil {
					t.Fatal(err)
				}
				for _, e := range events {
					if e.Type == domain.EventToolStarted || e.Type == domain.EventToolFinished {
						continue
					}
					if _, err := h.backend.Append(ctx, storage.Commit{RunID: run, Events: []domain.RunEvent{e}}); err != nil {
						t.Fatal(err)
					}
				}
			}
			if !interrupted {
				t.Fatal("native Eino did not checkpoint approval")
			}
			op, err := h.backend.GetToolOperation(ctx, run, "cached")
			wantStatus := domain.ToolOutcomeRecoverable
			if mode == "media" {
				wantStatus = domain.ToolOutcomeSuccess
			}
			if err != nil || op.State != domain.ToolOperationCompleted || op.OutcomeStatus != wantStatus {
				t.Fatalf("producer op=%+v %v", op, err)
			}
			approval := waitForPendingApproval(t, h.backend, run)
			fresh := NewService(h.svc.engine, "test", "test", ServiceDeps{Admission: h.backend, GenerationID: h.svc.deps.GenerationID, Journal: h.backend, Runs: h.backend, Messages: h.backend, Sessions: h.backend, Approvals: h.backend, Questions: h.backend, ToolOperations: h.backend, Sink: newTestSink()})
			t.Cleanup(func() { fresh.CancelAll(); fresh.WaitIdle(context.Background()) })
			if err := fresh.Recover(ctx); err != nil {
				t.Fatal(err)
			}
			if err := fresh.DecideApproval(ctx, approval.ID, domain.ApprovalApproved); err != nil {
				t.Fatal(err)
			}
			deadline := time.Now().Add(6 * time.Second)
			for time.Now().Before(deadline) {
				r, err := h.backend.GetRun(ctx, run)
				if err != nil {
					t.Fatal(err)
				}
				if r.Status == domain.RunCompleted {
					break
				}
				if r.Status.Terminal() {
					t.Fatalf("resumed run=%s", r.Status)
				}
				time.Sleep(10 * time.Millisecond)
			}
			r, _ := h.backend.GetRun(ctx, run)
			if r.Status != domain.RunCompleted {
				t.Fatalf("barrier stranded: status=%s", r.Status)
			}
			if len(sibling.snapshot()) != 1 {
				t.Fatalf("cached sibling invoked %d times", len(sibling.snapshot()))
			}
			n := 0
			for _, e := range replayAll(t, h.backend, run) {
				if e.Type == domain.EventToolFinished {
					var f payloadToolFinished
					_ = json.Unmarshal(e.Payload, &f)
					if f.ToolCallID == "cached" {
						n++
						if mode == "media" {
							if f.Result != "cached media" || len(f.Parts) != 4 || !strings.Contains(string(f.Parts[1]), `"base64_data":"YXVkaW8="`) {
								t.Fatalf("cached media projection lost: %+v", f)
							}
						} else if f.Outcome != domain.ToolOutcomeRecoverable || f.Reason != op.OutcomeReason || f.Effects != op.OutcomeEffects || f.Error == "" {
							t.Fatalf("restored metadata=%+v", f)
						}
					}
				}
				if e.Type == domain.EventPolicyEvaluated {
					var p struct {
						ToolName string `json:"tool_name"`
					}
					_ = json.Unmarshal(e.Payload, &p)
					if p.ToolName == name {
						t.Fatal("cached sibling re-entered governed adapter")
					}
				}
				if e.Type == domain.EventToolNudge {
					t.Fatal("cached sibling recounted as nudge")
				}
			}
			if n != 1 {
				t.Fatalf("cached finished count=%d", n)
			}
			if h.model.count() != 2 {
				t.Fatalf("model handoff count=%d", h.model.count())
			}
			encoded, _ := json.Marshal(h.model.entry(1).input)
			if !strings.Contains(string(encoded), "cached") {
				t.Fatal("cached result absent from next model request")
			}
		})
	}
}

type d03ApprovalStore struct {
	storage.ApprovalStore
	mutate func(*domain.Approval)
}

func (s d03ApprovalStore) GetApproval(ctx context.Context, id string) (domain.Approval, error) {
	a, err := s.ApprovalStore.GetApproval(ctx, id)
	if err == nil {
		s.mutate(&a)
	}
	return a, err
}

func TestToolApprovalDeniedRejectsChangedAuthority(t *testing.T) {
	for _, field := range []string{"run", "call", "tool", "target", "checkpoint", "intent", "missing", "old"} {
		t.Run(field, func(t *testing.T) {
			svc, b, _ := newApprovalService(t, 5*time.Minute)
			ctx := context.Background()
			mustCreateSession(t, b, "denied-binding")
			run, err := svc.Run(ctx, "denied-binding", "write note")
			if err != nil {
				t.Fatal(err)
			}
			a := waitForPendingApproval(t, b, run)
			waitForApprovalEvent(t, b, run)
			if field == "checkpoint" {
				if err := svc.engine.cfg.Checkpoints.Delete(ctx, checkpointIDFor(run)); err != nil {
					t.Fatal(err)
				}
			}
			svc.deps.Approvals = d03ApprovalStore{ApprovalStore: b, mutate: func(a *domain.Approval) {
				switch field {
				case "run":
					a.RunID = "foreign-run"
				case "call":
					a.ToolCallID = "foreign-call"
				case "tool":
					a.ToolName = "foreign-tool"
				case "target":
					a.ResumeTarget = "foreign-target"
				case "intent":
					a.ProposalData, _ = bindToolApprovalProposal(nil, json.RawMessage(`{"content":"different"}`))
				case "missing":
					a.ProposalData = json.RawMessage(`{"kind":"vivy.tool-approval-proposal/v2"}`)
				case "old":
					a.ProposalData = json.RawMessage(`{"kind":"vivy.tool-approval-proposal/v1","arguments_sha256":"old"}`)
				}
			}}
			err = svc.DecideApproval(ctx, a.ID, domain.ApprovalDenied)
			if err == nil {
				deadline := time.Now().Add(3 * time.Second)
				for time.Now().Before(deadline) {
					r, _ := b.GetRun(ctx, run)
					if r.Status.Terminal() {
						if r.Status == domain.RunCompleted {
							t.Fatalf("changed %s treated as valid denied invocation", field)
						}
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				r, _ := b.GetRun(ctx, run)
				if !r.Status.Terminal() {
					t.Fatalf("changed %s stranded active run", field)
				}
			}
			notes, err := b.ListNotes(ctx)
			if err != nil || len(notes) != 0 {
				t.Fatalf("denial changed data: %v/%v", notes, err)
			}
			op, err := b.GetToolOperation(ctx, run, a.ToolCallID)
			if err != nil || op.State != domain.ToolOperationAdmitted {
				t.Fatalf("invalid approval reached claim: %+v %v", op, err)
			}
		})
	}
}

func TestToolApprovalShellExactGovernanceDrift(t *testing.T) {
	for _, field := range []string{"command", "matcher", "hook_timeout", "chain_timeout", "order"} {
		t.Run(field, func(t *testing.T) {
			hook := NewScriptHook("guard --one", "bash", time.Second)
			hook.run = func(context.Context, string, []byte) ([]byte, []byte, int, error) {
				return []byte(`{"decision":"allow"}`), nil, 0, nil
			}
			other := NewScriptHook("other", "bash", time.Second)
			other.run = hook.run
			chain := NewToolHookChain(time.Second, hook, other)
			f := newShellFixture(t, domain.ApprovalPolicyAsk, chain)
			ctx := context.Background()
			run, err := f.service.RunShell(ctx, f.sessionID, "echo prohibited > drift_marker")
			if err != nil {
				t.Fatal(err)
			}
			a := waitForShellApproval(t, f.backend, run)
			waitForApprovalEvent(t, f.backend, run)
			switch field {
			case "command":
				hook.Command = "guard --two"
			case "matcher":
				hook.Matcher = "*"
			case "hook_timeout":
				hook.Timeout = 2 * time.Second
			case "chain_timeout":
				chain.timeout = 2 * time.Second
			case "order":
				chain.hooks[0], chain.hooks[1] = chain.hooks[1], chain.hooks[0]
			}
			if err := f.service.DecideApproval(ctx, a.ID, domain.ApprovalApproved); err != nil {
				t.Fatal(err)
			}
			waitForRunStatus(t, f.backend, run, domain.RunFailed)
			for _, e := range replayAll(t, f.backend, run) {
				if e.Type == domain.EventToolStarted {
					t.Fatal("changed governance executed")
				}
			}
		})
	}
}

func TestToolApprovalShellMissingGovernanceAndBound(t *testing.T) {
	f := newShellFixture(t, domain.ApprovalPolicyAsk)
	ctx := context.Background()
	run, err := f.service.RunShell(ctx, f.sessionID, "echo safe")
	if err != nil {
		t.Fatal(err)
	}
	a := waitForShellApproval(t, f.backend, run)
	waitForApprovalEvent(t, f.backend, run)
	provider, intended, err := unbindToolApprovalProposal(a.ProposalData)
	if err != nil {
		t.Fatal(err)
	}
	ref := shellStateRef(provider)
	invalid, _ := json.Marshal(map[string]any{"kind": "vivy.shell-state/v2", "args": intended, "session_id": f.sessionID, "policy_hash": "test"})
	if err := f.backend.Blobs().Put(ctx, ref, invalid); err != nil {
		t.Fatal(err)
	}
	fresh := NewService(f.service.engine, "test", "test", ServiceDeps{ShellState: f.backend.Blobs()})
	if _, _, err := fresh.loadShellState(ctx, ref); err == nil {
		t.Fatal("missing binding treated as explicit nil-chain")
	}
	hooks := shellHookBinding{ChainPresent: true, Hooks: []string{strings.Repeat("x", maxShellScriptBytes+4096)}}
	if err := fresh.storeShellState(ctx, "oversized-owned-state", shellState{Kind: "vivy.shell-state/v2", Args: intended, SessionID: f.sessionID, Policy: domain.PolicySnapshot{Profile: domain.PolicyProfileDefault, Definition: "policy"}, Hooks: &hooks}); err == nil {
		t.Fatal("oversized binding accepted")
	}
	if _, ok, err := f.backend.Blobs().Get(ctx, "oversized-owned-state"); err != nil || ok {
		t.Fatalf("oversized state written: %v %v", ok, err)
	}
}

func TestToolOperationConcurrentFlightSharesTypedResult(t *testing.T) {
	svc, _, run := newBrokerOperationService(t)
	ctx := context.Background()
	c := svc.newToolOperationCoordinator(run, "session-broker-operations")
	raw := []byte(`{}`)
	op, err := c.Admit(ctx, "flight", "test", raw, raw, raw)
	if err != nil {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	results := make(chan error, 2)
	go func() {
		_, err := c.Execute(ctx, op, func(context.Context) (domain.ToolOperationOutcome, error) {
			close(entered)
			<-release
			return outcomeWithFailure("diagnostic", toolFailure{Status: domain.ToolOutcomeRecoverable, Reason: domain.ToolReasonNotFound, Effects: domain.ToolEffectsNone, Diagnostic: "missing"}), nil
		})
		results <- err
	}()
	<-entered
	go func() {
		result, err := c.Execute(ctx, op, func(context.Context) (domain.ToolOperationOutcome, error) {
			return domain.ToolOperationOutcome{}, errors.New("duplicate invocation")
		})
		if result != "diagnostic" && err == nil {
			err = errors.New("flight lost result")
		}
		results <- err
	}()
	close(release)
	for range 2 {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
}

func TestToolApprovalOpaqueProviderAndContextCopies(t *testing.T) {
	provider := json.RawMessage{0, 255, 'x'}
	intended := json.RawMessage(`{"a":1}`)
	bound, err := bindToolApprovalProposal(provider, intended)
	if err != nil {
		t.Fatal(err)
	}
	provider[0] = 1
	intended[2] = 'z'
	got, args, err := unbindToolApprovalProposal(bound)
	if err != nil || got[0] != 0 || string(args) != `{"a":1}` {
		t.Fatalf("opaque roundtrip=%v %s %v", got, args, err)
	}
	ctx := withApprovedToolArguments(context.Background(), args)
	args[2] = 'b'
	read := approvedToolArguments(ctx)
	read[2] = 'c'
	if string(approvedToolArguments(ctx)) != `{"a":1}` {
		t.Fatal("context binding aliases caller bytes")
	}
}

func TestToolOperationRefusalLiveResultMatchesDurable(t *testing.T) {
	tool := newContractTool(func(context.Context, json.RawMessage) (string, error) {
		return "", refuseCall(strings.Repeat("refusal diagnostic ", 6000), nil, domain.ToolReasonPolicyDenied)
	})
	h := newContractHarness(t, []*schema.AgenticMessage{testToolCalls(testCall("long-refusal", contractToolName, `{"text":"test"}`)), testAssistant("handled")}, contractHarnessOpts{enhanced: true, streaming: true, prodNudge: true, noBarrier: true, extraTools: []tools.Tool{tool}})
	run, err := h.svc.Run(context.Background(), "refusal-result", "invoke tool")
	if err != nil {
		t.Fatal(err)
	}
	waitForRunStatus(t, h.backend, run, domain.RunCompleted)
	op, err := h.backend.GetToolOperation(context.Background(), run, "long-refusal")
	if err != nil {
		t.Fatal(err)
	}
	actual := ""
	for _, m := range toolResults(h.model.entry(1).input) {
		for _, r := range agenticToolResults(m) {
			if r.CallID == "long-refusal" {
				actual = agenticToolResultText(r)
			}
		}
	}
	expected, normalizeErr := normalizeEnhancedResult(op.Result, 64<<10)
	if normalizeErr != nil {
		t.Fatal(normalizeErr)
	}
	if actual != expected.Parts[0].Text {
		t.Fatalf("durable/live refusal differ: durable=%d live=%d", len(op.Result), len(actual))
	}
	if op.OutcomeStatus != domain.ToolOutcomeRefused || op.OutcomeEffects != domain.ToolEffectsNotExecuted {
		t.Fatalf("refusal outcome=%+v", op.Outcome())
	}
	events := replayAll(t, h.backend, run)
	governance := false
	for _, e := range events {
		if e.Type == domain.EventPolicyEvaluated {
			var p struct {
				Decision string `json:"decision"`
			}
			_ = json.Unmarshal(e.Payload, &p)
			governance = governance || p.Decision == "deny"
		}
	}
	if !governance {
		t.Fatal("live refusal lost governance event")
	}
}
