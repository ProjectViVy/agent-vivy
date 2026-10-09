package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/ProjectViVy/inofy"
	laputaevolution "github.com/ProjectViVy/laputa/laputa/evolution"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/testsupport"
	"agent-vivy/internal/tools"
)

func newCognitivePersistentService(t *testing.T, path string, model domain.ChatModel) (*Service, *sqlite.Backend) {
	t.Helper()
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, path)
	if err != nil {
		t.Fatalf("open persistent cognitive store: %v", err)
	}
	selected, err := tools.Builtin(backend).Resolve([]string{tools.EchoInfoName})
	if err != nil {
		t.Fatal(err)
	}
	engine, err := NewEngine(ctx, WrapModel(model), selected, EngineConfig{StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10})
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(engine, "test", "test-model", ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Notes: backend,
		Sessions: backend, Sink: newTestSink(), Truncations: backend,
	})
	workspaces, err := NewSessionWorkspaceManager(t.TempDir(), backend, backend)
	if err != nil {
		t.Fatal(err)
	}
	svc.deps.Workspaces = workspaces
	return svc, backend
}

// admitCognitiveWorkflowFixture commits a schema-2 trusted-strategy revision
// plus its workflow Run, mirroring admitINOFYWorkflowFixture for the trusted
// lane so recovery exercises rebind-to-code-catalog instead of the authored
// decoder.
func admitCognitiveWorkflowFixture(t *testing.T, svc *Service, backend *sqlite.Backend, sessionID domain.SessionID,
	parentRunID domain.RunID, operationKey string, input json.RawMessage, mutate ...func(*domain.WorkflowRevision)) (domain.Run, domain.WorkflowRevision) {
	t.Helper()
	ctx := context.Background()
	admitted, err := trustedStrategyAdmission(ctx, TrustedStrategyDIVA)
	if err != nil {
		t.Fatalf("trusted admission: %v", err)
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
			// live descendants is the realistic recovery shape.
			if err := backend.CreateRun(ctx, domain.Run{ID: rootID, SessionID: sessionID,
				Status: domain.RunCompleted, Kind: domain.RunKindPrimary, CreatedAt: 1}); err != nil {
				t.Fatal(err)
			}
		} else if err != nil {
			t.Fatal(err)
		}
	}
	sandboxMode, approvalPolicy := session.EffectiveSandbox()
	authorityJSON, authorityDigest, err := trustedWorkflowAuthorityRecord(snapshot, sandboxMode, approvalPolicy, TrustedStrategyDIVA)
	if err != nil {
		t.Fatal(err)
	}
	limitsJSON, err := json.Marshal(inofyWorkflowLimits())
	if err != nil {
		t.Fatal(err)
	}
	if len(input) == 0 {
		input = cognitiveInput(t)
	}
	canonicalInput, inputDigest, err := normalizeINOFYInput(input)
	if err != nil {
		t.Fatal(err)
	}
	hostBinding := inofyHostBinding(authorityDigest, admitted.Meta.ProgramDigest)
	wfID := domain.RunID("wfc-" + string(parentRunID))
	revision := domain.WorkflowRevision{
		RunID: wfID, ParentRunID: parentRunID, ParentSessionID: sessionID, RootRunID: rootID,
		OperationKey:     operationKey,
		DescriptorDigest: sha256Hex(admitted.CanonicalJSON), AuthorityDigest: authorityDigest,
		DescriptorJSON: admitted.CanonicalJSON, AuthorityJSON: authorityJSON,
		SchemaVersion: 2, CreatedAt: 2,
		ProgramDigest:   admitted.Meta.ProgramDigest,
		CatalogDigest:   admitted.Meta.CatalogDigest,
		CompilerVersion: admitted.Meta.CompilerVersion, EinoBuild: admitted.Meta.EinoBuild,
		InputDigest: inputDigest, InputJSON: canonicalInput, EffectiveLimits: limitsJSON,
		HostBindingID: hostBinding,
	}
	for _, m := range mutate {
		m(&revision)
	}
	if _, err := backend.CommitWorkflowAdmission(ctx, storage.WorkflowAdmission{
		Revision: revision,
		Run: domain.Run{ID: wfID, SessionID: sessionID, Status: domain.RunAccepted, Kind: domain.RunKindWorkflow,
			ParentID: parentRunID, RootID: rootID, Depth: parent.Depth + 1, CreatedAt: 2},
		Started: domain.RunEvent{RunID: wfID, Type: domain.EventRunStarted, CreatedAt: 2, PayloadVersion: 1,
			Payload: []byte(`{"mode":"test"}`)},
	}); err != nil {
		t.Fatalf("admit trusted revision: %v", err)
	}
	run, err := backend.GetRun(ctx, wfID)
	if err != nil {
		t.Fatal(err)
	}
	return run, revision
}

