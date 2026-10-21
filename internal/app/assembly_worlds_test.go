package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tools"
	"agent-vivy/plugins/hello-fs"
	"agent-vivy/sdk/module"
	"agent-vivy/sdk/port/toolworld"
)

func TestGeneratedToolWorldInvokesThroughGrantedHost(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "note.txt"), []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	provider := hellofs.NewProvider()
	staged, err := bindToolWorlds(
		context.Background(),
		[]toolworld.Provider{provider},
		map[string][]module.GrantBinding{"vivy.hello-fs": {{Name: module.GrantFSRead}}},
		func(context.Context) (string, error) { return root, nil },
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(staged) != 1 {
		t.Fatalf("staged ToolWorld providers = %d, want 1", len(staged))
	}
	registry, err := bindGeneratedTools(nil, tools.NewRegistry(staged...), nil)
	if err != nil {
		t.Fatal(err)
	}
	worldTool, ok := registry.Lookup("hello_stat")
	if !ok {
		t.Fatal("ToolHost did not expose hello_stat")
	}
	got, err := worldTool.InvokableRun(context.Background(), []byte(`{"path":"note.txt"}`))
	if err != nil {
		t.Fatal(err)
	}
	if got != "note.txt 5" {
		t.Fatalf("result = %q, want note.txt 5", got)
	}
}

func TestGeneratedToolWorldEnforcesGrantConstraints(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "allowed"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "allowed", "note.txt"), []byte("ok"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "outside.txt"), []byte("no"), 0o600); err != nil {
		t.Fatal(err)
	}
	host := generatedWorldHost{
		moduleID: "fixture/world",
		grants: map[module.Grant][]string{
			module.GrantFSRead:    {"allowed"},
			module.GrantProcSpawn: {"gopls"},
		},
		lookup: func(context.Context) (string, error) { return root, nil },
		ctx:    context.Background(),
	}
	reader, err := host.OpenRead("allowed/note.txt")
	if err != nil {
		t.Fatal(err)
	}
	_ = reader.Close()
	if _, err := host.OpenRead("outside.txt"); !errors.Is(err, toolworld.ErrDenied) {
		t.Fatalf("outside root error = %v, want ErrDenied", err)
	}
	if _, err := host.Spawn(context.Background(), toolworld.SpawnSpec{Command: "sh"}); !errors.Is(err, toolworld.ErrDenied) {
		t.Fatalf("unapproved command error = %v, want ErrDenied", err)
	}
}

type observingDiagnostics struct{ called bool }

func (observer *observingDiagnostics) ObserveWrite(_ context.Context, host toolworld.Host, _ []string) []string {
	observer.called = true
	reader, err := host.OpenRead("allowed/note.txt")
	if err != nil {
		return []string{err.Error()}
	}
	_ = reader.Close()
	return []string{"diagnostic"}
}

func TestGeneratedWriteDiagnosticsUsesGovernedWorldHost(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "allowed"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "allowed", "note.txt"), []byte("ok"), 0o600); err != nil {
		t.Fatal(err)
	}
	observer := &observingDiagnostics{}
	source := generatedWriteDiagnostics{
		observers: []toolworld.DiagnosticObserver{observer},
		worldIDs:  []string{"fixture.world"},
		grants: map[string][]module.GrantBinding{
			"fixture.world": {{Name: module.GrantFSRead, Constraints: map[string][]string{"roots": {"allowed"}}}},
		},
		lookup: func(context.Context) (string, error) { return root, nil },
	}
	got := source.WriteDiagnostics(context.Background(), []string{"allowed/note.txt"})
	if !observer.called || len(got) != 1 || got[0] != "diagnostic" {
		t.Fatalf("called=%v diagnostics=%v", observer.called, got)
	}
}

type recordingWorldVersions struct {
	mutations int
	tracked   int
	path      string
}

func (recorder *recordingWorldVersions) RecordMutation(_ context.Context, _ domain.SessionID, _ domain.RunID, path string, _, _ []byte) {
	recorder.mutations++
	recorder.path = path
}

func (recorder *recordingWorldVersions) TrackAccess(_ context.Context, _ domain.SessionID, path string, _ domain.FileReadState) {
	recorder.tracked++
	recorder.path = path
}

func (*recordingWorldVersions) LastAccess(context.Context, domain.SessionID, string) (*domain.FileReadState, bool, error) {
	return nil, false, nil
}

func TestGeneratedToolWorldWritesPreserveFileVersionHistory(t *testing.T) {
	root := t.TempDir()
	recorder := &recordingWorldVersions{}
	ctx := tools.WithSessionID(tools.WithRunID(context.Background(), "run-1"), "session-1")
	host := generatedWorldHost{
		moduleID: "example/world",
		grants:   map[module.Grant][]string{module.GrantFSWrite: nil},
		lookup:   func(context.Context) (string, error) { return root, nil },
		recorder: recorder,
		ctx:      ctx,
	}
	writer, err := host.OpenWrite("nested/file.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte("new content")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(root, "nested", "file.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "new content" || recorder.mutations != 1 || recorder.tracked != 1 || recorder.path != "nested/file.txt" {
		t.Fatalf("write = %q, mutations=%d tracked=%d path=%q", content, recorder.mutations, recorder.tracked, recorder.path)
	}
}

func TestGeneratedToolWorldNoOpRechecksStateAndGrant(t *testing.T) {
	for _, revoke := range []bool{false, true} {
		t.Run(map[bool]string{false: "changed-bytes", true: "revoked-grant"}[revoke], func(t *testing.T) {
			root := t.TempDir()
			target := filepath.Join(root, "file.txt")
			if err := os.WriteFile(target, []byte("AAA"), 0600); err != nil {
				t.Fatal(err)
			}
			host := generatedWorldHost{grants: map[module.Grant][]string{module.GrantFSWrite: nil}, lookup: func(context.Context) (string, error) { return root, nil }, ctx: context.Background()}
			writer, err := host.OpenWrite("file.txt")
			if err != nil {
				t.Fatal(err)
			}
			if revoke {
				delete(host.grants, module.GrantFSWrite)
			} else {
				if err := os.WriteFile(target, []byte("BBB"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := writer.Write([]byte("AAA")); err != nil {
				t.Fatal(err)
			}
			if err := writer.Close(); err == nil {
				t.Fatal("stale or unauthorized no-op reported success")
			}
			expected := "BBB"
			if revoke {
				expected = "AAA"
			}
			got, err := os.ReadFile(target)
			if err != nil || string(got) != expected {
				t.Fatalf("target changed: %s %v", got, err)
			}
		})
	}
}
