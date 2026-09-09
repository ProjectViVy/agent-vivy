# Verification

## Local checks

- `node --test scripts/check-plugin-v1-fixtures.test.mjs` — passed: 5 tests.
- `node scripts/check-plugin-v1-fixtures.mjs` — passed: 9 indexed cases
  (2 accepted, 7 rejected).
- `cd ui && pnpm install --frozen-lockfile && pnpm build` — passed; Vite built
  the production assets required by `go:embed`.
- `cd ui && ./node_modules/.bin/tsc --noEmit` — passed.
- `cd ui && ./node_modules/.bin/vitest run` — passed: 31 files and 274 tests.
- `node scripts/check-i18n-completeness.js` — passed: English and Chinese each
  contain 1,388 keys and 138 placeholders; runtime-copy audit clean.
- `node --test scripts/check-i18n-cross-face.test.js` — passed: 8 tests.
- `node scripts/check-i18n-cross-face.js` — passed: 13 shared semantic units.
- A Python/YAML contract check confirmed that `backend` and `ui` have no
  dependency on one another, `aggregate` requires both, and the workflow
  invokes `just backend-ci` and `just ui-ci` respectively.
- `git diff --check` — passed.

## I18N baseline transition

- Before PR #5 merged, `cd ui && ./node_modules/.bin/vitest run` reproduced the
  expected baseline: 16 failed and 197 passed assertions caused by stale
  Chinese text expectations.
- After PR #5 and its main-CI follow-up merged, this branch was updated to
  current `main` and the UI/I18N checks were rerun as recorded below.

## Full CI

`just ci` could not be executed in this Linux workspace because `just`, Go,
and PowerShell are unavailable. The pull request's Windows Actions run is the
authoritative full-path verification and must show the independent backend
and UI results before the aggregate status is accepted.

- Actions run 7 proved the lane split: `ui ci` passed independently, while
  `backend ci` reached `go test ./...` and failed only
  `TestCronSchedulerFiresDueJobAndWritesBack`.
- The failed row already had `LastStatus: ok`; its 120 ms next recurrence had
  elapsed before the loaded Windows runner could observe it. The test now
  uses a 30-second recurrence because it validates only the first write-back.
