package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"agent-vivy/internal/bundlecontract"
	"agent-vivy/internal/domain"
)

func ruleAdmissionFixture(t *testing.T, includeRules bool, ruleFace string) RunAdmission {
	t.Helper()
	const installID = "11111111-1111-4111-8111-111111111111"
	release := bundlecontract.ReleaseRef{InstallID: installID, BundleID: "acme/rules", ReleaseDigest: strings.Repeat("a", 64), ReleaseSequence: 1}
	record := bundlecontract.ComponentRecord{
		Ref:         bundlecontract.ComponentRef{InstallID: installID, Kind: bundlecontract.Rule, LocalID: "workflow"},
		ProjectedID: "b_11111111111141118111111111111111_workflow", Release: release,
		ContentDigest: release.ReleaseDigest, Body: "Use the repository's Go conventions.", Faces: []string{"code"}, Available: true,
	}
	view, err := bundlecontract.NewView(bundlecontract.CatalogIdentity{Epoch: 1, EffectiveDigest: strings.Repeat("b", 64)}, []bundlecontract.ReleaseRef{release}, []bundlecontract.ComponentRecord{record}, nil)
	if err != nil {
		t.Fatal(err)
	}
	bundlePayload := bundlecontract.SnapshotPayload(view, 0)
	bundleBytes, err := bundlecontract.EncodeRunSnapshotPayload(bundlePayload)
	if err != nil {
		t.Fatal(err)
	}
	bundleSum := sha256.Sum256(bundleBytes)
	runID := domain.RunID("run-rule-admission")
	bundleSnapshot := RunBundleSnapshot{
		RunID: runID, SchemaVersion: 1, Payload: bundleBytes,
		PayloadSHA256: hex.EncodeToString(bundleSum[:]), Releases: bundlePayload.Releases,
	}
	var rules *RunRulesSnapshot
	if includeRules {
		blocks := []RunRuleBlock{{Ref: record.Ref, Release: release, ContentDigest: record.ContentDigest, Body: record.Body}}
		rules = &RunRulesSnapshot{Face: ruleFace, Blocks: blocks, Body: RenderRunRules(blocks)}
		rules.Digest, err = RunRulesDigest(*rules)
		if err != nil {
			t.Fatal(err)
		}
	}
	promptPayload, err := json.Marshal(RunPromptPayload{Instruction: "system instruction", Rules: rules})
	if err != nil {
		t.Fatal(err)
	}
	promptSum := sha256.Sum256(promptPayload)
	prompt := &RunPromptSnapshot{
		RunID: runID, SchemaVersion: 2, ComposerVersion: "mask-prompt/2", GenerationID: "generation-test",
		Payload: promptPayload, PayloadSHA256: hex.EncodeToString(promptSum[:]),
	}
	startedPayload, err := json.Marshal(struct {
		Face                string `json:"face"`
		PromptSchema        int    `json:"prompt_schema"`
		PromptDigest        string `json:"prompt_digest"`
		BundleSchema        int    `json:"bundle_schema"`
		BundleDigest        string `json:"bundle_digest"`
		BundleEpoch         int64  `json:"bundle_epoch"`
		BundleCatalogDigest string `json:"bundle_catalog_digest"`
	}{"code", prompt.SchemaVersion, prompt.PayloadSHA256, bundleSnapshot.SchemaVersion, bundleSnapshot.PayloadSHA256, bundlePayload.Catalog.Epoch, bundlePayload.Catalog.EffectiveDigest})
	if err != nil {
		t.Fatal(err)
	}
	return RunAdmission{
		Message: domain.Message{ID: "message-rule-admission", SessionID: "session-rule-admission", RunID: runID, Role: domain.RoleUser},
		Run:     domain.Run{ID: runID, SessionID: "session-rule-admission", Status: domain.RunAccepted, Kind: domain.RunKindPrimary},
		Started: domain.RunEvent{RunID: runID, Type: domain.EventRunStarted, PayloadVersion: 1, Payload: startedPayload},
		Prompt:  prompt, Bundle: &bundleSnapshot, ExpectedBundle: ptr(bundlecontract.CaptureCheckFor(bundlePayload)),
	}
}

func TestValidateRunAdmissionBindsRulesToCapturedCatalogAndFace(t *testing.T) {
	in := ruleAdmissionFixture(t, true, "code")
	if err := ValidateRunAdmissionInput(in); err != nil {
		t.Fatalf("valid captured Rules admission rejected: %v", err)
	}

	in = ruleAdmissionFixture(t, false, "")
	if err := ValidateRunAdmissionInput(in); err == nil {
		t.Fatal("applicable captured Rule omitted from prompt snapshot was accepted")
	}

	in = ruleAdmissionFixture(t, true, "web")
	if err := ValidateRunAdmissionInput(in); err == nil {
		t.Fatal("Rule outside the run face scope was accepted")
	}
}

func ptr[T any](value T) *T { return &value }
