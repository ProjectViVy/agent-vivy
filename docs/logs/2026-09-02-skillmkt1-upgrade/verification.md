# Verification record: SKILL-MKT-1

## Focused tests (before submission)

- `go test ./internal/runtime/ -run "TestMarketplace" -count=1` → ok
  (12 cases including new `TestMarketplaceUpgradeMirrorsSnapshot` (snapshot mirroring +
  hosted deletion + root loose-file retention + manifest fields + create still 409),
  `TestMarketplaceUpgradeUpToDateWritesNothing` (manifest bytes unchanged),
  `TestMarketplaceUpgradeGuards` (unknown mode/no manifest/source mismatch), and
  `TestMarketplaceCheckUpdateStatuses` (four statuses + invalid-name rejection))
- `go test ./internal/rpc/ -run "TestMarketplaceInstallModeAndCheckRoute" -count=1` → ok
  (mode validation InvalidParams, upgrade/create pass-through, not marketplace-managed →
  CodeConflict, check route, unconfigured marketplace → MethodNotFound)
- `gofmt -w` + `go build ./...` + `go vet ./internal/{runtime,rpc,tools}/` → green
- `cd ui && pnpm typecheck` → green

## Product gates (`just ci` + `just ui-e2e`, background tail-check)

- `just ci` → **CI-EXIT:0** (`/tmp/ci-skillmkt1.log`)
- `just ui-e2e` → **E2E-EXIT:0, 17 passed (28.3s)** (`/tmp/uie2e-skillmkt1.log`)

## Real-path smoke (3015 split Vite, 2026-09-02)

- `just run` (8787, started after CI-EXIT) + `cd ui; pnpm dev` (3015), both ports 200.
- skills.sh reachable (`curl /api/search?q=commit` → 200).
- Playwright-driven (scratch script, deleted after the run):
  1. `/skills` → "Marketplace" tab → featured list shows `find-skills`;
  2. Click "Install" → real skills.sh snapshot download and successful install; the inline
     button becomes "Check for updates";
  3. Click "Check for updates" → real second download + byte-level hosted-set comparison →
     "Up to date" badge appears.
  - Output: `STEP install find-skills` → `STEP check-update button visible` →
    `STEP up-to-date badge visible` → `SMOKE-OK`.
- Smoke leaves a find-skills installation in `data/skills/` under the dev home (the product's
  own path, not one of the three air-gap restricted paths); the manual
  `upgrade_available` scenario depends on an upstream release and is covered by httptest
  replay.

## Notes

- Discriminating coverage: httptest replay asserts hosted-set mirroring (including
  deletion) and manifest guards; the `upgrade_available` scenario for a real upstream
  release does not depend on online timing.
