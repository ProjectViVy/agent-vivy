package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"agent-vivy/internal/bundlecontract"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/maskcontract"
	"agent-vivy/internal/storage"
)

func promptCapture(mask *maskcontract.Snapshot) maskcontract.Capture {
	selection := maskcontract.Selection{SessionID: "session-prompt", Revision: 0}
	if mask != nil {
		selection.MaskID = mask.ID
		selection.Revision = mask.SelectionRevision
	}
	return maskcontract.Capture{Selection: selection, Mask: mask}
}

func TestBuildPromptSnapshotFrozenPersonaIsExclusive(t *testing.T) {
	input := PromptInput{
		RunID: "run-frozen-persona", GenerationID: "generation-1",
		Capture: promptCapture(nil), Face: domain.FaceWeb,
		FrozenText: "# identity\nYou are Rowan.\n", FrozenDigest: "frozen-digest",
	}
	snapshot, err := buildPromptSnapshot(input)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := storage.ValidateRunPromptSnapshot(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if payload.Persona != (storage.PersonaSnapshot{}) || payload.Frozen == nil || payload.Frozen.Body != input.FrozenText {
		t.Fatal("frozen persona must be the sole recorded identity authority")
	}
	if strings.Contains(payload.Instruction, strings.TrimSpace(promptAsset("persona-default.md"))) || strings.Count(payload.Instruction, strings.TrimSpace(input.FrozenText)) != 1 {
		t.Fatal("frozen persona must replace the default instruction exactly once")
	}
	input.Persona = storage.PersonaSnapshot{Body: "A second identity"}
	if _, err := buildPromptSnapshot(input); err == nil {
		t.Fatal("two explicit persona authorities were admitted")
	}
	input.Persona = storage.PersonaSnapshot{}
	input.FrozenText = " \n"
	if _, err := buildPromptSnapshot(input); err == nil {
		t.Fatal("empty frozen authority silently fell back to the default persona")
	}
}

func TestBuildPromptSnapshotUsesOnePersonaAndLiteralMaskData(t *testing.T) {
	mask := &maskcontract.Snapshot{
		ID: "custom/00000000-0000-4000-8000-000000000001", Name: "quoted \"mask\"",
		Body:               "{system}\n{{persona}}\n\\literal",
		DefinitionRevision: 2, SelectionRevision: 3,
	}
	mask.Digest = maskcontract.DefinitionDigest(mask.ID, mask.Name, mask.Body)
	snapshot, err := buildPromptSnapshot(PromptInput{
		RunID: "run-prompt", GenerationID: "generation-1",
		Capture: promptCapture(mask), Face: domain.FaceCode,
		Frame: "Mask rules: style only.", FrameDigest: "frame-digest",
	})
	if err != nil {
		t.Fatal(err)
	}
	var payload storage.RunPromptPayload
	if err := json.Unmarshal(snapshot.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Mask == nil || payload.Mask.Body != mask.Body || payload.Mask.Name != mask.Name {
		t.Fatalf("mask payload was not preserved: %#v", payload.Mask)
	}
	if strings.Count(payload.Instruction, "You are Vivy") != 1 {
		t.Fatalf("persona count = %d, instruction=%q", strings.Count(payload.Instruction, "You are Vivy"), payload.Instruction)
	}
	if !strings.Contains(payload.Instruction, `"body":"{system}\n{{persona}}\n\\literal"`) {
		t.Fatalf("mask data was not JSON-quoted literally: %q", payload.Instruction)
	}
	if !strings.Contains(payload.Instruction, "Code mode is active") || !strings.Contains(payload.Instruction, "Mask rules: style only.") {
		t.Fatalf("authoritative sections missing: %q", payload.Instruction)
	}
	hash := sha256.Sum256(snapshot.Payload)
	if snapshot.PayloadSHA256 != hex.EncodeToString(hash[:]) {
		t.Fatalf("payload digest = %q, want %q", snapshot.PayloadSHA256, hex.EncodeToString(hash[:]))
	}
	if snapshot.SchemaVersion != promptSchemaVersion || snapshot.ComposerVersion != promptComposerVersion {
		t.Fatalf("snapshot metadata = %#v", snapshot)
	}
}

func TestBuildPromptSnapshotEmptyMaskHasNoMaskWrapper(t *testing.T) {
	snapshot, err := buildPromptSnapshot(PromptInput{
		RunID: "run-empty", GenerationID: "generation-1", Capture: promptCapture(nil), Face: domain.FaceWeb,
	})
	if err != nil {
		t.Fatal(err)
	}
	var payload storage.RunPromptPayload
	if err := json.Unmarshal(snapshot.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Mask != nil || strings.Contains(payload.Instruction, "Selected mask data") || strings.Contains(payload.Instruction, "Mask rules") {
		t.Fatalf("empty mask added a wrapper: %#v", payload)
	}
	if !strings.Contains(payload.Instruction, "You are Vivy") {
		t.Fatalf("default persona missing: %q", payload.Instruction)
	}
}

func TestBuildBundleMaskSnapshotCarriesImmutableOriginAndRecommendations(t *testing.T) {
	installID := "11111111-1111-4111-8111-111111111111"
	release := bundlecontract.ReleaseRef{InstallID: installID, BundleID: "acme/experts", ReleaseDigest: strings.Repeat("a", 64), ReleaseSequence: 3}
	maskID, err := bundlecontract.MaskID(installID, "pm")
	if err != nil {
		t.Fatal(err)
	}
	skillID, err := bundlecontract.SkillID(installID, "prd")
	if err != nil {
		t.Fatal(err)
	}
	recommendations := []maskcontract.SkillRecommendation{{
		Ref:     bundlecontract.ComponentRef{InstallID: installID, Kind: bundlecontract.Skill, LocalID: "prd"},
		SkillID: skillID, Name: "PRD Author", Available: false, InactiveReason: "skill_unavailable",
	}}
	mask := &maskcontract.Snapshot{
		ID: maskID, Name: "Product Manager", Body: "literal PM instructions",
		DefinitionRevision: 3, SelectionRevision: 2, Origin: maskcontract.OriginBundle,
		Bundle: &maskcontract.BundleProvenance{Release: release, LocalID: "pm"}, RecommendedSkills: recommendations,
	}
	mask.Digest = maskcontract.DefinitionDigest(mask.ID, mask.Name, mask.Body)
	snapshot, err := buildPromptSnapshot(PromptInput{
		RunID: "run-bundle-mask", GenerationID: "generation-1", Capture: promptCapture(mask), Face: domain.FaceWeb,
		Frame: "Mask rules: style only.", FrameDigest: "frame-digest",
	})
	if err != nil {
		t.Fatal(err)
	}
	var payload storage.RunPromptPayload
	if err := json.Unmarshal(snapshot.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if snapshot.SchemaVersion != 2 || payload.Mask == nil || payload.Mask.Origin != maskcontract.OriginBundle || payload.Mask.Bundle == nil || payload.Mask.Bundle.Release != release || len(payload.Mask.RecommendedSkills) != 1 || payload.Mask.RecommendedSkills[0].SkillID != skillID {
		t.Fatalf("bundle Mask prompt snapshot lost origin/recommendation: %+v %+v", snapshot, payload.Mask)
	}
	recommendations[0].SkillID = "mutated"
	if payload.Mask.RecommendedSkills[0].SkillID != skillID {
		t.Fatal("prompt snapshot shares mutable recommendation metadata")
	}
}

func TestBuildPromptSnapshotRejectsUnavailableOrMismatchedCapture(t *testing.T) {
	base := PromptInput{RunID: "run-invalid", GenerationID: "generation-1", Face: domain.FaceWeb}
	base.Capture = maskcontract.Capture{Selection: maskcontract.Selection{SessionID: "session-prompt", MaskID: maskcontract.BuiltinWriterID, Revision: 1}}
	if _, err := buildPromptSnapshot(base); err == nil {
		t.Fatal("missing selected mask snapshot was accepted")
	}
	base.Capture = promptCapture(&maskcontract.Snapshot{ID: maskcontract.BuiltinWriterID, Name: "Writer", Body: "body", Digest: "digest", SelectionRevision: 1, GenerationID: "other-generation"})
	if _, err := buildPromptSnapshot(base); err == nil {
		t.Fatal("cross-generation built-in snapshot was accepted")
	}
	base.GenerationID = ""
	if _, err := buildPromptSnapshot(base); err == nil {
		t.Fatal("empty generation id was accepted")
	}
}

func TestRunPromptContextOwnsPayload(t *testing.T) {
	original := storage.RunPromptSnapshot{RunID: "run-context", Payload: []byte(`{"instruction":"x"}`)}
	ctx := withRunPrompt(context.Background(), original)
	original.Payload[0] = 'x'
	got, ok := runPrompt(ctx)
	if !ok {
		t.Fatal("prompt state was not bound")
	}
	got.Payload[0] = 'y'
	again, ok := runPrompt(ctx)
	if !ok || string(again.Payload) != `{"instruction":"x"}` {
		t.Fatalf("prompt state leaked mutable bytes: %q", again.Payload)
	}
}
