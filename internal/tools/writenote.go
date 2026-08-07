package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"agent-vivy/internal/domain"
)

// WriteNoteName is the registered name of the effectful note tool
// (D-012).
const WriteNoteName = "write_note"

// noteContentLimit bounds one note (NFR: bounded).
const noteContentLimit = 4096

// writeNote appends a note to an in-memory list. It mutates state, so it
// is not readonly: the runtime interrupts before InvokableRun and only
// executes it after an approved decision (D-012). V0 keeps the list in
// process memory; persistence joins a later batch.
type writeNote struct {
	mu    sync.Mutex
	notes []string
}

// NewWriteNote returns the effectful write_note tool. Builtin registers
// exactly one instance, so every call shares the same note list.
func NewWriteNote() Tool { return &writeNote{} }

func (w *writeNote) Spec() domain.ToolSpec {
	return domain.ToolSpec{
		Name:        WriteNoteName,
		Description: "Saves a note for the user. Effectful: requires approval before executing.",
		Readonly:    false,
	}
}

type noteArgs struct {
	Content string `json:"content"`
}

func (w *writeNote) InvokableRun(_ context.Context, args json.RawMessage) (string, error) {
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
	w.mu.Lock()
	w.notes = append(w.notes, parsed.Content)
	total := len(w.notes)
	w.mu.Unlock()
	return fmt.Sprintf("note saved (%d total)", total), nil
}
