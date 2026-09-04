package rpc

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/storage"
)

func TestSessionSidebarUsesAuthoritativeOwners(t *testing.T) {
	projectRoot := filepath.Join(t.TempDir(), "code-project")
	skillsRoot := filepath.Join(t.TempDir(), "skills")
	for name, enabled := range map[string]bool{"enabled-skill": true, "disabled-skill": false} {
		dir := filepath.Join(skillsRoot, name)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		doc := "---\nname: " + name + "\ndescription: sidebar fixture\nenabled: "
		if enabled {
			doc += "true"
		} else {
			doc += "false"
		}
		doc += "\n---\n\nFixture.\n"
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(doc), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	skills, err := runtime.NewEinoSkillBackend(skillsRoot, nil)
	if err != nil {
		t.Fatal(err)
	}
	mcp := runtime.NewEinoMCPBackend([]runtime.MCPServerConfig{{Name: "docs", Endpoint: "http://127.0.0.1:1"}}, nil)
	env := newControlTestEnv(t, func(deps *ControlDeps) {
		deps.ProjectRoot = projectRoot
		deps.ModelMeta = func(context.Context, string, string) domain.ModelInfo {
			return domain.ModelInfo{ContextWindow: 8192, SupportsThinking: true}
		}
		deps.Skills = skills
		deps.MCP = mcp
	})
	created, rpcErr := callControl(t, env.handler, "session/create", map[string]string{"title": "truth"})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	session := created.(sessionResult)
	if err := env.backend.RecordFileMutation(context.Background(), session.ID, "run-sidebar", "main.go", []byte("old\n"), []byte("new\n")); err != nil {
		t.Fatal(err)
	}
	result, rpcErr := callControl(t, env.handler, "session/sidebar", map[string]string{"session_id": string(session.ID)})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	snapshot, ok := result.(sidebarResult)
	if !ok {
		t.Fatalf("result type = %T, want sidebarResult", result)
	}
	if snapshot.SessionID != session.ID || snapshot.Session.UpdatedAt == 0 {
		t.Fatalf("session truth = %+v", snapshot)
	}
	if snapshot.CWD != projectRoot {
		t.Fatalf("cwd = %q, want ControlDeps.ProjectRoot %q", snapshot.CWD, projectRoot)
	}
	if snapshot.Provider != "test" || snapshot.Model != "test-model" {
		t.Fatalf("route = %q/%q, want runtime route", snapshot.Provider, snapshot.Model)
	}
	if snapshot.ReasoningKnown {
		t.Fatalf("test runtime has no catalog, reasoning should stay unknown: known:%v supported:%v", snapshot.ReasoningKnown, snapshot.ReasoningSupported)
	}
	if len(snapshot.ModifiedFiles) != 1 || snapshot.ModifiedFiles[0].Path != "main.go" {
		t.Fatalf("modified files = %+v", snapshot.ModifiedFiles)
	}
	if !snapshot.MCPKnown || len(snapshot.MCP) != 1 || snapshot.MCP[0].Name != "docs" || snapshot.MCP[0].State != "configured" {
		t.Fatalf("mcp truth = %+v", snapshot.MCP)
	}
	if !snapshot.SkillsKnown || len(snapshot.Skills) != 1 || snapshot.Skills[0].Name != "enabled-skill" {
		t.Fatalf("enabled skill truth = %+v", snapshot.Skills)
	}
}

func TestBuildSidebarUsageKeepsUnknownAndPartialCostDistinct(t *testing.T) {
	rows := []storage.UsageRow{
		{SessionID: "sess", PromptTokens: 1000, CompletionTokens: 500, TotalTokens: 1500, ReasoningTokens: 20, CachedTokens: 10, Model: "known", Provider: "provider"},
		{SessionID: "sess", PromptTokens: 100, CompletionTokens: 100, TotalTokens: 200, Model: "unknown", Provider: "provider"},
		{SessionID: "other", PromptTokens: 999, CompletionTokens: 999, TotalTokens: 1998, Model: "known", Provider: "provider"},
	}
	usage := buildSidebarUsage(context.Background(), rows, "sess", func(_ context.Context, _ string, model string) domain.ModelInfo {
		if model == "unknown" {
			return domain.ModelInfo{}
		}
		return domain.ModelInfo{InputPerMTokens: 2, OutputPerMTokens: 4}
	})
	if usage.RequestCount != 2 || usage.TotalTokens != 1700 || usage.CostKnown {
		t.Fatalf("usage = %+v", usage)
	}
	// The priced row costs 0.002 + 0.002 = 0.004 USD, but the mixed session
	// remains unknown rather than presenting that partial sum as the total.
	if usage.CostUSD != 0.004 {
		t.Fatalf("cost = %v, want 0.004", usage.CostUSD)
	}
	unknown := buildSidebarUsage(context.Background(), rows[1:2], "sess", func(context.Context, string, string) domain.ModelInfo {
		return domain.ModelInfo{}
	})
	if unknown.CostKnown || unknown.CostUSD != 0 {
		t.Fatalf("unknown cost = %+v, want known=false and zero placeholder", unknown)
	}
	partialRate := buildSidebarUsage(context.Background(), rows[:1], "sess", func(context.Context, string, string) domain.ModelInfo {
		return domain.ModelInfo{InputPerMTokens: 2, OutputPerMTokens: 0}
	})
	if partialRate.CostKnown {
		t.Fatalf("partial reference price was treated as complete: %+v", partialRate)
	}
	aggregated := buildSidebarUsage(context.Background(), []storage.UsageRow{{
		SessionID: "sess", RequestCount: 3, PromptTokens: 300, CompletionTokens: 150, TotalTokens: 450, Model: "known", Provider: "provider",
	}}, "sess", func(context.Context, string, string) domain.ModelInfo {
		return domain.ModelInfo{InputPerMTokens: 2, OutputPerMTokens: 4}
	})
	if aggregated.RequestCount != 3 || !aggregated.CostKnown {
		t.Fatalf("route aggregate lost request cardinality: %+v", aggregated)
	}
}
