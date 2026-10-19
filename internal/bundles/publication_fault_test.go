package bundles

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agent-vivy/internal/bundlecontract"
	"agent-vivy/internal/storage/sqlite"
)

// TestToolkitBeforeCommitFailurePreservesCommittedEpoch proves a failure after
// the owner refresh but before the durable commit leaves the active View and
// the stored epoch untouched, and that a fresh prepare can still publish.
func TestToolkitBeforeCommitFailurePreservesCommittedEpoch(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	source := t.TempDir()
	writeSkillBundle(t, source, "Keep the committed epoch")
	backend, err := sqlite.Open(ctx, filepath.Join(root, "before-commit.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Close()
	if owned, err := backend.Acquire(ctx, "vivy/organism", "before-commit-owner", time.Minute); err != nil || !owned {
		t.Fatal(owned, err)
	}
	log := []string{}
	failBeforeCommit := false
	manager, err := NewManager(Config{
		StateRoot: root, Store: backend.BundleStore(), LeaseOwner: "before-commit-owner",
		Limits:       bundlecontract.DefaultLimits(),
		Capabilities: Capabilities{Owners: map[bundlecontract.ComponentKind]bool{bundlecontract.Skill: true}, Limits: bundlecontract.DefaultLimits()},
		Gate:         testGate{log: &log},
		PrepareOwners: func(_ context.Context, view bundlecontract.View) (PreparedOwners, error) {
			return &testOwners{log: &log, view: view, failBeforeCommit: &failBeforeCommit}, nil
		},
	}, backend.BundleStore())
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close(ctx)
	preview, err := manager.Preview(ctx, PreviewRequest{
		OperationID: "61000000-0000-4000-8000-000000000001",
		Source:      bundlecontract.SourceSpec{Kind: "local", Location: source},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Install(ctx, InstallRequest{
		OperationID: "61000000-0000-4000-8000-000000000002", PreviewOperationID: preview.OperationID,
		ExpectedReleaseDigest: preview.ReleaseDigest,
	}); err != nil {
		t.Fatal(err)
	}
	prepared, err := manager.Prepare(ctx, PrepareRequest{
		OperationID: "61000000-0000-4000-8000-000000000003",
		Mutation:    CatalogMutation{Kind: "enable", InstallID: preview.InstallID},
	})
	if err != nil {
		t.Fatal(err)
	}

	failBeforeCommit = true
	if _, err := manager.Publish(ctx, prepared.OperationID); err == nil || !strings.Contains(err.Error(), "owner preparation failed") {
		t.Fatalf("publish with failing BeforeCommit = %v, want owner preparation failure", err)
	}
	state, err := backend.ReadBundleState(ctx)
	if err != nil || state.Catalog.Epoch != 0 {
		t.Fatalf("failed pre-commit publication advanced the stored epoch: %+v, %v", state.Catalog, err)
	}
	if state.Installations[0].State != "disabled" {
		t.Fatalf("failed pre-commit publication changed the installation: %+v", state.Installations[0])
	}
	handle, err := manager.AcquireView(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if handle.View().Identity().Epoch != 0 || len(handle.View().Components()) != 0 {
		t.Fatalf("failed pre-commit publication replaced the active View: %+v", handle.View().Identity())
	}
	handle.Release()
	if got := log[len(log)-1]; got != "end" {
		t.Fatalf("failed pre-commit publication kept the drain gate closed: %v", log)
	}

	failBeforeCommit = false
	retry, err := manager.Prepare(ctx, PrepareRequest{
		OperationID: "61000000-0000-4000-8000-000000000004",
		Mutation:    CatalogMutation{Kind: "enable", InstallID: preview.InstallID},
	})
	if err != nil {
		t.Fatal(err)
	}
	op, err := manager.Publish(ctx, retry.OperationID)
	if err != nil || op.CommittedEpoch != 1 {
		t.Fatalf("retry publication = %+v, %v, want committed epoch 1", op, err)
	}
	handle, err = manager.AcquireView(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Release()
	if handle.View().Identity().Epoch != 1 || len(handle.View().Components()) != 1 {
		t.Fatalf("retry view = %+v, components=%d", handle.View().Identity(), len(handle.View().Components()))
	}
}

// TestToolkitPreviewSourceChangeDoesNotSubstitute proves installation uses the
// reviewed preview artifact, not the source bytes as they exist at install time.
func TestToolkitPreviewSourceChangeDoesNotSubstitute(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	source := t.TempDir()
	writeSkillBundle(t, source, "reviewed-v1")
	backend, err := sqlite.Open(ctx, filepath.Join(root, "source-change.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Close()
	if owned, err := backend.Acquire(ctx, "vivy/organism", "source-change-owner", time.Minute); err != nil || !owned {
		t.Fatal(owned, err)
	}
	manager, err := NewManager(Config{
		StateRoot: root, Store: backend.BundleStore(), LeaseOwner: "source-change-owner",
		Limits:       bundlecontract.DefaultLimits(),
		Capabilities: Capabilities{Owners: map[bundlecontract.ComponentKind]bool{bundlecontract.Skill: true}, Limits: bundlecontract.DefaultLimits()},
	}, backend.BundleStore())
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close(ctx)
	preview, err := manager.Preview(ctx, PreviewRequest{
		OperationID: "62000000-0000-4000-8000-000000000001",
		Source:      bundlecontract.SourceSpec{Kind: "local", Location: source},
	})
	if err != nil {
		t.Fatal(err)
	}
	writeSkillBundle(t, source, "mutated-v2")
	stale, err := manager.Preview(ctx, PreviewRequest{
		OperationID: "62000000-0000-4000-8000-000000000002",
		Source:      bundlecontract.SourceSpec{Kind: "local", Location: source},
	})
	if err != nil {
		t.Fatal(err)
	}
	if stale.ReleaseDigest == preview.ReleaseDigest {
		t.Fatal("fixture source mutation did not change the previewed digest")
	}
	if _, err := manager.Install(ctx, InstallRequest{
		OperationID: "62000000-0000-4000-8000-000000000003", PreviewOperationID: preview.OperationID,
		ExpectedReleaseDigest: stale.ReleaseDigest,
	}); err == nil || !strings.Contains(err.Error(), "preview_stale") {
		t.Fatalf("install with another preview's digest = %v, want preview_stale", err)
	}
	op, err := manager.Install(ctx, InstallRequest{
		OperationID: "62000000-0000-4000-8000-000000000003", PreviewOperationID: preview.OperationID,
		ExpectedReleaseDigest: preview.ReleaseDigest,
	})
	if err != nil || op.State != "committed" {
		t.Fatalf("install reviewed preview = %+v, %v", op, err)
	}
	page, err := manager.ReadComponent(ctx, preview.InstallID, bundlecontract.Skill, "plan", preview.ReleaseDigest, 0, 4096)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(page.Content, "reviewed-v1") || strings.Contains(page.Content, "mutated-v2") {
		t.Fatalf("installed content = %q, want the reviewed preview bytes", page.Content)
	}
}

// TestToolkitReinstallDoesNotInheritConsent proves a tombstoned installation's
// consent does not carry to a new installation of the same BundleID.
func TestToolkitReinstallDoesNotInheritConsent(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	sourceA := filepath.Join(root, "source-a")
	sourceB := filepath.Join(root, "source-b")
	writeConsentHookBundle(t, sourceA, "true")
	writeConsentHookBundle(t, sourceB, "false")
	backend, err := sqlite.Open(ctx, filepath.Join(root, "reinstall.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Close()
	if owned, err := backend.Acquire(ctx, "vivy/organism", "reinstall-owner", time.Minute); err != nil || !owned {
		t.Fatal(owned, err)
	}
	log := []string{}
	manager, err := NewManager(Config{
		StateRoot: root, Store: backend.BundleStore(), LeaseOwner: "reinstall-owner",
		Limits:       bundlecontract.DefaultLimits(),
		Capabilities: Capabilities{Owners: map[bundlecontract.ComponentKind]bool{bundlecontract.Hook: true}, Limits: bundlecontract.DefaultLimits()},
		Gate:         testGate{log: &log},
	}, backend.BundleStore())
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close(ctx)
	first, err := manager.Preview(ctx, PreviewRequest{
		OperationID: "63000000-0000-4000-8000-000000000001",
		Source:      bundlecontract.SourceSpec{Kind: "local", Location: sourceA},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Install(ctx, InstallRequest{
		OperationID: "63000000-0000-4000-8000-000000000002", PreviewOperationID: first.OperationID,
		ExpectedReleaseDigest: first.ReleaseDigest,
	}); err != nil {
		t.Fatal(err)
	}
	inspection, err := manager.InspectInstallation(ctx, first.InstallID)
	if err != nil || len(inspection.Executables) != 1 || inspection.Executables[0].Kind != bundlecontract.Hook {
		t.Fatalf("first Hook inspection = %+v, %v", inspection.Executables, err)
	}
	if _, err := manager.SetExecutableConsent(ctx, ExecutableConsentRequest{
		OperationID: "63000000-0000-4000-8000-000000000003", Expected: mustCurrentIdentity(t, manager),
		Ref:              bundlecontract.ComponentRef{InstallID: first.InstallID, Kind: bundlecontract.Hook, LocalID: "guard"},
		ExpectedRevision: 0, ExpectedReleaseDigest: inspection.Executables[0].ReleaseDigest,
		ExpectedDeclarationDigest: inspection.Executables[0].DeclarationDigest, Subject: "human", Allowed: true,
	}); err != nil {
		t.Fatal(err)
	}
	uninstall, err := manager.Prepare(ctx, PrepareRequest{
		OperationID: "63000000-0000-4000-8000-000000000004", Expected: mustCurrentIdentity(t, manager),
		Mutation: CatalogMutation{Kind: "uninstall", InstallID: first.InstallID},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Publish(ctx, uninstall.OperationID); err != nil {
		t.Fatal(err)
	}
	firstState, err := manager.InspectInstallation(ctx, first.InstallID)
	if err != nil || !firstState.Installation.Tombstone {
		t.Fatalf("uninstalled installation = %+v, %v, want tombstone", firstState.Installation, err)
	}

	second, err := manager.Preview(ctx, PreviewRequest{
		OperationID: "63000000-0000-4000-8000-000000000005",
		Source:      bundlecontract.SourceSpec{Kind: "local", Location: sourceB},
	})
	if err != nil {
		t.Fatal(err)
	}
	if second.InstallID == first.InstallID {
		t.Fatal("reinstall reused the tombstoned InstallID")
	}
	if _, err := manager.Install(ctx, InstallRequest{
		OperationID: "63000000-0000-4000-8000-000000000006", PreviewOperationID: second.OperationID,
		ExpectedReleaseDigest: second.ReleaseDigest,
	}); err != nil {
		t.Fatal(err)
	}
	reinstalled, err := manager.InspectInstallation(ctx, second.InstallID)
	if err != nil {
		t.Fatal(err)
	}
	if len(reinstalled.Executables) != 1 || reinstalled.Executables[0].ConsentAllowed || reinstalled.Executables[0].ConsentRevision != 0 {
		t.Fatalf("reinstalled Hook inherited consent: %+v", reinstalled.Executables)
	}
	if len(reinstalled.Consents) != 0 {
		t.Fatalf("reinstalled Hook inherited consent records: %+v", reinstalled.Consents)
	}
	if _, found, err := backend.BundleStore().ReadBundleConsent(ctx, second.InstallID, bundlecontract.Hook, "guard"); err != nil || found {
		t.Fatalf("durable consent for the new InstallID = found=%v, %v", found, err)
	}
}

func writeConsentHookBundle(t *testing.T, source, command string) {
	t.Helper()
	if err := os.MkdirAll(source, 0700); err != nil {
		t.Fatal(err)
	}
	manifest := `{"apiVersion":"vivy.bundle/v1","id":"example/reinstall","version":"1","name":"Reinstall Hooks","components":{"hooks":[{"id":"guard","event":"PreToolUse","command":"` + command + `","matcher":"bash","cwd":"bundle","timeoutMs":1000}]}}`
	if err := os.WriteFile(filepath.Join(source, bundlecontract.ManifestFile), []byte(manifest), 0600); err != nil {
		t.Fatal(err)
	}
}
