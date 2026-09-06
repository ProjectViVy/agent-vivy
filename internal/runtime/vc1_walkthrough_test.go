package runtime

import (
	"context"
	"database/sql"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/tools"
)

// waitForWalkthroughStatus is waitForRunStatus with a race-tolerant
// deadline: this test's bash steps spawn real shells, which -race slows
// past the shared 5s bound.
func waitForWalkthroughStatus(t *testing.T, runs storage.RunStore, runID domain.RunID, want domain.RunStatus) {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		r, err := runs.GetRun(context.Background(), runID)
		if err == nil && r.Status == want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("run %s never reached status %s", runID, want)
}

// TestVC1Walkthrough replays a scripted Vivy-Code turn over the full stack:
// the model writes a failing script, reads it back, locates the defect with
// grep, repairs it via multiedit, and verifies the fix on disk through the
// bash tool. The session is pinned to the 'auto' approval policy so the whole
// chain runs without an approval interrupt; the journal must carry five clean
// tool.finished events, the multiedit diff payload, a file_versions chain
// ending on the fixed content, and a single completed terminal.
func TestVC1Walkthrough(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash is not available on this host")
	}
	ctx := context.Background()

	dbPath := filepath.Join(t.TempDir(), "vc1-walkthrough.db")
	backend, err := sqlite.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })

	root := t.TempDir()
	manager, err := NewWorkspaceManager(filepath.Join(root, "workspaces"))
	if err != nil {
		t.Fatalf("workspace manager: %v", err)
	}
	sandbox, err := NewSandboxManager(domain.SandboxModeWorkspaceWrite, root, nil, nil)
	if err != nil {
		t.Fatalf("sandbox manager: %v", err)
	}
	filesBackend := NewEinoFilesystemBackend(manager, sandbox)
	filesBackend.SetFileVersionRecorder(NewFileVersionRecorder(backend, nil))
	commandBackend := NewCommandBackend(manager, sandbox, nil, 0)

	ts, err := tools.NewRegistry(
		tools.NewWriteFile(filesBackend),
		tools.NewReadFile(filesBackend),
		tools.NewGrep(filesBackend),
		tools.NewMultiEdit(filesBackend),
		tools.NewBash(commandBackend),
	).Resolve([]string{tools.WriteFileName, tools.ReadFileName, tools.GrepName, tools.MultiEditName, tools.BashName})
	if err != nil {
		t.Fatalf("resolve tools: %v", err)
	}
	checkpoints, err := NewVersionedCheckpointStore(backend.Blobs(), "test-engine")
	if err != nil {
		t.Fatalf("checkpoint store: %v", err)
	}
	eng, err := NewEngine(ctx, NewScriptedModel(
		schema.AssistantMessage("", []schema.ToolCall{{
			ID:       "call-wt-write",
			Function: schema.FunctionCall{Name: tools.WriteFileName, Arguments: `{"path":"calc.sh","content":"#!/bin/sh\necho FIAL\n"}`},
		}}),
		schema.AssistantMessage("", []schema.ToolCall{{
			ID:       "call-wt-read",
			Function: schema.FunctionCall{Name: tools.ReadFileName, Arguments: `{"path":"calc.sh"}`},
		}}),
		schema.AssistantMessage("", []schema.ToolCall{{
			ID:       "call-wt-grep",
			Function: schema.FunctionCall{Name: tools.GrepName, Arguments: `{"pattern":"FIAL"}`},
		}}),
		schema.AssistantMessage("", []schema.ToolCall{{
			ID:       "call-wt-edit",
			Function: schema.FunctionCall{Name: tools.MultiEditName, Arguments: `{"path":"calc.sh","edits":[{"old_string":"echo FIAL","new_string":"echo PASS"}]}`},
		}}),
		schema.AssistantMessage("", []schema.ToolCall{{
			ID:       "call-wt-bash",
			Function: schema.FunctionCall{Name: tools.BashName, Arguments: `{"command":"grep PASS calc.sh"}`},
		}}),
		schema.AssistantMessage("Walkthrough complete: script fixed and verified.", nil),
	), ts, EngineConfig{StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10, Checkpoints: checkpoints, AutoApproveTools: []string{tools.WriteFileName, tools.MultiEditName}})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	svc := NewService(eng, "scripted", "scripted-v0", ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Notes: backend, Approvals: backend,
		Questions:          backend,
		Sessions:           backend,
		ApprovalExpiration: 5 * time.Minute, Sink: newTestSink(),
	})
	if err := backend.CreateSession(ctx, domain.Session{ID: "sess-vc1-walkthrough", Title: "vc1 walkthrough", CreatedAt: time.Now().UnixMilli()}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	if err := backend.UpdateSandboxPolicy(ctx, "sess-vc1-walkthrough", domain.SandboxModeWorkspaceWrite, domain.ApprovalPolicyAuto); err != nil {
		t.Fatalf("set session approval policy: %v", err)
	}

	runID, err := svc.Run(ctx, "sess-vc1-walkthrough", "fix the failing script and verify")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	// The bash step spawns real shell processes per tool call, which is
	// slow under -race; the shared 5s helper is too tight there.
	waitForWalkthroughStatus(t, backend, runID, domain.RunCompleted)

	events := replayAll(t, backend, runID)
	if i := indexOfType(events, domain.EventToolApprovalRequired); i >= 0 {
		t.Fatalf("auto-pinned walkthrough raised an approval interrupt at %d", i)
	}

	type finished struct {
		ToolCallID string `json:"tool_call_id"`
		ToolName   string `json:"tool_name"`
		Result     string `json:"result"`
		Error      string `json:"error"`
	}
	var done []finished
	var completed int
	for _, ev := range events {
		switch ev.Type {
		case domain.EventToolFinished:
			var p finished
			if err := json.Unmarshal(ev.Payload, &p); err != nil {
				t.Fatalf("decode tool.finished payload: %v", err)
			}
			if p.Error != "" {
				t.Fatalf("tool %s finished with error: %s", p.ToolName, p.Error)
			}
			done = append(done, p)
		case domain.EventRunCompleted:
			completed++
		case domain.EventRunFailed:
			t.Fatalf("run failed: %s", ev.Payload)
		}
	}
	if len(done) != 5 {
		t.Fatalf("tool.finished count = %d, want 5: %v", len(done), events)
	}
	wantOrder := []string{tools.WriteFileName, tools.ReadFileName, tools.GrepName, tools.MultiEditName, tools.BashName}
	for i, name := range wantOrder {
		if done[i].ToolName != name {
			t.Fatalf("tool.finished[%d] = %s, want %s", i, done[i].ToolName, name)
		}
	}
	if !strings.Contains(done[3].Result, `"diff"`) {
		t.Fatalf("multiedit result lost the diff payload: %s", done[3].Result)
	}
	if !strings.Contains(done[3].Result, "echo PASS") {
		t.Fatalf("multiedit diff does not carry the fix: %s", done[3].Result)
	}
	if !strings.Contains(done[4].Result, "PASS") {
		t.Fatalf("bash verify output missing PASS: %s", done[4].Result)
	}
	if completed != 1 {
		t.Fatalf("run.completed count = %d, want single terminal", completed)
	}

	// The version chain must end on the fixed content.
	probe, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("probe open: %v", err)
	}
	defer func() { _ = probe.Close() }()
	rows, err := probe.QueryContext(ctx,
		`SELECT content FROM file_versions WHERE session_id = ? AND path = ? ORDER BY version`, "sess-vc1-walkthrough", "calc.sh")
	if err != nil {
		t.Fatalf("query file_versions: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var all [][]byte
	count := 0
	for rows.Next() {
		count++
		content := []byte{}
		if err := rows.Scan(&content); err != nil {
			t.Fatalf("scan version: %v", err)
		}
		all = append(all, content)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate versions: %v", err)
	}
	if count < 2 {
		t.Fatalf("file_versions chain for calc.sh has %d entries, want the created-file baseline plus the fix", count)
	}
	// The chain must open on the brand-new file's empty baseline (write_file
	// creation) and end on the fixed content.
	first, last := all[0], all[len(all)-1]
	if len(first) != 0 {
		t.Fatalf("file_versions chain = %q, want an empty pre-content baseline first", all)
	}
	if !strings.Contains(string(last), "echo PASS") {
		t.Fatalf("file_versions chain ends on %q, want fixed content", last)
	}
}
