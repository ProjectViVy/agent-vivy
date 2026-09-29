package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	inofy "github.com/ProjectViVy/inofy"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/testsupport"
	"agent-vivy/internal/tools"
)

func inofyTestDigest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func inofyTestInputDigest() string {
	return "inofy-normal-v1:sha256:" + inofyTestDigest("{}")
}

// prepareINOFYExecParent admits a schema-2 workflow revision plus its graph
// Run, marks it active and registers in-memory authority — the same state a
// live INOFY program holds while its executor boundary is invoked.
func prepareINOFYExecParent(t *testing.T, svc *Service, backend *sqlite.Backend, sessionID domain.SessionID, parentRunID domain.RunID, depth int, selected []string) (domain.RunID, inofy.ExecutionRef) {
	t.Helper()
	ctx := context.Background()
	if err := backend.CreateSession(ctx, domain.Session{ID: sessionID, Title: "inofy", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := backend.CreateRun(ctx, domain.Run{ID: parentRunID, SessionID: sessionID, Status: domain.RunActive,
		CreatedAt: 1, Kind: domain.RunKindPrimary, RootID: parentRunID, Depth: depth - 1}); err != nil {
		t.Fatal(err)
	}
	wfID := domain.RunID("wf-" + string(parentRunID))
	descriptorJSON := []byte(`{"definition":1}`)
	authorityJSON := []byte(`{"authority":1}`)
	revision := domain.WorkflowRevision{
		RunID: wfID, ParentRunID: parentRunID, ParentSessionID: sessionID, RootRunID: parentRunID,
		OperationKey:     "wf-op-" + string(parentRunID),
		DescriptorDigest: inofyTestDigest(string(descriptorJSON)), AuthorityDigest: inofyTestDigest(string(authorityJSON)),
		DescriptorJSON: descriptorJSON, AuthorityJSON: authorityJSON,
		SchemaVersion: 2, CreatedAt: 2,
		ProgramDigest: inofyTestDigest("program-" + string(parentRunID)), CatalogDigest: inofyTestDigest("catalog"),
		CompilerVersion: "inofy@6acfcc6", EinoBuild: "v0.9.13",
		InputDigest:     inofyTestDigest("input-" + string(parentRunID)),
		InputJSON:       []byte(`{}`),
		EffectiveLimits: []byte(`{"max_nodes":12,"max_attempts":1}`),
		HostBindingID:   inofyTestDigest("binding-" + string(parentRunID)),
	}
	if _, err := backend.CommitWorkflowAdmission(ctx, storage.WorkflowAdmission{
		Revision: revision,
		Run: domain.Run{ID: wfID, SessionID: sessionID, Status: domain.RunAccepted, Kind: domain.RunKindWorkflow,
			ParentID: parentRunID, RootID: parentRunID, Depth: depth, CreatedAt: 2},
		Started: domain.RunEvent{RunID: wfID, Type: domain.EventRunStarted, CreatedAt: 2, PayloadVersion: 1,
			Payload: []byte(`{"mode":"test"}`)},
	}); err != nil {
		t.Fatalf("admit workflow revision: %v", err)
	}
	if err := backend.SetRunStatus(ctx, wfID, domain.RunActive); err != nil {
		t.Fatal(err)
	}
	snapshot, err := svc.engine.cfg.Policy.Snapshot(domain.PolicyProfileDefault)
	if err != nil {
		t.Fatal(err)
	}
	ledger, err := NewBudgetLedger(svc.deps.Budget)
	if err != nil {
		t.Fatal(err)
	}
	svc.mu.Lock()
	svc.snapshots[wfID] = snapshot
	svc.ledgers[wfID] = ledger
	svc.runTools[wfID] = childToolSet(selected)
	svc.runSessions[wfID] = sessionID
	svc.mu.Unlock()
	return wfID, inofy.ExecutionRef{
		RunID:         string(wfID),
		Epoch:         1,
		ProgramDigest: revision.ProgramDigest,
		HostBindingID: revision.HostBindingID,
	}
}

func inofyExecService(t *testing.T, model domain.ChatModel) (*Service, *sqlite.Backend) {
	t.Helper()
	svc, backend, _ := newTestService(t, model)
	workspaces, err := NewSessionWorkspaceManager(t.TempDir(), backend, backend)
	if err != nil {
		t.Fatal(err)
	}
	svc.deps.Workspaces = workspaces
	t.Cleanup(func() { svc.CancelAll(); svc.WaitIdle(context.Background()) })
	return svc, backend
}

func inofyNodeCall(ref inofy.ExecutionRef, path, task string, input json.RawMessage, toolNames ...string) inofy.NodeCall {
	config, _ := json.Marshal(map[string]any{"task": task, "tool_names": toolNames})
	return inofy.NodeCall{
		Ref:              ref,
		Path:             path,
		TypeID:           workflowChildType,
		ImplementationID: workflowChildType,
		Config:           config,
		Input:            input,
		OperationKey:     ref.RunID + "/" + path,
		Attempt:          1,
	}
}

func inofyReplyOutput(t *testing.T, reply inofy.NodeReply) string {
	t.Helper()
	var out struct {
		Result string `json:"result"`
	}
	if err := json.Unmarshal(reply.Output, &out); err != nil {
		t.Fatalf("node reply output is not bounded JSON: %v", err)
	}
	return out.Result
}

func TestINOFYChildExecutorRunsStableGovernedChild(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, testsupport.NewEchoModel())
	wfID, ref := prepareINOFYExecParent(t, svc, backend, "sess-inofy-exec", "run-inofy-exec-parent", 1, []string{tools.EchoInfoName})
	exec := newINOFYNodeExecutor(svc)

	call := inofyNodeCall(ref, "root/draft", "draft a bounded result", json.RawMessage(`{}`))
	reply, err := exec.Execute(ctx, call)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if reply.Wait != nil || strings.TrimSpace(inofyReplyOutput(t, reply)) == "" {
		t.Fatalf("reply = %+v", reply)
	}
	if got := inofyReplyOutput(t, reply); !strings.Contains(got, "draft a bounded result") {
		t.Fatalf("output = %q", got)
	}
	wantChild := inofyChildRunID(call.OperationKey)
	child, err := backend.GetRun(ctx, wantChild)
	if err != nil {
		t.Fatalf("deterministic child run missing: %v", err)
	}
	if child.ParentID != wfID || child.Kind != domain.RunKindChild || child.EffectiveChildMode() != domain.ChildModeOneShot ||
		child.Depth != 2 || child.Status != domain.RunCompleted {
		t.Fatalf("child run = %+v", child)
	}
	// A repeated identical effect joins the durable child instead of
	// admitting a second activation.
	again, err := exec.Execute(ctx, call)
	if err != nil {
		t.Fatalf("repeat execute: %v", err)
	}
	if inofyReplyOutput(t, again) != inofyReplyOutput(t, reply) {
		t.Fatal("repeated execute returned a different output")
	}
	children, err := backend.ListChildRuns(ctx, wfID)
	if err != nil || len(children) != 1 {
		t.Fatalf("child runs = %+v err=%v", children, err)
	}
}

func TestINOFYChildExecutorParallelFanOutBindsOutputs(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, testsupport.NewEchoModel())
	wfID, ref := prepareINOFYExecParent(t, svc, backend, "sess-inofy-fan", "run-inofy-fan-parent", 1, []string{tools.EchoInfoName})
	exec := newINOFYNodeExecutor(svc)

	var wg sync.WaitGroup
	replies := make([]inofy.NodeReply, 2)
	errs := make([]error, 2)
	for i, path := range []string{"root/a", "root/b"} {
		wg.Add(1)
		go func(i int, path string) {
			defer wg.Done()
			replies[i], errs[i] = exec.Execute(ctx, inofyNodeCall(ref, path, "branch "+path, json.RawMessage(`{}`)))
		}(i, path)
	}
	wg.Wait()
	for i := range errs {
		if errs[i] != nil {
			t.Fatalf("fan-out execute %d: %v", i, errs[i])
		}
	}
	children, err := backend.ListChildRuns(ctx, wfID)
	if err != nil || len(children) != 2 {
		t.Fatalf("children = %+v err=%v", children, err)
	}
	if children[0].ID == children[1].ID {
		t.Fatal("distinct operations share a child run id")
	}
	// A dependent node binds upstream output only as explicitly labelled
	// untrusted data inside the child task.
	input, _ := json.Marshal(map[string]string{"first": inofyReplyOutput(t, replies[0])})
	joined, err := exec.Execute(ctx, inofyNodeCall(ref, "root/join", "combine", input))
	if err != nil {
		t.Fatalf("dependent execute: %v", err)
	}
	joinedChild := inofyChildRunID(ref.RunID + "/root/join")
	recorded := replayAll(t, backend, joinedChild)
	var task string
	for _, event := range recorded {
		if event.Type != domain.EventChildRequested {
			continue
		}
		var payload struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			t.Fatal(err)
		}
		task = payload.Text
	}
	if !strings.Contains(task, "untrusted") || !strings.Contains(task, "first") {
		t.Fatalf("child task does not bind predecessor output as untrusted: %q", task)
	}
	if out := inofyReplyOutput(t, joined); !strings.Contains(out, "combine") {
		t.Fatalf("joined output = %q", out)
	}
}

