package conformance

import (
	"agent-vivy/internal/bundlecontract"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type BundleSlot struct {
	Store      storage.BundleStore
	Admission  storage.RunAdmissionStore
	Engine     storage.Engine
	LeaseOwner string
	Close      func() error
}
type BundleHarness struct{ Setup func(*testing.T) BundleSlot }

// RunBundles exercises the backend-independent receipt, cancellation, and
// retained evidence contract against each real Core Storage dialect.
func RunBundles(t *testing.T, h BundleHarness) {
	t.Helper()
	slot := h.Setup(t)
	defer slot.Close()
	ctx := context.Background()
	state, err := slot.Store.ReadBundleState(ctx)
	if err != nil || state.Catalog.Epoch != 0 {
		t.Fatalf("initial toolkit state=%+v, %v", state, err)
	}
	if err := slot.Store.CheckBundleLease(ctx, "unowned-maintenance-process"); err == nil {
		t.Fatal("GC lease check accepted a non-owner")
	}
	id := "10000000-0000-4000-8000-000000000001"
	digest := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	op, err := slot.Store.BeginBundleOperation(ctx, storage.BeginOperation{ID: id, InputDigest: digest, Kind: "preview", ExpectedEpoch: 0})
	if err != nil || op.Revision != 1 {
		t.Fatalf("begin operation=%+v, %v", op, err)
	}
	retry, err := slot.Store.BeginBundleOperation(ctx, storage.BeginOperation{ID: id, InputDigest: digest, Kind: "preview", ExpectedEpoch: 0})
	if err != nil || retry.ID != op.ID {
		t.Fatalf("replay=%+v, %v", retry, err)
	}
	if _, err := slot.Store.BeginBundleOperation(ctx, storage.BeginOperation{ID: id, InputDigest: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", Kind: "preview", ExpectedEpoch: 0}); err == nil {
		t.Fatal("changed operation payload replayed")
	}
	cancelled, err := slot.Store.CancelBundleOperation(ctx, storage.CancelOperation{ID: id, ExpectedRevision: 1})
	if err != nil || cancelled.State != "cancelled" {
		t.Fatalf("cancel=%+v, %v", cancelled, err)
	}
	view, err := bundlecontract.NewView(bundlecontract.CatalogIdentity{}, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := bundlecontract.EncodeRunSnapshotPayload(bundlecontract.SnapshotPayload(view, 0))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(payload)
	snapshot := storage.RunBundleSnapshot{RunID: domain.RunID("bundle-conformance-run"), SchemaVersion: 1, Payload: payload, PayloadSHA256: hex.EncodeToString(sum[:]), Releases: []bundlecontract.ReleaseRef{}}
	if err := slot.Store.SaveRunBundleSnapshot(ctx, snapshot); err != nil {
		t.Fatal(err)
	}
	loaded, err := slot.Store.LoadRunBundleSnapshot(ctx, snapshot.RunID)
	if err != nil || string(loaded.Payload) != string(payload) {
		t.Fatalf("snapshot=%+v, %v", loaded, err)
	}
	if slot.Admission == nil || slot.Engine == nil {
		t.Fatal("bundle harness requires atomic run admission and Engine")
	}
	const sessionID domain.SessionID = "toolkit-admission-conformance"
	if err := slot.Engine.CreateSession(ctx, domain.Session{ID: sessionID, Title: "Toolkit conformance", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	makeAdmission := func(runID domain.RunID, revocationRevision int64) (storage.RunAdmission, error) {
		capturedView, err := bundlecontract.NewView(bundlecontract.CatalogIdentity{}, nil, nil, nil)
		if err != nil {
			return storage.RunAdmission{}, err
		}
		capturedPayload := bundlecontract.SnapshotPayload(capturedView, revocationRevision)
		encoded, err := bundlecontract.EncodeRunSnapshotPayload(capturedPayload)
		if err != nil {
			return storage.RunAdmission{}, err
		}
		capturedDigest := sha256.Sum256(encoded)
		captured := storage.RunBundleSnapshot{RunID: runID, SchemaVersion: 1, Payload: encoded,
			PayloadSHA256: hex.EncodeToString(capturedDigest[:]), Releases: []bundlecontract.ReleaseRef{}}
		capture := bundlecontract.CaptureCheckFor(capturedPayload)
		marker, err := json.Marshal(struct {
			BundleSchema int    `json:"bundle_schema"`
			BundleDigest string `json:"bundle_digest"`
		}{BundleSchema: captured.SchemaVersion, BundleDigest: captured.PayloadSHA256})
		if err != nil {
			return storage.RunAdmission{}, err
		}
		started := domain.RunEvent{RunID: runID, Type: domain.EventRunStarted, PayloadVersion: 1, Payload: marker, CreatedAt: 2}
		return storage.RunAdmission{
			Message: domain.Message{ID: "msg-" + string(runID), SessionID: sessionID, RunID: runID, Role: domain.RoleUser, CreatedAt: 2, Content: "start"},
			Run:     domain.Run{ID: runID, SessionID: sessionID, Status: domain.RunActive, CreatedAt: 2},
			Started: started, Bundle: &captured, ExpectedBundle: &capture,
		}, nil
	}
	atomicRun := domain.RunID("toolkit-admission-atomic")
	atomic, err := makeAdmission(atomicRun, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := slot.Admission.CommitRunAdmission(ctx, atomic); err != nil {
		t.Fatalf("atomic Toolkit run admission: %v", err)
	}
	if _, err := slot.Store.LoadRunBundleSnapshot(ctx, atomicRun); err != nil {
		t.Fatalf("atomic admission omitted snapshot: %v", err)
	}
	failingRun := domain.RunID("toolkit-admission-stale")
	failing, err := makeAdmission(failingRun, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := slot.Admission.CommitRunAdmission(ctx, failing); !errors.Is(err, storage.ErrBundleCaptureConflict) {
		t.Fatalf("stale catalog admission = %v, want capture conflict", err)
	}
	if _, err := slot.Engine.GetRun(ctx, failingRun); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("stale admission left a Run row: %v", err)
	}
	if _, err := slot.Store.LoadRunBundleSnapshot(ctx, failingRun); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("stale admission left snapshot evidence: %v", err)
	}
	messages, err := slot.Engine.ListMessages(ctx, sessionID)
	if err != nil || len(messages) != 1 || messages[0].RunID != atomicRun {
		t.Fatalf("failed atomic admission partially wrote messages: %+v, %v", messages, err)
	}
	leaseOwner := slot.LeaseOwner
	if leaseOwner == "" {
		t.Fatal("bundle harness requires the backend-owned organism lease owner")
	}
	if acquired, err := slot.Engine.Acquire(ctx, "vivy/organism", "unowned-maintenance-process", time.Minute); err != nil || acquired {
		t.Fatalf("non-owner replaced the organism lease: acquired=%v err=%v", acquired, err)
	}
	if err := slot.Store.CheckBundleLease(ctx, leaseOwner); err != nil {
		t.Fatalf("backend organism lease owner was rejected: %v", err)
	}
	const installID = "01234567-89ab-4def-8123-456789abcdef"
	const declarationDigest = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	if _, err := slot.Store.CommitBundleConsent(ctx, storage.ConsentCommit{
		OperationID: "10000000-0000-4000-8000-000000000004", InputDigest: "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
		LeaseOwner: leaseOwner, InstallID: installID, Kind: "hook", LocalID: "before-tool", DeclarationDigest: declarationDigest,
		Subject: "human", ExpectedEpoch: state.Catalog.Epoch, ExpectedRevision: 0, Allowed: true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := slot.Store.CommitBundleRevocation(ctx, storage.RevocationCommit{
		OperationID: "10000000-0000-4000-8000-000000000005", InputDigest: "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
		LeaseOwner: leaseOwner, InstallID: installID, Kind: "hook", LocalID: "before-tool", DeclarationDigest: declarationDigest,
		Subject: "human", ExpectedEpoch: state.Catalog.Epoch, ExpectedRevision: 0,
	}); err != nil {
		t.Fatal(err)
	}
	revoked := bundlecontract.CaptureCheck{Catalog: state.Catalog, RevocationRevision: 0, Executables: []bundlecontract.ExecutableRef{{
		ComponentRef:      bundlecontract.ComponentRef{InstallID: installID, Kind: bundlecontract.Hook, LocalID: "before-tool"},
		DeclarationDigest: declarationDigest,
	}}}
	if err := slot.Store.CheckBundleRevocations(ctx, revoked); !errors.Is(err, storage.ErrBundleRevoked) {
		t.Fatalf("captured exact executable after revocation = %v, want revoked", err)
	}
	unrelated := revoked
	unrelated.Executables = []bundlecontract.ExecutableRef{{
		ComponentRef:      bundlecontract.ComponentRef{InstallID: installID, Kind: bundlecontract.Hook, LocalID: "after-tool"},
		DeclarationDigest: declarationDigest,
	}}
	if err := slot.Store.CheckBundleRevocations(ctx, unrelated); err != nil {
		t.Fatalf("unrelated executable revocation fenced a run: %v", err)
	}
	revoked.RevocationRevision = 1
	if err := slot.Store.CheckBundleRevocations(ctx, revoked); err != nil {
		t.Fatalf("run admitted at revocation revision is incorrectly fenced: %v", err)
	}
}
