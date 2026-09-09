# PLG-P2 Default Generation Parity Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> `superpowers:subagent-driven-development` (recommended) or
> `superpowers:executing-plans` to implement this plan task-by-task. Steps use
> checkbox (`- [ ]`) syntax for tracking.

**Goal:** Cut the existing first-party Tool, ToolWorld, Channel, and Face
capabilities directly onto v1 Modules and generated Assembly wiring with no
default product behavior change and no v0 API remaining.

**Architecture:** Add focused public Port SDK packages, give every existing
first-party source a v1 Descriptor, and make `internal/app` consume one
generated Assembly. Preserve implementations and product contracts while
removing the legacy `sdk/plugin`, pluginhost registry, and hand-maintained
generation files.

**Tech Stack:** Go, generated Assembly, YAML Descriptors/Recipes, existing
ChannelHost/FaceHost/Runtime, `vivy-sdk`, `just ci`.

**Spec:** `docs/architecture/VIVY-PLUGIN-SPEC.md`,
`docs/architecture/VIVY-ASSEMBLY.md`, `VIVY-CHANNEL-PACK.md`, and
`VIVY-FACE-PACK.md`.

## Global Constraints

- State: `UNSCHEDULED`; schedule with P1 as one clean-break landing unit.
- Preserve user-visible behavior, config keys, Channel envelopes, RPC, Run,
  Journal, approval, and error semantics.
- Do not physically reorganize working implementation directories in this
  phase.
- P2 completion deletes the v0 SDK and every v0 Descriptor.

---

### Task 1: Freeze the default-body behavior baseline

**Files:**

- Create: `internal/app/default_generation_test.go`
- Create: `sdk/internal/testdata/default-generation.expected.json`
- Modify: `internal/app/realsmoke_test.go`

**Interfaces:**

- Consumes: the current full first-party channel register, built-in Tools,
  ToolWorld behavior, headless default, and configuration overlays.
- Produces: a behavior inventory and Inspect golden that later tasks cannot
  change accidentally.

- [ ] Write `TestDefaultGenerationBaselineInventory` with exact first-party
  Channels, protected Tools, ToolWorld providers, default Face posture, and
  inactive network state; observe RED because no v1 Manifest exposes them.
- [ ] Write a real smoke asserting an unconfigured default start opens no
  Channel or MCP connection.
- [ ] Capture expected public IDs and state only; never snapshot Secret or raw
  Journal data.
- [ ] Run `go test ./internal/app -run DefaultGenerationBaselineInventory` and
  record the pre-cut failure.
- [ ] Commit `test(app): freeze default generation behavior`.

### Task 2: Define focused Tool, ToolWorld, Channel, and Face SDK contracts

**Files:**

- Create: `sdk/port/tool/tool.go`
- Create: `sdk/port/tool/tool_test.go`
- Create: `sdk/port/toolworld/toolworld.go`
- Create: `sdk/port/toolworld/toolworld_test.go`
- Create: `sdk/port/channel/channel.go`
- Create: `sdk/port/channel/channel_test.go`
- Create: `sdk/port/face/face.go`
- Create: `sdk/port/face/face_test.go`

**Interfaces:**

- Consumes: current `sdk/plugin` capability types and preserved product
  contracts.
- Produces: focused Provider interfaces with no `Seam()` or God container.

Target shape:

```go
type ToolProvider interface {
    Definition() Definition
    Invoke(context.Context, Host, json.RawMessage) (Result, error)
}

type ChannelProvider interface {
    Definition() Definition
    Construct(context.Context, Host) (Instance, error)
}

type FaceProvider interface {
    Definition() Definition
    Construct(context.Context, Host) (Instance, error)
}
```

- [ ] Write compile-time interface tests that fail for missing typed methods and
  prove no interface mentions another Port.
- [ ] Preserve Channel message envelopes and Face RPC client semantics in their
  focused packages.
- [ ] Keep Host interfaces capability-scoped; do not copy raw Env access.
- [ ] Run `go test ./sdk/port/tool/... ./sdk/port/toolworld/... ./sdk/port/channel/... ./sdk/port/face/...`.
- [ ] Commit `feat(sdk): split first v1 port contracts`.

### Task 3: Describe the default internal body

**Files:**

- Create: `internal/modules/defaults/catalog.go`
- Create: `internal/modules/defaults/catalog_test.go`
- Create: `recipes/default.vivy.yml`
- Create: `recipes/headless.vivy.yml`
- Create: `recipes/vivy-code.vivy.yml`
- Create: `recipes/minimal.vivy.yml`

**Interfaces:**

- Consumes: internal constructors already composed by `internal/app.New`.
- Produces: T1 Source Catalog entries and explicit product Recipes.

- [ ] Write `TestDefaultCatalogHasOneRequiredInternalProvider` for each required
  `core/*` Port; expected RED is an empty v1 default catalog.
- [ ] Describe existing implementations without moving code or changing
  constructors.
- [ ] Make default Recipe include all established first-party feature Providers
  and public Port Consumers.
- [ ] Make minimal Recipe omit optional organs and prove required edges remain.
- [ ] Make Headless select zero Face and each Interactive recipe select exactly
  one Face.
