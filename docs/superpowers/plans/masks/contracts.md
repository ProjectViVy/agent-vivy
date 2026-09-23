# MASK shared contracts

Contract revision: MASK-C1. Proposed implementation types; authoritative design:
[mask architecture](../../specs/2026-09-21-mask-subsystem-design.md).
Status/dependencies live only in [index](index.md). Imports below use aliases
`mask` = `agent-vivy/internal/maskcontract`, `domain` = existing domain package.
Code fences are contract excerpts, not files to compile unchanged without imports.

## 1. Domain and internal service

Location NEW `internal/maskcontract/masks.go`. Carry forward design §4's
`Selection`, `Definition`, `Snapshot`, `Capture`, `Resolver` exactly. Additional
values and narrow management interface:

```go
type ListRequest struct { AfterID string; Limit int }
type Metadata struct {
    ID, Name, Description, Digest, GenerationID string
    Revision int64
    BuiltIn bool
}
type Page struct { Items []Metadata; NextAfterID string }
type GetRequest struct { ID string }
type CreateRequest struct { OperationID, Name, Description, Body string }
type UpdateRequest struct {
    ID string
    ExpectedRevision int64
    Name, Description, Body string
}
type DeleteRequest struct { ID string; ExpectedRevision int64 }
type DeleteResult struct { ID string }
type SelectionRequest struct { SessionID domain.SessionID }
type SetSelectionRequest struct {
    SessionID domain.SessionID
    MaskID string
    ExpectedRevision int64
}
type SelectionView struct {
    Selection Selection
    Available bool
    InactiveReason string // empty or "not_compiled"; failure is an error
}
type Manager interface {
    ListMasks(context.Context, ListRequest) (Page, error)
    GetMask(context.Context, GetRequest) (Definition, error)
    CreateMask(context.Context, CreateRequest) (Definition, error)
    UpdateMask(context.Context, UpdateRequest) (Definition, error)
    DeleteMask(context.Context, DeleteRequest) (DeleteResult, error)
    GetMaskSelection(context.Context, SelectionRequest) (SelectionView, error)
    SetMaskSelection(context.Context, SetSelectionRequest) (SelectionView, error)
}
type Service interface {
    Resolver
    Manager
    PromptAssets() (frame string, digest string)
}
type ActionHost interface { controlaction.Host; Manager }
```

Service Manager is not directly a public authority. ActionHost's internal wrapper
binds each Manager method to server identity and a compiler-validated T1 owner.
The service sees only authorized calls; Runtime uses only Resolver. Kernel/session
capability projection can report an inactive stored selection without constructing
this provider. No Eino or raw database types cross this boundary.

Proposed pure helpers owned by maskcontract:
`NormalizeCreate(CreateRequest) (CreateRequest, error)`,
`NormalizeUpdate(UpdateRequest) (UpdateRequest, error)`,
`DefinitionDigest(id, name, body string) string` and
`CreateRequestDigest(CreateRequest) string`. Request digest excludes OperationID
and includes normalized name/description/body; definition digest uses id/name/body.
JSON encoding is UTF-8, deterministic struct field order, no trailing newline;
Go json.Marshal's escaping is canonical for v1. Never recompute from display text.

Validation: body nonblank, valid UTF-8, max 16384 bytes after newline normalization;
name trimmed and 1–128 bytes, description max 1024 bytes. Permit TAB/LF in body,
reject other C0 controls and DEL; metadata permits no controls. Validate positive
expected definition revision and nonnegative selection revision, IDs max 128 ASCII
bytes, custom IDs UUID format under `custom/`, built-in IDs from sealed catalog.
Create OperationID is a UUID. No body rendering/template expansion in validation.

`Error` has `Code string`, `CurrentRevision int64`, `ReferenceCount int`, and an
unexported cause. Allowlisted codes are architecture §8's list; `Error()` never
includes supplied body, SQL or credential data. Unwrap retains internal cause.
ActionHost constructs a separate safe projection with no cause for transport.
Do not preserve arbitrary provider errors because they happen to use these names.

Catalog page order is lexical ID (builtins and custom together), exclusive AfterID;
invalid limit fails; absent limit becomes 50, maximum 100. NextAfterID is last ID
only if another row exists. List returns no Body. UI locale affects display keys,
not canonical built-in names/bodies/digests or pagination order.

## 2. Storage ownership and exact transaction inputs

NEW `internal/storage/masks.go`; storage imports plain maskcontract types, never
provider implementation. `MaskStore` is a narrow extension, not a public Storage Port:

```go
type MaskStore interface {
    ListCustomMasks(context.Context, mask.ListRequest) (mask.Page, error)
    GetCustomMask(context.Context, string) (mask.Definition, error)
    CreateCustomMask(context.Context, mask.CreateRequest) (mask.Definition, error)
    UpdateCustomMask(context.Context, mask.UpdateRequest) (mask.Definition, error)
    DeleteCustomMask(context.Context, mask.DeleteRequest) (mask.DeleteResult, error)
    ReadMaskCapture(context.Context, domain.SessionID) (mask.Capture, error)
    SetMaskSelection(context.Context, mask.SetSelectionRequest) (mask.Selection, error)
}
```

ReadMaskCapture reads session, selection and custom body atomically. For built-in
selection it returns Selection and nil Mask: the service fills immutable built-in
bytes. Empty Selection also yields nil Mask. Service validates built-in ID before
Set; storage rejects arbitrary custom IDs and handles custom delete races.
No method accepts caller-controlled scope/tenant. Custom creation uses server UUID;
existing OperationID compares immutable original request digest and returns current
definition with the original ID. Its bounded retry lifetime ends when deleted.

