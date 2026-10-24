# Pluggable memory providers — design

Status: planning baseline for issue #33 (PROPOSAL/UNSCHEDULED → G0 contract freeze).
Issue: https://github.com/ProjectViVy/agent-vivy/issues/33
Code baseline: `f6fb11bc` (main, worktree `agent-vivy-memory`, branch `feat/memory`).
Upstream reference baseline: `ProjectViVy/agent-diva@0fd005a1` (local clone
`~/reference/agent-diva`); `ProjectViVy/laputa` revision pending (MEM-0A pins it).

## Value and scope

Vivy gains selectable long-term memory without coupling Runtime, persona, UI, or
deployment to one memory framework. The first release proves the contract twice:
one first-party backend (BML, ported from Agent Diva's `agent-diva-laputa/src/bml`)
and one remote backend (pinned mem0). Everything else lands only when its
prerequisite contracts exist.

Out of scope for the first release (verbatim from the issue): runtime loading of
arbitrary Go/UI plugins, multi-provider write fan-out, a universal vector
database interface, reimplemented third-party engines, generalizing Garden to
every backend, self-modification, and replacing Vivy core persistence/runtime/
policy/approval.

## Requirements

| ID | Requirement | Issue source |
| --- | --- | --- |
| REQ-MEM-1 | Memory is a standardized capability bundle and conformance profile over existing Ports — no new universal database API, no mandatory MemoryHost. | Port mapping |
| REQ-MEM-2 | Bounded recall: `std/context-source@v1` returns scope-checked candidates and resource references; ContextHost re-checks and Runtime owns final model input. | Data flow |
| REQ-MEM-3 | Committed experience return: `std/observer/run@v1` (receipt-aware `ReceiptRunProvider`) delivers allowed-field projections with stable event IDs and idempotent ingestion. | Data flow |
| REQ-MEM-4 | Explicit remember/correct/forget/configure via `std/control-action@v1` through ActionHost; agent-requested search/read via `std/tool@v1` through ToolHost. | Port mapping |
| REQ-MEM-5 | Health/coverage/backlog via `std/status-source@v1`; optional management UI via `std/ui-extension@v1`; reusable skills via `std/skill-source@v1`. | Port mapping |
| REQ-MEM-6 | Trusted scope covers tenant, user, agent/persona, workspace, session; Host resolves identity mapping — plugins never derive authorization from query text or user metadata. | Capability contract |
| REQ-MEM-7 | Providers declare capabilities; unsupported operations fail explicitly; provider relevance scores are never compared cross-backend; provider metadata stays namespaced. | Capability contract |
| REQ-MEM-8 | One primary write destination per scope; zero or more mounted read sources; backend switching never implies migration, merge, or erase; no activation without credentials/network. | Selection |
| REQ-MEM-9 | Delivery acceptance and extraction completion are independently inspectable; optional recall failure is visible and degrades to no recall. | Reliability |
| REQ-MEM-10 | Completed-session export is a Host-owned contract; providers never read raw Journal; expensive extraction runs as separately budgeted task/tool operations under Service/task authority. | Reliability |
| REQ-MEM-11 | Laputa governance is independent: governs identity/persona revisions only, never filesystem, network, tool, or approval authority; persona projection is a Host-controlled, versioned, explicitly selected contract. | Laputa section |
| REQ-MEM-12 | Garden stays an optional external service behind a compiled adapter; its output remains source data under ContextHost/Runtime authority; no duplicate writes and no recursive routing. | Garden section |
| REQ-MEM-13 | Sealed Generation semantics: add/remove modules = new Generation; switching bindings only at defined safe boundaries; deactivation preserves data; migration is explicit export/import. | Lifecycle |

## Existing seams (verified against this checkout)

| Capability | Port / code anchor | Verified |
| --- | --- | --- |
| Bounded recall | `sdk/port/contextsource`: `Provider.Query(ctx, Request) Page`, `Resolver.Resolve`, `Candidate`/`ResourceReference`/`Scope{TenantID,WorkspaceID,SessionID}`, `Treatment{competitive,reserved,required}`, `VersionMode`, `ErrVersionUnavailable` | yes |
| Experience return | `sdk/port/observer`: `RunProvider.ObserveRun`, `ReceiptRunProvider.ObserveRunWithReceipt` → `DeliveryReceipt{EventID,ReceiptID,State∈{accepted,pending,completed,failed}}`; `EventID=runID:seq` | yes |
| Explicit mutation | `sdk/port/controlaction`: `Provider{Definition(),Invoke(ctx,Host,input)}`, `Effect{read,write,external-effect}`, `Host.Grant/HasGrant/Secret/Settings/StartRun/InvokeTool` | yes |
| Agent search/read | `std/tool@v1` via ToolHost (schema/Policy/Grant revalidated per call) | yes |
| Status | `sdk/port/status`: `Provider` → `Snapshot{revision,cursor,items}` / `Unavailable(reason)`; read-only, no probing | yes |
| Management UI | `std/ui-extension@v1`; existing `plugins/vivy-memory` ships the `/memory` sidebar shell over `getDemoMemories()` stubs | yes |
| Skills | `std/skill-source@v1` | yes |

`plugins/vivy-memory` (v0.1.0) is UI-only: one `ui-extension` provider, no Go
Port, no Grant. Backend work lands in new modules, not by growing this shell.

## Architecture decisions

1. **Ports, not a Memory Port.** The SCX-PLUGIN-INTEGRATION ruling stands: durable
   memory/index authority stays behind T1 `vivy/storage`; no public memory/index
   Port is introduced. Every memory capability maps onto the verified Ports above.
   `internal/contexthost`, `internal/observerhost`, `internal/actionhost` keep
   validation, authorization, cursor/retry, and budget authority.
2. **Memory module layout.** First-party BML lands as a new repository Module
   (proposed `vivy/memory-bml`) providing `context-source` + `observer/run`
   (receipt-aware) + `control-action` + `status-source` Providers, plus optional
   tool/skill-source providers. The existing `vivy/memory` UI module is rewired
   off demo stubs only when a real backend is selectable.
3. **Recall authority order.** Host resolves scope + provider binding →
   ContextSource returns bounded candidates → ContextHost re-checks scope/version/
   treatment/budget → Runtime builds model input → resume replays the committed
   prepared View. No refresh on resume.
4. **Experience return.** Run terminal event commits → ObserverHost projects
   allowed fields with stable EventID → adapter records/submits idempotently →
   durable `accepted` may acknowledge delivery; `completed` tracks extraction
   separately; `pending`/`failed`/ambiguous retain the Host cursor for retry.
5. **No hidden work in recall.** ContextSource.Query never starts an agent loop
   or unbounded model work. Extraction/deep-recall jobs enter through the
   existing Service/task path after a capability check (REQ-MEM-10).
6. **Backend isolation.** Each backend owns its database and jobs. BML gets its
   own scoped file/dir under the Host-granted fs authority — it does not touch
   `data/vivy.db`, the Journal, or the notes storage. Remote backends use
   `net.client`/`rpc.client` + `secret.read` Grants only.
7. **Eino quarantine preserved.** Only `internal/runtime/` and
   `internal/provider/` may import `eino*`; a memory module consumes Vivy domain
   interfaces and the SDK Ports, never Eino types. Eino Retriever/Indexer are
   candidates for host-side composition, not for plugin ABI.

## Gaps G0 must close (from the issue, sharpened)

- **GAP-A — persona projection.** `contextsource` is data-only; a `required`
  candidate is not a system instruction. Laputa support needs a narrow
  Host-controlled, versioned, explicitly selected persona projection contract
  that preserves Runtime ownership of model-message construction. Freeze: which
  Host stage injects the projection, revision pinning per session lifecycle,
  resume semantics (committed view), and the exact failure policy
  (required-identity failure must not silently yield a different persona).
- **GAP-B — completed-session export.** Observer delivers per-event projections;
  providers like memU need whole completed sessions. Define a Host-owned export
  contract (trigger point — session ≠ run; canonical shape; idempotency ledger;
  how providers declare the need) without raw Journal access.
- **GAP-C — mutation authority under Grants.** T2 control actions do not gain
  raw local DB writes via this profile. For each mutation (remember/correct/
  forget/configure) choose: existing authorized Host operation, or external
  service route, or record the smallest reviewed capability gap. BML first-party
  must respect T1 ownership boundaries.
- **GAP-D — upstream readiness.** `GovernedService.Mutate` writes state before
  audit append and ignores the append error; `RollbackRef` is a state hash, not
  a snapshot. Pin upstream revisions and verify atomicity/recoverability/
  audit/idempotency before claiming production authority integration.

## Rejected alternatives (carried from the issue)

Mandatory Garden for all memory; copying Garden orchestration into Vivy;
collapsing memory and persona governance into one plugin; requiring every
provider to implement the full BML schema; transparent model proxy as the
TencentDB integration; plugins rewriting Vivy instruction files as the
integration mechanism.

## Stage mapping to the plan package

| Stage | Story | Outcome |
| --- | --- | --- |
| G0 | MEM-0A | Pinned upstream revisions + drift reconciliation note |
| G0 | MEM-0B | Memory capability profile + scope/identity mapping contract |
| G0 | MEM-0C | Host contracts for GAP-A and GAP-B |
| G0 | MEM-0D | Mutation authority resolution + Eino/Laputa readiness revalidation |
| G0 | MEM-0E | Tracker and architecture doc updates (TODO/DEFER/SCX cross-refs) |
| G1 | MEM-1 | BML vertical slice (Blocked until MEM-0A/0B/0D outputs land) |
| G2 | MEM-2 | One pinned remote provider (mem0) — plan deferred |
| G3 | MEM-3 | Independent Laputa — plan deferred |
| G4 | MEM-4 | Garden connector — plan deferred |
| G5 | MEM-5 | memU/TencentDB extras — plan deferred |
