package bundles

import (
	"agent-vivy/internal/bundlecontract"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/sqlite"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func writeSkillBundle(t *testing.T, source, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(source, "skills", "plan"), 0700); err != nil {
		t.Fatal(err)
	}
	manifest := `{"apiVersion":"vivy.bundle/v1","id":"example/experts","version":"1","name":"Experts","components":{"skills":[{"id":"plan","path":"skills/plan/SKILL.md"}]}}`
	if err := os.WriteFile(filepath.Join(source, "vivy-bundle.json"), []byte(manifest), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "skills/plan/SKILL.md"), []byte("---\ndescription: Plan\n---\n"+body), 0600); err != nil {
		t.Fatal(err)
	}
}

func writeMaskBundle(t *testing.T, source string, maskIDs ...string) {
	t.Helper()
	components := make([]string, 0, len(maskIDs))
	for _, id := range maskIDs {
		path := "masks/" + id + ".md"
		components = append(components, `{"id":"`+id+`","name":"`+id+`","path":"`+path+`"}`)
		if err := os.MkdirAll(filepath.Join(source, "masks"), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(source, filepath.FromSlash(path)), []byte("Mask: "+id), 0600); err != nil {
			t.Fatal(err)
		}
	}
	manifest := `{"apiVersion":"vivy.bundle/v1","id":"example/masks","version":"1","name":"Masks","components":{"masks":[` + strings.Join(components, ",") + `]}}`
	if err := os.WriteFile(filepath.Join(source, "vivy-bundle.json"), []byte(manifest), 0600); err != nil {
		t.Fatal(err)
	}
}

func writeHookBundle(t *testing.T, source string) {
	t.Helper()
	if err := os.MkdirAll(source, 0700); err != nil {
		t.Fatal(err)
	}
	manifest := `{"apiVersion":"vivy.bundle/v1","id":"example/hooks","version":"1","name":"Hooks","components":{"hooks":[{"id":"before-tool","event":"PreToolUse","command":"echo ready","matcher":"*","cwd":"workspace","timeoutMs":1000,"envFrom":{"TOKEN":"ORIGINAL_TOKEN"}}]}}`
	if err := os.WriteFile(filepath.Join(source, bundlecontract.ManifestFile), []byte(manifest), 0600); err != nil {
		t.Fatal(err)
	}
}

