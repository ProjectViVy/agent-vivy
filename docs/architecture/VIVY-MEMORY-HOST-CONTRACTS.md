# Vivy Memory Host Contracts — Persona Projection and Session Export

Status: G0 contract freeze (MEM-0C). Docs-only; no code changes are implied by
or authorized by this document.
Issue: https://github.com/ProjectViVy/agent-vivy/issues/33
Spec: [memory providers design](../superpowers/specs/2026-09-26-memory-providers-design.md)
§Gaps GAP-A/GAP-B, REQ-MEM-9/10/11. Vocabulary:
[VIVY-MEMORY-PROFILE](VIVY-MEMORY-PROFILE.md) (MEM-0B). Upstream evidence:
[memory upstream pin](../research/2026-09-26-memory-upstream-pin.md),
[G0 readiness](../research/2026-09-26-memory-g0-readiness.md).

Both contracts are **Host-owned seams**. Runtime keeps sole ownership of model
input; providers and governance services supply data only. A `required`
`contextsource.Candidate` is never itself a system instruction — the Port is
data-only by definition (`sdk/port/contextsource/contextsource.go:1-4`), and
the promotion of provider data into an instruction happens only through the
seams defined here.

## 1. GAP-A — persona projection contract

### 1.1 The existing admission seam (survey)

The persona enters model input at run admission, inside the immutable prompt
snapshot — there is no other instruction path, and no new hook is needed:

- `internal/runtime/prompt_state.go:25-33` — `PromptInput` carries
  `Persona storage.PersonaSnapshot` as a Host-owned input to
  `buildPromptSnapshot`. Today no caller populates it
  (`internal/runtime/service.go:662-668` builds `PromptInput` without a
  `Persona` field), so `buildPromptSnapshot` substitutes the embedded
  `persona-default.md` asset with `Source="runtime/persona-default"`,
  `Revision="1"`, `Digest=sha256(body)` (`prompt_state.go:81-99`).
- `internal/storage/masks.go:33-38` — `PersonaSnapshot{Source, Revision,
  Digest, Body}` is already the versioned, digest-pinned carrier the contract
  needs; it rides inside `RunPromptPayload.Persona` (`masks.go:40-45`).
- `internal/runtime/prompt_state.go:163-191` —
  `composeAuthoritativeInstruction` is the only stage that promotes persona
  bytes into the Instruction: `runtime.md` + `configuration.md` + persona +
  `[code-mode.md]` + `[mask frame + mask JSON data]`. Mask bodies travel as
  JSON data inside a Host-owned section — never template-evaluated
  (`prompt_state.go:173-180`).
- `internal/storage/masks.go:56-67` — `RunAdmission.Prompt` is committed
  atomically with the message, run row, and `run.started` event by
  `RunAdmissionStore.CommitRunAdmission`, before any model call (MASK-3
  ordering; `internal/runtime/service.go:729-742`).
- `internal/runtime/service.go:1439-1474` — on resume,
  `promptSnapshotContext` reloads the row via `Admission.LoadRunPrompt` and
  re-validates run id, schema version, payload digest, GenerationID, and
  ComposerVersion; failure is closed (`CodeSnapshotMissing`,
  `CodeSnapshotCorrupt`, `CodeIncompatiblePrompt`, `CodeMaskUnavailable`).
- `internal/runtime/prompt_middleware.go:41-77` —
  `promptMiddleware.BeforeAgent` **assigns** (never appends) the admitted
  instruction to `runCtx.Instruction`, preserving only transient native
  middleware suffixes; `promptInstruction` (`prompt_middleware.go:150-168`)
  re-validates schema/composer/digest before the bytes reach Eino. The static
  Engine instruction (`composeStaticInstruction`, `prompt.go:19-21`) remains
  only as the legacy no-snapshot fallback.

**Seam verdict:** the persona projection attaches to `PromptInput.Persona` at
admission. The contract below defines who may populate it and under what
pinning rules; it does not add a model hook, does not touch the pre-tool
middleware, and does not change `composeAuthoritativeInstruction` ordering.

### 1.2 Projection content shape

A **persona projection** is a `storage.PersonaSnapshot` value produced by a
Host-owned resolver from a versioned persona authority (e.g. a Laputa
`persona.Service` document at pin `1e402835` — see
`docs/research/2026-09-26-memory-g0-readiness.md` §Task 3):

