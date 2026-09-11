# PLG-P6 Full UI Modules Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> `superpowers:subagent-driven-development` (recommended) or
> `superpowers:executing-plans` to implement this plan task-by-task. Steps use
> checkbox (`- [ ]`) syntax for tracking. REQUIRED DOMAIN SKILL: use
> `oil-frontend` for frontend implementation and verification.

**Goal:** Allow selected T2 Modules to modify or replace the complete Web Face
UI without a UI permission system while preserving server-side authority and
deterministic Generation builds.

**Architecture:** A public TypeScript UI SDK defines full-code extension and
exclusive-root entry points. Assembly compilation builds selected UI source
into a generated frontend composition; an internal PresentationHost invokes it
in Recipe order. Module-specific backend operations use one typed ActionHost
RPC method rather than arbitrary routes.

**Tech Stack:** React 19, TypeScript, Vite 7, TanStack Router, pnpm, Go Control
RPC, Playwright/Vitest, `just ci`.

**Spec:** `docs/architecture/VIVY-PLUGIN-SPEC.md` section 7 and
`docs/architecture/VIVY-PORT-CATALOG.md` sections 11–12.

## Global Constraints

- State: `IMPLEMENTATION COMPLETE · 2026-09-11`; depends on P2 and is not on
  the core SCX critical path. PLG-P9 release conformance remains separate and
  `UNSCHEDULED`.
- There is no `ui.full` Grant, approval dialog, component allow-list, DOM audit,
  CSS isolation, or permission registry.
- External UI source is explicit, pinned, compiled, and hashed in the
  Generation; runtime remote-code loading remains forbidden.
- All browser input is untrusted by the backend.

### Accepted cross-cutting I18N contract

This phase MUST implement the accepted plugin I18N contract from the four
normative architecture documents:

- Use one translation-unit schema for Web and TUI, while letting each plugin
  own an explicitly declared catalog.
- Reserve `vivy.*` for core messages and
  `plugin.<module-id>.*` for plugin messages. Validate ownership, duplicate
  keys, locale data, and placeholder parity during package/Recipe handling.
- Expose host localization as a key-and-arguments API to full-code UI Modules;
  descriptor-based UI should send keys and arguments instead of rendered
  English or Chinese strings.
- Resolve base messages as active locale, English, then a bounded visible
  diagnostic. Resolve `short`/`long` as active form, active base, English form,
  English base, then the diagnostic; missing arguments remain visible.
- Consume P1's sealed catalog projection and add shared Web/TUI conformance.
  Do not discover, download, or persist plugin locale state at runtime.

---

### Task 1: Define full-code UI Port contracts

**Files:**

- Create: `sdk/ui/package.json`
- Create: `sdk/ui/tsconfig.json`
- Create: `sdk/ui/src/index.ts`
- Create: `sdk/ui/src/module.ts`
- Create: `sdk/ui/src/module.test.ts`
- Modify: `ui/package.json`
- Modify: `ui/vite.config.ts`

**Interfaces:**

- Consumes: `std/ui-extension@v1` and `std/ui-root@v1` definitions.
- Produces: `UIExtension`, `UIRoot`, `FullUIHost`, and explicit cleanup handles.

```ts
export interface UIExtension {
  id: string
  install(host: FullUIHost): void | (() => void)
}

export interface UIRoot {
  id: string
  render(host: FullUIHost): React.ReactNode
}
```

- [x] Write `rejects duplicate root providers` as a RED test against the UI
  composition input.
- [x] Write type tests for missing IDs, duplicate IDs, invalid cleanup, and
  unresolved `before`/`after`/`replaces` references.
- [x] Expose full UI composition and current Face client APIs; do not add a
  permission request method.
- [x] Pin the SDK package version in UI build provenance.
- [x] Run `cd ui; pnpm test` and `cd ui; pnpm typecheck`.
- [x] Commit `feat(ui-sdk): define full ui module contracts`.

### Task 2: Generate deterministic UI composition

**Files:**

- Create: `sdk/internal/assembly/ui.go`
- Create: `sdk/internal/assembly/ui_test.go`
- Generate: `ui/src/generated/assembly.ts`
- Modify: `ui/vite.config.ts`

**Interfaces:**

- Consumes: selected UI Module sources, lockfiles, root selection, extension
  order, and replacement graph.
- Produces: deterministic TypeScript imports plus UI source/lock/output hashes
  in the Generation Manifest.

- [x] Write a golden RED test proving different filesystem enumeration order
  cannot change generated imports or hashes.
- [x] Write failures for two roots, ambiguous extension order, missing target,
  floating package dependency, and remote entry URL.
- [x] Generate only explicit Recipe Modules; never glob plugin directories.
- [x] Include source, dependency lock, SDK, and final asset hashes in Manifest.
- [x] Verify omitted UI Modules leave no import or asset in a minimal build.
- [x] Run `go test ./sdk/internal/assembly -run UI` and `cd ui; pnpm build`.
- [x] Commit `feat(assembly): generate full ui composition`.

### Task 3: Install UI Modules in PresentationHost

**Files:**

- Create: `ui/src/plugins/presentation-host.tsx`
- Create: `ui/src/plugins/presentation-host.test.tsx`
- Modify: `ui/src/routes/__root.tsx`
- Modify: `ui/src/main.tsx`
- Modify: `ui/src/styles.css`

**Interfaces:**

- Consumes: generated root and ordered extension constructors.
- Produces: one Web Face UI tree and reverse-order cleanup.

