package runtime

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/cloudwego/eino/adk/middlewares/agentsmd"

	"agent-vivy/internal/bundlecontract"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/tools"
	"github.com/cloudwego/eino/schema"
)

type bundleRuleTestAgentsMDBackend func(context.Context, *agentsmd.ReadRequest) (*agentsmd.FileContent, error)

func (b bundleRuleTestAgentsMDBackend) Read(ctx context.Context, req *agentsmd.ReadRequest) (*agentsmd.FileContent, error) {
	return b(ctx, req)
}

func toolkitRuleView(t *testing.T, rules ...bundlecontract.ComponentRecord) bundlecontract.View {
	t.Helper()
	installID := "11111111-1111-4111-8111-111111111111"
	release := bundlecontract.ReleaseRef{InstallID: installID, BundleID: "acme/rules", ReleaseDigest: strings.Repeat("a", 64), ReleaseSequence: 1}
	for i := range rules {
		rules[i].Ref = bundlecontract.ComponentRef{InstallID: installID, Kind: bundlecontract.Rule, LocalID: rules[i].Name}
		rules[i].ProjectedID, _ = bundlecontract.SkillID(installID, rules[i].Name)
		rules[i].Release = release
		rules[i].ContentDigest = release.ReleaseDigest
		rules[i].Available = true
	}
	view, err := bundlecontract.NewView(bundlecontract.CatalogIdentity{Epoch: 4, EffectiveDigest: strings.Repeat("b", 64)}, []bundlecontract.ReleaseRef{release}, rules, nil)
	if err != nil {
		t.Fatal(err)
	}
	return view
}

func TestToolkitRuleSnapshotFiltersFaceSortsAndFreezesContent(t *testing.T) {
	view := toolkitRuleView(t,
		bundlecontract.ComponentRecord{Name: "zeta", Body: "z-code", Faces: []string{"code"}},
		bundlecontract.ComponentRecord{Name: "alpha", Body: "alpha-all"},
		bundlecontract.ComponentRecord{Name: "web", Body: "web-only", Faces: []string{"web"}},
	)
	snapshot, err := PrepareBundleRules(view, domain.FaceCode)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot == nil || len(snapshot.Blocks) != 2 {
		t.Fatalf("code rules = %#v, want alpha and zeta", snapshot)
	}
	if snapshot.Blocks[0].Ref.LocalID != "alpha" || snapshot.Blocks[1].Ref.LocalID != "zeta" {
		t.Fatalf("rule order = %#v", snapshot.Blocks)
	}
	if !strings.Contains(snapshot.Body, "alpha-all") || !strings.Contains(snapshot.Body, "z-code") || strings.Contains(snapshot.Body, "web-only") {
		t.Fatalf("face-filtered body = %q", snapshot.Body)
	}
	if snapshot.Digest == "" || snapshot.Blocks[0].Body != "alpha-all" {
		t.Fatalf("snapshot did not retain exact block bytes and digest: %#v", snapshot)
	}

	// A later catalog revision must not alter already captured prompt evidence.
	updated := toolkitRuleView(t, bundlecontract.ComponentRecord{Name: "alpha", Body: "edited-source"})
	if _, err := PrepareBundleRules(updated, domain.FaceCode); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(snapshot.Body, "alpha-all") || strings.Contains(snapshot.Body, "edited-source") {
		t.Fatalf("captured rule changed after source edit: %q", snapshot.Body)
	}
}

func TestToolkitRuleBounds(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "recursive import", body: "local policy\n@../workspace/AGENTS.md"},
		{name: "single component cap", body: strings.Repeat("x", 16<<10+1)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			view := toolkitRuleView(t, bundlecontract.ComponentRecord{Name: "bad", Body: tc.body})
			if _, err := PrepareBundleRules(view, domain.FaceCode); err == nil {
				t.Fatalf("invalid rule accepted: %s", tc.name)
			}
		})
	}

	rules := make([]bundlecontract.ComponentRecord, 5)
	for i := range rules {
		rules[i] = bundlecontract.ComponentRecord{Name: string(rune('a' + i)), Body: strings.Repeat("x", 14<<10)}
	}
	if _, err := PrepareBundleRules(toolkitRuleView(t, rules...), domain.FaceCode); err == nil {
		t.Fatal("aggregate Rules size above 64 KiB was accepted")
	}
}

func TestToolkitRuleSnapshotIsValidatedInsidePromptSnapshot(t *testing.T) {
	rule, err := PrepareBundleRules(toolkitRuleView(t, bundlecontract.ComponentRecord{Name: "workflow", Body: "Use the repository's Go conventions."}), domain.FaceCode)
	if err != nil {
		t.Fatal(err)
	}
	prompt, err := buildPromptSnapshot(PromptInput{
		RunID: "run-rules", GenerationID: "generation-1", Face: domain.FaceCode,
		Capture: promptCapture(nil), Rules: rule,
	})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := storage.ValidateRunPromptSnapshot(prompt)
	if err != nil {
		t.Fatal(err)
	}
	if payload.Rules == nil || payload.Rules.Digest != rule.Digest || payload.Rules.Blocks[0].Body != rule.Blocks[0].Body {
		t.Fatalf("prompt snapshot lost Rules evidence: %#v", payload.Rules)
	}

	payload.Rules.Body += "tampered"
	if err := storage.ValidateRunRulesSnapshot(payload.Rules); err == nil {
		t.Fatal("modified captured Rules body passed digest validation")
	}
}

