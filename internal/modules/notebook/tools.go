package notebook

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"

	nb "agent-vivy/internal/notebookcontract"
	controlaction "agent-vivy/sdk/port/controlaction"
	toolport "agent-vivy/sdk/port/tool"
)

const (
	ToolListNotes = "list_notes"
	ToolReadNote  = "read_note"
	ToolWriteNote = "write_note"

	// toolBodyLimit keeps the retained 4 KiB note-tool bound.
	toolBodyLimit = 4096
)

var (
	activeMu     sync.RWMutex
	activeBundle nb.Bundle
)

// SetActive installs the constructed generation bundle for the tool
// contributions. Called once by App wiring after factory invocation; the
// tools fail closed (capability_unavailable) before it runs.
func SetActive(b nb.Bundle) {
	activeMu.Lock()
	defer activeMu.Unlock()
	activeBundle = b
}

// Active returns the current generation bundle, or nil when the module is
// not selected or not yet constructed.
func Active() nb.Bundle {
	activeMu.RLock()
	defer activeMu.RUnlock()
	return activeBundle
}

// ClearActive drops the bundle on Shutdown.
func ClearActive() { SetActive(nil) }

type notebookTool struct {
	definition toolport.Definition
	invoke     func(context.Context, nb.ScopedActions, json.RawMessage) (toolport.Result, error)
}

func (t *notebookTool) Definition() toolport.Definition { return t.definition }

// Invoke resolves the active bundle and binds the agent-origin facade to the
// Home scope. Unbound or omitted modules surface an error instead of a
// silent no-op.
func (t *notebookTool) Invoke(ctx context.Context, _ toolport.Host, args json.RawMessage) (toolport.Result, error) {
	b := Active()
	if b == nil {
		return toolport.Result{}, errors.New("notebook: module not active")
	}
	facade := b.Actions(nb.HomeScopeID, nb.Actor{Kind: nb.ActorAgent, Ref: "tool:" + t.definition.ID})
	return t.invoke(ctx, facade, append(json.RawMessage(nil), args...))
}

// ToolProviders returns the std/tool@v1 contribution rebinding the retained
// note tools onto the scoped notebook store.
func ToolProviders() []toolport.ToolProvider {
	return []toolport.ToolProvider{
		&notebookTool{definition: toolport.Definition{
			ID:          ToolListNotes,
			Description: "List notes in the user notebook (titles and ids).",
			Schema:      schemaOrNil(`{"type":"object","properties":{"cursor":{"type":"string"},"limit":{"type":"integer"}}}`),
		}, invoke: invokeListNotes},
		&notebookTool{definition: toolport.Definition{
			ID:          ToolReadNote,
			Description: "Read one note body from the user notebook by id.",
			Schema:      schemaOrNil(`{"type":"object","required":["id"],"properties":{"id":{"type":"string"}}}`),
		}, invoke: invokeReadNote},
		&notebookTool{definition: toolport.Definition{
			ID:          ToolWriteNote,
			Description: "Append a note to the user notebook.",
			Effect:      toolport.EffectWrite,
			Schema:      schemaOrNil(`{"type":"object","required":["content"],"properties":{"content":{"type":"string"},"operation_key":{"type":"string"}}}`),
		}, invoke: invokeWriteNote},
	}
}

type listNotesIn struct {
	Cursor string `json:"cursor,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}
type readNoteIn struct {
	ID string `json:"id"`
}
type writeNoteIn struct {
	Content      string `json:"content"`
	OperationKey string `json:"operation_key,omitempty"`
}

func decodeTool[T any](raw json.RawMessage, dst *T) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return &nb.Error{Code: nb.CodeInvalidRequest, Message: err.Error()}
	}
	return nil
}

func invokeListNotes(ctx context.Context, s nb.ScopedActions, raw json.RawMessage) (toolport.Result, error) {
	var in listNotesIn
	if err := decodeTool(raw, &in); err != nil {
		return toolport.Result{}, err
	}
	page, err := s.ListEntries(ctx, nb.ListEntriesRequest{Cursor: in.Cursor, Limit: in.Limit})
	if err != nil {
		return toolport.Result{}, err
	}
	lines, err := json.Marshal(page)
	if err != nil {
		return toolport.Result{}, err
	}
	return toolport.Result{Text: string(lines)}, nil
}

func invokeReadNote(ctx context.Context, s nb.ScopedActions, raw json.RawMessage) (toolport.Result, error) {
	var in readNoteIn
	if err := decodeTool(raw, &in); err != nil {
		return toolport.Result{}, err
	}
	if in.ID == "" {
		return toolport.Result{}, &nb.Error{Code: nb.CodeInvalidRequest, Message: "id is required"}
	}
	view, err := s.GetEntry(ctx, nb.GetEntryRequest{EntryID: in.ID})
	if err != nil {
		return toolport.Result{}, err
	}
	body, err := json.Marshal(view)
	if err != nil {
		return toolport.Result{}, err
	}
	return toolport.Result{Text: string(body)}, nil
}

func invokeWriteNote(ctx context.Context, s nb.ScopedActions, raw json.RawMessage) (toolport.Result, error) {
	var in writeNoteIn
	if err := decodeTool(raw, &in); err != nil {
		return toolport.Result{}, err
	}
	if in.Content == "" {
		return toolport.Result{}, &nb.Error{Code: nb.CodeInvalidRequest, Message: "content is required"}
	}
	if len(in.Content) > toolBodyLimit {
		return toolport.Result{}, &nb.Error{Code: nb.CodeLimitExceeded, Message: "content exceeds 4096 bytes"}
	}
	key := in.OperationKey
	if key == "" {
		// Default idempotency key: identical re-submitted content replays the
		// first receipt; distinct content admits a distinct write.
		sum := sha256.Sum256([]byte(in.Content))
		key = "tool:write_note:" + hex.EncodeToString(sum[:8])
	}
	receipt, err := s.CreateEntry(ctx, nb.OperationKeyed[nb.CreateEntryRequest]{
		OperationKey: key,
		Request: nb.CreateEntryRequest{
			SectionID: "section-notes",
			Title:     noteTitle(in.Content),
			Markdown:  in.Content,
		},
	})
	if err != nil {
		return toolport.Result{}, err
	}
	out, err := json.Marshal(receipt)
	if err != nil {
		return toolport.Result{}, err
	}
	return toolport.Result{Text: string(out)}, nil
}

func noteTitle(content string) string {
	const max = 48
	if len(content) <= max {
		return content
	}
	return content[:max]
}

func schemaOrNil(raw string) json.RawMessage {
	if raw == "" {
		return nil
	}
	return json.RawMessage(raw)
}

var _ = controlaction.EffectRead // kept for symmetry with actions inventory
