package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"agent-vivy/internal/cognitivecontract"
	"agent-vivy/internal/config"
	"agent-vivy/internal/domain"
	genassembly "agent-vivy/internal/generated/assembly"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/storage"
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
	t.Setenv("VIVY_API_BASE", "http://127.0.0.1:1/v1")
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
	app, err := NewWithAssembly(context.Background(), cfg, genassembly.BuildDefault(), WithoutEars(), WithMainConversationResolver(main), WithInstructionRoot(cfg.Runtime.WorkspaceRoot))
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
