# Vivy Memory Capability Profile

Status: G0 contract freeze (MEM-0B). Docs-only; no code changes are implied by
or authorized by this document.
Issue: https://github.com/ProjectViVy/agent-vivy/issues/33
Spec: [memory providers design](../superpowers/specs/2026-09-26-memory-providers-design.md)
(REQ-MEM-1/6/7/8). Upstream pins:
[2026-09-26-memory-upstream-pin](../research/2026-09-26-memory-upstream-pin.md).
Rows this profile extends live in
[SCX-PLUGIN-INTEGRATION](SCX-PLUGIN-INTEGRATION.md) §"Capability and authority
mapping".

This is a conformance profile, not a Go interface. A memory provider adapter
keeps provider-native behavior behind the existing Ports; this document fixes
the vocabulary every adapter must satisfy or refuse explicitly. Every clause
names the SDK symbol or host package that enforces it ("Seam:").

## 1. Record envelope

A memory record crosses the recall boundary as a `contextsource.Candidate`;
nothing else is added to that type for memory.

| Envelope field | Carrier | Seam |
| --- | --- | --- |
| Provider ID | `Candidate.SourceID` | `sdk/port/contextsource` `Candidate`; `internal/contexthost` `normalizeCandidate` rewrites an empty `SourceID` to `Provider.ID()` and rejects a mismatched one — the provider ID is Host-asserted, not provider-claimed |
| Record ID | `Candidate.ContentID` | `contextsource.Candidate`; bounded and validated by `internal/contexthost` `normalizeCandidate` |
| Revision | `Candidate.Version` (string label); on a reference, `ResourceReference.Version` | `contextsource.Candidate`, `ResourceReference`; a BML adapter maps `bml.MemoryEntry.Revision` (`bml/provider.go`) to a decimal string label |
| Source / evidence references | `Candidate.Resource` (`*ResourceReference`) or namespaced `Candidate.Metadata` keys | `contextsource.ResourceReference{URI,Version,MediaType,SizeHint,Scope,VersionMode,Replayable}`; resolved bytes return only through `contextsource.Resolver.Resolve` under `ResolveRequest{Reference,TenantID,SessionID,WorkspaceID,MaxBytes}` |
| Capture / effective time | `Candidate.UpdatedAt` (epoch milliseconds) | `contextsource.Candidate`; informational today — no Host comparison consumes it |
| Expiry | `Candidate.ValidUntil` (epoch milliseconds; `0` = no expiry) | `contextsource.Candidate`; `internal/contexthost` drops expired candidates against `now().UnixMilli()` and counts them in `Result.DroppedExpired`; negative values rejected by `normalizeCandidate` |
| Provider score | `Candidate.Confidence` in [0,1] | `contextsource.Candidate`; `internal/contexthost` `normalizeCandidate` rejects NaN/Inf/out-of-range |
| Treatment hint | `Candidate.Treatment` ∈ {`TreatmentCompetitive`,`TreatmentReserved`,`TreatmentRequired`} | `contextsource.Treatment`; unknown values rejected by `normalizeCandidate`; budget/overflow semantics enforced by `internal/contexthost` (`strictTreatment`, `ErrRequiredContextBudget`) |
| Namespaced provider metadata | `Candidate.Metadata` keys prefixed `<provider-id>.` | `contextsource.Candidate.Metadata`; `internal/contexthost` `normalizeCandidate` bounds the map and runs `tools.RedactSensitive` over every value |

Mutation payloads never travel in the envelope. New content, CAS base
revisions, deletion reasons, and operation outcomes live only inside
`controlaction` invoke inputs/outputs — e.g. BML's
`MemoryUpdateRequest.BaseRevision`, `MemoryRemoveRequest.Reason`, and
`MemoryCrudOutcome` (`bml/provider.go`). `Candidate` and `Metadata` are
read-side projections; a provider that needs extra mutation fields extends its
own action schema (`controlaction.Definition.InputSchema`), never the shared
envelope.

