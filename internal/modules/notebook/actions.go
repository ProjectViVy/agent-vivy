package notebook

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"

	nb "agent-vivy/internal/notebookcontract"
	controlaction "agent-vivy/sdk/port/controlaction"
)

const (
	ActionSectionsList    = "vivy.notebook.sections.list"
	ActionSectionsCreate  = "vivy.notebook.sections.create"
	ActionSectionsUpdate  = "vivy.notebook.sections.update"
	ActionSectionsDelete  = "vivy.notebook.sections.delete"
	ActionSectionsRestore = "vivy.notebook.sections.restore"
	ActionEntriesList     = "vivy.notebook.entries.list"
	ActionEntriesGet      = "vivy.notebook.entries.get"
	ActionEntriesCreate   = "vivy.notebook.entries.create"
	ActionEntriesSave     = "vivy.notebook.entries.save"
	ActionEntriesMove     = "vivy.notebook.entries.move"
	ActionEntriesDelete   = "vivy.notebook.entries.delete"
	ActionEntriesRestore  = "vivy.notebook.entries.restore"
	ActionRevisionsList   = "vivy.notebook.revisions.list"
	ActionRevisionsAdopt  = "vivy.notebook.revisions.adopt"
	ActionCommentsList    = "vivy.notebook.comments.list"
	ActionCommentsCreate  = "vivy.notebook.comments.create"
	ActionCommentsUpdate  = "vivy.notebook.comments.update"
	ActionExport          = "vivy.notebook.export"
)

const (
	maxActionInput  = nb.MaxBodyBytes + 16<<10 // body + envelope headroom
	maxActionOutput = 1 << 20                  // ActionHost wire ceiling
)

type notebookAction struct {
	definition controlaction.Definition
	invoke     func(context.Context, nb.ScopedActions, json.RawMessage) (any, error)
}

func (a notebookAction) Definition() controlaction.Definition { return a.definition }

// Invoke reaches the owner-bound facade only through the sealed internal
// ActionHost extension. A lookalike host fails closed before any decode.
func (a notebookAction) Invoke(ctx context.Context, host controlaction.Host, input json.RawMessage) (json.RawMessage, error) {
	privateHost, ok := host.(nb.ActionHost)
	if !ok || privateHost == nil {
		return nil, errors.New("notebook: action facade unavailable")
	}
	if len(input) > maxActionInput {
		return marshalOutcome(outcomeErr(&nb.Error{Code: nb.CodeLimitExceeded, Message: "input exceeds bound"}))
	}
	facade, err := privateHost.Notebook()
	if err != nil {
		return nil, err
	}
	result, err := a.invoke(ctx, facade, append(json.RawMessage(nil), input...))
	if err != nil {
		return marshalOutcome(outcomeErr(err))
	}
	return marshalOutcome(outcomeOK(result))
}