func TestINOFYChildExecutorConflictingOperationInput(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, testsupport.NewEchoModel())
	_, ref := prepareINOFYExecParent(t, svc, backend, "sess-inofy-conflict", "run-inofy-conflict-parent", 1, []string{tools.EchoInfoName})
	exec := newINOFYNodeExecutor(svc)

	call := inofyNodeCall(ref, "root/draft", "first task", json.RawMessage(`{"seed":"one"}`))
	if _, err := exec.Execute(ctx, call); err != nil {
		t.Fatalf("execute: %v", err)
	}
	changed := inofyNodeCall(ref, "root/draft", "first task", json.RawMessage(`{"seed":"two"}`))
	if _, err := exec.Execute(ctx, changed); !errors.Is(err, storage.ErrChildAdmissionConflict) {
		t.Fatalf("changed input retry = %v, want admission conflict", err)
	}
	var unknown *inofy.UnknownOutcomeError
	if _, err := exec.Execute(ctx, changed); errors.As(err, &unknown) {
		t.Fatal("changed digest must conflict, never report an unknown outcome")
	}
}

func TestINOFYChildExecutorRejectsUntrustedCallShape(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, testsupport.NewEchoModel())
	_, ref := prepareINOFYExecParent(t, svc, backend, "sess-inofy-shape", "run-inofy-shape-parent", 1, []string{tools.EchoInfoName})
	exec := newINOFYNodeExecutor(svc)

	wrongType := inofyNodeCall(ref, "root/x", "task", nil)
	wrongType.TypeID = "vivy.child-task@2"
	if _, err := exec.Execute(ctx, wrongType); err == nil {
		t.Fatal("untrusted type id was executed")
	}
	wrongImpl := inofyNodeCall(ref, "root/x", "task", nil)
	wrongImpl.ImplementationID = "vivy.child-task@2"
	if _, err := exec.Execute(ctx, wrongImpl); err == nil {
		t.Fatal("untrusted implementation id was executed")
	}
	missingKey := inofyNodeCall(ref, "root/x", "task", nil)
	missingKey.OperationKey = ""
	if _, err := exec.Execute(ctx, missingKey); err == nil {
		t.Fatal("operation-less call was executed")
	}
	badConfig := inofyNodeCall(ref, "root/x", "task", nil)
	badConfig.Config = json.RawMessage(`{"task":""}`)
	if _, err := exec.Execute(ctx, badConfig); err == nil {
		t.Fatal("empty task config was executed")
	}
}

