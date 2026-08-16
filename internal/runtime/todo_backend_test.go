package runtime

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/adk/middlewares/plantask"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/tools"
)

func newTodoTestBackend(t *testing.T) (*EinoTodoBackend, *sqlite.Backend, domain.SessionID, context.Context) {
	t.Helper()
	store, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "todo.db"))
	if err != nil {
		t.Fatalf("open todo store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	sessionID := domain.SessionID("session-todo")
	if err := store.CreateSession(context.Background(), domain.Session{ID: sessionID, Title: "todo", CreatedAt: time.Now().UnixMilli()}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	ctx := tools.WithSessionID(context.Background(), sessionID)
	return NewEinoTodoBackend(store, ".vivy-tasks"), store, sessionID, ctx
}

func TestEinoTodoBackendPersistsPlantaskShapeAndInvariants(t *testing.T) {
	backend, _, sessionID, ctx := newTodoTestBackend(t)
	first, err := backend.CreateTodo(ctx, sessionID, domain.Todo{Subject: "first", Description: "first task", Metadata: json.RawMessage(`{"priority":"high"}`), Status: domain.TodoPending})
	if err != nil {
		t.Fatalf("create first: %v", err)
	}
	second, err := backend.CreateTodo(ctx, sessionID, domain.Todo{Subject: "second", Description: "second task", Status: domain.TodoPending})
	if err != nil {
		t.Fatalf("create second: %v", err)
	}

	first.Status = domain.TodoInProgress
	first.Blocks = []string{second.ID}
	if _, err := backend.UpdateTodo(ctx, first); err != nil {
		t.Fatalf("update first: %v", err)
	}
	second.Status = domain.TodoInProgress
	if _, err := backend.UpdateTodo(ctx, second); err == nil || !strings.Contains(err.Error(), "only one") {
		t.Fatalf("second in_progress error = %v", err)
	}
	second.Status = domain.TodoPending
	second.Blocks = []string{first.ID}
	if _, err := backend.UpdateTodo(ctx, second); err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("dependency cycle error = %v", err)
	}

	items, err := backend.ListTodos(ctx, sessionID)
	if err != nil || len(items) != 2 {
		t.Fatalf("items = %+v/%v", items, err)
	}
	listed, err := backend.LsInfo(ctx, &plantask.LsInfoRequest{Path: ".vivy-tasks"})
	if err != nil || len(listed) != 2 {
		t.Fatalf("virtual list = %+v/%v", listed, err)
	}
	read, err := backend.Read(ctx, &plantask.ReadRequest{FilePath: ".vivy-tasks/1.json"})
	if err != nil || !strings.Contains(read.Content, `"subject":"first"`) {
		t.Fatalf("virtual read = %+v/%v", read, err)
	}
}
