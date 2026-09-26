# Memory G0 mutation authority and readiness revalidation

MEM-0D (G0) research note for issue #33. Closes spec gaps GAP-C (mutation
authority under the Port + Grant model) and GAP-D (revalidation of the
pinned upstream surfaces: Eino/EinoExt at the versions in `go.mod`, and
the current Laputa surface at the MEM-0A pin). Evidence = inspected file
lines at the pinned revision; anything not directly inspected is marked
*inferred*.

Inputs: `docs/superpowers/specs/2026-09-26-memory-providers-design.md`
(§Gaps), `docs/research/2026-09-26-memory-upstream-pin.md` (MEM-0A),
`docs/architecture/SCX-PLUGIN-INTEGRATION.md` (capability rulings),
`docs/architecture/VIVY-MODULE-STANDARD.md` (trust levels T0–T3,
Grant rules).

## Task 1 — Mutation authority matrix under the Port + Grant model

### Surfaces actually inspected

- **Construct-time Host is identity-only.** `module.Host` =
  `ModuleID() string` (`sdk/module/runtime.go`); the generated assembly
  wires `assemblyHosts.ForModule(id)` → `assemblyHost`, a `string` with
  only `ModuleID()` (`internal/app/app.go:168-174`,
  `internal/generated/assembly/zz_default.go`). No module — T1 included —
  receives an I/O Host at Construct.
- **Grant vocabulary (closed set)** at `sdk/module/descriptor.go:74-87`:
  `fs.read`, `fs.write`, `channel.poll`, `channel.webhook`,
  `channel.listen`, `channel.a2a`, `secret.read`, `proc.spawn`, `tty`,
  `argv`, `rpc.client`, `net.client`.
- **Effective Grants** = requested ∩ Port allow-list ∪ across all Ports
  the module provides ∩ Trust ceiling ∩ Recipe approval
  (`sdk/internal/assembly/grants.go:23-76`, union in
  `compiler.go:523-540` `allowedGrants`). Grants are *module-wide*, not
  per-Port: `ProviderSet.EffectiveGrants`
  (`runtime_generate.go:285`) carries the union to every provider.
- **Per-Port allow-list ceilings** (`sdk/port/catalog.go:40-61`):

  | Port | Allowed Grants |
  | --- | --- |
  | `std/control-action@v1` | `rpc.client` only |
  | `std/context-source@v1` | `fs.read`, `secret.read`, `rpc.client`, `net.client` |
  | `std/observer/run@v1`, `std/observer/diagnostic@v1` | `fs.write`, `rpc.client`, `net.client` |
  | `std/status-source@v1` | `rpc.client` |
  | `std/skill-source@v1` | `fs.read`, `rpc.client`, `net.client` |
  | `std/tool@v1`, `std/tool-world@v1` | full set incl. `fs.write`, `proc.spawn`, `net.client` |

