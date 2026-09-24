package app

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"agent-vivy/internal/channelhost"
	"agent-vivy/internal/channelhost/fake"
	"agent-vivy/internal/config"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/runtime"
	plugin "agent-vivy/sdk/port/channel"
)

type blockingShutdownChannel struct {
	*fake.Channel
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (c *blockingShutdownChannel) Stop(ctx context.Context) error {
	c.once.Do(func() { close(c.entered) })
	select {
	case <-c.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestAppShutdownStopsGoalAdmissionBeforeChannelDrain(t *testing.T) {
	for _, mode := range []string{"Close", "Run"} {
		t.Run(mode, func(t *testing.T) {
			runtime.SetEngineVersionOverride(pinnedEinoVersion)
			t.Cleanup(func() { runtime.SetEngineVersionOverride("") })
			ctx := context.Background()
			cfg := config.Config{
				Server:    config.Server{Addr: "127.0.0.1:0"},
				Storage:   config.Storage{Backend: "sqlite", SQLite: config.SQLite{Path: filepath.Join(t.TempDir(), "shutdown-order.db")}},
				Providers: config.Providers{Active: "deepseek"},
				Runtime:   config.Runtime{StreamBuffer: 256, MaxEventPayloadBytes: 65536},
				Tools:     config.Tools{Enabled: []string{"echo_info"}, Approval: config.Approval{Expiration: 5 * time.Minute}},
			}
			a, err := New(ctx, cfg, WithoutEars())
			if err != nil {
				t.Fatalf("compose app: %v", err)
			}
			channel := &blockingShutdownChannel{Channel: fake.New(), entered: make(chan struct{}), release: make(chan struct{})}
			channel.Publish = func(context.Context, plugin.ChannelEnv) error { return nil }
			a.channels = channelhost.New(channelhost.Deps{
				Journal: a.backend, Messages: a.backend, Sessions: a.backend,
				Run: func(context.Context, domain.SessionID, string, *domain.Provenance) (domain.RunID, error) {
					return "", nil
				},
				Channels: []plugin.Channel{channel},
				Config:   config.Channels{"fake": {Enabled: true, AllowFrom: []string{"alice"}}},
			})
			if err := a.channels.StartAll(ctx); err != nil {
				t.Fatalf("start channel fixture: %v", err)
			}
			const sessionID domain.SessionID = "sess-shutdown-order"
			if err := a.backend.CreateSession(ctx, domain.Session{ID: sessionID, CreatedAt: 1}); err != nil {
				t.Fatalf("create session: %v", err)
			}
			if _, err := a.service.CommitWork(ctx, domain.WorkMutation{
				SessionID: sessionID, RequestID: "create-shutdown-goal", RequestHash: "create-shutdown-goal",
				Kind: domain.WorkEventGoalCreated, Goal: domain.GoalRef{ID: "shutdown-goal", Revision: 1},
				Objective: "do not start during shutdown", MaxRounds: 2,
			}); err != nil {
				t.Fatalf("create Goal: %v", err)
			}
			done := make(chan error, 1)
			if mode == "Close" {
				go func() { done <- a.Close() }()
			} else {
				runCtx, cancel := context.WithCancel(ctx)
				go func() { done <- a.Run(runCtx) }()
				cancel()
			}
			var releaseOnce sync.Once
			release := func() { releaseOnce.Do(func() { close(channel.release) }) }
			defer func() {
				release()
				select {
				case err := <-done:
					if err != nil {
						t.Errorf("shutdown: %v", err)
					}
				case <-time.After(shutdownGrace + 3*time.Second):
					t.Error("app shutdown did not complete")
				}
			}()
			select {
			case <-channel.entered:
			case <-time.After(5 * time.Second):
				t.Fatal("shutdown did not enter channel drain")
			}
			// Stop is held, so the Journal remains open at this app-level
			// shutdown boundary. A late Goal wake must create no run.
			a.service.WakeGoal(sessionID)
			idleCtx, idleCancel := context.WithTimeout(ctx, 5*time.Second)
			if !a.service.WaitIdle(idleCtx) {
				idleCancel()
				t.Fatal("Goal wake did not settle during channel drain")
			}
			idleCancel()
			runs, err := a.backend.ListRunsBySession(ctx, sessionID)
			if err != nil || len(runs) != 0 {
				t.Fatalf("runs admitted after app shutdown began = %+v / %v", runs, err)
			}
		})
	}
}

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
			Active: "deepseek",
		},
		Runtime: config.Runtime{StreamBuffer: 256, MaxEventPayloadBytes: 65536},
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
	if _, err := a.mcpBackend.ListTools(ctx, "", "missing"); err == nil || !strings.Contains(err.Error(), "backend is closed") {
		t.Fatalf("MCP backend remained usable after app shutdown: %v", err)
	}
	if err := a.assembly.Close(ctx); err != nil {
		t.Fatalf("second generated Assembly close was not idempotent: %v", err)
	}
}
