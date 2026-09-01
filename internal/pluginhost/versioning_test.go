package pluginhost

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/tools"
	"agent-vivy/sdk/plugin"
)

type fakeRecorder struct {
	mutations []fakeMutation
	access    map[string]int64
}

type fakeMutation struct {
	session domain.SessionID
	run     domain.RunID
	path    string
	old     []byte
	new     []byte
}

func (f *fakeRecorder) RecordMutation(ctx context.Context, sessionID domain.SessionID, runID domain.RunID, path string, oldContent, newContent []byte) {
	f.mutations = append(f.mutations, fakeMutation{session: sessionID, run: runID, path: path, old: oldContent, new: newContent})
}

func (f *fakeRecorder) TrackAccess(ctx context.Context, sessionID domain.SessionID, path string, at int64) {
	f.access[path] = at
}

func (f *fakeRecorder) LastAccess(ctx context.Context, sessionID domain.SessionID, path string) (int64, bool, error) {
	at, ok := f.access[path]
	return at, ok, nil
}

type writeTool struct{ body string }

func (writeTool) Name() string            { return "write_tool" }
func (writeTool) Effect() plugin.Effect   { return plugin.EffectWrite }
func (writeTool) Schema() json.RawMessage { return json.RawMessage(`{}`) }

func (t writeTool) Run(ctx context.Context, env plugin.Env, args json.RawMessage) (string, error) {
	w, err := env.OpenWrite("docs/note.txt")
	if err != nil {
		return "", err
	}
	if _, err := io.WriteString(w, t.body); err != nil {
		_ = w.Close()
		return "", err
	}
	return "", w.Close()
}

type writePlugin struct{ tools []plugin.Tool }

func (writePlugin) Name() string           { return "fake" }
func (writePlugin) Seam() plugin.Seam      { return plugin.SeamTool }
func (writePlugin) Grants() []plugin.Grant { return []plugin.Grant{plugin.GrantFSWrite} }
func (p writePlugin) Tools() []plugin.Tool { return p.tools }

func runWriteTool(t *testing.T, ctx context.Context, root string, rec *fakeRecorder, body string) {
	t.Helper()
	var recorder tools.FileVersionRecorder
	if rec != nil {
		recorder = rec
	}
	toolset := Adapt([]plugin.Plugin{writePlugin{tools: []plugin.Tool{writeTool{body: body}}}}, func(context.Context) (string, error) {
		return root, nil
	}, recorder)
	if len(toolset) != 1 {
		t.Fatalf("Adapt produced %d tools, want 1", len(toolset))
	}
	if _, err := toolset[0].InvokableRun(ctx, nil); err != nil {
		t.Fatalf("tool run: %v", err)
	}
}

func TestOpenWriteRecordsNewFileMutation(t *testing.T) {
	root := t.TempDir()
	rec := &fakeRecorder{access: map[string]int64{}}
	ctx := tools.WithSessionID(tools.WithRunID(context.Background(), domain.RunID("run_x")), domain.SessionID("sess_x"))

	runWriteTool(t, ctx, root, rec, "written body")

	data, err := os.ReadFile(filepath.Join(root, "docs", "note.txt"))
	if err != nil || string(data) != "written body" {
		t.Fatalf("file content = %q, %v", data, err)
	}
	if len(rec.mutations) != 1 {
		t.Fatalf("mutations = %d, want 1", len(rec.mutations))
	}
	m := rec.mutations[0]
	if m.session != "sess_x" || m.run != "run_x" || m.path != "docs/note.txt" {
		t.Fatalf("unexpected mutation identity: %+v", m)
	}
	if len(m.old) != 0 || string(m.new) != "written body" {
		t.Fatalf("unexpected mutation content: old=%q new=%q", m.old, m.new)
	}
	if _, ok := rec.access["docs/note.txt"]; !ok {
		t.Fatal("access marker missing after write")
	}
}

func TestOpenWriteRecordsOverwriteBaseline(t *testing.T) {
	root := t.TempDir()
	rec := &fakeRecorder{access: map[string]int64{}}
	ctx := tools.WithSessionID(context.Background(), domain.SessionID("sess_x"))
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "note.txt"), []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}

	runWriteTool(t, ctx, root, rec, "replaced")

	if len(rec.mutations) != 1 {
		t.Fatalf("mutations = %d, want 1", len(rec.mutations))
	}
	m := rec.mutations[0]
	if string(m.old) != "existing" || string(m.new) != "replaced" {
		t.Fatalf("unexpected mutation content: old=%q new=%q", m.old, m.new)
	}
}