- **`controlaction.Host` ops** (`sdk/port/controlaction/action.go:169-186`;
  concrete impl `internal/actionhost/host.go` `providerHost`, methods at
  ~lines 1419-1666): `ModuleID`, `Grant`, `HasGrant`, `Secret`,
  `Settings`, `StartRun`, `InvokeTool`. **No fs op, no net op, no
  CallRPC/Do.** `Secret` enforces `secret.read`; `StartRun` requires the
  caller session (`RunRequest.SessionID` must equal the authenticated
  identity's session); `InvokeTool` enforces the provider's
  `AllowedTools` and forwards through the ToolHost.
- **The only Host file-write surface** is `toolworld.Host`
  (`sdk/port/toolworld/toolworld.go:79-85`: `Workspace()`, `OpenRead`,
  `OpenWrite`, `Spawn`) — workspace-rooted, `roots` constraints,
  FileVersionRecorder stale-write detection + atomic rename
  (`internal/app/assembly_worlds.go:161-388`). Its semantics target
  user-visible workspace files (versioned, stale-checked), not a
  module-private database.
- **A scoped generic Host facade exists but is unwired.**
  `internal/modulehost/facade.go` (`Facade` with `OpenRead`/`OpenWrite`/
  `Secret`/`Spawn`/`Do`/`CallRPC`, GrantBinding constraint enforcement)
  is imported by no package outside itself (grep over
  `agent-vivy/internal/modulehost`); committed in `a44e1695` as
  reference surface.
- **Approval path fails closed.** `RequiresApproval`/`ApprovalRequired`
  actions always return `ErrApprovalRequired`
  (`internal/app/app.go:766-778` — "no action-specific approval
  row/continuation route"; `internal/actionhost/host.go:1740`).
- **Other public Port Provider interfaces take no Host at all** —
  `contextsource.Provider.Query(ctx, Request)`,
  `observer.RunProvider.ObserveRun(ctx, ...)` wait/emit,
  `status.Provider.Status`, `skillsource.Provider.List/Get`
  (`sdk/port/*/...` Provider interfaces). Their allow-listed Grants are
  conformance contracts enforced by T2 review/evidence, not by a runtime
  Host op on the Port ABI.
- **Only channel Hosts get network ops** (`channel.Host` exposes
  `HTTP()`/`DialTLS`); `std/tool@v1`'s Host is only
  `InvokeTool` (`sdk/port/tool/tool.go:30-33`). No focused Host on
  control-action / context-source / observer exposes a network call.

### Mutation authority matrix

Mutations = closed set from the spec: `remember`, `correct`, `forget`,
`configure`, provider-internal `maintenance` (extraction/compaction/
vacuum jobs). "Route" names the authorized mechanism; `fit` means an
existing authorized route exists; `gap:<name>` names the smallest
missing capability.

| Mutation | First-party BML (T1 `vivy/memory-bml`) | Remote provider (mem0 class, T2 adapter) |
| --- | --- | --- |
| `remember` / `correct` / `forget` (local durable write) | **fit** — T1 module authority: the durable store is opened at composition from `config.Config`, same pattern as `internal/modules/storage.Open(ctx, cfg)` (`internal/modules/storage/module.go:26-53`; `MkdirAll` of the storage dir + `sqlite.Open`). BML owns its own scoped file/dir (design decision 6), never `data/vivy.db`/Journal. The action provider on `std/control-action@v1` is a thin router into the in-process store; it never exercises a fs Host op because none exists. | **gap:action-host-network-op** — the mutation's actual effect is a call to the remote service. `std/control-action@v1` allow-lists only `rpc.client`, and even that Grant has no op on `controlaction.Host` (no `CallRPC`/`Do`); the `InvokeTool` bridge only reaches ToolHost tools whose Host is likewise `InvokeTool`-only (`sdk/port/tool/tool.go:30-33`). T2 source-verified modules cannot self-dial raw `net`/`net/http` (source firewall), so there is no authorized network egress from inside an action today. |
| `configure` (provider settings, credentials) | **fit** — `Host.Settings()` is a read-only host-owned projection; provider-owned config persists under the same T1-owned data dir as above (review-enforced). Credential material routes through `Host.Secret` (`secret.read` Grant, enforced + redaction-scanned at `internal/actionhost/host.go:~1540-1590`). | **gap:action-host-network-op** (same) for persisting/applying remote-side config; reading a stored credential fits via `Host.Secret`. |
| Approval-required mutation (e.g. destructive `forget` if a Recipe/policy wants human approval) | **gap:action-approval-continuation** — `RequiresApproval` ⇒ always `ErrApprovalRequired` (`internal/app/app.go:766-778`, `internal/actionhost/host.go:1740`); no approval row/route exists. If the memory profile never marks actions approval-required, this is moot; if any mutation wants a human gate, the continuation route is missing. | same. |
| `maintenance` (extraction/compaction/vacuum jobs) | **fit** — the T1 module owns its job loop under its Instance lifecycle (`Start`/`Stop`); no Host scheduling op needed. Session-scoped heavier work can use `StartRun` (same-session only) or the observer/commit path noted in SCX row 28. | **fit** for provider-scheduled remote jobs via its own service — but *reaching* that service hits the same network gap above. |

### Named gaps (not workarounds)

1. **`gap:action-host-network-op`** — no focused Port Host exposes a
   constrained network/RPC operation to non-channel providers
   (`controlaction.Host`, `tool.Host`, `contextsource`/`observer`/
   `status` Providers take no Host). `rpc.client`/`net.client` Grants
   exist in the vocabulary and ceilings but have no enforcement surface
   on these Hosts; the intended enforcement surface
   (`internal/modulehost/facade.go` `Do`/`CallRPC`) is built but unwired.
   Smallest reviewed fix: extend `controlaction.Host` (or a dedicated
   memory Host) with a constrained `Do`/`CallRPC` backed by
   `modulehost.Facade`, ceiling `rpc.client`/`net.client` accordingly.
2. **`gap:module-private-fs`** — the only file-write Host op is
   `toolworld.Host.OpenWrite`, workspace-rooted with stale-write/version
   semantics for user files. No Host grants a provider a private,
   non-workspace fs root. For BML this is moot under the T1 route
   (composition wiring); it binds only a *T2* provider wanting local
   durable state — which the issue forbids anyway ("T2 control actions
   do not gain raw local database writes"). Recorded as a named gap so
   the profile can state it explicitly rather than imply a route.
3. **`gap:action-approval-continuation`** — approval-required actions
   fail closed with `ErrApprovalRequired`; no action-specific approval
   row/continuation route exists (`internal/app/app.go:771-778`).

### Module placement decision (T1 vs T2)

**Decision: first-party BML lands as a T1 repository module
(`vivy/memory-bml`), not a T2 plugin.** Consequences per the issue's
ownership rule:

- T1 authority is conferred by the repository + composition wiring
  (precedent: `internal/modules/storage` opens its engine from
  `config.Config`, not through any Port Host). BML's scoped data dir is
  therefore wired like the storage path — it does **not** need, and does
  not get, a `fs.write`-on-action-Host route. Note this corrects the
  plan's phrasing "scoped `fs.write` Grant": `fs.write` is not in
  `std/control-action@v1`'s allow-list and the action Host has no fs op
  to spend it on — the T1 route makes that irrelevant.
- T2 control-action providers "do not gain raw local database writes
  by implementing this profile" (issue #33) — verified: no Host op on
  the Port ABI could grant it even if allowed-listed.
- External (mem0-class) providers as T2 modules need a network route for
  mutations: blocked on `gap:action-host-network-op` unless mutations go
  through an external service path *outside* the action Host (e.g. the
  provider module's own T2 lifecycle goroutines — governance contract,
  no runtime enforcement) or the named gap is closed.
- SCX row 23 stands: no public memory/index Port; durable kernel storage
  stays behind `core/storage-engine@v1`. BML's provider-owned dir does
  not violate it — it is provider data, not Journal/Run state.

## Task 2 — Eino / EinoExt readiness at pinned versions

Pinned (`go.mod:13-16`): `eino v0.9.13`,
`eino-ext/components/model/claude v0.1.25`,
`eino-ext/components/model/openai v0.1.13`,
`eino-ext/components/tool/mcp v0.0.9`. Verified: the issue's cited
`v0.9.13` is still exact. Inspection via source clones at the pinned
tags (no Go module cache on this box).

| Surface needed by memory | Eino at v0.9.13 | Verdict |
| --- | --- | --- |
| Recall: retriever contract | `components/retriever/interface.go:48-49` — `Retrieve(ctx, query, opts...) ([]*schema.Document, error)`; pluggable `TopK`/`ScoreThreshold`/extra via `Options` | **reused API** — host-side adapter only (Eino confined to `internal/runtime`/`internal/provider`); Vivy keeps its `contextsource.Provider` ABI. Missing memory semantics (below). |
| Write/index side | `components/indexer/interface.go:38-40` — `Store(ctx, docs, opts...) (ids, err)` only | **gap:indexer-mutation-semantics** — no `Delete`/`Update`/tombstone; `correct`/`forget` cannot map onto `Indexer`. Vivy writes/deletes must ride its own store contract (or direct provider SDK). |
| Document model | `schema.Document.MetaData map[string]any` (`schema/document.go:40-46`) | **missing semantics** — no scope/tenant, revision, provenance, confidence, or tombstone fields; all of REQ-MEM's evidence/revision vocabulary would be convention inside `MetaData`, not contract. |
| Extraction chunking | `components/document/interface.go` — `Loader` (`Source{URI}`), `Transformer` | **reused API** — usable host-side for session-log → chunks; not required. |
| Orchestration | `compose.Chain.AppendRetriever` (`compose/chain.go:284-297`), retriever nodes in graphs/parallel | **reused API** — host-side only. |
| Extraction/compaction jobs | `adk/middlewares/` (summarization, reduction, skill, plantask, …) | **missing semantics** — these are in-loop context shaping (token-count-triggered compression), not completed-session extraction with durability/audit; GAP-B/MEM-0C owns the session-extraction task contract. |
| Vector/kv stores | eino-ext has retriever+indexer impls (milvus, qdrant, es7/8/9, redis, opensearch, volc…) — **none pinned in `go.mod`** | **migration boundary** — adopting one is a new dependency, importable only inside the quarantine dirs; adapters must convert to/from Vivy domain types at the boundary so swap-out when upstream adds delete/scope semantics stays local. |

**Eino verdict summary:** reusable for recall orchestration and document
pipelines *inside* `internal/runtime`/`internal/provider`; it supplies
no memory-domain semantics (delete/forget, scope, revision, provenance)
and no completed-session extraction primitive. Vivy's memory profile
contracts stay Vivy-owned.

## Task 3 — Laputa readiness at pin `1e402835` (rescoped)

MEM-0A recorded that `laputa/governance/governed.go` /
`GovernedService.Mutate` no longer exist upstream; `laputa/governance/`
is docs-only and `architectureguard/scanner.go:134-168` forbids
`NewGovernedService`/`WorldProjector`/`WorldClaim`/`v2/governance`.
**The issue's Mutate-state-before-audit defect is moot** — the engine is
deleted, not buggy. The surface actually present at the pin:

- `laputa/persona` — `Service` over 7 Markdown authority files
  (IDENTITY/RELATIONSHIP/REDLINE/USER/WORLD/DREAM/DARK) + `history/` +
  `requests/` queue (`service.go` 1368 lines, `types.go` 520 lines).
- `laputa/actmem` — companion activity-memory package.
- Module path is `github.com/dashimaki/laputa`, not
  `github.com/ProjectViVy/laputa` — embedding it requires an upstream
  module-path change or a `replace` (inferred consequence from
  MEM-0A's pin table).

Readiness of `persona.Service` against the issue's production-authority
properties:

| Property | Evidence at `1e402835` | Met? |
| --- | --- | --- |
| Atomicity | `persistRevisionLocked` (`service.go:895-939`): immutable snapshot → diff → append+fsync `log.jsonl` → *then* atomic rename of live doc — history before document (opposite of the retired defect). Crash between ⇒ `history_mismatch` detected by `fileStateLocked` (`service.go:154-159`) → StatusIncomplete → Repair. | mostly |
| Recoverability | history = real content snapshots (`ReadHistory` returns content, `service.go:235-257`; `writeImmutable` O_EXCL+fsync `service.go:1089-1109`); rollback-by-rewrite is possible — the retired hash-only `RollbackRef` defect does not exist. | yes |
| Durable audit | per-kind `log.jsonl`, append+fsync, entries carry actor/source/reason/base_revision (`types.go:240-250`). File-local, not tamper-evident. | yes (scope-limited) |
| Actor mapping | `actor` is a trusted string param — "HTTP adapters derive it from capability claims" (`service.go:619-621`); the service itself does not authenticate. In-process embedding inherits caller trust. | yes (with caveat) |
| Idempotency | revision CAS (`baseRevision`, `RevisionConflictError` `service.go:846-848`), content-hash no-op dedup (`service.go:849-852`), one pending request per kind (`RequestExistsError`, `service.go:657-661`), stale detection on drift (`service.go:728-736`), `stalePendingLocked` backs up+restores request records on failure (`service.go:941-968`). | yes |

Residual defects:

1. **Request-accept commit is not atomic.** `AcceptRequest`
   (`service.go:711-757`) writes the doc first, then `saveRequestLocked`
   — a crash between leaves doc applied but request still pending
   (self-heals to `stale` on retry via revision mismatch, so it
   converges, but it is not a single commit boundary).
2. **No directory fsync after rename.** `writeAtomicBytes`
   (`service.go:1111-1122`) fsyncs the temp file then renames but never
   fsyncs the parent directory — the rename itself may not survive a
   crash.
3. **Concurrency is single-process.** One `sync.Mutex` per Service
   (`service.go:22`); no inter-process lock — a second process sharing
   the persona dir races unsynchronized. For Vivy embedding this is an
   integration constraint (single writer per dir), not a code defect.
4. Module namespace `github.com/dashimaki/laputa` ≠ repo org (above).

**Laputa verdict: `gap:persona-commit-boundary`** — the current persona
surface satisfies recoverability, audit, idempotency, and ordering, but
is not yet `fit` for production authority: needs upstream changes in
`laputa/persona/service.go` — (a) atomic doc+request commit in
`AcceptRequest`, (b) parent-dir fsync in `writeAtomicBytes`, (c) a
documented single-writer posture (or flock). Plus the module-path
question (`dashimaki` vs `ProjectViVy`) before any Go import.

## Downstream notes for MEM-0E / MEM-1

- Memory mutations for the first-party provider do not need new Host
  ops: T1 composition wiring covers local durability. The memory
  profile's control-action set can assume `Grant`/`Secret`/`Settings`/
  `StartRun`/`InvokeTool` only.
- Any remote-provider write path inside a control action is blocked on
  `gap:action-host-network-op`; MEM-0E should either scope remote
  providers to read/suggest actions until the gap is closed, or record
  the dependency explicitly.
- Do not mark `forget`/`configure` approval-required in the profile
  unless `gap:action-approval-continuation` is closed first.
- Laputa persona integration (REQ-MEM-11) is viable after the named
  `persona-commit-boundary` fixes; Vivy must be the sole process writer
  and supply `actor` from its own authenticated identity.
