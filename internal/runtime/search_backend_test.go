package runtime

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/tools"
)

func writeGrepFixtures(t *testing.T, workspace Workspace) {
	t.Helper()
	files := map[string]string{
		"a.txt":               "Needle here\nother line",
		"nested/b.go":         "needle again\nno match",
		"nested/deep/d.go":    "third needle\n",
		"node_modules/pkg.js": "needle in modules",
		"ignored/secret.txt":  "needle ignored",
		"blob.bin":            "\x00needle\x00",
		".gitignore":          "ignored/\nnode_modules/\n",
	}
	for name, content := range files {
		full := filepath.Join(workspace.Path, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
			t.Fatalf("mkdir fixture %s: %v", name, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatalf("write fixture %s: %v", name, err)
		}
	}
}

func matchPaths(matches []tools.GrepMatch) []string {
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		out = append(out, m.Path)
	}
	return out
}

func containsString(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

func matchPathsFromGlob(files []tools.GlobFileInfo) []string {
	out := make([]string, 0, len(files))
	for _, f := range files {
		out = append(out, f.Path)
	}
	return out
}

// newServiceSearchBackend builds a filesystem backend whose sandbox covers
// the whole workspaces tree, mirroring the service wiring; the per-run
// workspaces of other runs stay valid paths.
func newServiceSearchBackend(t *testing.T) *EinoFilesystemBackend {
	t.Helper()
	root := t.TempDir()
	manager, err := NewWorkspaceManager(filepath.Join(root, "workspaces"))
	if err != nil {
		t.Fatalf("workspace manager: %v", err)
	}
	sandbox, err := NewSandboxManager(domain.SandboxModeWorkspaceWrite, root, nil, nil)
	if err != nil {
		t.Fatalf("sandbox manager: %v", err)
	}
	return NewEinoFilesystemBackend(manager, sandbox)
}

func TestBackendGrepFallbackWalk(t *testing.T) {
	backend, workspace, runID := newFilesystemTestBackend(t)
	writeGrepFixtures(t, workspace)
	backend.rgPath = "" // force the pure-Go walk
	res, err := backend.Grep(tools.WithRunID(context.Background(), runID), tools.GrepRequest{Pattern: "[Nn]eedle"})
	if err != nil {
		t.Fatalf("grep: %v", err)
	}
	paths := matchPaths(res.Matches)
	for _, want := range []string{"a.txt", "nested/b.go", "nested/deep/d.go"} {
		if !containsString(paths, want) {
			t.Fatalf("fallback matches = %v, want %s", paths, want)
		}
	}
	// The fallback walk prunes ignored directories (.git, node_modules) but
	// has no .gitignore awareness; binaries are skipped either way.
	if containsString(paths, "node_modules/pkg.js") {
		t.Fatalf("fallback searched node_modules: %v", paths)
	}
	if containsString(paths, "blob.bin") {
		t.Fatalf("fallback searched a binary file: %v", paths)
	}
	for _, m := range res.Matches {
		if m.Path == "a.txt" && (m.Line != 1 || !strings.Contains(m.Content, "Needle")) {
			t.Fatalf("a.txt match = %+v, want line 1", m)
		}
	}
}

func TestBackendGrepRipgrepHonorsGitignore(t *testing.T) {
	if _, err := exec.LookPath("rg"); err != nil {
		t.Skip("ripgrep is not available on this host")
	}
	backend, workspace, runID := newFilesystemTestBackend(t)
	writeGrepFixtures(t, workspace)
	res, err := backend.Grep(tools.WithRunID(context.Background(), runID), tools.GrepRequest{Pattern: "[Nn]eedle"})
	if err != nil {
		t.Fatalf("grep: %v", err)
	}
	paths := matchPaths(res.Matches)
	for _, want := range []string{"a.txt", "nested/b.go", "nested/deep/d.go"} {
		if !containsString(paths, want) {
			t.Fatalf("rg matches = %v, want %s", paths, want)
		}
	}
	for _, absent := range []string{"ignored/secret.txt", "node_modules/pkg.js"} {
		if containsString(paths, absent) {
			t.Fatalf("rg ignored .gitignore for %s: %v", absent, paths)
		}
	}
}

func TestBackendGrepIncludeFilter(t *testing.T) {
	backend, workspace, runID := newFilesystemTestBackend(t)
	writeGrepFixtures(t, workspace)
	ctx := tools.WithRunID(context.Background(), runID)

	backend.rgPath = ""
	fallback, err := backend.Grep(ctx, tools.GrepRequest{Pattern: "needle", Include: "*.go"})
	if err != nil {
		t.Fatalf("fallback grep: %v", err)
	}
	for _, p := range matchPaths(fallback.Matches) {
		if !strings.HasSuffix(p, ".go") {
			t.Fatalf("fallback include filter leaked %s: %v", p, fallback.Matches)
		}
	}
	if !containsString(matchPaths(fallback.Matches), "nested/deep/d.go") {
		t.Fatalf("fallback include matches = %v, want nested/deep/d.go", fallback.Matches)
	}

	if _, err := exec.LookPath("rg"); err != nil {
		return
	}
	backend.rgPath, _ = exec.LookPath("rg")
	rg, err := backend.Grep(ctx, tools.GrepRequest{Pattern: "needle", Include: "*.go"})
	if err != nil {
		t.Fatalf("rg grep: %v", err)
	}
	for _, p := range matchPaths(rg.Matches) {
		if !strings.HasSuffix(p, ".go") {
			t.Fatalf("rg include filter leaked %s: %v", p, rg.Matches)
		}
	}
}

func TestBackendGrepRejectsInvalidPattern(t *testing.T) {
	backend, _, runID := newFilesystemTestBackend(t)
	backend.rgPath = ""
	_, err := backend.Grep(tools.WithRunID(context.Background(), runID), tools.GrepRequest{Pattern: "("})
	if err == nil || !strings.Contains(err.Error(), "invalid grep pattern") {
		t.Fatalf("invalid pattern error = %v, want compile failure", err)
	}
}

func TestBackendGrepPathScoped(t *testing.T) {
	backend, workspace, runID := newFilesystemTestBackend(t)
	writeGrepFixtures(t, workspace)
	backend.rgPath = ""
	res, err := backend.Grep(tools.WithRunID(context.Background(), runID), tools.GrepRequest{Pattern: "[Nn]eedle", Path: "nested"})
	if err != nil {
		t.Fatalf("grep: %v", err)
	}
	for _, p := range matchPaths(res.Matches) {
		if !strings.HasPrefix(p, "nested/") {
			t.Fatalf("scoped grep leaked %s: %v", p, res.Matches)
		}
	}
	if len(res.Matches) != 2 {
		t.Fatalf("scoped matches = %v, want 2", res.Matches)
	}
}

func TestBackendGlobRecursiveNewestFirst(t *testing.T) {
	backend, workspace, runID := newFilesystemTestBackend(t)
	writeGrepFixtures(t, workspace)
	old := time.Now().Add(-2 * time.Hour)
	newer := time.Now().Add(-1 * time.Hour)
	if err := os.Chtimes(filepath.Join(workspace.Path, "nested", "b.go"), old, old); err != nil {
		t.Fatalf("chtimes b.go: %v", err)
	}
	if err := os.Chtimes(filepath.Join(workspace.Path, "nested", "deep", "d.go"), newer, newer); err != nil {
		t.Fatalf("chtimes d.go: %v", err)
	}
	ctx := tools.WithRunID(context.Background(), runID)

	res, err := backend.Glob(ctx, tools.GlobRequest{Pattern: "**/*.go"})
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	// doublestar recursion reaches nested/deep, and results are newest first.
	if len(res.Files) != 2 || res.Files[0].Path != "nested/deep/d.go" || res.Files[1].Path != "nested/b.go" {
		t.Fatalf("glob files = %+v, want d.go before b.go", res.Files)
	}

	txt, err := backend.Glob(ctx, tools.GlobRequest{Pattern: "*.txt"})
	if err != nil {
		t.Fatalf("glob txt: %v", err)
	}
	// Glob prunes known-ignored directories but has no .gitignore awareness,
	// so ignored/secret.txt legitimately matches too.
	if !containsString(matchPathsFromGlob(txt.Files), "a.txt") {
		t.Fatalf("glob txt = %+v, want a.txt", txt.Files)
	}
	if containsString(matchPathsFromGlob(txt.Files), "node_modules/pkg.js") {
		t.Fatalf("glob txt descended into node_modules: %+v", txt.Files)
	}

	all, err := backend.Glob(ctx, tools.GlobRequest{Pattern: "**/*"})
	if err != nil {
		t.Fatalf("glob all: %v", err)
	}
	for _, f := range all.Files {
		if strings.HasPrefix(f.Path, "node_modules/") {
			t.Fatalf("glob descended into node_modules: %v", all.Files)
		}
	}
	for _, f := range all.Files {
		if f.Size <= 0 || f.ModifiedAt == "" {
			t.Fatalf("glob file metadata missing: %+v", f)
		}
	}
}

func TestBackendGlobRejectsEscapePatterns(t *testing.T) {
	backend, _, runID := newFilesystemTestBackend(t)
	ctx := tools.WithRunID(context.Background(), runID)
	for _, pattern := range []string{"..", "../outside/*"} {
		if _, err := backend.Glob(ctx, tools.GlobRequest{Pattern: pattern}); err == nil || !strings.Contains(err.Error(), "inside the workspace") {
			t.Fatalf("pattern %q error = %v, want workspace escape rejection", pattern, err)
		}
	}
}

// TestServiceGrepToolEndToEnd drives the full stack over sqlite with the
// session pinned to the strictest 'ask' policy: readonly grep and glob must
// run without any approval interrupt and land their results in the journal.
// The run workspace starts empty, so the scripted pattern matches nothing —
// the assertions are on the readonly path and journal plumbing.
func TestServiceGrepToolEndToEnd(t *testing.T) {
	ctx := context.Background()

	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "grep-e2e.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })

	filesBackend := newServiceSearchBackend(t)
	ts, err := tools.NewRegistry(tools.NewGrep(filesBackend), tools.NewGlob(filesBackend)).Resolve([]string{tools.GrepName, tools.GlobName})
	if err != nil {
		t.Fatalf("resolve grep: %v", err)
	}
	checkpoints, err := NewVersionedCheckpointStore(backend.Blobs(), "test-engine")
	if err != nil {
		t.Fatalf("checkpoint store: %v", err)
	}
	eng, err := NewEngine(ctx, NewScriptedModel(
		schema.AssistantMessage("", []schema.ToolCall{{
			ID:       "call-grep-e2e",
			Function: schema.FunctionCall{Name: tools.GrepName, Arguments: `{"pattern":"vivy_nothing_here"}`},
		}}),
		schema.AssistantMessage("", []schema.ToolCall{{
			ID:       "call-glob-e2e",
			Function: schema.FunctionCall{Name: tools.GlobName, Arguments: `{"pattern":"**/*.txt"}`},
		}}),
		schema.AssistantMessage("Done: grep and glob ran readonly.", nil),
	), ts, EngineConfig{StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10, Checkpoints: checkpoints})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	svc := NewService(eng, "scripted", "scripted-v0", ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Notes: backend, Approvals: backend,
		Questions:          backend,
		Sessions:           backend,
		ApprovalExpiration: 5 * time.Minute, Sink: newTestSink(),
	})
	if err := backend.CreateSession(ctx, domain.Session{ID: "sess-grep-e2e", Title: "grep e2e", CreatedAt: time.Now().UnixMilli()}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	// 'ask' denies every effectful tool; readonly tools must sail through.
	if err := backend.UpdateSandboxPolicy(ctx, "sess-grep-e2e", domain.SandboxModeWorkspaceWrite, domain.ApprovalPolicyAsk); err != nil {
		t.Fatalf("set session approval policy: %v", err)
	}

	runID, err := svc.Run(ctx, "sess-grep-e2e", "search the workspace")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunCompleted)

	events := replayAll(t, backend, runID)
	if i := indexOfType(events, domain.EventToolApprovalRequired); i >= 0 {
		t.Fatalf("readonly grep raised an approval interrupt under 'ask' at %d", i)
	}
	if indexOfType(events, domain.EventToolRequested) < 0 || indexOfType(events, domain.EventToolFinished) < 0 {
		t.Fatalf("journal missing tool lifecycle events: %v", events)
	}
	var blob bytes.Buffer
	for _, ev := range events {
		blob.Write(ev.Payload)
		blob.WriteString("\n")
	}
	journal := blob.String()
	// Tool result strings are JSON-escaped inside event payloads, so the
	// assertions avoid quoting the result bodies: two untrusted-output
	// markers prove both readonly calls returned results to the journal.
	for _, want := range []string{`"pattern":"vivy_nothing_here"`, `"pattern":"**/*.txt"`} {
		if !strings.Contains(journal, want) {
			t.Fatalf("journal lost %s; events: %s", want, journal)
		}
	}
	if got := strings.Count(journal, "UNTRUSTED TOOL OUTPUT"); got != 2 {
		t.Fatalf("journal carried %d tool results, want 2; events: %s", got, journal)
	}
}
