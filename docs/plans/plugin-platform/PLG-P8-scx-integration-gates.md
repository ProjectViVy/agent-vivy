# PLG-P8 SCX Integration Gates Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> `superpowers:subagent-driven-development` (recommended) or
> `superpowers:executing-plans` to implement this plan task-by-task. Steps use
> checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bind SCX to the v1 Module/Port platform at three explicit gates so
context evolution cannot create direct implementation dependencies or a second
Runtime path.

**Architecture:** SCX capabilities are classified by authority before coding:
content producers are Context Sources, source composition remains ContextHost,
durable truth remains Kernel-owned storage/Journal contracts, and final Eino
message adaptation remains inside `internal/runtime`. PLUGINS provides the
assembly and evidence gates; it does not absorb SCX product logic.

**Tech Stack:** Go domain interfaces, ContextHost/SkillHost/ToolHost, generated
Assembly, Eino v0.9.13 adapters, SCX tests, `just ci`.

**Spec:** `docs/architecture/VIVY-MODULE-STANDARD.md`,
`docs/architecture/VIVY-PORT-CATALOG.md`, and this program README.

## Global Constraints

- State: `UNSCHEDULED`; the repository has no persisted `SCX` or `M-SCX*`
  document as of 2026-09-09, so this file uses stable Gate IDs instead of
  inventing stage names.
- Before SCX execution, the human-maintained SCX plan must link its real stage
  IDs to Gates A/B/C.
- SCX never imports a concrete plugin/Source implementation or public Module
  code from Runtime composition.
- SCX cannot own final Prompt, Journal, Policy, approval, or Tool execution.

---

### Task 1: Classify every SCX capability by authority

**Files:**

- Modify: the persisted SCX program index when it exists
- Create: `docs/architecture/SCX-PLUGIN-INTEGRATION.md`
- Modify: `docs/TODO.md`

**Interfaces:**

- Consumes: the approved SCX feature inventory and the Port Catalog.
- Produces: a row-by-row owner/Provider/Consumer/failure mapping.

Use this mandatory classification:

| SCX behavior | Correct home |
|---|---|
| Produces candidate context | `std/context-source@v1` Provider |
| Chooses, ranks, deduplicates, or budgets multiple Sources | T1 ContextHost policy |
| Stores durable memory/index data | closed internal backend behind Kernel identity/provenance |
| Injects final model messages | `internal/runtime` Eino adapter only |
| Exposes model-visible retrieval action | `std/tool@v1` through ToolHost |
| Discovers remote MCP context/tools | MCPHost -> ContextHost/ToolWorld |
| Displays SCX state | Status Source and/or full UI Module |
| Mutates SCX configuration | typed Control Action |

- [ ] For every SCX item, record Provider, Consumer, authority, data scope,
  lifecycle, failure mode, and conformance owner.
- [ ] Reject rows labeled only “SCX service” or “plugin” without a Port and
  authority mapping.
- [ ] Identify which rows consume Gate A, B, or C.
- [ ] Commit `docs(scx): map context work to plugin v1 ports`.

### Task 2: Pass SCX-PLUGIN-GATE-A — contract freeze

**Files:**

- Modify: `docs/architecture/SCX-PLUGIN-INTEGRATION.md`
- Modify: SCX design/plan documents linked by the human scheduler
- Modify: `docs/TODO.md`

**Interfaces:**

- Consumes: P0 contracts plus P1 compiler, P2 Tool/ToolWorld, P3 ToolHost, and
  P4 Context/Skill/MCP Host and Source evidence.
- Produces: start authorization for SCX tasks that depend only on frozen
  contracts.

Gate A requires:

- Module Descriptor and Port version frozen;
- Context Source, Skill Source, Tool, ToolWorld, and relevant Host contracts
  defined in code;
- required cardinality, Trust, Grant, lifecycle, and failure semantics tested;
- protected Tool names reserved;
- generated graph diagnostics available;
- no SCX direct-import exception.

- [ ] Add a gate test/build check that compiles the SCX-facing interfaces with
  fake Providers and no Runtime implementation import.
- [ ] Verify an invalid direct Provider dependency fails source/architecture
  checks.
- [ ] Record the exact P1, P2, P3, and P4 evidence commits and Port versions in
  the SCX integration document.
