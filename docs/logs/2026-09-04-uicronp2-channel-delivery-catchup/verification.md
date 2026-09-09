# UI-CRON-P2 Verification

Date: 2026-09-04

## Verification Commands & Status

| Command | Target / Scope | Result |
|---|---|---|
| `just ci` | Full CI gate (fmt-check, ui-ci, vet, test, headless-compile, plugin-ci) | **PASS** |
| `pnpm --prefix ui e2e ui/e2e/cron-tasks.spec.ts` | Playwright E2E (offline Mock mode validates outbound-switch interaction, linked inputs, and required fields) | **PASS** (2 passed, 1 skipped) |
| `go test -v ./internal/channelhost -run TestDeliver` | Channel outbound multi-part delivery, unstarted-channel rejection, and unregistered-channel unit tests | **PASS** |
| `go test -v ./internal/runtime -run 'TestCronSettleDeliversOutboundWhenEnabled\|TestCronRecoveryPastDueRecurringJobFiresOnceOnWakeAndSkipsStorm'` | Async delivery summary on task completion; overdue recurring job wakes once and skips storms | **PASS** |
| `go test -v ./internal/rpc -run TestControlCronDeliverValidation` | RPC parameter-layer required-field validation for channel/to when `deliver: true` | **PASS** |
| `pnpm --prefix ui test` | UI i18n bilingual dictionary key alignment and frontend unit tests | **PASS** |

## Test Evidence & Details

### 1. ChannelHost.Deliver unit tests
- `internal/channelhost/deliver_test.go`:
  - `TestDeliverSplitsRunesWhenLimiterConfigured`: mocks `plugin.RunesLimiter`, verifies that overlong text is safely split and the adapter `Send` is called.
  - `TestDeliverFailsWhenChannelNotRunning`: when the channel is not running (not configured or disabled), returns `channel %s is not running`.
  - `TestDeliverFailsWhenChannelNotRegistered`: safely reports an error for an unregistered channel.

### 2. Runtime Cron scheduling and outbound-delivery unit tests
- `internal/runtime/cron_scheduler_test.go`:
  - `TestCronSettleDeliversOutboundWhenEnabled`:
    - configures a Mock `ChannelDeliverer`;
    - runs a Cron task with `deliver: true, channel: "telegram", to: "12345"`;
    - verifies that settlement calls `ChannelDeliverer.Deliver(ctx, "telegram", "12345", content)` and includes the session summary;
    - verifies that outbound delivery runs asynchronously in an independent context and does not block the scheduler's main loop.
  - `TestCronRecoveryPastDueRecurringJobFiresOnceOnWakeAndSkipsStorm`:
    - configures a recurring task to run every 10 seconds;
    - keeps the system shut down for 60 seconds (missing 6 periods in between);
    - on startup wake, the scheduler executes exactly once (`wakeRuns == 1`), and the next scheduled time correctly advances to a future period (`nextRun > nowMs`), successfully avoiding a catch-up avalanche (no cumulative storm).

### 3. RPC parameter-validation unit tests
- `internal/rpc/cron_rpc_test.go`:
  - `TestControlCronDeliverValidation`:
    - when `payload.deliver = true` is provided but `channel` or `to` is missing, asserts that `-32602 (InvalidParams)` is returned.

### 4. E2E interface interaction
- `ui/e2e/cron-tasks.spec.ts`:
  - verifies toggling the “Send result to channel” switch in the task-creation dialog;
  - verifies the expanded channel selector (dropdown options come from compiled channels) and recipient address input;
  - verifies the client-side prompt that blocks submission when required fields are empty after enabling the switch.