| Field | Rule | Seam |
| --- | --- | --- |
| `Source` | Identifies the authority and document, e.g. `laputa/persona:IDENTITY`; free-form but Host-assigned — never provider-payload-derived | `storage.PersonaSnapshot.Source` (`internal/storage/masks.go:34`) |
| `Revision` | The authority's content revision label (Laputa revision counter; static assets use `"1"`) | `PersonaSnapshot.Revision` (`masks.go:35`) |
| `Digest` | `sha256(Body)`, **recomputed by the Host** — a provider-supplied digest is never trusted verbatim (precedent: `prompt_state.go:91-93` fills an empty digest; the contract makes recomputation mandatory, not optional) | `digestText` (`prompt_state.go:207-210`) |
| `Body` | The rendered Markdown bytes, bounded by the Host's prompt-byte rules | `PersonaSnapshot.Body` (`masks.go:37`) |

Projection is *data until promoted*: the resolver output only becomes model
input when `buildPromptSnapshot` embeds it in `RunPromptPayload.Persona` and
`composeAuthoritativeInstruction` places it after `configuration.md`
(`prompt_state.go:164`). No `contextsource` path may promote a `required`
Candidate into this slot (plan Global Constraint; `contextsource` package doc,
`contextsource.go:1-4`).

### 1.3 Explicit selection and revision pinning per session lifecycle

Persona binding follows the mask-selection precedent — per-session durable
selection, revision-checked at admission:

- Precedent: `maskcontract.Selection{SessionID, MaskID, Revision}` is a
  durable per-session row (`internal/maskcontract/masks.go:138-144`);
  `MaskResolver.Capture(sessionID)` reads it at admission
  (`masks.go:177-180`; `service.go:646-660`), and `MaskCaptureCheck`
  CAS-validates `SelectionRevision`/`DefinitionRevision`/`DefinitionDigest`
  inside the admission transaction (`internal/storage/masks.go:25-31`,
  `sqlite/run_admission.go`).
- **Contract:** a session carries a Host-owned persona binding
  `{authority, document, revision}`. The binding is explicit — sealed
  Generation default or operator selection — never inferred from query text
  or request payloads (MEM-0B §6 rule 1; REQ-MEM-6).
- At every run admission the Host resolves the binding and embeds the full
  `PersonaSnapshot` (not a reference) in the committed `RunPromptSnapshot`;
  the CAS precedent applies: if the binding's revision changed between
  resolve and commit, admission fails with a conflict and zero writes, not a
  silently newer persona.
- **Revision pinning per session lifecycle:** once a session has admitted a
  run under a persona revision, that revision is *pinned* for the session.
  Later runs in the same session continue to resolve to the pinned revision.
  A newly approved revision does **not** flow into a live session.

### 1.4 Effect boundary for newly approved revisions

A persona revision approved upstream (e.g. `AcceptRequest` in the Laputa
requests queue) takes effect only at a defined boundary:

- **New sessions** resolve the latest approved revision.
- **Existing sessions** keep their pinned revision until the operator makes
  an explicit re-bind (a new selection record with its own revision — the
  `SetMaskSelection` precedent, `internal/storage/masks.go:20`). Automatic
  mid-session re-resolution is banned: a long-lived session silently changing
  identity between turns is a contract violation.
- **Resumed runs** never see a newer revision at all (§1.5).

### 1.5 Resume semantics — committed view

Resume replays the committed snapshot, never re-resolves:

- `promptSnapshotContext` (`service.go:1439-1474`) loads the durable row and
  re-validates schema, digest, GenerationID, ComposerVersion; the admitted
  `payload.Persona` bytes are what `BeforeAgent` installs. There is no code
  path that re-asks the persona authority on resume — the contract freezes
  this: **no re-resolution on resume**.
- A masked resume without the mask provider fails and stays suspended
  (`service.go:1470-1473`, `CodeMaskUnavailable`); the same rule applies to a
  pinned persona whose authority module is absent from the Generation.

### 1.6 Failure policy — required identity

- If the session's binding is satisfied: the resolved `PersonaSnapshot` is
  embedded as in §1.2. If the binding is *absent* (no persona selected), the
  existing default fallback applies unchanged (`prompt_state.go:82-87`).
