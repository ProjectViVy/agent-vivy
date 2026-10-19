# Issue 81 L4 Toolkit matrix — 2026-10-19

Lane: `.worktrees/l4-toolkit-matrix/agent-vivy`, branch `feat/l4-toolkit-matrix`.
Base: `e9609296` (`origin/main`); the lane was originally cut from
`16808b5a`, an unpushed docs-only translation commit that this PR deliberately
excludes. Pinned Laputa source lock `4b2bec2cc2ab3374b7ec1ab8d448e612c1e4db24`.

This delivery implements the E4/PG-TM batch selected from
`docs/superpowers/plans/issue-81/product-gaps/` on
`origin/plan/issue81-product-gaps`: T0 (reproducible browser gate) and the
missing TK-13 verification slices selected for T1–T4, followed by T5
reconciliation. It adds no product behavior, runtime API, schema, dispatcher,
or second admission path.

Candidate identity for the browser evidence:
`generation 9de83d553ac96d483d7bab25cd7bda2ab3816527ecae3734a6284e332179ed95`,
42 modules, packed from this lane by `scripts/toolkits-e2e.mjs`. Evidence
commits are `2ba1312a` + `908cb039` (T0 launcher and hardening),
`07e8b131` (T1–T4 tests and generated UI Assembly), and the docs/TODO
reconciliation commit that contains this log.

## What changed

### T0: one reproducible Toolkit browser entry

`just toolkits-e2e` now runs `node scripts/toolkits-e2e.mjs`. The launcher:

- validates distinct integer ports (`VIVY_TOOLKITS_E2E_BACKEND_PORT` /
  `VIVY_TOOLKITS_E2E_UI_PORT`, defaults 18787/3015) and refuses a port that is
  already bound before preparation;
- builds `vivy-sdk`, rebuilds the embedded `ui/dist`, packs
  `recipes/default.vivy.yml` into lane-local
  `.workspace/toolkits-e2e/gate/default`, and runs
  `inspect-artifact` on the packed directory before Playwright starts;
- passes `VIVY_TOOLKITS_E2E_BINARY`, the ports, `VIVY_UI_PORT` and
  `VIVY_BACKEND_ADDR` to the existing configs;
- runs the lifecycle suite then the accessibility suite sequentially, fails
  closed on either exit, and forwards SIGINT/SIGTERM to the active child;
- merges `-tags=http2legacy` into existing `GOFLAGS` only on host Go ≥ 1.27
  (the pinned `x/net` lacks the legacy HTTP/2 symbols used by the pinned
  gRPC; CI's go.mod toolchain does not need the tag);
- keeps the direct-Playwright external-backend mode as an explicit opt-in,
  not as the reproducible gate.

`ui/playwright.toolkits.config.ts` now derives backend/UI ports, binary path,
platform binary suffix (`vivy`/`vivy.exe`) and pnpm invocation from the
environment instead of hardcoding them. The accessibility config remains a
thin `testMatch` override.

### T1: source/publication/storage fault evidence

Added to `internal/bundles`:

- `TestToolkitBeforeCommitFailurePreservesCommittedEpoch` drives a real
  publication through the owner `BeforeCommit` boundary, fails it, and proves
  the stored epoch, installation state, active View and drain gate are
  unchanged; a fresh prepare/publish then commits epoch 1.
- `TestToolkitPreviewSourceChangeDoesNotSubstitute` changes the source after
  preview, rejects the other preview's digest as `preview_stale`, and proves
  the installed component page contains the reviewed bytes rather than the
  mutated source.
- `TestToolkitReinstallDoesNotInheritConsent` arms a Hook, uninstalls it,
  reinstalls the same BundleID from another source, and proves the new
  InstallID has no durable or inspected consent.

### T2: snapshot recovery fault evidence

Added `TestToolkitCorruptSnapshotIsUnavailableWithoutLatestFallback` in
`internal/bundles`: after publishing a second epoch, the intact captured
snapshot restores its exact epoch 1 View; a payload whose digest no longer
matches fails `snapshot digest mismatch`; deleting the captured artifact makes
restore fail instead of returning the current epoch 2 catalog.

### T3: captured Hook failure matrix

Added `TestToolkitCapturedHookFailureMatrix` in `internal/runtime`. Each row
creates a real installed/armed Hook, captures a run, and dispatches
`PreToolUse` through the real platform shell: allow, exit 2 deny with stderr,
exit 1 fail-closed, non-JSON allow, envelope rewrite, non-object
`updated_input` fail-closed, timeout fail-closed, and missing executable
fail-closed.

### T4: UI stale/blocker/viewport evidence

- `ToolkitPage.test.tsx` adds a stale installation-detail response test:
  select A, select B, resolve B, then resolve A; the page keeps B's detail.
- `ToolkitCandidate.test.tsx` renders the prepared-change card with and
  without blockers and proves the run/session blocker list and hint appear
  only when the operation carries blockers.
- `toolkits-accessibility.spec.ts` adds a third browser case that opens
  `/toolkits` at 768px and 1440px and asserts no horizontal document overflow.

### T5: reconciliation

This log, `verification.md` and `acceptance.md` record the exact evidence and
the remaining B/A gaps. `docs/TODO.md` closes the `TOOLKITS-E2E-RECIPE` row and
points `CONTENT-BUNDLES-20` at the new evidence while leaving it IN PROGRESS;
the closed row is archived in `docs/COMPLETE.MD`.

## What was intentionally not done

- No Toolkit product code changed. The new tests assert existing behavior;
  no assertion was weakened to make them pass.
- No new MCP transport fixture, admission API, schema, status, scheduler, or
  dispatcher was added. Hook/MCP rows use the existing runtime adapters.
- The full B1–B14/A01–A28 matrix is **not** declared accepted. Rows without
  current evidence remain explicit residuals in `acceptance.md`; this is a
  completed reconciliation, not completed product acceptance.
- `just ci`, `just test-postgres`, native Windows acceptance, and
  `just release-conformance` were not available on this host. The focused
  substitute commands and their exact failures/skips are in `verification.md`.
- Default ports 18787/3015 could not be used end to end because an unrelated
  Vite server from the root checkout occupied 3015 during the attempt. The
  launcher failed closed; the isolated 18887/3115 run passed. No foreign
  process was terminated.

## Deviations

- The plan names one log per Story (`issue81-toolkit-gate`, T1–T4 logs,
  `issue81-toolkit-matrix`). This lane was assigned one batch, so the evidence
  is consolidated here; T0's evidence is the first section.
- `just` is not installed on this host. The recipe body was run directly with
  `node scripts/toolkits-e2e.mjs`; the `justfile` recipe itself is unchanged
  in behavior from that body.
- The lane is macOS/arm64 (Darwin), not the Linux or Windows acceptance venue.
- The PG-TM index asks the central D1 writer to own public status wording. This
  lane closed only its own `TOOLKITS-E2E-RECIPE` row (archived in
  `docs/COMPLETE.MD`) and refreshed the `CONTENT-BUNDLES-20` evidence link; it
  did not rewrite the broader Toolkit index or external issue #20.
