package app

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/sdk/plugin"
)

type statusPlugin struct {
	byRoot map[string][]plugin.LanguageServerStatus
	block  <-chan struct{}
}

func (*statusPlugin) Name() string           { return "status" }
func (*statusPlugin) Seam() plugin.Seam      { return plugin.SeamToolWorld }
func (*statusPlugin) Grants() []plugin.Grant { return nil }
func (*statusPlugin) Tools() []plugin.Tool   { return nil }
func (p *statusPlugin) LanguageServerStatuses(_ context.Context, root string) []plugin.LanguageServerStatus {
	if p.block != nil {
		<-p.block
	}
	return append([]plugin.LanguageServerStatus(nil), p.byRoot[root]...)
}

func TestLanguageServerStatusSourceScopesExistingSessionWorkspaces(t *testing.T) {
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "status.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Close()
	manager, err := runtime.NewWorkspaceManager(filepath.Join(t.TempDir(), "workspaces"))
	if err != nil {
		t.Fatal(err)
	}
	for _, sessionID := range []domain.SessionID{"session-a", "session-b"} {
		if err := backend.CreateSession(ctx, domain.Session{ID: sessionID, Title: string(sessionID), CreatedAt: 1}); err != nil {
			t.Fatal(err)
		}
		runID := domain.RunID("run-" + string(sessionID[len("session-"):]))
		if err := backend.CreateRun(ctx, domain.Run{ID: runID, SessionID: sessionID, Status: domain.RunAccepted, CreatedAt: 2, Kind: domain.RunKindPrimary, RootID: runID}); err != nil {
			t.Fatal(err)
		}
	}
	workspaceA, err := manager.Ensure(ctx, "run-a")
	if err != nil {
		t.Fatal(err)
	}
	workspaceB, err := manager.Ensure(ctx, "run-b")
	if err != nil {
		t.Fatal(err)
	}
	if err := backend.CreateRun(ctx, domain.Run{ID: "run-a-new", SessionID: "session-a", Status: domain.RunAccepted, CreatedAt: 3, Kind: domain.RunKindPrimary, RootID: "run-a-new"}); err != nil {
		t.Fatal(err)
	}
	workspaceANew, err := manager.Ensure(ctx, "run-a-new")
	if err != nil {
		t.Fatal(err)
	}
	provider := &statusPlugin{byRoot: map[string][]plugin.LanguageServerStatus{
		workspaceA.Path:    {{Language: "go", State: "initialized"}, {Language: "typescript", State: "starting"}},
		workspaceB.Path:    {{Language: "python", State: "initialized"}},
		workspaceANew.Path: {{Language: "rust", State: "initialized"}},
	}}
	source := buildLanguageServerStatusSource([]plugin.Plugin{provider}, backend, manager)
	if source == nil {
		t.Fatal("packed status provider was not wired")
	}
	got, err := source(ctx, "session-a")
	if err != nil {
		t.Fatal(err)
	}
	if !got.Known || len(got.Servers) != 1 || got.Servers[0].Language != "rust" {
		t.Fatalf("session-a statuses = %+v", got)
	}
	if buildLanguageServerStatusSource(nil, backend, manager) != nil {
		t.Fatal("generation without an LSP owner exposed a known status surface")
	}
}

func TestLanguageServerStatusSourceTimesOutBlockingProvider(t *testing.T) {
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "status-timeout.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Close()
	if err := backend.CreateSession(ctx, domain.Session{ID: "session", Title: "session", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := backend.CreateRun(ctx, domain.Run{ID: "run", SessionID: "session", Status: domain.RunAccepted, CreatedAt: 2, Kind: domain.RunKindPrimary, RootID: "run"}); err != nil {
		t.Fatal(err)
	}
	manager, err := runtime.NewWorkspaceManager(filepath.Join(t.TempDir(), "workspaces"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Ensure(ctx, "run"); err != nil {
		t.Fatal(err)
	}
	block := make(chan struct{})
	provider := &statusPlugin{block: block}
	source := buildLanguageServerStatusSource([]plugin.Plugin{provider}, backend, manager)
	started := time.Now()
	snapshot, err := source(ctx, "session")
	close(block)
	if err != nil || snapshot.Known || time.Since(started) > time.Second {
		t.Fatalf("blocking provider result = %+v/%v after %s", snapshot, err, time.Since(started))
	}
}
