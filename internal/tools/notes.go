package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

// Registered names of the read-only notes tools (MA-3). Both are
// readonly, so the runtime auto-executes them (D-012).
const (
	ListNotesName = "list_notes"
	ReadNoteName  = "read_note"
)

// Bounded output keeps tool results inside the event payload budget
// (NFR: bounded) regardless of notebook size.
const (
	listNotesMaxEntries   = 10
	listNotesSummaryRunes = 60
)

// listNotes reports the notebook's size and a bounded newest-first
// digest; full content stays behind read_note.
type listNotes struct {
	store storage.NoteStore
}

// NewListNotes returns the read-only list_notes tool over the given note
// store.
func NewListNotes(store storage.NoteStore) Tool { return &listNotes{store: store} }

func (l *listNotes) Spec() domain.ToolSpec {
	return domain.ToolSpec{
		Name:        ListNotesName,
		Description: "Lists the user's saved notes: count plus a short summary of the newest entries.",
		Readonly:    true,
	}
}

func (l *listNotes) InvokableRun(ctx context.Context, args json.RawMessage) (string, error) {
	if l.store == nil {
		return "", fmt.Errorf("tools: note store not wired")
	}
	if len(bytes.TrimSpace(args)) > 0 {
		var empty struct{}
		dec := json.NewDecoder(bytes.NewReader(args))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&empty); err != nil {
			return "", &ArgError{Field: "args", Reason: fmt.Sprintf("list_notes takes no arguments: %v", err)}
		}
	}
	notes, err := l.store.ListNotes(ctx)
	if err != nil {
		return "", fmt.Errorf("tools: list notes: %w", err)
	}
	if len(notes) == 0 {
		return "no notes saved yet", nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%d note(s) saved, newest first:\n", len(notes))
	shown := notes
	if len(shown) > listNotesMaxEntries {
		shown = shown[:listNotesMaxEntries]
	}
	for _, n := range shown {
		fmt.Fprintf(&b, "- %s (%s): %s\n", n.ID,
			time.UnixMilli(n.CreatedAt).UTC().Format("2006-01-02"),
			noteSummaryLine(n.Content))
	}
	if len(notes) > listNotesMaxEntries {
		fmt.Fprintf(&b, "(%d older note(s) not shown)\n", len(notes)-listNotesMaxEntries)
	}
	return strings.TrimRight(b.String(), "\n"), nil
}

// noteSummaryLine compresses one note into a single bounded line: the
// first line of content, truncated with an ellipsis marker.
func noteSummaryLine(content string) string {
	line := content
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = line[:i]
	}
	line = strings.TrimSpace(line)
	runes := []rune(line)
	if len(runes) > listNotesSummaryRunes {
		return string(runes[:listNotesSummaryRunes]) + "..."
	}
	if line == "" {
		return "(empty first line)"
	}
	return line
}

// readNote returns one note's full content by id.
type readNote struct {
	store storage.NoteStore
}

// NewReadNote returns the read-only read_note tool over the given note
// store.
func NewReadNote(store storage.NoteStore) Tool { return &readNote{store: store} }

func (r *readNote) Spec() domain.ToolSpec {
	return domain.ToolSpec{
		Name:        ReadNoteName,
		Description: "Reads the full content of one saved note by its id (see list_notes).",
		Readonly:    true,
		Params: map[string]domain.ToolParam{
			"id": {Desc: "The note id to read, e.g. note_abc123.", Required: true},
		},
	}
}

type readNoteArgs struct {
	ID string `json:"id"`
}

func (r *readNote) InvokableRun(ctx context.Context, args json.RawMessage) (string, error) {
	if r.store == nil {
		return "", fmt.Errorf("tools: note store not wired")
	}
	var parsed readNoteArgs
	dec := json.NewDecoder(bytes.NewReader(args))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&parsed); err != nil {
		return "", &ArgError{Field: "args", Reason: fmt.Sprintf("must be a JSON object {\"id\": string}: %v", err)}
	}
	if parsed.ID == "" {
		return "", &ArgError{Field: "id", Reason: "must be a non-empty note id"}
	}
	note, err := r.store.GetNote(ctx, parsed.ID)
	if errors.Is(err, storage.ErrNotFound) {
		return "", &ArgError{Field: "id", Reason: fmt.Sprintf("unknown note id %q (see list_notes)", parsed.ID)}
	}
	if err != nil {
		return "", fmt.Errorf("tools: read note: %w", err)
	}
	return note.Content, nil
}
