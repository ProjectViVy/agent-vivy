# Verification — 2026-09-14 channel hardening batch

All commands run from the repository root on Windows (Git Bash), branch
`feat/channel-hardening`, unless noted.

## Focused package runs during development

| Command | Result |
|---|---|
| `go test ./internal/storage/... -count=1` | ok (sqlite 40s incl. the full 28-case conformance suite; postgres DSN-gated cases compile/vet/skip cleanly without `VIVY_POSTGRES_TEST_DSN`) |
| `go test ./internal/channelhost/ -count=1` | ok — includes the 7 new recovery/drain tests (`TestRestartRedeliversArmedIntentOfCompletedRun`, `TestRestartSettlesArmedIntentOfFailedRun`, `TestRestartRedeliversPendingIntent`, `TestRestartParksExhaustedPendingIntentAsFailed`, `TestStopAllWaitsForInFlightDelivery`, `TestOnRunEventDuringDrainKeepsPendingIntent`, `TestDeliveryAttemptsExhaustedMarksFailed`) |
| `go test ./internal/channelhost/ -race -count=1` | ok |
| `go test ./internal/runtime/ -run Provenance -count=1` | ok — includes new `TestRunWithHeadlessProvenanceAccepted` and `TestRunWithUnknownProvenanceSourceRejected`; existing `TestRunWithEmptyProvenanceSourceRejected` stays green |
| `go test ./internal/domain/ ./internal/rpc/ ./internal/app/ -count=1` | ok |
| `cd plugins/dingtalk && go test ./... -count=1` | ok — includes new `TestStreamSilentLinkRedials` (black-hole loopback gateway, shrunken ping/deadline, redial proven by a second ticket exchange) |
| `cd plugins/dingtalk && go test -race -count=1 ./...` | ok (after capturing the liveness knobs into the client at construction; the first race run exposed an unsynchronized package-var read from a leftover pingLoop and was fixed, not suppressed) |

## Full gate

| Command | Result |
|---|---|
| `just ci` | **PASS** (exit 0). One earlier run failed in `sdk` pack tests with a dingtalk source-hash mismatch; root cause was the intended source change against the module's self-describing `sha256` pin. Recomputed with `internal/sourcehash.Tree` and re-embedded into `plugins/dingtalk/vivy-module.yaml` + `module_v1.go`; the fixed point (declaring the new digest verifies as the new digest) was checked before re-running. Also `gofmt -w` applied to all touched Go files; `go vet` clean on touched packages. |

## Real-path smoke

Isolated-process boot (no production Journal touched — `VIVY_CONFIG`
pointed at a temp config with a temp sqlite path, port 18787):

- `go build -o <tmp>/vivy-smoke.exe ./cmd/vivy` — build ok.
- Boot with the isolated config: process listens, `GET /healthz` → 200,
  zero ERROR-level log lines.
- The log shows all five compiled-in channels traversing the new StartAll
  path (`channelhost: channel compiled-in but not configured; not started`
  × 5 — the line sits after the new required-`Deliveries` check), and the
  new startup prune ran silently (no prune warning). `channel/inspect`
  over RPC is reachable and protected by server authentication
  (`unauthorized` without a token) — the RPC contract is unchanged, so no
  authenticated probe was exercised here; its behavior is covered by
  `internal/rpc` tests.
- Temp binary/config/db removed after the smoke.

## Honestly skipped

| Check | Why |
|---|---|
| Live-platform smoke (telegram/dingtalk/feishu/qq/discord ears) | No real bot credentials in this environment; the ears' wire behavior is unchanged except dingtalk's redial, which is proven against the loopback gateway stub. Matches the honest-skip precedent of the C4–C7 filings. |
| Real-Postgres migration v21 run | No Postgres/DSN locally (long-standing CH-C1-N5 debt); the migration is covered by compile-time checks and the postgres-gated conformance cases will exercise it when `VIVY_POSTGRES_TEST_DSN` is set. |
| Browser UI exercise at `127.0.0.1:3015` | The batch changes no UI/RPC surface: Settings channels page, `channel/inspect`, and history provenance projections are byte-identical (only the `Source` constant replaced equal literals). The composed-organism boot smoke above covers the executable path. |
