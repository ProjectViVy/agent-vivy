package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

// memNotes is the in-memory NoteStore the tools tests run against; the
// SQLite backend carries its own coverage in internal/storage/sqlite.
type memNotes struct {
	mu    sync.Mutex
	next  int
	notes []domain.Note
}

func (m *memNotes) AppendNote(_ context.Context, n domain.Note) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notes = append(m.notes, n)
	return nil
}

func (m *memNotes) ListNotes(_ context.Context) ([]domain.Note, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]domain.Note, len(m.notes))
	copy(out, m.notes)
	// Newest first, mirroring the SQLite backend.
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

func (m *memNotes) GetNote(_ context.Context, id string) (domain.Note, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, n := range m.notes {
		if n.ID == id {
			return n, nil
		}
	}
	return domain.Note{}, storage.ErrNotFound
}

func TestWriteNotePersists(t *testing.T) {
	store := &memNotes{}
	tool := NewWriteNote(store)

	out, err := tool.InvokableRun(context.Background(), json.RawMessage(`{"content":"buy milk"}`))
	if err != nil {
		t.Fatalf("InvokableRun: %v", err)
	}
	if !strings.Contains(out, "saved (1 total)") || !strings.Contains(out, "note_") {
		t.Fatalf("out = %q, want saved id and count", out)
	}
	out, err = tool.InvokableRun(context.Background(), json.RawMessage(`{"content":"call home"}`))
	if err != nil {
		t.Fatalf("InvokableRun: %v", err)
	}
	if !strings.Contains(out, "saved (2 total)") {
		t.Fatalf("out = %q, want count 2", out)
	}

	notes, err := store.ListNotes(context.Background())
	if err != nil || len(notes) != 2 {
		t.Fatalf("store holds %d notes, %v; want 2", len(notes), err)
	}
	if notes[0].Content != "call home" || notes[1].Content != "buy milk" {
		t.Fatalf("newest-first order broken: %+v", notes)
	}
}

func TestWriteNoteUnwiredStore(t *testing.T) {
	if _, err := NewWriteNote(nil).InvokableRun(context.Background(), json.RawMessage(`{"content":"x"}`)); err == nil {
		t.Fatal("unwired store must fail fast")
	}
}

func TestListNotesRun(t *testing.T) {
	store := &memNotes{}
	for i, content := range []string{"first note", "second note\nwith more lines"} {
		if err := store.AppendNote(context.Background(), domain.Note{
			ID: fmt.Sprintf("note_%d", i), Content: content, CreatedAt: int64(1000 + i),
		}); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	out, err := NewListNotes(store).InvokableRun(context.Background(), nil)
	if err != nil {
		t.Fatalf("InvokableRun: %v", err)
	}
	if !strings.Contains(out, "2 note(s) saved, newest first:") {
		t.Fatalf("missing count header: %q", out)
	}
	if !strings.Contains(out, "- note_1 (1970-01-01): second note") {
		t.Fatalf("newest entry missing or wrong: %q", out)
	}
	if strings.Contains(out, "with more lines") {
		t.Fatalf("summary must collapse to the first line: %q", out)
	}
	if strings.Index(out, "note_1") > strings.Index(out, "note_0") {
		t.Fatalf("list must be newest-first: %q", out)
	}
}

func TestListNotesEmptyAndCap(t *testing.T) {
	empty := &memNotes{}
	out, err := NewListNotes(empty).InvokableRun(context.Background(), nil)
	if err != nil || out != "no notes saved yet" {
		t.Fatalf("empty notebook = %q, %v", out, err)
	}

	full := &memNotes{}
	for i := 0; i < listNotesMaxEntries+3; i++ {
		_ = full.AppendNote(context.Background(), domain.Note{
			ID: fmt.Sprintf("note_%02d", i), Content: fmt.Sprintf("note %d", i), CreatedAt: int64(i),
		})
	}
	out, err = NewListNotes(full).InvokableRun(context.Background(), nil)
	if err != nil {
		t.Fatalf("InvokableRun: %v", err)
	}
	if !strings.Contains(out, "(3 older note(s) not shown)") {
		t.Fatalf("overflow tail missing: %q", out)
	}
	if strings.Contains(out, "note_00:") {
		t.Fatalf("oldest entry must be cut off: %q", out)
	}
}

func TestListNotesRejectsBadArgs(t *testing.T) {
	tool := NewListNotes(&memNotes{})
	for name, args := range map[string]string{
		"unknown field":  `{"x":1}`,
		"not an object":  `"plain"`,
		"malformed json": `{`,
	} {
		_, err := tool.InvokableRun(context.Background(), json.RawMessage(args))
		var argErr *ArgError
		if !errors.As(err, &argErr) {
			t.Errorf("%s: want *ArgError, got %v", name, err)
		}
	}
}

func TestReadNoteRun(t *testing.T) {
	store := &memNotes{}
	_ = store.AppendNote(context.Background(), domain.Note{
		ID: "note_target", Content: "full text\nsecond line", CreatedAt: 42,
	})

	out, err := NewReadNote(store).InvokableRun(context.Background(), json.RawMessage(`{"id":"note_target"}`))
	if err != nil {
		t.Fatalf("InvokableRun: %v", err)
	}
	if out != "full text\nsecond line" {
		t.Fatalf("read_note must return full content, got %q", out)
	}
}

func TestReadNoteRejectsBadArgs(t *testing.T) {
	store := &memNotes{}
	_ = store.AppendNote(context.Background(), domain.Note{ID: "note_a", Content: "x"})
	tool := NewReadNote(store)
	cases := map[string]string{
		"missing id":     `{}`,
		"empty id":       `{"id":""}`,
		"unknown id":     `{"id":"note_missing"}`,
		"unknown field":  `{"id":"note_a","extra":1}`,
		"not an object":  `"plain string"`,
		"malformed json": `{`,
	}
	for name, args := range cases {
		_, err := tool.InvokableRun(context.Background(), json.RawMessage(args))
		if err == nil {
			t.Errorf("%s: want error, got nil", name)
			continue
		}
		var argErr *ArgError
		if !errors.As(err, &argErr) {
			t.Errorf("%s: error %T is not a structured *ArgError", name, err)
		}
	}
}

func TestNotesToolsSpecsReadonly(t *testing.T) {
	for _, tool := range []Tool{NewListNotes(nil), NewReadNote(nil)} {
		spec := tool.Spec()
		if !spec.Readonly {
			t.Errorf("%s must be readonly to auto-execute (D-012)", spec.Name)
		}
	}
	if NewWriteNote(nil).Spec().Readonly {
		t.Error("write_note must stay effectful so the gate can interrupt it (D-012)")
	}
}
