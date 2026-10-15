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
	"sync"
	"testing"
	"time"

	"agent-vivy/internal/cognitivecontract"
	"agent-vivy/internal/config"
	"agent-vivy/internal/domain"
	genassembly "agent-vivy/internal/generated/assembly"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/storage"
)

// Exercise production NewWithAssembly dependencies, rather than constructing a
// runtime Service with the evidence store already supplied by a test fixture.
func TestIssue58RealAssemblyRecapNativeToolsAndRestart(t *testing.T) {
	const exact = "全文\r\n\n中文 😀\n"
	const searchArgs = `{ "query" : "select:actmem_update" }`
	updateRaw, err := json.Marshal(map[string]any{"base_revision": 0, "operation_id": "assembly-native-edit", "text": exact})
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var captures []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, readErr := io.ReadAll(r.Body)
		if readErr != nil {
			t.Error(readErr)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		mu.Lock()
		captures = append(captures, string(raw))
		call := len(captures)
		mu.Unlock()
		delta := map[string]any{"role": "assistant", "content": "done"}
		finish := "stop"
		if call <= 2 {
			name, args := "tool_search", searchArgs
			if call == 2 {
				name, args = "actmem_update", string(updateRaw)
			}
			delta["content"] = ""
			delta["tool_calls"] = []any{map[string]any{"index": 0, "id": fmt.Sprintf("assembly-call-%d", call), "type": "function", "function": map[string]any{"name": name, "arguments": args}}}
			finish = "tool_calls"
		}
		payload, _ := json.Marshal(map[string]any{"id": "fixture", "object": "chat.completion.chunk", "model": "deepseek-flash", "choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": finish}}})
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", payload)
	}))
	defer server.Close()
	capture := func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), captures...)
	}
	root := t.TempDir()
	t.Setenv("VIVY_USER_HOME", root)
	t.Setenv("VIVY_PROVIDER", "deepseek")
	t.Setenv("VIVY_MODEL", "deepseek-flash") // Published catalog entry with known capacity.
	t.Setenv("VIVY_API_BASE", server.URL)
	t.Setenv("DEEPSEEK_API_KEY", "synthetic-loopback-test")
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
	main := &issue58SplitMain{}
	open := func() *App {
		a, err := NewWithAssembly(context.Background(), cfg, genassembly.BuildDefault(), WithoutEars(), WithMainConversationResolver(main), WithInstructionRoot(cfg.Runtime.WorkspaceRoot))
		if err != nil {
			t.Fatal(err)
		}
		return a
	}
	a := open()
	t.Cleanup(func() { _ = a.Close() })
	owner := a.cognitive.(cognitivecontract.RecapProvider).Recap()
	head, err := owner.ReadRecap(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	const sid domain.SessionID = "assembly-recap-main"
	main.source = &storage.SubjectSourceBinding{SubjectHistoryBinding: storage.SubjectHistoryBinding{SubjectID: head.Binding.SubjectID, DomainID: head.Binding.DomainID}, SessionID: sid, SchemaVersion: 1, ConversationID: "assembly-recap-conversation", PolicyID: domain.SubjectResidentPolicy, ToolPolicyRef: "assembly-test-tools", Scope: "personal", Grant: storage.SubjectGrantSnapshot{Reference: "assembly-test-whole-domain", Version: 1, WholeDomainRead: true, Snapshot: json.RawMessage(`{"test_fixture":true}`)}}
	if err := main.source.Validate(); err != nil {
		t.Fatal(err)
	}
	// A titled session avoids an unrelated automatic-title provider request in
	// this exact foreground native-tool protocol fixture.
	if err := a.backend.CreateSession(context.Background(), domain.Session{ID: sid, Title: "Recap native assembly regression", CreatedAt: time.Now().UnixMilli()}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	auth, authErr := a.service.AuthorizeRecap(ctx, sid, "", "")
	status, statusErr := a.service.ContextStatus(ctx, sid)
	t.Logf("PRECHECK authorization=%+v error=%v context=%+v error=%v", auth, authErr, status, statusErr)
	if authErr != nil || statusErr != nil || !status.ModelLimitKnown || status.ModelLimitTokens <= 0 || auth.SubjectID != head.Binding.SubjectID || auth.DomainID != head.Binding.DomainID {
		t.Fatal("actual owner authority and known-model status must be valid before native admission")
	}
	run := func(input string) domain.Run {
		id, err := a.service.Run(ctx, sid, input)
		if err != nil || !a.service.WaitIdle(ctx) {
			t.Fatalf("native admission/idle: %v", err)
		}
		got, err := a.backend.GetRun(ctx, id)
		if err != nil || got.Status != domain.RunCompleted {
			iterator, replayErr := a.backend.Replay(ctx, id, 0)
			if replayErr == nil {
				for iterator.Next() {
					event := iterator.Value().Event
					t.Logf("NATIVE_EVENT %s %s", event.Type, event.Payload)
				}
				iterator.Close()
			}
			t.Fatalf("native assembled run=%+v error=%v provider_calls=%d", got, err, len(capture()))
		}
		return got
	}
	first := run("Search for Recap tools and apply the exact full text")
	wire := capture()
	for _, raw := range wire {
		t.Logf("ASSEMBLY_HTTP_CAPTURE %s", raw)
	}
	if len(wire) != 3 || strings.Contains(wire[0], `"name":"actmem_update"`) || !strings.Contains(wire[1], `"name":"actmem_update"`) {
		t.Fatalf("deferred search/schema boundary: %d provider calls", len(wire))
	}
	head, err = owner.ReadRecap(ctx)
	if err != nil || head.Text != exact || head.Revision != 1 {
		t.Fatalf("native edit must use the same bound owner: %+v %v", head, err)
	}
	store := a.backend.(storage.SubjectSourceStore)
	rows, interaction, err := store.ReadSubjectRunEvidence(ctx, main.source.SubjectHistoryBinding, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	toolRows := map[string]domain.Message{}
	for _, row := range rows {
		if row.Role == domain.RoleTool {
			if _, duplicate := toolRows[row.ToolCallID]; duplicate {
				t.Fatalf("duplicate exact tool row %s", row.ToolCallID)
			}
			toolRows[row.ToolCallID] = row
		}
	}
	if len(toolRows) != 2 {
		t.Fatalf("expected exactly search and update evidence, got %d", len(toolRows))
	}
	for i, expectedArgs := range []string{searchArgs, string(updateRaw)} {
		id := fmt.Sprintf("assembly-call-%d", i+1)
		op, err := a.backend.(storage.ToolOperationStore).GetToolOperation(ctx, first.ID, id)
		if err != nil || op.State != domain.ToolOperationCompleted || op.Failure != "" {
			t.Fatalf("completed native tool operation: %+v %v", op, err)
		}
		arguments, found, err := a.backend.Blobs().Get(ctx, storage.ToolEvidenceArgumentsID(op))
		if err != nil || !found || string(arguments) != expectedArgs || storage.VerifyToolEvidenceArguments(op, arguments) != nil {
			t.Fatalf("exact raw arguments for %s: %q found=%v error=%v", id, arguments, found, err)
		}
		ref, err := storage.DecodeToolEvidenceReference(op)
		if err != nil {
			t.Fatal(err)
		}
		raw, found, err := a.backend.Blobs().Get(ctx, ref.ID)
		if err != nil || !found {
			t.Fatalf("tool evidence blob: found=%v error=%v", found, err)
		}
		evidence, err := storage.VerifyToolEvidence(ref, raw)
		row, exists := toolRows[id]
		if err != nil || !exists || row.Content != evidence.Result || row.ToolError != evidence.Failure || row.ToolName != op.ToolName || evidence.Failure != "" || evidence.Outcome == nil || *evidence.Outcome != (storage.ToolEvidenceOutcome{}) {
			t.Fatalf("full C2 tool evidence %s: row=%+v evidence=%+v error=%v", id, row, evidence, err)
		}
		if i == 0 {
			var request struct {
				Messages []struct {
					Role       string
					Content    string
					ToolCallID string `json:"tool_call_id"`
				}
			}
			if err := json.Unmarshal([]byte(wire[1]), &request); err != nil {
				t.Fatal(err)
			}
			matched := false
			for _, message := range request.Messages {
				if message.Role == "tool" && message.ToolCallID == id && message.Content == evidence.Result {
					matched = true
				}
			}
			if !strings.Contains(evidence.Result, "actmem_update") || !matched {
				t.Fatal("native search must retain the actual complete selection result")
			}
		} else {
			var outcome struct {
				Status string
				Head   struct {
					Text     string
					Revision uint64
				}
			}
			if json.Unmarshal([]byte(evidence.Result), &outcome) != nil || outcome.Status != "applied" || outcome.Head.Text != exact || outcome.Head.Revision != 1 {
				t.Fatalf("full same-owner update result: %s", evidence.Result)
			}
		}
	}
	digest, err := storage.SubjectEvidenceDigest(rows)
	through, captureErr := store.CaptureSubjectHistory(ctx, main.source.SubjectHistoryBinding)
	items, listErr := store.ListSubjectInteractions(ctx, main.source.SubjectHistoryBinding, 0, through, 10)
	// ReadSubjectRunEvidence derives the run evidence identity (Sequence=0);
	// the sealed index owns its assigned sequence. Compare the actual identity
	// and exact digest, and independently require that one indexed seal.
	if err != nil || captureErr != nil || listErr != nil || through != 1 || len(items) != 1 || items[0].Sequence != 1 || items[0].ID != interaction.ID || items[0].RunID != first.ID || items[0].SessionID != sid || items[0].TerminalSeq != interaction.TerminalSeq || items[0].SourceDigest != digest || interaction.RunID != first.ID || interaction.SourceDigest != digest {
		t.Fatalf("exactly one complete C2 seal: through=%d items=%+v interaction=%+v errors=%v/%v/%v", through, items, interaction, err, captureErr, listErr)
	}
	if err := a.CloseContext(ctx); err != nil {
		t.Fatal(err)
	}
	a = open() // Normal host close/reopen, with production dependencies again.
	reopened := a.cognitive.(cognitivecontract.RecapProvider).Recap()
	head, err = reopened.ReadRecap(ctx)
	if err != nil || head.Text != exact || head.Revision != 1 {
		t.Fatalf("same owner after host restart: %+v %v", head, err)
	}
	reopenedRows, reopenedInteraction, err := a.backend.(storage.SubjectSourceStore).ReadSubjectRunEvidence(ctx, main.source.SubjectHistoryBinding, first.ID)
	reopenedDigest, digestErr := storage.SubjectEvidenceDigest(reopenedRows)
	if err != nil || digestErr != nil || reopenedInteraction != interaction || reopenedDigest != digest {
		t.Fatalf("exact evidence did not survive host restart: %v/%v", err, digestErr)
	}
	next := run("Continue after the actual host restart")
	if next.ID == first.ID || len(capture()) != 4 || !strings.Contains(capture()[3], `"name":"actmem_update"`) {
		t.Fatal("successful next admission must rehydrate the authorized deferred schema")
	}
	through, err = a.backend.(storage.SubjectSourceStore).CaptureSubjectHistory(ctx, main.source.SubjectHistoryBinding)
	if err != nil || through != 2 {
		t.Fatalf("next admission must add only its own seal: through=%d error=%v", through, err)
	}
}
