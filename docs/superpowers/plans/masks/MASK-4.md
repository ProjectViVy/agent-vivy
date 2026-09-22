# MASK-4 optional UI and release composition Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans for native execution, or superpowers:subagent-driven-development only when that method is selected. Steps use checkbox syntax for tracking.

**Goal:** Backend-authoritative mask UI is removable, with code mode independent and no legacy local selection authority.
**Architecture:** A typed chat.header value reuses the components registry; the removable `vivy/masks-ui` Module owns page/editor/selector/localization; shell owns independent model/code controls.
**Tech Stack:** React/TypeScript, existing UI SDK/PresentationHost/Vitest/Playwright, generated Recipe assets.
**Spec:** [design](../../specs/2026-09-21-mask-subsystem-design.md), baseline `5253f77`/`a0f892c`, [MASK-C1](contracts.md).
**Epic / requirements:** MASK-43 / M4, M5, M9. State/dependencies: [index](index.md).

The implementation slice is committed on `feat/issue43-mask-system`: `59297a4`
adds the typed header slot, `d33637e` adds independent code mode, and `5ca8144`
adds the removable backend-driven `vivy/masks-ui` source under
`plugins/vivy-masks-ui`; `3857a14` and `b2d4440` add its chat-header selector
and test. The UI Module is registered in the repository Source
Catalog but is intentionally not selected by the current default Recipe while
the permanent shell mask rule and generated selected/omitted artifacts await
their dedicated release gate.

## Global Constraints