func TestToolkitRulePromptBudgetRejectsBeforeAdmission(t *testing.T) {
	rule, err := PrepareBundleRules(toolkitRuleView(t, bundlecontract.ComponentRecord{Name: "workflow", Body: "Use the repository's Go conventions."}), domain.FaceCode)
	if err != nil {
		t.Fatal(err)
	}
	_, err = buildPromptSnapshot(PromptInput{
		RunID: "run-rules-budget", GenerationID: "generation-1", Face: domain.FaceCode,
		Capture: promptCapture(nil), Rules: rule, MaxContextBytes: 32,
	})
	if !errors.Is(err, ErrContextBudgetExceeded) {
		t.Fatalf("prompt budget error = %v, want %v", err, ErrContextBudgetExceeded)
	}
}

func TestToolkitAgentsMDBackendUsesCapturedRulesAndDelegatesProjectFiles(t *testing.T) {
	base := bundleRuleTestAgentsMDBackend(func(_ context.Context, req *agentsmd.ReadRequest) (*agentsmd.FileContent, error) {
		if req.FilePath == "AGENTS.md" {
			return &agentsmd.FileContent{Content: "project instructions"}, nil
		}
		return nil, os.ErrNotExist
	})
	view := toolkitRuleView(t, bundlecontract.ComponentRecord{Name: "workflow", Body: "Use the repository's Go conventions."})
	rules, err := PrepareBundleRules(view, domain.FaceCode)
	if err != nil {
		t.Fatal(err)
	}
	prompt, err := buildPromptSnapshot(PromptInput{
		RunID: "run-rules-adapter", GenerationID: "generation-1", Face: domain.FaceCode,
		Capture: promptCapture(nil), Rules: rules,
	})
	if err != nil {
		t.Fatal(err)
	}
	adapter := NewToolkitAgentsMDBackend(base)
	ctx := withRunPrompt(context.Background(), prompt)
	ruleContent, err := adapter.Read(ctx, &agentsmd.ReadRequest{FilePath: ToolkitAgentsMDFileName, Offset: 1})
	if err != nil || ruleContent == nil || ruleContent.Content != rules.Body {
		t.Fatalf("captured Rule read = %#v, %v", ruleContent, err)
	}
	projectContent, err := adapter.Read(ctx, &agentsmd.ReadRequest{FilePath: "AGENTS.md", Offset: 1})
	if err != nil || projectContent == nil || projectContent.Content != "project instructions" {
		t.Fatalf("project instruction read = %#v, %v", projectContent, err)
	}
	if _, err := adapter.Read(context.Background(), &agentsmd.ReadRequest{FilePath: ToolkitAgentsMDFileName, Offset: 1}); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Rule read without an admitted snapshot = %v, want not-exist", err)
	}
}

func TestEngineLoadsCapturedToolkitRulesBeforeProjectAgentsMD(t *testing.T) {
	ctx := context.Background()
	project := bundleRuleTestAgentsMDBackend(func(_ context.Context, req *agentsmd.ReadRequest) (*agentsmd.FileContent, error) {
		if req.FilePath == "AGENTS.md" {
			return &agentsmd.FileContent{Content: "PROJECT-AGENT-RULE"}, nil
		}
		return nil, os.ErrNotExist
	})
	rule, err := PrepareBundleRules(toolkitRuleView(t, bundlecontract.ComponentRecord{Name: "workflow", Body: "BUNDLE-TOOLKIT-RULE"}), domain.FaceCode)
	if err != nil {
		t.Fatal(err)
	}
	prompt, err := buildPromptSnapshot(PromptInput{
		RunID: "run-rules-order", GenerationID: "generation-1", Face: domain.FaceCode,
		Capture: promptCapture(nil), Rules: rule,
	})
	if err != nil {
		t.Fatal(err)
	}
	toolSet, err := tools.Builtin(nil).Resolve([]string{tools.EchoInfoName})
	if err != nil {
		t.Fatal(err)
	}
	recorder := &recordingModel{inner: NewScriptedModel(schema.AssistantMessage("done", nil))}
	engine, err := NewEngine(ctx, recorder, toolSet, EngineConfig{
		StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10,
		AgentsMDBackend: NewToolkitAgentsMDBackend(project), AgentsMDFiles: []string{"AGENTS.md"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := drainFinalText(t, engine.RunHistory(withRunPrompt(withRunID(ctx, "run-rules-order"), prompt), []*schema.Message{schema.UserMessage("start")})); got != "done" {
		t.Fatalf("final answer = %q", got)
	}
	inputs := recorder.snapshot()
	if len(inputs) != 1 {
		t.Fatalf("model calls = %d, want 1", len(inputs))
	}
	joined := ""
	for _, message := range inputs[0] {
		if message != nil {
			joined += message.Content
		}
	}
	rulePosition := strings.Index(joined, "BUNDLE-TOOLKIT-RULE")
	projectPosition := strings.Index(joined, "PROJECT-AGENT-RULE")
	if rulePosition < 0 || projectPosition < 0 || rulePosition >= projectPosition {
		t.Fatalf("agent context order does not put captured Toolkit Rules before project AGENTS.md: %q", joined)
	}
}
