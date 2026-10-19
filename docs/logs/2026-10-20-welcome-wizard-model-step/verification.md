# Verification — welcome wizard model step

Lane: `C:\Users\com01\Desktop\morediva\.worktrees\welcome-wizard-model\agent-vivy`
(Windows, `just 1.46.0`, `node v24.19.0`, `pnpm 10.11.0`, `go1.26.4`).
Base: `94e449ba` on the clean `feat/issue-91-base` line (vivy-code parity plus
the 2026-10-20 provider-catalog prune and provider-agentic migration). Earlier
runs against the discarded bloated `main` are superseded by the results below.

## Unit and component tests

```text
pnpm exec vitest run src/components/layout/welcome-model.test.ts \
  src/components/layout/WelcomeWizard.test.tsx
→ 2 files, 28 tests passed
```

Coverage:

- `welcome-model.test.ts` (21): executable-only rows (Profile `UNAVAILABLE` is
  filtered), vendor grouping, all prefill branches (legacy bundle name, sealed
  adapter + base URL, config fallback, unknown model fallback, custom gateway not
  overwritten), key status (registry entry / legacy overlay / none), upsert
  payload reuse vs new catalog-overlay id, `settings/update` selection payload,
  and the key-policy matrix.
- `WelcomeWizard.test.tsx` (7): loading and failure states with retry, prefill
  from backend settings, key-required rejection, already-configured key (no
  upsert), env-injection opt-in, typed key → upsert then `settings/update`
  (the key appears only in the upsert payload), and read-only/frozen advancing
  without writes.

## Full UI suite and gates

```text
pnpm --dir ui test
→ 79 files, 626 tests passed

pnpm --dir ui typecheck
→ tsc --noEmit, exit 0

node scripts/check-i18n-completeness.js
→ PASS: en=1574 keys / 161 placeholders; zh=1574 keys / 161 placeholders; runtime copy audit clean.

node scripts/check-i18n-cross-face.js
→ PASS: 13 shared semantic units; Web and TUI en/zh projections and arguments conform.
node --test scripts/check-i18n-cross-face.test.js
→ 8 tests passed

pnpm --dir ui build
→ vite v7.3.6, 2582 modules; dist/assets/index-qVqoyqqu.js (1,840.06 kB)
  and dist/assets/index-CWCywF8r.css (154.32 kB). TMP/TEMP were pointed at
  `.workspace/ci-tmp` because this Windows host denies esbuild deletion under
  the default temp path (`[vite:esbuild-transpile] remove …: Access is denied`).

just ci
→ not completed: the owner stopped the run before it finished and moved to
  manual verification. The full UI suite, typecheck, i18n gates, build and e2e
  below were run individually; backend `just ci` gates are not claimed.
```

## Browser e2e

```text
$env:GO_EXE = (Get-Command go).Source
pnpm exec playwright test e2e/welcome-wizard.spec.ts
→ 1 passed (13.4s; test 5.6s)
```

The spec pins:

- first-run auto-open, skip persistence, rerun entry;
- model step selects showing `DeepSeek` / `openai-completions` /
  `deepseek-flash` and the endpoint address from the pruned backend catalog;
- the provider dropdown opens above the `z-[200]` modal and a real option click
  changes the selection (`MiniMax`, then back to `DeepSeek`); switching back
  requires re-picking `deepseek-flash` because a vendor change clears the model;
- the password field starts empty and "Open in browser" is gone;
- Next without a key shows the explicit key-required message;
- the env-injection checkbox allows an empty-key save;
- done card deep-links to `/settings?tab=model` with the DeepSeek endpoint row
  selected, and the wizard stays closed after reload.

The event timeline was served by the real backend and the embedded bundle built
in this lane.

## Split-origin browser smoke

The live split pair (`just dev`, backend `:8787`, Vite `:3015`) was exercised
with a headless Playwright page against a fresh `VIVY_USER_HOME`:

- first-run wizard opened and advanced to the model step;
- all three comboboxes opened with `z-index: 250`; `elementFromPoint` at the
  option centre returned the option itself, not the modal overlay;
- selecting `MiniMax` changed the provider, `anthropic-messages` changed the
  DeepSeek protocol, and `abab6.5-chat` changed the MiniMax model;
- no browser console or page errors were recorded.

Before the fix, the same clicks left the options behind the overlay
(`SelectContent` was `z-50` under the wizard's `z-[200]`), which matched the
reported "下拉拉不开".

## Not run

- No live provider request or real API key was used.
- `ui-e2e` is not part of `just ci` (`CI-E2E-NOT-IN-GATE`), so the spec was run
  manually.
- No Go file changed in this lane; the rebased base carries its own backend
  verification record in `docs/logs/2026-10-20-provider-agentic-migration/` and
  `docs/logs/2026-10-20-provider-catalog-prune/`.
