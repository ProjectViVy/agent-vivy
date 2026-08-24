package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"agent-vivy/internal/config"
	"agent-vivy/internal/runtime"
)

func TestRPCBootstrapRoutePrecedesUIShell(t *testing.T) {
	runtime.SetEngineVersionOverride(pinnedEinoVersion)
	t.Cleanup(func() { runtime.SetEngineVersionOverride("") })
	a, err := New(context.Background(), config.Config{
		Server:    config.Server{Addr: "127.0.0.1:0"},
		Storage:   config.Storage{Backend: "sqlite", SQLite: config.SQLite{Path: filepath.Join(t.TempDir(), "route.db")}},
		Providers: config.Providers{Active: "openai", BundleDir: filepath.Join("..", "..", "fixtures", "provider"), OpenAI: config.Provider{EnvKey: "OPENAI_API_KEY", DefaultModel: "gpt-4o-mini"}, Anthropic: config.Provider{EnvKey: "ANTHROPIC_API_KEY", DefaultModel: "claude-sonnet-4-5"}},
		Runtime:   config.Runtime{Mock: true, StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10},
		Tools:     config.Tools{Enabled: []string{"echo_info"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.backend.Close() })
	recorder := httptest.NewRecorder()
	a.httpServer.Handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/rpc/bootstrap", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "protocol_version") {
		t.Fatalf("bootstrap response = %d %q", recorder.Code, recorder.Body.String())
	}
}
