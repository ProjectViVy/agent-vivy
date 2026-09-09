# Verification — 2026-08-25 VIVY UI i18n support

## `just ci` (repository root)

Result: **passed** (exit 0). Full pipeline:

- gofmt (*.go in cmd / internal / sdk / ui): no unformatted files
- `go vet ./...`: passed
- `go test ./...`: all passed (including the `-tags vivy_headless` headless-compile)
- UI `pnpm install --frozen-lockfile` + `pnpm typecheck`: passed
- UI `pnpm test`: **9 test files / 40 tests all passed**, including the new
  `src/i18n/index.test.ts` (9 cases: zh/en key consistency, interpolation,
  fallback, persistence, DOM lang) and the rpc / runtime-config /
  run-subscription / store / demo-api tests that previously failed because the
  `@` alias was missing
- UI `pnpm build`: Vite build succeeded (2188 modules)

> Note: during this round, `vitest.config.ts` was found to be missing `resolve.alias['@']`, causing 5 lib test files to report `Cannot find package '@/i18n'`. The alias was added to fix it; verification was not skipped.

## Browser smoke (http://127.0.0.1:3015, split Vite)

Inspected each view through DOM snapshots to confirm no regressions or raw-key
leaks after migration (default zh):

| Check | Result |
|---|---|
| App mount, default Chinese rendering | ✅ Navigation/top bar/settings/chat all render in Chinese |
| /masks mask library (including capabilities list) | ✅ No leaks; cards, capabilities, and details are all localized |
| /skills /memory /notebook /persona /dashboard /mcp /cron-tasks /approvals /lifecycle | ✅ All render normally; inspection found no `xxx.yyy` raw-key leaks |
| Settings-page “Language Preview” label (other implementation) | ✅ Present; clicking English **does not** change global copy (preview-only), as documented by the other implementation |

## Unverified items

- **Actual click path for the global language switch in Settings**:
  `LanguagePicker` is not mounted in `SettingsView` (the other party’s file,
  not touched in this round), so the browser has no real entry point to drive a
  global switch. EN rendering correctness is deterministically covered by
  `src/i18n/index.test.ts` (dictionary alignment + switching + persistence). See
  `acceptance.md` for the acceptance path after wiring.
- No visual screenshot was retained; this environment uses DOM text verification,
  consistent with existing logs.
