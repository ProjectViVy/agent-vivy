package sqlite

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agent-vivy/internal/bundlecontract"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

func TestBundleActivationRejectsRemovalOfSelectedMask(t *testing.T) {
	ctx := context.Background()
	b := openBackend(t)
	store := b.BundleStore()
	const installID = "40000000-0000-4000-8000-000000000001"
	const installOperation = "40000000-0000-4000-8000-000000000002"
	digest := strings.Repeat("a", 64)
	if _, err := store.SaveBundleInstallation(ctx, storage.InstallCommit{
		OperationID: installOperation, InputDigest: strings.Repeat("b", 64),
		Installation: storage.BundleInstallation{InstallID: installID, BundleID: "example/masks", State: "disabled", Source: bundlecontract.SourceLock{Kind: "local"}, ReleaseDigest: digest},
		Release:      bundlecontract.ReleaseRef{InstallID: installID, BundleID: "example/masks", ReleaseDigest: digest, ReleaseSequence: 1},
	}); err != nil {
		t.Fatal(err)
	}
	if err := b.CreateSession(ctx, domain.Session{ID: "selected-mask-session", Title: "selected mask", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	maskID := "bundle/" + installID + "/architect"
	if _, err := b.db.ExecContext(ctx, `INSERT INTO session_mask_selections(session_id,mask_id,revision) VALUES(?,?,1)`, "selected-mask-session", maskID); err != nil {
		t.Fatal(err)
	}
	if owned, err := b.Acquire(ctx, "vivy/organism", "toolkit-mask-test", time.Minute); err != nil || !owned {
		t.Fatal(owned, err)
	}
	const operationID = "40000000-0000-4000-8000-000000000003"
	inputDigest := strings.Repeat("c", 64)
	if _, err := store.BeginBundleOperation(ctx, storage.BeginOperation{ID: operationID, InputDigest: inputDigest, Kind: "publish"}); err != nil {
		t.Fatal(err)
	}
	_, err := store.CommitBundleActivation(ctx, storage.ActivationCommit{
		OperationID: operationID, InputDigest: inputDigest, LeaseOwner: "toolkit-mask-test",
		Expected: bundlecontract.CatalogIdentity{}, EffectiveDigest: strings.Repeat("d", 64), GenerationID: "generation",
		Installations: []storage.InstallationState{{InstallID: installID, State: "orphaned", ReleaseDigest: digest, Revision: 1, Tombstone: true}},
		RemovedMasks:  []string{maskID},
	})
	if err == nil || !strings.Contains(err.Error(), "mask_in_use") {
		t.Fatalf("selected Mask removal = %v, want mask_in_use", err)
	}
	state, err := store.ReadBundleState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if state.Catalog.Epoch != 0 || len(state.Installations) != 1 || state.Installations[0].Tombstone {
		t.Fatalf("failed Mask check partially committed: %+v", state)
	}
}

func TestBundleRunAdmissionCommitsSnapshotAtomically(t *testing.T) {
	ctx := context.Background()
	b, err := Open(ctx, filepath.Join(t.TempDir(), "bundle-run-admission.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	const sessionID domain.SessionID = "toolkit-run-snapshot"
	if err := b.CreateSession(ctx, domain.Session{ID: sessionID, Title: "snapshot", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	view, err := bundlecontract.NewView(bundlecontract.CatalogIdentity{}, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	payload := bundlecontract.SnapshotPayload(view, 0)
	encoded, err := bundlecontract.EncodeRunSnapshotPayload(payload)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(encoded)
	runID := domain.RunID("toolkit-snapshot-run")
	snapshot := storage.RunBundleSnapshot{
		RunID: runID, SchemaVersion: 1, Payload: encoded, PayloadSHA256: hex.EncodeToString(sum[:]), Releases: payload.Releases,
	}
	check := bundlecontract.CaptureCheckFor(payload)
	startedPayload, err := json.Marshal(map[string]any{
		"provider": "test", "model": "model", "mode": "normal", "bundle_schema": 1,
		"bundle_digest": snapshot.PayloadSHA256,
	})
	if err != nil {
		t.Fatal(err)
	}
	started := domain.RunEvent{RunID: runID, Type: domain.EventRunStarted, CreatedAt: 2, PayloadVersion: 1, Payload: startedPayload}
	message := domain.Message{ID: "toolkit-snapshot-message", SessionID: sessionID, RunID: runID, Role: domain.RoleUser, CreatedAt: 2, Content: "capture"}
	if _, err := b.CommitRunAdmission(ctx, storage.RunAdmission{
		Message: message, Run: domain.Run{ID: runID, SessionID: sessionID, Status: domain.RunActive, CreatedAt: 2},
		Started: started, Bundle: &snapshot, ExpectedBundle: &check,
	}); err != nil {
		t.Fatalf("commit run admission: %v", err)
	}
	loaded, err := b.LoadRunBundleSnapshot(ctx, runID)
	if err != nil {
		t.Fatalf("load atomically admitted snapshot: %v", err)
	}
	if loaded.PayloadSHA256 != snapshot.PayloadSHA256 || !bytes.Equal(loaded.Payload, snapshot.Payload) || len(loaded.Releases) != 0 {
		t.Fatalf("loaded bundle snapshot differs: %+v", loaded)
	}
}