## 2. Operation states

All asynchronous memory operations report the same four-state vocabulary,
reused verbatim from `observer.DeliveryState`
(`sdk/port/observer/observer.go`):

| State | Meaning for memory operations |
| --- | --- |
| `accepted` (`DeliveryAccepted`) | Durably received; work may still be outstanding. For ingestion this is a durable delivery acknowledgement only. |
| `pending` (`DeliveryPending`) | Received but not durably recorded; the Host retains its cursor and retries. |
| `completed` (`DeliveryCompleted`) | The operation's own work finished — e.g. extraction done, not merely delivered. |
| `failed` (`DeliveryFailed`) | The operation failed; retry/disposition is a Host decision. |

Per-surface state carriers and stable operation IDs:

- **Ingestion** (`std/observer/run@v1`): `observer.ReceiptRunProvider.
  ObserveRunWithReceipt` returns `DeliveryReceipt{EventID,ReceiptID,State}`.
  `EventID{RunID,Seq}` renders `runID:seq` and is the stable idempotency key;
  `ReceiptID` identifies the delivery disposition. `internal/observerhost`
  keeps a durable per-provider cursor (`storage.SnapshotStore`), replays only
  committed Journal events, and advances the cursor only on `accepted` or
  `completed`; `pending`/`failed`/ambiguous outcomes retain the cursor and
  retry under capped backoff (`ErrDeliveryPending`, `ErrDeliveryFailed`,
  `ErrInvalidReceipt`, `ErrCursorCorrupt`).
- **Mutations** (`std/control-action@v1`): action invocations are audited by
  `internal/actionhost` per call with the authenticated identity and outcome
  (`AuditOutcome*` records); a provider that cannot complete synchronously
  reports `pending`/`accepted`/`completed`/`failed` inside its own
  `ResultSchema` payload — BML already does this with
  `MemoryCrudOutcome.Status` ∈ {`applied`,`listed`,`proposal_created`,
  `failed`}, where `applied` maps to `completed` and `proposal_created` maps
  to `pending` (`bml/provider.go` `CrudOutcomeStatus`).
- **Inspection** (`std/status-source@v1`): `status.Provider.Status` returns
  `Snapshot{Available,UnavailableReason,Revision,Cursor,Items}`; each
  `status.Item{ID,State,Message,Fields}` reports one operation/backlog entry
  with the same state vocabulary. Status reads perform no activation or probe
  (`status` package contract; `Snapshot` produced by `Unavailable(reason)`
  when the backend is down).

Delivery acceptance and extraction completion are independently inspectable
(REQ-MEM-9): a provider acknowledging `accepted` while extraction runs is
correct; reporting `completed` before extraction finishes is a contract
violation.

## 3. Capability declaration

The capability set is closed. A provider advertises exactly the subset it
implements truthfully; new capabilities require a profile revision, not a
provider-invented key.

