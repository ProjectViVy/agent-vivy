package eval

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-vivy/internal/config"
)

func TestPrepareRejectsProductionSQLitePath(t *testing.T) {
	root := t.TempDir()
	_, err := Prepare(root, Isolation{
		ProductionSQLite: filepath.Join(root, "data", "vivy.db"),
		BundleDir:        fixtureBundle(t),
	})
	if err != ErrBlockedPath {
		t.Fatalf("err = %v, want ErrBlockedPath", err)
	}
}

func TestPrepareOmitsProductionPathsAndSecrets(t *testing.T) {
	root := t.TempDir()
	production := filepath.Join(t.TempDir(), "prod", "vivy.db")
	if err := os.MkdirAll(filepath.Dir(production), 0o700); err != nil {
		t.Fatal(err)
	}
	layout, err := Prepare(root, Isolation{
		ProductionSQLite:    production,
		ProductionWorkspace: filepath.Join(filepath.Dir(production), "workspaces"),
		ProductionListen:    "127.0.0.1:8787",
		BundleDir:           fixtureBundle(t),
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(layout.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	if containsPath(body, production) {
		t.Fatalf("config leaked production sqlite: %s", body)
	}
	if strings.Contains(strings.ToLower(body), "api_key:") {
		t.Fatalf("config leaked api_key field: %s", body)
	}
	if layout.Addr == "127.0.0.1:8787" {
		t.Fatal("candidate reused production listen")
	}
	if _, err := config.Load(layout.ConfigPath); err != nil {
		t.Fatal(err)
	}
	if samePath(layout.SQLitePath, production) {
		t.Fatal("candidate sqlite is production sqlite")
	}
}

func TestChildEnvStripsProviderSecrets(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-test-openai")
	t.Setenv("ANTHROPIC_API_KEY", "sk-test-anthropic")
	t.Setenv("VIVY_ADDR", "127.0.0.1:1")
	t.Setenv("VIVY_POSTGRES_DSN", "postgres://vivy:secret@postgres:5432/vivy")
	env := ChildEnv(filepath.Join(t.TempDir(), "config.yaml"), t.TempDir())
	joined := strings.Join(env, "\n")
	if strings.Contains(joined, "sk-test-openai") || strings.Contains(joined, "sk-test-anthropic") {
		t.Fatalf("child env leaked provider secret: %s", joined)
	}
	if strings.Contains(joined, "VIVY_ADDR=") {
		t.Fatalf("child env inherited VIVY_ADDR: %s", joined)
	}
	if strings.Contains(joined, "VIVY_POSTGRES_DSN=") || strings.Contains(joined, "postgres://") {
		t.Fatalf("child env inherited postgres DSN: %s", joined)
	}
	if !strings.Contains(joined, "VIVY_CONFIG=") {
		t.Fatalf("child env missing VIVY_CONFIG: %s", joined)
	}
}

func fixtureBundle(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "fixtures", "provider"))
	if err != nil {
		t.Fatal(err)
	}
	return path
}
