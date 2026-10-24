# MASK-1 contract and assembly foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans for native execution, or superpowers:subagent-driven-development only when that method is selected. Steps use checkbox syntax for tracking.

**Goal:** Establish a removable typed T1 provider foundation without advertising incomplete behavior.
**Architecture:** Closed maskcontract values and pure embedded catalog; generated optional factory. Storage and Runtime retain their existing authority.
**Tech Stack:** Go, existing SDK/Assembly compiler, embedded Markdown; no new dependency.
**Spec:** [design](../../specs/2026-09-21-mask-subsystem-design.md), source baseline `5253f77`, design baseline `a0f892c`, [MASK-C1](contracts.md).
**Epic / requirements:** MASK-43 / M2, M5, M7. State/predecessors: [index](index.md).

## Global Constraints

- `core/mask-service@v1`, owner `vivy/masks`, T1, cardinality 0..1, Runtime consumer.
- No Eino outside runtime/provider; no public provider for core Ports.
- Body 1–16384 UTF-8 bytes; name 1–128; description 0–1024; no prose constants.
- Construct has no external effects; incomplete feature is not production selectable.
- Read root AGENTS.md and `.agents/skills/vivy-plugin/SKILL.md` before execution.

## Review Focus

- A public provider claiming the reserved ID must fail before construction (Task 2).
- A built-in name containing quotes cannot change prompt delimiters (Task 1 literal data fixture).
- A backend-only selection must not force a Web UI dependency (Task 2).
- Omitted provider code must have no direct App import (Task 2 import fixture).
- Startup failure must close constructed owners once (Task 2 rollback fixture).

## Task 1: Shared contracts and pure catalog

**Files:** NEW `internal/maskcontract/masks.go`, `masks_test.go`;
NEW `internal/modules/masks/catalog.go`, `catalog_test.go`, `prompts/mask-frame.md`,
`prompts/programmer.md`, `prompts/researcher.md`, `prompts/writer.md`.
NEW `internal/storage/masks.go` contains interfaces/value declarations from MASK-C1
only; implementations belong to MASK-2/3. No fake methods returning success.

**Consumes:** Existing domain IDs and controlaction.Host.
**Produces:** MASK-C1 domain/management/storage declarations, Service.PromptAssets,
pure built-in resolver and canonical digest functions.

- [ ] Add table-driven tests for normalization limits including multibyte overflow,
  LF normalization, blank body, NUL, invalid UTF-8 and reserved ID collisions.
  Required observable example (test calls proposed helper):

```go
func TestMaskNormalizePreservesLiteralTemplate(t *testing.T) {
    in := CreateRequest{OperationID: "00000000-0000-4000-8000-000000000001",
        Name: "  Writer  ", Body: "{system}\r\n{{persona}}"}
    got, err := NormalizeCreate(in)
    if err != nil || got.Name != "Writer" || got.Body != "{system}\n{{persona}}" {
        t.Fatalf("literal normalization failed: %v", err)
    }
}
```

- [ ] Run `go test ./internal/maskcontract -run TestMask -count=1`; confirm meaningful
  red evidence, not a missing Go executable mistaken for test failure.
- [ ] Implement validation/digest functions exactly as MASK-C1. Add immutable catalog
  values derived from embedded bodies; no file scan or writable built-in row.
  Use canonical names Programmer, Researcher, Writer. Release revision begins at 1;
  Generation+digest identifies actual built-in bytes.
- [ ] Author concise Markdown bodies: programmer focuses on scoped coding and tests;
  researcher separates evidence/inference and identifies uncertainty; writer follows
  audience/format and does not invent facts. None grants tools, defines identity,
  orders hidden reasoning disclosure or overrides permissions. Mask frame includes
  persona/runtime precedence and explicit task-over-style precedence.
- [ ] Test deterministic digest, duplicate IDs, changed Name/Body digest, unchanged
  Description digest, missing asset and caller mutation of returned values.
- [ ] Run `go test ./internal/maskcontract ./internal/modules/masks -count=1`.
  Expected all tests pass with no external network. Commit explicit task paths as
  `feat: define internal mask contracts and embedded catalog`.

## Task 2: Typed assembly foundation and support staging

**Files:** Modify `internal/moduleport/ports.go`, `ports_test.go`,
`internal/modules/defaults/catalog.go`, `catalog_test.go`,
`sdk/internal/assembly/source.go`, `runtime_generate.go`, `core_ports_test.go`,
`runtime_generate_test.go`, `sdk/internal/frontend_v1.go`, `internal/app/assembly_validate.go`;
NEW `internal/moduleport/masks.go`, `internal/modules/masks/module.go`, `module_test.go`.
Modify `docs/architecture/VIVY-PORT-CATALOG.md` only to record SPECIFIED, not SUPPORTED.
Do not edit `internal/generated/assembly/zz_default.go` manually.

**Consumes:** Contract declarations from Task 1.
**Produces:** moduleport.MaskFactory; optional typed binding in generated
RuntimeAssembly; closed owner/cardinality metadata; generation validation fixtures.

- [ ] Write compiler tests named `TestMaskCorePortRejectsPublicProvider`,
  `TestMaskCorePortRejectsDuplicate`, `TestMaskOmittedHasNoProviderImport`,
  `TestMaskBackendDoesNotRequireUI`, and rollback test on the existing Generation
  lifecycle fixture. Assert errors before Provider.Start and no selected source
  in omitted generated imports.
- [ ] Run `go test ./internal/moduleport ./sdk/internal/assembly -run Mask -count=1`
  to demonstrate red contracts. Existing core table tests may require new expected
  metadata; never mechanically relabel seven-artifact evidence as passing.
- [ ] Add build-owned mask-factory metadata to defaults.Binding and assembly.GoBinding,
  and copy it through frontend_v1.go's internal-source conversion. Emit a typed field and import
  only when selected. Desired generator behavior:

```text
if selected graph includes core/mask-service@v1:
    enforce provider owner vivy/masks, T1, single provider
    emit moduleport.MaskFactory field with selected constructor reference
else:
    emit nil/absent optional binding and no masks implementation import
production selection remains rejected until support evidence is complete
```

- [ ] Implement pure Module descriptor/lifecycle foundation; storage checks are
  deferred to real Start/Open in MASK-2, not fake Ready. Compiler test fixtures
  may use isolated test Providers; never ship a placeholder runtime Provider.
- [ ] Verify the source catalog can bind this internal source; record required
  backend/UI selection metadata without auto-selecting a UI Port for headless.
  Do not add `vivy/masks` to default Recipe in this Story.
- [ ] Run `go test ./internal/moduleport ./sdk/internal/assembly ./internal/modules/masks -count=1`
  and `just ci`; inspect generated fixture imports. Commit explicit files as
  `feat: add closed mask assembly foundation`.

## Acceptance and handoff

Return AC-01 foundation evidence (see [acceptance](acceptance.md)), exact commit,
contract revision and failing/passing test output. Seven-artifact integration is
not complete until later Stories. Runtime still behaves as before. Any different
factory/storage layering or runtime discovery proposal stops this Story for a
shared-contract correction. No UI, CRUD, persona implementation or default Recipe
activation here. Documentation/source additions are reversible; no user DB changes.
