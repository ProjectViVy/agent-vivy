package sdk

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
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
	prior := pack("prior", "default")
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

	rolledBack, err := studio.Rollback(ctx, daily)
	if err != nil {
		t.Fatal(err)
	}
	if rolledBack.ReleaseID != priorRelease.ID {
		t.Fatalf("rollback restored release %s, want %s", rolledBack.ReleaseID, priorRelease.ID)
	}
	assertInstalledGeneration(t, daily, prior.Manifest.GenerationID)
	journalAfter, err := os.ReadFile(journal)
	if err != nil {
		t.Fatal(err)
	}
	if string(journalAfter) != journalSentinel {
		t.Fatal("SCX candidate install/rollback mutated the tenant Journal")
	}
}

func assertInstalledGeneration(t *testing.T, directory, want string) {
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
}
