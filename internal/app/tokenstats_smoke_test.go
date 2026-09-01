package app

// Token-stats real-path smoke: compose the full app on a scratch SQLite
// Journal, seed priced and unpriced usage rows through the storage layer,
// then call stats/tokens over the live WebSocket control plane and assert
// the D9 wire contract end to end (storage → resolver → aggregation → JSON).

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"agent-vivy/internal/config"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/storage"
)

func TestTokenStatsRPCSmoke(t *testing.T) {
	runtime.SetEngineVersionOverride(pinnedEinoVersion)
	t.Cleanup(func() { runtime.SetEngineVersionOverride("") })
	ctx := context.Background()
	cfg := config.Config{
		Server:  config.Server{Addr: "127.0.0.1:0"},
		Storage: config.Storage{Backend: "sqlite", SQLite: config.SQLite{Path: filepath.Join(t.TempDir(), "usage.db")}},
		Providers: config.Providers{
			Active: "openai", BundleDir: filepath.Join("..", "..", "fixtures", "provider"),
			OpenAI:    config.Provider{EnvKey: "OPENAI_API_KEY", DefaultModel: "gpt-4o"},
			Anthropic: config.Provider{EnvKey: "ANTHROPIC_API_KEY", DefaultModel: "claude-sonnet-4-5"},
		},
		Runtime: config.Runtime{StreamBuffer: 256, MaxEventPayloadBytes: 65536},
	}
	a, err := New(ctx, cfg)
	if err != nil {
		t.Fatalf("compose app: %v", err)
	}
	ts := httptest.NewServer(a.httpServer.Handler)
	t.Cleanup(func() {
		ts.Close()
		_ = a.backend.Close()
	})

	client := connectSmokeRPC(t, ts.URL, a.rpcToken)
	t.Cleanup(func() { _ = client.conn.Close() })
	callSmoke(t, client, "initialize", map[string]any{"protocol_version": "vivy.rpc.v1"})

	now := time.Now().UnixMilli()
	for _, sess := range []struct{ id, title string }{
		{"sess-priced", "Priced session"},
		{"sess-unpriced", "Unpriced session"},
	} {
		if err := a.backend.CreateSession(ctx, domain.Session{ID: domain.SessionID(sess.id), Title: sess.title, CreatedAt: now}); err != nil {
			t.Fatalf("seed session %s: %v", sess.id, err)
		}
	}
	seedRun := func(runID domain.RunID, sessionID domain.SessionID, model, usagePayload string) {
		if err := a.backend.CreateRun(ctx, domain.Run{
			ID: runID, SessionID: sessionID, Status: domain.RunCompleted,
			CreatedAt: now, Kind: domain.RunKindPrimary,
		}); err != nil {
			t.Fatalf("seed run %s: %v", runID, err)
		}
		started, err := json.Marshal(map[string]string{"provider": "openai", "model": model})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := a.backend.Append(ctx, storage.Commit{RunID: runID, Events: []domain.RunEvent{
			{RunID: runID, Type: domain.EventRunStarted, CreatedAt: now, PayloadVersion: 1, Payload: started},
			{RunID: runID, Type: domain.EventModelUsage, CreatedAt: now, PayloadVersion: 1, Payload: []byte(usagePayload)},
		}}); err != nil {
			t.Fatalf("seed journal %s: %v", runID, err)
		}
	}
	// gpt-4o is priced (2.5 USD/M in, 10 USD/M out): 1.0M prompt + 0.1M
	// completion = 3.5 USD; 700K prompt tokens served from cache.
	seedRun("run-priced", "sess-priced", "gpt-4o",
		`{"prompt_tokens":1000000,"completion_tokens":100000,"total_tokens":1100000,"cached_tokens":700000}`)
	// custom-model has no reference pricing: tokens count, cost stays unknown.
	seedRun("run-unpriced", "sess-unpriced", "custom-model",
		`{"prompt_tokens":50,"completion_tokens":10,"total_tokens":60}`)

	raw := callSmoke(t, client, "stats/tokens", map[string]any{"period": "1d", "tz_offset_minutes": 0})
	var snap struct {
		Period string `json:"period"`
		Total  struct {
			TotalInput   int     `json:"total_input"`
			TotalOutput  int     `json:"total_output"`
			TotalTokens  int     `json:"total_tokens"`
			TotalCached  int     `json:"total_cached"`
			RequestCount int     `json:"request_count"`
			TotalCostUSD float64 `json:"total_cost_usd"`
			CostKnown    bool    `json:"cost_known"`
		} `json:"total"`
		Models []struct {
			Model       string  `json:"model"`
			TotalTokens int     `json:"total_tokens"`
			CostUSD     float64 `json:"cost_usd"`
			CostKnown   bool    `json:"cost_known"`
		} `json:"models"`
		Sessions []struct {
			ID        string  `json:"id"`
			CostUSD   float64 `json:"cost_usd"`
			CostKnown bool    `json:"cost_known"`
		} `json:"sessions"`
	}
	decodeSmoke(t, raw, &snap)

	if snap.Period != "1d" {
		t.Fatalf("period = %q, want 1d", snap.Period)
	}
	total := snap.Total
	if total.TotalInput != 1000050 || total.TotalOutput != 100010 || total.TotalTokens != 1100060 {
		t.Fatalf("total token fields = %+v, want unpriced rows still counted", total)
	}
	if total.TotalCached != 700000 {
		t.Fatalf("total_cached = %d, want 700000", total.TotalCached)
	}
	if total.RequestCount != 2 {
		t.Fatalf("request_count = %d, want 2", total.RequestCount)
	}
	if !total.CostKnown || total.TotalCostUSD != 3.5 {
		t.Fatalf("total cost = (%v, known=%v), want (3.5, true)", total.TotalCostUSD, total.CostKnown)
	}
	byModel := map[string]struct {
		tokens int
		cost   float64
		known  bool
	}{}
	for _, m := range snap.Models {
		byModel[m.Model] = struct {
			tokens int
			cost   float64
			known  bool
		}{m.TotalTokens, m.CostUSD, m.CostKnown}
	}
	if got := byModel["gpt-4o"]; got.tokens != 1100000 || got.cost != 3.5 || !got.known {
		t.Fatalf("gpt-4o share = %+v, want tokens 1100000 cost 3.5 known", got)
	}
	if got := byModel["custom-model"]; got.tokens != 60 || got.known {
		t.Fatalf("custom-model share = %+v, want tokens 60 with cost unknown", got)
	}
	bySession := map[string]struct {
		cost  float64
		known bool
	}{}
	for _, s := range snap.Sessions {
		bySession[s.ID] = struct {
			cost  float64
			known bool
		}{s.CostUSD, s.CostKnown}
	}
	if got := bySession["sess-priced"]; got.cost != 3.5 || !got.known {
		t.Fatalf("sess-priced = %+v, want cost 3.5 known", got)
	}
	if got := bySession["sess-unpriced"]; got.cost != 0 || got.known {
		t.Fatalf("sess-unpriced = %+v, want cost unknown, never 0-known", got)
	}
}
