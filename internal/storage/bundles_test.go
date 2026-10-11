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

func TestToolkitBindingsPersistOnlyReferencesAndAdvanceConfigRevision(t *testing.T) {
	ctx := context.Background()
	b, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "bindings.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	store := b.BundleStore()
	const owner = "toolkit-bindings"
	if acquired, err := b.Acquire(ctx, "vivy/organism", owner, time.Minute); err != nil || !acquired {
		t.Fatalf("acquire lease: %v %v", acquired, err)
	}
	const installID = "20000000-0000-4000-8000-000000000001"
	const releaseDigest = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if _, err := store.SaveBundleInstallation(ctx, storage.InstallCommit{
		OperationID: "20000000-0000-4000-8000-000000000002", InputDigest: strings.Repeat("b", 64),
		Expected:     bundlecontract.CatalogIdentity{},
		Installation: storage.BundleInstallation{InstallID: installID, BundleID: "example/bindings", ReleaseDigest: releaseDigest},
		Release:      bundlecontract.ReleaseRef{InstallID: installID, BundleID: "example/bindings", ReleaseDigest: releaseDigest, ReleaseSequence: 1},
	}); err != nil {
		t.Fatal(err)
	}
	state, err := store.ReadBundleState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	req := storage.BindingCommit{
		OperationID: "20000000-0000-4000-8000-000000000003", InputDigest: strings.Repeat("c", 64),
		LeaseOwner: owner, Subject: "human", Expected: state.Catalog, InstallID: installID,
		Kind: "hook", LocalID: "before-tool", EffectiveDigest: strings.Repeat("d", 64),
		Bindings: []storage.ToolkitBinding{{InstallID: installID, Kind: "hook", LocalID: "before-tool", Name: "envFrom:API_TOKEN", Reference: "VIVY_HOOK_TOKEN"}},
	}
	op, err := store.CommitBundleBindings(ctx, req)
	if err != nil || op.State != "committed" || op.Progress != "bindings_updated" {
		t.Fatalf("binding operation = %+v, %v", op, err)
	}
	updated, err := store.ReadBundleState(ctx)
	if err != nil || updated.Catalog.ConfigRevision != 1 || updated.Catalog.EffectiveDigest != req.EffectiveDigest || len(updated.Bindings) != 1 || updated.Bindings[0].Reference != "VIVY_HOOK_TOKEN" {
		t.Fatalf("updated binding state = %+v, %v", updated, err)
	}
	replay, err := store.CommitBundleBindings(ctx, req)
	if err != nil || replay.Revision != op.Revision {
		t.Fatalf("binding retry = %+v, %v", replay, err)
	}
	updated, err = store.ReadBundleState(ctx)
	if err != nil || updated.Catalog.ConfigRevision != 1 || len(updated.Bindings) != 1 {
		t.Fatalf("retry changed configuration twice: %+v, %v", updated, err)
	}
	bad := req
	bad.OperationID = "20000000-0000-4000-8000-000000000004"
	bad.InputDigest = strings.Repeat("e", 64)
	bad.Expected = updated.Catalog
	bad.Bindings = []storage.ToolkitBinding{{InstallID: installID, Kind: "hook", LocalID: "before-tool", Name: "envFrom:API_TOKEN", Reference: "TOKEN=literal-secret"}}
	if _, err := store.CommitBundleBindings(ctx, bad); err == nil {
		t.Fatal("literal secret or assignment was accepted as a binding reference")
	}
}