// TestCognitiveRecoveryResumesAdmitted proves a trusted run admitted but
// never executed (crash between admission and launch) rebinds the trusted
// catalog during recovery and completes the strategy.
func TestCognitiveRecoveryResumesAdmitted(t *testing.T) {
	ctx := context.Background()
	d := &fakeCognitiveDomain{}
	svc, backend := inofyExecService(t, testsupport.NewEchoModel())
	svc.deps.Cognitive = &CognitiveBinding{Domain: d, Binding: cognitiveFixtureRunBinding()}
	sessionID := domain.SessionID("sess-cog-resume")
	parentRunID := domain.RunID("run-cog-resume-parent")
	prepareChildSessionAuthorizer(t, svc, backend, sessionID, parentRunID, nil)

	wfRun, _ := admitCognitiveWorkflowFixture(t, svc, backend, sessionID, parentRunID, "cog-op-resume", nil)
	if err := backend.SetRunStatus(ctx, wfRun.ID, domain.RunActive); err != nil {
		t.Fatal(err)
	}
	if err := svc.recoverWorkflowRun(ctx, wfRun, ""); err != nil {
		t.Fatalf("recover trusted run: %v", err)
	}
	waitForRunStatus(t, backend, wfRun.ID, domain.RunCompleted)
	details, err := svc.GetWorkflow(ctx, wfRun.ID)
	if err != nil {
		t.Fatalf("inspect resumed: %v", err)
	}
	if details.EngineStatus != string(inofy.RunSucceeded) {
		t.Fatalf("engine status = %q", details.EngineStatus)
	}
	if d.collects != 1 || len(d.applied) != 0 {
		t.Fatalf("domain activity = collects %d applied %d", d.collects, len(d.applied))
	}
}

// TestCognitiveRecoveryClassifiesRunning proves an interrupted trusted run
// is classified recovery_required without re-executing the strategy: no
// domain calls, no fabricated native terminal.
func TestCognitiveRecoveryClassifiesRunning(t *testing.T) {
	ctx := context.Background()
	d := &fakeCognitiveDomain{}
	svc, backend := inofyExecService(t, testsupport.NewEchoModel())
	svc.deps.Cognitive = &CognitiveBinding{Domain: d, Binding: cognitiveFixtureRunBinding()}
	sessionID := domain.SessionID("sess-cog-running")
	parentRunID := domain.RunID("run-cog-running-parent")
	prepareChildSessionAuthorizer(t, svc, backend, sessionID, parentRunID, nil)

	wfRun, revision := admitCognitiveWorkflowFixture(t, svc, backend, sessionID, parentRunID, "cognitive:activity:0-9:a0", nil)
	store := newINOFYRunStore(backend)
	ref := inofy.ExecutionRef{
		RunID: string(wfRun.ID), Epoch: 1,
		ProgramDigest: revision.ProgramDigest, HostBindingID: revision.HostBindingID,
	}
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
	svc.runTools[wfRun.ID] = childToolSet(nil)
	svc.runSessions[wfRun.ID] = sessionID
	svc.mu.Unlock()

	if err := svc.recoverWorkflowRun(ctx, wfRun, ""); err != nil {
		t.Fatalf("recover: %v", err)
	}
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
	if d.collects != 0 || len(d.applied) != 0 {
		t.Fatalf("interrupted run re-executed strategy: collects %d applied %d", d.collects, len(d.applied))
	}
	current, err := backend.GetRun(ctx, wfRun.ID)
	if err != nil {
		t.Fatal(err)
	}
	if current.Status.Terminal() {
		t.Fatalf("recovery_required must not fabricate a native terminal: %v", current.Status)
	}
	if _, err := svc.StartCognitiveWorkflow(ctx, parentRunID, "cognitive:activity:0-9:a0", TrustedStrategyDIVA, cognitiveInput(t)); !errors.Is(err, ErrWorkflowRecoveryRequired) {
		t.Fatalf("duplicate start on interrupted run = %v", err)
	}

	// The native Run remains active while the durable workflow projection is
	// recovery_required. Cognitive reconciliation must persist an unknown fence.
	svc.deps.Cognitive = cognitiveBinding(d, nil, backend.Snapshot(), func() int64 { return 50 })
	state, version, err := svc.loadCognitiveState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	state.Intent = &cognitiveIntent{ParentRunID: parentRunID, OperationKey: revision.OperationKey,
		StrategyID: TrustedStrategyDIVA, Input: append(json.RawMessage(nil), revision.InputJSON...), Attempt: 0}
	state.StateSchema = cognitiveStateSchema
	state.ActiveRunID = string(wfRun.ID)
	state.PendingThrough = 9
	state.Phase = "running"
	if err := svc.saveCognitiveState(ctx, state, version); err != nil {
		t.Fatal(err)
	}
	state, version, err = svc.loadCognitiveState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	state, _, err = svc.reconcileCognitiveIntent(ctx, state, version)
	if err != nil {
		t.Fatalf("reconcile recovery_required workflow: %v", err)
	}
	if state.Blocked != cognitiveBlockUnknown || state.ActiveRunID != "" || state.Phase != "blocked" {
		t.Fatalf("recovery_required workflow was not fenced: blocked=%q active=%q phase=%q", state.Blocked, state.ActiveRunID, state.Phase)
	}
}