- Read ui/AGENTS.md; never hand-edit ui/src/generated/** or import shell store in Module.
- EN/ZH plugin.vivy/masks.* catalog; backend IDs/bodies determine actual capability.
- No vivy.ui.activeMask reads/writes; no programmer-to-Face mapping.
- Real UI smoke runs at :3015 with split backend; embedded E2E is a separate gate.
- No rule edit until explicit authorization for the focused permanent-mask-entry correction.

## Review Focus

- Late response for session A must not overwrite B's selection (Task 2).
- Running label must distinguish captured current mask from next admission choice (Task 2).
- Removing masks must not remove model/code controls or preserve a static route (Task 3).
- Stale editor draft must survive conflict without optimistic overwrite (Task 2).
- Browser build with backend-only masks must not bundle mask UI (Task 3).

## Task 1: Typed header consumer and independent code control

**Files:** Modify `sdk/ui/src/module.ts`, `module.test.ts`,
`ui/src/plugins/presentation-host.tsx`, `presentation-host.test.tsx`,
`ui/src/routes/_layout.tsx`, `ui/src/components/chat/ChatView.tsx`,
`MaskAndModelSwitcher.tsx`, `ui/src/lib/api.ts`, `store.ts`,
`internal/rpc/control.go`; NEW `ui/src/components/chat/CodeModeControl.tsx`,
`CodeModeControl.test.tsx`, `ui/src/plugins/chat-header.tsx`.

**Consumes:** MASK-C1 ChatHeaderContribution and existing RunOptions.Face/store paths.
**Produces:** typed slot consumer, backend code_mode_available metadata, independent
CodeModeControl. Keep code mode as session-view/run input state, never infer from mask.

- [ ] Add slot tests with empty registry, unknown value, two ordered contributions,
  thrown render, cleanup and changing session context. Reuse current host registry
  error boundary and registration ownership, not a new global event bus.
- [ ] Add send/queue/edit/regenerate tests: choose Code then switch mask => FaceCode
  remains; choose No mask => code still works; backend unavailable => no false toggle.
- [ ] Run `(cd ui && pnpm test src/plugins/presentation-host.test.tsx src/components/chat/CodeModeControl.test.tsx)`
  and capture red; UI scripts stage generated assets automatically.
- [ ] Implement the typed slot via existing components registry and spec shape.
  Split existing model selector without changing its provider/settings behavior.
  Server projects code capability from existing allowed Face logic; do not equate
  exclusive face provider selection with permission to accept FaceCode without tracing it.
- [ ] Update typed host API projections and tests together. Existing UI API consumers
  see an additive optional boolean; absence disables the new code control until
  real backend support, rather than assuming support from a hard-coded list.
- [ ] Rerun slot/code path tests, SDK UI tests with `(cd sdk/ui && pnpm test)`,
  `pnpm typecheck` in ui, then commit `feat: add typed chat header and independent code mode`.

## Task 2: Mask extension, catalog and editor

**Files:** NEW `plugins/vivy-masks-ui/go.mod`, `vivy-module.yaml`, `module.go`,
`README.md`, `ui/vivy-masks/package.json`, `pnpm-lock.yaml`, `src/index.tsx`,
`src/MaskPage.tsx`, `src/MaskSelector.tsx`, `src/mask-client.ts`,
`src/mask-client.test.ts`, `src/MaskPage.test.tsx`, `src/MaskSelector.test.tsx`;
NEW `plugins/vivy-masks-ui/i18n/catalog.json`. The separate source root avoids
staging the complete `internal/` tree for a UI-only contribution.
Reuse current selected Module UI packaging pattern/dependency versions, don't add a
separate application framework. Modify SDK internal source/UI binding tests if needed.

**Consumes:** seven action DTOs/error taxonomy, host session store and typed header.
**Produces:** page/sidebar/header contribution; draft-safe backend state transitions.

- [ ] Write reducer/client interaction tests with deferred promises:

```text
load selection A; switch host active session to B; resolve B then A
assert UI still shows B and never sends A's revision to B
edit custom C; another client updates C; save old revision
assert conflict banner, draft preserved, committed state reread
```

- [ ] Write UI tests for no active session, loading, switch pending, active run next-run
  label, built-in duplicate, in-use delete count, denied policy and reconnect refresh.
  Action mocks are for tests only, never imported by production Module.
- [ ] Implement action client through SDK/host RPC, with exact schemas and committed
  response state. Maintain session request epoch/cancellation; errors never activate
  local selections. Refresh on open/focus/reconnect/mutation, no polling loop.
- [ ] Implement list/get lazy body loading, draft editor, create operation UUID lifetime,
  explicit update/delete expected revision, safe ambiguous-response reread. Show count
  for delete-in-use; do not automatically unmask other sessions.
- [ ] Register navigation/page/header with cleanup handles and EN/ZH owned keys.
  Module imports usePluginHost/usePluginTranslation and permitted UI kit only.
  Header with no session is disabled; catalog CRUD remains available to authorized
  local operator. Current run identity, if displayed, comes from admitted metadata.
- [ ] Run Module tests through the selected staged UI test path and existing SDK
  conformance harness. Staged tests under `ui/src/generated/ui/**` match the
  current `ui/vitest.config.ts` include `src/**/*.test.ts(x)`; assert staging
  preserves them and run `pnpm test src/generated/ui` from ui. If the stage tool
  deliberately excludes tests, add an explicit mask source include to that config
  and the same React/SDK aliases, and prove a deliberate failing assertion is
  detected. Do not leave Module tests outside every runner. Commit `feat: add backend-driven mask UI extension`.

## Task 3: Remove old authority, prove optional artifacts and browser flow

**Files:** Remove `ui/src/components/masks/mask-catalog.ts`, `mask-catalog.test.ts`,
old `MaskManagementView.tsx`, `MaskIdentity.tsx` after their presentation is moved;
remove `ui/src/routes/_layout.masks.tsx`; modify `ConversationSidebar.tsx`,
remaining `MaskAndModelSwitcher.tsx` references, core mask locale entries only when
no longer referenced, default Recipe and compiler UI source projection.
NEW `recipes/masks-selected.vivy.yml`, `recipes/masks-omitted.vivy.yml`,
`recipes/masks-backend-only.vivy.yml` as explicit acceptance recipes;
NEW `ui/e2e/masks.spec.ts`, `ui/playwright.masks.config.ts`.
Update only necessary source/conformance evidence through the existing generator.

**Consumes:** complete backend and extension. Produces AC-08/09/10 and seven-artifact closure.

- [ ] Before changing `ui/AGENTS.md`, obtain the focused approval already identified
  in the design: remove masks from the permanently rendered shell-entry clause;
  leave chat/toolbox and all other rules intact. If not authorized, keep that
  instruction edit and dependent UI movement blocked; don't silently override it.
- [ ] Write omission assertions for routes/sidebar/header/locales/source import and
  artifact bytes; tests must fail against the current static shell mask imports.
- [ ] Remove old localStorage authority and Face mapping. Do not migrate global mask
  preference to every session. Remove generated route through normal route generation,
  not manual edits. Preserve model control and existing non-mask locale keys.
- [ ] Add explicit selected/omitted/backend-only recipes using compiler-supported
  module/UI separation. Default includes completed masks. Headless has no forced UI;
  do not rename existing product recipes or alter Channel selections incidentally.
- [ ] Execute all seven Port artifacts; promote support only from actual accepted
  conformance. Use normal source-hash/evidence workflow on the execution branch,
  not ad-hoc hash edits or invented passing result JSON.
- [ ] Run from fresh output paths (remove only disposable task-owned outputs if rerunning):

```bash
go run ./sdk pack --recipe recipes/masks-selected.vivy.yml --output .workspace/mask-verify/selected
go run ./sdk inspect-artifact .workspace/mask-verify/selected
go run ./sdk pack --recipe recipes/masks-omitted.vivy.yml --output .workspace/mask-verify/omitted
go run ./sdk inspect-artifact .workspace/mask-verify/omitted
go run ./sdk pack --recipe recipes/masks-backend-only.vivy.yml --output .workspace/mask-verify/backend-only
```

- [ ] Add a dedicated split Playwright configuration with baseURL `http://127.0.0.1:3015`,
  disposable backend state/config, two webServer entries (backend :8787 + Vite :3015),
  one worker and the mask test only. Reuse `ui/e2e/global-setup.ts` preparation
  mechanics without touching production data. Existing playwright.config.ts targets
  embedded server; do not claim it is this split smoke.
- [ ] Run `(cd ui && pnpm exec playwright test --config playwright.masks.config.ts)`.
  Cover two browser contexts, persisted switch/reload, create/edit conflict, in-use
  delete, queued next-run application, code independently on/off, and no-mask artifact
  smoke. Backend-only artifact gets RPC/run smoke without browser extension.
- [ ] Run complete pressure matrix and `just ci`, then record live-model evaluation
  separately under acceptance AC-10. If unavailable, report missing evidence; don't
  label semantic obedience verified. Commit `feat: complete optional mask generation and acceptance`.

## Acceptance and handoff

Return all AC IDs with commands/results, artifact Inspect outputs, :3015 browser
record and failure cases. Verify a masked suspended run cannot resume in omitted
artifact; normal new runs still work. Confirm no raw prompt in logs/Inspect. Product
release requires integrated evidence, not merely passing component tests. Runtime
schema remains; omitting masks is not a database rollback or deletion operation.
