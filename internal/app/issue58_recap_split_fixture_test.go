package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"agent-vivy/internal/cognitivecontract"
	"agent-vivy/internal/config"
	"agent-vivy/internal/domain"
	genassembly "agent-vivy/internal/generated/assembly"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/conformance"
	gardenmemory "github.com/ProjectViVy/laputa/garden/memory"
	"github.com/ProjectViVy/laputa/laputa/evolution"
)

type issue58SplitMain struct{ source *storage.SubjectSourceBinding }

func (m *issue58SplitMain) ResolveMainConversation(context.Context) (cognitivecontract.MainConversationBinding, error) {
	if m.source == nil {
		return cognitivecontract.MainConversationBinding{}, storage.ErrSubjectBinding
	}
	s := m.source
	return cognitivecontract.MainConversationBinding{SubjectID: s.SubjectID, DomainID: s.DomainID, ConversationID: s.ConversationID, ActiveSessionID: s.SessionID, PolicyID: s.PolicyID, ToolPolicyRef: s.ToolPolicyRef, SchemaVersion: s.SchemaVersion, Grant: s.Grant}, nil
}

// Opt-in test process for Vite3015/Playwright. There is no production toggle,
// imported tenant state, alternate action handler, or model/network dependency.
// VIVY_ISSUE58_SPLIT_TEST=1 go test ./internal/app -run '^TestIssue58RecapSplitFixture$' -v
func TestIssue58RecapSplitFixture(t *testing.T) {
	if os.Getenv("VIVY_ISSUE58_SPLIT_TEST") != "1" {
		t.Skip("opt-in disposable split browser fixture")
	}
	root := t.TempDir()
	t.Setenv("VIVY_USER_HOME", root)
	t.Setenv("VIVY_PROVIDER", "deepseek")
	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "fixture-summary", "object": "chat.completion", "model": "deepseek-flash", "choices": []any{map[string]any{"index": 0, "message": map[string]any{"role": "assistant", "content": "完整的测试回顾\n\n第二段保持原样。\n"}, "finish_reason": "stop"}}})
	}))
	defer model.Close()
	t.Setenv("VIVY_API_BASE", model.URL)
	t.Setenv("VIVY_MODEL", "deepseek-flash")
	t.Setenv("DEEPSEEK_API_KEY", "issue58-synthetic-loopback-key")
	runtime.SetEngineVersionOverride(pinnedEinoVersion)
	t.Cleanup(func() { runtime.SetEngineVersionOverride("") })
	cfg := config.Default()
	cfg.Storage.DataDir = root
	cfg.Storage.SQLite.Path = filepath.Join(root, "fixture.db")
	cfg.Runtime.WorkspaceRoot = filepath.Join(root, "workspace")
	if err := os.MkdirAll(cfg.Runtime.WorkspaceRoot, 0700); err != nil {
		t.Fatal(err)
	}
	cfg.Governance.Profile = string(domain.PolicyProfileFullAuto)
	cfg.Runtime.Cron.Enabled = false
	cfg.Server.Addr = "127.0.0.1:8787"
	main := &issue58SplitMain{}
	var contribution *storage.SubjectSourceBinding
	resolveSource := func(_ context.Context, sid domain.SessionID) (*storage.SubjectSourceBinding, error) {
		if contribution != nil && sid == contribution.SessionID {
			return contribution, nil
		}
		return nil, nil
	}
	app, err := NewWithAssembly(context.Background(), cfg, genassembly.BuildDefault(), WithoutEars(), WithMainConversationResolver(main), WithSubjectSources(resolveSource), WithInstructionRoot(cfg.Runtime.WorkspaceRoot))
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	if app.cognitive == nil {
		t.Fatal("default cognitive owner absent")
	}
	owner := app.cognitive.(cognitivecontract.RecapProvider).Recap()
	head, err := owner.ReadRecap(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	const session domain.SessionID = "issue58-recap-main"
	main.source = &storage.SubjectSourceBinding{SubjectHistoryBinding: storage.SubjectHistoryBinding{SubjectID: head.Binding.SubjectID, DomainID: head.Binding.DomainID}, SessionID: session, SchemaVersion: 1, ConversationID: "issue58-recap-test-conversation", PolicyID: domain.SubjectResidentPolicy, ToolPolicyRef: "issue58-test-tool-policy", Scope: "personal", Grant: storage.SubjectGrantSnapshot{Reference: "issue58-test-whole-domain", Version: 1, WholeDomainRead: true, Snapshot: json.RawMessage(`{"test_fixture":true}`)}}
	if err = main.source.Validate(); err != nil {
		t.Fatal(err)
	}
	if err = app.backend.CreateSession(context.Background(), domain.Session{ID: session, Title: "Issue58 Recap fixture", CreatedAt: time.Now().UnixMilli()}); err != nil {
		t.Fatal(err)
	}
	if _, err = owner.CommitRecap(context.Background(), cognitivecontract.RecapCommit{OperationID: "fixture-seed", Cause: "edit", Text: "完整的测试回顾\n\n第二段保持原样。\n"}); err != nil {
		t.Fatal(err)
	}
	// Seed authorized contribution originals, then tombstone a real native BML
	// derived record and compact through the actual runtime.
	// Source navigation must still use retained core originals and file snapshots.
	contributor := conformance.Issue58Source("issue58-browser-contributor", false)
	contributor.SubjectHistoryBinding = main.source.SubjectHistoryBinding
	contribution = &contributor
	if err = app.backend.CreateSession(context.Background(), domain.Session{ID: contributor.SessionID, Title: "Historical contribution", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		admission := conformance.Issue58Admission(contributor, fmt.Sprintf("browser-source-%d", i))
		admission.Run.CreatedAt = int64(i + 1)
		admission.Message.CreatedAt = int64(i + 1)
		admission.Message.Content = strings.Repeat("exact-browser-original ", 1000)
		if i == 1 {
			admission.Message.Content = "latest browser tail"
		}
		admission.Message.FileContexts = []domain.FileContext{{Name: "historical.txt", Path: "past/historical.txt", Size: int64(len("immutable browser file snapshot")), Content: []byte("immutable browser file snapshot")}}
		if _, err = app.backend.(storage.RunAdmissionStore).CommitRunAdmission(context.Background(), admission); err != nil {
			t.Fatal(err)
		}
		if _, err = app.backend.Append(context.Background(), storage.Commit{RunID: admission.Run.ID, Events: []domain.RunEvent{{RunID: admission.Run.ID, Type: domain.EventRunCompleted, PayloadVersion: 1, Payload: []byte(`{"outcome":"completed"}`)}}}); err != nil {
			t.Fatal(err)
		}
		if err = app.backend.SetRunStatus(context.Background(), admission.Run.ID, domain.RunCompleted); err != nil {
			t.Fatal(err)
		}

	}
	selected := app.cognitive.(cognitivecontract.SubjectMemoryProvider).SubjectMemory()
	scope := evolution.Scope{SubjectID: head.Binding.SubjectID, Kind: evolution.ScopePersonal}
	if err = app.service.RepairSubjectHistory(context.Background(), contributor.SessionID); err != nil {
		t.Fatal(err)
	}
	rows, err := app.service.SubjectRecent(context.Background(), session, 0, 2, 10)
	if err != nil || len(rows) != 2 {
		t.Fatalf("fixture core originals: %+v %v", rows, err)
	}
	if _, err = selected.Mutate(context.Background(), gardenmemory.AuthorizedMutation{Scope: scope, DestinationID: "bml", OperationID: "browser-derived-create", PayloadDigest: "browser-derived-create", Operation: evolution.MutationCreate, RecordID: "browser-derived", ExpectedAbsent: true, Body: "exact-browser-original derived excerpt", Sources: []evolution.SourceRef{{SourceID: "vivy.core.c2", RecordID: rows[0].ID, Revision: rows[0].Sequence, Scope: scope}}, Inference: evolution.InferenceObserved}); err != nil {
		t.Fatal(err)
	}
	cards, err := selected.Search(context.Background(), gardenmemory.AuthorizedSearch{Scopes: []evolution.Scope{scope}, Query: "exact-browser-original", Limit: 10})
	if err != nil || len(cards.Items) != 1 {
		t.Fatalf("fixture native projection absent: %+v %v", cards, err)
	}
	for _, card := range cards.Items {
		if _, err = selected.Mutate(context.Background(), gardenmemory.AuthorizedMutation{Scope: scope, DestinationID: "bml", OperationID: "browser-remove-derived", PayloadDigest: "browser-remove-derived", Operation: evolution.MutationTombstone, RecordID: card.ID, ExpectedRevision: uint64(card.Revision), Inference: evolution.InferenceObserved}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = app.service.CompactSession(context.Background(), session, runtime.CompactOptions{}); err != nil {
		t.Fatal(err)
	}
	sourceHead, err := owner.ReadRecap(context.Background())
	if err != nil || sourceHead.Revision != 2 || sourceHead.CoveredThrough != 1 {
		t.Fatalf("fixture certified source missing: %+v %v", sourceHead, err)
	}
	duration := 300 * time.Second
	if seconds, parseErr := strconv.Atoi(os.Getenv("VIVY_ISSUE58_SPLIT_SECONDS")); parseErr == nil && seconds >= 10 && seconds <= 900 {
		duration = time.Duration(seconds) * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()
	ctx, stopSignal := signal.NotifyContext(ctx, os.Interrupt)
	defer stopSignal()
	if marker := os.Getenv("VIVY_ISSUE58_SPLIT_STOP_FILE"); marker != "" {
		go func() {
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					if _, err := os.Stat(marker); err == nil {
						cancel()
						return
					}
				}
			}
		}()
	}

	fmt.Printf("ISSUE58_SPLIT_READY session=%s backend=http://127.0.0.1:8787 disposable=true duration=%s\n", session, duration)
	if err = app.Run(ctx); err != nil {
		t.Fatal(err)
	}
}