// ActionProviders returns the sealed std/control-action@v1 inventory owned
// by vivy/notebook-core.
func ActionProviders() []controlaction.Provider {
	return []controlaction.Provider{
		notebookAction{definition: def(ActionSectionsList, "List notebook sections", controlaction.EffectRead, sectionsListSchema), invoke: invoke(func(s nb.ScopedActions, in listSectionsIn) (any, error) {
			return s.ListSections(context.Background(), nb.ListSectionsRequest{Cursor: in.Cursor, Limit: in.Limit})
		})},
		notebookAction{definition: def(ActionSectionsCreate, "Create a notebook section", controlaction.EffectWrite, createSectionSchema), invoke: invokeKeyed(func(s nb.ScopedActions, req nb.OperationKeyed[nb.CreateSectionRequest]) (nb.MutationReceipt, error) {
			return s.CreateSection(context.Background(), req)
		})},
		notebookAction{definition: def(ActionSectionsUpdate, "Rename a notebook section", controlaction.EffectWrite, updateSectionSchema), invoke: invokeKeyed(func(s nb.ScopedActions, req nb.OperationKeyed[nb.UpdateSectionRequest]) (nb.MutationReceipt, error) {
			return s.UpdateSection(context.Background(), req)
		})},
		notebookAction{definition: def(ActionSectionsDelete, "Delete an empty custom section", controlaction.EffectWrite, deleteSectionSchema), invoke: invokeKeyed(func(s nb.ScopedActions, req nb.OperationKeyed[nb.DeleteSectionRequest]) (nb.MutationReceipt, error) {
			return s.DeleteSection(context.Background(), req)
		})},
		notebookAction{definition: def(ActionSectionsRestore, "Restore a deleted section", controlaction.EffectWrite, restoreSectionSchema), invoke: invokeKeyed(func(s nb.ScopedActions, req nb.OperationKeyed[nb.RestoreSectionRequest]) (nb.MutationReceipt, error) {
			return s.RestoreSection(context.Background(), req)
		})},
		notebookAction{definition: def(ActionEntriesList, "List entry metadata", controlaction.EffectRead, entriesListSchema), invoke: invoke(func(s nb.ScopedActions, in listEntriesIn) (any, error) {
			return s.ListEntries(context.Background(), nb.ListEntriesRequest{SectionID: in.SectionID, Cursor: in.Cursor, Limit: in.Limit, IncludeDeleted: in.IncludeDeleted})
		})},
		notebookAction{definition: def(ActionEntriesGet, "Get an entry and one revision", controlaction.EffectRead, entryGetSchema), invoke: invoke(func(s nb.ScopedActions, in getEntryIn) (any, error) {
			return s.GetEntry(context.Background(), nb.GetEntryRequest{EntryID: in.ID, RevisionID: in.RevisionID})
		})},
		notebookAction{definition: def(ActionEntriesCreate, "Create a note", controlaction.EffectWrite, createEntrySchema), invoke: invokeKeyed(func(s nb.ScopedActions, req nb.OperationKeyed[nb.CreateEntryRequest]) (nb.MutationReceipt, error) {
			return s.CreateEntry(context.Background(), req)
		})},
		notebookAction{definition: def(ActionEntriesSave, "Save a note revision", controlaction.EffectWrite, saveEntrySchema), invoke: invokeKeyed(func(s nb.ScopedActions, req nb.OperationKeyed[nb.SaveEntryRequest]) (nb.MutationReceipt, error) {
			return s.SaveEntry(context.Background(), req)
		})},
		notebookAction{definition: def(ActionEntriesMove, "Move an entry between sections", controlaction.EffectWrite, moveEntrySchema), invoke: invokeKeyed(func(s nb.ScopedActions, req nb.OperationKeyed[nb.MoveEntryRequest]) (nb.MutationReceipt, error) {
			return s.MoveEntry(context.Background(), req)
		})},
		notebookAction{definition: def(ActionEntriesDelete, "Delete an entry", controlaction.EffectWrite, deleteEntrySchema), invoke: invokeKeyed(func(s nb.ScopedActions, req nb.OperationKeyed[nb.DeleteEntryRequest]) (nb.MutationReceipt, error) {
			return s.DeleteEntry(context.Background(), req)
		})},
		notebookAction{definition: def(ActionEntriesRestore, "Restore an entry", controlaction.EffectWrite, restoreEntrySchema), invoke: invokeKeyed(func(s nb.ScopedActions, req nb.OperationKeyed[nb.RestoreEntryRequest]) (nb.MutationReceipt, error) {
			return s.RestoreEntry(context.Background(), req)
		})},
		notebookAction{definition: def(ActionRevisionsList, "List revision metadata", controlaction.EffectRead, revisionsListSchema), invoke: invoke(func(s nb.ScopedActions, in listRevisionsIn) (any, error) {
			return s.ListRevisions(context.Background(), nb.ListRevisionsRequest{EntryID: in.EntryID, Cursor: in.Cursor, Limit: in.Limit})
		})},
		notebookAction{definition: def(ActionRevisionsAdopt, "Adopt an older revision as a new head", controlaction.EffectWrite, adoptRevisionSchema), invoke: invokeKeyed(func(s nb.ScopedActions, req nb.OperationKeyed[nb.AdoptRevisionRequest]) (nb.MutationReceipt, error) {
			return s.AdoptRevision(context.Background(), req)
		})},
		notebookAction{definition: def(ActionCommentsList, "List entry comments", controlaction.EffectRead, commentsListSchema), invoke: invoke(func(s nb.ScopedActions, in listCommentsIn) (any, error) {
			return s.ListComments(context.Background(), nb.ListCommentsRequest{EntryID: in.EntryID, Cursor: in.Cursor, Limit: in.Limit, Status: nb.CommentStatus(in.Status)})
		})},
		notebookAction{definition: def(ActionCommentsCreate, "Add a comment", controlaction.EffectWrite, createCommentSchema), invoke: invokeKeyed(func(s nb.ScopedActions, req nb.OperationKeyed[nb.CreateCommentRequest]) (nb.MutationReceipt, error) {
			return s.CreateComment(context.Background(), req)
		})},
		notebookAction{definition: def(ActionCommentsUpdate, "Edit or change a comment status", controlaction.EffectWrite, updateCommentSchema), invoke: invokeKeyed(func(s nb.ScopedActions, req nb.OperationKeyed[nb.UpdateCommentRequest]) (nb.MutationReceipt, error) {
			return s.UpdateComment(context.Background(), req)
		})},
		notebookAction{definition: def(ActionExport, "Export one revision with sidecar data", controlaction.EffectRead, exportSchema), invoke: invoke(func(s nb.ScopedActions, in exportIn) (any, error) {
			return s.Export(context.Background(), nb.GetEntryRequest{EntryID: in.EntryID, RevisionID: in.RevisionID})
		})},
	}
}

