package bundles

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agent-vivy/internal/bundlecontract"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage/sqlite"
)

// TestToolkitCorruptSnapshotIsUnavailableWithoutLatestFallback proves a run
// resumes from its exact captured epoch, and that a corrupted or missing
// snapshot fails closed instead of falling back to the currently published
// catalog.
func TestToolkitCorruptSnapshotIsUnavailableWithoutLatestFallback(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	firstSource := t.TempDir()
	secondSource := t.TempDir()
	writeSkillBundle(t, firstSource, "captured epoch one")
	writeSecondSkillBundle(t, secondSource)
	backend, err := sqlite.Open(ctx, filepath.Join(root, "snapshot.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Close()
	if owned, err := backend.Acquire(ctx, "vivy/organism", "snapshot-owner", time.Minute); err != nil || !owned {
		t.Fatal(owned, err)
	}
	log := []string{}
	manager, err := NewManager(Config{
		StateRoot: root, Store: backend.BundleStore(), LeaseOwner: "snapshot-owner",
		Limits:       bundlecontract.DefaultLimits(),
		Capabilities: Capabilities{Owners: map[bundlecontract.ComponentKind]bool{bundlecontract.Skill: true}, Limits: bundlecontract.DefaultLimits()},
		Gate:         testGate{log: &log},
	}, backend.BundleStore())
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close(ctx)

	firstInstallID, firstDigest := installAndEnableSkill(t, manager, firstSource, "71000000-0000-4000-8000-00000000000")
	handle, snapshot, _, err := manager.CaptureRun(ctx, domain.RunID("snapshot-run"))
	if err != nil {
		t.Fatal(err)
	}
	handle.Release()
	if err := backend.SaveRunBundleSnapshot(ctx, snapshot); err != nil {
		t.Fatal(err)
	}
	secondInstallID, _ := installAndEnableSkill(t, manager, secondSource, "72000000-0000-4000-8000-00000000000")
	if secondInstallID == firstInstallID {
		t.Fatal("second fixture did not create a distinct installation")
	}
	current, err := manager.AcquireView(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if current.View().Identity().Epoch != 2 || len(current.View().Components()) != 2 {
		t.Fatalf("current view = %+v, components=%d, want epoch 2 with two components", current.View().Identity(), len(current.View().Components()))
	}
	current.Release()

	loaded, err := backend.LoadRunBundleSnapshot(ctx, "snapshot-run")
	if err != nil {
		t.Fatal(err)
	}
	restored, check, err := manager.RestoreRun(ctx, loaded)
	if err != nil {
		t.Fatalf("restore of the intact snapshot failed: %v", err)
	}
	if restored.View().Identity().Epoch != 1 || len(restored.View().Components()) != 1 {
		t.Fatalf("restored view = %+v, components=%d, want the captured epoch 1 view", restored.View().Identity(), len(restored.View().Components()))
	}
	if check.Catalog.Epoch != 1 {
		t.Fatalf("restored capture check = %+v, want epoch 1", check)
	}
	restored.Release()

	tampered := loaded
	tampered.Payload = append(append([]byte(nil), loaded.Payload...), ' ')
	if handle, _, err := manager.RestoreRun(ctx, tampered); err == nil || !strings.Contains(err.Error(), "snapshot digest mismatch") {
		t.Fatalf("restore of a corrupted snapshot = %v, want snapshot digest mismatch", err)
	} else if handle != nil {
		handle.Release()
		t.Fatal("restore of a corrupted snapshot returned a view")
	}

	artifactDir := filepath.Join(root, "content-bundles", "objects", firstDigest)
	if err := os.RemoveAll(artifactDir); err != nil {
		t.Fatal(err)
	}
	if handle, _, err := manager.RestoreRun(ctx, loaded); err == nil {
		handle.Release()
		t.Fatal("restore succeeded without the captured artifact")
	}
}

func installAndEnableSkill(t *testing.T, manager *Manager, source, operationPrefix string) (string, string) {
	t.Helper()
	ctx := context.Background()
	preview, err := manager.Preview(ctx, PreviewRequest{
		OperationID: operationPrefix + "1",
		Source:      bundlecontract.SourceSpec{Kind: "local", Location: source},
	})
	if err != nil {
		t.Fatal(err)
	}
	installed, err := manager.Install(ctx, InstallRequest{
		OperationID: operationPrefix + "2", PreviewOperationID: preview.OperationID,
		ExpectedReleaseDigest: preview.ReleaseDigest,
	})
	if err != nil || installed.State != "committed" {
		t.Fatalf("install %s = %+v, %v", source, installed, err)
	}
	prepared, err := manager.Prepare(ctx, PrepareRequest{
		OperationID: operationPrefix + "3",
		Mutation:    CatalogMutation{Kind: "enable", InstallID: preview.InstallID},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Publish(ctx, prepared.OperationID); err != nil {
		t.Fatal(err)
	}
	return preview.InstallID, preview.ReleaseDigest
}

func writeSecondSkillBundle(t *testing.T, source string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(source, "skills", "review"), 0700); err != nil {
		t.Fatal(err)
	}
	manifest := `{"apiVersion":"vivy.bundle/v1","id":"example/second","version":"1","name":"Second","components":{"skills":[{"id":"review","path":"skills/review/SKILL.md"}]}}`
	if err := os.WriteFile(filepath.Join(source, bundlecontract.ManifestFile), []byte(manifest), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "skills/review/SKILL.md"), []byte("---\ndescription: Review\n---\nReview"), 0600); err != nil {
		t.Fatal(err)
	}
}