- If the binding exists but the projection **cannot be produced** at the
  pinned revision — authority unavailable, revision no longer served
  (`contextsource.ErrVersionUnavailable` precedent,
  `contextsource/contextsource.go:11`), or digest mismatch — run admission
  **fails**: `buildPromptSnapshot` returns an error and `runWithOptions`
  returns before `CommitRunAdmission` (`service.go:663-669`), so no message,
  run row, journal event, or model call exists. On resume the equivalent
  failure keeps the run suspended (§1.5 chain).
- **Never a different persona.** Failure of a required identity must not
  silently degrade to `persona-default.md`, to another revision, or to an
  empty body — each is a distinct failure state that rejects the run. This is
  the behavior the spec's GAP-A freeze line demands ("required-identity
  failure must not silently yield a different persona").

### 1.7 Audit hooks

- Every admitted run already journals persona provenance inside
  `run.started`: `payloadRunStarted{PromptSchema, PromptDigest}`
  (`internal/runtime/payloads.go:9-22`; built at `service.go:719-728`). The
  digest binds the run to the exact `run_prompt_snapshots` row containing the
  persona's `Source`/`Revision`/`Digest` — audit can reconstruct *which
  persona revision authored which run* without the payload becoming a second
  transcript (the `AuditRecord` design rule, `internal/runtime/audit.go:11-22`).
- Admission-time rejections propagate as typed `maskcontract` errors
  (`CodeSnapshotCorrupt`, `CodeMaskUnavailable`) and are recorded as run
  failures through the same terminal-event path as other pre-model failures.
- Re-binding a session's persona is itself a durable, revision-checked write
  (the `SetMaskSelection`/CAS precedent), giving an auditable before/after
  pair of binding revisions.

## 2. GAP-B — completed-session export contract

Providers such as memU consume *whole completed sessions* (v1 input is "1–10
completed sessions" supplied by the application —
`docs/research/2026-09-26-memory-upstream-pin.md` §Task 3), while
`std/observer/run@v1` delivers per-run event projections. This contract
defines the Host-owned bridge.

### 2.1 What "session complete" means today (survey)

There is **no session-completion concept** in the current code; the contract
must define the trigger rather than discover it:

- `domain.Session` has no lifecycle/status field — it is `ID, Title,
  CreatedAt, UpdatedAt, SandboxMode, ApprovalPolicy, WorkspacePath`
  (`internal/domain/session.go:23-37`). `UpdatedAt` is a durable
  last-activity stamp, not a state.
- `storage.SessionStore` offers `CreateSession`, `ListSessions`,
  `GetSession`, `RenameSession`, `UpdateSandboxPolicy`, `DeleteSession`
  (`internal/storage/contracts.go:108-116`) — no close/finalize op.
- The event vocabulary (`internal/domain/event.go:9-48`) contains session-
  scoped `session.truncated` and `session.forked` but **no**
  `session.completed`/`session.closed`; `EventType.Terminal()`
  (`event.go:106-122`) maps only the six run-terminal types
  (`run.completed`, `run.failed`, `run.cancelled`, `child.completed`,
  `child.failed`, `child.cancelled`).
- `Service.DeleteSession` (`internal/runtime/service.go:483-521`) cancels
  live runs, then `sqlite` `DeleteSession` removes the session and all its
  messages/runs/journal rows in one transaction
  (`internal/storage/sqlite/sessions.go:134-176`). Deletion is a *destructive*
  end, not a completion the export can follow — export must precede or ride
  with a close, never post-delete.
- Session-scoped durable signals do exist as precedent: rewind/fork write
  `session.truncated`/`session.forked` as **synthetic run events** under
  `tr_`-prefixed run IDs via `historyEvent` +
  `recordSyntheticSessionEvent` (`rewind_service.go:115-129, 207-216`;
  `service.go:1376-1397`) — a session-level fact journaled without inventing
  a run.
- Quiescence is checkable today: `rejectBusySession`
  (`rewind_service.go:236-247`) scans `Runs.ListActiveRuns`
  (`contracts.go:241-242`) and refuses the mutation while any run of the
  session is non-terminal.
- Observer delivery today is strictly **run-scoped**: `observer.EventID` is
  `{RunID, Seq}` rendering `runID:seq` (`sdk/port/observer/observer.go:21-30`);
  `observerhost.DeliverRun` replays one run's journal after a per-provider
  cursor (`internal/observerhost/host.go:294-368`).

### 2.2 Trigger — Host-owned session completion

The contract introduces one durable signal:

