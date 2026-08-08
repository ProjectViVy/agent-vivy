package tools

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

// WriteNoteName is the registered name of the effectful note tool
// (D-012).
const WriteNoteName = "write_note"

// noteContentLimit bounds one note (NFR: bounded).
const noteContentLimit = 4096

// writeNote appends a note to the persisted notebook (MA-3). It mutates
// state, so it is not readonly: the runtime interrupts before
// InvokableRun and only executes it after an approved decision (D-012).
type writeNote struct {
	store storage.NoteStore
}

// NewWriteNote returns the effectful write_note tool over the given note
// store; Builtin registers exactly one instance.
func NewWriteNote(store storage.NoteStore) Tool { return &writeNote{store: store} }

func (w *writeNote) Spec() domain.ToolSpec {
	return domain.ToolSpec{
		Name:        WriteNoteName,
		Description: "Saves a note for the user. Effectful: requires approval before executing.",
		Readonly:    false,
		Params: map[string]domain.ToolParam{
			"content": {Desc: "The note content to save.", Required: true},
		},
	}
}

type noteArgs struct {
	Content string `json:"content"`
}

func (w *writeNote) InvokableRun(ctx context.Context, args json.RawMessage) (string, error) {
	if w.store == nil {
		return "", fmt.Errorf("tools: note store not wired")
	}
	var parsed noteArgs
	dec := json.NewDecoder(bytes.NewReader(args))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&parsed); err != nil {
		return "", &ArgError{Field: "args", Reason: fmt.Sprintf("must be a JSON object {\"content\": string}: %v", err)}
	}
	if parsed.Content == "" {
		return "", &ArgError{Field: "content", Reason: "must be a non-empty string"}
	}
	if len(parsed.Content) > noteContentLimit {
		return "", &ArgError{Field: "content", Reason: fmt.Sprintf("exceeds %d bytes", noteContentLimit)}
	}
	note := domain.Note{
		ID:        newNoteID(),
		Content:   parsed.Content,
		CreatedAt: time.Now().UnixMilli(),
	}
	if err := w.store.AppendNote(ctx, note); err != nil {
		return "", fmt.Errorf("tools: save note: %w", err)
	}
	notes, err := w.store.ListNotes(ctx)
	if err != nil {
		return "", fmt.Errorf("tools: count notes: %w", err)
	}
	return fmt.Sprintf("note %s saved (%d total)", note.ID, len(notes)), nil
}

// newNoteID mints an identity-grade note id; like the runtime's prefixed
// ids there is no safe fallback for randomness failures.
func newNoteID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("tools: crypto/rand unavailable: %v", err))
	}
	return "note_" + hex.EncodeToString(b)
}
