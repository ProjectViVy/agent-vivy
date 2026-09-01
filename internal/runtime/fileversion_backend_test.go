package runtime

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tools"
)

type fakeMutation struct {
	session domain.SessionID
	run     domain.RunID
	path    string
	old     string
	new     string
}

type fakeFileVersionRecorder struct {
	mutations []fakeMutation
	access    map[domain.SessionID]map[string]int64
}

func newFakeFileVersionRecorder() *fakeFileVersionRecorder {
	return &fakeFileVersionRecorder{access: make(map[domain.SessionID]map[string]int64)}
}

func (f *fakeFileVersionRecorder) RecordMutation(_ context.Context, sessionID domain.SessionID, runID domain.RunID, path string, oldContent, newContent []byte) {
	f.mutations = append(f.mutations, fakeMutation{
		session: sessionID, run: runID, path: path, old: string(oldContent), new: string(newContent),
	})
}

func (f *fakeFileVersionRecorder) TrackAccess(_ context.Context, sessionID domain.SessionID, path string, at int64) {
	if f.access[sessionID] == nil {
		f.access[sessionID] = make(map[string]int64)
	}
	f.access[sessionID][path] = at
}

func (f *fakeFileVersionRecorder) LastAccess(_ context.Context, sessionID domain.SessionID, path string) (int64, bool, error) {
	pathAccess, ok := f.access[sessionID]
	if !ok {
		return 0, false, nil
	}
	at, ok := pathAccess[path]
	return at, ok, nil
}

func newFilesystemBackendWithRecorder(t *testing.T) (*EinoFilesystemBackend, *fakeFileVersionRecorder, Workspace, domain.RunID) {
	t.Helper()
	backend, workspace, runID := newFilesystemTestBackend(t)
	recorder := newFakeFileVersionRecorder()
	backend.SetFileVersionRecorder(recorder)
	return backend, recorder, workspace, runID
}

func sessionCtx(sessionID domain.SessionID) context.Context {
	return tools.WithSessionID(context.Background(), sessionID)
}

func TestFileVersionRecordingOnWriteReadPatch(t *testing.T) {
	backend, recorder, _, runID := newFilesystemBackendWithRecorder(t)
	ctx := sessionCtx("sess-fv")

	if _, err := backend.WriteFile(ctx, runID, tools.FileWriteRequest{Path: "a.txt", Content: "one", CreateParents: true}); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := backend.ReadFile(ctx, runID, tools.FileReadRequest{Path: "a.txt"}); err != nil {
		t.Fatalf("read: %v", err)
	}
	if _, err := backend.PatchFile(ctx, runID, tools.FilePatchRequest{Path: "a.txt", OldString: "one", NewString: "ONE"}); err != nil {
		t.Fatalf("patch: %v", err)
	}

	want := []fakeMutation{
		{session: "sess-fv", run: runID, path: "a.txt", old: "", new: "one"},
		{session: "sess-fv", run: runID, path: "a.txt", old: "one", new: "ONE"},
	}
	if len(recorder.mutations) != len(want) {
		t.Fatalf("mutations = %+v, want %d entries", recorder.mutations, len(want))
	}
	for i, m := range want {
		if recorder.mutations[i] != m {
			t.Fatalf("mutation %d = %+v, want %+v", i, recorder.mutations[i], m)
		}
	}
	if _, ok, _ := recorder.LastAccess(ctx, "sess-fv", "a.txt"); !ok {
		t.Fatalf("a.txt was never tracked for access")
	}
}

func TestFileVersionStaleReadGuard(t *testing.T) {
	backend, recorder, workspace, runID := newFilesystemBackendWithRecorder(t)
	ctx := sessionCtx("sess-fv")

	target := filepath.Join(workspace.Path, "a.txt")
	if err := os.WriteFile(target, []byte("before"), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	// A marker one second older than the file's mtime makes the next edit
	// look stale without depending on clock resolution.
	recorder.TrackAccess(ctx, "sess-fv", "a.txt", info.ModTime().UnixMilli()-1000)

	if _, err := backend.WriteFile(ctx, runID, tools.FileWriteRequest{Path: "a.txt", Content: "after"}); err == nil || !strings.Contains(err.Error(), "changed on disk after the last read") {
		t.Fatalf("stale write = %v, want stale-read rejection", err)
	}

	// Reading the file moves the marker forward; the edit then succeeds.
	if _, err := backend.ReadFile(ctx, runID, tools.FileReadRequest{Path: "a.txt"}); err != nil {
		t.Fatalf("read: %v", err)
	}
	at, ok, _ := recorder.LastAccess(ctx, "sess-fv", "a.txt")
	if !ok || at <= info.ModTime().UnixMilli()-1000 {
		t.Fatalf("marker after read = (%d, %v), want refreshed", at, ok)
	}
	if _, err := backend.WriteFile(ctx, runID, tools.FileWriteRequest{Path: "a.txt", Content: "after"}); err != nil {
		t.Fatalf("write after re-read = %v, want success", err)
	}
}

func TestFileVersionNeverTrackedFailsOpen(t *testing.T) {
	backend, recorder, _, runID := newFilesystemBackendWithRecorder(t)
	// The marker exists for another path only; this write must pass.
	recorder.TrackAccess(sessionCtx("sess-fv"), "sess-fv", "other.txt", time.Now().UnixMilli())

	if _, err := backend.WriteFile(sessionCtx("sess-fv"), runID, tools.FileWriteRequest{Path: "fresh.txt", Content: "x", CreateParents: true}); err != nil {
		t.Fatalf("write to never-tracked path = %v, want success", err)
	}
}

func TestFileVersionNoSessionNoRecording(t *testing.T) {
	backend, recorder, _, runID := newFilesystemBackendWithRecorder(t)
	ctx := context.Background()

	if _, err := backend.WriteFile(ctx, runID, tools.FileWriteRequest{Path: "a.txt", Content: "one", CreateParents: true}); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := backend.ReadFile(ctx, runID, tools.FileReadRequest{Path: "a.txt"}); err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(recorder.mutations) != 0 || len(recorder.access) != 0 {
		t.Fatalf("recorder touched outside a session run: %+v", recorder)
	}
}