// TestCognitiveRecoveryRejectsAlteredDescriptor fails the rebind when the
// persisted trusted definition drifted from the code-owned strategy: the
// program/descriptor digests no longer match the live catalog.
func TestCognitiveRecoveryRejectsAlteredDescriptor(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, testsupport.NewEchoModel())
	svc.deps.Cognitive = &CognitiveBinding{Domain: &fakeCognitiveDomain{}, Binding: cognitiveFixtureRunBinding()}
	sessionID := domain.SessionID("sess-cog-altered")
	parentRunID := domain.RunID("run-cog-altered-parent")
	prepareChildSessionAuthorizer(t, svc, backend, sessionID, parentRunID, nil)

	wfRun, _ := admitCognitiveWorkflowFixture(t, svc, backend, sessionID, parentRunID, "cog-op-altered", nil,
		func(revision *domain.WorkflowRevision) {
			// Tampered saved definition: valid JSON, different canonical bytes,
			// self-consistent digest — only the live-catalog compare catches it.
			revision.DescriptorJSON = append(revision.DescriptorJSON, ' ')
			revision.DescriptorDigest = sha256Hex(revision.DescriptorJSON)
		})
	if err := backend.SetRunStatus(ctx, wfRun.ID, domain.RunActive); err != nil {
		t.Fatal(err)
	}
	if err := svc.recoverWorkflowRun(ctx, wfRun, ""); !errors.Is(err, storage.ErrWorkflowRevisionConflict) {
		t.Fatalf("tampered descriptor = %v", err)
	}
}

// TestCognitiveRecoveryRequiresBinding keeps a trusted revision out of
// recovery when the bound Domain is absent: fail closed, never run unbound.
func TestCognitiveRecoveryRequiresBinding(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, testsupport.NewEchoModel())
	sessionID := domain.SessionID("sess-cog-nobind")
	parentRunID := domain.RunID("run-cog-nobind-parent")
	prepareChildSessionAuthorizer(t, svc, backend, sessionID, parentRunID, nil)

	svc.deps.Cognitive = &CognitiveBinding{Domain: &fakeCognitiveDomain{}, Binding: cognitiveFixtureRunBinding()}
	wfRun, _ := admitCognitiveWorkflowFixture(t, svc, backend, sessionID, parentRunID, "cog-op-nobind", nil)
	svc.deps.Cognitive = nil
	if err := backend.SetRunStatus(ctx, wfRun.ID, domain.RunActive); err != nil {
		t.Fatal(err)
	}
	if err := svc.recoverWorkflowRun(ctx, wfRun, ""); !errors.Is(err, ErrCognitiveUnavailable) {
		t.Fatalf("unbound recovery = %v", err)
	}
}

func TestCognitiveUnadmittedIntentFencesReplacedSupervisor(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, cognitiveTestModel())
	svc.deps.Cognitive = cognitiveBinding(&fakeCognitiveDomain{}, nil, backend.Snapshot(), func() int64 { return 50 })
	state, version, err := svc.loadCognitiveState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	parentID, err := svc.ensureCognitiveSupervisor(ctx, &state)
	if err != nil {
		t.Fatal(err)
	}
	input, _, err := normalizeINOFYInput(cognitiveInput(t))
	if err != nil {
		t.Fatal(err)
	}
	state.StateSchema = cognitiveStateSchema
	state.Phase = "admitting"
	state.PendingThrough = 9
	state.Intent = &cognitiveIntent{ParentRunID: parentID, OperationKey: "cognitive:activity:0-9:a0",
		StrategyID: TrustedStrategyDIVA, Input: input, Attempt: 0}
	if err := svc.saveCognitiveState(ctx, state, version); err != nil {
		t.Fatal(err)
	}
	if err := backend.SetRunStatus(ctx, parentID, domain.RunCancelled); err != nil {
		t.Fatal(err)
	}

	elig, err := svc.TriggerCognitive(ctx)
	if err != nil || elig.Reason != laputaevolution.EligibilityReason("blocked:"+cognitiveBlockUnknown) {
		t.Fatalf("invalid original parent was rebound: %+v, %v", elig, err)
	}
	state, _, err = svc.loadCognitiveState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if state.Intent == nil || state.Intent.ParentRunID != parentID || state.Blocked != cognitiveBlockUnknown || state.ActiveRunID != "" {
		t.Fatalf("original intent was not fenced intact: %+v", state)
	}
	runs, err := backend.ListRunsBySession(ctx, cognitiveSupervisorSessionID)
	if err != nil {
		t.Fatal(err)
	}
	primary, workflows := 0, 0
	for _, run := range runs {
		if run.Kind == domain.RunKindPrimary {
			primary++
		}
		if run.Kind == domain.RunKindWorkflow {
			workflows++
		}
	}
	if primary != 1 || workflows != 0 {
		t.Fatalf("fenced intent created replacement identity: primaries=%d workflows=%d", primary, workflows)
	}
}

