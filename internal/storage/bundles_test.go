package storage_test

import (
	"agent-vivy/internal/bundlecontract"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/sqlite"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBundleReceiptsAndCatalog(t *testing.T) {
	ctx := context.Background()
	b, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "toolkits.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	store, ok := any(b).(storage.BundleStore)
	if !ok {
		t.Fatal("SQLite backend lacks BundleStore")
	}
	a, err := store.ReadBundleState(ctx)
	if err != nil || a.Catalog.Epoch != 0 {
		t.Fatal(a, err)
	}
	req := storage.BeginOperation{ID: "01234567-89ab-4def-8123-456789abcdef", InputDigest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Kind: "preview", ExpectedEpoch: 0}
	one, err := store.BeginBundleOperation(ctx, req)
	if err != nil || one.Revision != 1 {
		t.Fatal(one, err)
	}
	replay, err := store.BeginBundleOperation(ctx, req)
	if err != nil || replay.ID != one.ID {
		t.Fatal(replay, err)
	}
	req.InputDigest = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	if _, err := store.BeginBundleOperation(ctx, req); err == nil {
		t.Fatal("changed retry accepted")
	}
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestBundleActivationRequiresLeaseAndDrains(t *testing.T) {
	ctx := context.Background()
	b, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "activate.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	store := b.BundleStore()
	const id = "01234567-89ab-4def-8123-456789abcdef"
	digest := strings.Repeat("a", 64)
	_, err = store.BeginBundleOperation(ctx, storage.BeginOperation{ID: id, InputDigest: digest, Kind: "publish"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.CommitBundleActivation(ctx, storage.ActivationCommit{OperationID: id, InputDigest: digest, Expected: bundlecontract.CatalogIdentity{}, EffectiveDigest: digest, GenerationID: "generation"})
	if err == nil || !strings.Contains(err.Error(), "lease_lost") {
		t.Fatal(err)
	}
	owned, err := b.Acquire(ctx, "vivy/organism", "toolkit-test", time.Minute)
	if err != nil || !owned {
		t.Fatal(owned, err)
	}
	if err := b.CreateSession(ctx, domain.Session{ID: "toolkit-session", Title: "Toolkit", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := b.CreateRun(ctx, domain.Run{ID: "toolkit-run", SessionID: "toolkit-session", Status: domain.RunActive, CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CommitBundleActivation(ctx, storage.ActivationCommit{OperationID: id, InputDigest: digest, LeaseOwner: "toolkit-test", Expected: bundlecontract.CatalogIdentity{}, EffectiveDigest: digest, GenerationID: "generation"}); err == nil || !strings.Contains(err.Error(), "catalog_update_pending") {
		t.Fatal(err)
	}
	if err := b.SetRunStatus(ctx, "toolkit-run", domain.RunCompleted); err != nil {
		t.Fatal(err)
	}
	op, err := store.CommitBundleActivation(ctx, storage.ActivationCommit{OperationID: id, InputDigest: digest, LeaseOwner: "toolkit-test", Expected: bundlecontract.CatalogIdentity{}, EffectiveDigest: digest, GenerationID: "generation"})
	if err != nil || op.CommittedEpoch != 1 {
		t.Fatal(op, err)
	}
	replay, err := store.CommitBundleActivation(ctx, storage.ActivationCommit{OperationID: id, InputDigest: digest, LeaseOwner: "toolkit-test", Expected: bundlecontract.CatalogIdentity{}, EffectiveDigest: digest, GenerationID: "generation"})
	if err != nil || replay.CommittedEpoch != 1 {
		t.Fatal(replay, err)
	}
}

func TestBundleConsentRevocationAndSnapshotRetention(t *testing.T) {
	ctx := context.Background()
	b, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "consent.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	store := b.BundleStore()
	owned, err := b.Acquire(ctx, "vivy/organism", "toolkit-consent", time.Minute)
	if err != nil || !owned {
		t.Fatal(err)
	}
	const install = "01234567-89ab-4def-8123-456789abcdef"
	release := bundlecontract.ReleaseRef{InstallID: install, BundleID: "example/experts", ReleaseDigest: strings.Repeat("a", 64), ReleaseSequence: 1}
	operation := func() string { return fmt.Sprintf("10000000-0000-4000-8000-%012d", time.Now().UnixNano()%999999999999) }
	allow := storage.ConsentCommit{OperationID: operation(), InputDigest: strings.Repeat("b", 64), LeaseOwner: "toolkit-consent", InstallID: install, Kind: "hook", LocalID: "before-tool", DeclarationDigest: strings.Repeat("c", 64), Subject: "human", ExpectedEpoch: 0, ExpectedRevision: 0, Allowed: true}
	if _, err := store.CommitBundleConsent(ctx, allow); err != nil {
		t.Fatal(err)
	}
	revoke := storage.RevocationCommit{OperationID: operation(), InputDigest: strings.Repeat("d", 64), LeaseOwner: "toolkit-consent", InstallID: install, Kind: "hook", LocalID: "before-tool", DeclarationDigest: allow.DeclarationDigest, Subject: "human", ExpectedEpoch: 0, ExpectedRevision: 0}
	if _, err := store.CommitBundleRevocation(ctx, revoke); err != nil {
		t.Fatal(err)
	}
	state, err := store.ReadBundleState(ctx)
	if err != nil || state.RevocationRevision != 1 {
		t.Fatal(state, err)
	}
	if _, err := store.CommitBundleRevocation(ctx, storage.RevocationCommit{OperationID: operation(), InputDigest: strings.Repeat("e", 64), LeaseOwner: "toolkit-consent", InstallID: install, Kind: "hook", LocalID: "before-tool", DeclarationDigest: allow.DeclarationDigest, Subject: "human", ExpectedEpoch: 0, ExpectedRevision: 0}); err == nil {
		t.Fatal("stale revocation revision accepted")
	}
	payload := []byte("immutable snapshot")
	h := sha256.Sum256(payload)
	snapshot := storage.RunBundleSnapshot{RunID: domain.RunID("toolkit-old-run"), SchemaVersion: 1, Payload: payload, PayloadSHA256: hex.EncodeToString(h[:]), Releases: []bundlecontract.ReleaseRef{release}}
	if err := store.SaveRunBundleSnapshot(ctx, snapshot); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.LoadRunBundleSnapshot(ctx, snapshot.RunID)
	if err != nil || string(loaded.Payload) != string(payload) {
		t.Fatal(loaded, err)
	}
	reachable, err := store.ReadBundleReachability(ctx)
	if err != nil || len(reachable.Digests) != 1 || reachable.Digests[0] != release.ReleaseDigest {
		t.Fatal(reachable, err)
	}
}
