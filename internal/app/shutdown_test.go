package app

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"agent-vivy/internal/config"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/runtime"
)

// E4: cancelling the app context must complete the bounded graceful
// shutdown (cancel -> drain -> HTTP close -> storage close) even with a
// run in flight, and Run must return within the grace window.
func TestAppShutdownBounded(t *testing.T) {
	// go test binaries lack embedded module metadata; anchor the
	// checkpoint store on the pinned eino version (see realsmoke_test.go).
	runtime.SetEngineVersionOverride(pinnedEinoVersion)
	t.Cleanup(func() { runtime.SetEngineVersionOverride("") })

	ctx := context.Background()
	cfg := config.Config{
		Server: config.Server{Addr: "127.0.0.1:8791"},
		Storage: config.Storage{
			Backend: "sqlite",
			SQLite:  config.SQLite{Path: filepath.Join(t.TempDir(), "shutdown.db")},
		},
		Providers: config.Providers{
			Active:    "openai",
			BundleDir: filepath.Join("..", "..", "fixtures", "provider"),
			OpenAI:    config.Provider{EnvKey: "OPENAI_API_KEY", DefaultModel: "gpt-4o"},
			Anthropic: config.Provider{EnvKey: "ANTHROPIC_API_KEY", DefaultModel: "claude-sonnet-4-5"},
		},
		Runtime: config.Runtime{Mock: true, StreamBuffer: 256, MaxEventPayloadBytes: 65536},
		Tools: config.Tools{
			Enabled:  []string{"echo_info", "write_note"},
			Approval: config.Approval{Expiration: 5 * time.Minute},
		},
	}
	a, err := New(ctx, cfg)
	if err != nil {
		t.Fatalf("compose app: %v", err)
	}

	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- a.Run(runCtx) }()
	time.Sleep(200 * time.Millisecond) // let the listener come up

	// A run in flight must not unbound the shutdown; the drain closes it
	// while storage is still open.
	if err := a.backend.CreateSession(ctx, domain.Session{
		ID: "sess-shutdown", Title: "shutdown", CreatedAt: time.Now().UnixMilli(),
	}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	if _, err := a.service.Run(ctx, "sess-shutdown", "hello during shutdown"); err != nil {
		t.Fatalf("start run: %v", err)
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned an error on graceful shutdown: %v", err)
		}
	case <-time.After(shutdownGrace + 3*time.Second):
		t.Fatal("shutdown did not complete within the bounded window")
	}
}
