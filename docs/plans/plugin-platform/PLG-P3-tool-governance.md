# PLG-P3 Tool Governance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> `superpowers:subagent-driven-development` (recommended) or
> `superpowers:executing-plans` to implement this plan task-by-task. Steps use
> checkbox (`- [ ]`) syntax for tracking.

**Goal:** Route protected internal, public static, public dynamic, and
MCP-derived Tools through one ToolHost with deterministic Middleware, Policy,
approval, result, Observer, and Inspect semantics.

**Architecture:** `internal/toolhost` becomes the sole catalog and dispatcher.
Focused SDK packages expose Tool, ToolWorld, pre-tool Middleware, Observer, and
Status Ports; the existing Eino Tool adapter consumes only the governed
ToolHost view inside `internal/runtime`.

**Tech Stack:** Go, Eino v0.9.13 Tool/ADK middleware adapters, JSON Schema,
Journal projections, `just ci`.

**Spec:** `docs/architecture/VIVY-PORT-CATALOG.md` sections 3–4 and 9–10.

## Global Constraints

- State: `UNSCHEDULED`; depends on P2.
- Protected Tool implementation stays T1 but uses `std/tool@v1` like every
  other Tool.
- No fast path, recursive ToolHost call, model-visible bypass, or second Policy
  evaluation stack.
- Middleware failure is fail-closed; Observer failure is isolated.

---

### Task 1: Establish one ToolHost catalog

**Files:**

- Create: `internal/toolhost/catalog.go`
- Create: `internal/toolhost/catalog_test.go`
- Create: `internal/toolhost/host.go`
- Create: `internal/toolhost/host_test.go`
- Modify: `internal/runtime/tooladapter.go`
- Modify: `internal/runtime/enhanced_tooladapter.go`
- Modify: `internal/app/app.go`

**Interfaces:**

- Consumes: typed static Tool and ToolWorld Providers from the Runtime Assembly.
- Produces: `toolhost.Host.Lookup`, `ListVisible`, `Discover`, and `Invoke`.

```go
type Host interface {
    ListVisible(context.Context, Scope) ([]tool.Definition, error)
    Invoke(context.Context, Request) (tool.Result, error)
}
```

- [ ] Write `TestEveryToolPathUsesHostInvoke`; expected RED is that built-in and
  plugin paths enter different adapters.
- [ ] Write duplicate static/dynamic ID, discovery timeout, and schema-hash
  change tests.
- [ ] Implement one catalog with deterministic namespaces and dynamic cache
  invalidation.
- [ ] Adapt Eino `components/tool` only in `internal/runtime`; cite the concrete
  pinned API in the iteration verification.
- [ ] Remove direct Tool execution and duplicate registries.
- [ ] Run `go test ./internal/toolhost ./internal/runtime -run Tool`.
- [ ] Commit `refactor(tool): centralize toolhost dispatch`.

### Task 2: Reserve and register protected internal Tools

**Files:**

- Create: `internal/toolhost/protected.go`
- Create: `internal/toolhost/protected_test.go`
- Modify: `internal/modules/defaults/catalog.go`
- Modify: constructors under `internal/tools/` for the protected set only.

**Interfaces:**

- Consumes: the 11 reserved IDs in the Port Catalog.
- Produces: T1 `std/tool@v1` Providers and compiler-visible reserved IDs.

- [ ] Write table tests asserting all 11 exact IDs are T1, default-on, and
  rejected from any T2 source.
- [ ] Write `TestMinimalRecipeCanOmitButCannotReplaceProtectedTool`.
- [ ] Convert each protected Tool constructor to the focused SDK contract
  without changing behavior.
- [ ] Make source/Trust, not a second Port, distinguish internal Tools.
- [ ] Run existing focused tests for filesystem, patch, multiedit, bash,
  ask-user, and skills Tools.
- [ ] Commit `refactor(tool): protect core tool identities`.

### Task 3: Implement `net.client` and scoped Host facades

**Files:**

- Create: `sdk/module/grant.go`
- Create: `internal/modulehost/facade.go`
- Create: `internal/modulehost/facade_test.go`
- Modify: `sdk/internal/assembly/grants.go`

**Interfaces:**

- Consumes: effective Grant records from the sealed Assembly.
- Produces: workspace-, identity-, instance-, and egress-scoped Host facades.

- [ ] Write RED tests for path escape, undeclared Secret enumeration,
  unapproved process spawn, HTTP host/scheme/port escape, and `rpc.client`
  misuse as general network.
- [ ] Implement scoped capabilities without handing out raw Journal, Policy,
  DB, Eino, or credential-store handles.
