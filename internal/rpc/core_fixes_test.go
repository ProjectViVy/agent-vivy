package rpc

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"agent-vivy/internal/domain"
)

func TestBackgroundRecoverRefusesLiveRun(t *testing.T) {
	model := &holdCancellationModel{entered: make(chan struct{}), release: make(chan struct{})}
	env, svc := newGoalLifecycleControlEnv(t, model, nil)
	defer func() { model.unblock(); svc.CancelAll(); svc.WaitIdle(context.Background()) }()
	ctx := context.Background()
	if err := env.backend.CreateSession(ctx, domain.Session{ID: "recovery-live"}); err != nil {
		t.Fatal(err)
	}
	id, err := svc.Run(ctx, "recovery-live", "keep running")
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-model.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("model not entered")
	}
	if _, rpcErr := callControl(t, env.handler, "background/recover", nil); rpcErr == nil || rpcErr.Code != CodeConflict {
		t.Fatalf("recover = %v, want conflict", rpcErr)
	}
	run, err := env.backend.GetRun(ctx, id)
	if err != nil || run.Status.Terminal() {
		t.Fatalf("live run changed: %+v, %v", run, err)
	}
}

func TestTurnFilesUseTargetSessionWorkspace(t *testing.T) {
	project, selected := t.TempDir(), t.TempDir()
	writeTestAttachment(t, filepath.Join(selected, "look.png"), testPNGBytes())
	if err := os.WriteFile(filepath.Join(selected, "notes.txt"), []byte("selected snapshot"), 0600); err != nil {
		t.Fatal(err)
	}
	env := newControlTestEnv(t, func(deps *ControlDeps) { deps.ProjectRoot = project })
	ctx := context.Background()
	if err := env.backend.CreateSession(ctx, domain.Session{ID: "selected", WorkspacePath: selected}); err != nil {
		t.Fatal(err)
	}
	_, rpcErr := callControl(t, env.handler, "turn/start", map[string]any{"session_id": "selected", "text": "selected input", "attachment_paths": []string{"look.png"}, "context_paths": []string{"notes.txt"}})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	svc := env.handler.(*controlHandler).deps.Service
	defer svc.WaitIdle(ctx)
	msgs, err := env.backend.ListMessages(ctx, "selected")
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) == 0 || len(msgs[0].Attachments) != 1 || len(msgs[0].FileContexts) != 1 || string(msgs[0].FileContexts[0].Content) != "selected snapshot" {
		t.Fatalf("input = %+v", msgs)
	}
}
