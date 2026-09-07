package rpc

import (
	"context"
	"errors"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/runtime"
)

type stubWorkspaceFiles struct {
	listErr error
	files   []runtime.WorkspaceFileInfo
	trunc   bool
	read    runtime.ReadFileResult
	readErr error
	calls   int
}

func (s *stubWorkspaceFiles) List(ctx context.Context, runID domain.RunID) ([]runtime.WorkspaceFileInfo, bool, error) {
	s.calls++
	if s.listErr != nil {
		return nil, false, s.listErr
	}
	return s.files, s.trunc, nil
}

func (s *stubWorkspaceFiles) Read(ctx context.Context, runID domain.RunID, path string) (runtime.ReadFileResult, error) {
	s.calls++
	if s.readErr != nil {
		return runtime.ReadFileResult{}, s.readErr
	}
	return s.read, nil
}

// createWorkspaceRun seeds the Journal with one session and one run so the
// workspace RPC ownership precheck accepts the run id.
func createWorkspaceRun(t *testing.T, env *controlTestEnv, runID domain.RunID) {
	t.Helper()
	ctx := context.Background()
	if err := env.backend.CreateSession(ctx, domain.Session{ID: domain.SessionID("sess_" + string(runID)), Title: "ws", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := env.backend.CreateRun(ctx, domain.Run{ID: runID, SessionID: domain.SessionID("sess_" + string(runID)), Status: domain.RunCompleted, CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
}

func TestWorkspaceRPCDisabledWithoutDep(t *testing.T) {
	env := newControlTestEnv(t)
	if _, rpcErr := callControl(t, env.handler, "workspace/list", map[string]any{"run_id": "run_x"}); rpcErr == nil || rpcErr.Code != MethodNotFound {
		t.Fatalf("list rpcErr = %v", rpcErr)
	}
	if _, rpcErr := callControl(t, env.handler, "workspace/read", map[string]any{"run_id": "run_x", "path": "a.txt"}); rpcErr == nil || rpcErr.Code != MethodNotFound {
		t.Fatalf("read rpcErr = %v", rpcErr)
	}
}

func TestWorkspaceRPCListAndRead(t *testing.T) {
	stub := &stubWorkspaceFiles{
		files: []runtime.WorkspaceFileInfo{{Path: "a.txt", Size: 2}},
		read:  runtime.ReadFileResult{Path: "a.txt", Content: "hi", Size: 2},
	}
	env := newControlTestEnv(t, func(deps *ControlDeps) { deps.WorkspaceFiles = stub })
	createWorkspaceRun(t, env, "run_x")
	if _, rpcErr := callControl(t, env.handler, "workspace/list", map[string]any{}); rpcErr == nil || rpcErr.Code != InvalidParams {
		t.Fatalf("missing run_id rpcErr = %v", rpcErr)
	}
	list, rpcErr := callControl(t, env.handler, "workspace/list", map[string]any{"run_id": "run_x"})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	files := list.(map[string]any)["files"].([]runtime.WorkspaceFileInfo)
	if len(files) != 1 || files[0].Path != "a.txt" || files[0].Size != 2 {
		t.Fatalf("files = %+v", files)
	}
	if _, rpcErr := callControl(t, env.handler, "workspace/read", map[string]any{"run_id": "run_x"}); rpcErr == nil || rpcErr.Code != InvalidParams {
		t.Fatalf("missing path rpcErr = %v", rpcErr)
	}
	read, rpcErr := callControl(t, env.handler, "workspace/read", map[string]any{"run_id": "run_x", "path": "a.txt"})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	body := read.(map[string]any)
	if body["content"] != "hi" || body["binary"] != false || body["path"] != "a.txt" || body["truncated"] != false {
		t.Fatalf("read = %+v", body)
	}
}

func TestWorkspaceRPCSurfacesInternalErrors(t *testing.T) {
	stub := &stubWorkspaceFiles{listErr: errors.New("boom"), readErr: errors.New("boom")}
	env := newControlTestEnv(t, func(deps *ControlDeps) { deps.WorkspaceFiles = stub })
	createWorkspaceRun(t, env, "run_x")
	if _, rpcErr := callControl(t, env.handler, "workspace/list", map[string]any{"run_id": "run_x"}); rpcErr == nil {
		t.Fatal("list error must surface")
	}
	if _, rpcErr := callControl(t, env.handler, "workspace/read", map[string]any{"run_id": "run_x", "path": "a.txt"}); rpcErr == nil {
		t.Fatal("read error must surface")
	}
}

func TestWorkspaceRPCFailsClosedForUnknownRun(t *testing.T) {
	stub := &stubWorkspaceFiles{}
	env := newControlTestEnv(t, func(deps *ControlDeps) { deps.WorkspaceFiles = stub })
	if _, rpcErr := callControl(t, env.handler, "workspace/list", map[string]any{"run_id": "run_never_created"}); rpcErr == nil || rpcErr.Code != CodeNotFound {
		t.Fatalf("list rpcErr = %v", rpcErr)
	}
	if _, rpcErr := callControl(t, env.handler, "workspace/read", map[string]any{"run_id": "run_never_created", "path": "a.txt"}); rpcErr == nil || rpcErr.Code != CodeNotFound {
		t.Fatalf("read rpcErr = %v", rpcErr)
	}
	if stub.calls != 0 {
		t.Fatalf("workspace accessor reached for unknown run %d times", stub.calls)
	}
}

func TestWorkspaceRPCWorkspaceNotFoundIsNotFound(t *testing.T) {
	stub := &stubWorkspaceFiles{listErr: runtime.ErrWorkspaceNotFound, readErr: runtime.ErrWorkspaceNotFound}
	env := newControlTestEnv(t, func(deps *ControlDeps) { deps.WorkspaceFiles = stub })
	createWorkspaceRun(t, env, "run_x")
	if _, rpcErr := callControl(t, env.handler, "workspace/list", map[string]any{"run_id": "run_x"}); rpcErr == nil || rpcErr.Code != CodeNotFound {
		t.Fatalf("list rpcErr = %v", rpcErr)
	}
	if _, rpcErr := callControl(t, env.handler, "workspace/read", map[string]any{"run_id": "run_x", "path": "a.txt"}); rpcErr == nil || rpcErr.Code != CodeNotFound {
		t.Fatalf("read rpcErr = %v", rpcErr)
	}
}
