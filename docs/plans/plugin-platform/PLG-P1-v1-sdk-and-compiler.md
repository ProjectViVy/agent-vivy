# PLG-P1 v1 SDK and Assembly Compiler Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> `superpowers:subagent-driven-development` (recommended) or
> `superpowers:executing-plans` to implement this plan task-by-task. Steps use
> checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the clean-break `vivy.module/v1` SDK and deterministic Assembly
Compiler foundation that rejects all v0 inputs and emits typed, inspectable
Generation plans.

**Architecture:** Focused public SDK packages define pure data and public Port
contracts; `sdk/internal/assembly` owns source resolution, graph compilation,
Grant calculation, generation, and sealing. The CLI delegates to this single
compiler instead of maintaining separate verify/pack logic.

**Tech Stack:** Go, YAML/JSON, SHA-256, generated Go source, `go test`, `just ci`.

**Spec:** `docs/architecture/VIVY-MODULE-STANDARD.md`,
`docs/architecture/VIVY-ASSEMBLY.md`, and
`docs/architecture/VIVY-PORT-CATALOG.md`.

## Global Constraints

- State: `COMPLETE · 2026-09-10` by human-owner decision; landed together with
  P2 as one clean-break implementation unit.
- The I18N descriptor, catalog-hash, fallback, and Generation identity
  contract was accepted on 2026-09-09 and is a required compiler input.
- P1 and P2 are one clean-break landing unit. P1 commits may exist on the
  feature branch, but main MUST NOT receive a half-cut state that still exposes
  v0 without the P2 default-body cutover.
- No v0 parser, Adapter, alias, warning period, or conversion command.
- Generated files are written only by the generator.
- No Eino type enters the public SDK.

---

### Task 1: Define pure-data Module and Port primitives

**Files:**

- Create: `sdk/module/descriptor.go`
- Create: `sdk/module/descriptor_test.go`
- Create: `sdk/module/i18n.go`
- Create: `sdk/module/i18n_test.go`
- Create: `sdk/module/lifecycle.go`
- Create: `sdk/port/catalog.go`
- Create: `sdk/port/catalog_test.go`
- Create: `sdk/port/support.go`
- Create: `sdk/port/support_test.go`

**Interfaces:**

- Consumes: the canonical Descriptor and 14 public Port names.
- Produces: `module.Descriptor`, `module.I18N`, `module.PortRef`, `module.Requirement`,
  `module.Grant`, `module.Lifecycle`, `port.Definition`, and the evidence-owned
  support states consumed by the compiler and Inspect.

Target types:

```go
type Descriptor struct {
    APIVersion      string
    Module          Identity
    Source          Source
    Provides        []PortRef
    Requires        []Requirement
    Optional        []Requirement
    Conflicts       []Conflict
    RequestedGrants []Grant
    I18N            *I18N
    Lifecycle       Lifecycle
}

type Definition struct {
    Ref         module.PortRef
    Visibility Visibility
    Cardinality Cardinality
    Consumer    module.PortRef
}
```

- [x] Write `TestDescriptorValidateRejectsV0` expecting failure containing
  `unsupported apiVersion vivy.plugin/v0`; run it and observe RED because the
  v1 validator does not exist.
- [x] Write tests for empty IDs, unnamespaced IDs, invalid semver, malformed
  SHA-256, duplicate Port references, and side-effect-free parsing.
- [x] Implement the minimal immutable data types and validation.
- [x] Write `TestCatalogContainsExactlyApprovedPublicPorts` with the 14 exact
  names from the normative catalog.
- [x] Implement the typed catalog constants and reject unknown Ports.
- [x] Write `TestDescriptorCannotClaimSupport` and table tests proving a
  `SPECIFIED` Port cannot be selected without build-owned evidence. A Module
  descriptor or README cannot set its support state.
- [x] Define the support-state and evidence-reference primitives now; later
  phases populate them as each Port's seven artifacts land, and P9 performs the
  release-wide audit rather than introducing this rule after selection exists.
