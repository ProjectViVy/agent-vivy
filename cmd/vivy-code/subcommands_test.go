package main

import (
	"bytes"
	"strings"
	"testing"

	"agent-vivy/internal/app/settings"
	"agent-vivy/internal/config"
)

func testConfig(t *testing.T) config.Config {
	t.Helper()
	cfg := config.Default()
	cfg.Storage.DataDir = t.TempDir()
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	return cfg
}

func run(t *testing.T, cfg config.Config, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	var out, errw bytes.Buffer
	handled, code := runSubcommand(args, cfg, &out, &errw)
	if !handled {
		t.Fatalf("args %v not handled", args)
	}
	return out.String(), errw.String(), code
}

func settingsFile(t *testing.T, cfg config.Config) settings.Settings {
	t.Helper()
	doc, err := settings.Load(settings.Path(cfg.DataDirectory()))
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func TestMCPAddRemoveRoundTrip(t *testing.T) {
	cfg := testConfig(t)

	out, _, code := run(t, cfg, "mcp", "add", "docs", "--endpoint", "https://docs.example.com/mcp")
	if code != 0 || !strings.Contains(out, "added docs") {
		t.Fatalf("add endpoint: code=%d out=%q", code, out)
	}
	out, _, code = run(t, cfg, "mcp", "add", "local-tool", "--command", "npx", "-y", "some-mcp")
	if code != 0 {
		t.Fatalf("add command: code=%d err out=%q", code, out)
	}

	doc := settingsFile(t, cfg)
	servers := doc.MCPServersOrEmpty()
	if len(servers) != 2 {
		t.Fatalf("servers = %+v", servers)
	}
	var lt *settings.MCPServer
	for i := range servers {
		if servers[i].Name == "local-tool" {
			lt = &servers[i]
		}
	}
	if lt == nil || lt.Command != "npx" || len(lt.Args) != 2 || lt.Args[0] != "-y" || lt.Args[1] != "some-mcp" {
		t.Fatalf("local-tool entry = %+v", lt)
	}

	out, _, code = run(t, cfg, "mcp", "list")
	if code != 0 || !strings.Contains(out, "docs") || !strings.Contains(out, "local-tool") {
		t.Fatalf("list: code=%d out=%q", code, out)
	}

	// remove
	if _, _, code = run(t, cfg, "mcp", "remove", "docs"); code != 0 {
		t.Fatalf("remove: %d", code)
	}
	if servers := settingsFile(t, cfg).MCPServersOrEmpty(); len(servers) != 1 {
		t.Fatalf("after remove: %+v", servers)
	}
	// removing again is a clean failure
	if _, _, code = run(t, cfg, "mcp", "remove", "docs"); code != 1 {
		t.Fatalf("remove missing: %d", code)
	}
	// status runs (unreachable is a fine answer, not a crash)
	if _, _, code = run(t, cfg, "mcp", "status"); code != 0 {
		t.Fatalf("status: %d", code)
	}
}

func TestMCPAddRejectsInvalidEndpoint(t *testing.T) {
	cfg := testConfig(t)
	_, stderr, code := run(t, cfg, "mcp", "add", "bad", "--endpoint", "https://example.com/mcp?api_key=secret")
	if code != 1 || stderr == "" {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	if got := settingsFile(t, cfg).MCPServersOrEmpty(); len(got) != 0 {
		t.Fatalf("rejected server persisted: %+v", got)
	}
}

func TestMCPLoginDeferred(t *testing.T) {
	cfg := testConfig(t)
	out, _, code := run(t, cfg, "mcp", "login", "x")
	if code != 0 || !strings.Contains(out, "deferred") {
		t.Fatalf("login: code=%d out=%q", code, out)
	}
}

func TestConfigGetSetRoundTrip(t *testing.T) {
	cfg := testConfig(t)
	if _, _, code := run(t, cfg, "config", "set", "provider", "deepseek"); code != 0 {
		t.Fatalf("set provider: %d", code)
	}
	if _, _, code := run(t, cfg, "config", "set", "default_model", "deepseek-chat"); code != 0 {
		t.Fatalf("set: %d", code)
	}
	out, _, code := run(t, cfg, "config", "get", "default_model")
	if code != 0 || !strings.Contains(out, "deepseek-chat") {
		t.Fatalf("get: code=%d out=%q", code, out)
	}
	doc := settingsFile(t, cfg)
	if doc.Provider != "deepseek" || doc.DefaultModel != "deepseek-chat" {
		t.Fatalf("settings doc: %+v", doc)
	}
}

func TestConfigSetRejectsUnknownAndSecretPaths(t *testing.T) {
	cfg := testConfig(t)
	if _, _, code := run(t, cfg, "config", "set", "nope.nothing", "x"); code != 1 {
		t.Fatalf("unknown path: %d", code)
	}
	_, stderr, code := run(t, cfg, "config", "set", "api_key", "sk-secret")
	if code != 2 || !strings.Contains(stderr, "secret") {
		t.Fatalf("secret path: code=%d stderr=%q", code, stderr)
	}
	// env-var references are references, not secrets — allowed
	if _, _, code := run(t, cfg, "config", "get", "network_search.provider"); code != 0 {
		t.Fatalf("nested get: %d", code)
	}
}