func writeMCPBundle(t *testing.T, source string) {
	t.Helper()
	if err := os.MkdirAll(source, 0700); err != nil {
		t.Fatal(err)
	}
	manifest := `{"apiVersion":"vivy.bundle/v1","id":"example/mcp","version":"1","name":"MCP","components":{"mcp":[{"id":"docs","endpoint":"https://example.invalid/mcp","args":[],"resourceBridge":false}]}}`
	if err := os.WriteFile(filepath.Join(source, bundlecontract.ManifestFile), []byte(manifest), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestBundleSkillResources(t *testing.T) {
	ctx := context.Background()
	stateRoot := t.TempDir()
	source := t.TempDir()
	if err := os.MkdirAll(filepath.Join(source, "skills", "guide"), 0700); err != nil {
		t.Fatal(err)
	}
	manifest := `{"apiVersion":"vivy.bundle/v1","id":"example/resources","version":"1","name":"Resources","components":{"skills":[{"id":"guide","path":"skills/guide/SKILL.md"}]},"resources":["references/guide.md","references/binary.dat","references/large.md"]}`
	for path, content := range map[string][]byte{
		"vivy-bundle.json":      []byte(manifest),
		"skills/guide/SKILL.md": []byte("---\ndescription: Guide\n---\nRead safely"),
		"references/guide.md":   []byte("declared UTF-8 guide"),
		"references/binary.dat": {'b', 0, 'i', 'n'},
		"references/large.md":   []byte("this is larger than five bytes"),
	} {
		fullPath := filepath.Join(source, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, content, 0600); err != nil {
			t.Fatal(err)
		}
	}
	artifact, err := Acquire(ctx, AcquireRequest{OperationID: "10000000-0000-4000-8000-000000000041", Source: bundlecontract.SourceSpec{Kind: "local", Location: source}, StateRoot: stateRoot, Limits: bundlecontract.DefaultLimits()})
	if err != nil {
		t.Fatal(err)
	}
	installID := "11111111-1111-4111-8111-111111111111"
	id, err := bundlecontract.SkillID(installID, "guide")
	if err != nil {
		t.Fatal(err)
	}
	release := bundlecontract.ReleaseRef{InstallID: installID, BundleID: "example/resources", ReleaseDigest: artifact.ReleaseDigest(), ReleaseSequence: 1}
	view, err := bundlecontract.NewView(bundlecontract.CatalogIdentity{Epoch: 1}, []bundlecontract.ReleaseRef{release}, []bundlecontract.ComponentRecord{{
		Ref:         bundlecontract.ComponentRef{InstallID: installID, Kind: bundlecontract.Skill, LocalID: "guide"},
		ProjectedID: id, Release: release, ContentDigest: artifact.ReleaseDigest(), Resources: []string{"references/guide.md", "references/binary.dat", "references/large.md"},
		Available: true,
	}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	manager := &Manager{cfg: Config{
		StateRoot: stateRoot, Limits: bundlecontract.DefaultLimits(),
		Capabilities: Capabilities{Owners: map[bundlecontract.ComponentKind]bool{bundlecontract.Skill: true}, Limits: bundlecontract.DefaultLimits()},
	}}
	got, err := manager.ReadSkillResource(ctx, view, id, "references/guide.md")
	if err != nil || got != "declared UTF-8 guide" {
		t.Fatalf("declared resource = %q, err=%v", got, err)
	}
	for _, path := range []string{"../vivy-bundle.json", "not-listed.md"} {
		if _, err := manager.ReadSkillResource(ctx, view, id, path); err == nil {
			t.Fatalf("resource %q was exposed", path)
		}
	}
	if _, err := manager.ReadSkillResource(ctx, view, id, "references/binary.dat"); err == nil {
		t.Fatal("binary resource was exposed as text")
	}
	limited := bundlecontract.DefaultLimits()
	limited.ResourceBytes = 5
	boundedManager := &Manager{cfg: Config{
		StateRoot: stateRoot, Limits: limited,
		Capabilities: Capabilities{Owners: map[bundlecontract.ComponentKind]bool{bundlecontract.Skill: true}, Limits: limited},
	}}
	if _, err := boundedManager.ReadSkillResource(ctx, view, id, "references/large.md"); err == nil {
		t.Fatal("oversized resource was exposed")
	}
	objectResource := filepath.Join(stateRoot, "content-bundles", "objects", artifact.ReleaseDigest(), "files", "references", "guide.md")
	if err := os.Chmod(objectResource, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(objectResource, []byte("tampered resource"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.ReadSkillResource(ctx, view, id, "references/guide.md"); err == nil {
		t.Fatal("resource whose stored digest changed was exposed")
	}
}

type testGate struct {
	log  *[]string
	fail *bool
}

func (g testGate) BeginDrain(context.Context, string) error {
	*g.log = append(*g.log, "drain")
	return nil
}
func (g testGate) WaitIdle(context.Context, string) error {
	*g.log = append(*g.log, "idle")
	if g.fail != nil && *g.fail {
		return errors.New("active runs")
	}
	return nil
}
func (g testGate) EndDrain(string) { *g.log = append(*g.log, "end") }

type testOwners struct {
	log              *[]string
	view             bundlecontract.View
	failAfterPublish *bool
}

func (o *testOwners) View() bundlecontract.View { return o.view }
func (o *testOwners) BeforeCommit(context.Context) error {
	*o.log = append(*o.log, "before-commit")
	return nil
}
func (o *testOwners) AfterPublish(context.Context) error {
	*o.log = append(*o.log, "after-publish")
	if o.failAfterPublish != nil && *o.failAfterPublish {
		*o.failAfterPublish = false
		return errors.New("owner projection failed")
	}
	return nil
}
func (o *testOwners) Publish()                    { *o.log = append(*o.log, "publish") }
func (o *testOwners) Close(context.Context) error { return nil }
func TestToolkitPublishOrder(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	source := t.TempDir()
	if err := os.MkdirAll(filepath.Join(source, "skills", "plan"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "vivy-bundle.json"), []byte(`{"apiVersion":"vivy.bundle/v1","id":"example/experts","version":"1","name":"Experts","components":{"skills":[{"id":"plan","path":"skills/plan/SKILL.md"}]}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "skills/plan/SKILL.md"), []byte("---\ndescription: Plan\n---\nPlan"), 0600); err != nil {
		t.Fatal(err)
	}
	b, err := sqlite.Open(ctx, filepath.Join(root, "db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if owned, e := b.Acquire(ctx, "vivy/organism", "owner", time.Minute); e != nil || !owned {
		t.Fatal(owned, e)
	}
	log := []string{}
	fail := false
	failAfterPublish := false
	manager, err := NewManager(Config{StateRoot: root, Limits: bundlecontract.DefaultLimits(), Capabilities: Capabilities{Owners: map[bundlecontract.ComponentKind]bool{bundlecontract.Skill: true}, Limits: bundlecontract.DefaultLimits()}, Store: b.BundleStore(), Gate: testGate{log: &log, fail: &fail}, LeaseOwner: "owner", PrepareOwners: func(_ context.Context, v bundlecontract.View) (PreparedOwners, error) {
		log = append(log, "prepare")
		return &testOwners{log: &log, view: v, failAfterPublish: &failAfterPublish}, nil
	}}, b.BundleStore())
	if err != nil {
		t.Fatal(err)
	}
	preview, err := manager.Preview(ctx, PreviewRequest{OperationID: "10000000-0000-4000-8000-000000000001", Source: bundlecontract.SourceSpec{Kind: "local", Location: source}})
	if err != nil {
		t.Fatal(err)
	}
	installed, err := manager.Install(ctx, InstallRequest{OperationID: "10000000-0000-4000-8000-000000000002", PreviewOperationID: preview.OperationID, ExpectedReleaseDigest: preview.ReleaseDigest})
	if err != nil || installed.State != "committed" {
		t.Fatal(installed, err)
	}
	prepared, err := manager.Prepare(ctx, PrepareRequest{OperationID: "10000000-0000-4000-8000-000000000003", Mutation: CatalogMutation{Kind: "enable", InstallID: preview.InstallID}})
	if err != nil {
		t.Fatal(err)
	}
	if len(log) != 1 || log[0] != "prepare" {
		t.Fatal(log)
	}
	op, err := manager.Publish(ctx, prepared.OperationID)
	if err != nil || op.CommittedEpoch != 1 {
		t.Fatal(op, err)
	}
	want := []string{"prepare", "drain", "idle", "before-commit", "publish", "after-publish", "end"}
	if len(log) != len(want) {
		t.Fatal(log)
	}
	for i := range want {
		if log[i] != want[i] {
			t.Fatal(log)
		}
	}
	h, err := manager.AcquireView(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Release()
	if h.View().Identity().Epoch != 1 || len(h.View().Components()) != 1 {
		t.Fatal(h.View().Identity(), h.View().Components())
	}
	state, err := b.ReadBundleState(ctx)
	if err != nil || state.Catalog.Epoch != 1 {
		t.Fatal(state, err)
	}
	fail = true
	blocked, err := manager.Prepare(ctx, PrepareRequest{OperationID: "10000000-0000-4000-8000-000000000005", Mutation: CatalogMutation{Kind: "disable", InstallID: preview.InstallID}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = manager.Publish(ctx, blocked.OperationID); err == nil {
		t.Fatal("publication passed a failing drain gate")
	}
	current, err := manager.AcquireView(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if current.View().Identity().Epoch != 1 || len(current.View().Components()) != 1 {
		t.Fatal("failed publication replaced the active View", current.View().Identity(), current.View().Components())
	}
	current.Release()
	fail = false
	retry, err := manager.Prepare(ctx, PrepareRequest{OperationID: "10000000-0000-4000-8000-000000000006", Mutation: CatalogMutation{Kind: "disable", InstallID: preview.InstallID}})
	if err != nil {
		t.Fatal(err)
	}
	failAfterPublish = true
	if op, err = manager.Publish(ctx, retry.OperationID); err == nil || op.CommittedEpoch != 2 || !strings.Contains(err.Error(), "publication_recovery_required") {
		t.Fatalf("post-commit projection failure = (%+v, %v), want committed epoch 2 and recovery-required error", op, err)
	}
	if got := log[len(log)-1]; got != "after-publish" {
		t.Fatalf("drain gate released before post-commit recovery: %v", log)
	}
	if err := manager.Recover(ctx); err != nil {
		t.Fatalf("recover post-commit owner projection: %v", err)
	}
	if got := log[len(log)-1]; got != "end" {
		t.Fatalf("recovery did not release the retained drain gate: %v", log)
	}
	if h.View().Identity().Epoch != 1 || len(h.View().Components()) != 1 {
		t.Fatal("pinned reader lost its original View", h.View().Identity(), h.View().Components())
	}
	current, err = manager.AcquireView(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer current.Release()
	if current.View().Identity().Epoch != 2 || len(current.View().Components()) != 0 {
		t.Fatal("new reader did not get the committed View", current.View().Identity(), current.View().Components())
	}
}

func TestToolkitRestoreRunUsesCapturedReleaseAfterDisable(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	source := t.TempDir()
	writeSkillBundle(t, source, "Pinned skill body")
	b, err := sqlite.Open(ctx, filepath.Join(root, "restore.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if owned, err := b.Acquire(ctx, "vivy/organism", "restore-owner", time.Minute); err != nil || !owned {
		t.Fatal(owned, err)
	}
	limits := bundlecontract.DefaultLimits()
	log := []string{}
	fail := false
	manager, err := NewManager(Config{
		StateRoot: root, Limits: limits, LeaseOwner: "restore-owner",
		Capabilities: Capabilities{Owners: map[bundlecontract.ComponentKind]bool{bundlecontract.Skill: true}, Limits: limits},
		Gate:         testGate{log: &log, fail: &fail},
	}, b.BundleStore())
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close(ctx)
	preview, err := manager.Preview(ctx, PreviewRequest{OperationID: "20000000-0000-4000-8000-000000000001", Source: bundlecontract.SourceSpec{Kind: "local", Location: source}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Install(ctx, InstallRequest{OperationID: "20000000-0000-4000-8000-000000000002", PreviewOperationID: preview.OperationID, ExpectedReleaseDigest: preview.ReleaseDigest}); err != nil {
		t.Fatal(err)
	}
	prepared, err := manager.Prepare(ctx, PrepareRequest{OperationID: "20000000-0000-4000-8000-000000000003", Mutation: CatalogMutation{Kind: "enable", InstallID: preview.InstallID}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Publish(ctx, prepared.OperationID); err != nil {
		t.Fatal(err)
	}
	handle, snapshot, check, err := manager.CaptureRun(ctx, "toolkit-pinned-run")
	if err != nil {
		t.Fatal(err)
	}
	capturedView := handle.View()
	if capturedView.Identity() != check.Catalog || len(capturedView.Components()) != 1 {
		t.Fatalf("captured view = %+v", capturedView)
	}
	if err := b.SaveRunBundleSnapshot(ctx, snapshot); err != nil {
		handle.Release()
		t.Fatal(err)
	}
	handle.Release()

	prepared, err = manager.Prepare(ctx, PrepareRequest{OperationID: "20000000-0000-4000-8000-000000000004", Mutation: CatalogMutation{Kind: "disable", InstallID: preview.InstallID}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Publish(ctx, prepared.OperationID); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.GarbageCollect(ctx); err != nil {
		t.Fatal(err)
	}
	restored, restoredCheck, err := manager.RestoreRun(ctx, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Release()
	if restored.View().Identity() != capturedView.Identity() || len(restored.View().Components()) != 1 || restored.View().Components()[0].Body != "Pinned skill body" {
		t.Fatalf("restored view does not match admitted epoch: identity=%+v components=%+v", restored.View().Identity(), restored.View().Components())
	}
	if !reflect.DeepEqual(restored.View().Releases(), snapshot.Releases) || restoredCheck.Catalog != check.Catalog {
		t.Fatalf("restored capture check = %+v, original = %+v", restoredCheck, check)
	}
}

func TestToolkitRollbackCreatesNewReleaseSequenceForRepeatedDigest(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	source := filepath.Join(root, "origin")
	writeSkillBundle(t, source, "Plan v1")
	b, err := sqlite.Open(ctx, filepath.Join(root, "db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if owned, err := b.Acquire(ctx, "vivy/organism", "owner", time.Minute); err != nil || !owned {
		t.Fatal(owned, err)
	}
	log := []string{}
	m, err := NewManager(Config{
		StateRoot: root, Limits: bundlecontract.DefaultLimits(),
		Capabilities: Capabilities{Owners: map[bundlecontract.ComponentKind]bool{bundlecontract.Skill: true}, Limits: bundlecontract.DefaultLimits()},
		Store:        b.BundleStore(), Gate: testGate{log: &log}, LeaseOwner: "owner",
	}, b.BundleStore())
	if err != nil {
		t.Fatal(err)
	}
	preview1, err := m.Preview(ctx, PreviewRequest{OperationID: "20000000-0000-4000-8000-000000000001", Source: bundlecontract.SourceSpec{Kind: "local", Location: source}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Install(ctx, InstallRequest{OperationID: "20000000-0000-4000-8000-000000000002", PreviewOperationID: preview1.OperationID, ExpectedReleaseDigest: preview1.ReleaseDigest}); err != nil {
		t.Fatal(err)
	}
	initial, err := m.Prepare(ctx, PrepareRequest{OperationID: "20000000-0000-4000-8000-000000000003", Mutation: CatalogMutation{Kind: "enable", InstallID: preview1.InstallID}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = m.Publish(ctx, initial.OperationID); err != nil {
		t.Fatal(err)
	}
	firstDigest := preview1.ReleaseDigest
	writeSkillBundle(t, source, "Plan v2")
	preview2, err := m.Preview(ctx, PreviewRequest{OperationID: "20000000-0000-4000-8000-000000000004", Source: bundlecontract.SourceSpec{Kind: "local", Location: source}})
	if err != nil {
		t.Fatal(err)
	}
	update, err := m.Prepare(ctx, PrepareRequest{OperationID: "20000000-0000-4000-8000-000000000005", Mutation: CatalogMutation{Kind: "update", InstallID: preview1.InstallID, ReleaseDigest: preview2.ReleaseDigest, PreviewOperationID: preview2.OperationID}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = m.Publish(ctx, update.OperationID); err != nil {
		t.Fatal(err)
	}
	rollback, err := m.Prepare(ctx, PrepareRequest{OperationID: "20000000-0000-4000-8000-000000000006", Mutation: CatalogMutation{Kind: "rollback", InstallID: preview1.InstallID, ReleaseDigest: firstDigest}})
	if err != nil {
		t.Fatal(err)
	}
	if op, err := m.Publish(ctx, rollback.OperationID); err != nil || op.CommittedEpoch != 3 {
		t.Fatal(op, err)
	}
	state, err := b.ReadBundleState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Releases) != 3 || state.Releases[2].ReleaseSequence != 3 || state.Releases[2].ReleaseDigest != firstDigest {
		t.Fatalf("rollback history did not retain a fresh sequence: %+v", state.Releases)
	}
	disable, err := m.Prepare(ctx, PrepareRequest{OperationID: "20000000-0000-4000-8000-000000000007", Mutation: CatalogMutation{Kind: "disable", InstallID: preview1.InstallID}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = m.Publish(ctx, disable.OperationID); err != nil {
		t.Fatal(err)
	}
	enable, err := m.Prepare(ctx, PrepareRequest{OperationID: "20000000-0000-4000-8000-000000000008", Mutation: CatalogMutation{Kind: "enable", InstallID: preview1.InstallID}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = m.Publish(ctx, enable.OperationID); err != nil {
		t.Fatal(err)
	}
	view, err := m.AcquireView(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer view.Release()
	if releases := view.View().Releases(); len(releases) != 1 || releases[0].ReleaseSequence != 3 || releases[0].ReleaseDigest != firstDigest {
		t.Fatalf("re-enable did not use the latest rollback receipt: %+v", releases)
	}
}

func TestToolkitUpdateOnlyBlocksRemovedMaskIdentities(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	source := filepath.Join(root, "origin")
	writeMaskBundle(t, source, "architect", "writer")
	old, err := Acquire(ctx, AcquireRequest{OperationID: "30000000-0000-4000-8000-000000000001", Source: bundlecontract.SourceSpec{Kind: "local", Location: source}, StateRoot: root, Limits: bundlecontract.DefaultLimits()})
	if err != nil {
		t.Fatal(err)
	}
	writeMaskBundle(t, source, "architect", "editor")
	updated, err := Acquire(ctx, AcquireRequest{OperationID: "30000000-0000-4000-8000-000000000002", Source: bundlecontract.SourceSpec{Kind: "local", Location: source}, StateRoot: root, Limits: bundlecontract.DefaultLimits()})
	if err != nil {
		t.Fatal(err)
	}
	const installID = "30000000-0000-4000-8000-000000000003"
	m := &Manager{cfg: Config{StateRoot: root}}
	state := storage.BundleState{Installations: []storage.BundleInstallation{{InstallID: installID, BundleID: "example/masks", State: "enabled", ReleaseDigest: old.ReleaseDigest()}}}
	p := PreparedCatalog{mutation: CatalogMutation{Kind: "update", InstallID: installID}, installations: []storage.InstallationState{{InstallID: installID, ReleaseDigest: updated.ReleaseDigest()}}}
	removed, err := m.removedMasks(ctx, p, state)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"bundle/" + installID + "/writer"}
	if len(removed) != len(want) || removed[0] != want[0] {
		t.Fatalf("removed masks = %v, want only changed identity %v", removed, want)
	}
	// A body change under a stable local ID must not invalidate a selection.
	writeMaskBundle(t, source, "architect", "writer")
	stable, err := Acquire(ctx, AcquireRequest{OperationID: "30000000-0000-4000-8000-000000000004", Source: bundlecontract.SourceSpec{Kind: "local", Location: source}, StateRoot: root, Limits: bundlecontract.DefaultLimits()})
	if err != nil {
		t.Fatal(err)
	}
	p.installations[0].ReleaseDigest = stable.ReleaseDigest()
	removed, err = m.removedMasks(ctx, p, state)
	if err != nil || len(removed) != 0 {
		t.Fatalf("stable Mask IDs were treated as removed: %v, %v", removed, err)
	}
}

func TestToolkitGarbageCollectionRespectsInstallRunAndPreviewReferences(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	b, err := sqlite.Open(ctx, filepath.Join(root, "db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if owned, err := b.Acquire(ctx, "vivy/organism", "gc-owner", time.Minute); err != nil || !owned {
		t.Fatal(owned, err)
	}
	acquire := func(id, folder, body string) Artifact {
		source := filepath.Join(root, folder)
		writeSkillBundle(t, source, body)
		a, err := Acquire(ctx, AcquireRequest{OperationID: id, Source: bundlecontract.SourceSpec{Kind: "local", Location: source}, StateRoot: root, Limits: bundlecontract.DefaultLimits()})
		if err != nil {
			t.Fatal(err)
		}
		return a
	}
	installed := acquire("60000000-0000-4000-8000-000000000001", "installed", "installed")
	preview := acquire("60000000-0000-4000-8000-000000000002", "preview", "preview")
	orphan := acquire("60000000-0000-4000-8000-000000000003", "orphan", "orphan")
	paused := acquire("60000000-0000-4000-8000-000000000004", "paused", "paused")
	installID := "60000000-0000-4000-8000-000000000005"
	if _, err := b.BundleStore().SaveBundleInstallation(ctx, storage.InstallCommit{
		OperationID: "60000000-0000-4000-8000-000000000006", InputDigest: strings.Repeat("a", 64),
		Expected:     bundlecontract.CatalogIdentity{},
		Installation: storage.BundleInstallation{InstallID: installID, BundleID: "example/experts", State: "disabled", Source: installed.SourceLock(), ReleaseDigest: installed.ReleaseDigest()},
		Release:      bundlecontract.ReleaseRef{InstallID: installID, BundleID: "example/experts", ReleaseDigest: installed.ReleaseDigest(), ReleaseSequence: 1},
	}); err != nil {
		t.Fatal(err)
	}
	pausedInstall := "60000000-0000-4000-8000-000000000007"
	pausedRelease := bundlecontract.ReleaseRef{InstallID: pausedInstall, BundleID: "example/paused", ReleaseDigest: paused.ReleaseDigest(), ReleaseSequence: 1}
	pausedView, err := bundlecontract.NewView(bundlecontract.CatalogIdentity{
		Epoch: 1, EffectiveDigest: strings.Repeat("b", 64), GenerationID: "generation-paused",
	}, []bundlecontract.ReleaseRef{pausedRelease}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := bundlecontract.EncodeRunSnapshotPayload(bundlecontract.SnapshotPayload(pausedView, 0))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(payload)
	if err := b.BundleStore().SaveRunBundleSnapshot(ctx, storage.RunBundleSnapshot{
		RunID: domain.RunID("paused-toolkit-run"), SchemaVersion: 1, Payload: payload, PayloadSHA256: hex.EncodeToString(sum[:]),
		Releases: []bundlecontract.ReleaseRef{pausedRelease},
	}); err != nil {
		t.Fatal(err)
	}
	m, err := NewManager(Config{StateRoot: root, Store: b.BundleStore(), LeaseOwner: "gc-owner"}, b.BundleStore())
	if err != nil {
		t.Fatal(err)
	}
	previewID := "60000000-0000-4000-8000-000000000008"
	m.previews[previewID] = PreparedSource{OperationID: previewID, ReleaseDigest: preview.ReleaseDigest()}
	result, err := m.GarbageCollect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	assertObject := func(digest string, want bool) {
		t.Helper()
		_, err := os.Stat(filepath.Join(root, "content-bundles", "objects", digest))
		if want && err != nil {
			t.Fatalf("referenced object %s missing: %v", digest, err)
		}
		if !want && !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("unreferenced object %s remains: %v", digest, err)
		}
	}
	assertObject(installed.ReleaseDigest(), true)
	assertObject(preview.ReleaseDigest(), true)
	assertObject(paused.ReleaseDigest(), true)
	assertObject(orphan.ReleaseDigest(), false)
	if len(result.Deleted) != 1 || result.Deleted[0] != orphan.ReleaseDigest() {
		t.Fatalf("first GC result = %+v", result)
	}
	if !m.CancelPreview(previewID) {
		t.Fatal("preview was not cancellable")
	}
	if _, err := m.GarbageCollect(ctx); err != nil {
		t.Fatal(err)
	}
	assertObject(preview.ReleaseDigest(), false)
}

func TestToolkitRecoveryRestoresCommittedEpochAndRejectsMissingArtifact(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	source := filepath.Join(root, "origin")
	writeSkillBundle(t, source, "recovery")
	b, err := sqlite.Open(ctx, filepath.Join(root, "db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if owned, err := b.Acquire(ctx, "vivy/organism", "recovery-owner", time.Minute); err != nil || !owned {
		t.Fatal(owned, err)
	}
	config := Config{
		StateRoot: root, Limits: bundlecontract.DefaultLimits(),
		Capabilities: Capabilities{Owners: map[bundlecontract.ComponentKind]bool{bundlecontract.Skill: true}, Limits: bundlecontract.DefaultLimits()},
		Store:        b.BundleStore(), LeaseOwner: "recovery-owner",
	}
	manager, err := NewManager(config, b.BundleStore())
	if err != nil {
		t.Fatal(err)
	}
	preview, err := manager.Preview(ctx, PreviewRequest{OperationID: "70000000-0000-4000-8000-000000000001", Source: bundlecontract.SourceSpec{Kind: "local", Location: source}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Install(ctx, InstallRequest{OperationID: "70000000-0000-4000-8000-000000000002", PreviewOperationID: preview.OperationID, ExpectedReleaseDigest: preview.ReleaseDigest}); err != nil {
		t.Fatal(err)
	}
	prepared, err := manager.Prepare(ctx, PrepareRequest{OperationID: "70000000-0000-4000-8000-000000000003", Mutation: CatalogMutation{Kind: "enable", InstallID: preview.InstallID}})
	if err != nil {
		t.Fatal(err)
	}
	committed, err := b.BundleStore().CommitBundleActivation(ctx, storage.ActivationCommit{
		OperationID: prepared.OperationID, InputDigest: mustDigestForOperation(prepared), LeaseOwner: "recovery-owner",
		Expected: prepared.Expected, EffectiveDigest: prepared.CandidateDigest, GenerationID: prepared.owners.View().Identity().GenerationID,
		Installations: prepared.installations, Releases: prepared.newReleases,
	})
	if err != nil || committed.CommittedEpoch != 1 {
		t.Fatal(committed, err)
	}
	// Simulate a process stopping after durable Core Storage commit and before
	// exposing its prepared owner pointers.
	if err := prepared.owners.Close(ctx); err != nil {
		t.Fatal(err)
	}
	restarted, err := NewManager(config, b.BundleStore())
	if err != nil {
		t.Fatal(err)
	}
	if err := restarted.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	view, err := restarted.AcquireView(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if view.View().Identity().Epoch != 1 || len(view.View().Components()) != 1 {
		t.Fatal("recovery did not rebuild the committed catalog", view.View().Identity(), view.View().Components())
	}
	view.Release()
	pendingID := "70000000-0000-4000-8000-000000000004"
	if _, err := b.BundleStore().BeginBundleOperation(ctx, storage.BeginOperation{ID: pendingID, InputDigest: strings.Repeat("e", 64), Kind: "publish", ExpectedEpoch: 1}); err != nil {
		t.Fatal(err)
	}
	if err := restarted.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	pending, err := b.BundleStore().GetBundleOperation(ctx, pendingID)
	if err != nil || pending.State != "cancelled" {
		t.Fatalf("uncommitted operation recovery = %+v, %v", pending, err)
	}
	if err := os.RemoveAll(filepath.Join(root, "content-bundles", "objects", preview.ReleaseDigest)); err != nil {
		t.Fatal(err)
	}
	if err := restarted.Recover(ctx); err == nil {
		t.Fatal("recovery accepted a missing committed artifact")
	}
	stillCurrent, err := restarted.AcquireView(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer stillCurrent.Release()
	if stillCurrent.View().Identity().Epoch != 1 || len(stillCurrent.View().Components()) != 1 {
		t.Fatal("failed recovery replaced the prior View", stillCurrent.View().Identity(), stillCurrent.View().Components())
	}
}

func TestToolkitBindingRevisionPublishesReferencesAndInvalidatesConsent(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	source := filepath.Join(root, "hook-toolkit")
	writeHookBundle(t, source)
	b, err := sqlite.Open(ctx, filepath.Join(root, "bindings.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if acquired, err := b.Acquire(ctx, "vivy/organism", "bindings-owner", time.Minute); err != nil || !acquired {
		t.Fatal(acquired, err)
	}
	gateEvents := []string{}
	m, err := NewManager(Config{StateRoot: root, Store: b.BundleStore(), LeaseOwner: "bindings-owner", Gate: testGate{log: &gateEvents}, Limits: bundlecontract.DefaultLimits(), Capabilities: Capabilities{Owners: map[bundlecontract.ComponentKind]bool{bundlecontract.Skill: true, bundlecontract.Mask: true, bundlecontract.Rule: true, bundlecontract.Hook: true, bundlecontract.MCP: true}, Limits: bundlecontract.DefaultLimits()}}, b.BundleStore())
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	preview, err := m.Preview(ctx, PreviewRequest{OperationID: "40000000-0000-4000-8000-000000000001", Source: bundlecontract.SourceSpec{Kind: "local", Location: source}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Install(ctx, InstallRequest{OperationID: "40000000-0000-4000-8000-000000000002", PreviewOperationID: preview.OperationID, ExpectedReleaseDigest: preview.ReleaseDigest}); err != nil {
		t.Fatal(err)
	}
	prepared, err := m.Prepare(ctx, PrepareRequest{OperationID: "40000000-0000-4000-8000-000000000003", Mutation: CatalogMutation{Kind: "enable", InstallID: preview.InstallID}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Publish(ctx, prepared.OperationID); err != nil {
		t.Fatal(err)
	}
	inspection, err := m.InspectInstallation(ctx, preview.InstallID)
	if err != nil || len(inspection.Executables) != 1 {
		t.Fatalf("inspection = %+v, %v", inspection, err)
	}
	consent, err := m.SetExecutableConsent(ctx, ExecutableConsentRequest{OperationID: "40000000-0000-4000-8000-000000000004", Expected: mustCurrentIdentity(t, m), Ref: bundlecontract.ComponentRef{InstallID: preview.InstallID, Kind: bundlecontract.Hook, LocalID: "before-tool"}, ExpectedRevision: 0, ExpectedReleaseDigest: preview.ReleaseDigest, ExpectedDeclarationDigest: inspection.Executables[0].DeclarationDigest, Subject: "human", Allowed: true})
	if err != nil || consent.State != "committed" {
		t.Fatalf("consent = %+v, %v", consent, err)
	}
	inspection, err = m.InspectInstallation(ctx, preview.InstallID)
	if err != nil || !inspection.Executables[0].ConsentCurrent {
		t.Fatalf("fresh consent = %+v, %v", inspection.Executables[0], err)
	}
	identity := mustCurrentIdentity(t, m)
	_, err = m.SetBindings(ctx, BindingUpdateRequest{OperationID: "40000000-0000-4000-8000-000000000005", Expected: identity, ExpectedConfigRevision: identity.ConfigRevision,
		InstallID: preview.InstallID, Kind: bundlecontract.Hook, LocalID: "before-tool", Subject: "human",
		Bindings: []bundlecontract.BindingRef{{Name: "envFrom:TOKEN", Reference: "VIVY_HOOK_TOKEN"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	view, err := m.AcquireView(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var hook bundlecontract.ComponentRecord
	for _, record := range view.View().Components() {
		if record.Ref.Kind == bundlecontract.Hook {
			hook = record
		}
	}
	view.Release()
	if hook.Hook == nil || hook.Hook.EnvFrom["TOKEN"] != "VIVY_HOOK_TOKEN" || len(hook.Bindings) != 1 {
		t.Fatalf("published Hook binding = %+v", hook)
	}
	current := mustCurrentIdentity(t, m)
	if current.ConfigRevision != identity.ConfigRevision+1 || current.Epoch != identity.Epoch {
		t.Fatalf("binding identity = %+v, prior %+v", current, identity)
	}
	inspection, err = m.InspectInstallation(ctx, preview.InstallID)
	if err != nil || inspection.Executables[0].ConsentCurrent || !inspection.Executables[0].ConsentAllowed {
		t.Fatalf("binding update retained executable consent: %+v, %v", inspection.Executables[0], err)
	}
	t.Setenv("VIVY_HOOK_TOKEN", "TEST_SECRET_VALUE")
	inspectionJSON, err := json.Marshal(inspection)
	if err != nil || strings.Contains(string(inspectionJSON), "TEST_SECRET_VALUE") {
		t.Fatalf("inspection leaked binding value: %s %v", inspectionJSON, err)
	}
	handle, snapshot, _, err := m.CaptureRun(ctx, domain.RunID("bindings-snapshot-run"))
	if err != nil {
		t.Fatal(err)
	}
	handle.Release()
	payload, err := bundlecontract.DecodeRunSnapshotPayload(snapshot.Payload)
	if err != nil || len(payload.Components) != 1 || len(payload.Components[0].Bindings) != 1 || payload.Components[0].Bindings[0].Reference != "VIVY_HOOK_TOKEN" {
		t.Fatalf("snapshot did not pin host reference: %+v, %v", payload, err)
	}
	state, err := b.BundleStore().ReadBundleState(ctx)
	if err != nil || state.Catalog.ConfigRevision != current.ConfigRevision || len(state.Bindings) != 1 || state.Bindings[0].Reference != "VIVY_HOOK_TOKEN" {
		t.Fatalf("durable Toolkit binding = %+v, %v", state, err)
	}
}

func TestToolkitExecutableConsentCanBeGrantedBeforeInstallIsEnabled(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	source := filepath.Join(root, "mcp-toolkit")
	writeMCPBundle(t, source)
	b, err := sqlite.Open(ctx, filepath.Join(root, "mcp-consent.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if acquired, err := b.Acquire(ctx, "vivy/organism", "mcp-consent-owner", time.Minute); err != nil || !acquired {
		t.Fatal(acquired, err)
	}
	gateEvents := []string{}
	manager, err := NewManager(Config{
		StateRoot: root, Store: b.BundleStore(), LeaseOwner: "mcp-consent-owner", Gate: testGate{log: &gateEvents},
		Limits:       bundlecontract.DefaultLimits(),
		Capabilities: Capabilities{Owners: map[bundlecontract.ComponentKind]bool{bundlecontract.MCP: true}, Limits: bundlecontract.DefaultLimits()},
	}, b.BundleStore())
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close(ctx)
	if err := manager.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	preview, err := manager.Preview(ctx, PreviewRequest{
		OperationID: "41000000-0000-4000-8000-000000000001",
		Source:      bundlecontract.SourceSpec{Kind: "local", Location: source},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Install(ctx, InstallRequest{
		OperationID: "41000000-0000-4000-8000-000000000002", PreviewOperationID: preview.OperationID,
		ExpectedReleaseDigest: preview.ReleaseDigest,
	}); err != nil {
		t.Fatal(err)
	}
	inspection, err := manager.InspectInstallation(ctx, preview.InstallID)
	if err != nil || len(inspection.Executables) != 1 || inspection.Executables[0].Kind != bundlecontract.MCP {
		t.Fatalf("installed MCP inspection = %+v, %v", inspection, err)
	}
	if inspection.Installation.State != "disabled" {
		t.Fatalf("fresh installation state = %q, want disabled", inspection.Installation.State)
	}
	identity := mustCurrentIdentity(t, manager)
	consent, err := manager.SetExecutableConsent(ctx, ExecutableConsentRequest{
		OperationID: "41000000-0000-4000-8000-000000000003", Expected: identity,
		Ref:              bundlecontract.ComponentRef{InstallID: preview.InstallID, Kind: bundlecontract.MCP, LocalID: "docs"},
		ExpectedRevision: 0, ExpectedReleaseDigest: inspection.Executables[0].ReleaseDigest,
		ExpectedDeclarationDigest: inspection.Executables[0].DeclarationDigest, Subject: "human", Allowed: true,
	})
	if err != nil || consent.State != "committed" {
		t.Fatalf("consent for installed disabled MCP = %+v, %v", consent, err)
	}
	inspection, err = manager.InspectInstallation(ctx, preview.InstallID)
	if err != nil || !inspection.Executables[0].ConsentAllowed || !inspection.Executables[0].ConsentCurrent {
		t.Fatalf("pre-enable consent inspection = %+v, %v", inspection.Executables, err)
	}
	prepared, err := manager.Prepare(ctx, PrepareRequest{
		OperationID: "41000000-0000-4000-8000-000000000004", Expected: mustCurrentIdentity(t, manager),
		Mutation: CatalogMutation{Kind: "enable", InstallID: preview.InstallID},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Publish(ctx, prepared.OperationID); err != nil {
		t.Fatal(err)
	}
	handle, snapshot, _, err := manager.CaptureRun(ctx, domain.RunID("mcp-consent-captured-run"))
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Release()
	payload, err := bundlecontract.DecodeRunSnapshotPayload(snapshot.Payload)
	if err != nil {
		t.Fatal(err)
	}
	captured := false
	for _, component := range payload.Components {
		if component.Ref != (bundlecontract.ComponentRef{InstallID: preview.InstallID, Kind: bundlecontract.MCP, LocalID: "docs"}) {
			continue
		}
		if !component.ConsentPresent || !component.ConsentAllowed || component.ConsentDigest == "" || component.ConsentRevision != 1 {
			t.Fatalf("enabled MCP lost pre-authorized consent: %+v", component)
		}
		captured = true
		break
	}
	if !captured {
		t.Fatal("enabled MCP missing from captured run snapshot")
	}
	manager.mu.Lock()
	manager.recoveryRequired = true
	manager.mu.Unlock()
	_, err = manager.SetExecutableConsent(ctx, ExecutableConsentRequest{
		OperationID: "41000000-0000-4000-8000-000000000005", Expected: mustCurrentIdentity(t, manager),
		Ref:              bundlecontract.ComponentRef{InstallID: preview.InstallID, Kind: bundlecontract.MCP, LocalID: "docs"},
		ExpectedRevision: 1, ExpectedReleaseDigest: inspection.Executables[0].ReleaseDigest,
		ExpectedDeclarationDigest: inspection.Executables[0].DeclarationDigest, Subject: "human", Allowed: false,
	})
	if err == nil || err.Error() != "toolkits: catalog_recovery_required" {
		t.Fatalf("consent during recovery-required state = %v, want catalog_recovery_required", err)
	}
	storedConsent, found, err := b.BundleStore().ReadBundleConsent(ctx, preview.InstallID, bundlecontract.MCP, "docs")
	if err != nil || !found || !storedConsent.Allowed || storedConsent.Revision != 1 {
		t.Fatalf("recovery-required consent request changed durable consent: %+v, found=%v, err=%v", storedConsent, found, err)
	}
}

func TestToolkitInspectionIncludesSecretFreeMCPRuntimeStatus(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	source := filepath.Join(root, "mcp-toolkit")
	writeMCPBundle(t, source)
	b, err := sqlite.Open(ctx, filepath.Join(root, "mcp-status.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if acquired, err := b.Acquire(ctx, "vivy/organism", "mcp-status-owner", time.Minute); err != nil || !acquired {
		t.Fatal(acquired, err)
	}
	statusName := ""
	m, err := NewManager(Config{
		StateRoot: root, Store: b.BundleStore(), LeaseOwner: "mcp-status-owner", Limits: bundlecontract.DefaultLimits(),
		Capabilities: Capabilities{Owners: map[bundlecontract.ComponentKind]bool{bundlecontract.Skill: true, bundlecontract.Mask: true, bundlecontract.Rule: true, bundlecontract.Hook: true, bundlecontract.MCP: true}, Limits: bundlecontract.DefaultLimits()},
		MCPStatuses: func() []MCPRuntimeStatus {
			return []MCPRuntimeStatus{{Name: statusName, State: "ready", Initialized: true, ToolCount: 4}}
		},
	}, b.BundleStore())
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	preview, err := m.Preview(ctx, PreviewRequest{OperationID: "50000000-0000-4000-8000-000000000001", Source: bundlecontract.SourceSpec{Kind: "local", Location: source}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Install(ctx, InstallRequest{OperationID: "50000000-0000-4000-8000-000000000002", PreviewOperationID: preview.OperationID, ExpectedReleaseDigest: preview.ReleaseDigest}); err != nil {
		t.Fatal(err)
	}
	statusName, err = bundlecontract.MCPInstanceID(preview.InstallID, "docs")
	if err != nil {
		t.Fatal(err)
	}
	inspection, err := m.InspectInstallation(ctx, preview.InstallID)
	if err != nil || len(inspection.Executables) != 1 {
		t.Fatalf("inspection = %+v, %v", inspection, err)
	}
	status := inspection.Executables[0]
	if status.RuntimeState != "ready" || !status.RuntimeInitialized || status.RuntimeToolCount != 4 {
		t.Fatalf("MCP status = %+v", status)
	}
	encoded, err := json.Marshal(inspection)
	if err != nil || strings.Contains(string(encoded), "Authorization") || strings.Contains(string(encoded), "secret") {
		t.Fatalf("MCP status contains sensitive values: %s (%v)", encoded, err)
	}
}

func TestToolkitInspectionEncodesEmptyBindingRequirementsAsArray(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	source := filepath.Join(root, "hook-toolkit")
	if err := os.MkdirAll(source, 0700); err != nil {
		t.Fatal(err)
	}
	manifest := `{"apiVersion":"vivy.bundle/v1","id":"example/empty-bindings","version":"1","name":"Empty bindings","components":{"hooks":[{"id":"before-tool","event":"PreToolUse","command":"echo ready","matcher":"*","cwd":"workspace","timeoutMs":1000}]}}`
	if err := os.WriteFile(filepath.Join(source, bundlecontract.ManifestFile), []byte(manifest), 0600); err != nil {
		t.Fatal(err)
	}
	b, err := sqlite.Open(ctx, filepath.Join(root, "hook-inspection.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if acquired, err := b.Acquire(ctx, "vivy/organism", "empty-binding-owner", time.Minute); err != nil || !acquired {
		t.Fatal(acquired, err)
	}
	m, err := NewManager(Config{
		StateRoot: root, Store: b.BundleStore(), LeaseOwner: "empty-binding-owner", Limits: bundlecontract.DefaultLimits(),
		Capabilities: Capabilities{Owners: map[bundlecontract.ComponentKind]bool{
			bundlecontract.Skill: true, bundlecontract.Mask: true, bundlecontract.Rule: true,
			bundlecontract.Hook: true, bundlecontract.MCP: true,
		}, Limits: bundlecontract.DefaultLimits()},
	}, b.BundleStore())
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	preview, err := m.Preview(ctx, PreviewRequest{OperationID: "60000000-0000-4000-8000-000000000001", Source: bundlecontract.SourceSpec{Kind: "local", Location: source}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Install(ctx, InstallRequest{OperationID: "60000000-0000-4000-8000-000000000002", PreviewOperationID: preview.OperationID, ExpectedReleaseDigest: preview.ReleaseDigest}); err != nil {
		t.Fatal(err)
	}
	inspection, err := m.InspectInstallation(ctx, preview.InstallID)
	if err != nil || len(inspection.Executables) != 1 {
		t.Fatalf("inspection = %+v, %v", inspection, err)
	}
	missing := inspection.Executables[0].MissingBindings
	if missing == nil || len(missing) != 0 {
		t.Fatalf("empty binding requirements = %#v, want a non-nil empty list", missing)
	}
	encoded, err := json.Marshal(inspection)
	if err != nil || !strings.Contains(string(encoded), `"missing_bindings":[]`) {
		t.Fatalf("inspection JSON = %s (%v), want an empty array", encoded, err)
	}
}

func mustCurrentIdentity(t *testing.T, manager *Manager) bundlecontract.CatalogIdentity {
	t.Helper()
	handle, err := manager.AcquireView(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Release()
	return handle.View().Identity()
}
