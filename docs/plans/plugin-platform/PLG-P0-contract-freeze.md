# PLG-P0 Contract Freeze Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> `superpowers:subagent-driven-development` (recommended) or
> `superpowers:executing-plans` to implement this plan task-by-task. Steps use
> checkbox (`- [ ]`) syntax for tracking.

**Goal:** Publish the normative v1 architecture, complete engineering plan,
repository rules, and reusable VIVY-PLUGIN development Skill without changing
functional code.

**Architecture:** Capture approved decisions in four canonical contracts and
make every legacy or research document defer to them. Convert the open proposal
into manually scheduled phases whose tasks already name code, tests, gates,
failure paths, and rollback evidence.

**Tech Stack:** Markdown, repository Skill format, Git, `just ci`.

**Spec:** The approved decisions recorded in
`docs/architecture/VIVY-MODULE-STANDARD.md` through
`docs/architecture/VIVY-ASSEMBLY.md`.

## Global Constraints

- Documentation and Skill changes only; no Go, TypeScript, generated code,
  manifest schema, test fixture, or runtime behavior change.
- v0 compatibility and migration are explicitly rejected.
- Functional phases remain `UNSCHEDULED` for manual scheduling.
- Product-contract changes normally run `just ci` and receive an iteration log.
  This documentation-only P0 may close with a recorded full-compile skip when
  the human explicitly rejects compilation; P1+ keeps the full gate.

---

### Task 1: Establish the normative source of truth

**Files:**

- Create: `docs/architecture/VIVY-MODULE-STANDARD.md`
- Create: `docs/architecture/VIVY-PORT-CATALOG.md`
- Replace: `docs/architecture/VIVY-PLUGIN-SPEC.md`
- Replace: `docs/architecture/VIVY-ASSEMBLY.md`
- Modify: `docs/architecture/VIVY-FACE-PACK.md`
- Modify: `docs/architecture/VIVY-CHANNEL-PACK.md`
- Modify: `docs/research/advanced-plugin-alliance-2026-09-08.md`

**Interfaces:**

- Consumes: approved Module, Port, Trust, Grant, Tool, UI, Eino, Generation,
  clean-break, and SCX decisions.
- Produces: the only normative vocabulary and authority graph used by all later
  phases.

- [x] Write Module Standard with four layers, Descriptor, typed Port rules,
  Trust/Grant calculation, double lifecycle, support states, and clean break.
- [x] Write Port Catalog with exactly 14 public Ports, protected Tool IDs,
  internal Hosts, authority, cardinality, and failure semantics.
- [x] Replace v0 Plugin Spec with v1 public Module rules and unrestricted UI.
- [x] Replace Assembly proposal with the six compiler gates, content-addressed
  Generation, Manifest, Inspect, runtime freeze, and rollback.
- [x] Mark Face/Channel v0 mechanics and the research proposal as superseded
  while retaining their still-valid product evidence.
- [x] Run:

  ```text
  rg -n "Status: \*\*Normative|状态：\*\*Normative" docs/architecture/VIVY-MODULE-STANDARD.md docs/architecture/VIVY-PORT-CATALOG.md docs/architecture/VIVY-PLUGIN-SPEC.md docs/architecture/VIVY-ASSEMBLY.md
  ```

  Expected: all four canonical documents identify normative v1 status.

### Task 2: Publish the phase-by-phase execution plan

**Files:**

- Create: `docs/plans/plugin-platform/README.md`
- Create: `docs/plans/plugin-platform/00-standing-orders.md`
- Create: `docs/plans/plugin-platform/PLG-P0-contract-freeze.md`
- Create: `docs/plans/plugin-platform/PLG-P1-v1-sdk-and-compiler.md`
- Create: `docs/plans/plugin-platform/PLG-P2-default-generation-parity.md`
- Create: `docs/plans/plugin-platform/PLG-P3-tool-governance.md`
- Create: `docs/plans/plugin-platform/PLG-P4-context-skill-mcp.md`
- Create: `docs/plans/plugin-platform/PLG-P5-provider-eino-adapters.md`
- Create: `docs/plans/plugin-platform/PLG-P6-full-ui-modules.md`
- Create: `docs/plans/plugin-platform/PLG-P7-internal-moduleization.md`
- Create: `docs/plans/plugin-platform/PLG-P8-scx-integration-gates.md`
- Create: `docs/plans/plugin-platform/PLG-P9-release-conformance.md`

