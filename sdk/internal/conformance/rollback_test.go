package conformance_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/studiocore"
	"agent-vivy/sdk/generation"
	assemblyv1 "agent-vivy/sdk/internal/assembly"
)

func TestGenerationRollbackRestoresCatalogAndLocaleIdentity(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	catalog := rollbackCatalog()
	priorManifest, priorRaw, err := assemblyv1.SealManifest(assemblyv1.AssemblyPlan{}, assemblyv1.SealInputs{
		SpecificationVersion: "vivy.module/v1", CompilerVersion: "plg-p9-test", SDKVersion: "v1",
		CanonicalRecipe: []byte(`{"apiVersion":"vivy.generation/v1","modules":["fixture/catalog"]}`),
		Catalogs:        []assemblyv1.CatalogManifest{catalog},
		UI:              &assemblyv1.UIAssemblyManifest{SDKPackage: assemblyv1.UIAssemblySDKPackageName, SDKVersion: assemblyv1.UIAssemblySDKVersion, Catalogs: []assemblyv1.CatalogManifest{catalog}},
	})
	if err != nil {
		t.Fatal(err)
	}
	candidateManifest, candidateRaw, err := assemblyv1.SealManifest(assemblyv1.AssemblyPlan{}, assemblyv1.SealInputs{
		SpecificationVersion: "vivy.module/v1", CompilerVersion: "plg-p9-test", SDKVersion: "v1",
		CanonicalRecipe: []byte(`{"apiVersion":"vivy.generation/v1","modules":[]}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	writeArtifact := func(name string, raw []byte) string {
		t.Helper()
		path := filepath.Join(root, name)
		body := []byte("sealed-test-artifact\n" + generation.FrameEmbeddedManifest(raw) + "\n")
		if err := os.WriteFile(path, body, 0o700); err != nil {
			t.Fatal(err)
		}
		return path
	}
	priorArtifact := writeArtifact("prior.exe", priorRaw)
	candidateArtifact := writeArtifact("candidate.exe", candidateRaw)

	worktree := filepath.Join(root, "worktree")
	journal := filepath.Join(worktree, "data", "vivy.db")
	if err := os.MkdirAll(filepath.Dir(journal), 0o700); err != nil {
		t.Fatal(err)
	}
	const journalSentinel = "durable-journal-is-not-generation-state"
	if err := os.WriteFile(journal, []byte(journalSentinel), 0o600); err != nil {
		t.Fatal(err)
	}
	studio, err := studiocore.NewService(ctx, studiocore.Options{
		Worktree: worktree, LedgerPath: filepath.Join(worktree, "data", "studio-home", "studio.db"),
		EvalRoot: filepath.Join(worktree, "data", "studio-home", "evals"), Timeout: 10 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = studio.Close() })

	release := func(manifest assemblyv1.GenerationManifest, artifact, suffix string) domain.Release {
		t.Helper()
		body, readErr := os.ReadFile(artifact)
		if readErr != nil {
			t.Fatal(readErr)
		}
		digest := sha256.Sum256(body)
		candidate := domain.Generation{ID: manifest.GenerationID, ArtifactSHA256: hex.EncodeToString(digest[:]), SourceRef: "file:" + artifact, Phase: domain.GenerationBuilt, CreatedAt: time.Now().UnixMilli()}
		if err := studio.Ledger().CreateGeneration(ctx, candidate); err != nil {
			t.Fatal(err)
		}
		evaluation := domain.EvalRun{ID: "evl_rollback_" + suffix, CandidateID: candidate.ID, Suite: studiocore.SuiteAirgapProbe, Verdict: domain.EvalMixed, CreatedAt: time.Now().UnixMilli()}
		if err := studio.Ledger().CreateEvalRun(ctx, evaluation); err != nil {
			t.Fatal(err)
		}
		released, releaseErr := studio.Release(ctx, candidate.ID, evaluation.ID, studiocore.PromotionActorHuman)
		if releaseErr != nil {
			t.Fatal(releaseErr)
		}
		return released
	}
	priorRelease := release(priorManifest, priorArtifact, "prior")
	candidateRelease := release(candidateManifest, candidateArtifact, "candidate")
	daily := filepath.Join(root, "daily")
	if _, err := studio.Install(ctx, priorRelease.ID, daily); err != nil {
		t.Fatal(err)
	}
	if _, err := studio.Install(ctx, candidateRelease.ID, daily); err != nil {
		t.Fatal(err)
	}
	rolledBack, err := studio.Rollback(ctx, daily)
	if err != nil {
		t.Fatal(err)
	}
	if rolledBack.ReleaseID != priorRelease.ID {
		t.Fatalf("rollback release = %s, want %s", rolledBack.ReleaseID, priorRelease.ID)
	}
	restoredArtifact, err := os.ReadFile(filepath.Join(daily, "vivy.exe"))
	if err != nil {
		t.Fatal(err)
	}
	restoredRaw, err := generation.ExtractEmbeddedManifest(restoredArtifact)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(restoredRaw, priorRaw) {
		t.Fatal("rollback changed the prior sealed Manifest bytes")
	}
	restored, err := assemblyv1.InspectManifest(restoredRaw)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(restored.Catalogs, priorManifest.Catalogs) || restored.Catalogs[0].Digest != catalog.Digest || !reflect.DeepEqual(restored.Catalogs[0].Locales, []string{"en", "zh"}) {
		t.Fatalf("restored catalog identity = %#v, want %#v", restored.Catalogs, priorManifest.Catalogs)
	}
	journalAfter, err := os.ReadFile(journal)
	if err != nil || string(journalAfter) != journalSentinel {
		t.Fatalf("rollback mutated Journal truth: %q, %v", journalAfter, err)
	}
}

func rollbackCatalog() assemblyv1.CatalogManifest {
	catalog := assemblyv1.CatalogManifest{
		Module: "fixture/catalog", APIVersion: "vivy.i18n/v1", SchemaVersion: "vivy.i18n/v1",
		Path: "i18n/catalog.json", DefaultLocale: "en", Locales: []string{"en", "zh"},
		Completeness: map[string]string{"en": "COMPLETE", "zh": "COMPLETE"},
		Units: map[string]assemblyv1.CatalogUnit{
			"plugin.fixture/catalog.title": {Description: "Title", Messages: map[string]string{"en": "Title", "zh": "标题"}},
		},
	}
	payload := struct {
		APIVersion string                            `json:"apiVersion"`
		Units      map[string]assemblyv1.CatalogUnit `json:"units"`
	}{APIVersion: catalog.APIVersion, Units: catalog.Units}
	raw, _ := json.Marshal(payload)
	digest := sha256.Sum256(raw)
	catalog.Digest = hex.EncodeToString(digest[:])
	return catalog
}
