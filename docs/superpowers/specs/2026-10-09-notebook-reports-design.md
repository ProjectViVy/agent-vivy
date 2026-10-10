# Notebook and report integration: detailed design

Status: **Planning baseline: owner requested the implementation package on
2026-10-09. Product implementation is not yet authorized.**

Date: 2026-10-09, Asia/Shanghai. Baseline: `017ec8cc37970b291e04c619990aed00d5403116`.

Trackers: [agent-vivy #39](https://github.com/ProjectViVy/agent-vivy/issues/39) owns notebook content and UI; [agent-vivy #5](https://github.com/ProjectViVy/agent-vivy/issues/5) owns constrained report production and schedules.

This is the authoritative design draft for this initiative. Earlier files named
`issue39-eino-orchestration` describe a different historical initiative; they are
not notebook designs and are not superseded by this document. No product code,
repository instruction, normative architecture contract, or tracker is changed
by this draft. Proposed symbols, paths, schema, policy changes, and bindings below
do not exist merely because this document names them.

## 1. Outcome and confirmed scope

Deliver a usable, independently selectable notebook before automatic reporting.
The user can organize Markdown documents into sections, edit reports, preserve
previous versions, and leave comments consumed by a subsequent relevant report.
Content is external to ordinary conversation context and automatic learning.

Confirmed in the 2026-10-09 discussion:

- Built-in storage is the sole content authority. No Obsidian integration,
  external file editing, synchronization, or interchangeable storage framework.
- Reports are editable in the first release. Preserve generated originals and
  human revisions; regeneration must not silently replace human work.
- Custom sections, Markdown editing/preview, document-level comments, version
  protection, optional UI, and headless operations are in scope.
- Reports are generated through workflows. Notebook writing and report completion
  must not trigger memory writes, persona changes, evolution, or automatic learning.
- Daily, weekly, and monthly reports share one engine. #39 must be deliverable
  without waiting for the #5 scheduler and execution work.

The following are design defaults, proposed for review rather than additional
user mandates: soft deletion, flat sections, explicit save, no rich-text editor,
candidate revisions after regeneration of edited reports, and bounded restart
catch-up. Section 15 records the trade-offs.

| ID | Requirement | Acceptance evidence |
| --- | --- | --- |
| NB-01 | No automatic notebook/report context injection | An ordinary turn neither lists notes nor contains seeded note text; explicit read returns only the selected content |
| NB-02 | Real durable section/document management | Create, edit, move, delete/restore, restart, and export preserve expected content |
| NB-03 | Editable reports without loss of generated provenance | Human edit and regeneration preserve both histories and original source references |
| NB-04 | Comments inform the next relevant report | The generation records exact selected comment versions and their input text |
| NB-05 | Independently selectable capability and UI | No-UI and no-notebook Recipes work; omission does not delete data |
| NB-06 | Scope and concurrent-write correctness | Cross-scope reads fail; stale writes return a conflict without replacing the saved head |
| NB-07 | Preserve legacy data and retire old hot-path behavior | Import is repeatable, old rows survive, and the old digest has no runtime caller |
| RP-01 | Report-only workflow capabilities | Attempts to invoke arbitrary tools, memory/persona writes, and other workflows are refused before effects |
| RP-02 | Truthful bounded reporting | Empty, partial, truncated, fallback, and full-coverage outcomes are distinguishable |
| RP-03 | One execution authority | Manual and scheduled work share admission, Run/Journal records, cancellation, and deduplication |
| RP-04 | No reporting feedback loop | Report runs and derived activity are excluded from collection and automatic cognitive ingestion |
| RP-05 | Deterministic period and lifecycle behavior | Timezone/DST, duplicate triggers, restart, module deactivation, and cancellation are covered |

Out of scope: collaborative editing/CRDT, nested section trees, rich-text blocks,
attachments, backlinks, vector search, autonomous notebook recall, new public
Storage Port, external delivery, PDF generation, runtime plugin loading, and an
Obsidian/backend abstraction. Agent editing tools beyond the existing three note
tools are not required for the first notebook release.

## 2. Inspected baseline and consequences

| Existing source | Observed behavior | Design consequence |
| --- | --- | --- |
| `internal/runtime/service.go`: `notesDigest`, preamble construction | Lists legacy notes during run context construction | Remove this read, its call, and related digest-only plumbing |
| `internal/runtime/prompt.go`: `composeRunPreamble`, `formatNotesDigest` | Injects five short recent-note summaries | Delete automatic notebook projection; this is not a new ContextSource |
| `internal/domain/note.go`; `internal/storage/contracts.go` | Append-only ID/content/created-at note model | Introduce a separate revision-aware notebook contract; preserve old rows |
| `internal/tools/notes.go`, `writenote.go` | Three built-in tools, one legacy store, 4 KiB write bound | Retain the three tool contracts through the new optional capability; do not make the UI inherit the test-era bound |
| `plugins/vivy-notebook/module.go` | `vivy/notebook` is a UI-only module | Preserve its identity and add a separately selectable internal content capability |
| `plugins/vivy-notebook/ui/vivy-notebook/src/view.tsx`; `ui/src/lib/demo-api.ts` | Notebook uses mock reports and localStorage | Replace notebook mock calls with ActionHost operations; no localStorage content authority |
| `sdk/port/controlaction/action.go`: `Host` | Settings, secrets, StartRun, InvokeTool; no notebook persistence or constrained inference | A public action declaration alone is insufficient; add a narrow internal host bridge |
| `internal/actionhost/cognitive.go`, `host.go` | Existing sealed T1 owner-specific facade pattern | Reuse this pattern for notebook/report actions without widening the public SDK Host |
| `internal/app/app.go`: `actionAuthorize` | Write actions can be rejected under non-full-auto profiles | Explicitly design human content-edit authorization; do not mislabel writes as reads |
| `internal/runtime/workflow_service.go` | INOFY admission expects a live parent Run | Reporting requires trusted root-workflow admission within the same Service authority |
| `internal/runtime/inofy_executor.go` | Only `vivy.child-task@1`, which starts an Agent child | Reports need a sealed node executor that does not start an ordinary Agent loop |
| `internal/runtime/cron_scheduler.go` | Only `agent_turn` dispatch | Add typed report dispatch to the same scheduler; no plugin timer |
| `internal/modules/memory/provider.go`; `internal/runtime/cognitive_binding.go` | Run completion can feed memory/cognitive capture | Exclusion must occur at trusted admission and observer/source projections |
| `internal/storage/workflow_steps.go` | Durable INOFY effects/checkpoints with replay and fencing | Reuse this ledger; a report table must not become a second execution state machine |

This is source inspection, not integrated runtime certification. No live model,
browser, migration, scheduler, or optional-Generation test was run for this design.

## 3. Architecture and module ownership

Choose two optional internal T1 modules and the existing optional UI module.
This avoids granting a public plugin raw storage or inventing a general-purpose
Storage Port. Internal storage owns all schema and transactions. ActionHost and
ToolHost remain the only respective control/model-visible entry points.

| Module | Proposed/retained source | Contributions | Required services |
| --- | --- | --- | --- |
| `vivy/notebook-core` (new, T1) | `internal/modules/notebook/` | Closed notebook action inventory; existing note tools through ToolHost | Storage; ActionHost; ToolHost when tools are selected |
| `vivy/reports` (new, T1) | `internal/modules/reports/` | Closed report actions; sealed report workflow definition and node catalog | Notebook content service; existing Service, model adapter, workflow storage, cron for schedules |
| `vivy/notebook` (retained, T2 UI) | `plugins/vivy-notebook/` | `std/ui-extension@v1` | Notebook actions; report controls appear only when report actions are available |

Dependency direction: UI/actions/tools -> scoped notebook operations -> Storage.
Report workflow -> scoped source reader + bounded inference + generated-revision
writer. The notebook has no dependency on reporting, memory, LAPUTA, or Garden.

Use supported `std/control-action@v1`, `std/tool@v1`, and
`std/ui-extension@v1`. No `std/context-source` or notebook run observer is
registered. Do not add a new public Port or report-specific event bus.

The action bridge is a typed ActionHost dependency, following the existing
owner-specific cognitive facade. A provider receives it only after ActionHost
checks its generated T1 module/action binding. Scope and actor provenance are
captured by the facade, never accepted from provider JSON. Public modules with
similar names cannot obtain it.

Separately, construction needs build-owned typed factory bindings so optional
backends do not depend on unconditional imports or a global registry. Follow the
existing mask/cognitive factory pattern with two proposed internal Ports:

| Internal Port | Provider / sole consumer | Contract |
| --- | --- | --- |
| `core/notebook-service@v1` | `vivy/notebook-core` / App composition | Factory receives Storage-owned narrow repository and trusted scope resolver; returns content operations and lifecycle handle |
| `core/report-service@v1` | `vivy/reports` / App composition | Factory receives existing Service admission/model/workflow capabilities and scoped notebook writer; returns report admission/settings operations and lifecycle handle |

Both have cardinality `0..1`, Generation lifecycle, and T1-only providers. They
convey no new public Grant; a duplicate binding or missing declared dependency
fails compilation. Construction failure rolls back constructed owners; an
unselected factory is absent; calls on a closed/unarmed handle fail explicitly.
Factory inputs are named typed interfaces, not raw DB connections, credentials,
or an unrestricted service locator. App is the sole factory consumer; it passes
the resulting scoped handles to the existing Hosts.

These two internal Ports are **proposed, not SUPPORTED**. Their implementation
must add catalog definitions, internal SDK/compiler binding contracts, providers,
App consumer wiring, failure tests, conformance evidence, and Inspect projection
before they can be selected. No public Storage Port is introduced. This small
composition cost is justified by the confirmed ability to omit each backend.

Proposed contract package `internal/notebookcontract/` contains domain-facing
request/result types and narrow operations, with no storage or Eino imports:

```go
// Schematic signatures; per-operation DTOs are specified in section 7.
type Reader interface {
    ListEntries(context.Context, ListEntriesRequest) (EntryPage, error)
    GetRevision(context.Context, GetRevisionRequest) (RevisionView, error)
}
type GeneratedWriter interface {
    CommitGenerated(context.Context, GeneratedCommit) (GeneratedReceipt, error)
}
type ActionHost interface {
    controlaction.Host
    Notebook() (ScopedActions, error)
}
```

App composition constructs the service only when its generated factory exists.
Pass typed handles to ActionHost and report runtime wiring;
do not use a global `Active()` registry, dynamic lookup, or an unconditional
runtime import in `engine.go`. Build-owned bindings must disappear in omitted
Generations, following the repository's existing generated-binding conventions.
The implementation plan must enumerate actual Source Catalog/compiler changes
and prove omission of module implementation/action/UI bindings, rather than claim
runtime nil checks are modularity. Core schema and read-only export code may
remain because Storage owns historical data even when a feature is omitted.

Recipe validation rejects the UI without notebook-core and reports without
notebook-core. Core without UI/reports is valid. Established first-party modules
join the default Generation once delivered; automatic reporting remains inactive
until configured. UI removal never stops generation or deletes data.

## 4. Scope, identity, and authorization

### 4.1 Trusted scope

All data keys include a host-resolved `scope_id`. The first release supports the
existing single local operator per data root, with a personal/home notebook and
workspace notebooks. It does not introduce multi-user authentication.

- Transport identity is evidence of admission, not a durable owner key:
  existing `face/connection` and `face/embedded` IDs must not become DB owners.
- Within the independently owned database, the stable owner namespace is the
  local operator, not an ephemeral connection token or absolute data-root path.
  Home uses a fixed local scope key; workspace keys derive from the host's
  canonical workspace identity with a versioned prefix. Moving the database
  does not change ownership. Workspace renaming/rebinding does not silently move
  documents; explicit content movement remains an authorized operation. An empty
  session workspace resolves to the personal scope, not an ephemeral run path.
- UI/headless requests choose `home` or supply a session selector. The host
  resolves and authorizes that session, then derives scope. They cannot supply
  arbitrary owner IDs, scope hashes, or filesystem paths.
- Agent tools inherit scope from their Run/session and cannot select another
  workspace. A local human can select an owned workspace through the control
  plane. A non-local principal requires an existing explicit ownership mapping;
  otherwise admission fails closed, rather than granting the local owner scope.
- Every query, uniqueness constraint, revision access, comment selection, source
  lookup, and report commit includes scope. Unknown and out-of-scope resource IDs
  share `not_found` behavior.

Legacy notes have no workspace ownership. Import them into the local personal
notebook only; never copy them into every workspace or infer ownership from text.
Workspace isolation of the old global note tools is an intentional contract
correction and must appear in implementation release notes.

### 4.2 Human edits versus Agent effects

The current action policy is insufficient for ordinary user editing. Add a
trusted admission-origin distinction at the transport/ToolHost boundary, never a
client-provided `is_human` field. Preserve server authentication, instance state,
explicit policy denies, action schemas, scope checks, and audit for both paths.

- A direct authenticated local-human control request can perform the closed
  notebook content-write inventory in its owned scope without switching the
  Agent to full-auto. This is a narrow policy rule for user-owned content.
- Agent-originated calls retain the existing ToolHost policy/approval path;
  headless Agent execution is not automatically human control.
- Report generation is a separately declared scoped model-compute operation.
  Manual admission authorizes its bounded report capability set; schedule
  settings authorize future runs under the saved configuration. Neither grants
  general Agent tools or external delivery.
- Read-only/error states stay actionable in the UI. An unimplemented policy
  bridge is a blocker, not a reason to bypass ActionHost or mark edits `read`.

The implementation must carry the origin explicitly across action-to-tool and
tool-to-action bridges and test spoof attempts. The broad existing authorization
behavior for unrelated actions is outside this change.

## 5. Content and persistence model

Use the existing SQLite/PostgreSQL Storage authority. Add paired append-only
migrations at the next unallocated migration numbers at implementation time;
do not reserve numbers against this rapidly changing baseline. No module DDL,
separate database, background index, or file-backed content mirror.

| Logical relation | Essential fields and constraints |
| --- | --- |
| `notebook_sections` | scope, ID, title, optional system role, version, timestamps, tombstone; unique system role per scope |
| `notebook_entries` | scope, ID, section ID, kind (`note`/`report`), current revision ID, version, timestamps, tombstone; optional report-series/window identity |
| `notebook_revisions` | scope, entry ID, revision ID, sequence, parent revision ID, title, Markdown, origin (`legacy`/`human`/`agent`/`generated`), base generated revision ID, actor reference, created-at; immutable content |
| `notebook_comments` | scope, ID, entry ID, anchored revision ID, body, version, author, status (`active`/`resolved`/`deleted`), timestamps |
| `notebook_mutations` | scope, operation key, canonical request digest, committed result reference; unique operation key; no execution lifecycle |
| `report_generations` (#5) | scope, Run ID, destination entry/revision, report configuration revision, window/timezone/as-of, immutable fact/source/feedback snapshot, input digest, outcome mode/reason; unique Run ID |

Report execution state remains in Run/Journal/workflow storage. The generation
record is immutable provenance and an idempotent output receipt, not a worker
queue or competing status authority. Large report snapshots can reference
existing protected workflow blobs; stable revision/provenance records must pin
their retention for as long as the report exists.

Foreign keys/transaction checks prevent revisions, comments, or entries from
crossing scope. Entry kinds cannot change after creation. Title is versioned
with the body; section movement is entry metadata protected by entry version.

Defaults: flat sections `Notes`, `Daily`, `Weekly`, `Monthly`; localized labels
are presentation values over stable system roles. Sections are ordinary content
containers, not individual timers. Renaming a report section does not alter
report policy. Custom sections can contain notes or moved reports.

Delete is a tombstone in V1. Deleted entries disappear from default lists and
report input selection; explicit restore preserves history. A nonempty section
cannot be deleted until contents are moved/deleted. System-role sections can be
renamed but not deleted while an active report configuration targets them.
There is no permanent purge command in this release.

### 5.1 Save transaction and idempotency

Every mutation carries `operation_key`; updates also carry `expected_version`
and, for body edits, `base_revision_id`. In one storage transaction:

1. Resolve trusted scope; check the mutation receipt before evaluating current
   state. Same key and request digest returns the original result; different
   payload with the same key returns `idempotency_conflict`.
2. Check resource ownership, tombstone status, version, and parent revision.
3. Insert the immutable revision or update comment/metadata as applicable.
4. Compare-and-swap the entry head/version and save the mutation receipt.
5. Commit before success is visible. Any failure rolls back the whole mutation.

After response loss, retry with the same key. Do not mint a new revision merely
because the client did not receive success. A stale base returns
`revision_conflict` plus current version/revision metadata; it never auto-merges
or overwrites. Existing ActionHost audit remains in force. If final audit/transport
acknowledgement fails after data commit, reconcile through the operation receipt.

### 5.2 Generated versus human revisions

Generated originals are immutable. A user save creates a human revision with a
link to its originating generated version. Original evidence remains accessible
but is not represented as validation of the edited text. The view labels human
revisions as edited and allows opening the original.

Regeneration snapshots the entry version/head at admission. Successful publication
creates a new generated revision. It becomes the visible head only when the head
has not changed and that head is an unedited generated revision. If the current
head is human-edited, or changed during generation, retain the new output as a
candidate and preserve the visible head. UI offers `View generated version` and
`Use this version`; adoption is an explicit CAS mutation. Restoring any old
version creates a new revision referencing it, rather than erasing history.
If the destination is deleted while generation is active, publication fails with
`destination_deleted`; it must not recreate or restore the document. Staged
workflow output remains subject to execution recovery/retention, not a visible
notebook entry. A user move is preserved; generation does not move the entry back.

Report document identity is scope + report configuration/series + local window
and timezone. Regeneration revises that document; the next period creates a new
document. A configuration revision affects attempt identity and provenance,
not the identity of an existing period's document.
For a first generation, reserve its target key/Run ownership at admission but
create the entry and first revision atomically only at publication. Do not expose
an empty successful report or an entry with a broken head reference. A tombstoned
existing period must be explicitly restored before generating into it again.

## 6. Comments and feedback semantics

V1 comments are document-level, optionally anchored to a displayed revision.
They support edit, resolve, delete, and restore through version checks. They do
not require fragile text-range anchors or a discussion-thread subsystem.

At report admission, freeze the latest active comment versions relevant to:

1. The target document, when regenerating it.
2. The immediately preceding document in the same report series, for a new
   period. Unresolved comments carry forward through an explicit inherited
   feedback reference in the generation record until resolved, rather than
   being copied as fresh comments.
3. Daily report documents actually used in a weekly/monthly fact bundle.

Deduplicate by comment ID + version; enforce the input budget and disclose any
omissions. Store ID, version, body snapshot, source document, and digest. Resolve
state is rechecked for new admissions; changes made after admission affect the
next run, not the in-flight snapshot. Completion records `included`, not a claim
that the model obeyed or proved the comment correct. Comments are never consumed
or resolved automatically.

Human report edits from selected input documents may be passed as explicitly
user-authored context, with revision IDs and budget accounting. Aggregate source
facts come from generated fact bundles or authorized sessions, not from edited
Markdown. A comment or edit is not a source-event reference. Unsupported factual
corrections remain user assertions, visibly distinguished from source evidence.

These reads happen only within the authorized report operation. No comment,
report, revision summary, or section list becomes an ordinary chat preamble,
automatic ContextSource, BML extraction input, or persona update trigger.

## 7. Control actions, tools, and error contract

All action IDs below are proposed sealed inventory under `std/control-action@v1`.
JSON schemas reject unknown fields; scope selectors are resolved by the Host.
The public UI uses the existing module-action RPC, not new raw RPC routes.

| Action prefix/name | Inputs beyond trusted scope selector | Result / behavior |
| --- | --- | --- |
| `vivy.notebook.sections.list` | pagination | Section metadata |
| `vivy.notebook.sections.create` | title, operation key | Created section/version |
| `vivy.notebook.sections.update` | ID, title, expected version, operation key | Updated metadata |
| `vivy.notebook.sections.delete` / `.restore` | ID, expected version, operation key | Tombstone/restore or `section_not_empty`/`section_in_use` |
| `vivy.notebook.entries.list` | section/filter, cursor, limit, include-deleted | Metadata only, stable cursor; no body injection |
| `vivy.notebook.entries.get` | ID, optional revision ID | One selected revision and metadata |
| `vivy.notebook.entries.create` | section, title, Markdown, operation key | Ordinary note; cannot claim generated origin |
| `vivy.notebook.entries.save` | ID, expected version, base revision, title, Markdown, operation key | Human/Agent revision stamped by Host |
| `vivy.notebook.entries.move` | ID, destination section, expected version, operation key | Metadata change |
| `vivy.notebook.entries.delete` / `.restore` | ID, expected version, operation key | Tombstone/restore |
| `vivy.notebook.revisions.list` | entry ID, cursor, limit | Revision metadata, including generated candidates |
| `vivy.notebook.revisions.adopt` | entry ID, revision ID, expected version, operation key | Explicitly select/restore content as a new revision |
| `vivy.notebook.comments.list` | entry ID, cursor, status | Scoped comments |
| `vivy.notebook.comments.create` | entry ID, anchor revision, body, operation key | Host-attributed comment |
| `vivy.notebook.comments.update` | ID, expected version, body or status, operation key | Edit/resolve/delete/restore |
| `vivy.notebook.export` | entry ID, exact revision ID | Bounded Markdown plus provenance/comment sidecar data |
| `vivy.reports.generate` (#5) | period, current/completed window selector, operation key, optional target entry | Accepted authoritative Run ID; no arbitrary prompt/graph/tool list |
| `vivy.reports.get` / `.cancel` (#5) | Run ID | Existing Run projection / Service cancellation |
| `vivy.reports.settings.read` / `.write` (#5) | period configuration; write uses CAS/key | Timezone, schedule, target section, provider selection via existing model catalog |

`generated` writes are available only through the internal workflow writer bound
to an admitted report Run. No ordinary save action accepts generated provenance,
source validation flags, actor type, or arbitrary source IDs as trusted values.

Bounded result envelope: `{status, data?, error?}`. Domain errors carry stable
`code`, `retryable`, and optional current-version metadata. Required codes:
`invalid_request`, `not_found`, `revision_conflict`, `idempotency_conflict`,
`section_not_empty`, `section_in_use`, `limit_exceeded`, `capability_unavailable`,
`storage_unavailable`, `cancelled`, `recovery_required`, `outcome_unknown`.
Report publication also distinguishes `destination_deleted` from a retryable
storage failure.
Authentication, policy, schema, and Host timeout errors retain their existing
ActionHost transport/error behavior instead of being disguised as successful
domain responses. A timeout does not prove that an effect did not commit.

Design limits for review, not performance claims: 256 KiB UTF-8 per document
revision, 16 KiB per comment, at most 100 metadata rows per page, with action
wire limits sized above these payloads and bounded by Host ceilings. Metadata
lists must paginate in storage; do not load all rows and slice afterward.
Generation source/token limits derive from existing Run/model budgets and are
recorded in the admitted configuration; oversized input produces visible partial
coverage, not silent truncation. The old 4 KiB tool input bound can remain for
compatibility while UI/headless document operations use the new bound.

Existing `list_notes`, `read_note`, and `write_note` become thin ToolHost consumers
of the selected notebook service, retaining their input/output compatibility as
far as scoped access permits. Remove unconditional legacy-store binding. No
notebook module means these tools are absent; no automatic recall substitutes
for them. New edit/delete/comment Agent tools are deferred. Explicit reads may
appear in the requesting turn's normal tool context; storing a document does not
cause such a read or automatic learning.

## 8. Notebook UI

Reuse `MasterDetail` and the current host-owned route frame. Main content has a
section selector and document list, with an editor/reader detail view. Existing
`react-markdown` and `remark-gfm` dependencies provide rendered preview; start
with a text editor and explicit Save. No new editor dependency is required.

Required interactions:

- Create/rename sections; create/move/delete/restore documents; show empty states.
- Edit/preview Markdown, save state, unsaved-change navigation confirmation.
- Show revision origin, original/candidate versions, and document comments.
- On conflict preserve the local buffer and offer reload/current-version view;
  do not drop local edits after a failed request.
- For reports show period, timezone, as-of, coverage, fallback reason, sources,
  human-edited status, and included feedback. Manual generate controls appear
  only when the backend reports the capability.
- Export the selected revision with sidecar provenance. Do not mutate the
  stored Markdown to insert editable authority metadata.

Render raw HTML inert and reject unsafe link schemes using the existing Markdown
rendering policy. Saved drafts/content must not use localStorage as a second
authority. An unsaved buffer lives in page state; the UI warns before leaving.
The existing mock session-search tab is not evidence of notebook functionality;
do not expand or reimplement session search in this delivery.

Headless controls use exactly the same actions and conflict semantics. Hiding or
omitting the UI changes neither content storage nor report execution.

## 9. Report-only execution contract (#5 integration)

### 9.1 One sealed workflow, three period policies

Use INOFY's existing engine and host RunStore binding. A sealed report program
contains four typed nodes: `collect`, `narrate`, `validate-render`, and `persist`.
Daily/weekly/monthly are input policies, not separate engines. The program and
node descriptors are Generation-owned and content-hashed. Users do not submit
arbitrary graph definitions, node implementations, prompts, or tools through
`reports.generate`.

The existing generic `vivy.child-task@1` executor remains unchanged for ordinary
workflows. A report binding supplies a restricted `inofy.NodeExecutor` which
checks Run identity, epoch, program digest, trusted report purpose, node type,
scope, and operation key before each effect. It exposes only authorized source
projection, one bounded model invocation, validation/rendering, and owned output
commit. Node replies are bounded; source snapshots use durable blob references
where inline limits would be exceeded.

Add an internal `Service.StartReport` adapter to the common Service admission
authority. It creates an owned root workflow Run without a synthetic user chat
turn or a general Agent parent. Reuse the current admission transaction, Journal,
policy, execution ownership, and cancellation machinery; extend the existing
workflow admission to permit this trusted root case. Do not introduce another
Service, queue, runtime, or unrelated public workflow-root API. This bridge is
new work, not an existing supported capability.

### 9.2 Model and orchestration reuse evidence

Inspected pinned source:

- `github.com/cloudwego/eino v0.9.13`, `components/model/interface.go`:
  `BaseChatModel.Generate` and `ToolCallingChatModel.WithTools`.
- Existing Vivy integration: `internal/provider/resolving.go`,
  `internal/provider/titler.go`, and `internal/runtime/modeladapter.go`.
- `github.com/ProjectViVy/inofy v0.0.0-20260930141905-71e2c9bbe47d`, `types.go`:
  `NodeDescriptor`, `NodeExecutor.Execute`, `RunStore.Commit/Load`, and
  `Bindings{Nodes, Runs}`.

Use the existing provider/model resolution path and one tool-free `Generate`
call inside `internal/runtime`/`internal/provider`, with context deadline,
output-token limit, usage accounting, and no Agent loop. Obtain a model instance
without tools; do not mutate a shared model with deprecated `BindTools` or reuse
an already tool-bound instance. A returned tool call is invalid narrative output,
never an executable request. Eino types do not cross the domain/Port boundary.

INOFY already supplies durable orchestration. The Vivy-specific work is the
small report node effect adapter and trusted admission/provenance binding, which
Eino cannot own because they are application authority. No custom graph engine
or new model/provider implementation is justified. The planning source audit and
pinned Diva report reference are recorded in section 16; recheck that exact
revision before porting behavior and do not copy its evolution types.

### 9.3 Sources, facts, and failures

Collection resolves scope and freezes an explicit `[start,end)` window and
`as_of`. Daily uses authorized session activity. Weekly/monthly reuse generated
daily fact bundles and exact source references; missing dates fall back to
original authorized sessions. Monthly does not depend on weekly generation.

Session selection is bounded and stable. Record excluded/unavailable sources,
truncation, missing windows, and snapshot watermarks. Source references identify
exact message/event versions and their digests where applicable. Never store
provider credentials or raw unbounded Journal dumps as report evidence.

The input bundle contains facts, original evidence references, coverage, and
separately labeled human feedback. Model output is structured sections/claims
with reference IDs. Validate schema and reference membership; this is not proof
of semantic truth. Renderer escapes untrusted content and produces Markdown.

- No permitted activity: deterministic `empty` report; no invented narrative.
- Model timeout/unavailable/invalid JSON/reference/tool call: deterministic fact
  rendering with `fallback` mode and categorized reason; no model repair loop.
- Partial collection: truthful partial report when a usable permitted bundle
  exists. Entire source read failure produces an error, not a false empty report.
- Storage failure: no successful output claim. Preserve/reconcile Run state.
- Explicit cancellation: cancel work rather than manufacture a fallback report.
- Lost acknowledgement after persistence: query the Run/output receipt and reuse
  the committed result. Do not regenerate an already committed report.

### 9.4 Provenance and learning exclusion

Persist a host-stamped `purpose=report` attribute at Run admission before any
report event can be observed. Root/descendant execution and related projections
inherit it. Browser JSON, prompts, comments, or ordinary Agent tool arguments
cannot set or remove this attribute. Extend existing Run storage/projections
and paired migrations; legacy Runs keep their ordinary purpose.

Enforce exclusion at both source collection and automatic observer dispatch for
memory/evolution/cognitive consumers, including BML and LAPUTA capture. An
excluded durable observer event still advances its cursor with an explicit
excluded receipt; it must not retry forever. Keep execution audit and token
accounting visible. Any report blob/persist event carries the same trusted
provenance. Explicit notebook reads remain allowed; they do not automatically
promote reports into memory. Automatic extraction must treat report-derived
tool content as excluded input rather than ingesting it through an ordinary
conversation wrapper.

The explicit-read case needs a minimal trusted result-provenance marker carried
alongside the tool result into persisted message/source projections, not a
special string embedded in Markdown. Automatic cognitive capture must exclude
both that notebook-derived segment and any derived summary that cannot separate
it faithfully; it may skip capture for that turn with a visible excluded receipt.
It must not erase the ordinary conversation or deny the explicit read. This
metadata path applies to explicit reads of both notes and reports. Introduce it
with N2 and extend it with report execution provenance in R0; it is not assumed
to exist today. This does not retroactively remove user-authored facts already
present in an ordinary conversation before a notebook operation.

These guarantees require end-to-end tests with observers enabled, as well as a
Generation without memory/LAPUTA/Garden. Prompt instructions alone are not an
acceptable implementation of this boundary.

## 10. Scheduling, recovery, and lifecycle

Manual generation and a proposed typed cron payload `report` enter the same
`Service.StartReport` operation. Extend existing cron validation/dispatch; leave
`agent_turn` behavior intact. Store report settings/configuration revision in the
existing host configuration/cron authority, not a second scheduling database.

Periods use an IANA timezone and local calendar boundaries, converted to UTC
instants for storage. Weeks start Monday. Automatic reports cover completed
periods only; manual current-period reports record `as_of`. DST days are not
assumed to contain 24 hours. Timezone/configuration changes apply to new
admissions; in-flight work retains its admitted snapshot.

Scheduled idempotency key: scope + series + period + UTC window + timezone +
configuration revision. Manual operations use the caller's operation key;
explicit regeneration creates a new attempt/revision, while network retries
reuse the key. Only one active generation per target report document is admitted;
another request receives the existing Run/busy result, never a competing writer.

Initial catch-up policy: at most the latest missed completed period per enabled
daily/weekly/monthly schedule on restart. Record skipped ranges; do not silently
claim that older reports were generated. Users can request historical periods
manually. Source fallback allows weekly/monthly reports without a backlog of
daily report generation.

Recovery uses existing execution fences and workflow records. Reuse durable
collected input/output when safe. An ambiguous model/effect attempt is marked
`recovery_required` under the existing workflow projection, not silently replayed
or billed twice. A report persisted before process loss is reconciled by Run ID
and output receipt. The generation commit must atomically persist the revision,
provenance record, and receipt. Journal/INOFY reconciliation must not mark success
before this transaction commits.

Disabling a report schedule stops new automatic admissions; an already admitted
run settles normally unless explicitly cancelled. Disabling the report runtime
capability stops all new admissions and cancels/drains active runs under the
existing Service shutdown deadline, recording durable outcomes. UI deactivation
has no such execution effect. Omitting modules on the next Generation start
keeps their data; previously active report Runs are fenced and reconciled rather
than resumed without the owning capability.

## 11. Migration, compatibility, and export

1. Remove runtime digest injection independently of content migration. Merely
   hiding the UI or disabling new notebook features must not restore it.
   Existing conversation history/compaction may already contain previously
   injected text; do not rewrite user history or promise retroactive removal.
   Acceptance proves the notebook is not newly consulted/injected automatically.
2. Storage migrations add notebook tables without modifying released migrations
   or dropping `notes`. Import legacy rows once into personal `Notes`, preserving
   ID, bytes, and creation time; origin is `legacy` and title derives from the
   first line. Legacy records above new write limits remain readable/exportable.
3. Migration markers and immutable source IDs make import/reopen idempotent.
   After cutover the old table is archival; runtime/tools no longer write it.
   This is a data migration, not revival of the retired plugin-v0 system.
4. Browser localStorage mock reports are not automatically imported as genuine
   reports. If real user-authored content is discovered there, provide a separate
   explicit export/recovery procedure before clearing anything; do not invent
   source provenance for demo data.
5. Rollback preserves new data and the old table, but an old executable may not
   understand new revisions. Use the repository's supported schema-version
   checks; do not promise transparent downgrade or dual-write synchronization.
   Take a normal database backup before schema changes.

Normal export emits exact Markdown plus a JSON sidecar containing metadata,
origin, source references, and selected comments, without granting those fields
authority on re-import. No import feature is included in V1. When notebook-core
is omitted, normal notebook actions are absent; implement a read-only export
mode in the existing headless command surface backed by core Storage so content
remains recoverable without loading notebook/report execution modules. This is
a narrow proposed export facility, not an assumed existing artifact API.

## 12. Verification and delivery boundaries

The following are design acceptance cases, not completed tests:

| Boundary | Required evidence |
| --- | --- |
| Ordinary context | Seed distinctive legacy/new/report text; exercise normal and resumed turns; prove no store enumeration or automatic injection |
| Explicit access | Tool read returns requested revision; unrelated sections stay absent; no automatic memory ingestion of report-derived content |
| Persistence | SQLite/PostgreSQL fresh install, upgrades, reopen, migration rollback, legacy preservation, scope parity |
| CAS/idempotency | Two editors; lost save acknowledgement; same-key changed input; regeneration racing an edit/delete/adopt |
| Feedback | Edited/resolved comments before and after admission; inherited feedback; duplicates; omitted over-budget comments; exact frozen input |
| Authorization | Human save in normal mode; Agent write approval; forged actor/scope; missing session; explicit policy deny; foreign IDs |
| Module selection | Full; notebook-only; no-UI; no-reports; no-notebook; invalid missing dependency; physical omission and Inspect |
| Report restrictions | Malicious comments/model output cannot call tools, memory/persona writes, alternate graph nodes, or external delivery |
| Sources | Missing dailies, partially unavailable sessions, empty activity, invalid references, edited report input, missing source version |
| Lifecycle | Duplicate cron/manual requests, model timeout, storage failure, crash before/after commit, shutdown, deactivation, DST |
| User flow | At split UI `:3015`: create section/document, save/reload, preview, comment, conflict, edit report, regenerate candidate, export |

Implementation gates: repository `just ci`, applicable storage conformance,
ActionHost/ToolHost suites, SDK verify/pack/Inspect and optional-Generation matrix,
plus the real split-UI and headless paths. Record commands/outcomes in delivery
logs. Live-provider evidence is separate from deterministic fake-model tests.
Design review does not claim any of these gates passed.

## 13. Proposed implementation surfaces

Existing files to extend or remove behavior from:

- `internal/runtime/service.go`, `prompt.go`, relevant prompt/context tests:
  delete note digest loading/injection and stale references.
- `internal/tools/notes.go`, `writenote.go`, existing tool registration: scope
  and optional-service binding while preserving the three tool schemas.
- `internal/storage/contracts.go`, paired migrations and SQLite/PostgreSQL
  implementations: notebook transactions, import, receipts, report metadata.
- `internal/modules/defaults/catalog.go`, generated-assembly compiler sources,
  Recipes and conformance: selectable internal owners and dependency edges.
- `internal/actionhost/host.go`, `internal/app/app.go`, transport admission:
  typed scoped facade, human/Agent origin, narrow policy rules and lifecycle.
- `plugins/vivy-notebook/`: keep UI Module identity, replace demo wiring, add
  edit/preview/comments/history and backend-derived capability display.
- `internal/runtime/workflow_service.go`, `inofy_executor.go`, cron scheduler,
  Run/provenance storage and observer projections: #5-only report admission,
  execution binding, exclusions, and scheduling.

Proposed new paths: `internal/notebookcontract/`,
`internal/modules/notebook/`, `internal/modules/reports/`,
`internal/storage/notebook.go`, and `internal/runtime/report_*.go`.
Use existing package conventions at implementation; do not hand-edit generated
Assembly files or leak runtime/Eino types into public plugins.

Instruction cleanup is proposed, not performed: `.agents/skills/vivy-plugin`
currently lists notes as a universally selected tool. The owner's confirmed
optional-notebook requirement supplies a real variation boundary. Update that
specific obsolete example only when repository-instruction changes are explicitly
authorized; its default must not silently override this product requirement.

## 14. Delivery decomposition and readiness

This section defines architectural delivery seams. The owner subsequently
requested the implementation package; its authoritative status/dependency index
is [notebook-reports/index.md](../plans/notebook-reports/index.md). No implementation
or runtime acceptance follows from the existence of those plans.

| Increment | Architectural outcome (execution dependencies live in the index) |
| --- | --- |
| N0 | Remove legacy automatic injection, retain data |
| N1 | Scoped notebook persistence, CAS, receipts, legacy import and offline export |
| N2 | Selectable module, actions, human-edit policy, optional note tools and headless operations |
| N3 | Real notebook editor, sections, comments, revision UX |
| R0 | Trusted report Run purpose and constrained root-workflow admission |
| R1 | Manual daily/weekly/monthly workflow and provenance |
| R2 | Report controls, edited-report candidate UX, feedback integration |
| R3 | Typed cron dispatch, recovery and deactivation |

Notebook release: N0-N3, with no report-runtime dependency. Report release:
R0-R3; R2 and R3 are logically independent after their prerequisites, but may
share files and should not be delegated concurrently without explicit ownership.
Critical cross-cutting reviews are scope/policy (N2), trusted provenance (R0),
and atomic report publication/recovery (R1/R3).

The package resolves generated-binding touch points (N2/R0), admission-origin
propagation (N2), both cognitive consumers (N2/R0), Storage/Journal reconciliation
(R1/R3), and the pinned Diva reference (section 16). Ready status still requires
execution authorization, accepted predecessor evidence and resolved environment
gates; written plans are not completed integrations.

## 15. Decision record and economy

Built-in Storage with Markdown bodies and immutable revisions is the least-cost
coherent choice for confirmed editing, provenance, and concurrency needs. A
Markdown directory authority would add file watchers, external conflict rules,
and metadata reconciliation without a current consumer; two backends would
duplicate that work. Reuse the existing UI frame, Markdown packages, actions,
provider model, INOFY ledger, and cron; add only the scoped content model and
report authority bridges they do not provide. Delete old automatic digest reads
and mock runtime wiring as explicit deliverables.

Revision snapshots grow with edits; this is an accepted V1 storage cost for
recoverability, bounded per revision but without an invented total quota or
premature compactor. Explicit Save avoids per-keystroke revisions. Lists use
indexed cursor pagination; no speculative cache or search service is added.
Regular chat removes one legacy notes read and digest construction; no latency
improvement is claimed without measurement. Reporting is on-demand/background
work and never participates in the ordinary turn's context assembly.

The implementation package preserves the scope/authorization behavior, candidate
revision policy, comment inheritance, deletion/export semantics, and notebook-first
release boundary. Plan review does not certify implementation, tests, or deployment.

## 16. Planning elaborations and pinned reference

These elaborations resolve implementation choices within the existing design;
they do not add user-facing capabilities. Exact produced interfaces belong to
their owning Story and are linked by consumers rather than duplicated.

- **N1 owns notebook DTOs and transaction contracts.** N2 owns the authenticated
  module facade and N3 consumes its wire schemas. The current 256 KiB body,
  16 KiB comment and 100-row limits remain; the SDK/action Host already support
  a 1 MiB default wire limit. Both raw-content and serialized-wire limits apply.
- **N2 owns explicit-read provenance.** Extend durable ToolOperation/message
  metadata and an ingestion-exclusion projection, preserving it through replay,
  compaction and any history actually reused. Exclude a derived turn from
  automatic cognitive capture when reliable segment separation is unavailable.
  Do not infer provenance from user text or alter ordinary chat output.
- **R0 owns root-report admission.** Use one hidden control Session per notebook
  scope as the storage/session lock owner, with no synthetic user turn or Agent
  parent. A report Run remains `kind=workflow`, `purpose=report`, `depth=0`,
  `parent_id` empty and `root_id` equal to itself. Hidden control Sessions are
  excluded from conversational lists/source collection but included in recovery.
  Extend the existing workflow revision schema to admit this precise root case;
  retain ordinary child lineage checks. No self-parent or fabricated primary Run.
- **R0 owns report request identity.** Add a workflow admission namespace separate
  from lineage: ordinary children retain parent-based deduplication; root reports
  use their scope/control-Session namespace. A stored canonical *request* digest
  detects same-key payload changes. Dynamic source/feedback snapshots do not
  change the digest of a retried request: a retry rejoins the admitted Run first.
- **R1 owns publication.** One Storage transaction writes revision, generation
  provenance and output receipt, conditionally advances the head, and releases
  no Run ownership prematurely. A repeated persist node returns that receipt;
  INOFY owns its subsequent node/terminal commit. Lost acknowledgement therefore
  requires reconciliation, not a second generated version.
- **R1 owns settings schema/defaults; R3 owns editing and dispatch.** One existing
  CronJob per scope/period stores the report
  configuration, including `enabled=false` for manual-only use. Add row revision
  and typed report payload fields there; do not maintain a duplicate config file
  or report-settings database. R1 creates validated manual defaults through the
  same contract; R3 adds schedule dispatch and editing.

The old #5 main-branch Rust links are stale: DIVA main at
`518a33ef09858ee1bb190579dd7529aceaa15dd6` is the Go host tree. The source reference
actually inspected for this package is DIVA `dev` at
[`c565bb245cc920258d7f8c7fcd9544fbba545af7`](https://github.com/ProjectViVy/agent-diva/tree/c565bb245cc920258d7f8c7fcd9544fbba545af7).
Inspected paths: `agent-diva-core/src/reports/{generator,fact_bundle,validate,period}.rs`
and `agent-diva-autodream/src/{rhythm,monthly}.rs`. It demonstrates bounded fact
bundles, categorized narrative failures, reference validation, daily aggregation
and missing-date session fallback. Its fact bundle imports evolution evidence
types, uses file storage, and treats no activity as an error in some paths. Do
not port those packaging/behavior choices: this design owns independent report
evidence types, core Storage, explicit empty outcomes and local-time windows.