- [x] Run `go test ./sdk/module ./sdk/port` and expect PASS.
- [x] Commit `feat(sdk): define v1 module and port contracts`.

### Task 2: Replace Manifest parsing with v1-only canonical parsing

**Files:**

- Replace: `sdk/internal/manifest.go`
- Modify: `sdk/internal/verify.go`
- Create: `sdk/internal/manifest_test.go`
- Modify: `sdk/internal/verify_test.go`
- Replace legacy fixtures under: `sdk/internal/testdata/`; preserve and consume
  `sdk/internal/testdata/plugin-v1/`

**Interfaces:**

- Consumes: `module.Descriptor`.
- Produces: `loadDescriptor(path) (module.Descriptor, []byte, error)`, where the
  byte slice is canonical semantic input for hashing.

- [x] Write `TestLoadDescriptorRejectsEveryLegacyFixture` over each existing
  `vivy-plugin.json`; expected RED is that current parsing accepts v0.
- [x] Write canonicalization tests proving map order and formatting do not
  change canonical bytes.
- [x] Implement strict `vivy.module/v1` decoding with duplicate-key and unknown
  semantic-field rejection.
- [x] Strictly decode the selected `vivy.i18n/v1` catalog; enforce path
  confinement, literal owner namespace, English completeness, locale/form
  placeholder parity, and all normative resource limits.
- [x] Emit deterministic `COMPLETE`, `INCOMPLETE_LOCALE`, per-locale
  `INCOMPLETE`, or `NOT_APPLICABLE` evidence; invalid catalog input emits no
  formal artifact.
- [x] Replace old Seam fixtures with v1 Descriptor success/failure fixtures;
  retain one v0 fixture solely to prove hard rejection.
- [x] Load the preflight `plugin-v1` fixture index in the real Go tests; keep
  its case IDs and stable diagnostic substrings usable across compiler tasks.
- [x] Remove `apiVersionV0`, Seam validation, and Seam-specific branches.
- [x] Run `go test ./sdk/internal -run 'Descriptor|Manifest|Verify'`.
- [x] Commit `feat(sdk): accept only v1 module descriptors`.

### Task 3: Compile identity, Trust, and the dependency graph

**Files:**

- Create: `sdk/internal/assembly/source.go`
- Create: `sdk/internal/assembly/source_test.go`
- Create: `sdk/internal/assembly/graph.go`
- Create: `sdk/internal/assembly/graph_test.go`
- Create: `sdk/internal/assembly/compiler.go`
- Create: `sdk/internal/assembly/compiler_test.go`

**Interfaces:**

- Consumes: `[]module.Descriptor`, canonical Port Catalog, Source Catalog, and
  a parsed v1 Recipe.
- Produces: `AssemblyPlan` with resolved Modules, Port edges, lifecycle order,
  ordered contributions, and deterministic diagnostics.

Target entry point:

```go
type Compiler struct {
    Ports   port.Catalog
    Sources SourceCatalog
}

func (c Compiler) Compile(ctx context.Context, recipe Recipe) (AssemblyPlan, error)
```

- [x] Write failing tests for duplicate Module, missing Provider, duplicate
  exclusive Provider, unused Provider, unresolved order edge, conflict, and
  dependency cycle.
- [x] Run the matching cases from `sdk/internal/testdata/plugin-v1/` through
  the real compiler rather than duplicating their data in Go source.
- [x] Write `TestDescriptorCannotAssignTrust`; expected RED is the absence of a
  Source Catalog authority.
- [x] Implement source resolution and T1/T2 assignment independent of Module
  data.
- [x] Implement deterministic graph validation and topological lifecycle order.
- [x] Ensure diagnostic sorting is stable across repeated runs.
- [x] Run `go test ./sdk/internal/assembly -run 'Source|Graph|Compile'`.
- [x] Commit `feat(sdk): compile module dependency graphs`.