- **Completion is a Host act.** The Host (Runtime/Service layer at the
  control plane's direction — e.g. a `session/close`-class operation sibling
  to `session/delete`, `internal/rpc/control.go:1085`) declares a session
  complete. It is never provider-initiated, never inferred from idleness
  alone, and never tied to `run.completed` — a session may hold many
  completed runs while still open.
- **Precondition — quiescence:** completion requires zero non-terminal runs
  in the session (the `rejectBusySession`/`ListActiveRuns` check,
  `rewind_service.go:236-247`). A completion attempted under an active run
  fails like `ErrSessionBusy`.
- **Commit:** the Host journals a session-scoped completion marker — a
  `session.completed` event type added to the closed vocabulary
  (`domain/event.go:51-88`) and appended through the existing
  synthetic-session-event seam (`recordSyntheticSessionEvent`,
  `service.go:1376-1397`), under its own synthetic run id as the
  `tr_` markers do. The marker payload carries
  `{session_id, head_message_id, turn_count, transcript_digest}` (§2.3).
  The journaled event's `EventID{RunID, Seq}` is the export's **stable
  export id** — unique, ordered, and replayable.
- **Idempotent completion:** the marker is written once; a repeated close on
  an already-completed session is a no-op returning the existing marker
  coordinates (first-writer-wins at the journal append; `Journal.Append`
  ordering guarantees, `contracts.go:61-69`).

### 2.3 Canonical session projection

The export body is a **Host-built projection**; providers never read the raw
Journal (REQ-MEM-10) and never read message storage directly.

- Source rows: `MessageStore.ListMessages(sessionID)` — the append-only
  conversation in creation order (`internal/storage/contracts.go:136-144`).
- View: the **effective session view** — the same rows folded by
  `ApplySessionTruncations` (`contracts.go:452-474`) that every session view
  shares via `effectiveSessionMessages`
  (`internal/runtime/rewind_service.go:271-287`). Exporting
  the effective view is deliberate: rewound/edited-out turns must not leak
  into memory backends. Compaction-folded history is exported as the fold
  state presents it (summary + verbatim tail), matching what the user and
  model saw.
- Turn list: canonical ordered `user`/`assistant` turns. `domain.Message`
  roles are `{user, assistant, tool}` (`internal/domain/session.go:4-19`);
  `feedableMessages` (`internal/runtime/context.go:174-183`) is the existing
  role filter precedent. Each exported turn carries `{seq, role, message_id,
  run_id, created_at, content, provenance}` where provenance is
  `{source, channel, chat_id}` from `Message.Source/Channel/ChatID`
  (`session.go:96-112`) — enough for backend-side identity mapping under the
  Host-resolved scope tuple, and nothing more.
- Envelope metadata: `{export_id, schema_version, session_id, tenant_id,
  workspace_id, title, created_at, completed_at, head_message_id,
  turn_count, transcript_digest}`; `transcript_digest` is sha256 over the
  canonical serialized turn list, giving adapters a self-verifying payload.
- Tool turns: `tool` role rows and assistant `ToolCallID`/`ToolArgs`
  payloads are **not** part of the canonical turn list; see exclusions §2.6.

### 2.4 Delivery — observer receipt path, session sibling

Delivery reuses the observer machinery with a session-scoped sibling surface:

- **Receipt vocabulary is shared.** The sibling uses the identical
  `DeliveryReceipt{EventID, ReceiptID, State}` and
  `DeliveryState{accepted,pending,completed,failed}` contract
  (`sdk/port/observer/observer.go:53-80`): the `EventID` names the journaled
  `session.completed` event; `accepted` is a durable delivery
  acknowledgement; `completed` additionally means the provider's own
  extraction finished (REQ-MEM-9 — the two stay independently inspectable,
  and reporting `completed` early is the same violation as in the profile
  §2).
- **Host cursor.** ObserverHost keeps a durable per-provider, per-export
  cursor in `SnapshotStore` (`contracts.go:72-83`) — same mechanism as
  `cursorKey(providerID, runID)` (`host.go:512-515`) but session-keyed. The
  cursor advances only on `accepted`/`completed`; `pending`/`failed`/missing
  or invalid receipts keep the cursor and retry under the existing capped
  exponential backoff (`host.go:236-279`, `ErrDeliveryPending`,
  `ErrDeliveryFailed`, `ErrInvalidReceipt`).
