package studio

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"agent-vivy/internal/buildinfo"
	"agent-vivy/internal/domain"
	"agent-vivy/sdk/generation"
)

func testLive() LiveView {
	return LiveView{
		Provider:      "test",
		PolicyProfile: domain.PolicyProfileDefault,
		PolicyHash:    "hash-default",
		Tools: []domain.ToolSpec{
			{Name: "echo_info", Readonly: true},
			{Name: "write_note", Readonly: false},
		},
	}
}

func TestInspectBuiltinWithoutStore(t *testing.T) {
	previousManifest := generation.EmbeddedManifestBase64
	generation.EmbeddedManifestBase64 = ""
	t.Cleanup(func() { generation.EmbeddedManifestBase64 = previousManifest })
	rep, err := (*Service)(nil).Inspect(context.Background(), testLive())
	if err != nil {
		t.Fatal(err)
	}
	if rep.GenerationID != BuiltinGenerationID || rep.BinaryID != buildinfo.Version {
		t.Fatalf("builtin = %+v", rep)
	}
	if len(rep.Tools) != 2 || rep.Tools[0].Name != "echo_info" || !rep.Tools[0].Readonly {
		t.Fatalf("tools = %+v", rep.Tools)
	}
	if rep.Recipe.Loop != "eino" || len(rep.Recipe.Tools) != 2 || len(rep.Grants) != 0 {
		t.Fatalf("recipe/grants = %+v", rep)
	}
	assertNoSecretsOrPaths(t, rep)
}

func TestInspectProjectsSealedUIArtifactHash(t *testing.T) {
	previousManifest := generation.EmbeddedManifestBase64
	generation.EmbeddedManifestBase64 = generation.FrameEmbeddedManifest(testSealedManifest(t, map[string]string{
		"ui/dist":        strings.Repeat("d", 64),
		"ui/assembly.ts": strings.Repeat("s", 64),
	}))
	t.Cleanup(func() { generation.EmbeddedManifestBase64 = previousManifest })

	rep, err := (*Service)(nil).Inspect(context.Background(), testLive())
	if err != nil {
		t.Fatal(err)
	}
	if rep.GenerationID == BuiltinGenerationID || rep.UIArtifactSHA256 != strings.Repeat("d", 64) {
		t.Fatalf("sealed provenance = generation %q, ui artifact %q", rep.GenerationID, rep.UIArtifactSHA256)
	}
}

func TestInspectRejectsUnsealedManifestAndNeverLabelsAssemblySourceAsArtifact(t *testing.T) {
	previousManifest := generation.EmbeddedManifestBase64
	t.Cleanup(func() { generation.EmbeddedManifestBase64 = previousManifest })

	generation.EmbeddedManifestBase64 = generation.FrameEmbeddedManifest([]byte(`{"generationId":"fixture-generation","uiArtifacts":{"ui/dist":"asset-hash"}}`))
	rep, err := (*Service)(nil).Inspect(context.Background(), testLive())
	if err != nil {
		t.Fatal(err)
	}
	if rep.GenerationID != BuiltinGenerationID || rep.UIArtifactSHA256 != "" {
		t.Fatalf("unsealed manifest was projected: %+v", rep)
	}

	generation.EmbeddedManifestBase64 = generation.FrameEmbeddedManifest(testSealedManifest(t, map[string]string{
		"ui/assembly.ts": strings.Repeat("s", 64),
	}))
	rep, err = (*Service)(nil).Inspect(context.Background(), testLive())
	if err != nil {
		t.Fatal(err)
	}
	if rep.UIArtifactSHA256 != "" {
		t.Fatalf("source hash was mislabeled as UI artifact: %q", rep.UIArtifactSHA256)
	}
}

func testSealedManifest(t *testing.T, uiArtifacts map[string]string) []byte {
	t.Helper()
	type manifest struct {
		GenerationID         string              `json:"generationId"`
		SpecificationVersion string              `json:"specificationVersion"`
		CompilerVersion      string              `json:"compilerVersion"`
		SDKVersion           string              `json:"sdkVersion"`
		RecipeDigest         string              `json:"recipeDigest"`
		Modules              []any               `json:"modules"`
		PortEdges            []any               `json:"portEdges"`
		LifecycleOrder       []string            `json:"lifecycleOrder"`
		OrderedContributions map[string][]string `json:"orderedContributions"`
		DependencyLocks      map[string]string   `json:"dependencyLocks"`
		UIArtifacts          map[string]string   `json:"uiArtifacts"`
		UI                   any                 `json:"ui,omitempty"`
		Catalogs             []any               `json:"catalogs"`
		CapabilityStates     map[string]string   `json:"capabilityStates,omitempty"`
	}
	recipe := []byte(`{"apiVersion":"vivy.generation/v1","modules":[]}`)
	recipeDigest := sha256.Sum256(recipe)
	sealed := manifest{
		SpecificationVersion: "vivy.assembly/v1",
		CompilerVersion:      "fixture-compiler",
		SDKVersion:           "fixture-sdk",
		RecipeDigest:         hex.EncodeToString(recipeDigest[:]),
		UIArtifacts:          uiArtifacts,
	}
	identityBytes, err := json.Marshal(sealed)
	if err != nil {
		t.Fatal(err)
	}
	identity := sha256.Sum256(identityBytes)
	sealed.GenerationID = hex.EncodeToString(identity[:])
	raw, err := json.Marshal(sealed)
	if err != nil {
		t.Fatal(err)
	}
	if sealed.GenerationID == "" {
		t.Fatal("sealed manifest has no generation identity")
	}
	return raw
}

func TestInspectAfterPromoteUsesCandidate(t *testing.T) {
	svc, _ := newStudio(t)
	ctx := context.Background()
	from, err := svc.CreateGeneration(ctx, domain.Generation{ArtifactSHA256: "from"})
	if err != nil {
		t.Fatal(err)
	}
	to, err := svc.CreateGeneration(ctx, domain.Generation{
		ParentID: from.ID, ArtifactSHA256: "to-sha",
		Recipe: domain.AssemblyRecipe{Loop: "eino", World: "sandbox", Tools: []string{"echo_info"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RecordEval(ctx, domain.EvalRun{
		CandidateID: to.ID, BaselineID: from.ID, Suite: "s1", Verdict: domain.EvalBetter,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Promote(ctx, from.ID, to.ID, "", domain.PromotionActorHuman); err != nil {
		t.Fatal(err)
	}
	rep, err := svc.Inspect(ctx, testLive())
	if err != nil {
		t.Fatal(err)
	}
	if rep.GenerationID != to.ID || rep.ArtifactSHA256 != "to-sha" {
		t.Fatalf("promoted inspect = %+v, want %s", rep, to.ID)
	}
	if rep.Recipe.Loop != "eino" || len(rep.Recipe.Tools) != 1 {
		t.Fatalf("promoted recipe = %+v", rep.Recipe)
	}
	assertNoSecretsOrPaths(t, rep)
}

func assertNoSecretsOrPaths(t *testing.T, rep Report) {
	t.Helper()
	raw, err := json.Marshal(rep)
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	if strings.Contains(strings.ToLower(body), "api_key") {
		t.Fatalf("inspect leaked api_key: %s", body)
	}
	for i := 0; i+2 < len(body); i++ {
		if ((body[i] >= 'A' && body[i] <= 'Z') || (body[i] >= 'a' && body[i] <= 'z')) && body[i+1] == ':' && (body[i+2] == '\\' || body[i+2] == '/') {
			t.Fatalf("inspect leaked host path: %s", body)
		}
	}
}
