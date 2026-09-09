# UI-CRON-P2 — Scheduled-task outbound channels and catch-up closure

## What changed

After the `at` (one-time scheduling) form entry was completed on 2026-09-02, this slice fully completed the remaining “outbound-channel wiring” and “catch-up characterization closure” for `UI-CRON-P2`, bringing the scheduled-task system to full closure:

### 1. Channel host and kernel-isolated outbound delivery (`ChannelHost.Deliver` + `runtime.ChannelDeliverer`)
- **`internal/channelhost/host.go`**:
  - Added the public method `Deliver(ctx context.Context, channelName, chatID, content string) error`.
  - Strictly validates that the channel is compiled and currently started (`started`); an unstarted channel returns a safe, descriptive error.
  - Uses the `plugin.RunesLimiter` interface to detect the adapter platform's rune limit and `splitRunes` to split and deliver content intelligently.
  - The unit test `internal/channelhost/deliver_test.go` covers normal multi-part delivery and rejection of unstarted/unregistered channels and other errors.
- **`internal/runtime/service.go`**:
  - Defines the decoupled `ChannelDeliverer` interface; `internal/runtime` has zero `internal/channelhost` import statements, fully following the D-007 isolation principle.
  - Adds optional `Channels ChannelDeliverer` to `ServiceDeps`.
- **`internal/app/app.go`**:
  - The assembly layer injects `channelHost` into `ServiceDeps.Channels` when instantiating `runtime.NewService`.
- **`internal/runtime/cron_scheduler.go`**:
  - Terminal-state settlement in `settleCronRun`: when `payload.deliver === true`, a successful run extracts its latest assistant message in the dedicated session as the summary; a failed run extracts the terminal error.
  - Asynchronously calls `s.deps.Channels.Deliver` in an independent goroutine using a context with a hard `30s` timeout, never blocking scheduler settlement or the clock loop.
  - Added the deterministic assertion `TestCronSettleDeliversOutboundWhenEnabled` to `cron_scheduler_test.go`.

### 2. Catch-up characterization closure (Fire once on wake, skip storms)
- **`internal/runtime/cron_scheduler.go`**:
  - Startup recovery in `recoverCron`:
    - One-time schedule (`at`): when its time has passed, immediately marks it disabled (`Enabled = false, NextRunAtMs = 0`), preserving the existing safe semantics;
    - Recurring schedule (`every` / `cron`): when `0 < NextRunAtMs <= nowMs` (due during downtime), preserves its due state instead of jumping ahead to the future. The scheduler's main loop `fireDueCronJobs` triggers **exactly one** execution on its first wake, after which `settleCronRun` automatically advances to the future `NextCronAfter(schedule, nowMs)`, skipping all intermediate periods missed during downtime.
  - Added the deterministic unit test `TestCronRecoveryPastDueRecurringJobFiresOnceOnWakeAndSkipsStorm` to `cron_scheduler_test.go`.

### 3. Control-plane parameter validation (`internal/rpc/control.go`)
- `buildCronJob`: when `payload.Deliver === true`, requires `payload.Channel` and `payload.To` to be non-empty; otherwise returns `-32602 (InvalidParams)`.
- Added `TestControlCronDeliverValidation` to `cron_rpc_test.go`.

### 4. Frontend form and channel dropdown (`ui/`)
- **`ui/src/components/cron/CronTaskManagementView.tsx`**:
  - Automatically retrieves the list of compiled channels through `inspectChannels()` at startup.
  - Adds a “Send result to channel” switch to the create/edit dialog; when enabled, it expands a channel dropdown (`Select`, showing compiled platform names) and a recipient target input (`Input`).
  - Fully repopulates the `deliver`, `channel`, and `to` states when editing an existing task.
  - Adds “Result delivery” information to the details panel (for example, `Telegram · Target: 12345` or `Not enabled`).
  - Client-side validation: when outbound delivery is enabled, a channel must be selected and a recipient target entered.
- **`ui/src/i18n/zh.ts` & `en.ts`**:
  - Added bilingual entries while keeping dictionary keys strictly aligned (verified by `i18n.test.ts`).
- **`ui/e2e/cron-tasks.spec.ts`**:
  - Added an offline Playwright E2E specification asserting outbound-switch interaction, input visibility, and required-field validation.

---

## What was explicitly not done

- Did not introduce a complicated distributed queue or backlog catch-up storm mechanism (the architectural characterization explicitly adopts `fire once on wake, skip storms`).
- Did not break the channel quarantine wall (`internal/runtime` retains zero channelhost/plugin/eino dependencies).