- [ ] Preserve error cause chains while redacting Secret content.
- [ ] Run `go test ./internal/modulehost ./sdk/internal/assembly -run Grant`.
- [ ] Commit `feat(plugin): enforce scoped module host grants`.

### Task 4: Add deterministic pre-tool Middleware

**Files:**

- Create: `sdk/port/pretool/pretool.go`
- Create: `sdk/port/pretool/pretool_test.go`
- Create: `internal/toolhost/middleware.go`
- Create: `internal/toolhost/middleware_test.go`
- Modify: `internal/runtime/tooladapter.go`

**Interfaces:**

- Consumes: Recipe-ordered `pretool.Provider` values.
- Produces: typed `Pass`, `Deny`, `RequireApproval`, or `RewriteArgs` decisions.

- [ ] Write `TestRewriteRevalidatesSchemaPolicyAndGrant`; expected RED proves the
  current path cannot consume a public ordered chain.
- [ ] Write tests for identity rewrite, recursive execution, timeout, panic,
  invalid decision, and multiple rewrites.
- [ ] Execute the exact Port Catalog order and fail closed on every Middleware
  failure.
- [ ] Keep Kernel Policy as final authority even when Middleware passes.
- [ ] Run `go test ./internal/toolhost -run Middleware`.
- [ ] Commit `feat(tool): add governed pre-tool middleware`.

### Task 5: Split reliable and diagnostic Observers

**Files:**

- Create: `sdk/port/observer/observer.go`
- Create: `sdk/port/observer/observer_test.go`
- Create: `internal/observerhost/host.go`
- Create: `internal/observerhost/host_test.go`
- Modify: `internal/runtime/audit.go`
- Modify: `internal/app/lsp_status.go`

**Interfaces:**

- Consumes: post-commit redacted Run projections and bounded diagnostics.
- Produces: at-least-once Run delivery with cursors and best-effort diagnostic
  delivery with drop counters.

- [ ] Write RED tests proving a Run Observer sees only committed events and can
  receive a duplicate stable event ID.
- [ ] Write tests proving diagnostic overload increments a visible drop counter
  without blocking a Run.
- [ ] Remove mutation-capable Observer callbacks and old optional type
  assertions.
- [ ] Ensure Observer failure cannot roll back or change Tool results.
- [ ] Run `go test ./internal/observerhost ./internal/runtime -run Observer`.
- [ ] Commit `feat(observer): separate run and diagnostic delivery`.

### Task 6: Add read-only StatusHost

**Files:**

- Create: `sdk/port/status/status.go`
- Create: `internal/statushost/host.go`
- Create: `internal/statushost/host_test.go`
- Modify: `internal/app/lsp_status.go`
- Modify: `internal/rpc/control.go`

**Interfaces:**

- Consumes: namespaced `status.Provider` snapshots.
- Produces: bounded, deadline-limited, redacted status through existing Control
  RPC projections.

- [ ] Write `TestStatusReadDoesNotStartOrProbeProvider`; expected RED is any
  lifecycle effect caused by inspection.
- [ ] Implement per-provider timeout, count bound, namespace, and unavailable
  projection.
- [ ] Convert LSP status directly to `std/status-source@v1`.
- [ ] Run `go test ./internal/statushost ./internal/app ./internal/rpc -run Status`.
- [ ] Commit `feat(status): host read-only module status`.

### Task 7: Prove the complete Tool envelope

**Files:**

- Create: `internal/toolhost/conformance_test.go`
- Modify: `internal/runtime/tooladapter_test.go`
- Modify: `internal/runtime/approval_test.go`
- Modify: `internal/runtime/audit_test.go`

**Interfaces:**

- Consumes: static, protected, dynamic, and fake MCP-derived Providers.
- Produces: Gate B Tool governance evidence.

- [ ] Run the same request through all four source classes and assert identical
  schema, Policy, Middleware, approval, execution, result-bound, and Journal
  stages.
- [ ] Assert a rewritten request cannot reuse an approval for old arguments.
- [ ] Assert Secrets are absent from error and Observer payloads.
- [ ] Run `go test ./internal/toolhost ./internal/runtime ./internal/app`.
- [ ] Run `just ci` and record the Tool envelope trace.
- [ ] Commit `test(tool): prove one governed execution envelope`.

## Phase exit and rollback

Exit requires one ToolHost for every source, all protected names enforced,
deterministic Middleware, separated Observers, read-only Status, and no Eino
import outside quarantine. Rollback uses the prior sealed Generation; it never
restores v0.
