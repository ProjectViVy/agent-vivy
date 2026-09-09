# Verification

## Commands (run in `../agent-vivy-trajectory` worktree, branch `feat/trajectory-panel`)

| Check | Result |
|---|---|
| `pnpm install --frozen-lockfile` (ui) | pass (19s, shared pnpm store; esbuild postinstall ignored but build works) |
| `pnpm typecheck` (ui) | pass (0 errors) |
| `pnpm test` (ui) | pass — 16 files / 122 tests, incl. new `trajectory-utils.test.ts` (17 tests) |
| `pnpm build` (ui) | pass (2207 modules, dist emitted) |
| `just ci` (worktree root) | **pass, exit 0** — fmt-check · go vet ./... · go test ./... · headless-compile · ui-ci |

> Fresh-checkout note: a clean tree has no `ui/dist`, so the first `just ci`
> fails at `go vet ./...` on `ui/embed.go`'s `go:embed all:dist` before the
> first `pnpm build` runs. Normal dev trees carry a prior build's `ui/dist`.
> This run built the UI first, then `just ci` passed end to end. Tracked as
> `docs/TODO.md` §0.1 `UI-CI-BOOTSTRAP`.

## Static checks

- `git status` in worktree: only this deliverable's paths changed.
- No audit leftovers in `ui/src`:
  `grep -ri "audit|审计" ui/src` → 3 benign hits only: the Evolution-page
  “Auditable Evolution Governance” subtitle (zh/en) and `diva-preview-data.test.ts`’s
  `not.toContain('audit')` assertion.
- `ui/src/components/audit/` no longer exists; `DIVA_AUDIT_EVENTS` removed.
- i18n zh/en parity enforced by `src/i18n/index.test.ts` (passes) after
  removing `audit`/`settings.preview.audit` and adding `trajectory`.
- `TrajectoryPanel` imports no `src/lib/demo-api.ts` and no RPC/api layers:
  data comes from `trajectory-demo-data.ts` static constants only.

## User-visible smoke (browser, `http://127.0.0.1:3016/dashboard`)

Headless Chromium smoke against the worktree's Vite dev server (port 3016,
same `ui/` sources; the always-on :3015 split pair serves the root tree).
Storage pre-seeded to dismiss the first-run welcome wizard and demo banner. All
checks passed, `pageerror` count 0:

1. `/dashboard` renders `Dashboard`, tabs = **Overview / Token / Trajectory**,
   with no Audit tab.
2. Trajectory tab: toolbar present (Trajectory Toolbar), one timeline (24 spans,
   3 lanes), ledger with 24 rows, 8 `Request #N` chips, and spy spans matching
   the record count.
3. Click row `rec-4` to open the detail panel (Input/Output/Thinking tabs); close works.
4. Click the `Request 1` chip to open request details—header `Request 1 · #1 ·
   Step 1`; Summary shows Status=Complete / Provider=deepseek / Model=deepseek-chat
   / Tool Calls=1 / Result text; Summary/Usage/Timing tabs are present.
5. The “Actual Duration” toggle re-projects spans: first span width 34.4px → 51.4px.
6. Collapse All Runs → summary rows `…#1 · Collapsed · 7 records` / `#2 · 7` /
   `#3 · 8`; record rows drop from 24 → 2; expanding restores 24.
7. An error row exists (`data-error`, `bash · pnpm build` → `exit 1 · Artifact check failed`).
8. Search “build” → 2 rows, all matching; clearing restores the list.
9. Drag-select a timeline range → ledger rows outside the range dim (opacity 0.35,
   12 rows); after focusing the track, Escape clears the selection (0 rows dimmed).
10. Run labels `#1/#2/#3` and the session-start `Session` label render correctly.
11. Three-lane labels Input/Model/Tools render at 7/21/35px (confirmed at DOM
    breakpoints); `src/i18n/index.test.ts` provides the zh/en dictionary-parity
    assertion (browser language-switch smoke was not run this time).
