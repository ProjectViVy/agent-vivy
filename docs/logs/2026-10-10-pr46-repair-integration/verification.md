# Verification

## Integrated checkpoint

- Exact old-pin clean sibling compilation reproduced all five unresolved Laputa API failures; the reviewed new-pin clean sibling headless and embedded compile checkpoint passed. Full final clean-artifact verification is pending below.
- Root real browser: `PLAYWRIGHT_CHROMIUM=/usr/bin/chromium pnpm --dir ui exec playwright test --config playwright.queue.config.ts`: PASS (1 test, 24.7s). Runs headless backend :8787, split Vite :3015, isolated SQLite and loopback model :8797; busy high-thinking PNG survives reload, recall and resend and reaches the actual second model request. Original local attachment FIFO and text-only recall were independently RED.
- Independent final recovery review: both previously reproduced parked cancellation retry and sealed-carrier queue-edit failures now PASS. Control replay/close/write retry, decision ownership, encoded DTO ceiling and SQLite cleanup tests passed. PostgreSQL cleanup parity also exercised in the queue lane's live PostgreSQL17 suite.
- Root live PostgreSQL17 full suite: `go test ./internal/storage/postgres -count=1`: PASS (23.447s before final control cleanup). Final integrated CI uses the isolated live DSN to execute final storage tests too.
- Focused lane tests, race tests, provider-failure injection and RED-to-GREEN evidence are recorded in the M1–M6 and queue-editor iteration logs. No live provider calls were needed.
- Final internal source SHA256: `f53cc3550f51e107871f5394229b9d4345d3730197b3d576f83d07ffd8d1f53e`. Only source-bound digest fields were refreshed; no evidence pass flags were edited. `go generate ./internal/generated/assembly` succeeded and reproduced the checked-in declared headless form byte-for-byte.

## Final gates

`just ci`, executed source-bound conformance suites, final clean sibling headless/embedded builds, minimal/default pack and inspect, and integrated race results will be recorded after those gates finish.

## Environment and limits

Go1.26.4, PowerShell7.6 and just1.42 were installed into excluded task scratch because the cloud image's `go` is an unrelated CLI. Frozen pnpm installs used `PNPM_CONFIG_REGISTRY=https://registry.npmjs.org` because the repository's npmmirror endpoint is blocked by this environment; dependency lockfiles were not changed. An earlier conformance attempt failed that mirror access and another was stopped before a newer source patch; neither is counted as passing.

Public Laputa module Origin.Hash and six checksums match the exact lock. Broad `go mod verify` encounters missing ziphash for existing local v0.0.0 replacements and is not claimed as passing. Scripted prefix write/read evidence validates accounting, not live monetary savings. Windows diagnostics runtime behavior is untested; cross-compilation passed. Full accepted turn DTOs must fit the configured durable event ceiling.