| Capability | Surface | Declaration rule |
| --- | --- | --- |
| `recall` | `contextsource.Provider.Query` on `std/context-source@v1`; agent-initiated recall rides `std/tool@v1` through ToolHost | Advertise only if `Query` returns scope-checked candidates within the Host's timeout and bounds (`internal/contexthost` `querySource` timeout, `maxCandidates`, `sourcePageLimit`) |
| `evidence-read` | `contextsource.Resolver.Resolve` (optional companion of `Provider`) | Advertise only if references resolve to bytes under `ResolveRequest` scope/`MaxBytes`; `internal/contexthost` `resolveCandidate` re-checks `reference.Scope` against the request (`ErrResourceScope`) and exact versions (`ErrResourceVersionMismatch`) |
| `ingestion` | `observer.RunProvider` or `observer.ReceiptRunProvider` on `std/observer/run@v1` | Advertise only if the provider consumes committed, allowed-field projections idempotently by `EventID` (`internal/observerhost` cursor/retry contract) |
| `correction` | `controlaction.Provider` on `std/control-action@v1` | Advertise only if update is atomic per record — e.g. CAS on `MemoryUpdateRequest.BaseRevision` (`bml/provider.go`); blind overwrite is not `correction` |
| `deletion` | `controlaction.Provider` | Advertise only if the backend durably deletes or tombstones — e.g. `MemoryRemove` CAS (`bml/provider.go`); hiding a record from list output is banned emulation |
| `export` | Host-owned completed-session export contract (GAP-B; frozen by MEM-0C) | Declare consumption only; providers never read the raw Journal (REQ-MEM-10) |
| `exact-version` | `ResourceReference.VersionMode` = `VersionExact` | Advertise only if the backend resolves a pinned revision or returns `contextsource.ErrVersionUnavailable`; returning current bytes under an old label is a contract violation (`ResourceReference` doc comment) |
| `async-extraction` | Delivery-state split (§2) + task/run entry through `controlaction.Host.StartRun` or the Service/task path (REQ-MEM-10) | Advertise only if expensive extraction runs as separately budgeted operations; `Query` must never start unbounded model work (spec decision 5) |
| `graph` | Provider-native relational structure surfaced through ordinary `recall`/`evidence-read` | Advertise only if the backend maintains traversable relations (e.g. BML `supersedes` chains); graph results still arrive as bounded `Candidate`s under ContextHost authority |
| `skill` | `skillsource.Provider` on `std/skill-source@v1` (`List`/`Get`) | Advertise only if emitted skill content is versioned (`Summary.Version`, `SourceHash`) and loading grants no Tool/authority — `skillsource` package contract |

Truthful-advertisement rules (REQ-MEM-7):

1. A capability bit is a claim the conformance suite can exercise. If the
   backend cannot perform it, the bit stays off and the operation fails
   explicitly (§4) — never a stub that silently emulates.
2. Declaration lives in sealed Module metadata: which Providers are compiled
   into `controlaction.ProviderSet.Providers`/`AllowedIDs`, which Ports the
   Module provides, and Recipe-level binding. There is no runtime
   self-report channel a provider can inflate.
3. Optional companion interfaces declare themselves by implementation:
   `contextsource.Resolver` for `evidence-read`, `observer.ReceiptRunProvider`
   for receipt-aware `ingestion`. Absence of the companion is itself the
   declaration (`internal/contexthost` `resolveCandidate` fails with
   `ErrResourceResolver`; `internal/observerhost` detects the receipt-aware
   form).
4. Providers never widen an advertised capability's meaning — `deletion` means
   deletion, `correction` means atomic update, `exact-version` means the
   pinned revision or an error.

## 4. Error contract

Unsupported operations produce explicit errors; silent emulation is banned
(plan Global Constraints; REQ-MEM-7).

- **Not provided at all:** the provider omits the Port/companion/action.
  `std/control-action@v1` callers then hit `controlaction.ErrActionNotFound`;
  `Resolver` absence surfaces as `ErrResourceResolver` at resolve time.
- **Provided but unsupported per operation:** return the operation's explicit
  failure shape, not a fabricated success. Canonical shape: BML's
  `UnsupportedCrudOutcome(op)` → `MemoryCrudOutcome{Status: failed, Reason:
  "<op> not supported by this memory provider"}`; `ErrActmemUnavailable` /
  `ErrMemoryRulesUnavailable` on unavailable surfaces (`bml/provider.go` —
  `NopProvider` is the reference for an inert + explicit-unsupported mix).
- **Version guarantees:** `contextsource.ErrVersionUnavailable` is the only
  honest `exact-version` failure; `internal/contexthost` independently rejects
  a mismatched resolved version (`ErrResourceVersionMismatch`) and any
  URI/media/scope drift (`ErrResourceScope`).
