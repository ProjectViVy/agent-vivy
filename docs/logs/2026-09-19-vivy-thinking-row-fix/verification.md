# Verification — thinking row: one live renderer, left-to-right summary

Environment: split dev pair already running (`vivy` backend `127.0.0.1:8787`,
Vite dev UI `127.0.0.1:3015`), Windows. The root tree also carries another
lane's uncommitted approval-timeout work (`internal/`, `sdk/ui/src/module.ts`,
`ui/src/i18n/{en,zh}.ts`, `docs/TODO.md`); none of it was touched by this change.

## Unit / static

| Command | Result |
| --- | --- |
| `cd ui; pnpm typecheck` | OK (`tsc --noEmit`, exit 0) |
| `cd ui; pnpm test` | OK — 45 files / 373 tests (4 new `ChatView.test.tsx`, 1 new `ReasoningRow` case) |
| `just ui-ci` (repo root) | OK — frozen-lockfile install, typecheck, tests, `vite build`, i18n completeness (`en=1441` / `zh=1441` keys), cross-face contract |

### The new tests fail on the pre-fix source (regression proof)

Method: save the three source files as a patch, `git checkout --` them (the
`547e656` versions), run the chat suites, then `git apply` the patch back.

| Test | Pre-fix | Post-fix |
| --- | --- | --- |
| `ChatView` — streaming reasoning segment renders through the reasoning row alone | FAIL (a second `<details>` thinking disclosure in the transcript) | PASS |
| `ChatView` — prints a streaming answer once | FAIL (`expected 2 to be 1`) | PASS |
| `ChatView` — settled run reuses the projected message | PASS | PASS |
| `ChatView` — falls back to the projection without events | PASS | PASS |
| `ReasoningRow` — prints the running summary left to right like the settled one | FAIL (running summary is `justify-end`, not `flex-1 truncate`) | PASS |

Pre-fix run summary: `Tests 3 failed | 5 passed`; post-fix: `15 passed`.

## Live UI (`http://127.0.0.1:3015`, real split pair)

Probes are gitignored scratch under `ui/.dev-workdir/`: `thinking-smoke.mjs`
(read-only walk over the console's sessions) and `thinking-live.mjs` (creates a
session, sends one prompt, samples the DOM while the run is in flight, then
deletes the session through `session/delete`).

Read-only walk (`thinking-smoke.log`, 8 sessions, best = `你好`):

```text
reasoningRows=10  thinkingDisclosures=0  streamingLabelShown=false  assistantBubbles=13
every row: hasSummary=true  leftGap=18  rightGap=0  justify=normal
```

`leftGap=18` is the 2 px separator plus its 8 px margins: the summary starts
right after the 思考过程 title; `rightGap=0` is the `flex-1` stretch to the row's
right edge; `justify=normal` is flex-start (no right alignment).

Real run, before vs after (`thinking-live-before.log` / `-after.log`), one
prompt (`请先思考再回答：1+1 等于几`), sampled every 120 ms while the run was in
flight:

| Probe field (peak running sample) | Pre-fix | Post-fix |
| --- | --- | --- |
| `runningRows` | 1 | 1 |
| `detailsInBubbles` | **1** | **0** |
| `streamingLabel` (思考过程（进行中） on screen) | **true** | **false** |
| `emptyBubbles` (`…`) | 0 | 0 |
| running summary geometry | `hasSummary=false`, no left-aligned summary | `leftGap=18`, `rightGap=0`, `justify=normal` |
| `maxDuplicateTextGroup` (same text in two bubbles at once) | 1 | 1 |
| `PROBLEMS` | 3 reported (second disclosure, `<details>` in a bubble, not left-aligned) | `[]` |
| `CONSOLE_ERRORS` | — | `[]` |

The pre-fix sample shows the user's first symptom directly: the transcript's last
text was `思考过程（进行中）\n\n…` — a card bubble carrying the second thinking
disclosure next to the running reasoning row. Post-fix the running run shows one
reasoning row, no `<details>` in any message bubble, and the summary left-aligned
and stretched. Settled sample: 1 reasoning row (`data-state="ok"`), 0 `<details>`,
answer text present once. Screenshots: `thinking-live-settled.png`,
`thinking-settled.png`.

Not exercised live: nothing — the running state, the settled state and the
absence of the second disclosure were all observed on the real page. The text
duplication counter only ever saw one bubble per text in this run (the mock
backend's answering text arrives short); that path is pinned by the
`ChatView` "prints a streaming answer once" test, which fails pre-fix with
`expected 2 to be 1`.

## `just ci` (repo root)

Green on the merged tree, exit 0: `fmt-check`, `ui-ci`, `vet`, `test`
(including `TestCheckedInProviderConformanceMatchesExecutedSuites`),
`headless-compile` and `plugin-ci` all pass. The UI suite reports 45 files /
373 tests; i18n completeness is `en=1441` / `zh=1441` keys and the 13 shared
semantic units conform.

Earlier in the iteration the same gate stopped at
`agent-vivy/sdk/internal/conformance`
(`TestCheckedInProviderConformanceMatchesExecutedSuites`, 196 s) because the
checked-in conformance results differed from the executed suites
(`reproduction_test.go:93`) — with 0 per-suite failures and 0 `source digest = `
mismatches, i.e. the checked-in `sdk/internal/assembly/conformance_results.json`
pin was stale against another lane's uncommitted `internal/` work (the same
condition recorded in
`docs/logs/2026-09-19-vivy-tool-reasoning-rows/verification.md`). The resident
approval lane's commit `cf9ac64` landed the refreshed pin and the halt cleared.
This change could not have moved the pin either way: the conformance suites hash
only `internal`, `plugins/*`, `faces/*` and `sdk/internal/testdata/full-ui-module`
(`sdk/internal/conformance/reproduction_test.go:440-468`).

The `LIVE-TAIL-DERIVED-DUP` row in `docs/TODO.md` §0.1 landed with `cf9ac64`,
which staged that file while carrying its own approval-timeout rows; this
iteration's commit therefore carries only the chat components, their tests and
this record.
