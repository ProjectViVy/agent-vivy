package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	inofy "github.com/ProjectViVy/inofy"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/testsupport"
	"agent-vivy/internal/tools"
)

const inofyTwoNodeDefinition = `{"schema_version":"inofy.workflow/v1","graph":{"nodes":[
	{"id":"a","kind":"call","type":"vivy.child-task@1","config":{"task":"first half"}},
	{"id":"join","kind":"call","type":"vivy.child-task@1","config":{"task":"combine halves"},"inputs":{"first":{"source":"a","pointer":"/result"}}}
],"edges":[{"from":"a","to":"join"}],"exits":["join"],"outputs":{"answer":{"source":"join","pointer":"/result"}}}}`

// TestINOFYWorkflowAdmitRunInspect drives the real production route: admit an
// INOFY definition, run it to a committed terminal through governed children,
// and inspect the committed projection.
func TestINOFYWorkflowAdmitRunInspect(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, testsupport.NewEchoModel())
	sessionID := domain.SessionID("sess-wf-e2e")
	parentRunID := domain.RunID("run-wf-e2e-parent")
	prepareChildSessionAuthorizer(t, svc, backend, sessionID, parentRunID, []string{tools.EchoInfoName})

	proposal, err := svc.ProposeINOFYWorkflow(ctx, parentRunID, json.RawMessage(inofyTwoNodeDefinition))
	if err != nil || proposal.Digest == "" {
		t.Fatalf("propose: %v digest=%q", err, proposal.Digest)
	}
	started, err := svc.StartINOFYWorkflow(ctx, parentRunID, "wf-op-admit", json.RawMessage(inofyTwoNodeDefinition))
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if !started.Created || started.Run.Kind != domain.RunKindWorkflow || started.Revision.SchemaVersion != 2 {
		t.Fatalf("start result = %+v", started)
	}
	waitForRunStatus(t, backend, started.Run.ID, domain.RunCompleted)

	details, err := svc.GetWorkflow(ctx, started.Run.ID)
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if len(details.Definition) == 0 || !json.Valid(details.Definition) {
		t.Fatalf("stored definition missing: %+v", details)
	}
	if details.EngineStatus != string(inofy.RunSucceeded) {
		t.Fatalf("engine status = %q", details.EngineStatus)
	}
	if len(details.Nodes) != 2 {
		t.Fatalf("node projection = %+v", details.Nodes)
	}
	for _, node := range details.Nodes {
		if node.Status != "completed" || !strings.HasPrefix(node.ChildRunID, "workflow_child_") || node.ResultDigest == "" {
			t.Fatalf("node = %+v", node)
		}
	}
	if got := details.Outputs["answer"]; !strings.Contains(got, "combine halves") {
		t.Fatalf("outputs = %+v", details.Outputs)
	}
	listed, err := svc.ListWorkflows(ctx, parentRunID)
	if err != nil || len(listed) != 1 || listed[0].Run.ID != started.Run.ID {
		t.Fatalf("list = %+v err=%v", listed, err)
	}
	terminal := 0
	for _, event := range replayAll(t, backend, started.Run.ID) {
		if event.Type == domain.EventRunCompleted || event.Type == domain.EventRunFailed || event.Type == domain.EventRunCancelled {
			terminal++
		}
	}
	if terminal != 1 {
		t.Fatalf("terminal events = %d", terminal)
	}
}

// TestINOFYWorkflowOperationDedup covers join-by-operation and identity
// conflict for the same operation key.
func TestINOFYWorkflowOperationDedup(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, testsupport.NewEchoModel())
	sessionID := domain.SessionID("sess-wf-dedup")
	parentRunID := domain.RunID("run-wf-dedup-parent")
	prepareChildSessionAuthorizer(t, svc, backend, sessionID, parentRunID, []string{tools.EchoInfoName})

	started, err := svc.StartINOFYWorkflow(ctx, parentRunID, "wf-op-dedup", json.RawMessage(inofyTwoNodeDefinition))
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	waitForRunStatus(t, backend, started.Run.ID, domain.RunCompleted)
	again, err := svc.StartINOFYWorkflow(ctx, parentRunID, "wf-op-dedup", json.RawMessage(inofyTwoNodeDefinition))
	if err != nil {
		t.Fatalf("retry same operation: %v", err)
	}
	if again.Created || again.Run.ID != started.Run.ID {
		t.Fatalf("dedup result = %+v", again)
	}
	changed := strings.Replace(inofyTwoNodeDefinition, "first half", "changed task", 1)
	if _, err := svc.StartINOFYWorkflow(ctx, parentRunID, "wf-op-dedup", json.RawMessage(changed)); err == nil {
		t.Fatal("changed definition under the same operation key was admitted")
	}
}

