# PLG-P7 Internal Moduleization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> `superpowers:subagent-driven-development` (recommended) or
> `superpowers:executing-plans` to implement this plan task-by-task. Steps use
> checkbox (`- [ ]`) syntax for tracking.

**Goal:** Express required and optional internal organs through the same
Descriptor/Port/Assembly mechanics without exposing Kernel authority or
changing established product behavior.

**Architecture:** T1 Source Catalog entries provide closed `core/*` Ports to
generated Assembly. Each slice extracts one constructor boundary, proves
contract parity, and leaves L0 authority in existing Service, Journal, Policy,
identity, RPC, ChannelHost, and FaceHost owners.

**Tech Stack:** Go, existing SQLite/Postgres/runtime/provider implementations,
Eino v0.9.13 quarantine, generated Assembly, `just ci`.

**Spec:** `docs/architecture/VIVY-MODULE-STANDARD.md` sections 2 and 7 and
`docs/architecture/VIVY-PORT-CATALOG.md` section 13.

## Global Constraints

- State: `UNSCHEDULED`; depends on P2–P5. In particular, Task 2 consumes the
  P5 ModelHost, so P5 is not optional for phase completion.
- Migrate one internal Port per focused commit; physical directory moves occur
  only after semantic wiring is green.
- Public Recipes cannot provide, override, or configure `core/*` authority.
- Do not expose Eino types to non-quarantine packages.

---

### Task 1: Define closed internal Port contracts

**Files:**

- Create: `internal/moduleport/ports.go`
- Create: `internal/moduleport/ports_test.go`
- Modify: `sdk/port/catalog.go`
- Modify: `sdk/internal/assembly/graph.go`

**Interfaces:**

- Consumes: the 14 Internal Host/Port rows in the canonical catalog.
- Produces: non-public typed references, cardinalities, and conditional-required
  relationships.

- [ ] Write `TestPublicSourceCannotProvideCorePort`; expected RED is any graph
  path that accepts a T2 Provider for `core/*`.
- [ ] Write exact cardinality and conditional-Host tests.
- [ ] Implement internal definitions outside public SDK import paths.
- [ ] Run `go test ./internal/moduleport ./sdk/internal/assembly -run Core`.
- [ ] Commit `feat(assembly): define closed internal ports`.

### Task 2: Extract LoopDriver and ModelHost composition

**Files:**

- Create: `internal/modules/loop/module.go`
- Create: `internal/modules/loop/module_test.go`
- Create: `internal/modules/model/module.go`
- Create: `internal/modules/model/module_test.go`
- Modify: `internal/runtime/engine.go`
- Modify: `internal/app/model.go`
- Modify: `internal/app/app.go`

**Interfaces:**

- Consumes: current Eino ADK Engine and P5 ModelHost.
- Produces: exactly one `core/loop-driver@v1` and one
  `core/chat-model-host@v1` Provider.

- [ ] Write parity tests for streaming, tool calls, cancellation, budgets,
  checkpoint/resume, and exactly-one terminal event.
- [ ] Record actual pinned Eino ADK/model APIs used.
- [ ] Extract constructors without adding a second Engine or model broker.
- [ ] Run `go test ./internal/modules/loop ./internal/modules/model ./internal/runtime ./internal/app`.
- [ ] Commit `refactor(runtime): compose loop and model modules`.

### Task 3: Extract Storage Engine and Checkpoint Store

**Files:**

- Create: `internal/modules/storage/module.go`
- Create: `internal/modules/storage/module_test.go`
- Create: `internal/modules/checkpoint/module.go`
- Create: `internal/modules/checkpoint/module_test.go`
- Modify: `internal/app/app.go`
- Modify: `internal/runtime/checkpoint.go`
- Modify: `internal/runtime/checkpointadapter.go`

**Interfaces:**

- Consumes: current SQLite/Postgres Engine implementations and versioned
  checkpoint envelope.
- Produces: one `core/storage-engine@v1` and one
  `core/checkpoint-store@v1` Provider without moving Journal authority.

- [ ] Write contract tests for transaction boundaries,
  durability-before-visibility, checksum, engine version, generation flip,
  recovery failure, and cleanup.
- [ ] Ensure a Storage Provider cannot redefine event schema or terminal
  uniqueness.
