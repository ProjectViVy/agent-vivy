package app

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-vivy/internal/config"
	"agent-vivy/internal/runtime"
)

func TestSkillsCatalogRPCSmoke(t *testing.T) {
	runtime.SetEngineVersionOverride(pinnedEinoVersion)
	t.Cleanup(func() { runtime.SetEngineVersionOverride("") })

	root := filepath.Join(t.TempDir(), "skills")
	dir := filepath.Join(root, "demo-skill")
	if err := os.MkdirAll(filepath.Join(dir, "references"), 0o700); err != nil {
		t.Fatal(err)
	}
	doc := "---\nname: demo-skill\ndescription: A test skill\n---\n\nUse this carefully.\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "references", "guide.md"), []byte("reference content"), 0o600); err != nil {
		t.Fatal(err)
	}

	a, err := New(context.Background(), config.Config{
		Server:    config.Server{Addr: "127.0.0.1:0", AllowedOrigins: []string{"http://127.0.0.1:3015"}},
		Storage:   config.Storage{Backend: "sqlite", SQLite: config.SQLite{Path: filepath.Join(t.TempDir(), "skills.db")}},
		Providers: config.Providers{Active: "openai", BundleDir: filepath.Join("..", "..", "fixtures", "provider"), OpenAI: config.Provider{EnvKey: "OPENAI_API_KEY", DefaultModel: "gpt-4o-mini"}, Anthropic: config.Provider{EnvKey: "ANTHROPIC_API_KEY", DefaultModel: "claude-sonnet-4-5"}},
		Runtime:   config.Runtime{StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10, SkillsRoot: root},
		Tools:     config.Tools{Enabled: []string{"echo_info", "skills_list", "skill_view"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(a.httpServer.Handler)
	t.Cleanup(func() {
		ts.Close()
		a.service.CancelAll()
		_ = a.backend.Close()
	})

	client := connectSmokeRPC(t, ts.URL, a.rpcToken)
	t.Cleanup(func() { _ = client.conn.Close() })
	callSmoke(t, client, "initialize", map[string]any{"protocol_version": "vivy.rpc.v1"})

	listed := callSmoke(t, client, "skills/list", nil)
	var catalog struct {
		Skills []struct {
			Name        string   `json:"name"`
			Description string   `json:"description"`
			Hash        string   `json:"hash"`
			Warnings    []string `json:"warnings"`
		} `json:"skills"`
	}
	decodeSmoke(t, listed, &catalog)
	if len(catalog.Skills) != 1 || catalog.Skills[0].Name != "demo-skill" || catalog.Skills[0].Hash == "" {
		t.Fatalf("skills/list = %+v", catalog)
	}

	viewed := callSmoke(t, client, "skills/get", map[string]any{"name": "demo-skill"})
	var view struct {
		Name            string   `json:"name"`
		Content         string   `json:"content"`
		RelativePath    string   `json:"relative_path"`
		SupportingFiles []string `json:"supporting_files"`
	}
	decodeSmoke(t, viewed, &view)
	if view.Name != "demo-skill" || !strings.Contains(view.Content, "Use this carefully") || view.RelativePath != "SKILL.md" {
		t.Fatalf("skills/get = %+v", view)
	}
	if len(view.SupportingFiles) != 1 || view.SupportingFiles[0] != "references/guide.md" {
		t.Fatalf("supporting files = %v", view.SupportingFiles)
	}

	ref := callSmoke(t, client, "skills/get", map[string]any{"name": "demo-skill", "path": "references/guide.md"})
	var refView struct {
		Content      string `json:"content"`
		RelativePath string `json:"relative_path"`
	}
	decodeSmoke(t, ref, &refView)
	if refView.Content != "reference content" {
		t.Fatalf("supporting get = %+v", refView)
	}
}
