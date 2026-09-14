# Verification — channel tier-1

All commands run from the repository root on `feat/channel-tier1`
(Windows, Git Bash) unless noted. Every feature commit was gated by a full
`just ci` run; the final state was gated again after the docs landed.

## Feature 1 — failed-delivery list + redeliver (c773a66)

- `go test ./internal/storage/sqlite/... -count=1` — ok (29 conformance
  cases including the new CN-29 failed-listing partition).
- `go test ./internal/channelhost/ -count=1 -race` — ok, including the four
  new redeliver tests (success path deletes the row; re-failure re-parks
  with attempts+1 and exactly one Send; operator errors rejected: unknown
  run, non-failed row, channel not running, draining host; failed listing
  surfaces only failed rows).
- `go test ./internal/rpc/ -count=1 -run 'TestChannelDeliveriesRPC|TestChannelInspectRPC'` — ok.
- `pnpm exec tsc --noEmit -p tsconfig.json` in `ui/` — ok (the pre-existing
  `ui-sdk-face-compat` pin correctly forced the `sdk/ui` FaceClientAPI to
  gain the two new methods; `vitest` compat test green).
- P9: internal tree digest re-pinned (`go run ./sdk/internal/cmd/source-hash
  internal <sha>` iterated to the fixed point), reproduction_test.go +
  conformance_results.json updated;
  `go test ./sdk/internal/conformance/ -run TestCheckedInProviderConformance
  -count=1` — ok (150s).
- Full `just ci` — EXIT:0. One intermediate failure was fixed on the way:
  the four new i18n keys had to be declared in
  `scripts/i18n-cross-face-contract.json` webKeys (cross-face governance).

## Feature 2 — health probing + error classification (291a45e)

- Per-plugin suites (`cd plugins/<name> && go test ./... -count=1`) — ok for
  dingtalk, telegram, feishu, qq, discord, including the new Health tests:
  dingtalk `TestHealthClassifiesRedialingStream` (temporary while redialing,
  nil after reconnect), qq `TestHealthClassifiesRedialingGateway` /
  `TestHealthDeadAfterGiveUp` (dead after the cannot-identify give-up),
  feishu `TestHealthClassifiesRedialingGateway`, discord
  `TestHealthClassifiesRedialingGateway`, telegram
  `TestHealthReportsStartedEar`. feishu's `wsRedialDelay` became a var so
  its lifecycle tests can shrink it (same pattern as the siblings).
- `go test ./internal/channelhost/ -count=1 -run TestInspect` — ok
  (`TestInspectProbesStartedHealth`: healthy / classified dead / plain error
  defaults temporary / no Health face / not-started never probed).
- UI: tsc + `pnpm test` — ok (317 tests; channel-store fixture extended with
  the `health` field).
- P9: internal tree + all five plugin digests re-pinned to new fixed points
  (each plugin's `vivy-module.yaml` + `module_v1.go` self-describing pin,
  `reproduction_test.go`, `conformance_results.json`); P9 gate test — ok
  after one iteration (the first run failed because the JSON's five internal
  rows still carried the previous digest; replaced and re-run green).
- Full `just ci` — EXIT:1 on `agent-vivy/sdk/internal` panicking with
  "test timed out after 20m" while `frontend_v1` waited on a spawned
  `node vite build` (Windows process stall, not a code failure — the
  feature-1 CI had passed the same suite). Rerun of the package alone: ok
  (408s). Full `just ci` rerun: EXIT:0.

## Feature 3 — HITL notifications + channel commands

- `go test ./internal/channelhost/ -count=1 -race` — ok, including
  `TestApprovalRequiredNotifiesWithoutConsumingTarget` (notification text
  reaches the chat AND the later run.completed still delivers the reply),
  `TestApprovalCommandDecidesWithinSession` (/pending list, /approve prefix,
  actor `channel:fake:alice`, no run, no intent row),
  `TestApprovalCommandScopedToSession` (cross-chat approval invisible,
  DecideApproval never called), `TestNonCommandSlashTextOpensRun`
  (`/tmp shows a path` opens a normal run).
- `go test ./internal/runtime/ -count=1 -run TestServiceApprovalActorAttribution`
  — ok (actor recorded in the durable `tool.approval_decided` payload; empty
  and over-long actors rejected), plus the full runtime suite — ok (549s).
- RPC regression: `approval/respond` still calls `DecideApprovalWithReason`
  (actor `local_user`) — covered by the existing control_test suite in the
  full CI run.
- P9: internal digest re-pinned; gate test — ok.
- Full `just ci` — EXIT:0 (final state, includes docs).

## Isolation notes

- Postgres conformance remains DSN-gated (no real Postgres locally; that is
  the pre-existing CH-C1-N5 debt, not carried by this batch).
- No production `data/` journal was touched: the browser smoke ran against a
  throwaway instance (`VIVY_CONFIG` → `.workspace/tier1-smoke/config.yaml`,
  sqlite inside `.workspace/`, scratch removed afterwards).

## Real-path browser smoke (feature 1 UI, 2026-09-15)

Split dev pair on the throwaway journal: `go run ./cmd/vivy` with
`VIVY_CONFIG` (healthz ok, one seeded failed delivery row written through
`storage.sqlite` by a one-off in gitignored `.workspace/`), `pnpm dev` in
`ui/`, then Chrome at `http://127.0.0.1:3015/settings?tab=channels`:

- The Channels tab renders over the real websocket RPC: five compiled-in
  channel cards (钉钉/Discord/飞书/QQ/Telegram, all "no config envelope"),
  control plane "connected".
- The new 失败投递 (Failed deliveries) section lists the seeded row —
  Telegram · 12345678 · chanin_smokerow1 · "3 attempts" — with a 重投
  (Redeliver) button.
- Clicking 重投 exercises the full loop: the real
  `channel/deliveries/redeliver` RPC reaches `Host.RedeliverDelivery`, which
  correctly refuses (telegram is not started: "no config envelope"), and the
  UI surfaces the exact server refusal in the action-error banner — no fake
  success. (The happy path is proven by the channelhost/RPC tests; a real
  redeliver needs a live platform transport, which this offline smoke does
  not have.)

