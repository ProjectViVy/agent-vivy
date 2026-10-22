package runtime

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
)

func TestD14SteerExactDeliverySurvivesStateReplay(t *testing.T) {
	text := "keep the pending tool, then inspect B"
	prior := testUser(text)
	prior.Extra = map[string]any{steerExtraKey: text}
	pending := testToolCalls(testCall("pending", "read_file", `{}`))
	state := &adk.TypedChatModelAgentState[*schema.AgenticMessage]{Messages: []*schema.AgenticMessage{nil, prior, pending}}
	middleware := newSteerMiddleware()
	_, replayed, err := middleware.BeforeModelRewriteState(withSteerText(context.Background(), text), state, nil)
	if err != nil || replayed != state {
		t.Fatalf("exact admitted delivery was duplicated: state=%p replayed=%p err=%v", state, replayed, err)
	}
	changed := "keep the pending tool, then inspect C"
	_, next, err := middleware.BeforeModelRewriteState(withSteerText(context.Background(), changed), state, nil)
	if err != nil || len(next.Messages) != 4 || next.Messages[3] != pending {
		t.Fatalf("changed steer lost pending tool continuity: %+v, err=%v", next, err)
	}
	if next.Messages[2].Extra[steerExtraKey] != changed {
		t.Fatal("new delivery did not retain its exact text marker")
	}
	_, again, err := middleware.BeforeModelRewriteState(withSteerText(context.Background(), changed), next, nil)
	if err != nil || again != next || len(state.Messages) != 3 {
		t.Fatalf("delivery retry changed history: err=%v", err)
	}
}

func TestD14SelectedWorkspaceIdentityAndConfinement(t *testing.T) {
	selected := t.TempDir()
	canonical, err := filepath.EvalSymlinks(selected)
	if err != nil {
		t.Fatal(err)
	}
	sessions := workspaceSessionLookup{"session": {ID: "session", WorkspacePath: selected}, "other": {ID: "other", WorkspacePath: selected}}
	runs := workspaceRunLookup{"run": {ID: "run", SessionID: "session"}, "other-run": {ID: "other-run", SessionID: "other"}}
	manager, err := NewSessionWorkspaceManager(filepath.Join(t.TempDir(), "private"), sessions, runs)
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := manager.Ensure(context.Background(), "run")
	if err != nil {
		t.Fatal(err)
	}
	if workspace.ID != "selected:session" || workspace.Path != canonical {
		t.Fatalf("selected identity must use its durable session owner and private canonical root: %+v", workspace)
	}
	other, err := manager.Ensure(context.Background(), "other-run")
	if err != nil || other.ID == workspace.ID || other.Path != workspace.Path {
		t.Fatalf("separate session lost its workspace ownership: %+v, err=%v", other, err)
	}
	prePersist, err := manager.Ensure(withSessionID(context.Background(), "session"), "new-run")
	if err != nil || prePersist != workspace {
		t.Fatalf("pre-persist bridge changed the durable session identity: %+v, err=%v", prePersist, err)
	}
	reopened, exists, err := manager.Existing(context.Background(), "run")
	if err != nil || !exists || reopened != workspace {
		t.Fatalf("reopen = %+v, %v, %v", reopened, exists, err)
	}
	if err := manager.ValidateRunPath(context.Background(), domain.RunID("run"), filepath.Join(selected, "new.txt")); err != nil {
		t.Fatal(err)
	}
	if err := manager.ValidateRunPath(context.Background(), "run", filepath.Join(t.TempDir(), "outside.txt")); err == nil {
		t.Fatal("selected workspace accepted an escaping path")
	}
	if err := manager.Release(context.Background(), workspace); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(selected); err != nil {
		t.Fatalf("release removed selected project: %v", err)
	}
}