- [ ] Mark Gate A passed only after evidence exists; documentation approval
  alone is insufficient.

### Task 3: Implement SCX Providers/Consumers without authority drift

**Files:**

- Create or modify only the exact SCX paths named by its persisted plan
- Test: matching SCX package tests and Host conformance tests

**Interfaces:**

- Consumes: v1 ContextHost/SkillHost/ToolHost and closed internal backends.
- Produces: SCX capability Modules with no composition-root ownership.

- [ ] Write a failing Host-level contract test for one SCX slice before its
  implementation.
- [ ] Implement the slice as the classified Source, Host policy, internal
  backend, Tool, Status Source, UI Module, or Control Action.
- [ ] Prove workspace/session/tenant identity propagates across every request.
- [ ] Prove cancellation, token budget, provenance, and result bounds.
- [ ] Keep Eino conversion in `internal/runtime`; cite the exact pinned API or
  defer the affected capability indefinitely.
- [ ] Commit one SCX capability slice per independently reviewable behavior.

### Task 4: Pass SCX-PLUGIN-GATE-B — default Generation equivalence

**Files:**

- Modify: `internal/modules/defaults/catalog.go`
- Modify: `recipes/default.vivy.yml`
- Modify: SCX integration and default-body tests
- Modify: `docs/architecture/SCX-PLUGIN-INTEGRATION.md`

**Interfaces:**

- Consumes: P2, P3, P4, P7, required P5 adapter evidence, and implemented SCX
  Modules.
- Produces: a default Generation in which SCX uses only governed Hosts.

Gate B requires:

- established first-party behavior remains default-on;
- SCX Sources are selected explicitly and visible in Inspect;
- no network instance activates when unconfigured;
- every SCX Tool and MCP Tool uses ToolHost;
- Context and Skill projection use their sole Hosts;
- SCX creates no second Service, Journal, Policy, registry, or Eino graph;
- default and minimal Recipe behavior is tested;
- failure marks the relevant instance unavailable without silent fallback.

- [ ] Add a default-Generation SCX trace test from source query through final
  Runtime projection.
- [ ] Add failure tests for slow Source, invalid provenance, missing internal
  Host, duplicate Provider, unavailable MCP, and token overflow.
- [ ] Build and inspect default/minimal Generations.
- [ ] Mark Gate B passed only when `just ci` and the real SCX path are green.

### Task 5: Pass SCX-PLUGIN-GATE-C — release and rollback

**Files:**

- Modify: `docs/architecture/SCX-PLUGIN-INTEGRATION.md`
- Modify: SCX release acceptance document
- Modify: `docs/TODO.md`

**Interfaces:**

- Consumes: P9 Generation conformance, Inspect, and rollback evidence.
- Produces: authorization for an SCX release candidate.

Gate C requires:

- every SCX-relevant Port is `SUPPORTED` with seven artifacts;
- sealed Manifest includes SCX Module source/version/hash and Port edges;
- deterministic rebuild produces the same Generation identity;
- previous whole Generation can be restored;
- Observer lag or UI replacement cannot alter durable SCX truth;
- Secret and raw private context are absent from diagnostics;
- default, minimal, unavailable, and rollback scenarios pass.

- [ ] Execute the P9 release matrix with the SCX candidate.
- [ ] Record artifact IDs and redacted evidence.
- [ ] Mark Gate C passed only after rollback is actually exercised.

## SCX critical and non-critical PLUGINS scope

Critical:

- P1 Module/Port/Compiler contract;
- P2 default Generation parity;
- P3 ToolHost and protected Tools;
- P4 ContextHost, SkillHost, and MCPHost;
- P5 ModelHost and Provider Profile required by P7 Task 2;
- P7 relevant internal authority composition;
- P9 conformance, Inspect, and rollback.

Not on the SCX core critical path:

- arbitrary third-party example catalog;
- full UI Module framework unless an SCX release explicitly requires custom UI;
- provider families SCX does not use;
- A2A, WASM, generic Sidecars, and marketplace breadth;
- OAuth capability absent from the pinned Eino/EinoExt surface.

## Phase exit and rollback

Exit requires all three Gates to contain executable evidence and the persisted
SCX plan to cite them by real stage ID. A failed Gate blocks the dependent SCX
transition but does not expand PLUGINS scope. Rollback restores the prior sealed
Generation and retains durable Kernel truth.