- [ ] Run `go test ./internal/modules/defaults`.
- [ ] Commit `feat(assembly): describe the default vivy body`.

### Task 4: Convert existing public source modules directly to v1

**Files:**

- Replace: `plugins/*/vivy-plugin.json` with `plugins/*/vivy-module.yaml`
- Modify: `plugins/*/*.go`
- Modify: `plugins/*/*_test.go`
- Replace: `faces/*/vivy-plugin.json` with `faces/*/vivy-module.yaml`
- Modify: `faces/*/*.go`
- Modify: `faces/*/*_test.go`

**Interfaces:**

- Consumes: focused v1 SDK Port packages.
- Produces: native v1 Modules for dingtalk, discord, feishu, qq, telegram,
  hello-fs, lsp, headless, and tui source trees.

- [ ] For each Module, write a failing Descriptor/interface conformance test
  before changing imports.
- [ ] Replace old constructors with typed Port Providers; do not wrap a v0
  `Plugin` or infer a Seam.
- [ ] Preserve Channel settings, message bounds, transports, and lifecycle.
- [ ] Map LSP directly to ToolWorld, Diagnostic Observer, and Status Source
  contracts; do not retain optional interface type assertions.
- [ ] Run each standalone module's own `go test ./...` from its module root.
- [ ] Run `vivy-sdk verify` against each converted source.
- [ ] Commit one coherent batch per Port family, never a mixed behavior change.

### Task 5: Replace hand-maintained registration with generated Assembly

**Files:**

- Delete: `internal/generated/plugins/zz_register.go`
- Delete: `internal/generated/face/zz_face.go`
- Generate: `internal/generated/assembly/zz_default.go`
- Modify: `internal/app/app.go`
- Modify: `internal/app/channels.go`
- Modify: `internal/app/facehost.go`
- Modify: `internal/app/*_test.go`

**Interfaces:**

- Consumes: compiler `AssemblyPlan` and typed Provider sets.
- Produces: one `assembly.BuildDefault()` result consumed by `app.New`.

Target composition result:

```go
type RuntimeAssembly struct {
    Tools     []tool.ToolProvider
    Worlds    []toolworld.Provider
    Channels  []channel.ChannelProvider
    Face      face.FaceProvider
    Manifest  generation.Manifest
}
```

- [ ] Write `TestAppUsesGeneratedRuntimeAssembly`; expected RED is direct calls
  to legacy `plugins.Register()` and `face.Register()`.
- [ ] Generate typed imports and constructor calls from the default Recipe.
- [ ] Make App composition accept only `RuntimeAssembly`; remove legacy slices
  and Seam partitioning.
- [ ] Verify every inbound Channel still reaches the existing ChannelHost and
  `Service.Run`.
- [ ] Run `go test ./internal/app ./internal/channelhost`.
- [ ] Commit `refactor(app): consume the generated v1 assembly`.

### Task 6: Delete the v0 API and registry

**Files:**

- Delete: `sdk/plugin/plugin.go`
- Delete: `sdk/plugin/channel.go`
- Delete: `sdk/plugin/face.go`
- Delete: `internal/pluginhost/`
- Modify: all remaining imports reported by `rg "agent-vivy/sdk/plugin|plugin\.Plugin|\.Seam\("`.

**Interfaces:**

- Consumes: complete v1 source conversion and generated Assembly.
- Produces: a repository with no old public API or runtime registration path.

- [ ] Add a source audit test that fails while any v0 API symbol, v0
  `apiVersion`, or legacy register package remains outside historical docs.
- [ ] Delete the legacy packages and tests rather than forwarding them.
- [ ] Run `rg -n "vivy\.plugin/v0|type Seam|type Plugin interface|plugin\.Plugin" --glob '!docs/**' .`; expected: no matches.
- [ ] Run `go test ./...` and every standalone plugin/face module test.
- [ ] Commit `refactor(plugin): remove the unreleased v0 api`.

### Task 7: Prove default parity and physical removal

**Files:**

- Modify: `internal/app/default_generation_test.go`
- Modify: `sdk/internal/pack_test.go`
- Create: `sdk/internal/removal_conformance_test.go`

**Interfaces:**

- Consumes: default and minimal generated Assemblies.
- Produces: Gate B evidence that behavior remains while omitted code is absent.

- [ ] Make the baseline inventory test pass against the embedded v1 Manifest.
- [ ] Build default and minimal artifacts and inspect both.
- [ ] Assert an omitted external Module has no import, constructor, asset, Port
  edge, or Grant in the minimal artifact.
- [ ] Run `vivy-sdk pack --recipe recipes/default.vivy.yml --output dist/default-v1`
  and `vivy-sdk inspect-artifact dist/default-v1`.
- [ ] Start the default artifact without credentials and verify no network
  instance activates.
- [ ] Run `just ci`.
- [ ] Commit `test(assembly): prove default parity and module removal`.

## Phase exit and rollback

Exit requires no functional v0 source, full default inventory parity, explicit
inactive network state, and physical-removal proof. P1 and P2 land together.
Rollback switches the branch back to the pre-v1 commit; no compatibility layer
is introduced.
