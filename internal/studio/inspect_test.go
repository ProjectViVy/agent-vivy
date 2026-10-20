package studio

import (
	"context"
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

func TestInspectProjectsEmbeddedGenerationIdentity(t *testing.T) {
	previous := generation.EmbeddedManifestBase64
	generation.EmbeddedManifestBase64 = generation.FrameEmbeddedManifest([]byte(`{"generationId":"fixture-opaque-generation","specificationVersion":"vivy.assembly/v1","compilerVersion":"fixture","sdkVersion":"fixture","modules":[],"portEdges":[],"lifecycleOrder":[],"orderedContributions":{},"catalogs":[]}`))
	t.Cleanup(func() { generation.EmbeddedManifestBase64 = previous })
	rep, err := (*Service)(nil).Inspect(context.Background(), testLive())
	if err != nil {
		t.Fatal(err)
	}
	if rep.GenerationID != "fixture-opaque-generation" {
		t.Fatalf("embedded identity = %q", rep.GenerationID)
	}
	raw, err := json.Marshal(rep)
	if err != nil {
		t.Fatal(err)
	}
	for _, retired := range []string{"artifact_sha256", "ui_artifact_sha256", "policy_hash"} {
		if strings.Contains(string(raw), retired) {
			t.Fatalf("retired inspect field %s", retired)
		}
	}
}

func TestInspectRejectsMalformedEmbeddedManifest(t *testing.T) {
	previous := generation.EmbeddedManifestBase64
	t.Cleanup(func() { generation.EmbeddedManifestBase64 = previous })
	for _, framed := range []string{"invalid framing", generation.FrameEmbeddedManifest([]byte(`{"generationId":"fixture","unknown":true}`)), generation.FrameEmbeddedManifest([]byte(`{"generationId":`))} {
		generation.EmbeddedManifestBase64 = framed
		rep, err := (*Service)(nil).Inspect(context.Background(), testLive())
		if err != nil {
			t.Fatal(err)
		}
		if rep.GenerationID != BuiltinGenerationID {
			t.Fatalf("malformed manifest projected: %+v", rep)
		}
	}
}

func TestInspectAfterPromoteUsesCandidate(t *testing.T) {
	svc, _ := newStudio(t)
	ctx := context.Background()
	from, err := svc.CreateGeneration(ctx, domain.Generation{ArtifactSHA256: "retained-core-from"})
	if err != nil {
		t.Fatal(err)
	}
	to, err := svc.CreateGeneration(ctx, domain.Generation{
		ParentID: from.ID, ArtifactSHA256: "retained-core-to",
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
	if rep.GenerationID != to.ID {
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