Keep `Engine` stable through type assertions to these focused extensions at App
composition. A selected mask Module requires both extensions (MaskStore and
RunAdmissionStore) at startup; no provider silently falls back to old persistence.

```go
type MaskCaptureCheck struct {
    SessionID domain.SessionID
    SelectionRevision int64
    MaskID string
    DefinitionRevision int64 // custom rows only; zero for empty/builtin
    DefinitionDigest string  // custom rows only
}
type PersonaSnapshot struct { Source, Revision, Digest, Body string }
type RunPromptPayload struct {
    Persona PersonaSnapshot
    Mask *mask.Snapshot
    Instruction string
    FramingDigest string
}
type RunPromptSnapshot struct {
    RunID domain.RunID
    SchemaVersion int            // 1
    ComposerVersion string       // "mask-prompt/1"
    GenerationID string
    Payload []byte               // canonical JSON RunPromptPayload
    PayloadSHA256 string
}
type RunAdmission struct {
    Message domain.Message
    Run domain.Run
    Started domain.RunEvent
    Prompt *RunPromptSnapshot
    ExpectedMask *MaskCaptureCheck
    Edit *SessionTruncation
}
type RunAdmissionStore interface {
    CommitRunAdmission(context.Context, RunAdmission) (domain.RunEvent, error)
    LoadRunPrompt(context.Context, domain.RunID) (RunPromptSnapshot, error)
}
```

All first-party new primary admissions include Prompt, even empty mask. nil Prompt
is only legacy/test/child compatibility, never accepted with ExpectedMask or a new
prompt-schema run marker. ExpectedMask nil means capability omitted; no selection
check is made and stored selection stays untouched. Non-nil empty-mask checks still
validate selection revision. Validate Run/Message/Started session and run binding;
user Message.RunID follows the existing user-message convention, not invented IDs.
Started gets `prompt_schema:1` and `prompt_digest`; events never carry Body.

Commit returns the assigned Journal sequence. Repeated identical admitted run IDs
return the original event only if every persisted admission scalar and snapshot
matches; otherwise conflict. No update method exists for run snapshots. Fork copy
belongs in existing CommitSessionFork transaction and does not need a second API.
Session deletion cascades selections/snapshots, definition catalog survives.

## 3. Generation and prompt adapters

Keep the construction signature outside maskcontract to avoid the cycle
`maskcontract -> storage -> maskcontract`. NEW `internal/moduleport/masks.go`:

```go
type MaskDependencies struct {
    Store storage.MaskStore
    GenerationID string
}
type MaskFactory func(context.Context, MaskDependencies) (mask.Service, error)
```

RuntimeAssembly stores an optional factory, instantiated once
by App after Core Storage starts. Its missing value means capability omitted.
Provider constructor is NEW `masks.Open(ctx, moduleport.MaskDependencies)`.
Generated wiring may import provider; App and Runtime may not import it directly.

Mask frame data comes from Service.PromptAssets (pure embedded read).
App passes those immutable values separately to Runtime prompt configuration;
Resolver remains only Capture. Runtime owns all template names/order/substitution.
Mask frame cannot redefine persona text or supply an arbitrary template function.

NEW Runtime-private helpers:
`buildPromptSnapshot(runID, generationID, persona, capture, face, assets)` returns
storage.RunPromptSnapshot and error; arguments are grouped into a `PromptInput`
struct (RunID, GenerationID, Persona, Capture, Face, Frame, FrameDigest).
`withRunPrompt(ctx, snapshot)` and `runPrompt(ctx)` carry immutable state.
`loadRunPrompt(ctx, runID)` loads/validates schema, digest, Generation, module
availability and run.started marker. Never resolve current catalog on resume.

Checkpoint envelope adds PromptSchema, PromptRunID, PromptDigest, ComposerVersion,
GenerationID. New-format Set/Get require matching run context. Legacy envelopes
are accepted only for a run whose authoritative start record lacks prompt_schema.
Prompt metadata absence cannot downgrade a marked run. Check before model AND
before resumed effectful tool execution.

## 4. Wire contract and UI surface

Go wire DTOs use explicit snake_case tags in NEW `internal/modules/masks/actions.go`;
they map to the internal types above. Every request rejects extra properties.
Definition fields: id/name/description/body/revision/digest/built_in/generation_id.
List: `{items:[metadata],next_after_id:""}`. Selection: `{session_id,mask_id,revision,
available,inactive_reason}`. Write inputs use expected_revision; create uses
operation_id. Empty values remain explicit where they express unmasked state.
Revision integers are restricted to JS safe integer range at the wire boundary;
reject overflow rather than truncate. No string/int union.

Action owner and IDs are exactly design §8. Max input 32 KiB and max output 256 KiB;
list excludes bodies, default page50/max100. The existing host timeout applies.
Catalog full get/create/update return one bounded Definition. RequiredGrants remain
empty for local T1 storage operations; this does not bypass write Policy.

Proposed SDK UI shape:

```ts
export interface ChatHeaderContext {
  readonly sessionId: string | null;
  readonly running: boolean;
}
export interface ChatHeaderContribution {
  readonly slot: 'chat.header';
  readonly render: (context: ChatHeaderContext) => React.ReactNode;
}
```

Components registry ID `vivy.masks.header`; page `/masks`; navigation group `vivy`.
No new registry/Port. Register via existing composition and cleanup handles. One
request epoch per session prevents delayed old-session reads overwriting current
selection; backend session assertion remains the authority. Code Face capability
is advertised by server initialization metadata as additive `code_mode_available`,
derived from allowed Face handling, not mask presence or client hardcoded catalog.
The code-mode control continues through existing RunOptions.Face paths.