- [ ] Keep Eino checkpoint adaptation inside `internal/runtime`.
- [ ] Run SQLite and Postgres package tests plus runtime recovery tests.
- [ ] Commit `refactor(storage): compose storage and checkpoint modules`.

### Task 4: Extract Credential Resolver

**Files:**

- Create: `internal/modules/credential/module.go`
- Create: `internal/modules/credential/module_test.go`
- Modify: `internal/provider/resolving.go`
- Modify: `internal/channelhost/channelenv.go`
- Modify: `internal/app/app.go`

**Interfaces:**

- Consumes: env-key references and existing redaction authority.
- Produces: exactly one `core/credential-resolver@v1` scoped resolver.

- [ ] Write RED tests for enumeration, cross-Module access, Manifest leak,
  Journal leak, error leak, and missing Secret.
- [ ] Extract resolution while keeping Secret authority internal-only.
- [ ] Run provider, ChannelHost, app, and secret audit tests.
- [ ] Commit `refactor(secret): compose credential resolver module`.

### Task 5: Extract Sandbox Backend

**Files:**

- Create: `internal/modules/sandbox/module.go`
- Create: `internal/modules/sandbox/module_test.go`
- Modify: `internal/runtime/sandbox_manager.go`
- Modify: `internal/runtime/filesystem_backend.go`
- Modify: `internal/runtime/shell.go`
- Modify: `internal/commandpolicy/policy.go`

**Interfaces:**

- Consumes: workspace identity, command Policy, approval, and scoped Grants.
- Produces: exactly one `core/sandbox-backend@v1` implementation.

- [ ] Write parity tests for path containment, command classification, approval
  hash, cancellation, process tree cleanup, and audit projection.
- [ ] Ensure the backend cannot replace final Policy or approval authority.
- [ ] Keep protected Tools on the same ToolHost envelope.
- [ ] Run sandbox, filesystem, shell, and command-policy tests.
- [ ] Commit `refactor(sandbox): compose governed sandbox backend`.

### Task 6: Register optional internal Hosts conditionally

**Files:**

- Create: `internal/modules/optional/catalog.go`
- Create: `internal/modules/optional/catalog_test.go`
- Modify: `internal/modules/defaults/catalog.go`
- Modify: `recipes/default.vivy.yml`
- Modify: `recipes/minimal.vivy.yml`

**Interfaces:**

- Consumes: ToolHost, ContextHost, SkillHost, MCPHost, ObserverHost, StatusHost,
  PresentationHost, and ActionHost implementations.
- Produces: default-on T1 Module entries and conditional Host graph rules.

- [ ] Write a table test proving each selected public Provider requires its
  Host and each omitted capability disappears from minimal Assembly.
- [ ] Register default Hosts without activating unconfigured instances.
- [ ] Reject a Provider/Host mismatch at G2.
- [ ] Run `go test ./internal/modules/... ./sdk/internal/assembly -run Host`.
- [ ] Commit `feat(assembly): register optional internal hosts`.

### Task 7: Prove lifecycle transaction and no authority drift

**Files:**

- Create: `internal/modules/lifecycle_conformance_test.go`
- Create: `internal/modules/authority_conformance_test.go`
- Modify: `internal/app/shutdown_test.go`
- Modify: `internal/runtime/recovery_test.go`

**Interfaces:**

- Consumes: complete internal Assembly lifecycle.
- Produces: Gate B hardening evidence.

- [ ] Inject required startup failure after several owners and assert reverse
  Stop/Close, idempotence, deadlines, and preserved cause chains.
- [ ] Assert no Module can create a second Service, Journal, Policy evaluator,
  RPC server, ChannelHost, FaceHost, or ToolHost.
- [ ] Assert Eino import quarantine with an automated source test.
- [ ] Run `go test ./internal/modules/... ./internal/app ./internal/runtime`.
- [ ] Run `just ci`.
- [ ] Commit `test(assembly): prove internal module authority`.

## Phase exit and rollback

Exit requires every required internal Port to have exactly one Provider, every
conditional Host to follow its Providers, parity tests to remain green, and L0
authority to remain in place. Rollback selects the prior Generation or reverts
the focused internal-Port commit; it never opens a `core/*` Port publicly.
