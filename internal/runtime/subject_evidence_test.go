package runtime

import (
	"agent-vivy/internal/cognitivecontract"
	"agent-vivy/internal/provider"
	"agent-vivy/internal/tools"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/cloudwego/eino/schema"
	"strings"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/conformance"
	"agent-vivy/internal/testsupport"
)

type issue58ExactTool struct {
	calls  int
	result string
}

func (t *issue58ExactTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{Name: "issue58_exact", Readonly: true, Exposure: domain.ToolExposureDirect, Schema: json.RawMessage(`{"type":"object","properties":{"number":{"type":"number"}},"additionalProperties":false}`)}
}
func (t *issue58ExactTool) InvokableRun(context.Context, json.RawMessage) (string, error) {
	t.calls++
	return t.result, nil
}

func TestIssue58OriginalToolResultBeforeCompaction(t *testing.T) {
	ctx := context.Background()
	svc, b, _ := newTestService(t, testsupport.NewEchoModel())
	conformance.Issue58Identity(t, b)
	source := conformance.Issue58Source("exact-ordinary", false)
	mustCreateSession(t, b, source.SessionID)
	svc.deps.SubjectSources = func(context.Context, domain.SessionID) (*storage.SubjectSourceBinding, error) { return &source, nil }
	admission := conformance.Issue58Admission(source, "exact-tool-run")
	if _, err := b.CommitRunAdmission(ctx, admission); err != nil {
		t.Fatal(err)
	}
	svc.engine.cfg.MaxToolResultBytes = 128
	coordinator := svc.newToolOperationCoordinator(admission.Run.ID, source.SessionID)
	args := []byte("{ \"value\" : 42 }")
	op, err := coordinator.Admit(ctx, "exact-call", "issue58_exact", args, args, args)
	if err != nil {
		t.Fatal(err)
	}
	tool := &issue58ExactTool{result: "head-" + strings.Repeat("authorized original bytes\n", 10000) + "-tail"}
	adapter := newToolAdapter(tool, 128, nil, nil, nil)
	callCtx := withToolOperationCoordinator(ctx, coordinator)
	projection, err := adapter.run(callCtx, string(args), &op)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := b.GetToolOperation(ctx, admission.Run.ID, op.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	var reference struct {
		Schema          string `json:"schema"`
		ID              string `json:"id"`
		SHA256          string `json:"sha256"`
		ByteCount       int    `json:"byte_count"`
		ResultSHA256    string `json:"result_sha256"`
		ResultByteCount int    `json:"result_byte_count"`
	}
	if json.Unmarshal([]byte(stored.Result), &reference) != nil || reference.Schema != "vivy.tool-evidence/v1" || reference.ID == "" {
		t.Fatalf("completed receipt lost full original before compaction: %.300s", stored.Result)
	}
	raw, found, err := b.Blobs().Get(ctx, reference.ID)
	if err != nil || !found {
		t.Fatalf("original artifact missing: %v", err)
	}
	var evidence struct {
		Result  string `json:"result"`
		Failure string `json:"failure"`
	}
	if err := json.Unmarshal(raw, &evidence); err != nil {
		t.Fatal(err)
	}
	if evidence.Result != tool.result || sha256Hex(raw) != reference.SHA256 || len(raw) != reference.ByteCount || reference.ResultSHA256 != sha256Hex([]byte(tool.result)) || reference.ResultByteCount != len(tool.result) {
		t.Fatal("original bytes/hash/length changed")
	}
	if !strings.Contains(projection, reference.ID) || len(projection) >= len(tool.result) {
		t.Fatalf("projection lacks exact reference: %.300s", projection)
	}
	replay, err := adapter.run(callCtx, string(args), &op)
	if err != nil || replay != projection || tool.calls != 1 {
		t.Fatalf("receipt replay invoked tool twice: calls=%d error=%v", tool.calls, err)
	}
}

func TestIssue58ShellAdmissionRetainsExactEvidence(t *testing.T) {
	for _, main := range []bool{false, true} {
		t.Run(fmt.Sprintf("main-%t", main), func(t *testing.T) {
			f := newShellFixture(t, domain.ApprovalPolicyAuto)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			conformance.Issue58Identity(t, f.backend)
			source := conformance.Issue58Source(f.sessionID, main)
			if main {
				f.service.deps.MainConversation = issue58MainResolver{binding: cognitivecontract.MainConversationBinding{SubjectID: source.SubjectID, DomainID: source.DomainID, ConversationID: source.ConversationID, ActiveSessionID: source.SessionID, PolicyID: source.PolicyID, ToolPolicyRef: source.ToolPolicyRef, SchemaVersion: source.SchemaVersion, Grant: source.Grant}}
			} else {
				f.service.deps.SubjectSources = func(context.Context, domain.SessionID) (*storage.SubjectSourceBinding, error) { return &source, nil }
			}
			rid, err := f.service.RunShell(ctx, f.sessionID, "printf 'exact shell original'")
			if err != nil {
				t.Fatalf("eligible shell failed instead of atomic protected admission: %v", err)
			}
			pin, err := f.backend.GetRunSubjectSource(ctx, rid)
			if err != nil || !storage.SameSubjectSourceBinding(pin, source) {
				t.Fatalf("shell execution preceded durable policy/grant pins: %+v %v", pin, err)
			}
			if !f.service.WaitIdle(ctx) {
				t.Fatal("shell did not settle")
			}
			rows, err := f.backend.ListMessages(ctx, f.sessionID)
			if err != nil {
				t.Fatal(err)
			}
			for _, m := range rows {
				if m.Role == domain.RoleUser {
					t.Fatal("shell invented user transcript")
				}
			}
			high, err := f.backend.CaptureSubjectHistory(ctx, source.SubjectHistoryBinding)
			if err != nil || high != 1 {
				t.Fatalf("shell exact interaction not sealed: %d %v", high, err)
			}
			if err := f.backend.DeleteSession(ctx, f.sessionID); err != storage.ErrRetainedSession {
				t.Fatalf("shell originals not protected: %v", err)
			}

			// The same pinned source also accepts subsequent shell interactions.
			next, err := f.service.RunShell(ctx, f.sessionID, "printf 'second exact shell original'")
			if err != nil {
				t.Fatal(err)
			}
			if !f.service.WaitIdle(ctx) {
				t.Fatal("second shell did not settle")
			}
			nextPin, err := f.backend.GetRunSubjectSource(ctx, next)
			if err != nil || !storage.SameSubjectSourceBinding(nextPin, pin) {
				t.Fatalf("existing source pins changed: %+v %v", nextPin, err)
			}
			high, err = f.backend.CaptureSubjectHistory(ctx, source.SubjectHistoryBinding)
			if err != nil || high != 2 {
				t.Fatalf("existing source shell seal count: %d %v", high, err)
			}
		})
	}
}

func TestIssue58ShellNoContextExcluded(t *testing.T) {
	f := newShellFixture(t, domain.ApprovalPolicyAuto)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conformance.Issue58Identity(t, f.backend)
	source := conformance.Issue58Source(f.sessionID, false)
	f.service.deps.SubjectSources = func(context.Context, domain.SessionID) (*storage.SubjectSourceBinding, error) { return &source, nil }
	rid, err := f.service.RunShell(ctx, f.sessionID, "printf 'excluded'", ShellRunOptions{NoContext: true})
	if err != nil {
		t.Fatal(err)
	}
	if !f.service.WaitIdle(ctx) {
		t.Fatal("shell did not settle")
	}
	if _, err := f.backend.GetRunSubjectSource(ctx, rid); err != storage.ErrNotFound {
		t.Fatalf("NoContext shell acquired subject eligibility: %v", err)
	}
	rows, err := f.backend.ListMessages(ctx, f.sessionID)
	if err != nil || len(rows) != 0 {
		t.Fatalf("NoContext shell entered model context: %d %v", len(rows), err)
	}
}

type issue58ErrorTool struct{ calls int }

func (t *issue58ErrorTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{Name: "issue58_error", Readonly: true}
}
func (t *issue58ErrorTool) InvokableRun(context.Context, json.RawMessage) (string, error) {
	t.calls++
	return "known partial result", errors.New("exact failure with authorized task text")
}
func TestIssue58OriginalErrorBeforeSoftConversion(t *testing.T) {
	ctx := context.Background()
	svc, b, _ := newTestService(t, testsupport.NewEchoModel())
	conformance.Issue58Identity(t, b)
	source := conformance.Issue58Source("tool-error-source", false)
	mustCreateSession(t, b, source.SessionID)
	svc.deps.SubjectSources = func(context.Context, domain.SessionID) (*storage.SubjectSourceBinding, error) { return &source, nil }
	a := conformance.Issue58Admission(source, "error-run")
	if _, err := b.CommitRunAdmission(ctx, a); err != nil {
		t.Fatal(err)
	}
	c := svc.newToolOperationCoordinator(a.Run.ID, source.SessionID)
	args := []byte(`{}`)
	op, err := c.Admit(ctx, "error-call", "issue58_error", args, args, args)
	if err != nil {
		t.Fatal(err)
	}
	tool := &issue58ErrorTool{}
	adapter := newToolAdapter(tool, 128, nil, nil, nil)
	if _, err := adapter.run(withToolOperationCoordinator(ctx, c), string(args), &op); err == nil {
		t.Fatal("tool failure lost")
	}
	stored, err := b.GetToolOperation(ctx, a.Run.ID, op.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := storage.DecodeToolEvidenceReference(stored)
	if err != nil {
		t.Fatal(err)
	}
	raw, found, err := b.Blobs().Get(ctx, ref.ID)
	if err != nil || !found {
		t.Fatalf("missing failure artifact: %v", err)
	}
	evidence, err := storage.VerifyToolEvidence(ref, raw)
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Result != "known partial result" || evidence.Failure != "exact failure with authorized task text" {
		t.Fatalf("partial result/error substituted before capture: %+v", evidence)
	}
}

type issue58FailBlobs struct {
	storage.BlobStore
	fail bool
}

func (b *issue58FailBlobs) Put(ctx context.Context, id string, raw []byte) error {
	if b.fail {
		return errors.New("injected artifact save failure")
	}
	return b.BlobStore.Put(ctx, id, raw)
}

type issue58FailOperations struct{ storage.ToolOperationStore }

func (s issue58FailOperations) CompleteToolOperation(context.Context, domain.RunID, string, string, string, string) (domain.ToolOperation, domain.RunEvent, error) {
	return domain.ToolOperation{}, domain.RunEvent{}, errors.New("injected operation save failure")
}
func TestIssue58EvidenceSaveFailureBlocksContinuation(t *testing.T) {
	for _, fault := range []string{"artifact", "operation"} {
		t.Run(fault, func(t *testing.T) {
			ctx := context.Background()
			svc, b, _ := newTestService(t, testsupport.NewEchoModel())
			conformance.Issue58Identity(t, b)
			source := conformance.Issue58Source("fault-source", false)
			mustCreateSession(t, b, source.SessionID)
			svc.deps.SubjectSources = func(context.Context, domain.SessionID) (*storage.SubjectSourceBinding, error) { return &source, nil }
			a := conformance.Issue58Admission(source, "fault-run")
			if _, err := b.CommitRunAdmission(ctx, a); err != nil {
				t.Fatal(err)
			}
			blobs := &issue58FailBlobs{BlobStore: b.Blobs()}
			svc.deps.ToolEvidence = blobs
			c := svc.newToolOperationCoordinator(a.Run.ID, source.SessionID)
			args := []byte(`{}`)
			op, err := c.Admit(ctx, "fault-call", "issue58_exact", args, args, args)
			if err != nil {
				t.Fatal(err)
			}
			if fault == "artifact" {
				blobs.fail = true
			} else {
				svc.deps.ToolOperations = issue58FailOperations{ToolOperationStore: b}
			}
			calls := 0
			result, err := c.Execute(ctx, op, func(context.Context) (string, error) { calls++; return "known external outcome", nil })
			if err == nil || result != "" {
				t.Fatalf("failed exact persistence reached successful continuation: %q %v", result, err)
			}
			stored, err := b.GetToolOperation(ctx, a.Run.ID, op.OperationID)
			if err != nil || stored.State != domain.ToolOperationClaimed {
				t.Fatalf("failed persistence invented completed receipt: %+v %v", stored, err)
			}
			svc.deps.ToolOperations = b
			blobs.fail = false
			if _, err := c.Execute(ctx, op, func(context.Context) (string, error) { calls++; return "reexecuted", nil }); !errors.Is(err, ErrToolOperationUnknown) || calls != 1 {
				t.Fatalf("uncertain operation reexecuted: calls=%d %v", calls, err)
			}
			m := newEventMapper(a.Run.ID, 64<<10)
			svc.emitTerminal(ctx, m, m.build(domain.EventRunFailed, payloadRunFailed{Message: "uncertain evidence"}))
			issue58AssertExcludedOnly(t, b, source.SubjectHistoryBinding)
		})
	}
}

func TestIssue58DefaultOrdinaryToolEvidenceExcluded(t *testing.T) {
	ctx := context.Background()
	svc, b, _ := newTestService(t, testsupport.NewEchoModel())
	mustCreateSession(t, b, "ordinary-source")
	if err := b.CreateRun(ctx, domain.Run{ID: "ordinary-run", SessionID: "ordinary-source", Status: domain.RunActive}); err != nil {
		t.Fatal(err)
	}
	c := svc.newToolOperationCoordinator("ordinary-run", "ordinary-source")
	args := []byte(`{}`)
	op, err := c.Admit(ctx, "ordinary-call", "issue58_exact", args, args, args)
	if err != nil {
		t.Fatal(err)
	}
	// Descriptor-shaped ordinary task text is opaque and must never be expanded.
	raw := `{"schema":"vivy.tool-evidence/v1","id":"not-an-artifact","projection":"wrong"}`
	got, err := c.Execute(ctx, op, func(context.Context) (string, error) { return raw, nil })
	if err != nil || got != raw {
		t.Fatalf("ordinary text interpreted as internal reference: %q %v", got, err)
	}
	ids, err := b.Blobs().(storage.BlobPrefixLister).ListPrefix(ctx, "subject-tool/")
	if err != nil || len(ids) != 0 {
		t.Fatalf("ordinary operation retained subject artifacts: %v %v", ids, err)
	}
}

func TestIssue58InvalidUTF8CannotBecomeOriginal(t *testing.T) {
	ctx := context.Background()
	svc, b, _ := newTestService(t, testsupport.NewEchoModel())
	conformance.Issue58Identity(t, b)
	source := conformance.Issue58Source("invalid-source", false)
	mustCreateSession(t, b, source.SessionID)
	svc.deps.SubjectSources = func(context.Context, domain.SessionID) (*storage.SubjectSourceBinding, error) { return &source, nil }
	a := conformance.Issue58Admission(source, "invalid-run")
	if _, err := b.CommitRunAdmission(ctx, a); err != nil {
		t.Fatal(err)
	}
	c := svc.newToolOperationCoordinator(a.Run.ID, source.SessionID)
	args := []byte(`{}`)
	op, err := c.Admit(ctx, "invalid-call", "issue58_exact", args, args, args)
	if err != nil {
		t.Fatal(err)
	}
	output, err := c.Execute(ctx, op, func(context.Context) (string, error) { return string([]byte{'a', 0xff, 'b'}), nil })
	if !errors.Is(err, storage.ErrSubjectEvidenceIncomplete) || output != "" {
		t.Fatalf("invalid bytes silently became a successful original: %q %v", output, err)
	}
}

func TestIssue58AuthorizedOrdinaryAndLifeKeepTools(t *testing.T) {
	for _, main := range []bool{false, true} {
		t.Run(fmt.Sprintf("resident-%t", main), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			svc, b, _ := newTestService(t, testsupport.NewEchoModel())
			conformance.Issue58Identity(t, b)
			source := conformance.Issue58Source("capability-source", main)
			mustCreateSession(t, b, source.SessionID)
			if main {
				svc.deps.MainConversation = issue58MainResolver{binding: cognitivecontract.MainConversationBinding{SubjectID: source.SubjectID, DomainID: source.DomainID, ConversationID: source.ConversationID, ActiveSessionID: source.SessionID, PolicyID: source.PolicyID, ToolPolicyRef: source.ToolPolicyRef, SchemaVersion: source.SchemaVersion, Grant: source.Grant}}
			} else {
				svc.deps.SubjectSources = func(context.Context, domain.SessionID) (*storage.SubjectSourceBinding, error) { return &source, nil }
			}
			args := "{ \"number\" : 42 }"
			tool := &issue58ExactTool{result: strings.Repeat("exact retained original", 10000)}
			model := NewScriptedModel(schema.AssistantMessage("intermediate", []schema.ToolCall{{ID: "capability-call", Function: schema.FunctionCall{Name: tool.Spec().Name, Arguments: args}}}), schema.AssistantMessage("final response", nil))
			recorder := &recordingChatModel{inner: model}
			engine, err := NewEngine(ctx, recorder, []tools.Tool{tool}, EngineConfig{StreamBuffer: 8, MaxToolResultBytes: 128, MaxEventPayloadBytes: 64 << 10})
			if err != nil {
				t.Fatal(err)
			}
			svc.engine = engine
			if main {
				svc.deps.Recap = issue58Owner(t)
				svc.deps.ToolEvidence = b.Blobs()
				// This producer capability fixture intentionally emits a 220KiB result;
				// give its declared model enough capacity for exact selected-policy input.
				svc.SetCatalog(provider.NewCatalog(provider.Vendor{Name: "test", Endpoints: []provider.Endpoint{{Adapter: provider.AdapterOpenAICompletions, DefaultModel: "test-model", Models: []provider.Model{{ID: "test-model", ContextWindow: 1 << 20}}}}}))
			}
			rid, err := svc.Run(ctx, source.SessionID, "authorized actual tool request")
			if err != nil {
				t.Fatal(err)
			}
			if !svc.WaitIdle(ctx) {
				t.Fatal("tool run did not settle")
			}
			run, err := b.GetRun(ctx, rid)
			if err != nil || run.Status != domain.RunCompleted || tool.calls != 1 {
				t.Fatalf("coding/tool capability lost: %+v calls=%d %v", run, tool.calls, err)
			}
			rows, in, err := b.ReadSubjectRunEvidence(ctx, source.SubjectHistoryBinding, rid)
			if err != nil {
				t.Fatal(err)
			}
			exact := false
			for _, m := range rows {
				if m.Role == domain.RoleTool && m.Content == tool.result {
					exact = true
				}
			}
			if !exact {
				t.Fatal("actual engine path shortened original")
			}
			visibleRef := false
			for _, msg := range recorder.lastInput() {
				if msg.Role == schema.Tool {
					encoded, _ := json.Marshal(msg)
					if (!main && strings.Contains(string(encoded), "exact tool result:")) || (main && strings.Contains(string(encoded), tool.result)) {
						visibleRef = true
					}
				}
			}
			if !visibleRef {
				t.Fatal("enhanced normalization lost the ordinary exact reference or resident complete tool output")
			}
			pin, err := b.GetRunSubjectSource(ctx, rid)
			if err != nil || pin.PolicyID != source.PolicyID {
				t.Fatalf("request strategy changed: %+v %v", pin, err)
			}
			t.Logf("manifest resident=%t %s %s", main, in.ID, in.SourceDigest)
		})
	}
}

func TestIssue58FailedReceiptNeverAdvancesModel(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	svc, b, _ := newTestService(t, testsupport.NewEchoModel())
	conformance.Issue58Identity(t, b)
	source := conformance.Issue58Source("continuation-source", false)
	mustCreateSession(t, b, source.SessionID)
	svc.deps.SubjectSources = func(context.Context, domain.SessionID) (*storage.SubjectSourceBinding, error) { return &source, nil }
	tool := &issue58ExactTool{result: "actual outcome"}
	model := NewScriptedModel(schema.AssistantMessage("", []schema.ToolCall{{ID: "continuation-call", Function: schema.FunctionCall{Name: tool.Spec().Name, Arguments: `{}`}}}), schema.AssistantMessage("must not continue", nil))
	engine, err := NewEngine(ctx, model, []tools.Tool{tool}, EngineConfig{StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10})
	if err != nil {
		t.Fatal(err)
	}
	svc.engine = engine
	svc.deps.ToolOperations = issue58FailOperations{ToolOperationStore: b}
	rid, err := svc.Run(ctx, source.SessionID, "request")
	if err != nil {
		t.Fatal(err)
	}
	if !svc.WaitIdle(ctx) {
		t.Fatal("run did not settle")
	}
	run, err := b.GetRun(ctx, rid)
	if err != nil || run.Status != domain.RunFailed || model.calls != 1 || tool.calls != 1 {
		t.Fatalf("failed receipt reached successful continuation: %+v model=%d tool=%d %v", run, model.calls, tool.calls, err)
	}
	issue58AssertExcludedOnly(t, b, source.SubjectHistoryBinding)
}

type issue58SubmittedPlan struct{ calls int }

func (t *issue58SubmittedPlan) Spec() domain.ToolSpec {
	return domain.ToolSpec{Name: tools.SubmitPlanName, Readonly: true}
}
func (t *issue58SubmittedPlan) InvokableRun(context.Context, json.RawMessage) (string, error) {
	t.calls++
	return `{"plan":{"submission_id":"exact-candidate","review_status":"pending"},"markdown":"exact candidate original"}`, nil
}
func TestIssue58PostInvokeInterruptRetainsKnownCandidate(t *testing.T) {
	ctx := context.Background()
	svc, b, _ := newTestService(t, testsupport.NewEchoModel())
	conformance.Issue58Identity(t, b)
	source := conformance.Issue58Source("interrupt-source", false)
	mustCreateSession(t, b, source.SessionID)
	svc.deps.SubjectSources = func(context.Context, domain.SessionID) (*storage.SubjectSourceBinding, error) { return &source, nil }
	a := conformance.Issue58Admission(source, "interrupt-run")
	if _, err := b.CommitRunAdmission(ctx, a); err != nil {
		t.Fatal(err)
	}
	c := svc.newToolOperationCoordinator(a.Run.ID, source.SessionID)
	args := []byte(`{}`)
	op, err := c.Admit(ctx, "interrupt-call", tools.SubmitPlanName, args, args, args)
	if err != nil {
		t.Fatal(err)
	}
	tool := &issue58SubmittedPlan{}
	adapter := newToolAdapter(tool, 4096, nil, nil, nil)
	if _, err := adapter.run(withToolOperationCoordinator(ctx, c), string(args), &op); err == nil {
		t.Fatal("Plan interrupt became ordinary successful result")
	}
	stored, err := b.GetToolOperation(ctx, a.Run.ID, op.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.State != domain.ToolOperationCompleted || stored.Failure != "" {
		t.Fatalf("known candidate stranded in claimed operation: %+v", stored)
	}
	ref, err := storage.DecodeToolEvidenceReference(stored)
	if err != nil {
		t.Fatal(err)
	}
	raw, found, err := b.Blobs().Get(ctx, ref.ID)
	if err != nil || !found {
		t.Fatalf("candidate artifact lost: %v", err)
	}
	evidence, err := storage.VerifyToolEvidence(ref, raw)
	if err != nil || !strings.Contains(evidence.Result, "exact candidate original") || evidence.Failure != "" {
		t.Fatalf("candidate substituted: %+v %v", evidence, err)
	}
	high, err := b.CaptureSubjectHistory(ctx, source.SubjectHistoryBinding)
	if err != nil || high != 0 {
		t.Fatalf("unfinished interrupted run sealed: %d %v", high, err)
	}
}

func TestIssue58RetainedMultimodalToolKeepsMedia(t *testing.T) {
	ctx := context.Background()
	svc, b, _ := newTestService(t, testsupport.NewEchoModel())
	conformance.Issue58Identity(t, b)
	source := conformance.Issue58Source("media-source", false)
	mustCreateSession(t, b, source.SessionID)
	svc.deps.SubjectSources = func(context.Context, domain.SessionID) (*storage.SubjectSourceBinding, error) { return &source, nil }
	a := conformance.Issue58Admission(source, "media-run")
	if _, err := b.CommitRunAdmission(ctx, a); err != nil {
		t.Fatal(err)
	}
	c := svc.newToolOperationCoordinator(a.Run.ID, source.SessionID)
	args := []byte(`{}`)
	op, err := c.Admit(ctx, "media-call", "envelope", args, args, args)
	if err != nil {
		t.Fatal(err)
	}
	envelope := `{"parts":[{"type":"text","text":"` + strings.Repeat("description", 10000) + `"},{"type":"image","mime_type":"image/png","base64data":"` + strings.Repeat("QUJD", 20000) + `"}]}`
	adapter := newToolAdapter(envelopeStubTool{result: envelope}, 128, nil, nil, nil)
	result, err := adapter.run(withToolOperationCoordinator(ctx, c), string(args), &op)
	if err != nil {
		t.Fatal(err)
	}
	normalized, err := normalizeEnhancedResult(result, 128, true)
	if err != nil {
		t.Fatal(err)
	}
	media := false
	reference := false
	for _, part := range normalized.Parts {
		if part.Image != nil && part.Image.Base64Data != nil && *part.Image.Base64Data == strings.Repeat("QUJD", 20000) {
			media = true
		}
		if strings.Contains(part.Text, "exact tool result:") {
			reference = true
		}
	}
	if !media || !reference {
		t.Fatalf("retention damaged existing media/tool capability: media=%t reference=%t", media, reference)
	}
}

func TestIssue58NoContextFirstDoesNotDisableTrustedSingleton(t *testing.T) {
	for _, prior := range []string{"excluded-shell", "ordinary-shell", "ordinary-chat"} {
		t.Run(prior, func(t *testing.T) {
			f := newShellFixture(t, domain.ApprovalPolicyAuto)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			conformance.Issue58Identity(t, f.backend)
			source := conformance.Issue58Source(f.sessionID, true)
			// The current trusted resolver names this singleton before its first shell.
			f.service.deps.MainConversation = issue58MainResolver{binding: cognitivecontract.MainConversationBinding{SubjectID: source.SubjectID, DomainID: source.DomainID, ConversationID: source.ConversationID, ActiveSessionID: source.SessionID, PolicyID: source.PolicyID, ToolPolicyRef: source.ToolPolicyRef, SchemaVersion: source.SchemaVersion, Grant: source.Grant}}
			var rid domain.RunID
			var err error
			if prior == "ordinary-chat" {
				f.service.deps.MainConversation = nil
				rid, err = f.service.Run(ctx, source.SessionID, "ordinary prior conversation")
			} else if prior == "ordinary-shell" {
				f.service.deps.MainConversation = nil
				rid, err = f.service.RunShell(ctx, source.SessionID, "printf 'ordinary'")
			} else {
				rid, err = f.service.RunShell(ctx, source.SessionID, "printf 'excluded'", ShellRunOptions{NoContext: true})
			}
			if err != nil {
				t.Fatal(err)
			}
			if !f.service.WaitIdle(ctx) {
				t.Fatal("prior activity did not settle")
			}
			if _, err := f.backend.GetRunSubjectSource(ctx, rid); !errors.Is(err, storage.ErrNotFound) {
				t.Fatalf("unshared prior activity acquired subject pins: %v", err)
			}
			f.service.deps.MainConversation = issue58MainResolver{binding: cognitivecontract.MainConversationBinding{SubjectID: source.SubjectID, DomainID: source.DomainID, ConversationID: source.ConversationID, ActiveSessionID: source.SessionID, PolicyID: source.PolicyID, ToolPolicyRef: source.ToolPolicyRef, SchemaVersion: source.SchemaVersion, Grant: source.Grant}}
			engine, engineErr := NewEngine(ctx, NewScriptedModel(schema.AssistantMessage("singleton response", nil)), f.service.engine.activeTools, f.service.engine.cfg)
			if engineErr != nil {
				t.Fatal(engineErr)
			}
			f.service.engine = engine
			next, err := f.service.Run(ctx, source.SessionID, "first singleton interaction")
			if prior != "excluded-shell" {
				if !errors.Is(err, storage.ErrSubjectBinding) || next != "" {
					t.Fatalf("ordinary prior activity converted to main: %s %v", next, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("excluded shell permanently disabled trusted singleton: %v", err)
			}
			if !f.service.WaitIdle(ctx) {
				t.Fatal("singleton did not settle")
			}
			high, err := f.backend.CaptureSubjectHistory(ctx, source.SubjectHistoryBinding)
			if err != nil || high != 1 {
				t.Fatalf("excluded prior shell entered C2: %d %v", high, err)
			}
		})
	}
}

func TestIssue58PendingShellCancellationRetainsOriginalRequest(t *testing.T) {
	f := newShellFixture(t, domain.ApprovalPolicyAsk)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conformance.Issue58Identity(t, f.backend)
	source := conformance.Issue58Source(f.sessionID, false)
	f.service.deps.SubjectSources = func(context.Context, domain.SessionID) (*storage.SubjectSourceBinding, error) { return &source, nil }
	command := "printf '" + strings.Repeat("original-authorized-task", 1000) + "'"
	rid, err := f.service.RunShell(ctx, source.SessionID, command)
	if err != nil {
		t.Fatal(err)
	}
	waitForShellApproval(t, f.backend, rid)
	if !f.service.Cancel(rid) {
		t.Fatal("pending shell was not cancellable")
	}
	if !f.service.WaitIdle(ctx) {
		t.Fatal("shell cancel did not settle")
	}
	rows, _, err := f.backend.ReadSubjectRunEvidence(ctx, source.SubjectHistoryBinding, rid)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Role != domain.RoleAssistant {
		t.Fatalf("pending shell invented transcript: %+v", rows)
	}
	var args map[string]string
	if err := json.Unmarshal(rows[0].ToolArgs, &args); err != nil {
		t.Fatal(err)
	}
	if args["command"] != command {
		t.Fatal("pending shell lost exact original request during protected-state cleanup")
	}
}

func TestIssue58CompletedReceiptRejectsMissingOrCorruptArtifacts(t *testing.T) {
	ctx := context.Background()
	svc, b, _ := newTestService(t, testsupport.NewEchoModel())
	conformance.Issue58Identity(t, b)
	source := conformance.Issue58Source("replay-source", false)
	mustCreateSession(t, b, source.SessionID)
	svc.deps.SubjectSources = func(context.Context, domain.SessionID) (*storage.SubjectSourceBinding, error) { return &source, nil }
	a := conformance.Issue58Admission(source, "replay-run")
	if _, err := b.CommitRunAdmission(ctx, a); err != nil {
		t.Fatal(err)
	}
	c := svc.newToolOperationCoordinator(a.Run.ID, source.SessionID)
	args := []byte(`{}`)
	op, err := c.Admit(ctx, "replay-call", "issue58_exact", args, args, args)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	if _, err := c.Execute(ctx, op, func(context.Context) (string, error) { calls++; return "original result", nil }); err != nil {
		t.Fatal(err)
	}
	stored, err := b.GetToolOperation(ctx, a.Run.ID, op.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := storage.DecodeToolEvidenceReference(stored)
	if err != nil {
		t.Fatal(err)
	}
	raw, _, err := b.Blobs().Get(ctx, ref.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"result-missing", "result-corrupt", "arguments-missing"} {
		t.Run(fault, func(t *testing.T) {
			if err := b.Blobs().Put(ctx, ref.ID, raw); err != nil {
				t.Fatal(err)
			}
			if err := b.Blobs().Put(ctx, storage.ToolEvidenceArgumentsID(op), args); err != nil {
				t.Fatal(err)
			}
			switch fault {
			case "result-missing":
				err = b.Blobs().Delete(ctx, ref.ID)
			case "result-corrupt":
				err = b.Blobs().Put(ctx, ref.ID, []byte("corrupted"))
			case "arguments-missing":
				err = b.Blobs().Delete(ctx, storage.ToolEvidenceArgumentsID(op))
			}
			if err != nil {
				t.Fatal(err)
			}
			result, err := c.Execute(ctx, op, func(context.Context) (string, error) { calls++; return "reexecution", nil })
			if !errors.Is(err, ErrToolOperationUnknown) || result != "" || calls != 1 {
				t.Fatalf("completed receipt lost artifact but continued/reexecuted: %q calls=%d %v", result, calls, err)
			}
		})
	}
}

// issue58AssertExcludedOnly checks the run was sealed as the subject's only
// interaction, as an excluded one that exposes no originals.
func issue58AssertExcludedOnly(t *testing.T, b storage.SubjectSourceStore, binding storage.SubjectHistoryBinding) {
	t.Helper()
	ctx := context.Background()
	high, err := b.CaptureSubjectHistory(ctx, binding)
	if err != nil || high != 1 {
		t.Fatalf("unresolvable run was not sealed once: %d %v", high, err)
	}
	rows, err := b.ListSubjectInteractions(ctx, binding, 0, high, 10)
	if err != nil || len(rows) != 1 || rows[0].Disposition != storage.SubjectInteractionExcluded {
		t.Fatalf("run not sealed as excluded: %+v %v", rows, err)
	}
	originals, err := b.ReadSubjectInteraction(ctx, binding, rows[0].ID)
	if err != nil || len(originals) != 0 {
		t.Fatalf("excluded interaction exposed originals: %v %v", originals, err)
	}
}
