# PLG-P9 Release Conformance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> `superpowers:subagent-driven-development` (recommended) or
> `superpowers:executing-plans` to implement this plan task-by-task. Steps use
> checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prove the complete plugin platform, publish accurate Inspect and
developer contracts, exercise whole-Generation rollback, and close PLG-1 only
after all claimed Ports are truly supported.

**Architecture:** A shared conformance harness runs focused Provider/Consumer
contracts and whole-Generation scenarios. Release truth comes from the sealed
Manifest plus executable evidence, not Descriptor claims or documentation.

**Tech Stack:** Go conformance harness, `vivy-sdk`, generated Assembly,
React/Vitest/Playwright where UI is included, `just ci`, Git iteration logs.

**Spec:** All four normative plugin architecture documents and P0–P8 plans.

## Global Constraints

- State: `IN PROGRESS · HUMAN-SCHEDULED · 2026-09-14`; depends on every phase
  included in the release scope.
- A Port without all seven artifacts remains `SPECIFIED` or
  `DEFERRED-INDEFINITE`; release does not promote it optimistically.
- No live-network dependency in deterministic unit tests.
- There is no v0 compatibility acceptance matrix.

---

### Task 1: Build the reusable Port conformance harness

**Files:**

- Create: `sdk/conformance/provider.go`
- Create: `sdk/conformance/provider_test.go`
- Create: `sdk/internal/conformance/generation.go`
- Create: `sdk/internal/conformance/generation_test.go`

**Interfaces:**

- Consumes: typed Port Definition, fake Provider factories, Host Consumers,
  and an Assembly plan.
- Produces: machine-readable `ConformanceResult` records embedded in Manifest.

```go
type ConformanceResult struct {
    Port       module.PortRef
    ProviderID string
    SourceSHA256 string
    Suite      string
    Passed     bool
    EvidenceID string
}
```

- [x] Write `TestUnsupportedPortCannotClaimSupported`; expected RED is a
  Descriptor-only capability appearing as supported.
- [x] Encode common checks for registration, missing/duplicate Provider,
  version, cycle, Grant, timeout, cancellation, startup, unavailable state,
  cleanup, redaction, provenance, default behavior, and a real failure.
- [x] Keep Port-specific semantic suites beside their Host packages.
- [x] Run `go test ./sdk/conformance ./sdk/internal/conformance`.
- [x] Commit `feat(conformance): define plugin port evidence`.

### Task 2: Aggregate and audit evidence-derived support state

**Files:**

- Modify: `sdk/port/catalog.go`
- Modify: `sdk/internal/assembly/manifest.go`
- Modify: `sdk/internal/inspect.go`
- Create: `sdk/internal/support_state_test.go`

**Interfaces:**

- Consumes: the P1 support-state primitives, per-phase evidence records, Port
  Definitions, and Conformance results.
- Produces: `RESERVED`, `SPECIFIED`, `CANDIDATE`, `SUPPORTED`, or
  `DEFERRED-INDEFINITE` Inspect state.

- [x] Write release-wide table tests proving each missing artifact still
  prevents `SUPPORTED` and no phase bypassed the P1 selection rule.
- [x] Prevent Module or README claims from changing computed state.
- [x] Include evidence IDs without embedding local absolute paths or Secrets.
- [x] Audit catalog completeness states from compiler evidence; Descriptor or
  documentation claims cannot promote `INCOMPLETE_LOCALE` to `COMPLETE`.
- [x] Run `go test ./sdk/internal -run SupportState`.
- [x] Commit `feat(inspect): derive capability support from evidence`.

### Task 3: Run the whole-Generation failure matrix

**Files:**

- Create: `sdk/internal/conformance/testdata/` fixture Generations
- Create: `sdk/internal/conformance/failure_matrix_test.go`
- Modify: `sdk/internal/pack_test.go`

**Interfaces:**

- Consumes: default, minimal, invalid graph, denied Grant, failing lifecycle,
  full UI, MCP unavailable, and SCX candidate Recipes.
- Produces: deterministic build/no-build decisions and diagnostic snapshots.

- [x] Write fixtures for missing Provider, duplicate exclusive Provider, cycle,
  conflict, unused Provider, protected Tool override, UI root conflict,
  Middleware timeout, startup rollback, bad hash, v0 input, and unsupported
  Eino-scoped capability.
- [x] Cover catalog duplicate keys, unknown fields, ownership, placeholder
  drift, missing English, confinement/symlink escape, and every resource limit.
- [x] Assert every invalid case emits no formal artifact.
- [x] Assert errors name Module, Port/edge, and violated rule with redaction.
- [x] Repeat valid builds and assert identical canonical Manifest and
  Generation ID.
- [x] Run `go test ./sdk/internal/conformance ./sdk/internal -run Generation`.
- [x] Commit `test(assembly): cover generation failure matrix`.

### Task 4: Prove default parity and minimal removal

**Files:**

- Modify: `internal/app/default_generation_test.go`
- Modify: `sdk/internal/removal_conformance_test.go`
- Create: `docs/research/plugin-v1-default-parity.md`

**Interfaces:**

- Consumes: default and minimal sealed artifacts.
- Produces: human-readable inventory comparison and binary/Manifest removal
  proof.

- [x] Compare established first-party Tool, Channel, Face, Provider Profile,
  Context, Skill, MCP, status, and UI behavior against the P2 baseline.
