# CH-P0-4 UI Modularization and Configuration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> `superpowers:executing-plans` or `superpowers:subagent-driven-development`,
> `superpowers:test-driven-development`, and
> `superpowers:verification-before-completion`. Load `oil-frontend` before UI
> edits when that project skill is available. Stop and report the missing skill
> if the executor requires it but cannot load it. Do not begin until CH-P0-3 is
> accepted in the Epic index.

**Story:** CH-P0-4

**Parent:** [#42 — Channel subsystem modularization](https://github.com/ProjectViVy/agent-vivy/issues/42)

**Index:** [Channel Modularization Epic Execution Index](index.md)

**Status:** BLOCKED pending accepted CH-P0-3

**Planning baseline:** `feat/channel-modularization@1be71c4d0892967fd8f4192abb1c2414dad9f498`; execution rebases onto the accepted CH-P0-3 aggregate head

**Immediate predecessor:** accepted CH-P0-3

**Requirements:** CH-R4, CH-R5, CH-R7

**Goal:** Make the dedicated Channel browser UI a selected
`vivy/channel-ui` Module, add typed settings and Cron delivery contributions,
negotiate backend visibility before installation, and prove disabled/omitted UI
has no mount, request, subscription, catalog, or dedicated asset.

**Architecture:** Add a build-owned UI binding to the sealed Source Catalog and
derive the Channel extension input from the same `AssemblyPlan` that selected
its Module; Recipe module selection remains the inclusion authority. Reuse the
existing `std/ui-extension@v1` compiler and PresentationHost `components`
registry, adding only typed adapters over that registry for settings sections
and Cron delivery targets. The backend emits a generic, secret-free
`ui_extensions` projection. The root filters generated extensions before
PresentationHost installation and keys the host by negotiation epoch so
reconnect disposes all old registrations.

**Tech Stack:** Go 1.26, Vivy UI Assembly, React 19, TypeScript 5.8, Vite 7,
Zustand, TanStack Router, Vitest, Playwright, JSON-RPC v1.

**Spec:** `docs/architecture/CHANNEL-MODULARIZATION.md`, especially CH-09,
CH-10, CH-12, UI projection contract, absence behavior, and CH-P0-4 slice.

## Global Constraints

- Start only from an accepted P0-3 aggregate head; preserve its backend
  dependency-closure and no-channel live evidence.
- Follow root `AGENTS.md`, `ui/AGENTS.md`, and
  `.agents/skills/vivy-plugin/SKILL.md`. Generated UI source is regenerated,
  never patched by hand.
- `vivy/channel-ui` provides existing `std/ui-extension@v1` and requires the
  canonical PresentationHost. Do not add a new UI Port, loader, root, React
  tree, registry, or runtime import mechanism.
- Recipe selection compiles code. `ui.extensions.vivy/channel-ui.enabled` only
  controls installation at startup; it never selects or loads code.
- Omitted config defaults enabled only when the extension is compiled. Applying
  a config change requires process restart and browser reload.
- Browser installation requires all three facts: generated local extension,
  backend projection `enabled:true`, and negotiated `channel.inspect`,
  `channel.get`, `channel.update` capabilities.
- Disabled/omitted means no settings tab/content, Channel store subscription,
  Channel RPC call, wizard, Cron Channel target contribution, or stale deep
  link. Omitted additionally means no Channel catalog/module source/dedicated
  assets in the final artifact.
- Keep generic historical provenance in core. Old messages remain readable;
  unknown Channel names/icons fall back to generic text.
- Do not change Channel backend ownership, Provider ABI, platform protocols,
  settings overlay semantics, or P0-3 Recipes beyond adding/removing the UI
  Module as specified.
- Do not implement the release-wide four-artifact/rollback record; P0-5 owns
  that closure.

## Review Focus

- `recipe.modules` plus a build-owned Source Catalog UI binding is the sole
  selection authority for first-party Channel UI. Do not maintain a second
  hand-authored Channel feature list under `recipe.ui.extensions`.
- The existing generic `components` registry remains the storage primitive.
  Typed settings/Cron adapters validate reserved IDs and read that registry;
  they are not parallel registries.
- No Channel RPC request may occur during import, extension installation, a
  disabled/omitted generation, or core Settings/Cron render without the
  corresponding contribution mounted.
- `ui_extensions` is copied through Go RPC, the browser transport snapshot,
  Zustand initialization, SDK host types, and reconnect/reset without losing
  or retaining stale values.
- Static Channel components, schema, icons, platform metadata, active typed API
  client, store, and EN/ZH messages leave core `ui/src` and live under the
  selected Module source/catalog. Three inert source-compatibility API shims
  may remain in core until the next UI SDK version.
- Default and Telegram-only remain UI-bearing; no-channel and
  backend-without-UI have no Channel UI selection.

## Minimal interfaces

### Build-owned UI binding

Add to `sdk/internal/assembly/source.go` and mirror in the defaults catalog:

```go
type UIBinding struct {
    ID     string
    Port   string
    Entry  string
    Export string
}

type SourceRecord struct {
    // existing fields
    UIBindings []UIBinding
}

type ResolvedModule struct {
    // existing fields
    UIBindings []UIBinding
}
```

`UIBinding` is Source Catalog metadata, not Module-controlled input. For
`vivy/channel-ui` it is exactly:

```text
ID: vivy/channel-ui
Port: std/ui-extension@v1
Entry: ./ui/src/index.tsx
Export: extension
```

`deriveSelectedUIInput` appends selected build-owned bindings to the existing
explicit UI input used by external Modules, rejects duplicate Provider IDs,
and then passes one combined input through existing hash, dependency, order,
staging, generation, and Manifest code. It never scans directories.

### Typed component adapters

Add host-owned types in `ui/src/plugins/component-contributions.tsx`:

```ts
export const SETTINGS_SECTION_PREFIX = 'vivy.settings.section/';
export const CRON_DELIVERY_TARGET_PREFIX = 'vivy.cron.delivery-target/';

export interface SettingsSectionContribution {
  readonly id: string;
  readonly order: number;
  readonly label: string;
  readonly render: () => React.ReactNode;
}

export interface CronDeliveryTargetContribution {
  readonly id: string;
  readonly order: number;
  readonly createInitialValue: () => unknown;
  readonly renderEditor: (props: CronDeliveryEditorProps) => React.ReactNode;
  readonly renderSummary: (value: unknown) => string;
  readonly validate: (value: unknown) => string | undefined;
}

export function useSettingsSections(): readonly SettingsSectionContribution[];
export function useCronDeliveryTargets(): readonly CronDeliveryTargetContribution[];
```

These hooks read validated active values from PresentationHost's existing
`components` registry. Reserved-prefix duplicate active IDs fail extension
installation deterministically. Cleanup removes entries in reverse install
order.

### Startup UI configuration and projection

Add strict YAML types:

```go
type UI struct {
    Extensions map[string]UIExtension `yaml:"extensions"`
}

type UIExtension struct {
    Enabled *bool `yaml:"enabled"`
}
```

`nil` means enabled. Validation accepts only canonical Module IDs and the
`enabled` key. The app passes an already evaluated
`[]rpc.UIExtensionProjection` to `ControlDeps`. RPC serializes it without
credentials or Provider settings and does not evaluate Channel state.

---

### Task 1: Populate and preserve the generic backend UI projection

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `config.example.yaml`
- Modify: `internal/rpc/control.go`
- Modify: `internal/rpc/control_test.go`
- Modify: `internal/app/app.go`
- Create: `internal/app/ui_extensions.go`
- Create: `internal/app/ui_extensions_test.go`
- Modify: `ui/src/lib/rpc.ts`
- Modify: `ui/src/lib/rpc.test.ts`
- Modify: `ui/src/lib/store.ts`
- Modify: `ui/src/lib/store.test.ts`
- Modify: `sdk/ui/src/module.ts`
- Modify: `sdk/ui/src/module.test.ts`

**Interfaces:**
- Consumes: sealed `Manifest.UI.Extensions`, startup config, Channel owner
  `Inspect().ProcessAvailable`, contributed capabilities
- Produces: secret-free `ui_extensions` plus a browser negotiation epoch

- [ ] **Step 1: Write failing config tests**

Require strict decode, validation, and defaults for:

```yaml
ui:
  extensions:
    vivy/channel-ui:
      enabled: false
```

Assert omitted `enabled` is effectively true, explicit false is false, unknown
fields fail strict decoding, invalid Module IDs fail validation, and no config
path adds a Module to the Manifest.

- [ ] **Step 2: Write failing projection tests**

In `ui_extensions_test.go` table these cases:

| Compiled UI | Config | Channel owner/process | required capabilities | Wire result |
|---:|---|---|---|---|
| no | omitted | absent | absent | omitted row |
| yes | omitted | available | all three | `enabled:true` |
| yes | false | available | all three | `enabled:false` |
| yes | true | unavailable | all three | `enabled:false` |
| yes | true | available | one missing | `enabled:false` |

The helper returns rows only for compiled UI extensions. It may special-case
Channel process truth through `channelcontract.State` at the app composition
boundary, but RPC receives only the evaluated generic row and must not import a
Channel contract or implementation.

- [ ] **Step 3: Confirm backend RED**

```bash
go test ./internal/config ./internal/app ./internal/rpc \
  -run 'Test.*UIExtension|TestUIExtensionProjectionSerialization' -count=1
```

Expected: config fields/evaluator/dependency injection are missing.

- [ ] **Step 4: Implement strict config and projection injection**

Add `Config.UI`, validation, and `effectiveUIExtensionEnabled`. Add
`UIExtensions []UIExtensionProjection` to `ControlDeps`, defensively copy it
in `NewControlHandler`, and return it for both `initialize` and
`capabilities`. Reject duplicate/blank projection IDs at handler construction;
sort by ID for deterministic wire output.

In app composition, compute Channel UI's process result only through
`channelcontract.Owned.Inspect()` and the validated contributed capability set.
Do not import `internal/modules/channel` or `internal/channelhost`.

- [ ] **Step 5: Write browser transport/store RED tests**

Require `RpcClient.connect` and `getRpcCapabilitiesSnapshot` to preserve a
detached `ui_extensions` copy; close/reset clears it. Add
`uiExtensions: UIExtensionProjection[]` and `negotiationEpoch: number` to the
Zustand state. Successful initialize copies rows and increments the epoch;
retry clears rows before reconnect.

- [ ] **Step 6: Implement browser/SDK propagation**

Extend `FaceRPCCapabilities` with optional readonly `ui_extensions`. Update
`createLazyRPC` and `createWebFaceHost` to copy the projection. Never expose
config or credentials. Keep absence backward compatible as an empty array.

- [ ] **Step 7: Run and commit**

```bash
go test ./internal/config ./internal/app ./internal/rpc -count=1
pnpm --dir ui typecheck
pnpm --dir ui test -- rpc.test.ts store.test.ts
pnpm --dir sdk/ui test
git add internal/config config.example.yaml internal/rpc internal/app \
  ui/src/lib/rpc.ts ui/src/lib/rpc.test.ts ui/src/lib/store.ts \
  ui/src/lib/store.test.ts sdk/ui/src/module.ts sdk/ui/src/module.test.ts
git commit -m "feat(channel): negotiate UI extension visibility"
```

### Task 2: Select `vivy/channel-ui` through the compiled Assembly plan

**Files:**
- Modify: `internal/modules/defaults/catalog.go`
- Modify: `internal/modules/defaults/catalog_test.go`
- Modify: `internal/modules/defaults/constructors.go`
- Modify: `sdk/internal/assembly/source.go`
- Modify: `sdk/internal/assembly/source_test.go`
- Modify: `sdk/internal/assembly/compiler.go`
- Modify: `sdk/internal/assembly/compiler_test.go`
- Modify: `sdk/internal/frontend_v1.go`
- Modify: `sdk/internal/frontend_v1_test.go`
- Modify: `sdk/internal/cmd/generate-default/main.go`
- Create: `internal/modules/channelui/i18n/catalog.json`
- Create: `internal/modules/channelui/ui/package.json`
- Create: `internal/modules/channelui/ui/pnpm-lock.yaml`
- Create: `internal/modules/channelui/ui/src/index.tsx` initially with a marker-only extension
- Regenerate: `internal/generated/assembly/zz_default.go`
- Regenerate: `ui/src/generated/assembly.ts`

**Interfaces:**
- Produces: one T1 `vivy/channel-ui` descriptor/provider and deterministic
  build-owned UI binding; a generator that writes both checked-in default
  assemblies

- [ ] **Step 1: Write failing catalog/compiler tests**

Require the descriptor:

```text
module: vivy/channel-ui@1.0.0
provides: std/ui-extension@v1 id vivy/channel-ui
requires: core/presentation-host@v1 provider vivy/presentation-host
requires: core/channel-host@v1 provider vivy/channel-host
requestedGrants: rpc.client
i18n: i18n/catalog.json, default en, locales en+zh
lifecycle: generation
```

Require its `UIBindings` entry to match the frozen values above. Add clone
tests so callers cannot mutate binding slices. Compile must mark the selected
UI Provider used without a hand-authored `recipe.ui.extensions` entry.

- [ ] **Step 2: Confirm catalog/compiler RED**

```bash
go test ./internal/modules/defaults ./sdk/internal/assembly \
  -run 'Test.*ChannelUI|Test.*UIBinding' -count=1
```

Expected: Module/binding and implicit selected-UI marking do not exist.

- [ ] **Step 3: Add build-owned binding and selected-input derivation**

Carry `UIBindings` through SourceRecord -> ResolvedModule -> AssemblyPlan.
Update the compiler's UI-provider usage marking to include those bindings.
Give `vivy/channel-ui` its own verified source root
`file:internal/modules/channelui`; update the defaults-to-SourceCatalog adapter
to resolve each T1 root from its descriptor `Source.Ref` instead of hard-coding
`internal`. All existing default records still resolve to `file:internal`.
Implement `deriveSelectedUIInput` in `frontend_v1.go`:

1. clone any explicit external UI input;
2. append UI bindings from selected plan modules;
3. reject duplicate Provider IDs or conflicting explicit/build-owned entries;
4. derive extension order from `recipe.order[std/ui-extension@v1]`;
5. pass the combined input to existing content-hash, dependency-pin, staging,
   generation, Manifest, and Vite paths.

Do not walk Module directories or parse arbitrary exports.

- [ ] **Step 4: Add the minimal Module shell**

Add a no-op T1 lifecycle constructor only because selected Modules participate
in the generated lifecycle graph; all UI behavior remains the exported
TypeScript `extension`. The initial extension registers a stable
`data-vivy-channel-ui` marker and an immediate cleanup so generator tests can
prove install/dispose. The catalog contains at least one EN/ZH title unit and
passes current i18n compilation.

Add a Module-local `ui/package.json` and `ui/pnpm-lock.yaml`. Pin direct Module
dependencies to the versions already sealed in the repository toolchain:
`@vivy/ui-sdk@1.0.0`, `react@19.2.8`, `lucide-react@0.575.0`, and
`react-markdown@10.1.0`. The Pack dependency verifier must derive and verify
these pins; it must not install packages or use floating ranges.

- [ ] **Step 5: Extend the official default generator**

Add `--ui-output` to `generate-default`. It must compile the default Recipe,
derive selected UI input through the same helper as Pack, generate UI source,
and atomically write both outputs. If either generation fails, write neither.
Tests run the command twice and compare both outputs byte-for-byte.

- [ ] **Step 6: Update Recipe selection without crossing later tasks**

Add `vivy/channel-ui` and explicit
`order.std/ui-extension@v1: [vivy/channel-ui]` to
`recipes/default.vivy.yml` and `recipes/web-telegram-only.vivy.yml`, plus the
`rpc.client` grant approval for that selected Module. Keep
`web-no-channels` unchanged. Task 5 creates the backend-without-UI Recipe.

- [ ] **Step 7: Regenerate and run focused tests**

```bash
go run ./sdk/internal/cmd/generate-default --repo . \
  --output internal/generated/assembly/zz_default.go \
  --ui-output ui/src/generated/assembly.ts
gofmt -w internal/generated/assembly/zz_default.go
go test ./internal/modules/defaults ./sdk/internal/assembly ./sdk/internal -run 'Test.*(ChannelUI|UIBinding|UIAssembly|Default)' -count=1
pnpm --dir ui typecheck
pnpm --dir ui test -- presentation-host
```

Expected: default UI Assembly has exactly `vivy/channel-ui`; no-channel has
none; source/catalog hashes are compiler-derived.

- [ ] **Step 8: Commit**

```bash
git add internal/modules/defaults internal/modules/channelui \
  sdk/internal/assembly sdk/internal/frontend_v1.go sdk/internal/frontend_v1_test.go \
  sdk/internal/cmd/generate-default recipes/default.vivy.yml \
  recipes/web-telegram-only.vivy.yml internal/generated/assembly/zz_default.go \
  ui/src/generated/assembly.ts
git commit -m "feat(channel): select dedicated UI module"
```

### Task 3: Add typed settings and Cron adapters over PresentationHost

**Files:**
- Modify: `ui/src/plugins/presentation-host.tsx`
- Modify: `ui/src/plugins/presentation-host.test.tsx`
- Create: `ui/src/plugins/component-contributions.tsx`
- Create: `ui/src/plugins/component-contributions.test.tsx`
- Modify: `ui/src/components/settings/SettingsView.tsx`
- Create: `ui/src/components/settings/SettingsView.test.tsx`
- Modify: `ui/src/routes/_layout.settings.tsx`
- Modify: `ui/src/components/cron/CronTaskManagementView.tsx`
- Create or modify: `ui/src/components/cron/CronTaskManagementView.test.tsx`

**Interfaces:**
- Consumes: existing live `components` registry
- Produces: typed, sorted, validated settings/Cron views with automatic cleanup

- [ ] **Step 1: Write PresentationHost registry-read RED tests**

Require a React context owned by PresentationHost to expose active component
entries to host-tree consumers. Tests cover registration, stable order,
reserved-prefix type rejection, duplicate reserved ID rejection, owner-scoped
cleanup, reverse disposal, and rerender after registration removal. Generic
component IDs retain current stacking/replacement behavior.

- [ ] **Step 2: Implement the read adapter, not a new registry**

Wrap hosted content in a context carrying the existing
`LiveCompositionRuntime`. Export a narrow
`useUIComponentContributions(prefix)` hook returning active entries. Build
`useSettingsSections` and `useCronDeliveryTargets` on top with runtime guards,
ID/prefix checks, numeric order, stable ID tie-break, and no side effects.

- [ ] **Step 3: Write Settings deep-link RED tests**

Remove hard-coded `channels` from `SETTINGS_TAB_VALUES`. Require:

- a registered Channel section produces one tab and its content;
- no contribution produces no tab/content;
- `?tab=channels` falls back to `general` when absent;
- the same deep link activates Channel when registered;
- removing the contribution while active switches to `general`; and
- duplicate section IDs fail extension install rather than rendering twice.

Route search validation accepts only a bounded safe string; availability is
resolved in `SettingsView` from core tabs plus live contributions.

- [ ] **Step 4: Render dynamic Settings sections**

Keep core tabs unchanged. Render contributed triggers/content from the typed
hook. Labels are already localized strings supplied by the selected Module;
the core does not know Channel translation keys.

- [ ] **Step 5: Write and implement generic Cron target tests**

Refactor Channel-specific editor/summary behavior behind
`CronDeliveryTargetContribution`. Core Cron payload keeps existing
`deliver/channel/to` wire compatibility but obtains editors, labels,
validation, and summaries only from active contributions. Tests require:

- no contribution: no delivery toggle/editor and no Channel request;
- Channel contribution: existing create/edit/summary behavior;
- saved historical Channel delivery with no contribution: generic
  `channel · target` read-only summary, never an icon lookup;
- contribution cleanup removes the editor without corrupting the job form.

- [ ] **Step 6: Run and commit**

```bash
pnpm --dir ui typecheck
pnpm --dir ui test -- presentation-host component-contributions SettingsView CronTaskManagementView
git add ui/src/plugins ui/src/components/settings/SettingsView.tsx \
  ui/src/components/settings/SettingsView.test.tsx ui/src/routes/_layout.settings.tsx \
  ui/src/components/cron
git commit -m "feat(ui): add typed extension component adapters"
```

### Task 4: Move all dedicated Channel UI source and catalog into the Module

**Files:**
- Move into `internal/modules/channelui/ui/src/`:
  - `ChannelCard.tsx`
  - `ChannelCardView.tsx`
  - `ChannelEditorForm.tsx`
  - `ChannelTutorialModal.tsx`
  - `ChannelWizardModal.tsx`
  - `ChannelsSettings.tsx`
  - `channel-icons.tsx`
  - `channel-platforms.ts`
  - `channel-schema.ts` and tests
  - `channel-store.ts` and tests
- Create: `internal/modules/channelui/ui/src/api.ts`
- Modify: `internal/modules/channelui/ui/src/index.tsx`
- Expand: `internal/modules/channelui/i18n/catalog.json`
- Modify: `ui/src/lib/api.ts`
- Modify: `ui/src/lib/api.test.ts`
- Modify: `sdk/ui/src/module.ts`
- Modify: `ui/src/i18n/en.ts`
- Modify: `ui/src/i18n/zh.ts`
- Modify: `ui/src/i18n/index.test.ts`
- Modify: `ui/tsconfig.json`
- Modify: `ui/vitest.config.ts`
- Modify: `ui/src/components/settings/diva-preview-data.ts`
- Modify: `ui/src/components/settings/diva-preview-data.test.ts`
- Delete the moved originals from `ui/src/components/settings/`

**Interfaces:**
- Consumes: `FullUIHost.rpc`, `FullUIHost.t`, typed component adapters
- Produces: one self-cleaning `extension`; lazy Channel store scoped to one
  installation; Module-owned `plugin.vivy/channel-ui.*` translations

- [ ] **Step 1: Write API/store RED tests at the destination**

`api.ts` uses only `host.rpc.call` for exact methods and local DTOs. The store
is created per `install(host)`, not a process-global singleton. Tests prove:

- import and install make zero RPC calls;
- first rendered/subscribed Channel surface calls `channel/inspect` once, then
  `channel/get` for compiled names;
- save sends the existing whole-envelope `channel/update` semantics;
- cleanup drops subscribers/in-flight result application and subsequent
  remount creates fresh state;
- credentials remain environment references; no secret value enters catalog
  or projection.

- [ ] **Step 2: Move components and inject Module dependencies**

Move, do not duplicate, the dedicated files. Replace global `t` and global API
imports with a Module context created by `index.tsx` containing the scoped
store, `host.rpc`, and `host.t`. Keep UI primitives imported from the host
`@/components/ui/*` paths; do not copy core primitives or create another React
root.

Extend the UI typecheck and Vitest include globs to cover
`../internal/modules/channelui/ui/src/**/*`. This is test/compiler coverage,
not runtime discovery; production inclusion still comes only from generated
UI Assembly imports.

- [ ] **Step 3: Register both contributions with one cleanup**

`extension.install(host)` registers:

```text
vivy.settings.section/channels
vivy.cron.delivery-target/channel
```

The settings contribution renders `ChannelsSettings` with the stable marker
`data-vivy-channel-settings`. The Cron contribution shares the scoped store,
renders current compiled Provider choices, and uses generic fallback for
unknown historical names. Return one idempotent cleanup that disposes both
registration handles and the store.

- [ ] **Step 4: Move API ownership while preserving the v1 compatibility surface**

The Module's `api.ts` becomes the only active typed client and uses the generic
`FaceClientRPC` contract. Keep the existing Channel DTOs and three methods in
`FaceClientAPI`/`ui/src/lib/api.ts` as deprecated, inert source-compatibility
shims for UI SDK v1; no core component imports or calls them. Add an AST test
whose allowlist permits those method strings only in the compatibility file,
its tests, the selected Module client, and generated Assembly. Do not change
backend JSON-RPC, public `std/channel@v1`, or the UI SDK major version.

- [ ] **Step 5: Move localization/catalog content**

Translate all component-owned `channelDescriptions`, `channelFields`,
`channelGuide`, `channels`, and Settings tab strings into
`plugin.vivy/channel-ui.*` catalog units with complete EN/ZH values and
placeholder parity. Retain core Cron words needed for generic historical
display. Remove Channel-only mock preview rows/keys from Diva preview data so
an omitted build has no false Channel dashboard.

- [ ] **Step 6: Audit generic history and unknown fallbacks**

Keep `MessageBubble.tsx` provenance rendering implementation-agnostic. Add
tests that messages from omitted/unknown Channel names render source/chat
strings without importing platform icons, opening settings, or calling
`channel/*`. Add Module tests for unknown Provider icon/name fallback.

- [ ] **Step 7: Run focused tests and scans**

```bash
pnpm --dir ui typecheck
pnpm --dir ui test
node scripts/check-i18n-completeness.js
node --test scripts/check-i18n-cross-face.test.js
node scripts/check-i18n-cross-face.js
test -z "$(find ui/src/components/settings -maxdepth 1 -iname '*channel*' -print)"
rg -n 'CHANNEL_PLATFORMS|ChannelsSettings|channel-store' \
  ui/src --glob '!generated/assembly.ts'
go test ./sdk/internal -run TestChannelUICompatibilityMethodAllowlist -count=1
```

Expected: `rg` has no core dedicated component/store import. Only the tested
v1 API compatibility shims may retain Channel RPC method strings; allowed
generic historical provenance fields remain in shared types.

- [ ] **Step 8: Commit**

```bash
git add internal/modules/channelui ui/src sdk/ui/src/module.ts
git commit -m "refactor(channel): move dedicated UI into module"
```

### Task 5: Gate installation, reconnect cleanup, Recipes, and asset omission

**Files:**
- Modify: `ui/src/routes/__root.tsx`
- Create: `ui/src/routes/__root.test.tsx`
- Modify: `ui/src/plugins/presentation-host.test.tsx`
- Create: `recipes/web-channels-no-ui.vivy.yml`
- Modify: `recipes/default.vivy.yml`
- Modify: `recipes/web-telegram-only.vivy.yml`
- Verify unchanged: `recipes/web-no-channels.vivy.yml`
- Modify: `sdk/internal/removal_conformance_test.go`
- Modify: `sdk/internal/frontend_v1_test.go`
- Create: `ui/e2e/channel-modularization.spec.ts`

**Interfaces:**
- Produces: `selectEnabledUIExtensions(generated, projection, capabilities)`
  and four truthful Recipe variants

- [ ] **Step 1: Write root selection RED tests**

For `vivy/channel-ui`, require all of:

```text
present in generatedUIExtensions
backend row exists with enabled=true
capabilities contain channel.inspect, channel.get, channel.update
```

Missing any input yields no installation. Unknown backend rows never import or
install code. Projection false yields no install. Backward-compatible missing
`ui_extensions` yields no Channel install.

- [ ] **Step 2: Implement pre-install filtering**

After store initialization, call the pure selector and pass only the accepted
extensions to PresentationHost. Key PresentationHost by
`negotiationEpoch + species.generation_id`. Retry/reconnect first unmounts the
old keyed host, invoking reverse cleanup, then negotiates and installs a fresh
set. Add tests that an old Channel contribution/store cannot survive a
Generation change.

- [ ] **Step 3: Create the backend-without-UI Recipe**

`web-channels-no-ui.vivy.yml` copies default backend module/grant selection but
omits `vivy/channel-ui` and its UI order entry. Confirm matrix:

| Recipe | Channel backend | Channel UI |
|---|---:|---:|
| default | all five | yes |
| web-no-channels | none | no |
| web-channels-no-ui | all five | no |
| web-telegram-only | Telegram | yes |

- [ ] **Step 4: Add artifact/asset conformance**

Pack all four Recipes in a table. Assert from Inspect and `ui/dist`:

- UI-bearing variants list exactly `vivy/channel-ui`, its catalog, source/lock
  hashes, and the `data-vivy-channel-settings` marker;
- omitted variants list none and contain none of the marker,
  `plugin.vivy/channel-ui.` keys, platform tutorial paths, or Module entry;
- backend-without-UI still has Host/providers/channel capability state;
- no-channel retains P0-3 dependency-closure proof;
- two builds per Recipe have identical Generation ID and UI artifact digest.

Use exact Manifest membership and selected source staging as primary evidence;
dist-string scans are secondary.

- [ ] **Step 5: Add browser/network smoke**

`channel-modularization.spec.ts` runs against parameterized packed servers and
records requests/WebSocket JSON-RPC frames:

- default: Channel tab exists; opening it causes inspect/get; wizard remains
  functional with fake/inert config;
- config-disabled default: no tab, stale `?tab=channels` falls back, no
  `channel/*` frame;
- backend-without-UI: same no-tab/no-frame result, ordinary Settings and Cron
  work;
- no-channel: same, plus ordinary session UI works;
- reconnect from enabled to omitted projection disposes tab/Cron target/store
  and performs no later Channel request;
- historical Channel-origin message still renders generically.

No test contacts a platform network.

- [ ] **Step 6: Regenerate, run, and commit**

```bash
go run ./sdk/internal/cmd/generate-default --repo . \
  --output internal/generated/assembly/zz_default.go \
  --ui-output ui/src/generated/assembly.ts
gofmt -w internal/generated/assembly/zz_default.go
go test ./sdk/internal ./sdk/internal/assembly -run 'Test.*Channel.*(UI|Asset|Recipe|Removal)' -count=1
pnpm --dir ui typecheck
pnpm --dir ui test
pnpm --dir ui build
pnpm --dir ui exec playwright test e2e/channel-modularization.spec.ts
git add recipes ui/src/routes sdk/internal internal/generated/assembly/zz_default.go \
  ui/src/generated/assembly.ts ui/e2e/channel-modularization.spec.ts
git commit -m "test(channel): prove UI absence and cleanup"
```

### Task 6: Run the P0-4 gate and publish acceptance evidence

**Files:**
- Modify only if producer requests it:
  `sdk/internal/assembly/conformance_results.json`
- Create: `docs/logs/YYYY-MM-DD-channel-p0-4/summary.md`
- Create: `docs/logs/YYYY-MM-DD-channel-p0-4/verification.md`
- Create: `docs/logs/YYYY-MM-DD-channel-p0-4/acceptance.md`
- Modify: `docs/superpowers/plans/channel-modularization/index.md`

- [ ] **Step 1: Run full focused backend/UI gates**

```bash
go test ./internal/config ./internal/rpc ./internal/app ./internal/modules/... \
  ./sdk/internal/assembly ./sdk/internal -count=1
pnpm --dir ui typecheck
pnpm --dir ui test
pnpm --dir ui build
node scripts/check-i18n-completeness.js
node --test scripts/check-i18n-cross-face.test.js
node scripts/check-i18n-cross-face.js
```

- [ ] **Step 2: Run platform conformance and repository CI**

Run the `vivy-plugin` pressure matrix, then `just ci`. Run the Windows
Playwright lane because packaged executable/UI startup is a release-critical
path. Record exact CI URL/run ID and do not convert an unavailable browser
venue into PASS.

- [ ] **Step 3: Run scope and ownership scans**

```bash
rg -n 'ChannelsSettings|channel-store|channel-icons|channel-platforms|channel-schema' ui/src
go test ./sdk/internal -run TestChannelUICompatibilityMethodAllowlist -count=1
story_base="$(git merge-base HEAD origin/feat/channel-modularization)"
test "$story_base" != "$(git rev-parse HEAD)"
git diff "$story_base" -- internal/channelhost plugins/dingtalk \
  plugins/discord plugins/feishu plugins/qq plugins/telegram
git diff --check
```

Expected: the source scan is empty outside generic tests/generated import and
the compatibility allowlist passes;
backend/platform implementation diff empty; formatting clean.

- [ ] **Step 4: Write evidence and request independent review**

Record the four Recipe Manifest/UI matrices, Vite asset lists/digests,
disabled/omitted network frame captures, reconnect cleanup count, deep-link and
history results, translation completeness, and full command output. Acceptance
must say P0-5 release/rollback closure is still outstanding.

- [ ] **Step 5: Mark complete only after acceptance**

After review, change P0-4 to `COMPLETE` and P0-5 to `READY` in the index.

```bash
git add sdk/internal/assembly/conformance_results.json \
  docs/logs/YYYY-MM-DD-channel-p0-4 \
  docs/superpowers/plans/channel-modularization/index.md
git commit -m "docs(channel): accept UI modularization slice"
```

## Story acceptance checklist

- [ ] `vivy/channel-ui` is a selected `std/ui-extension@v1` Module derived
  from the same Assembly plan, not a second feature list.
- [ ] Startup config controls installation only and omitted config defaults
  enabled when compiled.
- [ ] Backend projection is secret-free and process/capability truthful.
- [ ] Disabled/omitted UI performs no mount, Channel request, subscription,
  wizard, settings tab, or Cron target contribution.
- [ ] Omitted artifacts contain no Channel UI Module/catalog/dedicated assets.
- [ ] Reconnect disposes the prior generation's registrations/store before
  renegotiation.
- [ ] Stale links, historical messages, and unknown Channels degrade safely.
- [ ] Default and Telegram-only retain Channel UI; backend-without-UI and
  no-channel do not.
- [ ] P0-3 backend omission remains green and no P0-5 completion is claimed.
