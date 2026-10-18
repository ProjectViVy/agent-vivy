# E1-BML-04: Real-backend memory UI smoke Implementation Plan

> **For agentic workers:** After separate execution authorization, use `superpowers:executing-plans` task by task. Do not create child sessions. Steps use checkbox syntax.

**Goal:** Verify the shipped BML management UI against a real backend, including persistence, a genuine stale-CAS draft and targeted deletion.
**Architecture:** One SDK-packed default backend, a disposable configuration/data root, and Vite at the manual split origin `http://127.0.0.1:3015`. Browser actions travel through existing RPC/ActionHost; no network or transport mock supplies memory results.
**Tech Stack:** Existing Playwright, Vite, pnpm 10.33.0, SDK pack, BML SQLite/FTS5.
**Spec:** [Architecture](architecture.md), A1–A6/A8–A9; BML-R5/R6. Predecessor/status: [index](index.md).

## Global constraints

- Read `ui/AGENTS.md` before execution. Do not edit `ui/src/generated/**` or product UI/Recipe files.
- Preserve existing mocked `view.test.tsx`; add a real-path smoke rather than rewrite it.
- Use a fresh pack for the exact candidate and isolated root; never reuse a cached binary without verified source/Generation identity.
- Run at split Vite `:3015`, not embedded `:8787`. Fail on occupied fixture ports; do not kill unrelated listeners.
- Configure `full_auto` only in the disposable test config for existing module writes. No tenant policy or global config change.
- No inference is needed in this Story and no paid endpoint may be called.

## Files and interfaces

- Create (proposed): `ui/playwright.memory.config.ts` — one worker, zero retries, Vite server/proxy configuration, owned artifact locations.
- Create (proposed): `ui/e2e/memory-backend.ts` — isolated packed-process lifecycle and bounded health/readiness checks.
- Create (proposed): `ui/e2e/memory.spec.ts` — real UI CRUD, restart and stale-CAS smoke.
- Read only/reuse patterns: `ui/playwright.notebook.config.ts`, `ui/e2e/notebook.spec.ts`, `ui/e2e/notebook-backend.ts`. Its helper imports notebook-specific config, so do not import it unchanged or start its pack/config side effects.
- Read only: `plugins/vivy-memory/ui/vivy-memory/src/{memory-client.ts,view.tsx,dialogs.tsx,rules.tsx,view.test.tsx}`, its i18n catalog, `ui/AGENTS.md`.

Proposed local harness contracts:

```ts
startMemoryBackend(binary: string, configPath: string, address: string): Promise<ChildProcess>
stopMemoryBackend(child: ChildProcess | null): Promise<void>
invokeMemory(page: Page, actionId: string, input: unknown): Promise<Record<string, unknown>>
```

Use `spawn` argument arrays and the existing process-local `VIVY_CONFIG` pattern.
Await confirmed exit before reopening the same root. A forced stop is a reported
failure for the graceful-restart acceptance step, not an unnoticed successful
restart. `invokeMemory` follows the existing real bootstrap/WebSocket test pattern,
initializes `vivy.rpc.v1`, invokes `module.action.invoke` with
`module_id="vivy/memory-bml"`, and validates outer `ok` separately from inner
`listed/applied/failed`. Never log bootstrap tokens or auth headers.

Produces one serial Playwright smoke with screenshots, real action outcomes,
record IDs/revisions and before/after-restart evidence. There is no new runtime
or public UI API.

## Review focus

1. Cached pack points to a different SHA: pack afresh into owned scratch and record candidate/Generation/digest.
2. UI local state survives while backend persistence fails: stop/restart the real process with identical root and reload the page.
3. Conflict is faked by a mocked response: use two real browser pages holding the same backend revision.
4. Delete appears successful but search still returns the ID: verify via independent real action read and reload after restart.
5. Backend outage appears as an empty successful list: stop the backend and require the visible connection/error state, never success.

## Ordered tasks

- [ ] **1. Recheck existing UI contracts.** Read current `view.test.tsx`, action DTOs, and en/zh catalog. Run existing UI tests/typecheck without altering their mocks:

  ```sh
  pnpm --dir ui test
  pnpm --dir ui typecheck
  ```

  Expected: normal existing gates pass. Do not inherit the historical log's test count as today's result.