func TestCognitiveIntentCrashBeforeAdmissionReusesIdentity(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "cognitive-reopen.db")
	svc, backend := newCognitivePersistentService(t, path, cognitiveTestModel())
	d := &fakeCognitiveDomain{batch: laputaevolution.EvidenceBatch{ActivityRevision: 3, Entries: []laputaevolution.Entry{{
		ID: "e1", Section: "work", Field: "next", SessionID: "s1", EventID: "ev1", Body: "follow up",
	}}}}
	svc.deps.Cognitive = cognitiveBinding(d, nil, backend.Snapshot(), func() int64 { return 50 })
	state, version, err := svc.loadCognitiveState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	parentID, err := svc.ensureCognitiveSupervisor(ctx, &state)
	if err != nil {
		t.Fatal(err)
	}
	input, _, err := normalizeINOFYInput(cognitiveInput(t))
	if err != nil {
		t.Fatal(err)
	}
	state.StateSchema = cognitiveStateSchema
	state.Phase = "admitting"
	state.PendingThrough = 9
	state.Intent = &cognitiveIntent{ParentRunID: parentID, OperationKey: "cognitive:activity:0-9:a0",
		StrategyID: TrustedStrategyDIVA, Input: input, Attempt: 0}
	if err := svc.saveCognitiveState(ctx, state, version); err != nil {
		t.Fatal(err)
	}
	if err := backend.Close(); err != nil {
		t.Fatal(err)
	}

	// A new backend handle and Service instance prove the exact intent
	// survives a real SQLite close/reopen before the admission transaction.
	reopened, reopenedBackend := newCognitivePersistentService(t, path, cognitiveTestModel())
	reopened.deps.Cognitive = cognitiveBinding(d, nil, reopenedBackend.Snapshot(), func() int64 { return 50 })
	t.Cleanup(func() { reopened.CancelAll(); reopened.WaitIdle(context.Background()); _ = reopenedBackend.Close() })

	elig, err := reopened.TriggerCognitive(ctx)
	if err != nil || elig.Reason != laputaevolution.ReasonActive {
		t.Fatalf("pre-admission intent did not converge: %+v, %v", elig, err)
	}
	runs := listWorkflowRuns(t, reopened, reopenedBackend)
	if len(runs) != 1 {
		t.Fatalf("workflow count after intent recovery = %d, want one", len(runs))
	}
	revision, err := reopenedBackend.GetWorkflowRevision(ctx, runs[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if revision.ParentRunID != parentID || revision.OperationKey != "cognitive:activity:0-9:a0" || !bytes.Equal(revision.InputJSON, input) {
		t.Fatalf("recovery changed persisted identity: parent=%q key=%q input=%s", revision.ParentRunID, revision.OperationKey, revision.InputJSON)
	}
	waitForRunStatus(t, reopenedBackend, runs[0].ID, domain.RunCompleted)
	if d.collects != 1 || len(d.applied) != 1 {
		t.Fatalf("recovery duplicated or lost the admitted effect: collects=%d applies=%d", d.collects, len(d.applied))
	}
}

func TestCognitiveOrphanSupervisorIsAdoptedBeforeAdmission(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, cognitiveTestModel())
	svc.deps.Cognitive = cognitiveBinding(&fakeCognitiveDomain{}, &fakeSource{high: 9}, backend.Snapshot(), func() int64 { return 50 })

	// Model the cut after the supervisor row commits but before its sequence,
	// ID, and admission intent reach the cognitive snapshot.
	st, _, err := svc.loadCognitiveState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	orphanID, err := svc.ensureCognitiveSupervisor(ctx, &st)
	if err != nil {
		t.Fatal(err)
	}

	elig, err := svc.cognitiveAttempt(ctx, true)
	if err != nil || !elig.Run {
		t.Fatalf("admission after supervisor-only crash cut = %+v, %v", elig, err)
	}
	state, _, err := svc.loadCognitiveState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if state.SupervisorRunID != string(orphanID) || state.SupervisorSeq != 1 {
		t.Fatalf("orphan supervisor was not adopted: id=%q seq=%d, want %q/1", state.SupervisorRunID, state.SupervisorSeq, orphanID)
	}
	runs := listWorkflowRuns(t, svc, backend)
	if len(runs) != 1 || runs[0].ParentID != orphanID {
		t.Fatalf("admitted workflows=%+v, want one child of orphan supervisor %q", runs, orphanID)
	}
}

func TestCognitiveIntentKeyAttemptMismatchFencesBeforeAdmission(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, cognitiveTestModel())
	svc.deps.Cognitive = cognitiveBinding(&fakeCognitiveDomain{}, &fakeSource{high: 9}, backend.Snapshot(), func() int64 { return 50 })
	state, version, err := svc.loadCognitiveState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	parentID, err := svc.ensureCognitiveSupervisor(ctx, &state)
	if err != nil {
		t.Fatal(err)
	}
	input, _, err := normalizeINOFYInput(cognitiveInput(t))
	if err != nil {
		t.Fatal(err)
	}
	state.StateSchema = cognitiveStateSchema
	state.SupervisorRunID = string(parentID)
	state.PendingThrough = 9
	state.Phase = "admitting"
	state.Intent = &cognitiveIntent{ParentRunID: parentID, OperationKey: "cognitive:activity:0-9:a2",
		StrategyID: TrustedStrategyDIVA, Input: input, Attempt: 0}
	if err := svc.saveCognitiveState(ctx, state, version); err != nil {
		t.Fatal(err)
	}

	elig, err := svc.cognitiveAttempt(ctx, true)
	if err != nil || elig.Reason != laputaevolution.EligibilityReason("blocked:"+cognitiveBlockUnknown) {
		t.Fatalf("inconsistent intent was admitted: eligibility=%+v err=%v", elig, err)
	}
	if workflows := listWorkflowRuns(t, svc, backend); len(workflows) != 0 {
		t.Fatalf("inconsistent intent admitted workflows: %+v", workflows)
	}
}