func TestOpenWriteOversizePayloadPassesThroughUnrecorded(t *testing.T) {
	root := t.TempDir()
	rec := &fakeRecorder{access: map[string]int64{}}
	ctx := tools.WithSessionID(context.Background(), domain.SessionID("sess_x"))
	body := strings.Repeat("x", storage.FileVersionMaxBytes+17)

	runWriteTool(t, ctx, root, rec, body)

	data, err := os.ReadFile(filepath.Join(root, "docs", "note.txt"))
	if err != nil || len(data) != len(body) {
		t.Fatalf("oversize payload lost: len=%d err=%v", len(data), err)
	}
	if len(rec.mutations) != 0 {
		t.Fatalf("oversize payload recorded %d mutations, want 0", len(rec.mutations))
	}
	if _, ok := rec.access["docs/note.txt"]; !ok {
		t.Fatal("access marker missing after oversize write")
	}
}

func TestOpenWriteOversizeOldFileSkipsChainEntry(t *testing.T) {
	root := t.TempDir()
	rec := &fakeRecorder{access: map[string]int64{}}
	ctx := tools.WithSessionID(context.Background(), domain.SessionID("sess_x"))
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "note.txt"), []byte(strings.Repeat("y", storage.FileVersionMaxBytes+5)), 0o600); err != nil {
		t.Fatal(err)
	}

	runWriteTool(t, ctx, root, rec, "small")

	data, err := os.ReadFile(filepath.Join(root, "docs", "note.txt"))
	if err != nil || string(data) != "small" {
		t.Fatalf("file content = %q, %v", data, err)
	}
	if len(rec.mutations) != 0 {
		t.Fatalf("truncated baseline recorded, want 0 mutations")
	}
	if _, ok := rec.access["docs/note.txt"]; !ok {
		t.Fatal("access marker missing after skip write")
	}
}

func TestOpenWriteNoSessionSkipsRecording(t *testing.T) {
	root := t.TempDir()
	rec := &fakeRecorder{access: map[string]int64{}}

	runWriteTool(t, context.Background(), root, rec, "no session")

	data, err := os.ReadFile(filepath.Join(root, "docs", "note.txt"))
	if err != nil || string(data) != "no session" {
		t.Fatalf("file content = %q, %v", data, err)
	}
	if len(rec.mutations) != 0 || len(rec.access) != 0 {
		t.Fatalf("recording without session: %+v %v", rec.mutations, rec.access)
	}
}

func TestOpenWriteNilRecorderStillWrites(t *testing.T) {
	root := t.TempDir()
	ctx := tools.WithSessionID(context.Background(), domain.SessionID("sess_x"))

	runWriteTool(t, ctx, root, nil, "plain")

	data, err := os.ReadFile(filepath.Join(root, "docs", "note.txt"))
	if err != nil || string(data) != "plain" {
		t.Fatalf("file content = %q, %v", data, err)
	}
}

// TestOpenWriteBuffersUntilClose pins the kernel-side capture contract:
// content only reaches disk (and the chain) on a successful Close, and a
// writer that is never closed leaves the old content and no mutation.
func TestOpenWriteBuffersUntilClose(t *testing.T) {
	root := t.TempDir()
	rec := &fakeRecorder{access: map[string]int64{}}
	ctx := tools.WithSessionID(context.Background(), domain.SessionID("sess_x"))
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "note.txt"), []byte("old body"), 0o600); err != nil {
		t.Fatal(err)
	}
	env := hostedEnv{
		plugin:   writePlugin{},
		lookup:   func(context.Context) (string, error) { return root, nil },
		ctx:      ctx,
		recorder: rec,
	}

	w, err := env.OpenWrite("docs/note.txt")
	if err != nil {
		t.Fatalf("open write: %v", err)
	}
	t.Cleanup(func() { _ = w.Close() })
	if _, err := io.WriteString(w, "new body"); err != nil {
		t.Fatalf("write: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "docs", "note.txt"))
	if err != nil || string(data) != "old body" {
		t.Fatalf("content leaked before Close: %q, %v", data, err)
	}
	if len(rec.mutations) != 0 {
		t.Fatalf("mutation recorded before Close: %+v", rec.mutations)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	data, err = os.ReadFile(filepath.Join(root, "docs", "note.txt"))
	if err != nil || string(data) != "new body" {
		t.Fatalf("content after Close = %q, %v", data, err)
	}
	if len(rec.mutations) != 1 {
		t.Fatalf("mutations = %d, want 1", len(rec.mutations))
	}
	if m := rec.mutations[0]; string(m.old) != "old body" || string(m.new) != "new body" {
		t.Fatalf("unexpected mutation content: old=%q new=%q", m.old, m.new)
	}
}
