# CH-P0-1 Contract and Conformance Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. Do not start CH-P0-2 ownership migration from this plan.

**Story:** CH-P0-1

**Parent:** [#42 — Channel subsystem modularization](https://github.com/ProjectViVy/agent-vivy/issues/42)

**Epic index:** [Channel Modularization Epic Execution Index](channel-modularization/index.md)

**Status:** COMPLETE; see `docs/logs/2026-09-19-channel-p0-1/acceptance.md`

**Baseline:** `main@9e6db43c81e1d7ce249ee785f1b6efd91a09c5bb`

**Immediate predecessors:** none; #42 is the approved architecture authority

**Goal:** Freeze the smallest internal Channel Host composition contracts and executable conformance expectations without moving runtime ownership, changing behavior, or implementing Channel omission/UI modularization.

**Architecture:** Add one implementation-free `internal/channelcontract` package for Channel composition types, and one generic typed RPC contribution seam inside `internal/rpc`. Document `vivy/channel-host` as the canonical build-owned T1 provider of `core/channel-host@v1` (`0..1`) and describe the later generated Assembly dependency, but leave all current app construction, lifecycle, handlers, settings behavior, generated files, and UI unchanged.

**Tech Stack:** Go 1.x, Vivy Module/Port v1, JSON-RPC v1, Go tests, Markdown normative contracts.

**Spec:** `docs/architecture/CHANNEL-MODULARIZATION.md` (created by Task 1 from issue #42)

## Global Constraints

- Follow Module -> typed Port -> Recipe -> generated Assembly -> Generation evidence.
- `vivy/channel-host` is a build-owned T1 Module; public Modules cannot provide `core/channel-host@v1`.
- `core/channel-host@v1` has cardinality `0..1`; selected `std/channel@v1` Providers require the canonical Host.
- Preserve the public `std/channel@v1` ABI and the single `Service.RunWithOptions`, Journal, policy, credential, and storage authorities.
- Constructors perform no networking; lifecycle remains generated-order start and reverse-order stop/close.
- Preserve `channels.<platform>` data, overlay precedence, disabled omitted-provider entries, and restart-required updates.
- Do not add runtime discovery, build tags, a second plugin loader, a second dispatcher, or a public RPC registration API.
- Do not hand-edit `internal/generated/assembly/*` or `ui/src/generated/assembly.ts`.
- CH-P0-1 may add contracts, documentation, fixtures, and tests only. It must not relocate or delete production behavior.

## Scope Fence

| In CH-P0-1 | Hard stop — later slice |
|---|---|
| Normative design and Port/Assembly/Module wording | Real Host owner/factory wiring and app migration (CH-P0-2) |
| Compile-only Channel composition types | Moving binding, configuration interpretation, or management handlers (CH-P0-2) |
| Generic typed RPC contribution validation | Registering Channel methods through contributions (CH-P0-2) |
| Conformance case definitions and negative compiler fixtures | Optional generated imports, reduced Recipes, physical omission (CH-P0-3) |
| Generic UI-extension projection wire type only | Settings registry, Channel UI move, visibility/mount logic (CH-P0-4) |
| Baseline source/digest inventory | Release matrix, packaged artifact/browser evidence (CH-P0-5) |

Stop and request architecture review if implementation requires changing the public Channel ABI, `module.Instance`, JSON-RPC wire behavior, configuration schema, or the approved slice boundary.

## Review Focus

- A public or duplicate `core/channel-host@v1` provider must be rejected, never resolved by selection order (Task 4).
- A `std/channel@v1` Provider without the canonical Host must be an Assembly compile error (Task 4).
- Duplicate or core-method RPC bindings must fail contribution validation deterministically (Task 3).
- An absent RPC contribution must contribute neither methods nor capability names (Task 3).
- Settings for an omitted provider must remain lossless during unrelated settings writes; CH-P0-1 captures this as an existing preservation contract without moving settings code (Task 4).

## Frozen Minimal Interface Boundary

The worker must use these responsibilities and dependency directions. Minor file splitting is allowed; signature changes require review because CH-P0-2 consumes them.

```text
app + generated assembly -> internal/channelcontract <- internal/modules/channel (CH-P0-2)
internal/modules/channel -> internal/rpccontract contribution types
internal/rpc dispatcher -> internal/rpccontract <- validated generic contribution set
internal/channelcontract -X-> internal/channelhost or platform SDKs
internal/rpccontract -X-> internal/rpc or internal/channelhost
```

### `internal/rpccontract/contribution.go` and `internal/rpc/contribution.go`

```go
package rpccontract

type MethodBinding struct {
    Method     string
    Capability string
    Handler    HandlerFunc // context.Context, typed Peer, shared Request/Error
}

type Contribution interface {
    RPCBindings() []MethodBinding
}

func ValidateMethodBindings(coreMethods map[string]struct{}, bindings []MethodBinding) error
```

Rules: trim neither identity nor namespace; reject empty method/capability, nil handler, duplicate methods, and collision with any core method. A capability may intentionally cover more than one method. Preserve `context.Context`, the typed least-authority Peer view, shared `Request`, and existing `*Error` mapping through `HandlerFunc`. `internal/rpc` re-exports these names and compile-checks `*rpc.Peer` against the Peer view. This is a typed build-owned attachment, not a public registry. Validation must sort diagnostic method names so failures are deterministic.

### `internal/channelcontract/contract.go`

```go
package channelcontract

type RunCallback func(context.Context, domain.SessionID, string, *domain.Provenance) (domain.RunID, error)

type CredentialResolver interface {
    Resolve(moduleID, ref string) (string, error)
}

type ProviderConfig struct {
    Enabled   bool
    AllowFrom []string
    TokenEnv  string
    Settings  json.RawMessage
}

type Config map[string]ProviderConfig

type ProviderState struct {
    Name       string
    Compiled   bool
    Configured bool
    Running    bool
    Error      string
}

type State struct {
    Compiled         bool
    ProcessAvailable bool
    Providers        []ProviderState
}

type ChannelOverlay struct {
    Name      string
    Enabled   *bool
    AllowFrom *[]string
    TokenEnv  *string
}

type Settings struct { Channels []ChannelOverlay }

type SettingsAccess interface {
    Read(context.Context) (Settings, error)
    Update(context.Context, ChannelOverlay) (Settings, error)
    Writable() bool
    Frozen() bool
}

type Dependencies struct {
    Journal     storage.Journal
    Messages    storage.MessageStore
    Sessions    storage.SessionStore
    Credentials CredentialResolver
    Settings    SettingsAccess
    Logger      *slog.Logger
    Run         RunCallback
    OnSettingsChanged func()
}

type Selection struct {
    Providers []channel.ChannelProvider
    Grants    map[string][]module.GrantBinding
    Config    Config
}

type Factory interface {
    Construct(context.Context, Dependencies, Selection) (Owned, error)
}

type Owned interface {
    module.Instance
    runtime.RunHook
    runtime.ChannelDeliverer
    rpccontract.Contribution
    Inspect() State
}
```

`Config`, `ProviderConfig`, `State`, `ChannelOverlay`, and `Settings` are inert contract-owned data with the exact fields above. The later implementation adapter converts the current YAML node into bounded JSON without exposing a platform type. `SettingsAccess` is the only settings authority: its adapter preserves the rest of the shared document while exposing Channel projection reads, atomic overlay upserts, and writable/frozen policy. These types must not validate a platform, start providers, or import `internal/channelhost`. Keep `CredentialResolver` local: its methods deliberately match the current `internal/channelhost.CredentialResolver` without importing that implementation package or expanding `sdk/port/channel`.

`State` must distinguish `Compiled`, `ProcessAvailable`, and provider runtime states so later `WithoutEars()` handling cannot conflate artifact presence with an active process. No RPC DTOs belong in `channelcontract`.

### Generic UI projection

Add only the wire contract needed by later slices:

```go
type UIExtensionProjection struct {
    ID      string `json:"id"`
    Enabled bool   `json:"enabled"`
}
```

It belongs with the generic `initialize`/`capabilities` response contract, not in `channelcontract`. CH-P0-1 pins serialization and secret-free fields; it does not populate, negotiate, or consume the projection.

---

### Task 1: Freeze the normative Channel modularization design

**Files:**
- Create: `docs/architecture/CHANNEL-MODULARIZATION.md`
- Modify: `docs/architecture/VIVY-MODULE-STANDARD.md`
- Modify: `docs/architecture/VIVY-PORT-CATALOG.md`
- Modify: `docs/architecture/VIVY-ASSEMBLY.md`

**Interfaces:**
- Consumes: issue #42 at baseline `9e6db43`
- Produces: authoritative CH-P0 requirements `CH-01` through `CH-12`, module/Port ownership, lifecycle, absence semantics, and the CH-P0-1 scope fence

- [ ] **Step 1: Record the verified baseline**

Run:

```bash
git rev-parse HEAD
rg -n 'channelhost.New|bindChannels|Channels: +channelHost|NewChannelHost' internal/app internal/modules/defaults
rg -n 'internal/channelhost|channel/inspect|channel/get|channel/update' internal/rpc/control.go
```

Expected: revision `9e6db43c81e1d7ce249ee785f1b6efd91a09c5bb`; manual construction in `internal/app/app.go`; no-op owner in `internal/modules/defaults/constructors.go`; concrete RPC coupling in `internal/rpc/control.go`.

- [ ] **Step 2: Write the design with stable requirement IDs**

Use these IDs verbatim:

```text
CH-01 canonical T1 owner: vivy/channel-host provides core/channel-host@v1 (0..1)
CH-02 std/channel@v1 provider conditionally requires the canonical Host
CH-03 factory/owned instance is the only composition seam
CH-04 one Service.RunWithOptions path; unchanged L0 authorities
CH-05 generic typed RPC contribution; no raw public registration
CH-06 absent contribution means MethodNotFound and no channel.* capabilities
CH-07 compiled, process-available, configured, and running are distinct states
CH-08 stored omitted-provider configuration is inert and lossless
CH-09 generated imports are the implementation-inclusion authority
CH-10 UI projection is generic, secret-free, and not build selection
CH-11 reverse-order rollback and idempotent Stop/Close
CH-12 default Generation remains functionally complete
```

The design must include current coupling, chosen boundaries, normal/absent/failure paths, performance/economy notes, rollback, evidence, and explicit CH-P0-2..5 exclusions. State that Eino is not implicated because the existing Run callback path is preserved.

- [ ] **Step 3: Amend existing normative contracts narrowly**

In the Module Standard, define canonical build-owned internal providers. In the Port Catalog, add `core/channel-host@v1`, `0..1`, canonical T1 provider only, and amend the old L0 wording without changing FaceHost. In Assembly, specify conditional Provider -> Host compilation and distinguish compiled inventory from process availability. Link each amendment to `CHANNEL-MODULARIZATION.md`; do not duplicate its complete prose.

- [ ] **Step 4: Verify documentation integrity**

Run:

```bash
rg -n 'CH-(0[1-9]|1[0-2])' docs/architecture/CHANNEL-MODULARIZATION.md
rg -n 'core/channel-host@v1|vivy/channel-host' docs/architecture/{VIVY-MODULE-STANDARD,VIVY-PORT-CATALOG,VIVY-ASSEMBLY}.md
git diff --check
```

Expected: all 12 IDs appear; each normative document contains the canonical identity; no whitespace errors.

- [ ] **Step 5: Commit the contract freeze**

```bash
git add docs/architecture/CHANNEL-MODULARIZATION.md docs/architecture/VIVY-MODULE-STANDARD.md docs/architecture/VIVY-PORT-CATALOG.md docs/architecture/VIVY-ASSEMBLY.md
git commit -m "docs(channel): freeze modular host contract"
```

**Evidence:** baseline command output and requirement-to-document mapping.

### Task 2: Add implementation-free Channel composition types

**Files:**
- Create: `internal/channelcontract/contract.go`
- Create: `internal/channelcontract/contract_test.go`
- Verify unchanged: `internal/app/app.go`, `internal/app/channels.go`, `internal/channelhost/*`

**Interfaces:**
- Consumes: existing `module.Instance`, `runtime.RunHook`, `runtime.ChannelDeliverer`, storage interfaces, domain IDs/provenance, `channel.ChannelProvider`, and `module.GrantBinding`
- Produces: `RunCallback`, `Dependencies`, `Selection`, `Factory`, `Owned`, inert `Config`/`ProviderConfig`/`State`

- [ ] **Step 1: Write compile-time contract tests**

Tests must prove a fake owned instance satisfies all five facets and that zero-provider `Selection` is valid data. Use compile assertions and a fake factory; do not instantiate `channelhost.Host`.

```go
var _ channelcontract.Owned = (*fakeOwned)(nil)
var _ channelcontract.Factory = fakeFactory{}

func TestSelectionAllowsZeroProviders(t *testing.T) {
    got := channelcontract.Selection{Config: channelcontract.Config{}}
    if len(got.Providers) != 0 { t.Fatalf("providers = %d, want 0", len(got.Providers)) }
}
```

- [ ] **Step 2: Confirm the tests fail before the package exists**

Run: `go test ./internal/channelcontract -count=1`

Expected: FAIL because the package/contracts do not exist.

- [ ] **Step 3: Implement only the frozen types**

Add the interfaces above plus minimal inert data. Add comments assigning validation, settings I/O, construction, and lifecycle to the later canonical implementation. Imports must not include `internal/channelhost`, any package under `plugins/`, or platform SDKs.

- [ ] **Step 4: Verify package isolation**

Run:

```bash
go test ./internal/channelcontract -count=1
go list -deps ./internal/channelcontract | rg 'agent-vivy/internal/channelhost|agent-vivy/plugins/(telegram|dingtalk|feishu|qq|discord)' && exit 1 || true
```

Expected: tests PASS; forbidden dependency search prints nothing.

- [ ] **Step 5: Prove production wiring did not move**

Run:

```bash
git diff --exit-code 9e6db43 -- internal/app/app.go internal/app/channels.go internal/channelhost
```

Expected: PASS with no diff.

- [ ] **Step 6: Commit the composition contract**

```bash
git add internal/channelcontract
git commit -m "feat(channel): define internal host composition contract"
```

**Evidence:** passing package test, empty forbidden-dependency output, unchanged production wiring.

### Task 3: Add and validate generic RPC contributions

**Files:**
- Create: `internal/rpccontract/protocol.go`
- Create: `internal/rpccontract/contribution.go`
- Create: `internal/rpc/contribution.go` as compatibility aliases/bridge
- Create: `internal/rpc/contribution_test.go`
- Modify: `internal/rpc/control.go` only to centralize/read the immutable core-method set if required by validation tests; do not dispatch contributions

**Interfaces:**
- Consumes: existing RPC protocol wire shapes and the public operations of `rpc.Peer`
- Produces: `MethodBinding`, `Contribution`, `ValidateMethodBindings`

- [ ] **Step 1: Write table-driven validation tests**

Cover: valid binding; empty method; empty capability; nil handler; duplicate method; repeated capability across distinct methods is accepted; core-method collision; deterministic sorted diagnostic. Also assert an empty binding slice validates and yields no implied capability.

```go
valid := rpccontract.MethodBinding{
    Method: "channel/inspect", Capability: "channel.inspect",
    Handler: func(context.Context, rpccontract.Peer, rpccontract.Request) (any, *rpccontract.Error) { return nil, nil },
}
```

- [ ] **Step 2: Confirm RED**

Run: `go test ./internal/rpccontract ./internal/rpc -run 'TestValidateMethodBindings|TestEmptyContribution' -count=1`
Expected: FAIL because the contribution types do not exist.

- [ ] **Step 3: Implement deterministic validation**

Return ordinary Go errors from assembly-time validation. Do not wrap handlers, alter request context, expose a mutable dispatcher, or add Channel-specific logic. If the core-method set is extracted from the switch, preserve the exact current method vocabulary and behavior.

- [ ] **Step 4: Verify RPC behavior remains unchanged**

Run:

```bash
go test ./internal/rpccontract ./internal/rpc -run 'TestValidateMethodBindings|TestEmptyContribution|TestControlHandler' -count=1
git diff 9e6db43 -- internal/rpc/control.go
```

Expected: tests PASS; any `control.go` diff is limited to immutable method-name data with no Channel handler relocation, capability change, or dispatch change.

- [ ] **Step 5: Commit the generic seam**

```bash
git add internal/rpccontract internal/rpc/contribution.go internal/rpc/contribution_test.go internal/rpc/protocol.go
git commit -m "feat(rpc): define typed method contributions"
```

**Evidence:** validation matrix results and reviewed `control.go` diff.

### Task 4: Encode CH-P0 compiler/conformance cases without enabling omission

**Files:**
- Create: `sdk/internal/assembly/testdata/channel-host/valid-empty-host.vivy.yml`
- Create: `sdk/internal/assembly/testdata/channel-host/provider-without-host.vivy.yml`
- Create: `sdk/internal/assembly/testdata/channel-host/duplicate-host.vivy.yml`
- Create: `sdk/internal/assembly/testdata/channel-host/public-core-provider.vivy.yml`
- Create: `sdk/internal/assembly/channel_host_contract_test.go`
- Modify: `sdk/internal/assembly/graph.go` and/or `compiler.go` only for general conditional-dependency/core-provider validation required to make negative fixtures pass
- Modify: `internal/app/settings/settings_test.go` only to pin lossless omitted-provider round-trip if existing coverage is insufficient

**Interfaces:**
- Consumes: `core/channel-host@v1` catalog decision, existing graph/compiler diagnostic conventions, current settings YAML preservation
- Produces: executable cases for CH-01, CH-02, CH-08; no generated Assembly or reduced product Recipe

- [ ] **Step 1: Inspect and reuse the existing fixture schema**

Run:

```bash
find sdk/internal/assembly/testdata -maxdepth 2 -type f | sort | head -40
sed -n '1,240p' sdk/internal/assembly/graph_test.go
sed -n '1,220p' sdk/internal/assembly/core_ports_test.go
```

Expected: use the repository's actual Recipe/descriptor fixture form and diagnostic assertion style; do not invent a parallel fixture loader.

- [ ] **Step 2: Add four focused cases**

Assert:

```text
valid-empty-host: canonical Host, zero std/channel Providers -> compile succeeds
provider-without-host: std/channel Provider only -> missing core/channel-host@v1 error
duplicate-host: two core/channel-host@v1 Providers -> 0..1 cardinality error
public-core-provider: T2/public source provides core/channel-host@v1 -> authority error
```

These are graph/contract cases only. Do not create `recipes/web-no-channels.vivy.yml`, generate imports, or build an omission artifact.

- [ ] **Step 3: Run RED and capture exact gaps**

Run: `go test ./sdk/internal/assembly -run TestChannelHostContract -count=1`

Expected before minimal compiler work: one or more cases fail because current optional catalog/graph lacks the approved Host relationship. Record each failing case; a failure is CH-P0-1 evidence, not permission to implement CH-P0-3 generation.

- [ ] **Step 4: Add only general validation needed by these cases**

Prefer existing Port cardinality, trust, and required-edge machinery. Add no Channel implementation import, generated constructor, default selection change, Recipe, build tag, or asset rule. Keep the production default graph unchanged.

- [ ] **Step 5: Pin lossless omitted configuration**

If `internal/app/settings/settings_test.go` does not already prove it, add a test that loads settings containing an unknown Channel overlay, performs an unrelated settings mutation through the existing save/update path, reloads, and compares the complete Channel overlay. Do not move validation or persistence code.

- [ ] **Step 6: Verify focused conformance**

Run:

```bash
go test ./sdk/internal/assembly -run 'TestChannelHostContract|TestCompilePluginV1GraphFixtures' -count=1
go test ./internal/app/settings -run 'Test.*Channel.*(RoundTrip|Preserv)' -count=1
go test ./internal/modules/... -count=1
```

Expected: all PASS; no generated Assembly diff and no new product Recipe.

- [ ] **Step 7: Commit the conformance foundation**

```bash
git add sdk/internal/assembly internal/app/settings/settings_test.go
git commit -m "test(channel): pin host assembly conformance"
```

**Evidence:** per-fixture outcomes, exact diagnostics, settings before/after equality.

### Task 5: Pin the generic UI projection wire shape and close CH-P0-1

**Files:**
- Modify: `internal/rpc/control.go` (type declaration only; no population)
- Modify: `internal/rpc/control_test.go`
- Modify: `ui/src/lib/rpc.ts`
- Modify: `ui/src/lib/rpc.test.ts`
- Create: `docs/logs/2026-09-19-channel-p0-1/summary.md`
- Create: `docs/logs/2026-09-19-channel-p0-1/verification.md`
- Create: `docs/logs/2026-09-19-channel-p0-1/acceptance.md`

**Interfaces:**
- Consumes: existing `initialize`/`capabilities` response and frontend `RpcCapabilities`
- Produces: optional `ui_extensions` wire field typed as `{id, enabled}[]`; source/digest baseline and CH-P0-1 evidence index

- [ ] **Step 1: Add backend/frontend serialization tests first**

Pin: omitted field remains backward compatible; present field serializes only `id` and `enabled`; frontend accepts missing and present values. Do not add `channel-ui`, mount logic, settings UI, or capability negotiation.

- [ ] **Step 2: Run RED**

Run:

```bash
go test ./internal/rpc -run TestUIExtensionProjection -count=1
cd ui && pnpm test -- --run src/lib/rpc.test.ts
```

Expected: new tests fail until the generic types are added.

- [ ] **Step 3: Add the optional wire types without populating them**

Backend JSON name is `ui_extensions`; frontend property is `ui_extensions?`. Do not alter the capabilities list or derive Channel visibility in this slice.

- [ ] **Step 4: Capture the baseline and scope evidence**

Record in `verification.md`:

```bash
git rev-parse HEAD
git ls-tree -r HEAD internal/channelhost internal/app/channels.go internal/rpc/control.go internal/modules/defaults sdk/internal/assembly ui/src/components/settings ui/src/lib/api.ts
go list -deps ./cmd/vivy
git diff --name-status 9e6db43
```

The logs must state explicitly that runtime ownership, omission, reduced Recipes, generated wiring, and Channel UI are not implemented. Record current source tree object IDs as baseline evidence; do not claim them as release artifact digests.

- [ ] **Step 5: Run the CH-P0-1 gate**

Run:

```bash
go test ./internal/channelcontract ./internal/rpc ./internal/app/settings ./internal/modules/... ./sdk/internal/assembly -count=1
git diff --check
just ci
```

Expected: focused suites and `just ci` PASS. If `just ci` fails for an environment reason, record the command, failing stage, and raw cause; do not mark acceptance passed.

- [ ] **Step 6: Prove later slices did not leak in**

Run:

```bash
test ! -e recipes/web-no-channels.vivy.yml
test ! -e recipes/web-channels-no-ui.vivy.yml
test ! -e recipes/web-telegram-only.vivy.yml
git diff --exit-code 9e6db43 -- internal/generated/assembly ui/src/generated/assembly.ts ui/src/components/settings
rg -n 'channelcontract' internal/app internal/modules/channel 2>/dev/null && exit 1 || true
```

Expected: all commands PASS; the new contract is not yet production-wired.

- [ ] **Step 7: Commit closure evidence**

```bash
git add internal/rpc/control.go internal/rpc/control_test.go ui/src/lib/rpc.ts ui/src/lib/rpc.test.ts docs/logs/2026-09-19-channel-p0-1
git commit -m "docs(channel): record p0 contract foundation evidence"
```

**Evidence:** wire tests, focused gate, `just ci`, baseline tree IDs, scope-fence commands.

## Acceptance Map

| Requirement | Acceptance evidence |
|---|---|
| CH-01/CH-02 authority and dependency | Task 1 normative text + Task 4 graph fixtures |
| CH-03 minimal seam | Task 2 compile assertions and dependency closure |
| CH-04 single runtime authority | unchanged production wiring diff + design contract |
| CH-05/CH-06 RPC contribution semantics | Task 3 validation matrix; no dispatcher wiring |
| CH-07 truthful state vocabulary | `channelcontract.State` tests and design definitions |
| CH-08 lossless omitted settings | existing or extended settings round-trip test |
| CH-09 generated-import authority | Assembly normative amendment; no generator change yet |
| CH-10 generic UI projection | backend/frontend wire tests; no Channel consumer |
| CH-11 lifecycle/rollback contract | `Owned` embeds `module.Instance`; normative ordering |
| CH-12 default remains complete | `just ci`; no Recipe/generated/default wiring diff |

## Completion Definition

CH-P0-1 is complete only when the contracts compile, the focused conformance cases pass, `just ci` passes (or is honestly recorded as blocked), and the scope-fence commands prove no CH-P0-2 through CH-P0-5 implementation entered the change. It does **not** make Channel removable, change runtime ownership, add reduced Recipes, or modularize the UI.

## Executor Handoff

Read issue #42, `docs/architecture/CHANNEL-MODULARIZATION.md` after Task 1, the four normative v1 architecture documents, and `.agents/skills/vivy-plugin/SKILL.md`. Work on one branch/worktree, execute tasks in order, and return commit IDs plus the evidence named by each task. Stop on a required public-interface change, a baseline mismatch, or pressure to wire the new contracts into production; those decisions belong to CH-P0-2 or later.
