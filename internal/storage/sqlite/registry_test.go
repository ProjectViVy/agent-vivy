package sqlite

import (
	"context"
	"errors"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

func TestSessionCRUD(t *testing.T) {
	b := openBackend(t)
	ctx := context.Background()

	if err := b.CreateSession(ctx, domain.Session{ID: "sess-a", Title: "first", CreatedAt: 100}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := b.CreateSession(ctx, domain.Session{ID: "sess-b", Title: "second", CreatedAt: 200}); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := b.GetSession(ctx, "sess-a")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Title != "first" || got.CreatedAt != 100 {
		t.Fatalf("get = %+v", got)
	}
	if _, err := b.GetSession(ctx, "sess-none"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("get missing = %v, want ErrNotFound", err)
	}

	list, err := b.ListSessions(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 || list[0].ID != "sess-b" || list[1].ID != "sess-a" {
		t.Fatalf("list order wrong (want newest first): %+v", list)
	}

	if err := b.RenameSession(ctx, "sess-a", "renamed"); err != nil {
		t.Fatalf("rename: %v", err)
	}
	if got, _ := b.GetSession(ctx, "sess-a"); got.Title != "renamed" {
		t.Fatalf("rename not applied: %+v", got)
	}
	if err := b.RenameSession(ctx, "sess-none", "x"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("rename missing = %v, want ErrNotFound", err)
	}
}

func TestDeleteSessionCascade(t *testing.T) {
	b := openBackend(t)
	ctx := context.Background()

	if err := b.CreateSession(ctx, domain.Session{ID: "sess-x", Title: "t", CreatedAt: 1}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	if err := b.AppendMessage(ctx, domain.Message{ID: "msg-1", SessionID: "sess-x", Role: domain.RoleUser, CreatedAt: 2, Content: "hi"}); err != nil {
		t.Fatalf("append message: %v", err)
	}
	if err := b.CreateRun(ctx, domain.Run{ID: "run-x", SessionID: "sess-x", Status: domain.RunCompleted, CreatedAt: 3}); err != nil {
		t.Fatalf("create run: %v", err)
	}
	if _, err := b.Append(ctx, storage.Commit{
		RunID:  "run-x",
		Events: []domain.RunEvent{ev(domain.EventRunStarted), ev(domain.EventRunCompleted)},
	}); err != nil {
		t.Fatalf("append events: %v", err)
	}

	if err := b.DeleteSession(ctx, "sess-x"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := b.GetSession(ctx, "sess-x"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("session still there: %v", err)
	}
	msgs, err := b.ListMessages(ctx, "sess-x")
	if err != nil || len(msgs) != 0 {
		t.Fatalf("messages after cascade = %d, %v", len(msgs), err)
	}
	if _, err := b.GetRun(ctx, "run-x"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("run still there: %v", err)
	}
	it, err := b.Replay(ctx, "run-x", 0)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	defer func() { _ = it.Close() }()
	if it.Next() {
		t.Fatal("journal events must be cascaded away")
	}

	if err := b.DeleteSession(ctx, "sess-none"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("delete missing = %v, want ErrNotFound", err)
	}
}

func TestMessageAppendList(t *testing.T) {
	b := openBackend(t)
	ctx := context.Background()

	if err := b.CreateSession(ctx, domain.Session{ID: "sess-m", Title: "t", CreatedAt: 1}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	for _, m := range []domain.Message{
		{ID: "msg-a", SessionID: "sess-m", Role: domain.RoleUser, CreatedAt: 10, Content: "hello"},
		{ID: "msg-b", SessionID: "sess-m", RunID: "run-9", Role: domain.RoleAssistant, CreatedAt: 20, Content: "world"},
	} {
		if err := b.AppendMessage(ctx, m); err != nil {
			t.Fatalf("append: %v", err)
		}
	}

	got, err := b.ListMessages(ctx, "sess-m")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 2 || got[0].ID != "msg-a" || got[1].ID != "msg-b" {
		t.Fatalf("list order wrong: %+v", got)
	}
	if got[1].RunID != "run-9" || got[1].Role != domain.RoleAssistant {
		t.Fatalf("assistant message fields wrong: %+v", got[1])
	}
	if got[0].RunID != "" {
		t.Fatalf("user message must carry empty run id: %+v", got[0])
	}

	// Unknown session lists empty; existence is the caller's concern.
	empty, err := b.ListMessages(ctx, "sess-none")
	if err != nil || len(empty) != 0 {
		t.Fatalf("list unknown session = %d, %v", len(empty), err)
	}
}

func TestRunStoreLifecycle(t *testing.T) {
	b := openBackend(t)
	ctx := context.Background()

	if err := b.CreateSession(ctx, domain.Session{ID: "sess-r", Title: "t", CreatedAt: 1}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	if err := b.CreateRun(ctx, domain.Run{ID: "run-1", SessionID: "sess-r", Status: domain.RunAccepted, CreatedAt: 2}); err != nil {
		t.Fatalf("create run: %v", err)
	}
	if err := b.CreateRun(ctx, domain.Run{ID: "run-2", SessionID: "sess-r", Status: domain.RunAccepted, CreatedAt: 3}); err != nil {
		t.Fatalf("create run: %v", err)
	}

	if err := b.SetRunStatus(ctx, "run-1", domain.RunActive); err != nil {
		t.Fatalf("set active: %v", err)
	}
	if err := b.SetRunStatus(ctx, "run-2", domain.RunCompleted); err != nil {
		t.Fatalf("set completed: %v", err)
	}

	r1, err := b.GetRun(ctx, "run-1")
	if err != nil || r1.Status != domain.RunActive {
		t.Fatalf("run-1 = %+v, %v", r1, err)
	}
	if _, err := b.GetRun(ctx, "run-none"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("get missing = %v, want ErrNotFound", err)
	}
	if err := b.SetRunStatus(ctx, "run-none", domain.RunActive); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("set missing = %v, want ErrNotFound", err)
	}

	active, err := b.ListActiveRuns(ctx)
	if err != nil {
		t.Fatalf("list active: %v", err)
	}
	if len(active) != 1 || active[0].ID != "run-1" {
		t.Fatalf("active runs = %+v, want only run-1", active)
	}

	// Closing the last active run empties the enumeration.
	if err := b.SetRunStatus(ctx, "run-1", domain.RunCancelled); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	active, err = b.ListActiveRuns(ctx)
	if err != nil || len(active) != 0 {
		t.Fatalf("active after cancel = %+v, %v", active, err)
	}
}

func TestRunTreeQueriesAndChildApprovalKind(t *testing.T) {
	b := openBackend(t)
	ctx := context.Background()
	if err := b.CreateSession(ctx, domain.Session{ID: "sess-tree", Title: "tree", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := b.CreateRun(ctx, domain.Run{ID: "root", SessionID: "sess-tree", Status: domain.RunActive, CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := b.CreateRun(ctx, domain.Run{ID: "child", SessionID: "sess-tree", Status: domain.RunActive, CreatedAt: 2, Kind: domain.RunKindChild, ParentID: "root", RootID: "root", Depth: 1}); err != nil {
		t.Fatal(err)
	}
	if err := b.CreateRun(ctx, domain.Run{ID: "grandchild", SessionID: "sess-tree", Status: domain.RunActive, CreatedAt: 3, Kind: domain.RunKindChild, ParentID: "child", RootID: "root", Depth: 2}); err != nil {
		t.Fatal(err)
	}
	children, err := b.ListChildRuns(ctx, "root")
	if err != nil || len(children) != 1 || children[0].ID != "child" || children[0].Depth != 1 {
		t.Fatalf("children = %+v, %v", children, err)
	}
	tree, err := b.ListRunTree(ctx, "root")
	if err != nil || len(tree) != 2 || tree[1].ID != "grandchild" {
		t.Fatalf("tree = %+v, %v", tree, err)
	}
	if err := b.CreateApproval(ctx, domain.Approval{ID: "apr-child", RunID: "child", ToolCallID: "tool-1", Decision: domain.ApprovalPending, ExpiresAt: 9999, Kind: domain.ApprovalKindChild}); err != nil {
		t.Fatal(err)
	}
	approval, err := b.GetApproval(ctx, "apr-child")
	if err != nil || approval.Kind != domain.ApprovalKindChild {
		t.Fatalf("approval = %+v, %v", approval, err)
	}
}
