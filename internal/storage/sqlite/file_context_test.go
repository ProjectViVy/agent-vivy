package sqlite

import (
	"context"
	"testing"

	"agent-vivy/internal/domain"
)

func TestMessagesPersistFileContextSnapshots(t *testing.T) {
	b := openBackend(t)
	ctx := context.Background()
	if err := b.CreateSession(ctx, domain.Session{ID: "s-file", Title: "file", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	body := []byte("captured before edit")
	if err := b.AppendMessage(ctx, domain.Message{
		ID: "m-file", SessionID: "s-file", Role: domain.RoleUser, CreatedAt: 1, Content: "inspect",
		FileContexts: []domain.FileContext{{Path: "README.md", Name: "README.md", Size: int64(len(body)), Content: body}},
	}); err != nil {
		t.Fatal(err)
	}
	body[0] = 'X'
	got, err := b.ListMessages(ctx, "s-file")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || len(got[0].FileContexts) != 1 {
		t.Fatalf("messages = %+v", got)
	}
	file := got[0].FileContexts[0]
	if file.Path != "README.md" || file.Name != "README.md" || file.Size != int64(len("captured before edit")) || string(file.Content) != "captured before edit" {
		t.Fatalf("file context = %+v", file)
	}
	if err := b.DeleteSession(ctx, "s-file"); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := b.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM message_file_contexts`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("file context rows after delete = %d", count)
	}
}

func TestMessagesPersistEmptyFileContextSnapshot(t *testing.T) {
	b := openBackend(t)
	ctx := context.Background()
	if err := b.CreateSession(ctx, domain.Session{ID: "s-empty-file", Title: "empty", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := b.AppendMessage(ctx, domain.Message{
		ID: "m-empty-file", SessionID: "s-empty-file", Role: domain.RoleUser, CreatedAt: 1, Content: "inspect",
		FileContexts: []domain.FileContext{{Path: "empty.txt", Name: "empty.txt", Content: []byte{}, Size: 0}},
	}); err != nil {
		t.Fatal(err)
	}
	got, err := b.ListMessages(ctx, "s-empty-file")
	if err != nil || len(got) != 1 || len(got[0].FileContexts) != 1 || got[0].FileContexts[0].Size != 0 || got[0].FileContexts[0].Content == nil {
		t.Fatalf("empty snapshot = %+v/%v", got, err)
	}
	copyMessage := got[0]
	copyMessage.ID = "m-empty-fork"
	copyMessage.SessionID = "s-empty-fork"
	if _, err := b.CommitSessionFork(ctx, domain.Session{ID: "s-empty-fork", Title: "fork", CreatedAt: 2}, []domain.Message{copyMessage}, nil, nil); err != nil {
		t.Fatalf("fork empty snapshot: %v", err)
	}
	forked, err := b.ListMessages(ctx, "s-empty-fork")
	if err != nil || len(forked) != 1 || len(forked[0].FileContexts) != 1 || forked[0].FileContexts[0].Content == nil {
		t.Fatalf("forked empty snapshot = %+v/%v", forked, err)
	}
}
