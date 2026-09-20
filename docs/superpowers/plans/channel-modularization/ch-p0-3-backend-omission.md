# CH-P0-3 Backend Omission Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> `superpowers:executing-plans` or `superpowers:subagent-driven-development`,
> plus `superpowers:test-driven-development` and
> `superpowers:verification-before-completion`. Implement only this Story.
> Do not move Channel UI or claim Epic release completion.

**Story:** CH-P0-3

**Parent:** [#42 — Channel subsystem modularization](https://github.com/ProjectViVy/agent-vivy/issues/42)

**Index:** [Channel Modularization Epic Execution Index](index.md)

**Status:** READY

**Planning baseline:** `feat/channel-modularization@1be71c4d0892967fd8f4192abb1c2414dad9f498`

**Immediate predecessor:** CH-P0-2, accepted at
`docs/logs/2026-09-19-channel-p0-2/acceptance.md`

**Requirements:** CH-R1, CH-R2, CH-R3, CH-R7

**Goal:** Make backend Channel selection physically optional, add real
no-channel and Telegram-only Recipes, and prove generated/Manifest/Inspect/live
runtime truth without beginning UI modularization.

**Architecture:** Keep `channelcontract` fields in the stable application
shape, but make the canonical implementation import conditional. Bind
`vivy/channel-host` directly to `internal/modules/channel.NewFactory`; delete
the transitional defaults bridge that otherwise drags the implementation and
platform closure into every binary. Derive compiled Channel state from the
selected Host, reject active configuration for omitted Providers, and use the
same Pack build inputs for a `go list -deps` removal proof.

**Tech Stack:** Go 1.26, Vivy Module/Port v1 compiler, generated Go Assembly,
YAML Recipes, Pack/Inspect, JSON-RPC/WebSocket smoke tests.

**Spec:** `docs/architecture/CHANNEL-MODULARIZATION.md`, especially CH-02,
CH-06 through CH-09, CH-12, and the CH-P0-3 slice fence.

## Global Constraints

- Branch from the current aggregate head after verifying the P0-2 acceptance
  log. Do not branch from the historical planning SHA if the aggregate moved.
- Use `.agents/skills/vivy-plugin/SKILL.md` and the `vivy-kernel-ci` companion
  skill before compiler, generator, Host, Recipe, or Inspect edits.
- `vivy/channel-host` remains the canonical T1 owner of
  `core/channel-host@v1`; Host selected with zero Providers is legal.
- Preserve `channelcontract.Factory`/`Owned`, `std/channel@v1`, the single Run
  callback, Channel management wire shape, overlay persistence, and default
  five-Provider behavior.
- An omitted backend must have no implementation Module, Provider, platform
  SDK, management contribution, listener, worker, or delivery seam. A nil
  factory alone is not acceptance evidence.
- Do not edit `ui/src/components/settings`, `ui/src/lib/rpc.ts`,
  `ui/src/plugins/presentation-host.tsx`, UI localization, or
  `ui/src/generated/assembly.ts` in this Story.
- Do not add `vivy/channel-ui`, `ui.extensions`,
  `web-channels-no-ui.vivy.yml`, browser visibility logic, or asset claims.
- No build tags, runtime loader, reflection-based feature switch, second
  registry, or hand-edited generated file.

## Review Focus

- The default catalog must import `internal/modules/channel` only through a
  selected generated binding; `internal/modules/defaults` must not import it.
- `channels=UNCONFIGURED` means the Host is compiled, including the legal
  zero-Provider case. It must not be inferred from `std/channel@v1` count.
- A configured `enabled: true` omitted Provider fails before runtime
  construction. `enabled: false` remains stored and inert; unrelated settings
  writes do not delete it.
- The primary removal proof is the exact Go dependency list for the same
  overlay/modfile Pack uses. Generated-source string scans are secondary.
- The no-channel executable must boot ordinary control-plane paths, advertise
  no `channel.*` capability, and return standard `MethodNotFound` for
  `channel/inspect`, `channel/get`, and `channel/update`.

## Minimal interface boundary

```text
defaults Source Catalog --selected binding--> internal/modules/channel.NewFactory
Recipe -> Compiler -> AssemblyPlan -> generated import/factory/Manifest
Pack build inputs -> go build
                 \-> go list -deps (test evidence, identical overlay/modfile)
app config validation -> sealed Manifest module/provider inventory
app/RPC core -X-> concrete Channel Host or platform SDK
```

No new public interface is needed. The only private refactor is a Pack build
input helper shared by `go build` and the removal test so evidence cannot drift
from production compilation.

---

### Task 1: Replace the transitional factory bridge with a conditional binding

**Files:**
- Modify: `internal/modules/defaults/catalog.go`
- Modify: `internal/modules/defaults/catalog_test.go`
- Modify: `internal/modules/defaults/constructors.go`
- Create: `internal/modules/defaults/constructors_test.go`
- Modify: `sdk/internal/assembly/runtime_generate_test.go`
- Verify: `sdk/internal/assembly/runtime_generate.go`

**Interfaces:**
- Consumes: `GoBinding.ChannelFactoryConstructor` and
  `internal/modules/channel.NewFactory`
- Produces: selected binding
  `agent-vivy/internal/modules/channel`, package `channel`, constructor
  `NewFactory`; zero reference from `internal/modules/defaults/constructors.go`

- [ ] **Step 1: Write the failing catalog/generator tests**

Require the `vivy/channel-host` record to carry exactly:

```go
Binding{
    ImportPath: "agent-vivy/internal/modules/channel",
    Package: "channel",
    ChannelFactoryConstructor: "NewFactory",
}
```

Update `TestGenerateRuntimeAssemblyEmitsCanonicalChannelFactory` so generated
source imports `agent-vivy/internal/modules/channel` and emits
`channel.NewFactory()`. Add an AST/import test that
`internal/modules/defaults/constructors.go` imports neither
`internal/channelcontract` nor `internal/modules/channel` and declares no
`NewChannelFactory`.

- [ ] **Step 2: Confirm RED**

Run:

```bash
go test ./internal/modules/defaults ./sdk/internal/assembly \
  -run 'Test.*Channel.*Binding|TestGenerateRuntimeAssemblyEmitsCanonicalChannelFactory' -count=1
```

Expected: FAIL because the catalog still points at `defaults.NewChannelFactory`.

- [ ] **Step 3: Make the smallest binding change**

Replace `factoryRecord` use for `vivy/channel-host` with a specialized record
whose descriptor stays unchanged but whose binding names the Channel Module
package directly. Remove `NewChannelFactory` and its now-unused imports from
`constructors.go`. Do not move any other defaults constructor.

- [ ] **Step 4: Prove selected and omitted generation**

Add assertions that a selected Host produces one Channel Module import and one
factory initialization, while an omitted Host produces neither string. The
stable `ChannelFactory channelcontract.Factory` field may remain nil in the
generic generated struct.

- [ ] **Step 5: Run and commit**

```bash
go test ./internal/modules/defaults ./sdk/internal/assembly -count=1
git add internal/modules/defaults sdk/internal/assembly/runtime_generate_test.go
git commit -m "feat(channel): make backend factory import conditional"
```

Expected: PASS; focused commit contains no Recipe or UI changes.

### Task 2: Make capability and configuration truth follow sealed selection

**Files:**
- Modify: `sdk/internal/frontend_v1.go`
- Modify: `sdk/internal/frontend_v1_test.go`
- Modify: `sdk/internal/assembly/runtime_generate.go`
- Modify: `sdk/internal/assembly/runtime_generate_test.go`
- Modify: `internal/app/assembly_validate.go`
- Modify: `internal/app/assembly_sources_test.go`
- Modify: `internal/app/app.go`
- Modify: `internal/app/settings_overlay_test.go`
- Modify: `internal/app/settings/settings_test.go`

**Interfaces:**
- Consumes: selected `vivy/channel-host`, `Manifest.Channels`,
  `config.Channels`, `ChannelEnvelope.Enabled`
- Produces: `channels=UNCONFIGURED` iff the Host is compiled;
  `channels=NOT_COMPILED` otherwise; startup rejection for active unavailable
  Provider configuration

- [ ] **Step 1: Add failing Host-truth tests**

Add table cases to `frontend_v1_test.go` and `runtime_generate_test.go`:

| Selection | Expected Channel capability/network state |
|---|---|
| no Host, no Provider | `NOT_COMPILED` |
| Host, zero Providers | `UNCONFIGURED` |
| Host plus Telegram | `UNCONFIGURED` |

The Host/zero-Provider case must fail against the current Provider-count
implementation.

- [ ] **Step 2: Add failing config/selection tests**

In `assembly_sources_test.go`, construct sealed assemblies for no Host and a
Telegram-only Host. Assert:

- no Host + `channels.telegram.enabled: true` fails with
  `configured Channel provider "telegram" is not compiled`;
- Telegram-only + `channels.discord.enabled: true` fails with the same stable
  provider-specific diagnostic;
- an explicit `enabled: true` unavailable entry loaded from `settings.yaml`
  fails before overlay filtering, while an entry with `enabled: false` is
  logged and kept dormant;
- either assembly + unavailable `enabled: false` succeeds; and
- a settings-overlay write to an unrelated compiled provider retains the
  disabled omitted entry byte-for-byte in `settings.yaml`.

- [ ] **Step 3: Confirm RED**

```bash
go test ./sdk/internal ./sdk/internal/assembly ./internal/app ./internal/app/settings \
  -run 'Test.*Channel.*(Capability|Network|Config|Omitted|Unavailable|Preserv)' -count=1
```

Expected: Host/zero-Provider state and active omitted-config cases fail.

- [ ] **Step 4: Derive state from the Host binding**

Change `capabilityStatesForPlan` and generated `NetworkStates["channels"]` to
look for the canonical selected Host with a valid
`ChannelFactoryConstructor`, not a public Provider count. Keep
`Manifest.Channels` as only the selected Provider names.

Extend `validateRuntimeAssemblyConfig` to build a set from
`assembly.Manifest.Channels` and reject every unavailable config entry whose
`Enabled` value is true. Do not delete, normalize, or rewrite disabled entries;
do not make configuration select code. Add
`validateRuntimeAssemblySettings(path, compiledChannels)` before
`applySettingsOverlayAt`: it reads the existing typed settings document and
rejects only an explicitly true unavailable overlay. The existing merge path
continues to warn/drop unavailable entries from the effective process config
without deleting them from disk.

- [ ] **Step 5: Run the focused truth suites and commit**

```bash
go test ./sdk/internal ./sdk/internal/assembly ./internal/app ./internal/app/settings \
  -run 'Test.*Channel.*(Capability|Network|Config|Omitted|Unavailable|Preserv)' -count=1
git add sdk/internal/frontend_v1.go sdk/internal/frontend_v1_test.go \
  sdk/internal/assembly/runtime_generate.go sdk/internal/assembly/runtime_generate_test.go \
  internal/app/assembly_validate.go internal/app/assembly_sources_test.go internal/app/app.go \
  internal/app/settings_overlay_test.go \
  internal/app/settings/settings_test.go
git commit -m "fix(channel): derive backend truth from sealed selection"
```

### Task 3: Add truthful no-channel and single-Provider Recipes

**Files:**
- Create: `recipes/web-no-channels.vivy.yml`
- Create: `recipes/web-telegram-only.vivy.yml`
- Modify: `sdk/internal/frontend_v1_test.go`
- Modify: `sdk/internal/assembly/channel_host_contract_test.go`
- Verify unchanged: `recipes/default.vivy.yml`

**Interfaces:**
- Produces: a Web Generation with no Channel modules/grants and a Web
  Generation with canonical Host + Telegram + only Telegram grants

- [ ] **Step 1: Write failing Recipe compilation tests**

Add a table that loads both exact paths through the production parser/compiler
and asserts selected modules, Port edges, lifecycle order, grants, Host factory,
and `Manifest.Channels`:

```text
web-no-channels: excludes vivy/channel-host and all five provider modules;
                 no channel grants or std/channel edge
web-telegram-only: includes vivy/channel-host and vivy/telegram only;
                   channels=[telegram] and exactly telegram's poll/secret/net grants
```

Both retain the default Web/core modules needed for ordinary chat and tools.

- [ ] **Step 2: Confirm RED**

```bash
go test ./sdk/internal ./sdk/internal/assembly \
  -run 'TestChannelReducedRecipes|Test.*ChannelHost.*Recipe' -count=1
```

Expected: FAIL because the Recipe files do not exist.

- [ ] **Step 3: Create explicit Recipes**

Copy the default module order, remove all six backend Channel modules and all
Channel grants for `web-no-channels`, and retain only Host + Telegram and its
three existing grant approvals for `web-telegram-only`. Do not add UI fields,
UI modules, placeholder hashes, or `web-channels-no-ui`.

- [ ] **Step 4: Prove failure diagnostics remain deterministic**

Extend the Recipe table with in-memory invalid variants: Telegram without
Host must report the conditional Host diagnostic; non-canonical/duplicate Host
cases remain covered by `channel_host_contract_test.go`.

- [ ] **Step 5: Run and commit**

```bash
go test ./sdk/internal ./sdk/internal/assembly \
  -run 'TestChannelReducedRecipes|Test.*ChannelHost' -count=1
git add recipes/web-no-channels.vivy.yml recipes/web-telegram-only.vivy.yml \
  sdk/internal/frontend_v1_test.go sdk/internal/assembly/channel_host_contract_test.go
git commit -m "feat(channel): add reduced backend recipes"
```

### Task 4: Prove physical dependency omission and live method absence

**Files:**
- Modify: `sdk/internal/frontend_v1.go`
- Modify: `sdk/internal/removal_conformance_test.go`
- Create: `sdk/internal/channel_omission_smoke_test.go`
- Modify: `internal/app/realsmoke_test.go`

**Interfaces:**
- Consumes: the exact runtime source overlay, temporary modfile, and external
  source replacements used by Pack
- Produces: private `prepareRuntimeBuildInputs` shared by `go build` and test
  `go list -deps`; real no-channel executable/RPC evidence

- [ ] **Step 1: Write the failing dependency-closure assertion**

Refactor no behavior yet. Add a test that asks the production Pack preparation
path for its overlay/modfile, runs:

```bash
go list -deps -overlay "$overlay_file" -modfile "$modfile" ./cmd/vivy
```

and rejects these import prefixes in the returned exact line set:

```text
agent-vivy/internal/modules/channel
agent-vivy/internal/channelhost
agent-vivy/plugins/dingtalk
agent-vivy/plugins/discord
agent-vivy/plugins/feishu
agent-vivy/plugins/qq
agent-vivy/plugins/telegram
```

Also reject each external platform SDK module path obtained from those five
plugin `go.mod` files. Assert the default closure contains the Channel Module
and the Telegram-only closure contains Channel Module + Telegram but no other
platform plugin. Do not use binary substring absence as the primary proof.

- [ ] **Step 2: Confirm RED**

```bash
go test ./sdk/internal -run TestChannelRecipeDependencyClosure -count=1
```

Expected: FAIL because the Pack preparation seam is not reusable and/or the
defaults bridge retains Channel implementation dependencies.

- [ ] **Step 3: Extract one private Pack build-input helper**

Move only the existing generated-runtime-source, overlay JSON, temporary
modfile, and external source preparation into `prepareRuntimeBuildInputs`.
`Pack` must consume the returned paths unchanged for `go build`; tests consume
the same paths for `go list -deps`. The helper owns a temporary directory and
returns an idempotent cleanup. Do not expose it from `sdk/internal` and do not
change artifact format.

- [ ] **Step 4: Add live executable/RPC smoke**

Pack `web-no-channels`, start the real executable in an isolated
`VIVY_USER_HOME` and loopback address, retrieve `/rpc/bootstrap`, connect with
the existing Gorilla WebSocket pattern from `internal/app/realsmoke_test.go`,
and assert:

- `initialize` succeeds and contains ordinary `session`, `turn`, and core tool
  capabilities but no capability prefixed `channel.`;
- `session/create` succeeds, proving the ordinary durable chat control path;
- each of `channel/inspect`, `channel/get`, and `channel/update` returns code
  `-32601` (`MethodNotFound`); and
- process shutdown is bounded and leaves no child/listener behind.

Add an app-level no-channel Assembly smoke using the existing deterministic
fake model/tool harness in `internal/app/realsmoke_test.go`; one turn must enter
the normal Service loop and complete one compiled tool call without any
Channel owner. This is the chat/tool execution proof; it must not use a live
model or network.

- [ ] **Step 5: Strengthen generated/Inspect assertions**

For the no-channel artifact assert no Channel modules, Port edges, grants,
providers, or Channel catalog rows in Inspect; `CapabilityStates["channels"]`
is `NOT_COMPILED`; generated `zz_assembly.go` has no implementation/plugin
imports. Keep these as secondary evidence alongside the dependency list.

- [ ] **Step 6: Run and commit**

```bash
go test ./sdk/internal -run 'Test(ChannelRecipeDependencyClosure|NoChannelArtifactRPCSmoke|MinimalArtifactPhysicallyOmitsOptionalModules)' -count=1
go test ./internal/app -run 'TestNoChannelAssemblyRunsChatAndToolLoop' -count=1
git add sdk/internal/frontend_v1.go sdk/internal/removal_conformance_test.go \
  sdk/internal/channel_omission_smoke_test.go internal/app/realsmoke_test.go
git commit -m "test(channel): prove physical backend omission"
```

### Task 5: Regenerate, run the P0-3 gate, and publish acceptance evidence

**Files:**
- Regenerate: `internal/generated/assembly/zz_default.go`
- Modify only if producer requests it:
  `sdk/internal/assembly/conformance_results.json`
- Create: `docs/logs/YYYY-MM-DD-channel-p0-3/summary.md`
- Create: `docs/logs/YYYY-MM-DD-channel-p0-3/verification.md`
- Create: `docs/logs/YYYY-MM-DD-channel-p0-3/acceptance.md`
- Modify: `docs/superpowers/plans/channel-modularization/index.md`

- [ ] **Step 1: Regenerate from authority and prove reproducibility**

```bash
go run ./sdk/internal/cmd/generate-default --repo . --output internal/generated/assembly/zz_default.go
gofmt -w internal/generated/assembly/zz_default.go
tmp_generated="$(mktemp)"
go run ./sdk/internal/cmd/generate-default --repo . --output "$tmp_generated"
gofmt -w "$tmp_generated"
cmp internal/generated/assembly/zz_default.go "$tmp_generated"
```

Expected: second output is byte-identical. Never patch the generated file.

- [ ] **Step 2: Run focused Channel/backend gates**

```bash
go test ./internal/channelcontract ./internal/rpccontract ./internal/modules/channel \
  ./internal/channelhost ./internal/rpc ./internal/app/settings ./internal/app \
  ./internal/moduleport ./internal/modules/... ./sdk/internal/assembly ./sdk/internal -count=1
go test ./sdk/internal/conformance \
  -run 'TestCheckedInProviderConformanceMatchesExecutedSuites|TestGeneration(FailureMatrixEvidence|RollbackRestoresCatalogAndLocaleIdentity)' -count=1
```

- [ ] **Step 3: Run the repository gate**

Run `just ci`. If the executor lacks PowerShell/`just`, run every command
represented by `fmt-check`, `ui-ci`, `vet`, `test`, `headless-compile`, and
`plugin-ci`, retaining only the repository's documented
`internal/workflow` carve-out. Record the limitation; do not claim literal
`just ci`.

- [ ] **Step 4: Perform scope and dependency scans**

```bash
rg -n 'internal/(channelhost|modules/channel)|plugins/(dingtalk|discord|feishu|qq|telegram)' \
  internal/app internal/rpc
git diff 1be71c4d0892967fd8f4192abb1c2414dad9f498 -- ui/src/components/settings ui/src/lib/rpc.ts \
  ui/src/plugins/presentation-host.tsx ui/src/generated/assembly.ts
git diff --check
```

Expected: implementation imports absent from app/RPC; UI diff empty; no
whitespace errors.

- [ ] **Step 5: Write evidence and request review**

`verification.md` must include the exact dependency closure lists/digests,
pack/Inspect output for all three backend Recipes, live RPC transcript, and
command results. `acceptance.md` maps CH-R1/R2/R3/R7 and explicitly states
that UI/assets/release matrix remain P0-4/P0-5.

- [ ] **Step 6: Mark complete only after acceptance and commit**

Update the index row from `READY` to `COMPLETE` and P0-4 from `BLOCKED` to
`READY` only after independent review accepts the evidence.

```bash
git add internal/generated/assembly/zz_default.go \
  sdk/internal/assembly/conformance_results.json \
  docs/logs/YYYY-MM-DD-channel-p0-3 \
  docs/superpowers/plans/channel-modularization/index.md
git commit -m "docs(channel): accept backend omission slice"
```

## Story acceptance checklist

- [ ] Default still carries five Providers and unchanged management behavior.
- [ ] Host with zero Providers remains legal and truthfully
  `UNCONFIGURED`/compiled.
- [ ] No-channel and Telegram-only Recipes compile deterministically.
- [ ] Active unavailable Provider config fails; disabled dormant config is
  preserved.
- [ ] No-channel Go dependency closure excludes Host implementation, Providers,
  and platform SDKs.
- [ ] Real no-channel artifact boots, runs ordinary session/chat/tool paths,
  and exposes no Channel methods/capabilities.
- [ ] No Channel UI/config/assets work appears in the diff.
- [ ] Full gates, review, and evidence log pass before successor unblocks.
