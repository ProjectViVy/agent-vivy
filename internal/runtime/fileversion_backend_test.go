package runtime

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
	lastAccess func()
	mutations  []fakeMutation
	access     map[domain.SessionID]map[string]*domain.FileReadState
}

func newFakeFileVersionRecorder() *fakeFileVersionRecorder {
	return &fakeFileVersionRecorder{access: make(map[domain.SessionID]map[string]*domain.FileReadState)}
}

func (f *fakeFileVersionRecorder) RecordMutation(_ context.Context, sessionID domain.SessionID, runID domain.RunID, path string, oldContent, newContent []byte) {
	f.mutations = append(f.mutations, fakeMutation{
		session: sessionID, run: runID, path: path, old: string(oldContent), new: string(newContent),
	})
}

func (f *fakeFileVersionRecorder) TrackAccess(_ context.Context, sessionID domain.SessionID, path string, state domain.FileReadState) {
	if f.access[sessionID] == nil {
		f.access[sessionID] = make(map[string]*domain.FileReadState)
	}
	f.access[sessionID][path] = &domain.FileReadState{Exists: state.Exists, Content: append([]byte{}, state.Content...)}
}

func (f *fakeFileVersionRecorder) LastAccess(_ context.Context, sessionID domain.SessionID, path string) (*domain.FileReadState, bool, error) {
	if f.lastAccess != nil {
		f.lastAccess()
	}
	pathAccess, ok := f.access[sessionID]
	if !ok {
		return nil, false, nil
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
	// The previous read admitted different actual bytes.
	recorder.TrackAccess(ctx, "sess-fv", "a.txt", domain.FileReadState{Exists: true, Content: []byte("older")})

	if _, err := backend.WriteFile(ctx, runID, tools.FileWriteRequest{Path: "a.txt", Content: "after"}); err == nil || !strings.Contains(err.Error(), "changed on disk after the last read") {
		t.Fatalf("stale write = %v, want stale-read rejection", err)
	}

	// Reading the file moves the marker forward; the edit then succeeds.
	if _, err := backend.ReadFile(ctx, runID, tools.FileReadRequest{Path: "a.txt"}); err != nil {
		t.Fatalf("read: %v", err)
	}
	at, ok, _ := recorder.LastAccess(ctx, "sess-fv", "a.txt")
	if !ok || at == nil || !at.Exists || string(at.Content) != "before" {
		t.Fatalf("state after read = (%+v, %v), want refreshed", at, ok)
	}
	if _, err := backend.WriteFile(ctx, runID, tools.FileWriteRequest{Path: "a.txt", Content: "after"}); err != nil {
		t.Fatalf("write after re-read = %v, want success", err)
	}
}

func TestFileVersionNeverTrackedFailsOpen(t *testing.T) {
	backend, recorder, _, runID := newFilesystemBackendWithRecorder(t)
	// The marker exists for another path only; this write must pass.
	recorder.TrackAccess(sessionCtx("sess-fv"), "sess-fv", "other.txt", domain.FileReadState{Exists: true, Content: []byte("other")})

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

func TestFileVersionReadDetectsRestoredTimeAndDeletion(t *testing.T) {
	for _, remove := range []bool{false, true} {
		t.Run(map[bool]string{false: "same-size-restored-time", true: "empty-to-missing"}[remove], func(t *testing.T) {
			backend, _, workspace, runID := newFilesystemBackendWithRecorder(t)
			ctx := sessionCtx("exact")
			target := filepath.Join(workspace.Path, "read.txt")
			content := []byte("AAA")
			if remove {
				content = []byte{}
			}
			if err := os.WriteFile(target, content, 0600); err != nil {
				t.Fatal(err)
			}
			info, err := os.Stat(target)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := backend.ReadFile(ctx, runID, tools.FileReadRequest{Path: "read.txt"}); err != nil {
				t.Fatal(err)
			}
			if remove {
				if err := os.Remove(target); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.WriteFile(target, []byte("BBB"), 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Chtimes(target, info.ModTime(), info.ModTime()); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := backend.WriteFile(ctx, runID, tools.FileWriteRequest{Path: "read.txt", Content: "NEW"}); err == nil {
				t.Fatal("changed read bytes accepted")
			}
		})
	}
}

func TestFileVersionUnknownReadRequiresPresentation(t *testing.T) {
	backend, recorder, workspace, runID := newFilesystemBackendWithRecorder(t)
	ctx := sessionCtx("unknown")
	recorder.access["unknown"] = map[string]*domain.FileReadState{"empty.txt": nil}
	target := filepath.Join(workspace.Path, "empty.txt")
	if err := os.WriteFile(target, []byte{}, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := backend.WriteFile(ctx, runID, tools.FileWriteRequest{Path: "empty.txt", Content: "new"}); err == nil {
		t.Fatal("historical timestamp authorized write")
	}
	if _, err := backend.ReadFile(ctx, runID, tools.FileReadRequest{Path: "empty.txt"}); err != nil {
		t.Fatal(err)
	}
	if _, err := backend.WriteFile(ctx, runID, tools.FileWriteRequest{Path: "empty.txt", Content: "new"}); err != nil {
		t.Fatal(err)
	}
}
func TestD09FinalWriteRacePreservesConcurrentBytes(t *testing.T) {
	backend, recorder, workspace, runID := newFilesystemBackendWithRecorder(t)
	ctx := sessionCtx("race")
	target := filepath.Join(workspace.Path, "race.txt")
	if err := os.WriteFile(target, []byte("AAA"), 0600); err != nil {
		t.Fatal(err)
	}
	recorder.lastAccess = func() {
		if err := os.WriteFile(target, []byte("BBB"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := backend.WriteFile(ctx, runID, tools.FileWriteRequest{Path: "race.txt", Content: "NEW"}); err == nil {
		t.Fatal("concurrent bytes overwritten")
	}
	got, err := os.ReadFile(target)
	if err != nil || string(got) != "BBB" {
		t.Fatalf("concurrent state changed: %s %v", got, err)
	}
	leftovers, err := filepath.Glob(filepath.Join(workspace.Path, ".vivy-write-*"))
	if err != nil || len(leftovers) != 0 {
		t.Fatalf("scratch leaked: %v %v", leftovers, err)
	}
}

func TestD09NoOpWriteStillRechecksConcurrentBytes(t *testing.T) {
	backend, recorder, workspace, runID := newFilesystemBackendWithRecorder(t)
	target := filepath.Join(workspace.Path, "race.txt")
	if err := os.WriteFile(target, []byte("AAA"), 0600); err != nil {
		t.Fatal(err)
	}
	recorder.lastAccess = func() {
		if err := os.WriteFile(target, []byte("BBB"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := backend.WriteFile(sessionCtx("race"), runID, tools.FileWriteRequest{Path: "race.txt", Content: "AAA"}); err == nil {
		t.Fatal("stale no-op reported success")
	}
	got, err := os.ReadFile(target)
	if err != nil || string(got) != "BBB" {
		t.Fatalf("concurrent state changed: %s %v", got, err)
	}
}
