package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"agent-vivy/internal/cognitivecontract"
	"agent-vivy/internal/config"
	"agent-vivy/internal/domain"
	genassembly "agent-vivy/internal/generated/assembly"
	memorymodule "agent-vivy/internal/modules/memory"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/storage"
	"github.com/ProjectViVy/agent-vivy/bml"
	gardenmemory "github.com/ProjectViVy/laputa/garden/memory"
	"github.com/ProjectViVy/laputa/laputa/evolution"
)

func TestIssue58SingleProjectionSink(t *testing.T) {
	var recall atomic.Bool
	var recallCalls atomic.Int32
	var observedQualified atomic.Bool
	var leakedLegacy atomic.Bool
	var rejectedLegacy atomic.Bool
	var legacyID atomic.Value
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		if strings.Contains(string(raw), "qualified-original-body") {
			observedQualified.Store(true)
		}
		if strings.Contains(string(raw), "subject_memory_unavailable") {
			rejectedLegacy.Store(true)
		}
		if strings.Contains(string(raw), "legacy-private-sentinel") {
			leakedLegacy.Store(true)
		}
		delta := map[string]any{"role": "assistant", "content": "completed"}
		finish := "stop"
		if recall.Load() {
			step := recallCalls.Add(1)
			name, args := "", ""
			switch step {
			case 1:
				name, args = "tool_search", `{"query":"select:memory_search"}`
			case 2:
				name, args = "memory_search", `{"query":"qualifiedneedle"}`
			case 3:
				name, args = "tool_search", `{"query":"select:memory_get"}`
			case 4:
				name, args = "memory_get", `{"id":"qualified-fixture","expected_revision":1}`
			case 5:
				name = "memory_get"
				encoded, _ := json.Marshal(map[string]any{"id": legacyID.Load().(string), "expected_revision": 1})
				args = string(encoded)
			}
			if name != "" {
				delta["content"] = ""
				delta["tool_calls"] = []any{map[string]any{"index": 0, "id": fmt.Sprintf("recall-call-%d", step), "type": "function", "function": map[string]any{"name": name, "arguments": args}}}
				finish = "tool_calls"
			}
		}
		payload, _ := json.Marshal(map[string]any{"id": "fixture", "object": "chat.completion.chunk", "model": "deepseek-flash", "choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": finish}}})
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", payload)
	}))
	defer server.Close()
	root := t.TempDir()
	t.Setenv("VIVY_USER_HOME", root)
	t.Setenv("VIVY_PROVIDER", "deepseek")
	t.Setenv("VIVY_MODEL", "deepseek-flash")
	t.Setenv("VIVY_API_BASE", server.URL)
	t.Setenv("DEEPSEEK_API_KEY", "fixture")
	runtime.SetEngineVersionOverride(pinnedEinoVersion)
	t.Cleanup(func() { runtime.SetEngineVersionOverride("") })
	cfg := config.Default()
	cfg.Storage.DataDir = root
	cfg.Storage.SQLite.Path = filepath.Join(root, "fixture.db")
	cfg.Runtime.WorkspaceRoot = filepath.Join(root, "workspace")
	cfg.Runtime.Cron.Enabled = false
	cfg.Governance.Profile = string(domain.PolicyProfileFullAuto)
	cfg.Tools.Enabled = append(cfg.Tools.Enabled, "memory_search", "memory_get")
	if cfg.Tools.Exposure == nil {
		cfg.Tools.Exposure = map[string]string{}
	}
	cfg.Tools.Exposure["memory_search"] = "deferred"
	cfg.Tools.Exposure["memory_get"] = "deferred"
	if err := os.MkdirAll(cfg.Runtime.WorkspaceRoot, 0700); err != nil {
		t.Fatal(err)
	}
	main := &issue58SplitMain{}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	a, err := NewWithAssembly(ctx, cfg, genassembly.BuildDefault(), WithoutEars(), WithMainConversationResolver(main), WithInstructionRoot(cfg.Runtime.WorkspaceRoot))
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	head, err := a.cognitive.(cognitivecontract.RecapProvider).Recap().ReadRecap(ctx)
	if err != nil {
		t.Fatal(err)
	}
	sid := domain.SessionID("single-sink-main")
	main.source = &storage.SubjectSourceBinding{SubjectHistoryBinding: storage.SubjectHistoryBinding{SubjectID: head.Binding.SubjectID, DomainID: head.Binding.DomainID}, SessionID: sid, SchemaVersion: 1, ConversationID: "single-sink-conversation", PolicyID: domain.SubjectResidentPolicy, ToolPolicyRef: "fixture-tools", Scope: "personal", Grant: storage.SubjectGrantSnapshot{Reference: "fixture-grant", Version: 1, WholeDomainRead: true, Snapshot: json.RawMessage(`{"fixture":true}`)}}
	if err = a.backend.CreateSession(ctx, domain.Session{ID: sid, Title: "single sink", CreatedAt: time.Now().UnixMilli()}); err != nil {
		t.Fatal(err)
	}
	rid, err := a.service.Run(ctx, sid, "singleprojectionneedle retained historical original")
	if err != nil || !a.service.WaitIdle(ctx) {
		t.Fatalf("run: %v", err)
	}
	run, err := a.backend.GetRun(ctx, rid)
	if err != nil || run.Status != domain.RunCompleted {
		t.Fatalf("run=%+v %v", run, err)
	}
	for i := 0; i < 2; i++ {
		if err = a.observerHost.DeliverRun(ctx, rid); err != nil {
			t.Fatal(err)
		}
	}
	selected := a.cognitive.(cognitivecontract.SubjectMemoryProvider).SubjectMemory()
	scope := evolution.Scope{SubjectID: head.Binding.SubjectID, Kind: evolution.ScopePersonal}
	cards, err := selected.Search(ctx, gardenmemory.AuthorizedSearch{Scopes: []evolution.Scope{scope}, Query: "singleprojectionneedle", Limit: 10})
	if err != nil || len(cards.Items) != 1 {
		t.Fatalf("selected sink did not project once: %+v %v", cards, err)
	}
	high, err := a.cognitive.Source().HighWatermark(ctx)
	if err != nil || high != 0 {
		t.Fatalf("legacy cognitive double write: %d %v", high, err)
	}
	iterator, err := a.backend.Replay(ctx, rid, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer iterator.Close()
	for iterator.Next() {
		event := iterator.Value().Event
		if event.Type == domain.EventRunCompleted {
			legacyID := fmt.Sprintf("run-history-%s-%d", rid, event.Seq)
			if _, err := memorymodule.Active().Home().ExportSource(ctx, legacyID, 1); err == nil {
				t.Fatal("legacy BML double write")
			}
		}
	}
	original, err := a.service.SubjectRecent(ctx, sid, 0, 1, 10)
	if err != nil || len(original) != 1 {
		t.Fatalf("core retained seal=%+v %v", original, err)
	}
	if err = iterator.Close(); err != nil {
		t.Fatal(err)
	}
	activated, err := a.cognitive.(cognitivecontract.RecapProvider).Recap().CommitRecap(ctx, cognitivecontract.RecapCommit{BaseRevision: head.Revision, OperationID: "activate-fixture-v3", Cause: "edit", Text: "resident text must survive ordinary coding"})
	if err != nil {
		t.Fatal(err)
	}
	coding := domain.SessionID("ordinary-coding-after-v3")
	if err = a.backend.CreateSession(ctx, domain.Session{ID: coding, Title: "ordinary coding", CreatedAt: time.Now().UnixMilli()}); err != nil {
		t.Fatal(err)
	}
	codingRun, err := a.service.Run(ctx, coding, "ordinary coding after native v3")
	if err != nil || !a.service.WaitIdle(ctx) {
		t.Fatalf("coding after v3: %v", err)
	}
	codingOutcome, err := a.backend.GetRun(ctx, codingRun)
	if err != nil || codingOutcome.Status != domain.RunCompleted {
		t.Fatalf("coding outcome=%+v %v", codingOutcome, err)
	}
	if err = a.observerHost.DeliverRun(ctx, codingRun); err != nil {
		t.Fatal(err)
	}
	unchanged, err := a.cognitive.(cognitivecontract.RecapProvider).Recap().ReadRecap(ctx)
	if err != nil || unchanged.Revision != activated.Revision || unchanged.Text != activated.Text {
		t.Fatalf("coding overwrote v3: %+v %v", unchanged, err)
	}
	for {
		high, err = a.cognitive.Source().HighWatermark(ctx)
		if err != nil || high != 0 {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("ordinary capture did not reach native activity watermark")
		case <-time.After(10 * time.Millisecond):
		}
	}
	if err != nil || high != 1 {
		t.Fatalf("ordinary raw source did not survive closed legacy activity: %d %v", high, err)
	}

	if _, err = selected.Mutate(ctx, gardenmemory.AuthorizedMutation{Scope: scope, DestinationID: "bml", OperationID: "qualified-fixture-create", PayloadDigest: "qualified-fixture-create", Operation: evolution.MutationCreate, RecordID: "qualified-fixture", ExpectedAbsent: true, Body: "qualifiedneedle qualified-original-body", Inference: evolution.InferenceObserved}); err != nil {
		t.Fatal(err)
	}
	legacy := a.memory.Add(ctx, bml.MemoryAddRequest{Content: "qualifiedneedle legacy-private-sentinel"})
	if legacy.Entry == nil {
		t.Fatalf("legacy fixture absent: %+v", legacy)
	}
	legacyID.Store(legacy.Entry.ID)
	recall.Store(true)
	recalled, err := a.service.Run(ctx, sid, "Find qualifiedneedle in memory and expand its saved original")
	if err != nil || !a.service.WaitIdle(ctx) {
		t.Fatalf("qualified recall: %v", err)
	}
	recalledRun, err := a.backend.GetRun(ctx, recalled)
	if err != nil || recalledRun.Status != domain.RunCompleted || recallCalls.Load() != 6 || !observedQualified.Load() || leakedLegacy.Load() || !rejectedLegacy.Load() {
		it, replayErr := a.backend.Replay(ctx, recalled, 0)
		if replayErr == nil {
			for it.Next() {
				event := it.Value().Event
				if event.Type == domain.EventRunFailed || event.Type == domain.EventToolFinished {
					t.Logf("QUALIFIED_FAILURE %s %s", event.Type, event.Payload)
				}
			}
			_ = it.Close()
		}
		t.Fatalf("qualified recall outcome=%+v err=%v calls=%d qualified=%v leaked=%v", recalledRun, err, recallCalls.Load(), observedQualified.Load(), leakedLegacy.Load())
	}

}