- [x] Write a RED test proving an extension can replace navigation, register a
  route, change global styles, and observe current client state without asking
  for UI permission.
- [x] Write lifecycle tests for install failure, root render failure, cleanup,
  and extension ordering.
- [x] Install the selected root once and extensions in exact Recipe order.
- [x] Display Module provenance for diagnostics without restricting behavior.
- [x] Run `cd ui; pnpm test` and `cd ui; pnpm typecheck`.
- [x] Commit `feat(ui): host unrestricted generation ui modules`.

### Task 4: Define typed Control Actions

**Files:**

- Create: `sdk/port/controlaction/action.go`
- Create: `sdk/port/controlaction/action_test.go`
- Create: `internal/actionhost/host.go`
- Create: `internal/actionhost/host_test.go`
- Modify: `internal/app/app.go`

**Interfaces:**

- Consumes: namespaced Action Providers and effective Grants.
- Produces: `ActionHost.Invoke(ctx, caller, moduleID, actionID, input)`.

```go
type Provider interface {
    Definition() Definition
    Invoke(context.Context, Host, json.RawMessage) (json.RawMessage, error)
}
```

- [x] Write `TestActionHostDistrustsBrowserAuthorityClaims`; expected RED is
  any caller-supplied approval/Trust result accepted by the Host.
- [x] Write tests for invalid input/output schema, wrong owner, unavailable
  instance, denied Grant, Secret leak, timeout, and effect-specific audit.
- [x] Implement one Host with no arbitrary route registration.
- [x] Ensure starting a Run and executing a model Tool re-enter their existing
  authoritative paths.
- [x] Run `go test ./sdk/port/controlaction ./internal/actionhost`.
- [x] Commit `feat(action): host typed module control actions`.

### Task 5: Expose one authenticated Action RPC

**Files:**

- Modify: `internal/rpc/protocol.go`
- Modify: `internal/rpc/control.go`
- Create: `internal/rpc/module_action_test.go`
- Create: `sdk/ui/src/action-client.ts`
- Create: `sdk/ui/src/action-client.test.ts`

**Interfaces:**

- Consumes: authenticated Face caller and ActionHost.
- Produces: `module.action.invoke` with schema-bounded request/result.

- [x] Write RPC RED tests for spoofed Module ID, forged approval, oversized
  payload, unknown Action, cancellation, and redacted error.
- [x] Implement the single method and reject plugin-defined paths or methods.
- [x] Add a typed UI SDK client with no local authority decisions.
- [x] Run `go test ./internal/rpc -run ModuleAction` and UI SDK tests.
- [x] Commit `feat(rpc): expose governed module actions`.

### Task 6: Build a real full-access fixture

**Files:**

- Create: `sdk/internal/testdata/full-ui-module/vivy-module.yaml`
- Create: `sdk/internal/testdata/full-ui-module/ui/package.json`
- Create: `sdk/internal/testdata/full-ui-module/ui/pnpm-lock.yaml`
- Create: `sdk/internal/testdata/full-ui-module/ui/src/index.tsx`
- Create: `ui/e2e/plugin-full-ui.spec.ts`

**Interfaces:**

- Consumes: UI SDK, generated composition, and a fake typed Control Action.
- Produces: realistic proof of root/route/style/state modification with no UI
  permission prompt.

- [x] Write the Playwright test first and observe the default UI cannot display
  the fixture route or replacement.
- [x] Build the fixture as a selected T2 Module.
- [x] Assert it changes global UI and calls its Action through the one RPC.
- [x] Assert a forged approval still fails server-side.
- [x] Assert no permission dialog or UI Grant appears in Inspect.
- [x] Run the split development pair and Playwright test at
  `http://127.0.0.1:3015`.
- [x] Commit `test(ui): prove unrestricted ui module composition`.

### Task 7: Complete UI conformance and removal proof

**Files:**

- Create: `ui/src/plugins/conformance.test.tsx`
- Modify: `sdk/internal/assembly/ui_test.go`
- Modify: `sdk/internal/assembly/manifest_test.go`

**Interfaces:**

- Consumes: default, extension, replacement-root, and minimal UI builds.
- Produces: seven-artifact proof for both UI Ports and Control Action.

- [x] Cover duplicate/missing Provider, order conflict, build failure, runtime
  install failure, cleanup, hash provenance, Action failure, and omission.
- [x] Build/inspect all four fixture Generations.
- [x] Run UI unit, typecheck, build, Playwright smoke, and `just ci`.
- [x] Commit `test(ui): prove full ui module conformance`.

The focused unit/typecheck and hermetic Go fixture checks are complete. The
split-pair Playwright smoke and repository `just ci` also passed on Windows
during PR #20 integration on 2026-09-11. PLG-P9 remains a separate,
unscheduled release-conformance phase.

Final review corrections are complete: Pack embeds the selected Vite output,
source/lock/dependency/output provenance is compiler-bound, WebSocket action
bridges use durable server-bound SessionID/RunID state, and audit invocation
IDs are monotonic and collision-safe. Focused ActionHost/RPC/Assembly race
coverage passes; the unrelated broader app race remains an existing
Claude/Eino dependency issue.

## Phase exit and rollback

Exit requires complete UI modification without a UI authorization system,
deterministic build provenance, one UI root, ordered cleanup, one Action RPC,
and server-side authority tests. Rollback selects the previous sealed
Generation and does not hot-unload frontend code.