- [x] Start default with no credentials and prove network instances remain
  unconfigured/inactive.
- [x] Verify each omitted Module has no import, constructor, asset, Port edge,
  Grant, catalog, generated localization projection, or Inspect record in the
  minimal artifact.
- [x] Run both executable smoke paths required by the selected Recipes.
- [x] Commit `test(release): prove default parity and minimal removal`.

### Task 5: Exercise whole-Generation rollback

**Files:**

- Modify: `internal/domain/generation.go`
- Modify: `internal/studiocore/` only if the existing lifecycle owner requires
  the new Manifest fields
- Create: `sdk/internal/conformance/rollback_test.go`
- Create: release iteration `rollback.md`

**Interfaces:**

- Consumes: two sealed Generation artifacts and existing lifecycle promotion.
- Produces: an exercised rollback to the previous artifact without code hot
  swap or Journal mutation.

- [x] Write a RED test that installs candidate B, detects failed acceptance,
  and restores sealed artifact A on the next launch.
- [x] Assert artifact A's embedded identity and Manifest are unchanged.
- [x] Assert rollback restores catalog digests and packaged-locale identity
  together with the whole sealed Generation.
- [x] Assert durable data follows existing compatibility contracts and no
  plugin-specific rollback writer touches Journal truth.
- [x] Perform the real lifecycle rollback and record artifact IDs.
- [x] Commit `test(release): exercise generation rollback`.

### Task 6: Validate developer workflow and the VIVY-PLUGIN Skill

**Files:**

- Modify: `.agents/skills/vivy-plugin/SKILL.md` only when observed execution
  evidence exposes a gap
- Modify: `docs/architecture/VIVY-PLUGIN-SPEC.md`
- Modify: `docs/plans/plugin-platform/README.md`
- Create: `plugins/example-v1/` only if a real maintained example is approved
  as part of the scheduled release scope

**Interfaces:**

- Consumes: final v1 commands and contracts.
- Produces: truthful developer instructions that do not mention nonexistent or
  legacy APIs.

- [x] Run the Skill's quick validator and pressure-scenario matrix.
- [x] Execute `verify`, `pack`, `inspect-artifact`, removal, and failure flow
  from the documented commands.
- [x] Remove or correct any instruction whose command/output differs from the
  shipped implementation.
- [x] Do not create an example solely to satisfy documentation; an approved
  example must pass full conformance and remain maintained.
- [x] Commit `docs(plugin): align v1 developer workflow`.

### Task 7: Run final product gates and close PLG-1

**Files:**

- Modify: `docs/TODO.md`
- Create: `docs/logs/YYYY-MM-DD-plugin-platform-v1/summary.md`
- Create: `docs/logs/YYYY-MM-DD-plugin-platform-v1/verification.md`
- Create: `docs/logs/YYYY-MM-DD-plugin-platform-v1/acceptance.md`
- Create: `docs/logs/YYYY-MM-DD-plugin-platform-v1/rollback.md`

**Interfaces:**

- Consumes: all supported Port and Generation evidence.
- Produces: auditable PLG-1 closure and SCX Gate C input.

- [x] Run every focused Port suite.
- [x] Run `vivy-sdk verify` on all selected public Modules.
- [x] Pack and inspect default, minimal, full-UI, and SCX candidate Generations.
- [x] Prove canonical catalog formatting is identity-neutral and semantic
  catalog changes deterministically change Generation identity.
- [ ] Run `just ci`.
- [ ] Run the split browser smoke at `http://127.0.0.1:3015` for user-visible
  UI behavior.
- [x] Run the appropriate VIVY CODE/Channel/MCP real-path smokes without using
  tenant Journal data.
- [x] Record every exact command, result, artifact ID, and skipped slice reason.
- [ ] Move PLG-1 from the open board only after all selected capabilities and
  SCX Gate C are complete.
- [ ] Commit `docs(log): close plugin platform v1`.

## Phase exit and rollback

Exit requires evidence-derived support states, full failure matrix, default
parity, minimal removal, exercised rollback, truthful Skill instructions,
`just ci`, real-path smokes, iteration logs, and PLG-1 closure. Any failed gate
keeps PLG-1 open without restoring v0.

## SCX alignment (2026-09-12)

Apply release evidence to the selected SCX scope. Initial design fixtures cover personality/emotion, predetermined file retrieval and memory return; they are not executed proof or live integration evidence. No additional hardware/simulator gate is introduced. Original device validation stays with its unrecovered original plan. Generation rollback does not undo external memory writes; record compatibility and pending receipts separately.

See [SCX integration mapping](../../architecture/SCX-PLUGIN-INTEGRATION.md).
This note records design dependencies; it does not reopen completed work or schedule implementation.

### Selected P8 Gate C matrix (2026-09-13)

The owner explicitly authorized all remaining P8 Gates. The bounded subset of
P9 needed by P8 Gate C is therefore executed without scheduling or claiming
completion of the full PLG-P9 phase: selected SCX/default/minimal pack and
Inspect, seven-artifact evidence for the two selected public Ports,
deterministic rebuild, physical removal, redaction/outage cases, and actual
Studio rollback to the prior sealed Generation without tenant Journal mutation.
Full-UI, browser, channel, live MCP, and every-Port release closure were not
implied by the SCX candidate result; they remain explicit gates in the PLG-P9
phase scheduled on 2026-09-14.