func TestINOFYChildExecutorRechecksToolCeiling(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, testsupport.NewEchoModel())
	_, ref := prepareINOFYExecParent(t, svc, backend, "sess-inofy-tools", "run-inofy-tools-parent", 1, []string{tools.EchoInfoName})
	exec := newINOFYNodeExecutor(svc)

	widening := inofyNodeCall(ref, "root/draft", "task", nil, tools.WriteNoteName)
	if _, err := exec.Execute(ctx, widening); err == nil {
		t.Fatal("tool widening past parent ceiling was admitted")
	}
	narrow, err := exec.Execute(ctx, inofyNodeCall(ref, "root/safe", "task", nil, tools.EchoInfoName))
	if err != nil {
		t.Fatalf("narrowed execute: %v", err)
	}
	if out := inofyReplyOutput(t, narrow); !strings.Contains(out, "task") {
		t.Fatalf("output = %q", out)
	}
}

func TestINOFYChildExecutorDepthCeiling(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, testsupport.NewEchoModel())
	_, ref := prepareINOFYExecParent(t, svc, backend, "sess-inofy-depth", "run-inofy-depth-parent", maxChildSessionDepth, []string{tools.EchoInfoName})
	exec := newINOFYNodeExecutor(svc)
	if _, err := exec.Execute(ctx, inofyNodeCall(ref, "root/deep", "task", nil)); err == nil {
		t.Fatal("child admitted beyond the session depth ceiling")
	}
}