// TestINOFYWorkflowRejectsLegacyAndMalformed makes old first-party payloads
// and malformed definitions fail clearly at admission.
func TestINOFYWorkflowRejectsLegacyAndMalformed(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, testsupport.NewEchoModel())
	sessionID := domain.SessionID("sess-wf-legacy")
	parentRunID := domain.RunID("run-wf-legacy-parent")
	prepareChildSessionAuthorizer(t, svc, backend, sessionID, parentRunID, []string{tools.EchoInfoName})

	legacy := `{"schema_version":1,"start_nodes":["a"],"nodes":[{"key":"a","task":"x"}],"edges":[],"outputs":["a"]}`
	if _, err := svc.StartINOFYWorkflow(ctx, parentRunID, "wf-op-legacy", json.RawMessage(legacy)); !errors.Is(err, ErrINOFYInvalidDefinition) {
		t.Fatalf("legacy descriptor error = %v", err)
	}
	if _, err := svc.StartINOFYWorkflow(ctx, parentRunID, "wf-op-bad", json.RawMessage(`{`)); err == nil {
		t.Fatal("malformed definition admitted")
	}
	if _, err := svc.StartINOFYWorkflow(ctx, parentRunID, "wf-op-bad2", nil); err == nil {
		t.Fatal("empty definition admitted")
	}
}