- **Crash recovery.** On composition, sessions holding a committed
  `session.completed` marker with an unfinished cursor are re-delivered —
  the `RecoverRunIDs` precedent (`host.go:52-55`;
  `internal/app/assembly_observers.go:85-110` discovers terminal runs from
  durable state). Redelivery after an ambiguous acknowledgement is normal
  and safe because the export id is stable (§2.5).
- **Why a sibling, not new payload fields:** `RunSubscription.
  AllowedPayloadFields` projects *journal event payloads* field-by-field
  (`host.go:40-46, 423-446`); a whole-session transcript is not an event
  payload. The `session.completed` event itself may be delivered through the
  ordinary run path as a trigger notice, but the canonical body always comes
  from the Host projector (§2.3), not the event.
- **Adapter mapping:** the export trigger is what provider-side session-end
  hooks consume — e.g. the first-party BML library already exposes
  `Provider.OnSessionEnd(SessionEndRequest) (SessionEndResponse)` with an
  idempotent `SessionEndStatusAlreadyHandled` state
  (`bml/provider.go:89-108, 810-813, 946-948`); a BML adapter maps
  `export_id` onto that idempotent call.

### 2.5 Adapter idempotency ledger

- The **adapter** keeps a delivery ledger keyed by `export_id`: on
  re-delivery of an `export_id` it already durably accepted, it returns the
  **same** `ReceiptID` and state — the exact idempotency semantics
  `DeliveryReceipt` documents (`observer.go:62-72`). The ledger is
  provider-owned state (BML keeps it inside its own scoped store per design
  decision 6; remote providers map it to backend dedupe keys).
- The **Host** side needs only its cursor + the immutable completion event:
  together they make delivery at-least-once and replayable; the ledger makes
  it effectively-once for the backend.
- Retry policy, timeouts, and `ErrCursorCorrupt` handling are inherited
  unchanged from the run path (`host.go:307-368`).

### 2.6 Provider opt-in and exclusions

- **Capability declaration:** a provider consumes exports only if it
  advertises `export` (profile §3 row; consumption-only — providers never
  read raw Journal). When extraction is heavyweight, the provider also
  advertises `async-extraction` (profile §3): the export then rides
  separately budgeted task/run operations through `controlaction.Host.
  StartRun` or the Service/task path (REQ-MEM-10), never inline in the
  delivery call (`Query`-class surfaces may not start unbounded work —
  spec decision 5).
- **Per-scope enablement:** enablement is build-owned policy, sealed at
  Generation scope — the `RunObserverPolicies` precedent
  (`sdk/internal/assembly/manifest.go:86,128`;
  `internal/app/assembly_observers.go:32-71` builds subscriptions from the
  sealed manifest; providers cannot subscribe themselves,
  `observerhost/host.go:40-42`). The export policy declares which trusted
  scope tuples (profile §6: tenant / user / agent / workspace / session) the
  provider may receive; a session whose resolved scope is not enabled is
  never projected for that provider.
- **Exclusions (always):**
  - Raw Journal events, event payloads, checkpoint bytes, and
    `run_prompt_snapshots` content — the export is a message-view
    projection, nothing else leaves storage.
  - Secrets: every projected string field passes `tools.RedactSensitive`
    and the sensitive-key scrub precedent (`observerhost`
    `redactJSONStrings`/`sensitiveJSONField`, `host.go:448-487`), as
    `Candidate.Metadata` already does (profile §1).
  - Tool payloads: `tool`-role rows, `ToolCallID`, `ToolArgs`, approval and
    hook records are excluded; a future allowed-projection field list
    (the `AllowedPayloadFields` pattern) may re-admit named tool summaries,
    never raw args.
  - Attachment binaries (`Attachment.Data`, `session.go:69-76`) and
    `FileContext.Content` bodies (`session.go:78-88`) — metadata fields
    (names, sizes, paths) only.
  - The admitted instruction/persona snapshot itself — provider prompts are
    not exported (Laputa governs identity under REQ-MEM-11, not via the
    export channel).

## 3. Constraints restated

- No new public model hook: §1 attaches to the existing `PromptInput.
  Persona` → `RunPromptSnapshot` → `BeforeAgent` admission seam.
- No pre-tool middleware overload: neither contract touches `pretool`.
- Eino import quarantine preserved: both contracts name Vivy domain types
  and SDK Ports only; nothing here imports `eino*` outside
  `internal/runtime`/`internal/provider`.
- Providers never read raw Journal; `session.completed` ≠ `run.completed`;
  persona projection is data until `composeAuthoritativeInstruction`
  promotes it.
