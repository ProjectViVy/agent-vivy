package sdk

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/eval"
	"agent-vivy/internal/studiocore"
	"agent-vivy/sdk/generation"
	assemblyv1 "agent-vivy/sdk/internal/assembly"
)

func TestSCXCandidateRollbackRestoresPriorSealedGenerationWithoutJournalMutation(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	uiSource, sourceHash, lockHash := writePackUIFixture(t, repoRoot)
	priorRecipe := filepath.Join(root, "prior-full-ui.yml")
	priorRecipeBody := fmt.Sprintf(`apiVersion: vivy.generation/v1
profile: rollback-prior
modules: [vivy/loop, vivy/model, vivy/tool-host, vivy/storage, vivy/checkpoint, vivy/credential, vivy/sandbox, vivy/presentation-host, example/pack-ui]
sources:
  example/pack-ui: {ref: file:pack-ui, sha256: %s}
exclusive:
  std/ui-root@v1: example/pack-ui
order:
  std/ui-extension@v1: [example/pack-ui]
ui:
  sdkVersion: %s
  root: {id: example.pack-ui-root, moduleId: example/pack-ui, port: std/ui-root@v1, entry: ./root.tsx, export: root, sourceHash: %s, dependencyLockHash: %s, assetHash: %s}
  extensions:
    - {id: example.pack-ui-extension, moduleId: example/pack-ui, port: std/ui-extension@v1, entry: ./extension.tsx, export: extension, sourceHash: %s, dependencyLockHash: %s, assetHash: %s}
`, sourceHash, assemblyv1.UIAssemblySDKVersion, sourceHash, lockHash, strings.Repeat("c", 64), sourceHash, lockHash, strings.Repeat("f", 64))
	if err := os.WriteFile(priorRecipe, []byte(priorRecipeBody), 0o600); err != nil {
		t.Fatal(err)
	}
	pack := func(name, recipe string) Artifact {
		t.Helper()
		artifact, err := Pack(ctx, packOptions{
			Recipe: filepath.Join("..", "..", "recipes", recipe+".vivy.yml"),
			Output: filepath.Join(root, name),
		})
		if err != nil {
			t.Fatal(err)
		}
		return artifact
	}
	prior, err := Pack(ctx, packOptions{Recipe: priorRecipe, Output: filepath.Join(root, "prior"), Sources: []string{uiSource}})
	if err != nil {
		t.Fatal(err)
	}
	if len(prior.Manifest.Catalogs) != 1 || prior.Manifest.Catalogs[0].Digest == "" || len(prior.Manifest.Catalogs[0].Locales) == 0 {
		t.Fatalf("rollback prior lacks sealed catalog identity: %#v", prior.Manifest.Catalogs)
	}
	candidate := pack("candidate", "scx")
	probe, err := eval.Launch(ctx, eval.LaunchRequest{
		Executable: candidate.Binary, EvalRoot: filepath.Join(root, "candidate-eval"), Timeout: 60 * time.Second,
		Isolation: eval.Isolation{BundleDir: filepath.Join("..", "..", "fixtures", "provider")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if probe.Verdict != domain.EvalMixed {
		stderr, readErr := os.ReadFile(filepath.Join(probe.Layout.Root, "stderr.log"))
		t.Fatalf("packed SCX candidate did not boot its generated Assembly: verdict=%s stderr=%q read_error=%v", probe.Verdict, stderr, readErr)
	}

	worktree := filepath.Join(root, "worktree")
	journal := filepath.Join(worktree, "data", "vivy.db")
	if err := os.MkdirAll(filepath.Dir(journal), 0o700); err != nil {
		t.Fatal(err)
	}
	const journalSentinel = "tenant-journal-is-not-release-state"
	if err := os.WriteFile(journal, []byte(journalSentinel), 0o600); err != nil {
		t.Fatal(err)
	}
	studio, err := studiocore.NewService(ctx, studiocore.Options{
		Worktree:   worktree,
		LedgerPath: filepath.Join(worktree, "data", "studio-home", "studio.db"),
		EvalRoot:   filepath.Join(worktree, "data", "studio-home", "evals"),
		Timeout:    60 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = studio.Close() })

	release := func(artifact Artifact, suffix string) domain.Release {
		t.Helper()
		binary, err := os.ReadFile(artifact.Binary)
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(binary)
		gen := domain.Generation{
			ID: artifact.Manifest.GenerationID, ArtifactSHA256: hex.EncodeToString(digest[:]),
			SourceRef: "file:" + artifact.Binary, Phase: domain.GenerationBuilt,
			CreatedAt: time.Now().UnixMilli(),
		}
		if err := studio.Ledger().CreateGeneration(ctx, gen); err != nil {
			t.Fatal(err)
		}
		evaluation := domain.EvalRun{
			ID: "evl_scx_" + suffix, CandidateID: gen.ID,
			Suite: studiocore.SuiteAirgapProbe, Verdict: domain.EvalMixed,
			CreatedAt: time.Now().UnixMilli(),
		}
		if err := studio.Ledger().CreateEvalRun(ctx, evaluation); err != nil {
			t.Fatal(err)
		}
		released, err := studio.Release(ctx, gen.ID, evaluation.ID, studiocore.PromotionActorHuman)
		if err != nil {
			t.Fatal(err)
		}
		return released
	}
	priorRelease := release(prior, "prior")
	candidateRelease := release(candidate, "candidate")

	daily := filepath.Join(root, "daily")
	if _, err := studio.Install(ctx, priorRelease.ID, daily); err != nil {
		t.Fatal(err)
	}
	if _, err := studio.Install(ctx, candidateRelease.ID, daily); err != nil {
		t.Fatal(err)
	}
	assertInstalledGeneration(t, daily, candidate.Manifest.GenerationID)
	priorBinary, err := os.ReadFile(prior.Binary)
	if err != nil {
		t.Fatal(err)
	}
	priorEmbedded, err := generation.ExtractEmbeddedManifest(priorBinary)
	if err != nil {
		t.Fatal(err)
	}

	rolledBack, err := studio.Rollback(ctx, daily)
	if err != nil {
		t.Fatal(err)
	}
	if rolledBack.ReleaseID != priorRelease.ID {
		t.Fatalf("rollback restored release %s, want %s", rolledBack.ReleaseID, priorRelease.ID)
	}
	t.Logf("rollback prior_generation=%s candidate_generation=%s restored_release=%s", prior.Manifest.GenerationID, candidate.Manifest.GenerationID, rolledBack.ReleaseID)
	restored := assertInstalledGeneration(t, daily, prior.Manifest.GenerationID)
	if !reflect.DeepEqual(restored.Catalogs, prior.Manifest.Catalogs) || restored.UI == nil || !reflect.DeepEqual(restored.UI.Catalogs, prior.Manifest.UI.Catalogs) {
		t.Fatalf("rollback did not restore catalog digests/locales with sealed Generation:\nrestored=%#v\nprior=%#v", restored.Catalogs, prior.Manifest.Catalogs)
	}
	restoredBinary, err := os.ReadFile(filepath.Join(daily, "vivy.exe"))
	if err != nil {
		t.Fatal(err)
	}
	restoredEmbedded, err := generation.ExtractEmbeddedManifest(restoredBinary)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(restoredEmbedded, priorEmbedded) {
		t.Fatal("rollback changed the prior artifact's embedded Manifest bytes")
	}
	nextLaunch, err := eval.Launch(ctx, eval.LaunchRequest{
		Executable: filepath.Join(daily, "vivy.exe"), EvalRoot: filepath.Join(root, "rollback-eval"), Timeout: 60 * time.Second,
		Isolation: eval.Isolation{BundleDir: filepath.Join("..", "..", "fixtures", "provider")},
	})
	if err != nil || nextLaunch.Verdict != domain.EvalMixed {
		t.Fatalf("restored Generation did not boot on next launch: verdict=%s error=%v", nextLaunch.Verdict, err)
	}
	journalAfter, err := os.ReadFile(journal)
	if err != nil {
		t.Fatal(err)
	}
	if string(journalAfter) != journalSentinel {
		t.Fatal("SCX candidate install/rollback mutated the tenant Journal")
	}
}

func assertInstalledGeneration(t *testing.T, directory, want string) assemblyv1.GenerationManifest {
	t.Helper()
	binary, err := os.ReadFile(filepath.Join(directory, "vivy.exe"))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := generation.ExtractEmbeddedManifest(binary)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := assemblyv1.InspectManifest(raw)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.GenerationID != want {
		t.Fatalf("installed Generation = %s, want %s", manifest.GenerationID, want)
	}
	return manifest
}