- [ ] **2. Prepare one isolated selected pack and config.** Use owned scratch `.workspace/issue81-bml-e2e/<unique-run-id>/` (proposed). Pack once per candidate with `go run ./sdk pack --recipe recipes/default.vivy.yml --output <owned-pack-path>` using argument arrays in the harness, and retain the resulting identity/manifest plus binary digest. Put config, all state and workspace beneath that run root: `storage.data_dir`, `storage.sqlite.path`, `runtime.workspace_root`; backend address `127.0.0.1:8806`; `server.allowed_origins: []`; `governance.profile: full_auto`. Set process env `VIVY_CONFIG` to that exact config. Follow SDK/CLI validation rather than fabricating sealed identity. Never write daily home or activate another Recipe.

- [ ] **3. Start the split test pair.** In `playwright.memory.config.ts`, use `testMatch: ['memory.spec.ts']`, `workers:1`, `retries:0`; Vite command `pnpm dev` in `ui`, `VIVY_BACKEND_ADDR=http://127.0.0.1:8806`, URL/baseURL `http://127.0.0.1:3015`, `reuseExistingServer:false`. The spec starts/stops only its packed backend and polls `/healthz` with a bounded deadline and process-exit diagnostics. Ensure Playwright artifacts stay under the owned run root. If browser/toolchain/port is absent, stop with a precise prerequisite result; do not install unknown software or reuse someone else's server.

- [ ] **4. Exercise public UI creation and restart.** Complete only existing non-destructive onboarding required to reach `/memory`; a welcome-completed localStorage flag may suppress welcome chrome but cannot replace backend setup. Use existing accessible names from the English catalog, `getByRole`/`getByLabel`; no new product test IDs. Add `bmluiprobe <random synthetic body>` using the existing dialog, assert the visible row/details and independent real `vivy.memory.get` result (`listed`, same ID, revision 1). Stop the backend gracefully, await exit, start the same binary/config/root, reload `/memory`; require the same ID/body/revision and truthful database-present status.

- [ ] **5. Exercise correction and a real stale editor.** Open two pages on that same record and start edit mode in both at revision 1. Save a correction in page A; real get must return revision 2 and corrected body. Save page B's different stale draft; require the existing conflict state, draft retained, and backend still at revision 2 with A's body. Use the existing refresh/retry affordance; do not force overwrite or demand a new conflict UI. Verify search finds the corrected marker and excludes the old marker. If a source defect appears, preserve it and stop; no production UI fix is authorized by this plan.

- [ ] **6. Exercise targeted deletion and outage.** Confirm delete is disabled until a reason is provided, then remove with the displayed current revision and a synthetic reason. Require real applied outcome, missing row/selection after reload, real get `failed/memory_not_found`, and no selected ID in search results. Restart the same backend again and recheck absence. Finally stop it and reload: require the existing visible connection/error state with no fabricated successful list. Do not claim deletion of history, Recap or Journal.

- [ ] **7. Run and review the real smoke.**

  ```sh
  pnpm --dir ui exec playwright test --config playwright.memory.config.ts --project chromium
  ```

  Define a `chromium` project in the proposed config so this command is exact. Expected: smoke executes with no route interception/mocked RPC, all assertions pass, no skipped scenario; screenshots and action results match real backend state. Run typecheck on the new harness using the existing UI setup; the final integrator runs `just ci`. Independent reviewer inspects pack freshness, captured outcomes, browser origin, conflict and restart evidence. Commit only the new harness/spec and owned evidence with the responsible human identity.

## Stop conditions, acceptance and rollback

Missing browser/Go/pack prerequisites, busy ports, unsealed identity, policy denial,
outside-root writes, forced graceful-restart termination, false UI state or a
required skip blocks 04 only. Record action, target, reason and known outcome;
01–03 do not depend on browser availability. No silent fallback to mocked tests.

Accept with real split-origin evidence for create→restart→get/search→edit/conflict
→remove→restart plus truthful outage, reviewed against the exact candidate. The
ordinary mocked UI suite remains regression evidence, not the real-backend pass.
Rollback stops only owned processes; remove only this run's disposable root after
copying evidence. Do not remove shared caches, daily data or another session's files.