### Task 4: Calculate effective Grants

**Files:**

- Create: `sdk/internal/assembly/grants.go`
- Create: `sdk/internal/assembly/grants_test.go`
- Modify: `sdk/internal/assembly/compiler.go`

**Interfaces:**

- Consumes: requested Grants, Port allow-list, Trust ceiling, and structured
  Recipe approvals.
- Produces: sorted `[]EffectiveGrant` embedded in `AssemblyPlan`.

```go
type EffectiveGrant struct {
    Name        module.Grant
    Constraints map[string][]string
}
```

- [x] Write a table-driven RED test for all 12 approved Grants, unknown Grant,
  unauthorized `proc.spawn`, constrained `net.client`, and implicit default
  denial.
- [x] Run the `unapproved-grant` acceptance fixture through this calculation.
- [x] Implement the four-way intersection exactly once in the compiler.
- [x] Reject Secret values and environment-derived material in constraints.
- [x] Run `go test ./sdk/internal/assembly -run Grant`.
- [x] Commit `feat(sdk): enforce generation grants`.

### Task 5: Generate typed wiring and seal provenance

**Files:**

- Create: `sdk/internal/assembly/generate.go`
- Create: `sdk/internal/assembly/generate_test.go`
- Create: `sdk/internal/assembly/manifest.go`
- Create: `sdk/internal/assembly/manifest_test.go`
- Modify: `sdk/internal/pack.go`
- Modify: `sdk/internal/inspect.go`
- Replace generated targets under: `internal/generated/assembly/`

**Interfaces:**

- Consumes: `AssemblyPlan` and canonical build inputs.
- Produces: generated typed binder source and an immutable
  `GenerationManifest` with content-addressed `GenerationID`.

- [x] Write a golden RED test showing two semantically identical Recipes must
  produce byte-identical binder and Manifest output.
- [x] Write a RED test proving a source hash, UI hash, Port version, or compiler
  version change changes Generation ID.
- [x] Prove canonical catalog formatting is identity-neutral while a
  translation, placeholder declaration, schema, or packaged-locale change
  changes its SHA-256 digest and Generation ID.
- [x] Implement stable generated imports, typed constructor calls, and reverse
  cleanup ownership without reflection or `[]any`.
- [x] Remove random/time-based Generation identity from the new compiler path.
- [x] Make Inspect distinguish not compiled, unconfigured, inactive, ready,
  unavailable, specified, and deferred.
- [x] Project catalog schema, path, digest, default and packaged locales,
  per-locale completeness, and evidence identifiers into Manifest and Inspect.
- [x] Run `go test ./sdk/internal/assembly ./sdk/internal`.
- [x] Commit `feat(sdk): generate and seal v1 assemblies`.

### Task 6: Make the CLI one compiler front end

**Files:**

- Modify: `sdk/main.go`
- Modify: `sdk/internal/cmd.go`
- Modify: `sdk/internal/verify.go`
- Modify: `sdk/internal/pack.go`
- Modify: `sdk/internal/inspect.go`
- Modify: `sdk/README.md`

**Interfaces:**

- Consumes: the sole `assembly.Compiler`.
- Produces: v1-only `verify`, `pack`, and `inspect-artifact` commands.

- [x] Write CLI tests proving all three commands use the same validation and
  v0 fails before graph construction.
- [x] Remove legacy command wording and Seam output.
- [x] Verify failed compilation emits no formal Generation directory.
- [x] Run `go test ./sdk/...`.
- [x] Run `just ci`.
- [x] Commit `feat(sdk): cut vivy-sdk over to assembly v1`.

## Phase exit and rollback

Exit requires deterministic compiler tests, hard v0 rejection, no public Eino
types, and a P2-ready generated binder. Do not merge P1 to main alone. Rollback
is deleting the isolated feature branch/worktree; do not add a v0 Adapter to
make a partial branch releasable.