**Interfaces:**

- Consumes: canonical architecture contracts and real repository file map.
- Produces: dependency-ordered, manually schedulable implementation tasks.

- [x] Give every phase the required Goal, Architecture, Tech Stack, Spec, and
  Global Constraints header.
- [x] Name exact production and test paths for every task.
- [x] State target interfaces used between phases.
- [x] State a focused RED failure and GREEN command for every code task.
- [x] Include Eino evidence, failure paths, Inspect output, rollback, acceptance,
  and phase exit conditions.
- [x] Mark P1–P9 `UNSCHEDULED`; do not assign dates or engineers.

### Task 3: Make repository rules route all plugin work through v1

**Files:**

- Modify: `AGENTS.md`
- Modify: `docs/TODO.md`

**Interfaces:**

- Consumes: normative contracts and phase status.
- Produces: mandatory agent routing and a living PLG-1 program board entry.

- [x] Add concise plugin-development rules to `AGENTS.md`, including no v0,
  Module/Port order, default Generation, protected Tools, unrestricted UI,
  Eino defer, no discovery, and the new Skill.
- [x] Expand PLG-1 in `docs/TODO.md` to link the contracts and phase plan,
  identify P0 as documentation-only, leave P1–P9 unscheduled, and record SCX
  Gates A/B/C.
- [x] Preserve unrelated TODO rows verbatim.

### Task 4: Create and validate the VIVY-PLUGIN Skill

**Files:**

- Create: `.agents/skills/vivy-plugin/SKILL.md`
- Replace: `.agents/skills/vivy-plugin-five/SKILL.md`
- Modify: `docs/logs/2026-09-09-plugin-platform-v1-plan/notes.md`

**Interfaces:**

- Consumes: canonical architecture contracts and current v0-skill failure
  scenarios.
- Produces: a discoverable v1 decision workflow and a legacy redirect that
  cannot teach v0.

- [x] Record the failing baseline before creating the Skill.
- [x] Write a concise Skill whose trigger covers plugin, Module, Port, Recipe,
  Generation, `vivy-sdk`, pack, and Inspect work.
- [x] Route architecture/product-contract work to `vivy-kernel-ci`, frontend
  implementation to `oil-frontend`, and Eino-scoped work to `vivy-eino`.
- [x] Re-run the baseline scenarios and record the expected compliant decision.
- [x] Run the system `quick_validate.py` against `.agents/skills/vivy-plugin`.
- [x] Replace `vivy-plugin-five` with a redirect that refuses v0 development.

### Task 5: Verify and close the documentation deliverable

**Files:**

- Create: `docs/logs/2026-09-09-plugin-platform-v1-plan/summary.md`
- Create: `docs/logs/2026-09-09-plugin-platform-v1-plan/verification.md`
- Create: `docs/logs/2026-09-09-plugin-platform-v1-plan/acceptance.md`
- Modify: `docs/plans/plugin-platform/README.md`
- Modify: `docs/plans/plugin-platform/PLG-P0-contract-freeze.md`

**Interfaces:**

- Consumes: the complete documentation diff.
- Produces: a verified, committed P0 deliverable.

- [x] Search for conflicting accepted v0 routes, duplicate Port names,
  forbidden UI authorization, and functional source changes.
- [x] Run `git diff --check`.
- [x] Record the explicit docs-only `just ci` skip reason in `verification.md`.
- [x] Record exact commands and outcomes in `verification.md`.
- [x] Mark P0 `COMPLETE` only after documentation evidence is green.
- [x] Stage only the documented paths and commit:

  ```text
  git commit -m "docs: define plugin platform v1 plan"
  ```

## Acceptance

A human can open the program README, follow any phase to exact target files and
tests, trace every Port to one Host authority, see that UI is completely open
without a UI Grant, confirm that v0 has no migration route, and invoke the new
Skill without being directed to code that does not yet exist.
