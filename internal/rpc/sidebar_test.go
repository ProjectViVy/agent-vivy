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
	mcp := runtime.NewMCPBackend([]runtime.MCPServerConfig{{Name: "docs", Endpoint: "http://127.0.0.1:1"}}, nil)
	env := newControlTestEnv(t, func(deps *ControlDeps) {
		deps.ProjectRoot = projectRoot
		deps.ModelMeta = func(context.Context, string, string) domain.ModelInfo {
			return domain.ModelInfo{ContextWindow: 8192, SupportsThinking: true}
		}
		deps.Skills = skills
		deps.MCP = mcp
		deps.LanguageServers = func(context.Context, domain.SessionID) (LanguageServerSnapshot, error) {
			return LanguageServerSnapshot{Known: true, Servers: []LanguageServerStatus{{Language: "\x1b]2;hidden-title\a typescript", State: "initialized"}, {Language: "go", State: "starting"}, {Language: "bad", State: "guessed"}, {Language: "C:/secret/gopls.exe", State: "initialized"}}}, nil
		}
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
	if !snapshot.MCPKnown || len(snapshot.MCP) != 1 || snapshot.MCP[0].Name != "docs" || snapshot.MCP[0].State != string(runtime.MCPStateInactive) {
		t.Fatalf("mcp truth = %+v", snapshot.MCP)
	}
	if !snapshot.SkillsKnown || len(snapshot.Skills) != 1 || snapshot.Skills[0].Name != "enabled-skill" || snapshot.Skills[0].Origin != "user" {
		t.Fatalf("enabled skill truth = %+v", snapshot.Skills)
	}
	if !snapshot.LSPKnown || len(snapshot.LSP) != 2 || snapshot.LSP[0].Language != "go" || snapshot.LSP[0].State != "starting" || snapshot.LSP[1].Language != "typescript" {
		t.Fatalf("lsp truth = %+v", snapshot.LSP)
	}
}

type sidebarStatusCatalog struct {
	mcpCatalogStub
	statuses []runtime.MCPServerStatus
}

func (s *sidebarStatusCatalog) ServerStatuses() []runtime.MCPServerStatus {
	return append([]runtime.MCPServerStatus(nil), s.statuses...)
}

func TestSessionSidebarProjectsMCPStatusPriorityAndWireUnknowns(t *testing.T) {
	statusCatalog := &sidebarStatusCatalog{statuses: []runtime.MCPServerStatus{
		{Name: "initialized", Transport: "http", State: runtime.MCPStateReady, Initialized: true, Error: "stale handshake", AuthMissing: true, ToolCount: 0},
		{Name: "failed", Transport: "stdio", State: runtime.MCPStateUnavailable, Error: "connection refused", AuthMissing: true, EnvMissing: []string{"MCP_TOKEN"}, ToolCount: -1},
		{Name: "configured", State: runtime.MCPStateInactive, AuthMissing: true, ToolCount: -1},
	}}
	env := newControlTestEnv(t, func(deps *ControlDeps) {
		deps.MCP = statusCatalog
	})
	created, rpcErr := callControl(t, env.handler, "session/create", map[string]string{"title": "mcp status"})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	session := created.(sessionResult)
	result, rpcErr := callControl(t, env.handler, "session/sidebar", map[string]string{"session_id": string(session.ID)})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	snapshot := result.(sidebarResult)
	if len(snapshot.MCP) != 3 {
		t.Fatalf("mcp status projection = %+v", snapshot.MCP)
	}
	if snapshot.MCP[0].State != string(runtime.MCPStateReady) || snapshot.MCP[0].Transport != "http" || snapshot.MCP[0].Error != "stale handshake" || !snapshot.MCP[0].AuthMissing || snapshot.MCP[0].ToolCount == nil || *snapshot.MCP[0].ToolCount != 0 {
		t.Fatalf("initialized status priority/count = %+v", snapshot.MCP[0])
	}
	if snapshot.MCP[1].State != string(runtime.MCPStateUnavailable) || snapshot.MCP[1].Transport != "stdio" || snapshot.MCP[1].Error != "connection refused" || !snapshot.MCP[1].AuthMissing || len(snapshot.MCP[1].EnvMissing) != 1 || snapshot.MCP[1].EnvMissing[0] != "MCP_TOKEN" || snapshot.MCP[1].ToolCount != nil {
		t.Fatalf("error status/unknown count = %+v", snapshot.MCP[1])
	}
	if snapshot.MCP[2].State != string(runtime.MCPStateInactive) || !snapshot.MCP[2].AuthMissing || snapshot.MCP[2].ToolCount != nil {
		t.Fatalf("configured status/fallback fields = %+v", snapshot.MCP[2])
	}
}

func TestSessionSidebarCatalogOnlyMCPRemainsConfigured(t *testing.T) {
	catalog := &mcpCatalogStub{replaced: []runtime.MCPServerConfig{{Name: "catalog", Endpoint: "http://example.invalid/mcp"}}}
	env := newControlTestEnv(t, func(deps *ControlDeps) {
		deps.MCP = catalog
	})
	created, rpcErr := callControl(t, env.handler, "session/create", map[string]string{"title": "catalog"})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	session := created.(sessionResult)
	result, rpcErr := callControl(t, env.handler, "session/sidebar", map[string]string{"session_id": string(session.ID)})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	snapshot := result.(sidebarResult)
	if len(snapshot.MCP) != 1 || snapshot.MCP[0].State != string(runtime.MCPStateInactive) || snapshot.MCP[0].Error != "" || snapshot.MCP[0].AuthMissing || snapshot.MCP[0].ToolCount != nil {
		t.Fatalf("catalog-only MCP = %+v", snapshot.MCP)
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
	// A mixed session must not expose its partial sum as a usable total.
	if usage.CostUSD != 0 {
		t.Fatalf("cost = %v, want zero placeholder", usage.CostUSD)
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