// keyedWire is the on-wire mutation envelope: operation_key + the request's
// own fields flattened — the JSON schema rejects scope/actor/origin fields.
type keyedWire[T any] struct {
	OperationKey string `json:"operation_key"`
	Request      T      `json:"request"`
}

// --- input shapes (strict decoders reject forged scope/actor/origin) ---------

type listSectionsIn struct {
	Cursor string `json:"cursor,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}
type listEntriesIn struct {
	SectionID      string `json:"section_id,omitempty"`
	Cursor         string `json:"cursor,omitempty"`
	Limit          int    `json:"limit,omitempty"`
	IncludeDeleted bool   `json:"include_deleted,omitempty"`
}
type getEntryIn struct {
	ID         string `json:"id"`
	RevisionID string `json:"revision_id,omitempty"`
}
type listRevisionsIn struct {
	EntryID string `json:"entry_id"`
	Cursor  string `json:"cursor,omitempty"`
	Limit   int    `json:"limit,omitempty"`
}
type listCommentsIn struct {
	EntryID string `json:"entry_id"`
	Cursor  string `json:"cursor,omitempty"`
	Limit   int    `json:"limit,omitempty"`
	Status  string `json:"status,omitempty"`
}
type exportIn struct {
	EntryID    string `json:"entry_id"`
	RevisionID string `json:"revision_id"`
}

// --- result envelope ----------------------------------------------------------

type outcome struct {
	Status string         `json:"status"`
	Data   any            `json:"data,omitempty"`
	Error  *outcomeErrObj `json:"error,omitempty"`
}

type outcomeErrObj struct {
	Code              nb.Code `json:"code"`
	Message           string  `json:"message"`
	Retryable         bool    `json:"retryable"`
	CurrentVersion    int64   `json:"current_version,omitempty"`
	CurrentRevisionID string  `json:"current_revision_id,omitempty"`
}

func outcomeOK(data any) outcome { return outcome{Status: "ok", Data: data} }

func outcomeErr(err error) outcome {
	obj := &outcomeErrObj{Code: nb.CodeStorageUnavailable, Message: err.Error(), Retryable: true}
	var ne *nb.Error
	if errors.As(err, &ne) {
		obj.Code, obj.Message, obj.Retryable = ne.Code, ne.Message, ne.Retryable
		obj.CurrentVersion, obj.CurrentRevisionID = ne.CurrentVersion, ne.CurrentRevisionID
	}
	return outcome{Status: "error", Error: obj}
}

func marshalOutcome(o outcome) (json.RawMessage, error) {
	raw, err := json.Marshal(o)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func decodeStrict[T any](raw json.RawMessage, dst *T) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return &nb.Error{Code: nb.CodeInvalidRequest, Message: err.Error()}
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return &nb.Error{Code: nb.CodeInvalidRequest, Message: "trailing JSON"}
	}
	return nil
}

// invoke adapts a typed read handler to the provider invoke signature.
func invoke[T any](fn func(nb.ScopedActions, T) (any, error)) func(context.Context, nb.ScopedActions, json.RawMessage) (any, error) {
	return func(ctx context.Context, s nb.ScopedActions, raw json.RawMessage) (any, error) {
		var in T
		if err := decodeStrict(raw, &in); err != nil {
			return nil, err
		}
		return fn(s, in)
	}
}

// invokeKeyed decodes the {operation_key, request} mutation envelope and
// forwards both into the facade's trusted context.
func invokeKeyed[T any](fn func(nb.ScopedActions, nb.OperationKeyed[T]) (nb.MutationReceipt, error)) func(context.Context, nb.ScopedActions, json.RawMessage) (any, error) {
	return func(ctx context.Context, s nb.ScopedActions, raw json.RawMessage) (any, error) {
		var wire keyedWire[T]
		if err := decodeStrict(raw, &wire); err != nil {
			return nil, err
		}
		if wire.OperationKey == "" {
			return nil, &nb.Error{Code: nb.CodeInvalidRequest, Message: "operation_key is required"}
		}
		return fn(s, nb.OperationKeyed[T]{OperationKey: wire.OperationKey, Request: wire.Request})
	}
}

func def(id, description string, effect controlaction.Effect, input json.RawMessage) controlaction.Definition {
	return controlaction.Definition{
		ID: id, Owner: ID, ModuleID: ID, Description: description, Effect: effect,
		MaxInputBytes: maxActionInput, MaxOutputBytes: maxActionOutput,
		InputSchema: input, ResultSchema: outcomeSchema,
	}
}

// --- strict input schemas -----------------------------------------------------
// The wire carries no scope/actor/origin/provenance fields — those are bound by
// the ActionHost from the authenticated identity. additionalProperties:false at
// both envelope and request level rejects forged authority claims at the schema
// boundary before decodeStrict re-checks at the Go layer.

const (
	strID       = `{"type":"string","minLength":1,"maxLength":128}`
	strNonEmpty = `{"type":"string","minLength":1}`
	cursorProp  = `{"type":"string","maxLength":512}`
	limitProp   = `{"type":"integer","minimum":0,"maximum":4294967295}`
	opKeyProp   = `{"type":"string","minLength":1,"maxLength":256}`
	versionProp = `{"type":"integer","minimum":0,"maximum":9007199254740991}`
)

var (
	sectionsListSchema  = json.RawMessage(`{"type":"object","properties":{"cursor":` + cursorProp + `,"limit":` + limitProp + `},"additionalProperties":false}`)
	entriesListSchema   = json.RawMessage(`{"type":"object","properties":{"section_id":` + strID + `,"cursor":` + cursorProp + `,"limit":` + limitProp + `,"include_deleted":{"type":"boolean"}},"additionalProperties":false}`)
	entryGetSchema      = json.RawMessage(`{"type":"object","properties":{"id":` + strID + `,"revision_id":` + strID + `},"required":["id"],"additionalProperties":false}`)
	revisionsListSchema = json.RawMessage(`{"type":"object","properties":{"entry_id":` + strID + `,"cursor":` + cursorProp + `,"limit":` + limitProp + `},"required":["entry_id"],"additionalProperties":false}`)
	commentsListSchema  = json.RawMessage(`{"type":"object","properties":{"entry_id":` + strID + `,"cursor":` + cursorProp + `,"limit":` + limitProp + `,"status":{"type":"string","enum":["active","resolved","deleted"]}},"required":["entry_id"],"additionalProperties":false}`)
	exportSchema        = json.RawMessage(`{"type":"object","properties":{"entry_id":` + strID + `,"revision_id":` + strID + `},"required":["entry_id","revision_id"],"additionalProperties":false}`)

	createSectionSchema  = keyedSchema(`{"type":"object","properties":{"title":` + strNonEmpty + `},"required":["title"],"additionalProperties":false}`)
	updateSectionSchema  = keyedSchema(`{"type":"object","properties":{"id":` + strID + `,"title":` + strNonEmpty + `,"expected_version":` + versionProp + `},"required":["id","title","expected_version"],"additionalProperties":false}`)
	deleteSectionSchema  = keyedSchema(`{"type":"object","properties":{"id":` + strID + `,"expected_version":` + versionProp + `},"required":["id","expected_version"],"additionalProperties":false}`)
	restoreSectionSchema = keyedSchema(`{"type":"object","properties":{"id":` + strID + `,"expected_version":` + versionProp + `},"required":["id","expected_version"],"additionalProperties":false}`)

	createEntrySchema   = keyedSchema(`{"type":"object","properties":{"section_id":` + strID + `,"title":` + strNonEmpty + `,"markdown":{"type":"string"}},"required":["section_id","title","markdown"],"additionalProperties":false}`)
	saveEntrySchema     = keyedSchema(`{"type":"object","properties":{"entry_id":` + strID + `,"expected_version":` + versionProp + `,"base_revision_id":` + strID + `,"title":` + strNonEmpty + `,"markdown":{"type":"string"}},"required":["entry_id","expected_version","base_revision_id","title","markdown"],"additionalProperties":false}`)
	moveEntrySchema     = keyedSchema(`{"type":"object","properties":{"entry_id":` + strID + `,"section_id":` + strID + `,"expected_version":` + versionProp + `},"required":["entry_id","section_id","expected_version"],"additionalProperties":false}`)
	deleteEntrySchema   = keyedSchema(`{"type":"object","properties":{"entry_id":` + strID + `,"expected_version":` + versionProp + `},"required":["entry_id","expected_version"],"additionalProperties":false}`)
	restoreEntrySchema  = keyedSchema(`{"type":"object","properties":{"entry_id":` + strID + `,"expected_version":` + versionProp + `},"required":["entry_id","expected_version"],"additionalProperties":false}`)
	adoptRevisionSchema = keyedSchema(`{"type":"object","properties":{"entry_id":` + strID + `,"revision_id":` + strID + `,"expected_version":` + versionProp + `},"required":["entry_id","revision_id","expected_version"],"additionalProperties":false}`)

	createCommentSchema = keyedSchema(`{"type":"object","properties":{"entry_id":` + strID + `,"anchor_revision_id":` + strID + `,"body":` + strNonEmpty + `},"required":["entry_id","body"],"additionalProperties":false}`)
	updateCommentSchema = keyedSchema(`{"type":"object","properties":{"comment_id":` + strID + `,"expected_version":` + versionProp + `,"body":{"type":"string"},"status":{"type":"string","enum":["active","resolved","deleted"]}},"required":["comment_id","expected_version"],"additionalProperties":false}`)

	outcomeSchema = json.RawMessage(`{"type":"object","required":["status"]}`)
)

func keyedSchema(request string) json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"operation_key":` + opKeyProp + `,"request":` + request + `},"required":["operation_key","request"],"additionalProperties":false}`)
}
