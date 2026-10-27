# UI overhaul Block 6 — verification

Environment: macOS arm64 worktree
`/Users/mastwet-macmini/pjvv/agent-vivy/.worktrees/ui-overhaul/agent-vivy`
(branch `feat/ui-overhaul`). UI gates run with the pinned toolchain
(`PATH=$HOME/.local/bin:/usr/local/go/bin:$PATH GOTOOLCHAIN=go1.26.4`);
localStorage suites with `NODE_OPTIONS=--localstorage-file=/tmp/vitest-ls.db`.

## Static gates

| Command | Result |
| --- | --- |
| `pnpm typecheck` (stages assembly first) | clean, `tsc --noEmit` no errors |
| `pnpm test` (88 files) | 709/710 passed; the single red is the pre-existing `STORE-RUN-FAILED-COPY` (`src/lib/store.test.ts:568`), unchanged from stable |
| `pnpm vitest run src/theme/tokens.test.ts` | 7/7; **src palette 0 (baseline 0), src textSize 0 (0), src hex 0 (0)**; plugins 29/45/0 untouched |
| `pnpm build` | built in 1.66 s |
| `node scripts/check-i18n-cross-face.js` | PASS: 13 shared semantic units |
| `node scripts/check-i18n-cross-face.test.js` | 8 tests, 8 pass |
| `node scripts/check-i18n-completeness.js` | exit 1, unchanged pre-existing red: three `Uncatalogued Han` rows at `ui/src/lib/failure.ts:56-58` (tracked in `docs/TODO.md` I18N-CHECK-RED) |
| `git status ui/src/generated` | empty — assembly regeneration produced no diff |

## Browser evidence

Isolated pair only: backend `127.0.0.1:8796` with
`VIVY_USER_HOME=/tmp/vivy-ui-overhaul` and the mock provider on
`127.0.0.1:9913`; worktree Vite on `127.0.0.1:3016`. The owner's daily pair
(`:3015`/`:8787`) was not touched. Capture scripts (gitignored):
`ui/.audit/capture-block6.mjs` (route sweep) and `ui/.audit/capture-panels.mjs`
(files/todos panels). Artifacts in this directory: `block6-report.json` plus 52
PNGs.

### Route sweep — every family × every width

`capture-block6.mjs` seeds `vivy.theme` per family and visits all eight routes
(`/`, `/dashboard`, `/approvals`, `/mcp`, `/settings`, `/skills`,
`/cron-tasks`, `/lifecycle`) at 390 / 768 / 1280 for all seven families
(`default`, `dark`, `pink`, `miku`, `rice`, `fluor`, `indigo`):

- **168 combos, 0 errored, 0 with document-level horizontal overflow.**
- 28 element-level offenders remain, all benign and identical across families:
  - 21 × `span.sr-only` "Context usage" (`clientWidth 1`) — visually-hidden
    screen-reader text, present at every width; does not affect layout.
  - 7 × lifecycle `TabsList` (`inline-flex … overflow-x-auto`) at 390 only —
    the mobile tab strip is intentionally horizontally scrollable
    (`client 358 / scroll 399`); it scrolls, the document does not.
- Screenshots: `route-<route>-default-<width>.png` for all eight routes at
  390/768/1280, plus `route-<route>-miku-<width>.png` (a dark family) for the
  same matrix — 48 route PNGs. The other five families are covered by the
  numeric overflow sweep in `block6-report.json`.

### Panels — files + todos

`capture-panels.mjs` opens each panel in isolation (fresh page per panel, so
the mobile modal sheets do not block each other):

| Panel @ width | Document overflow | Offenders | Notes |
| --- | --- | --- | --- |
| todos @1280 | no | 1 (`sr-only`) | `#session-todo-panel` present |
| files @1280 | no | 2 (`sr-only`) | panel opens inline |
| todos @390 | no | 3 | 2 × `sr-only` + the shared `sheet.tsx` header (`flex … text-center sm:text-left pr-12`) at 19 px inner overflow — a pre-existing primitive measurement, no document overflow |
| files @390 | no | 2 (`sr-only`) | opens as a mobile sheet |

### Targeted probes (during the sweep)

- Cron empty state @768: before the detail-column fix the master column was
  78 px and the 95 px action button clipped (`scroll 86 / client 78`); after
  `md:grid-cols-[minmax(0,1fr)_16rem]` the master is 142 px, button fits,
  `scroll == client`.
- Skills name @768: the `vivy-advance-dev-env` row overflowed 9 px before
  `truncate` + `title`; clean afterwards.

## Not run

- `just ci` / `just wiring-ci` / `just kernel-ci`: no Linux/WSL checkout on
  this macOS host; Block 6 is UI-only and the UI gates above are the affected
  surface. Recorded per the validation policy.
- Embedded-UI e2e (`pnpm e2e`): known red on stable (EMBEDDED-E2E-LOCALE),
  not re-run.
