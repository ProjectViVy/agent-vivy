package runtime

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/tools"
)

func TestD09ScopedReadIdentityStaysPrivateOnStorageFailure(t *testing.T) {
	root := t.TempDir()
	store, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	recorder := NewFileVersionRecorder(store, slog.New(slog.NewTextHandler(&logs, nil)))
	_, _, err = recorder.LastAccess(context.Background(), "private-session", root, "public.txt")
	if err == nil || strings.Contains(err.Error(), root) {
		t.Fatalf("private read identity escaped storage failure: %v", err)
	}
	recorder.TrackAccess(context.Background(), "private-session", root, "public.txt", domain.FileReadState{Exists: true})
	if strings.Contains(logs.String(), root) {
		t.Fatal("private read identity escaped through error logging")
	}
}

func TestD09ReadMarkersFollowActualWorkspaceAndKeepHistoryRelative(t *testing.T) {
	backend, first, firstRun := newFilesystemTestBackend(t)
	store, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	backend.SetFileVersionRecorder(NewFileVersionRecorder(store, nil))
	ctx := sessionCtx("same-session")
	request := tools.FileWriteRequest{Path: "same.txt", Content: "first"}
	if _, err := backend.WriteFile(ctx, firstRun, request); err != nil {
		t.Fatal(err)
	}
	secondRun := domain.RunID("second-private-root")
	request.Content = "second"
	if _, err := backend.WriteFile(ctx, secondRun, request); err != nil {
		t.Fatalf("different workspace inherited read authority: %v", err)
	}
	if err := os.WriteFile(filepath.Join(first.Path, "same.txt"), []byte("other"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := backend.WriteFile(ctx, firstRun, request); err == nil {
		t.Fatal("other workspace erased the first workspace's stale-read guard")
	}
	if _, err := backend.ReadFile(ctx, firstRun, tools.FileReadRequest{Path: "same.txt"}); err != nil {
		t.Fatal(err)
	}
	if _, err := backend.WriteFile(ctx, firstRun, request); err != nil {
		t.Fatalf("fresh exact read did not repair the guard: %v", err)
	}
	versions, err := store.ListModifiedFiles(ctx, "same-session", 20)
	if err != nil || len(versions) != 1 || versions[0].Path != "same.txt" {
		t.Fatalf("relative history changed: %+v %v", versions, err)
	}
	backend.manager, err = NewLocalWorkspaceManager(first.Path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(first.Path, "same.txt"), []byte("other!"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := backend.WriteFile(ctx, domain.RunID("shared-local-root"), request); err == nil {
		t.Fatal("different Run bypassed the same workspace's exact read guard")
	}
}

func TestD09UnboundReadMarkerRequiresActualPresentOrAbsentRead(t *testing.T) {
	backend, workspace, runID := newFilesystemTestBackend(t)
	confined, err := NewSandboxManager(domain.SandboxModeWorkspaceWrite, workspace.Path, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	backend.sandbox = confined
	store, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	backend.SetFileVersionRecorder(NewFileVersionRecorder(store, nil))
	ctx := sessionCtx("converted-session")
	// Even retained exact bytes do not prove which workspace owned this path.
	if err := store.TrackFileAccess(ctx, "converted-session", "present.txt", domain.FileReadState{Exists: true, Content: []byte("same")}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace.Path, "present.txt"), []byte("same"), 0600); err != nil {
		t.Fatal(err)
	}
	present := tools.FileWriteRequest{Path: "present.txt", Content: "new"}
	if _, err := backend.WriteFile(ctx, runID, present); err == nil {
		t.Fatal("unbound exact bytes authorized another workspace")
	}
	if _, err := backend.ReadFile(ctx, runID, tools.FileReadRequest{Path: "present.txt"}); err != nil {
		t.Fatal(err)
	}
	if _, err := backend.WriteFile(ctx, runID, present); err != nil {
		t.Fatalf("present-file read did not restore authority: %v", err)
	}
	if err := store.TrackFileAccess(ctx, "converted-session", "nested/missing.txt", domain.FileReadState{Exists: true, Content: []byte("old-root")}); err != nil {
		t.Fatal(err)
	}
	request := tools.FileWriteRequest{Path: "nested/missing.txt", Content: "new-root", CreateParents: true}
	if _, err := backend.WriteFile(ctx, runID, request); err == nil {
		t.Fatal("unbound historical read authorized creation")
	}
	if _, err := backend.ReadFile(ctx, runID, tools.FileReadRequest{Path: "nested/missing.txt"}); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("actual absence read: %v", err)
	}
	if _, err := backend.WriteFile(ctx, runID, request); err != nil {
		t.Fatalf("observed absence did not allow correction: %v", err)
	}
}
