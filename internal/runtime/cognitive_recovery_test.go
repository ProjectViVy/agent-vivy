package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/ProjectViVy/inofy"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/testsupport"
	laputaevolution "github.com/dashimaki/laputa/evolution"
)

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

	wfRun, revision := admitCognitiveWorkflowFixture(t, svc, backend, sessionID, parentRunID, "cog-op-running", nil)
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
	if _, err := svc.StartCognitiveWorkflow(ctx, parentRunID, "cog-op-running", TrustedStrategyDIVA, cognitiveInput(t)); !errors.Is(err, ErrWorkflowRecoveryRequired) {
		t.Fatalf("duplicate start on interrupted run = %v", err)
	}
	// Controller boundary setup: bind the original admitted window to the
	// actual recovered engine record. It must expose a durable block rather
	// than describing a lost process as still actively doing useful work.
	svc.deps.Cognitive.SourceID = "activity"
	svc.deps.Cognitive.Source = &fakeSource{high: 5}
	svc.deps.Cognitive.Store = backend.Snapshot()
	svc.deps.Cognitive.Policy = laputaevolution.TriggerPolicy{Enabled: true}
	st, version, err := svc.loadCognitiveState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	st.ActiveRunID, st.PendingThrough = string(wfRun.ID), 5
	if err := svc.saveCognitiveState(ctx, st, version); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		elig, err := svc.TriggerCognitive(ctx)
		if err != nil || elig.Run || string(elig.Reason) != "blocked:"+cognitiveBlockUnknown {
			t.Fatalf("interrupted controller not visibly fenced: %+v %v", elig, err)
		}
	}
	view, watermark, err := svc.CognitiveStatus(ctx)
	if err != nil || view.Phase != "blocked" || view.BlockReason != cognitiveBlockUnknown || view.ActiveRunID != string(wfRun.ID) || view.PendingThrough != 5 || watermark != 0 {
		t.Fatalf("original recovery window not exposed: %+v watermark=%d %v", view, watermark, err)
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