func TestBundleInstallChecksExpectedCatalogAtomically(t *testing.T) {
	ctx := context.Background()
	b, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "install-cas.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	store := b.BundleStore()
	owned, err := b.Acquire(ctx, "vivy/organism", "toolkit-install-cas", time.Minute)
	if err != nil || !owned {
		t.Fatal(owned, err)
	}
	activationID := "50000000-0000-4000-8000-000000000001"
	activationDigest := strings.Repeat("a", 64)
	if _, err := store.BeginBundleOperation(ctx, storage.BeginOperation{ID: activationID, InputDigest: activationDigest, Kind: "publish"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CommitBundleActivation(ctx, storage.ActivationCommit{OperationID: activationID, InputDigest: activationDigest, LeaseOwner: "toolkit-install-cas", Expected: bundlecontract.CatalogIdentity{}, EffectiveDigest: activationDigest, GenerationID: "generation"}); err != nil {
		t.Fatal(err)
	}
	digest := strings.Repeat("b", 64)
	_, err = store.SaveBundleInstallation(ctx, storage.InstallCommit{
		OperationID: "50000000-0000-4000-8000-000000000002", InputDigest: strings.Repeat("c", 64), Expected: bundlecontract.CatalogIdentity{},
		Installation: storage.BundleInstallation{InstallID: "50000000-0000-4000-8000-000000000003", BundleID: "example/install", State: "disabled", ReleaseDigest: digest},
		Release:      bundlecontract.ReleaseRef{InstallID: "50000000-0000-4000-8000-000000000003", BundleID: "example/install", ReleaseDigest: digest, ReleaseSequence: 1},
	})
	if err == nil || !strings.Contains(err.Error(), "revision_conflict") {
		t.Fatalf("stale install = %v, want revision_conflict", err)
	}
	state, err := store.ReadBundleState(ctx)
	if err != nil || state.Catalog.Epoch != 1 || len(state.Installations) != 0 {
		t.Fatalf("stale install partially committed: %+v, %v", state, err)
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
	view, err := bundlecontract.NewView(bundlecontract.CatalogIdentity{Epoch: 1, EffectiveDigest: strings.Repeat("d", 64), GenerationID: "generation"}, []bundlecontract.ReleaseRef{release}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := bundlecontract.EncodeRunSnapshotPayload(bundlecontract.SnapshotPayload(view, state.RevocationRevision))
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(payload)
	snapshot := storage.RunBundleSnapshot{RunID: domain.RunID("toolkit-old-run"), SchemaVersion: 1, Payload: payload, PayloadSHA256: hex.EncodeToString(h[:]), Releases: []bundlecontract.ReleaseRef{release}}
	if err := store.SaveRunBundleSnapshot(ctx, snapshot); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.LoadRunBundleSnapshot(ctx, snapshot.RunID)
	if err != nil || string(loaded.Payload) != string(payload) || len(loaded.Releases) != 1 || loaded.Releases[0] != release {
		t.Fatal(loaded, err)
	}
	reachable, err := store.ReadBundleReachability(ctx)
	if err != nil || len(reachable.Digests) != 1 || reachable.Digests[0] != release.ReleaseDigest {
		t.Fatal(reachable, err)
	}
}

func TestBundleConsentChecksConfigRevisionInCommitTransaction(t *testing.T) {
	ctx := context.Background()
	b, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "consent-config-revision.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	store := b.BundleStore()
	owned, err := b.Acquire(ctx, "vivy/organism", "toolkit-consent-cas", time.Minute)
	if err != nil || !owned {
		t.Fatal(owned, err)
	}
	state, err := store.ReadBundleState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	const install = "01234567-89ab-4def-8123-456789abcdef"
	const releaseDigest = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if _, err := store.SaveBundleInstallation(ctx, storage.InstallCommit{
		OperationID: "11000000-0000-4000-8000-000000000001", InputDigest: strings.Repeat("b", 64), Expected: state.Catalog,
		Installation: storage.BundleInstallation{InstallID: install, BundleID: "example/mcp", ReleaseDigest: releaseDigest},
		Release:      bundlecontract.ReleaseRef{InstallID: install, BundleID: "example/mcp", ReleaseDigest: releaseDigest, ReleaseSequence: 1},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CommitBundleBindings(ctx, storage.BindingCommit{
		OperationID: "11000000-0000-4000-8000-000000000002", InputDigest: strings.Repeat("c", 64), LeaseOwner: "toolkit-consent-cas",
		Expected: state.Catalog, InstallID: install, Kind: string(bundlecontract.MCP), LocalID: "docs",
		EffectiveDigest: strings.Repeat("d", 64), Subject: "human",
	}); err != nil {
		t.Fatal(err)
	}
	stale := storage.ConsentCommit{
		OperationID: "11000000-0000-4000-8000-000000000003", InputDigest: strings.Repeat("e", 64), LeaseOwner: "toolkit-consent-cas",
		InstallID: install, Kind: string(bundlecontract.MCP), LocalID: "docs", DeclarationDigest: strings.Repeat("f", 64),
		Subject: "human", ExpectedEpoch: state.Catalog.Epoch, ExpectedConfigRevision: state.Catalog.ConfigRevision, ExpectedRevision: 0, Allowed: true,
	}
	if _, err := store.CommitBundleConsent(ctx, stale); err == nil {
		t.Fatal("consent accepted a config revision changed after declaration inspection")
	}
	current, err := store.ReadBundleState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	stale.OperationID = "11000000-0000-4000-8000-000000000004"
	stale.InputDigest = strings.Repeat("1", 64)
	stale.ExpectedConfigRevision = current.Catalog.ConfigRevision
	if _, err := store.CommitBundleConsent(ctx, stale); err != nil {
		t.Fatal(err)
	}
	consent, found, err := store.ReadBundleConsent(ctx, install, bundlecontract.MCP, "docs")
	if err != nil || !found || !consent.Allowed || consent.Revision != 1 {
		t.Fatalf("fresh consent = %+v, found=%v, err=%v", consent, found, err)
	}
}