- **Banned emulations** (non-exhaustive): deletion implemented as
  list-filtering; correction implemented as add-without-supersede;
  exact-version served as current bytes under the requested label; `pending`
  reported as `completed`; an unavailable backend reported through
  `status.Snapshot.Available=true` instead of `Unavailable(reason)`.
- Action-side denials are already explicit: `ErrGrantDenied`,
  `ErrApprovalRequired`, `ErrUnauthenticated`, `ErrActionTimeout`,
  `ErrProviderPanic`, `ErrProviderFailed` (`sdk/port/controlaction`),
  each audit-recorded by `internal/actionhost` (`denyAudit`,
  `completionAudit`).

## 5. Score isolation

`Candidate.Confidence` is a provider-local signal, not a cross-backend
currency.

- The only Host use of `Confidence` is a stable mechanical tiebreak inside a
  `Treatment` class: `internal/contexthost` sorts by `treatmentRank`
  (required < reserved < competitive) first and `Confidence` second
  (`sort.SliceStable`, host.go). Nothing normalizes, aggregates, or compares
  two providers' scores, and the sort does not establish comparability —
  scales differ per backend by construction (REQ-MEM-7).
- A provider must not treat `Confidence` as global relevance or inflate it for
  priority. Selection intent belongs to `Treatment`, which is itself bounded:
  `required`/`reserved` overflow fails the call with
  `ErrRequiredContextBudget` rather than displacing competitors silently.
- Final selection is ContextHost's and Runtime's alone: scope, version,
  redaction, dedup, expiry, and budget all run in `internal/contexthost`
  (`normalizeCandidate`, `Result.Dropped*` counters, `makeView`), and Runtime
  owns the final model-input projection (`internal/runtime`
  `contextadapter.go`). Scores never reach the model input as authority.

## 6. Trusted scope model

The memory scope tuple extends `contextsource.Scope` semantics from three
dimensions to five (REQ-MEM-6):

| Dimension | Semantics | Assignment source (Host-only) |
| --- | --- | --- |
| `tenant` | Process/organism isolation boundary — `Scope.TenantID` | `internal/runtime` `ServiceDeps.TenantID` ("process-owned isolation identity forwarded to every ContextHost request and terminal Observer projection", service.go); defaulted to `local` for the single-tenant organism |
| `user` | The authenticated human/operator principal — new dimension | Resolved by the Host from the authenticated session principal — the same trust path as `actionhost.Identity` (`internal/actionhost`), which is produced only by `AuthenticateFunc` from the opaque transport `Caller`; `WithIdentity`/`IdentityFromContext` exist for transport adapters. Never from request payloads |
| `agent`/`persona` | The agent or persona identity the memory belongs to — new dimension | Host-assigned from the sealed Generation's selected persona/agent binding (REQ-MEM-13: bindings change only at defined safe boundaries); `actionhost.Identity.Face` is the audit-side carrier on the action path. Never payload-derived |
| `workspace` | Workspace container — `Scope.WorkspaceID` | `Workspaces.Ensure` at run setup (`internal/runtime` service.go), then carried on `contexthost.Request.WorkspaceID` |
| `session` | Session container — `Scope.SessionID` | The run's session ID supplied by Service (`internal/runtime` service.go `contexthost.Request{SessionID: ...}`) |

Rules:

