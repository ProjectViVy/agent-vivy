package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

// The notebook survives a backend reopen: a note written through the
// approval gate must still be visible after a restart (MA-3 acceptance).
func TestNoteStorePersistsAcrossReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "notes.db")

	b, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := b.AppendNote(ctx, domain.Note{ID: "note_a", Content: "buy milk", CreatedAt: 1000}); err != nil {
		t.Fatalf("append: %v", err)
	}
	if err := b.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	again, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = again.Close() }()
	got, err := again.GetNote(ctx, "note_a")
	if err != nil {
		t.Fatalf("get after reopen: %v", err)
	}
	if got.Content != "buy milk" || got.CreatedAt != 1000 {
		t.Fatalf("note after reopen = %+v", got)
	}
}

func TestNoteStoreListNewestFirst(t *testing.T) {
	b := openBackend(t)
	ctx := context.Background()

	for _, n := range []domain.Note{
		{ID: "note_old", Content: "first", CreatedAt: 1000},
		{ID: "note_mid", Content: "second", CreatedAt: 2000},
		{ID: "note_new", Content: "third", CreatedAt: 3000},
	} {
		if err := b.AppendNote(ctx, n); err != nil {
			t.Fatalf("append %s: %v", n.ID, err)
		}
	}

	notes, err := b.ListNotes(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(notes) != 3 || notes[0].ID != "note_new" || notes[1].ID != "note_mid" || notes[2].ID != "note_old" {
		t.Fatalf("list = %+v, want newest-first", notes)
	}
}

func TestNoteStoreGetUnknown(t *testing.T) {
	b := openBackend(t)
	if _, err := b.GetNote(context.Background(), "note_missing"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("unknown id must yield storage.ErrNotFound, got %v", err)
	}
	empty, err := b.ListNotes(context.Background())
	if err != nil || len(empty) != 0 {
		t.Fatalf("empty notebook = %d, %v", len(empty), err)
	}
}