// TestINOFYWorkflowCancelPropagates cancels a running workflow and asserts the
// graph-owned child and the committed terminal stay consistent.
func TestINOFYWorkflowCancelPropagates(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, blockingModel{})
	sessionID := domain.SessionID("sess-wf-cancel")
	parentRunID := domain.RunID("run-wf-cancel-parent")
	prepareChildSessionAuthorizer(t, svc, backend, sessionID, parentRunID, []string{tools.EchoInfoName})

	started, err := svc.StartINOFYWorkflow(ctx, parentRunID, "wf-op-cancel", json.RawMessage(inofyTwoNodeDefinition))
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	var child domain.Run
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		children, _ := backend.ListChildRuns(ctx, started.Run.ID)
		if len(children) > 0 {
			child = children[0]
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if child.ID == "" {
		t.Fatal("no governed child was admitted")
	}
	if _, err := svc.CancelWorkflow(ctx, started.Run.ID); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	waitForRunStatus(t, backend, child.ID, domain.RunCancelled)
	// Cancellation can race the durable child acknowledgement. Assert the
	// native and engine projections against the actual unresolved ledger:
	// known cancellation is terminal; an uncertain effect requires recovery.
	var details WorkflowDetails
	deadline = time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		details, err = svc.GetWorkflow(ctx, started.Run.ID)
		if err != nil {
			t.Fatalf("inspect cancelled: %v", err)
		}
		if details.EngineStatus == string(inofy.RunRecoveryRequired) || details.EngineStatus == string(inofy.RunCancelled) {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	engine, err := svc.inofyEngine()
	if err != nil {
		t.Fatal(err)
	}
	state, err := engine.LoadWorkflowStep(ctx, started.Run.ID)
	if err != nil || state.Projection == nil {
		t.Fatalf("missing durable cancellation projection: %v", err)
	}
	var unresolved []json.RawMessage
	if len(state.Projection.UnresolvedJSON) > 0 {
		if err := json.Unmarshal(state.Projection.UnresolvedJSON, &unresolved); err != nil {
			t.Fatal(err)
		}
	}
	if details.EngineStatus == string(inofy.RunCancelled) {
		waitForRunStatus(t, backend, started.Run.ID, domain.RunCancelled)
		if len(unresolved) != 0 {
			t.Fatal("terminal cancellation fabricated success over unresolved effects")
		}
		replayed, err := svc.StartINOFYWorkflow(ctx, parentRunID, "wf-op-cancel", json.RawMessage(inofyTwoNodeDefinition))
		if err != nil || replayed.Created || replayed.Run.ID != started.Run.ID {
			t.Fatalf("known cancellation was re-executed: %+v %v", replayed, err)
		}
	} else {
		run, err := backend.GetRun(ctx, started.Run.ID)
		if details.EngineStatus != string(inofy.RunRecoveryRequired) || err != nil || run.Status.Terminal() || len(unresolved) == 0 {
			t.Fatalf("uncertain cancellation lost its recovery state: engine=%s run=%+v unresolved=%d err=%v", details.EngineStatus, run, len(unresolved), err)
		}
		if _, err := svc.StartINOFYWorkflow(ctx, parentRunID, "wf-op-cancel", json.RawMessage(inofyTwoNodeDefinition)); !errors.Is(err, ErrWorkflowRecoveryRequired) {
			t.Fatalf("duplicate start on interrupted run = %v", err)
		}
	}
}

// admitINOFYWorkflowFixture commits a real schema-2 revision and its workflow
// Run exactly as the production route would, without launching execution.
func admitINOFYWorkflowFixture(t *testing.T, svc *Service, backend *sqlite.Backend, sessionID domain.SessionID,
	parentRunID domain.RunID, operationKey string, raw json.RawMessage, toolsAllowed []string) (domain.Run, domain.WorkflowRevision, inofy.ExecutionRef) {
	t.Helper()
	ctx := context.Background()
	admitted, err := validateINOFYDefinition(ctx, raw, toolsAllowed)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	snapshot, err := svc.engine.cfg.Policy.Snapshot(domain.PolicyProfileDefault)
	if err != nil {
		t.Fatal(err)
	}
	session, err := backend.GetSession(ctx, sessionID)
	if err != nil {
		t.Fatal(err)
	}
	parent, err := backend.GetRun(ctx, parentRunID)
	if err != nil {
		t.Fatal(err)
	}
	rootID := parent.RootID
	if rootID == "" {
		rootID = parent.ID
	}
	if rootID != parent.ID {
		if _, err := backend.GetRun(ctx, rootID); errors.Is(err, storage.ErrNotFound) {
			// The run-tree head is a completed primary: a finished root with
			// live descendants is the realistic recovery shape and avoids the
			// single-active-primary constraint in the fixture session.
			if err := backend.CreateRun(ctx, domain.Run{ID: rootID, SessionID: sessionID,
				Status: domain.RunCompleted, Kind: domain.RunKindPrimary, CreatedAt: 1}); err != nil {
				t.Fatal(err)
			}
		} else if err != nil {
			t.Fatal(err)
		}
	}
	sandboxMode, approvalPolicy := session.EffectiveSandbox()
	authorityJSON, authorityDigest, err := workflowAuthorityRecord(snapshot, sandboxMode, approvalPolicy, toolsAllowed)
	if err != nil {
		t.Fatal(err)
	}
	limitsJSON, err := json.Marshal(inofyWorkflowLimits())
	if err != nil {
		t.Fatal(err)
	}
	hostBinding := inofyHostBinding(authorityDigest, admitted.Meta.ProgramDigest)
	wfID := domain.RunID("wf-" + string(parentRunID))
	descriptorDigest := inofyTestDigest(string(admitted.CanonicalJSON))
	revision := domain.WorkflowRevision{
		RunID: wfID, ParentRunID: parentRunID, ParentSessionID: sessionID, RootRunID: rootID,
		OperationKey:     operationKey,
		DescriptorDigest: descriptorDigest, AuthorityDigest: authorityDigest,
		DescriptorJSON: admitted.CanonicalJSON, AuthorityJSON: authorityJSON,
		SchemaVersion: 2, CreatedAt: 2,
		ProgramDigest:   admitted.Meta.ProgramDigest,
		CatalogDigest:   admitted.Meta.CatalogDigest,
		CompilerVersion: admitted.Meta.CompilerVersion, EinoBuild: admitted.Meta.EinoBuild,
		InputDigest: inofyTestInputDigest(), InputJSON: []byte(`{}`), EffectiveLimits: limitsJSON,
		HostBindingID: hostBinding,
	}
	if _, err := backend.CommitWorkflowAdmission(ctx, storage.WorkflowAdmission{
		Revision: revision,
		Run: domain.Run{ID: wfID, SessionID: sessionID, Status: domain.RunAccepted, Kind: domain.RunKindWorkflow,
			ParentID: parentRunID, RootID: rootID, Depth: parent.Depth + 1, CreatedAt: 2},
		Started: domain.RunEvent{RunID: wfID, Type: domain.EventRunStarted, CreatedAt: 2, PayloadVersion: 1,
			Payload: []byte(`{"mode":"test"}`)},
	}); err != nil {
		t.Fatalf("admit revision: %v", err)
	}
	run, err := backend.GetRun(ctx, wfID)
	if err != nil {
		t.Fatal(err)
	}
	return run, revision, inofy.ExecutionRef{
		RunID: string(wfID), Epoch: 1,
		ProgramDigest: revision.ProgramDigest, HostBindingID: revision.HostBindingID,
	}
}

// TestINOFYWorkflowRecoveryRequiredOnInterruptedRun proves a run left in the
// engine's running state is classified recovery_required, never silently
// replayed, and the host fences a duplicate start.
func TestINOFYWorkflowRecoveryRequiredOnInterruptedRun(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, testsupport.NewEchoModel())
	sessionID := domain.SessionID("sess-wf-recover")
	parentRunID := domain.RunID("run-wf-recover-parent")
	prepareChildSessionAuthorizer(t, svc, backend, sessionID, parentRunID, []string{tools.EchoInfoName})

	opKey := "wf-op-interrupted"
	wfRun, revision, ref := admitINOFYWorkflowFixture(t, svc, backend, sessionID, parentRunID, opKey,
		json.RawMessage(inofyTwoNodeDefinition), []string{tools.EchoInfoName})
	store := newINOFYRunStore(backend)
	admitData, _ := json.Marshal(map[string]any{
		"input_digest": revision.InputDigest,
		"limits":       json.RawMessage(revision.EffectiveLimits),
	})
	if _, err := store.Commit(ctx, ref, inofy.RunCommit{
		CommitID:   "admit",
		Events:     []inofy.Event{{Kind: inofy.EventRunAdmitted, Data: admitData}},
		Transition: inofy.StateTransition{Expected: "", Target: inofy.RunAdmitted},
	}); err != nil {
		t.Fatalf("commit admitted: %v", err)
	}
	if _, err := store.Commit(ctx, ref, inofy.RunCommit{
		CommitID:   "start",
		Events:     []inofy.Event{{Kind: inofy.EventRunStarted}},
		Transition: inofy.StateTransition{Expected: inofy.RunAdmitted, Target: inofy.RunRunning},
	}); err != nil {
		t.Fatalf("commit running: %v", err)
	}
	if err := backend.SetRunStatus(ctx, wfRun.ID, domain.RunActive); err != nil {
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
	svc.snapshots[wfRun.ID] = snapshot
	svc.ledgers[wfRun.ID] = ledger
	svc.runTools[wfRun.ID] = childToolSet([]string{tools.EchoInfoName})
	svc.runSessions[wfRun.ID] = sessionID
	svc.mu.Unlock()
	_ = revision

	if err := svc.recoverWorkflowRun(ctx, wfRun, ""); err != nil {
		t.Fatalf("recover: %v", err)
	}
	// The relaunched engine classifies the stale running projection to
	// recovery_required asynchronously; wait for its durable commit.
	var details WorkflowDetails
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		details, err = svc.GetWorkflow(ctx, wfRun.ID)
		if err != nil {
			t.Fatalf("inspect recovered: %v", err)
		}
		if details.EngineStatus == string(inofy.RunRecoveryRequired) {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if details.EngineStatus != string(inofy.RunRecoveryRequired) {
		t.Fatalf("engine status = %q", details.EngineStatus)
	}
	current, err := backend.GetRun(ctx, wfRun.ID)
	if err != nil {
		t.Fatal(err)
	}
	if current.Status.Terminal() {
		t.Fatalf("recovery_required must not fabricate a native terminal: %v", current.Status)
	}
	if _, err := svc.StartINOFYWorkflow(ctx, parentRunID, opKey, json.RawMessage(inofyTwoNodeDefinition)); !errors.Is(err, ErrWorkflowRecoveryRequired) {
		t.Fatalf("duplicate start on interrupted run = %v", err)
	}
}

// TestINOFYWorkflowResumesAdmittedAfterCrash proves a run admitted but never
// executed (crash between admission and launch) resumes through recovery and
// completes through real governed children.
func TestINOFYWorkflowResumesAdmittedAfterCrash(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, testsupport.NewEchoModel())
	sessionID := domain.SessionID("sess-wf-resume")
	parentRunID := domain.RunID("run-wf-resume-parent")
	prepareChildSessionAuthorizer(t, svc, backend, sessionID, parentRunID, []string{tools.EchoInfoName})

	wfRun, _, _ := admitINOFYWorkflowFixture(t, svc, backend, sessionID, parentRunID, "wf-op-crash",
		json.RawMessage(inofyTwoNodeDefinition), []string{tools.EchoInfoName})
	if err := backend.SetRunStatus(ctx, wfRun.ID, domain.RunActive); err != nil {
		t.Fatal(err)
	}
	if err := svc.recoverWorkflowRun(ctx, wfRun, ""); err != nil {
		t.Fatalf("recover admitted run: %v", err)
	}
	waitForRunStatus(t, backend, wfRun.ID, domain.RunCompleted)
	details, err := svc.GetWorkflow(ctx, wfRun.ID)
	if err != nil {
		t.Fatalf("inspect resumed: %v", err)
	}
	if details.EngineStatus != string(inofy.RunSucceeded) || len(details.Nodes) != 2 {
		t.Fatalf("resumed details = %+v", details)
	}
	children, err := backend.ListChildRuns(ctx, wfRun.ID)
	if err != nil || len(children) != 2 {
		t.Fatalf("resumed children = %+v err=%v", children, err)
	}
}

// TestINOFYWorkflowLegacyRowsStayHistorical keeps discriminator-1 rows out of
// auto-recovery and inspection.
func TestINOFYWorkflowLegacyRowsStayHistorical(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, testsupport.NewEchoModel())
	sessionID := domain.SessionID("sess-wf-hist")
	parentRunID := domain.RunID("run-wf-hist-parent")
	prepareChildSessionAuthorizer(t, svc, backend, sessionID, parentRunID, []string{tools.EchoInfoName})

	parent, err := backend.GetRun(ctx, parentRunID)
	if err != nil {
		t.Fatal(err)
	}
	rootID := parent.RootID
	if rootID == "" {
		rootID = parent.ID
	}
	descriptorJSON := []byte(`{"schema_version":1}`)
	authorityJSON := []byte(`{"authority":1}`)
	wfID := domain.RunID("wf-legacy-row")
	revision := domain.WorkflowRevision{
		RunID: wfID, ParentRunID: parentRunID, ParentSessionID: sessionID, RootRunID: rootID,
		OperationKey:     "wf-op-legacy-row",
		DescriptorDigest: inofyTestDigest(string(descriptorJSON)), AuthorityDigest: inofyTestDigest(string(authorityJSON)),
		DescriptorJSON: descriptorJSON, AuthorityJSON: authorityJSON,
		SchemaVersion: 1, CreatedAt: 2,
	}
	if _, err := backend.CommitWorkflowAdmission(ctx, storage.WorkflowAdmission{
		Revision: revision,
		Run: domain.Run{ID: wfID, SessionID: sessionID, Status: domain.RunAccepted, Kind: domain.RunKindWorkflow,
			ParentID: parentRunID, RootID: rootID, Depth: parent.Depth + 1, CreatedAt: 2},
		Started: domain.RunEvent{RunID: wfID, Type: domain.EventRunStarted, CreatedAt: 2, PayloadVersion: 1,
			Payload: []byte(`{"mode":"test"}`)},
	}); err != nil {
		t.Fatalf("admit legacy revision: %v", err)
	}
	run, err := backend.GetRun(ctx, wfID)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.recoverWorkflowRun(ctx, run, ""); err != nil {
		t.Fatalf("legacy row must be skipped, not failed: %v", err)
	}
	current, err := backend.GetRun(ctx, wfID)
	if err != nil || current.Status != run.Status {
		t.Fatalf("legacy row was touched: %+v err=%v", current, err)
	}
	if _, err := svc.GetWorkflow(ctx, wfID); err == nil {
		t.Fatal("legacy inspection must fail clearly")
	}
}
