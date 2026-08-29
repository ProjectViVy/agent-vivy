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
		Server:    config.Server{Addr: "127.0.0.1:0", AllowedOrigins: []string{"http://127.0.0.1:3015"}},
		Storage:   config.Storage{Backend: "sqlite", SQLite: config.SQLite{Path: filepath.Join(t.TempDir(), "route.db")}},
		Providers: config.Providers{Active: "openai", BundleDir: filepath.Join("..", "..", "fixtures", "provider"), OpenAI: config.Provider{EnvKey: "OPENAI_API_KEY", DefaultModel: "gpt-4o-mini"}, Anthropic: config.Provider{EnvKey: "ANTHROPIC_API_KEY", DefaultModel: "claude-sonnet-4-5"}},
		Runtime:   config.Runtime{StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10},
		Tools:     config.Tools{Enabled: []string{"echo_info"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.backend.Close() })
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/rpc/bootstrap", nil)
	request.Header.Set("Origin", "http://127.0.0.1:3015")
	a.httpServer.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "protocol_version") {
		t.Fatalf("bootstrap response = %d %q", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "http://127.0.0.1:3015" {
		t.Fatalf("allow-origin = %q", got)
	}
	denied := httptest.NewRecorder()
	deniedRequest := httptest.NewRequest(http.MethodGet, "/rpc/bootstrap", nil)
	deniedRequest.Header.Set("Origin", "http://127.0.0.1:3016")
	a.httpServer.Handler.ServeHTTP(denied, deniedRequest)
	if denied.Code != http.StatusForbidden {
		t.Fatalf("denied status = %d, want %d", denied.Code, http.StatusForbidden)
	}
}