1. **Host-assigned only.** `contexthost.Request` scope fields are filled by
   `internal/runtime` Service from owned state (service.go: `Request{Query:
   userText, TenantID: s.deps.TenantID, SessionID: ..., WorkspaceID: ...}`).
   `actionhost` identity comes from `AuthenticateFunc`, never the `Caller`
   string ("callers cannot construct an authoritative Identity through
   Invoke"). Providers and request text contribute nothing: `Request.Query`
   and `Candidate.Metadata` are data, not authority (REQ-MEM-6).
2. **Remote mapping is Host-owned.** For remote providers the Host translates
   the trusted tuple into backend account/namespace identifiers from Module
   configuration (`controlaction.Host.Settings()` projection) and
   `secret.read`-granted credentials (`Host.Secret`, leak-checked via
   `ErrSecretLeak`). A plugin never reads identity hints out of query text or
   user-supplied metadata to choose a remote namespace.
3. **Missing mapping denies, never widens.** If no configured mapping exists
   for the resolved tuple, the scope is denied: the provider is not invoked
   for that scope. For an optional recall source this is a visible omission
   (`contexthost.Result.Failures`); for a required path it fails closed. A
   fallback to tenant-global, a default namespace, or a guessed account is a
   contract violation.
4. **Scope check is re-run at resolve.** A `ResourceReference` carries its
   issued `Scope`; `internal/contexthost` `scopeMatches` re-checks it against
   the live request on every resolve (`ErrResourceScope`), and `scopeEqual`
   rejects drift between issued and resolved references. References are not
   bearer permissions (`contextsource.Scope` doc comment).

## 7. Selection and ownership rules

From issue #33 / REQ-MEM-8, verbatim where still true:

- **One primary write destination per scope.** A scope tuple has exactly one
  backend that accepts `ingestion`, `correction`, and `deletion`.
- **Zero or more mounted read sources.** `std/context-source@v1` is
  `CardinalityMany` (`sdk/port` `PublicCatalog`); additional backends mount as
  read sources and participate in recall only.
- **No implicit migration on switch.** Switching the primary binding never
  implies migration, merge, or erase; migration is an explicit export/import
  operation (REQ-MEM-13), and the export contract is Host-owned (GAP-B /
  MEM-0C).
- **No auto-activation without credentials or network.** A Module activates
  only with its compiler-sealed effective Grants — the port ceilings are
  fixed in `sdk/port` `PublicCatalog().AllowedGrants` (context-source:
  `fs.read`, `secret.read`, `rpc.client`, `net.client`; observer/run:
  `fs.write`, `rpc.client`, `net.client`; status-source and control-action:
  `rpc.client`) and effective bindings are enforced via
  `module.GrantBinding` constraints (`sdk/module` `CloneGrantBindings`,
  `FindGrant`, `ConstraintValues`) plus Host-side `Grant`/`HasGrant` checks
  (`controlaction.Host`). An unconfigured remote backend reports
  `status.Unavailable(reason)`, not activation attempts.

Ownership boundary: backends own their databases and jobs (SCX row "External
memory may own its own database and jobs"); `data/vivy.db`, the Journal, and
notes storage stay behind T1 `vivy/storage` — no memory/index public Port
exists (SCX row "Durable memory/index authority"; spec decision 1). Mutation
authority under Grants is resolved per operation by MEM-0D (GAP-C); this
profile already bans raw local-DB writes through `std/control-action@v1`
(port ceiling: `rpc.client` only).

## 8. Conformance checklist

An adapter claiming this profile must satisfy, and a reviewer must verify:

1. Every emitted `Candidate` survives `normalizeCandidate`: valid bounded IDs,
   known `Treatment`, `Confidence` ∈ [0,1], non-negative `ValidUntil`, UTF-8
   content, namespaced `Metadata` keys.
2. `exact-version` advertised ⇒ `Resolve` returns the pinned version or
   `ErrVersionUnavailable`.
3. `deletion`/`correction` advertised ⇒ atomic per-record semantics
   (CAS or tombstone), never emulation.
4. Every unsupported operation produces the explicit failure shape of its
   surface — omitted Provider, `ErrActionNotFound`, failed outcome with
   stable reason — and is audit-visible.
5. Operation reports use only the `DeliveryState` vocabulary; `completed` is
   never reported before the operation's own work finished.
6. All five scope dimensions come from Host assignment; no payload-derived
   identity reaches a backend namespace; unmapped scope ⇒ denied.
7. `Confidence` is local-ordering only; the adapter does not depend on
   cross-backend score comparison.
8. One primary write destination per configured scope; switching leaves prior
   data untouched.
