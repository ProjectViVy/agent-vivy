# Cron closed loop (UI CRON panel connected to the real scheduler backend)

Date: 2026-08-30 — branch: `feat/cron-closed-loop` (worktree `../agent-vivy-cron`) —
reference: diva implementation (`ui/agent-diva-source/agent-diva-core/src/cron/`)

## What was delivered

The `/cron-tasks` panel was upgraded from a localStorage demo page
(`vivy.demo.cron-jobs` + DemoBanner) to a real closed loop:
the UI creates/edits/deletes/enables/disables tasks → the Go armed-timer scheduler
triggers them when due → the task message drives one agent run in its dedicated session
→ the terminal run state writes back `lastRun/lastStatus/lastError/nextRun` → the panel
shows live data with a light 5-second poll and can jump to the task session (watch an
active run through `background/attach`, and view history through `selectSession`).

Scheduler semantics align with diva:

- Three schedules: `at` (one-off timestamp) / `every` (interval) / `cron` (5-field
  expression + tz; a 6-field expression accepts only second position 0, equivalent to
  diva's `0 <5-field expression>` normalization).
- The armed timer sleeps until the nearest trigger (30s fallback recomputation);
  **missed triggers are not caught up**; restart recomputes next, and expired `at`
  tasks are disabled automatically.
- Only one run per task is allowed at a time (duplicate triggers / second scheduling are
  skipped; the RPC layer maps this to -32009 Conflict).
- An `at` task is disabled automatically after completion; if
  `deleteAfterRun=true` and it succeeds, the task row is deleted.
- Manual triggers (`cron/trigger`) do not require enabled; scheduled fires require it.
- Terminal-state write-back: a watcher polls run rows (the bus terminal publication
  only serves subscription channels and does not deliver events, so it is not subscribed
  to); `run.completed→ok`, `run.failed→error+journal failure message`,
  `run.cancelled→error`.
- The cron expression parser is implemented in pure Go (`internal/runtime/cronexpr.go`;
  the sandbox has no network access and robfig is not in the module cache);
  `import _ "time/tzdata"` embeds the IANA database for Windows support.

## Component inventory

- **domain**: `internal/domain/cron.go` (CronJob/Schedule/Payload/State; JSON tags
  serve the shared `schedule_json`/`payload_json` columns for both backends).
- **storage**: mount the `CronStore` contract into `Engine`; SQLite `migration016`
  (`cron_jobs` table + enabled/next_run indexes) + `sqlite/crons.go`; Postgres bootstrap
  upgrades to `schemaV15` (version constant 15) + equivalent `postgres/crons.go`.
- **runtime**: `cronexpr.go` parse/Next/validation; `cron_scheduler.go` scheduler
  (`StartCronScheduler/StopCronScheduler/KickCronScheduler` +
  `CronRunner{TriggerCron,StopCron,ActiveCronRun}`); inject `ServiceDeps.Crons`, safe
  within one organism lease.
- **rpc**: six methods `cron/list|create|update|delete|trigger|stop` + `cron.*`
  capabilities; wire DTO fields align one-for-one with UI `CronJobDto` (diva's mixed
  casing convention), with Vivy extension field `sessionId`; error mappings
  404/-32009/-32602.
- **app**: `runtime.cron.enabled` toggle (default true); `App.Run` starts the scheduler;
  shutdown drains after `StopInteractionSweeper` and before `CancelAll` (watcher bounded
  at 3s, timeout tolerated).
- **ui**: `api.ts` adds six methods and Cron types (types moved out of the types.ts demo
  area); `CronTaskManagementView` uses real RPC, toggles update the whole object, demo
  payload-kind selection removed, 5-second polling, View session/View run buttons, and
  recent-error display; route removes DemoBanner; zh/en i18n updated together and
  `demo.cron.*` deleted; the cron mock block is removed from `demo-api.ts`.
- **tests**: parser table tests (including timezone/unreachable expressions), scheduler
  behavior tests (trigger write-back / at disable / delete / recovery / conflict /
  cancel), SQLite CRUD, RPC wire + validation + 404/409, config defaults, and
  `ui/e2e/cron-tasks.spec.ts` (full real-backend flow).

## Explicitly not done (outside this iteration)

- Real outbound semantics for `deliver/channel/to` (storage only; Vivy has no outbound
  channel).
- Catch-up for missed schedules; cross-process/distributed scheduling (the single
  organism lease already guarantees one instance).
- A form entry point for the `at` type (backend/API support it; the UI form currently
  exposes only cron/every).
- A separate approval policy for cron runs (reuse the session's default Smart preset;
  HITL todos remain unchanged).

These remain for `docs/TODO.md` §0.1 `UI-CRON-P2`.
