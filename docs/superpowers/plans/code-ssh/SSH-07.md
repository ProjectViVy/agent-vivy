# SSH-07 control, status and GUI implementation plan

> **For agentic workers:** Use superpowers:executing-plans for native execution. Steps use checkboxes; follow the index's readiness gates before coding.

**Goal:** Make remote target, jobs and deliverables controllable in the GUI using backend-owned state.
**Architecture:** One plugin owner shared by ToolWorld/Actions/Context/Status; removable UI through @vivy/ui-sdk, no second RPC/store.
**Tech Stack:** Go public Ports, React/TypeScript, existing UI kit, Vitest/Playwright and split dev loop.
**Spec:** [SSH-D1](../../specs/2026-10-11-code-ssh-design.md) §4/7; [SSH-C1](contracts.md); baseline `ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666`.
**Epic / requirements:** E2 / R1, R3, R5-R6, R8. State and predecessors: [index](index.md).

## Global constraints and review focus

BLOCKED on H5/H6's exact shared-owner and approved control producer. Current
Action Grant allow-list cannot simply authorize Spawn/Secret. Every job side
effect reenters ToolHost, with Host-generated operation identity. Status reads
cached state only. UI claims are untrusted; no shell store import, raw RPC route,
localStorage target authority or new UI Grant. Read ui/AGENTS.md and use the
required frontend skill before UI implementation.

## Prerequisites and contracts

Consume SSH-06 bound resource/delivery path and shared Manager. Existing
`controlaction.Provider` supplies Definition and Invoke(ctx,Host,input);
`status.Provider` supplies ID and Status(ctx,Request)(Snapshot,error). UI uses
`createModuleActionClient(host.rpc).invoke({moduleId,actionId,input})`,
route/navigation registration and typed ChatHeaderContribution from SDK.
Produce approved control adapters and copied status snapshots. H5's real
factory wiring and H6's accepted control/selection signatures must be inserted
here before Ready; constructors must not independently own connections.

## Task 1: Governed target controls and status

**Files:** NEW proposed plugin `actions.go`, `status.go`, `actions_test.go`,
`status_test.go`; UPDATE tools/descriptor/module/manager within accepted signatures.

- [ ] Declare owner `vivy/ssh-workspace` and action IDs
  `vivy.ssh-workspace.targets.list`, `.target.select`, `.connect`, `.disconnect`,
  `.status`, `.jobs.list`, `.jobs.output`, `.jobs.cancel`, `.reconcile`.
  targets.list returns approved redacted profiles only. target.select takes
  target_id/expected_revision/session_id, not arbitrary host/root/credentials;
  backend binds the selected immutable profile through the accepted H1 path.
- [ ] Add `TestActionsCannotBypassToolHost`, `TestSwitchWithOwnedOperationDenied`,
  `TestStatusDoesNotSpawn`, `TestPortsShareOneOwnerAndCloseOnce`,
  `TestUnauthenticatedControlDenied`, `TestStatusHasNoSecrets`.
  Include admitted/running/awaiting-approval operations and stale revisions.
- [ ] Run plugin focused tests for red. Implement connect/disconnect/reconcile
  through the proved control path. If H6 proves the InvokeTool bridge, add
  governed `ssh_connect`, `ssh_disconnect`, `ssh_reconcile` lifecycle tools with
  schema/effect/approval metadata and server-issued scope; Actions call that
  bridge, never Spawn directly. If H6 requires a different approved generic
  path, use its exact contract and update these tasks before execution.
- [ ] Reuse one explicit lifecycle owner for all projections; no package globals.
  Snapshot carries target/root/origin, connection/job state, capabilities,
  revision and last-known timestamp. Offline data is visibly stale. Switching is
  refused while an operation owns the binding; do not merely disable a button.
- [ ] Run plugin tests/vet, SDK verify and accepted ActionHost/Status/generated
  lifecycle suites. Commit as `feat: govern SSH target controls and status`.

## Task 2: Removable target page, indicator and job controls

**Files:** NEW proposed `ui/ssh-workspace/{package.json,pnpm-lock.yaml,src/index.tsx,src/page.tsx,src/header.tsx,src/api.ts,src/types.ts,src/page.test.tsx,src/header.test.tsx}`
under the plugin; NEW `i18n/catalog.json`; UPDATE plugin descriptor. UI test
projection uses a disposable explicit Recipe until SSH-08 adds the product Recipe.

- [ ] Use supported SDK registrations: route `/ssh-workspace`, module navigation,
  components slot `chat.header`. Add tests for origin/target label, connecting/
  unavailable/stale state, selection conflict, job output/cancel and exact
  deliverable access. Use module action client, typed results and the existing
  authorized delivery UI API; no hand-built download URL.
- [ ] Run UI Vitest selected module tests for red via staged sources. Implement
  compact target/profile selection, explicit connect/disconnect, capability
  diagnostics, jobs/output/cancel/reconcile and resource links. Keep technical
  details in diagnostics; normal operation needs target, state and next action.
  All visible text uses `plugin.vivy/ssh-workspace.*` en/zh catalog keys.
- [ ] Unmount disposes SDK registrations/subscriptions; last-known snapshots do
  not restart connections. Add tests for owner omission and reinstallation
  without duplicate page/header subscriptions. Backend response, never UI state,
  determines binding and action permission.
- [ ] Execute selected UI typecheck/tests/build, split browser smoke and full
  `just ci`. Use disposable Vivy user home and actual selected backend at :8787,
  Vite at :3015. Record A07 before committing as
  `feat: add removable SSH workspace controls and GUI status`.

## Task 3: Keep selected Recipe through UI pre-hooks

**Files:** UPDATE existing `ui/scripts/stage-ui-assembly.mjs`; NEW proposed
`ui/scripts/stage-ui-assembly.test.mjs`. No generated output is manually edited.

- [ ] Current pre-hooks always restage default, overwriting a manual selected
  projection. Add `TestDefaultRecipeRemainsDefault`, `TestExplicitRecipeSurvivesPrehooks`
  and `TestInvalidRecipeFailsClosed` as Node tests of argument selection/spawn.
  Run `node --test ui/scripts/stage-ui-assembly.test.mjs` for red.
- [ ] Add a generic optional `VIVY_UI_RECIPE` override resolved against repository
  root; absence continues to select recipes/default.vivy.yml. No SSH-specific
  shell branch. Preserve `go run ./sdk stage-ui --recipe <path> --out ui/src/generated`
  and the generated-marker check. A missing/invalid selected Recipe fails, never
  switches to default silently. Document this limited tooling change.
- [ ] For the disposable selected Recipe, set VIVY_UI_RECIPE to its path during
  `pnpm typecheck`, `pnpm test`, `pnpm build` and `pnpm dev` in ui/. Confirm each
  pre-hook retains selected Module. Unset override for normal full `just ci`;
  run selected checks separately and record both identities. Commit as
  `dev: preserve explicit UI Recipe selection through pre-hooks`.

## Acceptance and stop conditions

Return A07, action schemas, shared-owner evidence, real split GUI results and
generic Recipe-selection checks. The default Generation is not changed. Stop
on missing authority, unsupported slots or shared ownership; add a concrete
gate correction rather than shell internals. Rollback disposes UI contributions
and closes owned transport; server-bound operations remain reconcilable.
