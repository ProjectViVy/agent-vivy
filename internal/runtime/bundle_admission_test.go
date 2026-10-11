package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"agent-vivy/internal/bundlecontract"
	"agent-vivy/internal/bundles"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/testsupport"
)

type managerToolkitRuntime struct{ manager *bundles.Manager }

type managerToolkitHandle struct{ handle *bundles.ViewHandle }

func (h managerToolkitHandle) View() bundlecontract.View { return h.handle.View() }
func (h managerToolkitHandle) Retain() ToolkitViewHandle {
	retained := h.handle.Retain()
	if retained == nil {
		return nil
	}
	return managerToolkitHandle{handle: retained}
}
func (h managerToolkitHandle) Release() { h.handle.Release() }

func (m managerToolkitRuntime) CaptureRun(ctx context.Context, runID domain.RunID) (ToolkitViewHandle, storage.RunBundleSnapshot, bundlecontract.CaptureCheck, error) {
	handle, snapshot, check, err := m.manager.CaptureRun(ctx, runID)
	if err != nil {
		return nil, snapshot, check, err
	}
	return managerToolkitHandle{handle: handle}, snapshot, check, nil
}
func (m managerToolkitRuntime) RestoreRun(ctx context.Context, snapshot storage.RunBundleSnapshot) (ToolkitViewHandle, bundlecontract.CaptureCheck, error) {
	handle, check, err := m.manager.RestoreRun(ctx, snapshot)
	if err != nil {
		return nil, check, err
	}
	return managerToolkitHandle{handle: handle}, check, nil
}
func (m managerToolkitRuntime) CheckLive(ctx context.Context, check bundlecontract.CaptureCheck) error {
	return m.manager.CheckLive(ctx, check)
}

func wireEmptyToolkitRuntime(t *testing.T, svc *Service, backend *sqlite.Backend) *bundles.Manager {
	t.Helper()
	limits := bundlecontract.DefaultLimits()
	manager, err := bundles.NewManager(bundles.Config{
		StateRoot: t.TempDir(), Limits: limits,
		Capabilities: bundles.Capabilities{Owners: map[bundlecontract.ComponentKind]bool{}, Limits: limits},
	}, backend.BundleStore())
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = manager.Close(context.Background()) })
	svc.deps.Admission = backend
	svc.deps.Bundles = backend
	svc.deps.GenerationID = "toolkit-test-generation"
	svc.SetToolkitRuntime(managerToolkitRuntime{manager: manager})
	return manager
}

func TestToolkitDrainRejectsNewRootAdmission(t *testing.T) {
	svc, backend, _ := newTestService(t, testsupport.NewEchoModel())
	ctx := context.Background()
	const sessionID domain.SessionID = "toolkit-drain-root"
	mustCreateSession(t, backend, sessionID)
	svc.deps.Bundles = backend.BundleStore()

	if err := svc.BeginDrain(ctx, "90000000-0000-4000-8000-000000000001"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { svc.EndDrain("90000000-0000-4000-8000-000000000001") })

	if _, err := svc.Run(ctx, sessionID, "must wait"); !errors.Is(err, ErrCatalogUpdatePending) {
		t.Fatalf("run during publication = %v, want catalog update pending", err)
	}
	runs, err := backend.ListRunsBySession(ctx, sessionID)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 0 {
		t.Fatalf("drained root admission persisted %d runs", len(runs))
	}
}