func TestCognitiveLegacyActiveRunReconstructsIntent(t *testing.T) {
	ctx := context.Background()
	gate := make(chan struct{})
	entered := make(chan struct{}, 1)
	d := &fakeCognitiveDomain{gate: gate, entered: entered}
	svc, backend := inofyExecService(t, cognitiveTestModel())
	svc.deps.Cognitive = cognitiveBinding(d, nil, backend.Snapshot(), func() int64 { return 50 })
	parentID := domain.RunID("run-cognitive_supervisor_1")
	prepareChildSessionAuthorizer(t, svc, backend, cognitiveSupervisorSessionID, parentID, nil)
	input := cognitiveInput(t)
	started, err := svc.StartCognitiveWorkflow(ctx, parentID, "cognitive:activity:0-9:a0", TrustedStrategyDIVA, input)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("legacy workflow did not reach held Collect")
	}
	legacy, err := json.Marshal(cognitiveState{
		SupervisorSeq: 1, SupervisorRunID: string(parentID), ActiveRunID: string(started.Run.ID),
		PendingThrough: 9, Policy: laputaevolution.TriggerPolicy{Enabled: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.deps.Cognitive.Store.Put(ctx, cognitiveStateKey, legacy, 0); err != nil {
		t.Fatal(err)
	}

	elig, err := svc.TriggerCognitive(ctx)
	if err != nil || elig.Reason != laputaevolution.ReasonActive {
		t.Fatalf("legacy active run was not adopted: %+v, %v", elig, err)
	}
	state, _, err := svc.loadCognitiveState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if state.StateSchema != cognitiveStateSchema || state.Intent == nil || state.Intent.OperationKey != started.Revision.OperationKey || state.ActiveRunID != string(started.Run.ID) {
		t.Fatalf("legacy identity not reconstructed from revision: %+v", state)
	}
	close(gate)
	waitForRunStatus(t, backend, started.Run.ID, domain.RunCompleted)
}

func TestCognitiveLegacyFailedRunPreservesRetryBudget(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, cognitiveTestModel())
	svc.deps.Cognitive = cognitiveBinding(&fakeCognitiveDomain{}, &fakeSource{high: 9}, backend.Snapshot(), func() int64 { return 50 })
	safeRun := makeFailedCognitiveEvidenceFixtureAttempt(t, svc, backend,
		cognitiveSupervisorSessionID, "run-cog-legacy-a2-parent", "legacy-a2", true, false, 2)
	revision, err := backend.GetWorkflowRevision(ctx, safeRun.ID)
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := json.Marshal(cognitiveState{
		SupervisorRunID: string(revision.ParentRunID), ActiveRunID: string(safeRun.ID),
		PendingThrough: 9, Attempt: 2, Policy: laputaevolution.TriggerPolicy{Enabled: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.deps.Cognitive.Store.Put(ctx, cognitiveStateKey, legacy, 0); err != nil {
		t.Fatal(err)
	}
	state, version, err := svc.loadCognitiveState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	state, version, err = svc.upgradeCognitiveState(ctx, state, version)
	if err != nil {
		t.Fatal(err)
	}
	if state.Intent == nil || state.Intent.Attempt != 2 {
		t.Fatalf("legacy attempt not reconstructed from immutable operation key: %+v", state.Intent)
	}
	state, _, err = svc.reconcileCognitiveIntent(ctx, state, version)
	if err != nil {
		t.Fatal(err)
	}
	if state.Attempt != 3 || state.Blocked != cognitiveBlockExhausted || state.Intent != nil {
		t.Fatalf("legacy retry budget was reset: attempt=%d blocked=%q intent=%+v", state.Attempt, state.Blocked, state.Intent)
	}
}

func TestCognitivePersistedAttemptMismatchDoesNotRetry(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, cognitiveTestModel())
	svc.deps.Cognitive = cognitiveBinding(&fakeCognitiveDomain{}, &fakeSource{high: 9}, backend.Snapshot(), func() int64 { return 50 })
	safeRun := makeFailedCognitiveEvidenceFixtureAttempt(t, svc, backend,
		cognitiveSupervisorSessionID, "run-cog-schema2-a2-parent", "schema2-a2", true, false, 2)
	revision, err := backend.GetWorkflowRevision(ctx, safeRun.ID)
	if err != nil {
		t.Fatal(err)
	}
	state, version, err := svc.loadCognitiveState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	state.StateSchema = cognitiveStateSchema
	state.ActiveRunID = string(safeRun.ID)
	state.PendingThrough = 9
	state.Intent = &cognitiveIntent{ParentRunID: revision.ParentRunID, OperationKey: revision.OperationKey,
		StrategyID: TrustedStrategyDIVA, Input: append(json.RawMessage(nil), revision.InputJSON...), Attempt: 0}
	if err := svc.saveCognitiveState(ctx, state, version); err != nil {
		t.Fatal(err)
	}
	state, version, err = svc.loadCognitiveState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	state, _, err = svc.reconcileCognitiveIntent(ctx, state, version)
	if err != nil {
		t.Fatal(err)
	}
	if state.Blocked != cognitiveBlockUnknown || state.Attempt != 0 || state.Intent == nil {
		t.Fatalf("mismatched a2 intent reopened the retry budget: %+v", state)
	}
}

func TestCognitiveCancelledIntentRetainsFence(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, cognitiveTestModel())
	svc.deps.Cognitive = cognitiveBinding(&fakeCognitiveDomain{}, nil, backend.Snapshot(), func() int64 { return 50 })
	sessionID := cognitiveSupervisorSessionID
	parentID := domain.RunID("run-cog-cancel-parent")
	prepareChildSessionAuthorizer(t, svc, backend, sessionID, parentID, nil)
	workflow, revision := admitCognitiveWorkflowFixture(t, svc, backend, sessionID, parentID, "cognitive:activity:0-9:a0", cognitiveInput(t))
	if err := backend.SetRunStatus(ctx, workflow.ID, domain.RunCancelled); err != nil {
		t.Fatal(err)
	}
	state, version, err := svc.loadCognitiveState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	state.StateSchema = cognitiveStateSchema
	state.SupervisorRunID = string(parentID)
	state.ActiveRunID = string(workflow.ID)
	state.PendingThrough = 9
	state.Phase = "running"
	state.Intent = &cognitiveIntent{ParentRunID: parentID, OperationKey: revision.OperationKey,
		StrategyID: TrustedStrategyDIVA, Input: append(json.RawMessage(nil), revision.InputJSON...), Attempt: 0}
	if err := svc.saveCognitiveState(ctx, state, version); err != nil {
		t.Fatal(err)
	}
	svc.StartCognitiveLoop(ctx, time.Hour)
	t.Cleanup(svc.StopCognitiveLoop)
	elig, err := svc.cognitiveAttempt(ctx, false)
	if err != nil || elig.Reason != laputaevolution.EligibilityReason("blocked:"+cognitiveBlockCancelled) {
		t.Fatalf("cancelled intent did not fence: %+v, %v", elig, err)
	}
	state, _, err = svc.loadCognitiveState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if state.Blocked != cognitiveBlockCancelled || state.Intent == nil || state.ActiveRunID != "" || state.Watermark != 0 {
		t.Fatalf("cancel settlement lost its fence or advanced the window: %+v", state)
	}
	if len(listWorkflowRuns(t, svc, backend)) != 1 {
		t.Fatal("cancelled intent was automatically replayed")
	}
}

func TestCognitiveRetrySafetyRequiresSettlementEvidence(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, cognitiveTestModel())
	svc.deps.Cognitive = cognitiveBinding(&fakeCognitiveDomain{}, nil, backend.Snapshot(), func() int64 { return 50 })
	prepareChildSessionAuthorizer(t, svc, backend, "sess-cog-unsettled-workflow", "run-cog-unsettled-parent", nil)
	unsettledWorkflow, _ := admitCognitiveWorkflowFixture(t, svc, backend, "sess-cog-unsettled-workflow", "run-cog-unsettled-parent", "op-unsettled", cognitiveInput(t))
	if err := backend.SetRunStatus(ctx, unsettledWorkflow.ID, domain.RunFailed); err != nil {
		t.Fatal(err)
	}
	if err := backend.SetRunStatus(ctx, "run-cog-unsettled-parent", domain.RunCompleted); err != nil {
		t.Fatal(err)
	}
	if safe, err := svc.cognitiveRunRetrySafe(ctx, unsettledWorkflow.ID); err != nil || safe {
		t.Fatalf("unsettled workflow projection safe=%v err=%v, want unsafe", safe, err)
	}

	safeRun := makeFailedCognitiveEvidenceFixture(t, svc, backend, cognitiveSupervisorSessionID, "run-cog-safe-parent", "safe-call", true, false)
	if safe, err := svc.cognitiveRunRetrySafe(ctx, safeRun.ID); err != nil || !safe {
		t.Fatalf("settled definite failure safe=%v err=%v, want safe", safe, err)
	}

	missingFinish := makeFailedCognitiveEvidenceFixture(t, svc, backend, "sess-cog-missing-finish", "run-cog-missing-parent", "unfinished-call", false, false)
	if safe, err := svc.cognitiveRunRetrySafe(ctx, missingFinish.ID); err != nil || safe {
		t.Fatalf("missing model.call.finished safe=%v err=%v, want unsafe", safe, err)
	}

	unknownEffect := makeFailedCognitiveEvidenceFixture(t, svc, backend, "sess-cog-unknown-effect", "run-cog-effect-parent", "effect-call", true, true)
	if safe, err := svc.cognitiveRunRetrySafe(ctx, unknownEffect.ID); err != nil || safe {
		t.Fatalf("started effects node safe=%v err=%v, want unsafe", safe, err)
	}

	// The first case is also the retry boundary: attempt 1 must retain the
	// persisted Through=9 although SourceHigh has already advanced to 10.
	state, version, err := svc.loadCognitiveState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	parentID := domain.RunID("run-cog-safe-parent")
	revision, err := backend.GetWorkflowRevision(ctx, safeRun.ID)
	if err != nil {
		t.Fatal(err)
	}
	state.StateSchema = cognitiveStateSchema
	state.Phase = "blocked"
	state.Blocked = cognitiveBlockUnknown
	state.SupervisorRunID = string(parentID)
	state.SupervisorSeq = 1
	state.ActiveRunID = string(safeRun.ID)
	state.PendingThrough = 9
	state.SourceHigh = 10
	state.Intent = &cognitiveIntent{ParentRunID: parentID, OperationKey: revision.OperationKey,
		StrategyID: TrustedStrategyDIVA, Input: append(json.RawMessage(nil), revision.InputJSON...), Attempt: 0}
	if err := svc.saveCognitiveState(ctx, state, version); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.TriggerCognitive(ctx); err != nil {
		t.Fatalf("retry safe failed window: %v", err)
	}
	state, _, err = svc.loadCognitiveState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if state.Attempt != 1 || state.Intent == nil || state.Intent.Attempt != 1 || state.PendingThrough != 9 {
		t.Fatalf("retry intent changed window/attempt: %+v", state)
	}
	var retriedInput laputaevolution.Input
	if err := json.Unmarshal(state.Intent.Input, &retriedInput); err != nil {
		t.Fatal(err)
	}
	if retriedInput.Window.After != 0 || retriedInput.Window.Through != 9 || state.SourceHigh != 10 {
		t.Fatalf("retry widened the original window: input=%+v SourceHigh=%d", retriedInput.Window, state.SourceHigh)
	}
}

func TestCognitiveFailedRunWaitsForDurableProjection(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, cognitiveTestModel())
	svc.deps.Cognitive = cognitiveBinding(&fakeCognitiveDomain{}, &fakeSource{high: 9}, backend.Snapshot(), func() int64 { return 50 })
	sessionID := domain.SessionID(cognitiveSupervisorSessionID)
	parentID := domain.RunID("run-cog-pending-projection-parent")
	prepareChildSessionAuthorizer(t, svc, backend, sessionID, parentID, nil)
	workflow, revision := admitCognitiveWorkflowFixture(t, svc, backend, sessionID, parentID,
		"cognitive:activity:0-9:a0", cognitiveInput(t))
	store := newINOFYRunStore(backend)
	ref := inofy.ExecutionRef{RunID: string(workflow.ID), Epoch: 1,
		ProgramDigest: revision.ProgramDigest, HostBindingID: revision.HostBindingID}
	admitData, err := json.Marshal(map[string]any{
		"input_digest": revision.InputDigest,
		"limits":       json.RawMessage(revision.EffectiveLimits),
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, commit := range []inofy.RunCommit{
		{CommitID: "pending-projection-admit", Events: []inofy.Event{{Kind: inofy.EventRunAdmitted, Data: admitData}},
			Transition: inofy.StateTransition{Expected: "", Target: inofy.RunAdmitted}},
		{CommitID: "pending-projection-start", Events: []inofy.Event{{Kind: inofy.EventRunStarted}},
			Transition: inofy.StateTransition{Expected: inofy.RunAdmitted, Target: inofy.RunRunning}},
	} {
		if _, err := store.Commit(ctx, ref, commit); err != nil {
			t.Fatal(err)
		}
	}
	if err := backend.SetRunStatus(ctx, workflow.ID, domain.RunFailed); err != nil {
		t.Fatal(err)
	}
	state, version, err := svc.loadCognitiveState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	state.StateSchema = cognitiveStateSchema
	state.Phase = "running"
	state.ActiveRunID = string(workflow.ID)
	state.PendingThrough = 9
	state.Intent = &cognitiveIntent{ParentRunID: parentID, OperationKey: revision.OperationKey,
		StrategyID: TrustedStrategyDIVA, Input: append(json.RawMessage(nil), revision.InputJSON...), Attempt: 0}
	if err := svc.saveCognitiveState(ctx, state, version); err != nil {
		t.Fatal(err)
	}
	state, version, err = svc.loadCognitiveState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	state, version, err = svc.reconcileCognitiveIntent(ctx, state, version)
	if err != nil {
		t.Fatal(err)
	}
	if state.Blocked != "" || state.Intent == nil || state.ActiveRunID != string(workflow.ID) {
		t.Fatalf("native failure without terminal projection was permanently fenced: %+v", state)
	}

}

func makeFailedCognitiveEvidenceFixture(t *testing.T, svc *Service, backend *sqlite.Backend,
	sessionID domain.SessionID, parentID domain.RunID, callID string, settled, startEffects bool) domain.Run {
	return makeFailedCognitiveEvidenceFixtureAttempt(t, svc, backend, sessionID, parentID, callID, settled, startEffects, 0)
}

func makeFailedCognitiveEvidenceFixtureAttempt(t *testing.T, svc *Service, backend *sqlite.Backend,
	sessionID domain.SessionID, parentID domain.RunID, callID string, settled, startEffects bool, attempt int) domain.Run {
	t.Helper()
	ctx := context.Background()
	prepareChildSessionAuthorizer(t, svc, backend, sessionID, parentID, nil)
	operationKey := fmt.Sprintf("cognitive:activity:0-9:a%d", attempt)
	workflow, revision := admitCognitiveWorkflowFixture(t, svc, backend, sessionID, parentID, operationKey, cognitiveInput(t))
	markCognitiveWorkflowFailed(t, backend, workflow, revision, startEffects)
	childID := domain.RunID("run-cog-infer-" + callID)
	child := domain.Run{ID: childID, SessionID: sessionID, Status: domain.RunFailed,
		Kind: domain.RunKindChild, ChildMode: domain.ChildModeOneShot,
		ParentID: workflow.ID, RootID: workflow.RootID, Depth: workflow.Depth + 1}
	if err := backend.CreateRun(ctx, child); err != nil {
		t.Fatal(err)
	}
	if callID != "safe-call" {
		if err := backend.SetRunStatus(ctx, parentID, domain.RunCompleted); err != nil {
			t.Fatal(err)
		}
	}
	request, err := json.Marshal(payloadModelRequestV3{CallID: callID, Mode: "generate"})
	if err != nil {
		t.Fatal(err)
	}
	events := []domain.RunEvent{
		{Type: domain.EventRunStarted, CreatedAt: 1, PayloadVersion: 1, Payload: []byte(`{"mode":"test"}`)},
		{Type: domain.EventModelRequest, CreatedAt: 2, PayloadVersion: 3, Payload: request},
	}
	if settled {
		finish, err := json.Marshal(payloadModelCallFinished{CallID: callID, Mode: "generate", Status: "failed",
			Error: &payloadModelCallError{Name: "provider", Message: "definite provider failure"}})
		if err != nil {
			t.Fatal(err)
		}
		events = append(events, domain.RunEvent{Type: domain.EventModelCallFinished, CreatedAt: 3, PayloadVersion: 1, Payload: finish})
	}
	events = append(events, domain.RunEvent{Type: domain.EventRunFailed, CreatedAt: 4, PayloadVersion: 1, Payload: []byte(`{"outcome":"failed"}`)})
	if _, err := backend.Append(ctx, storage.Commit{RunID: childID, Events: events}); err != nil {
		t.Fatal(err)
	}
	return workflow
}

func markCognitiveWorkflowFailed(t *testing.T, backend *sqlite.Backend, workflow domain.Run, revision domain.WorkflowRevision, startEffects bool) {
	t.Helper()
	ctx := context.Background()
	store := newINOFYRunStore(backend)
	ref := inofy.ExecutionRef{RunID: string(workflow.ID), Epoch: 1,
		ProgramDigest: revision.ProgramDigest, HostBindingID: revision.HostBindingID}
	admitData, err := json.Marshal(map[string]any{
		"input_digest": revision.InputDigest,
		"limits":       json.RawMessage(revision.EffectiveLimits),
	})
	if err != nil {
		t.Fatal(err)
	}
	commits := []inofy.RunCommit{
		{CommitID: "retry-evidence-admit", Events: []inofy.Event{{Kind: inofy.EventRunAdmitted, Data: admitData}},
			Transition: inofy.StateTransition{Expected: "", Target: inofy.RunAdmitted}},
		{CommitID: "retry-evidence-start", Events: []inofy.Event{{Kind: inofy.EventRunStarted}},
			Transition: inofy.StateTransition{Expected: inofy.RunAdmitted, Target: inofy.RunRunning}},
	}
	for _, commit := range commits {
		if _, err := store.Commit(ctx, ref, commit); err != nil {
			t.Fatalf("commit terminal INOFY evidence: %v", err)
		}
	}
	if startEffects {
		_, err := backend.Append(ctx, storage.Commit{RunID: workflow.ID, Events: []domain.RunEvent{{
			Type: domain.EventWorkflowNodeStarted, CreatedAt: 3, PayloadVersion: 1,
			Payload: []byte(`{"node_key":"/graph/nodes/effects"}`),
		}}})
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.Commit(ctx, ref, inofy.RunCommit{CommitID: "retry-evidence-failed",
		Events:     []inofy.Event{{Kind: inofy.EventRunFailed}},
		Transition: inofy.StateTransition{Expected: inofy.RunRunning, Target: inofy.RunFailed}}); err != nil {
		t.Fatalf("commit terminal INOFY evidence: %v", err)
	}
	if err := backend.SetRunStatus(ctx, workflow.ID, domain.RunFailed); err != nil {
		t.Fatal(err)
	}
}