func TestINOFYChildExecutorProgramBinding(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, testsupport.NewEchoModel())
	_, ref := prepareINOFYExecParent(t, svc, backend, "sess-inofy-bind", "run-inofy-bind-parent", 1, []string{tools.EchoInfoName})
	exec := newINOFYNodeExecutor(svc)

	forged := ref
	forged.ProgramDigest = inofyTestDigest("forged")
	if _, err := exec.Execute(ctx, inofyNodeCall(forged, "root/x", "task", nil)); err == nil {
		t.Fatal("call under a foreign program digest was executed")
	}
	forged = ref
	forged.HostBindingID = inofyTestDigest("forged-binding")
	if _, err := exec.Execute(ctx, inofyNodeCall(forged, "root/x", "task", nil)); err == nil {
		t.Fatal("call under a foreign host binding was executed")
	}
}

func TestINOFYChildExecutorTerminalOutcomeIsDurable(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, testsupport.NewEchoModel())
	_, ref := prepareINOFYExecParent(t, svc, backend, "sess-inofy-term", "run-inofy-term-parent", 1, []string{tools.EchoInfoName})
	exec := newINOFYNodeExecutor(svc)

	call := inofyNodeCall(ref, "root/draft", "finish now", nil)
	reply, err := exec.Execute(ctx, call)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	// Rebuild the executor as after a process restart: durable journal and
	// run state, no in-memory children. The same call must return the same
	// committed output without invoking the model again.
	restartExec := newINOFYNodeExecutor(svc)
	again, err := restartExec.Execute(ctx, call)
	if err != nil {
		t.Fatalf("restart execute: %v", err)
	}
	if inofyReplyOutput(t, again) != inofyReplyOutput(t, reply) {
		t.Fatal("durable terminal outcome changed across restart")
	}
}

func TestINOFYChildExecutorUnknownOutcomeAfterAdmissionLoss(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, testsupport.NewEchoModel())
	wfID, ref := prepareINOFYExecParent(t, svc, backend, "sess-inofy-orphan", "run-inofy-orphan-parent", 1, []string{tools.EchoInfoName})
	exec := newINOFYNodeExecutor(svc)

	call := inofyNodeCall(ref, "root/lost", "orphaned task", nil)
	childID := inofyChildRunID(call.OperationKey)
	task, err := inofyChildNodeTask("orphaned task", call.Input)
	if err != nil {
		t.Fatal(err)
	}
	child := domain.Run{ID: childID, SessionID: "sess-inofy-orphan", Status: domain.RunActive,
		CreatedAt: time.Now().UnixMilli(), Kind: domain.RunKindChild, ChildMode: domain.ChildModeOneShot,
		ParentID: wfID, RootID: wfID, Depth: 2}
	if err := backend.CreateRun(ctx, child); err != nil {
		t.Fatal(err)
	}
	if _, err := backend.Append(ctx, storage.Commit{RunID: childID, Events: []domain.RunEvent{
		{Type: domain.EventChildRequested, CreatedAt: child.CreatedAt, PayloadVersion: 1,
			Payload: []byte(`{"parent_run_id":"` + string(wfID) + `","depth":2,"text":` + mustJSONString(t, task) + `}`)},
		{Type: domain.EventChildStarted, CreatedAt: child.CreatedAt + 1, PayloadVersion: 1,
			Payload: []byte(`{"parent_run_id":"` + string(wfID) + `","workspace_id":"ws"}`)},
	}}); err != nil {
		t.Fatal(err)
	}
	// The child claims to be running but no live process owns it: the
	// executor must refuse the effect rather than invoke the model again.
	var unknown *inofy.UnknownOutcomeError
	_, err = exec.Execute(ctx, call)
	if !errors.As(err, &unknown) {
		t.Fatalf("orphaned effect retry = %v, want UnknownOutcomeError", err)
	}
}