func TestToolkitDrainWaitIdleIncludesDurableSuspendedRuns(t *testing.T) {
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "toolkit-drain.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	const sessionID domain.SessionID = "toolkit-drain-suspended"
	if err := backend.CreateSession(ctx, domain.Session{ID: sessionID, Title: "suspended", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := backend.CreateRun(ctx, domain.Run{ID: "toolkit-suspended-run", SessionID: sessionID, Status: domain.RunActive, CreatedAt: 2}); err != nil {
		t.Fatal(err)
	}
	service := NewService(nil, "", "", ServiceDeps{Runs: backend, Bundles: backend.BundleStore()})
	if err := service.BeginDrain(ctx, "90000000-0000-4000-8000-000000000002"); err != nil {
		t.Fatal(err)
	}
	defer service.EndDrain("90000000-0000-4000-8000-000000000002")

	waitCtx, cancel := context.WithTimeout(ctx, 80*time.Millisecond)
	defer cancel()
	if err := service.WaitCatalogIdle(waitCtx, "90000000-0000-4000-8000-000000000002"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("WaitIdle with a durable suspended run = %v, want deadline", err)
	}
	if err := backend.SetRunStatus(ctx, "toolkit-suspended-run", domain.RunCompleted); err != nil {
		t.Fatal(err)
	}
	idleCtx, idleCancel := context.WithTimeout(ctx, time.Second)
	defer idleCancel()
	if err := service.WaitCatalogIdle(idleCtx, "90000000-0000-4000-8000-000000000002"); err != nil {
		t.Fatalf("WaitIdle after durable run closes: %v", err)
	}
}

func TestToolkitRootAdmissionStoresSnapshotAndMarker(t *testing.T) {
	svc, backend, _ := newTestService(t, testsupport.NewEchoModel())
	ctx := context.Background()
	const sessionID domain.SessionID = "toolkit-snapshot-root"
	mustCreateSession(t, backend, sessionID)
	wireEmptyToolkitRuntime(t, svc, backend)
	t.Cleanup(func() {
		svc.CancelAll()
		_ = svc.WaitIdle(context.Background())
	})

	runID, err := svc.Run(ctx, sessionID, "capture the catalog")
	if err != nil {
		t.Fatal(err)
	}
	if !svc.WaitIdle(ctx) {
		t.Fatal("run did not finish")
	}
	snapshot, err := backend.LoadRunBundleSnapshot(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := storage.ValidateRunBundleSnapshot(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.RunID != runID || payload.Catalog.Epoch != 0 || len(payload.Releases) != 0 {
		t.Fatalf("unexpected empty-catalog capture: %+v %+v", snapshot, payload)
	}
	it, err := backend.Replay(ctx, runID, 0)
	if err != nil {
		t.Fatal(err)
	}
	var started payloadRunStarted
	for it.Next() {
		event := it.Value().Event
		if event.Type == domain.EventRunStarted {
			if err := json.Unmarshal(event.Payload, &started); err != nil {
				t.Fatal(err)
			}
			break
		}
	}
	if err := it.Err(); err != nil {
		t.Fatal(err)
	}
	if err := it.Close(); err != nil {
		t.Fatal(err)
	}
	if started.BundleSchema != snapshot.SchemaVersion || started.BundleDigest != snapshot.PayloadSHA256 || started.BundleEpoch != payload.Catalog.Epoch {
		t.Fatalf("run.started marker = %+v, snapshot = %+v", started, snapshot)
	}
	if err := svc.ensureToolkitLive(ctx, runID); err != nil {
		t.Fatalf("terminal run must revalidate its persisted capture: %v", err)
	}
}

func TestToolkitChildAdmissionInheritsParentCapture(t *testing.T) {
	svc, backend, _ := newTestService(t, testsupport.NewEchoModel())
	ctx := context.Background()
	const sessionID domain.SessionID = "toolkit-child-parent"
	mustCreateSession(t, backend, sessionID)
	manager := wireEmptyToolkitRuntime(t, svc, backend)
	t.Cleanup(func() { svc.cleanupRunState("toolkit-parent-run") })

	parentID := domain.RunID("toolkit-parent-run")
	parentHandle, parentSnapshot, parentCheck, err := managerToolkitRuntime{manager}.CaptureRun(ctx, parentID)
	if err != nil {
		t.Fatal(err)
	}
	startedPayload, _ := json.Marshal(payloadRunStarted{
		Provider: "test", Model: "test-model", Mode: string(domain.RunModeNormal), Face: string(domain.FaceWeb),
		BundleSchema: parentSnapshot.SchemaVersion, BundleDigest: parentSnapshot.PayloadSHA256,
		BundleEpoch: parentCheck.Catalog.Epoch, BundleCatalogDigest: parentCheck.Catalog.EffectiveDigest,
	})
	started := domain.RunEvent{RunID: parentID, Type: domain.EventRunStarted, PayloadVersion: 1, Payload: startedPayload, CreatedAt: 2}
	if _, err := backend.CommitRunAdmission(ctx, storage.RunAdmission{
		Message: domain.Message{ID: "toolkit-parent-message", SessionID: sessionID, RunID: parentID, Role: domain.RoleUser, CreatedAt: 2, Content: "parent"},
		Run:     domain.Run{ID: parentID, SessionID: sessionID, Status: domain.RunActive, CreatedAt: 2},
		Started: started, Bundle: &parentSnapshot, ExpectedBundle: &parentCheck,
	}); err != nil {
		parentHandle.Release()
		t.Fatal(err)
	}
	svc.adoptToolkitRun(parentID, parentHandle, &parentCheck)

	childSession := domain.Session{ID: sessionID}
	child := domain.Run{ID: "toolkit-child-run", SessionID: sessionID, Status: domain.RunAccepted, CreatedAt: 3,
		Kind: domain.RunKindChild, ChildMode: domain.ChildModeOneShot, ParentID: parentID, RootID: parentID, Depth: 1}
	admissionStore := svc.deps.Admission
	svc.deps.Admission = nil
	admission, childHandle, childCheck, err := svc.buildChildRunAdmission(ctx, childSession, child, "child task", domain.PolicySnapshot{Profile: domain.PolicyProfileDefault, Hash: "policy"}, "workspace")
	svc.deps.Admission = admissionStore
	if err != nil {
		t.Fatal(err)
	}
	if childHandle != nil {
		defer childHandle.Release()
	}
	if admission.Bundle == nil || childCheck == nil || admission.Bundle.RunID != child.ID || admission.Bundle.PayloadSHA256 != parentSnapshot.PayloadSHA256 || childCheck.Catalog != parentCheck.Catalog {
		t.Fatalf("child capture differs from parent: bundle=%+v check=%+v", admission.Bundle, childCheck)
	}
	admission.Message = domain.Message{}
	admission.OmitMessage = true
	if _, err := backend.CommitRunAdmission(ctx, admission); err != nil {
		t.Fatalf("commit child admission: %v (cause: %v)", err, errors.Unwrap(err))
	}
	storedChild, err := backend.LoadRunBundleSnapshot(ctx, child.ID)
	if err != nil || storedChild.PayloadSHA256 != parentSnapshot.PayloadSHA256 {
		t.Fatalf("child snapshot = %+v, %v", storedChild, err)
	}
}

func TestToolkitDrainKeepsQueuedTurnAndResumesAfterPublish(t *testing.T) {
	svc, backend, _ := newTestService(t, testsupport.NewEchoModel())
	ctx := context.Background()
	const sessionID domain.SessionID = "toolkit-queued-follow-up"
	mustCreateSession(t, backend, sessionID)
	svc.deps.Bundles = backend.BundleStore()
	q, err := svc.queueFor(ctx, sessionID)
	if err != nil {
		t.Fatal(err)
	}
	q.mu.Lock()
	q.followUp = append(q.followUp, domain.QueuedTurn{ID: "toolkit-queued-turn", SessionID: sessionID,
		Track: domain.QueueTrackFollowUp, Text: "queued through publication", CreatedAt: time.Now()})
	q.mu.Unlock()
	const operationID = "90000000-0000-4000-8000-000000000003"
	if err := svc.BeginDrain(ctx, operationID); err != nil {
		t.Fatal(err)
	}
	svc.drainFollowUps(sessionID, "old-run")
	q.mu.Lock()
	stillQueued := len(q.followUp) == 1
	q.mu.Unlock()
	if !stillQueued {
		t.Fatal("queued turn was consumed while catalog publication held admission")
	}
	svc.EndDrain(operationID)
	deadline := time.Now().Add(5 * time.Second)
	var runs []domain.Run
	for time.Now().Before(deadline) {
		runs, err = backend.ListRunsBySession(ctx, sessionID)
		if err != nil {
			t.Fatal(err)
		}
		if len(runs) != 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(runs) == 0 {
		t.Fatal("queued turn was not admitted after publication ended")
	}
	if !svc.WaitIdle(ctx) {
		t.Fatal("queued run did not finish")
	}
	messages, err := backend.ListMessages(ctx, sessionID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, message := range messages {
		if message.Content == "queued through publication" {
			found = true
		}
	}
	if !found {
		t.Fatalf("queued turn was not preserved in admitted messages: %+v", messages)
	}
}

func TestToolkitRevocationStopsModelAndToolBoundaries(t *testing.T) {
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "toolkit-revocation.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	const owner = "toolkit-revocation-owner"
	if owned, err := backend.Acquire(ctx, "vivy/organism", owner, time.Minute); err != nil || !owned {
		t.Fatal(owned, err)
	}
	store := backend.BundleStore()
	const installID = "01234567-89ab-4def-8123-456789abcdef"
	const declarationDigest = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	if _, err := store.CommitBundleConsent(ctx, storage.ConsentCommit{
		OperationID: "30000000-0000-4000-8000-000000000001", InputDigest: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		LeaseOwner: owner, InstallID: installID, Kind: "hook", LocalID: "before-tool", DeclarationDigest: declarationDigest,
		Subject: "human", ExpectedEpoch: 0, ExpectedRevision: 0, Allowed: true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CommitBundleRevocation(ctx, storage.RevocationCommit{
		OperationID: "30000000-0000-4000-8000-000000000002", InputDigest: "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
		LeaseOwner: owner, InstallID: installID, Kind: "hook", LocalID: "before-tool", DeclarationDigest: declarationDigest,
		Subject: "human", ExpectedEpoch: 0, ExpectedRevision: 0,
	}); err != nil {
		t.Fatal(err)
	}
	manager, err := bundles.NewManager(bundles.Config{StateRoot: t.TempDir()}, store)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = manager.Close(context.Background()) })
	svc := NewService(nil, "", "", ServiceDeps{Journal: backend, Runs: backend, Bundles: store, Toolkits: managerToolkitRuntime{manager}})
	const sessionID domain.SessionID = "toolkit-revocation-session"
	const runID domain.RunID = "toolkit-revoked-run"
	if err := backend.CreateSession(ctx, domain.Session{ID: sessionID, Title: "revocation", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := backend.CreateRun(ctx, domain.Run{ID: runID, SessionID: sessionID, Status: domain.RunActive, CreatedAt: 2}); err != nil {
		t.Fatal(err)
	}
	svc.bundleChecks[runID] = bundlecontract.CaptureCheck{
		Catalog: bundlecontract.CatalogIdentity{}, RevocationRevision: 0,
		Executables: []bundlecontract.ExecutableRef{{
			ComponentRef:      bundlecontract.ComponentRef{InstallID: installID, Kind: bundlecontract.Hook, LocalID: "before-tool"},
			DeclarationDigest: declarationDigest,
		}},
	}

	model := &promptBudgetModel{inner: WrapModel(testsupport.NewEchoModel())}
	modelCtx := withBundleFence(ctx, func(boundary context.Context) error { return svc.ensureToolkitLive(boundary, runID) })
	if _, err := model.Generate(modelCtx, nil); !errors.Is(err, storage.ErrBundleRevoked) {
		t.Fatalf("model boundary after revocation = %v, want bundle revoked", err)
	}
	toolGate := serviceToolOperationCoordinator{service: svc, runID: runID, sessionID: sessionID}
	if err := toolGate.ensureRunCanExecute(ctx); !errors.Is(err, storage.ErrBundleRevoked) {
		t.Fatalf("tool boundary after revocation = %v, want bundle revoked", err)
	}
}
