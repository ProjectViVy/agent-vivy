# Verification

## Local checks

- `node --test scripts/check-plugin-v1-fixtures.test.mjs` — passed: 5 tests.
- `node scripts/check-plugin-v1-fixtures.mjs` — passed: 9 indexed cases
  (2 accepted, 7 rejected).
- `cd ui && pnpm install --frozen-lockfile && pnpm build` — passed; Vite built
  the production assets required by `go:embed`.
- `cd ui && ./node_modules/.bin/tsc --noEmit` — passed.
- A Python/YAML contract check confirmed that `backend` and `ui` have no
  dependency on one another, `aggregate` requires both, and the workflow
  invokes `just backend-ci` and `just ui-ci` respectively.
- `git diff --check` — passed.

## Known red baseline

- `cd ui && ./node_modules/.bin/vitest run` — failed as expected on the stacked
  base: 16 failed and 197 passed assertions. The failures are stale Chinese
  text expectations against the already-English implementation and remain
  owned by the active I18N lane.

## Full CI

`just ci` could not be executed in this Linux workspace because `just`, Go,
and PowerShell are unavailable. The stacked pull request's Windows Actions
run is the authoritative full-path verification. Its independent backend job
must complete even while the known UI assertions remain red.