func TestINOFYChildExecutorChildFailureIsTerminalError(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, errorModel{})
	_, ref := prepareINOFYExecParent(t, svc, backend, "sess-inofy-fail", "run-inofy-fail-parent", 1, []string{tools.EchoInfoName})
	exec := newINOFYNodeExecutor(svc)

	_, err := exec.Execute(ctx, inofyNodeCall(ref, "root/boom", "explode", nil))
	if err == nil {
		t.Fatal("failed child returned a successful reply")
	}
	var unknown *inofy.UnknownOutcomeError
	if errors.As(err, &unknown) {
		t.Fatal("terminal child failure must be a definite error, not unknown")
	}
	child, getErr := backend.GetRun(ctx, inofyChildRunID(ref.RunID+"/root/boom"))
	if getErr != nil || child.Status != domain.RunFailed {
		t.Fatalf("child status = %+v err=%v", child, getErr)
	}
}

func TestINOFYChildExecutorCancelPropagatesToChild(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	svc, backend := inofyExecService(t, blockingModel{})
	_, ref := prepareINOFYExecParent(t, svc, backend, "sess-inofy-cancel", "run-inofy-cancel-parent", 1, []string{tools.EchoInfoName})
	exec := newINOFYNodeExecutor(svc)

	call := inofyNodeCall(ref, "root/slow", "wait", nil)
	childID := inofyChildRunID(call.OperationKey)
	done := make(chan error, 1)
	go func() {
		_, err := exec.Execute(ctx, call)
		done <- err
	}()
	deadline := time.Now().Add(10 * time.Second)
	for {
		child, err := backend.GetRun(ctx, childID)
		if err == nil && child.Status == domain.RunActive {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("child never reached active: %+v err=%v", child, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	err := <-done
	if err == nil {
		t.Fatal("cancelled execute returned success")
	}
	waitForRunStatus(t, backend, childID, domain.RunCancelled)
}

// TestINOFYProgramRunCommitsThroughCoreStorage is the cutover seam smoke
// test: a real INOFY program executes against the atomic two-driver
// RunStore and the governed child executor, end to end.
func TestINOFYProgramRunCommitsThroughCoreStorage(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, testsupport.NewEchoModel())
	sessionID := domain.SessionID("sess-inofy-e2e")
	parentRunID := domain.RunID("run-inofy-e2e-parent")
	if err := backend.CreateSession(ctx, domain.Session{ID: sessionID, Title: "e2e", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := backend.CreateRun(ctx, domain.Run{ID: parentRunID, SessionID: sessionID, Status: domain.RunActive,
		CreatedAt: 1, Kind: domain.RunKindPrimary, RootID: parentRunID}); err != nil {
		t.Fatal(err)
	}
	raw := json.RawMessage(`{"schema_version":"inofy.workflow/v1","graph":{"nodes":[
		{"id":"a","kind":"call","type":"vivy.child-task@1","config":{"task":"first half"}},
		{"id":"join","kind":"call","type":"vivy.child-task@1","config":{"task":"combine"},"inputs":{"first":{"source":"a","pointer":"/result"}}}
	],"edges":[{"from":"a","to":"join"}],"exits":["join"],"outputs":{"answer":{"source":"join","pointer":"/result"}}}}`)
	admitted, err := validateINOFYDefinition(ctx, raw, []string{tools.EchoInfoName})
	if err != nil {
		t.Fatalf("validate definition: %v", err)
	}
	wfID := domain.RunID("run-inofy-e2e-wf")
	requestLimits := inofy.Limits{
		MaxNodes: 12, MaxEdges: 24, Parallelism: 4, MaxActivations: 12,
		MaxAttemptsPerCall: 1, MaxDefinitionBytes: 64 << 10,
		MaxNodeInputBytes: 8 << 10, MaxNodeOutputBytes: 8 << 10, MaxOutputBytesTotal: 8 << 10,
		MaxRepeatNesting: 1, MaxIterations: 8, MaxPredicateDepth: 8,
		NodeTimeoutMS: 60_000, RunTimeoutMS: 600_000,
		MaxCheckpointBytes:   16 << 20,
		MaxConcurrentRuns:    4,
		MaxPendingAdmissions: 32,
	}
	limitsJSON, err := json.Marshal(requestLimits)
	if err != nil {
		t.Fatal(err)
	}
	inputDigest := "inofy-normal-v1:sha256:" + inofyTestDigest("{}")
	hostBinding := inofyTestDigest("host-binding-e2e")
	revision := domain.WorkflowRevision{
		RunID: wfID, ParentRunID: parentRunID, ParentSessionID: sessionID, RootRunID: parentRunID,
		OperationKey:     "wf-e2e",
		DescriptorDigest: inofyTestDigest(string(admitted.CanonicalJSON)),
		AuthorityDigest:  inofyTestDigest(`{"authority":1}`),
		DescriptorJSON:   admitted.CanonicalJSON, AuthorityJSON: []byte(`{"authority":1}`),
		SchemaVersion: 2, CreatedAt: 2,
		ProgramDigest:   admitted.Meta.ProgramDigest,
		CatalogDigest:   admitted.Meta.CatalogDigest,
		CompilerVersion: admitted.Meta.CompilerVersion, EinoBuild: admitted.Meta.EinoBuild,
		InputDigest: inputDigest, InputJSON: []byte(`{}`), EffectiveLimits: limitsJSON,
		HostBindingID: hostBinding,
	}
	if _, err := backend.CommitWorkflowAdmission(ctx, storage.WorkflowAdmission{
		Revision: revision,
		Run: domain.Run{ID: wfID, SessionID: sessionID, Status: domain.RunAccepted, Kind: domain.RunKindWorkflow,
			ParentID: parentRunID, RootID: parentRunID, Depth: 1, CreatedAt: 2},
		Started: domain.RunEvent{RunID: wfID, Type: domain.EventRunStarted, CreatedAt: 2, PayloadVersion: 1,
			Payload: []byte(`{"mode":"test"}`)},
	}); err != nil {
		t.Fatalf("admit revision: %v", err)
	}
	if err := backend.SetRunStatus(ctx, wfID, domain.RunActive); err != nil {
		t.Fatal(err)
	}
	snapshot, err := svc.engine.cfg.Policy.Snapshot(domain.PolicyProfileDefault)
	if err != nil {
		t.Fatal(err)
	}
	ledger, err := NewBudgetLedger(svc.deps.Budget)
	if err != nil {
		t.Fatal(err)
	}
	svc.mu.Lock()
	svc.snapshots[wfID] = snapshot
	svc.ledgers[wfID] = ledger
	svc.runTools[wfID] = childToolSet([]string{tools.EchoInfoName})
	svc.runSessions[wfID] = sessionID
	svc.mu.Unlock()

	result, err := admitted.Program.Run(ctx, inofy.RunRequest{
		Ref:    inofy.ExecutionRef{RunID: string(wfID), Epoch: 1, ProgramDigest: admitted.Meta.ProgramDigest, HostBindingID: hostBinding},
		Input:  json.RawMessage(`{}`),
		Limits: requestLimits,
	}, inofy.Bindings{Nodes: newINOFYNodeExecutor(svc), Runs: newINOFYRunStore(backend)})
	if err != nil {
		t.Fatalf("program run: %v", err)
	}
	if result.Status != inofy.RunSucceeded {
		t.Fatalf("run status = %v diagnostics=%+v", result.Status, result.Diagnostics)
	}
	var outputs struct {
		Answer string `json:"answer"`
	}
	if err := json.Unmarshal(result.Outputs, &outputs); err != nil || !strings.Contains(outputs.Answer, "combine") {
		t.Fatalf("outputs = %s err=%v", string(result.Outputs), err)
	}
	run, err := backend.GetRun(ctx, wfID)
	if err != nil || run.Status != domain.RunCompleted {
		t.Fatalf("workflow run status = %+v err=%v", run, err)
	}
	children, err := backend.ListChildRuns(ctx, wfID)
	if err != nil || len(children) != 2 {
		t.Fatalf("governed children = %+v err=%v", children, err)
	}
	for _, child := range children {
		if child.Status != domain.RunCompleted || !strings.HasPrefix(string(child.ID), "workflow_child_") {
			t.Fatalf("child = %+v", child)
		}
	}
	seen := map[domain.EventType]bool{}
	for _, event := range replayAll(t, backend, wfID) {
		seen[event.Type] = true
	}
	for _, typ := range []domain.EventType{domain.EventWorkflowAdmitted, domain.EventWorkflowStarted,
		domain.EventWorkflowNodeStarted, domain.EventWorkflowNodeAttempt, domain.EventWorkflowNodeCompleted,
		domain.EventRunCompleted} {
		if !seen[typ] {
			t.Fatalf("journal missing %s: %+v", typ, seen)
		}
	}
}

func mustJSONString(t *testing.T, value string) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
