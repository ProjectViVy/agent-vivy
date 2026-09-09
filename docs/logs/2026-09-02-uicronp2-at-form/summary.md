# UI-CRON-P2 (feasible portion) — expose one-shot (`at`) scheduling in the cron form

## What changed

The cron backend fully supports one-shot `at` scheduling (`ValidateCronSchedule` only
requires a positive `atMs`, and `NextCronAfter`/scheduler already consumes it), but the UI
form exposed only cron expressions and fixed intervals; `CronTaskManagementView`'s form type
even explicitly used `Exclude<ScheduleKind, 'at'>`. This slice wires `at` into the form so all
three schedule types can be created/edited in the UI:

- `ui/src/components/cron/CronTaskManagementView.tsx`:
  - Widen form state `scheduleKind` to the complete `ScheduleKind` and add `atValue`
    (a datetime-local string); `openEdit` restores `job.schedule.kind` as-is, with `atMs`
    converted to local `toDatetimeLocal` format.
  - Add "One-shot (at time)" to the "Schedule mode" dropdown; selecting it shows the
    `datetime-local` trigger-time field (`#cron-at`).
  - Submission validation: empty/invalid time → `cron.errors.atTimeRequired`; not later than
    the current time → `cron.errors.atTimeFuture` (the UI enforces a future moment; backend
    positive-`atMs` semantics are unchanged). Submit `{ kind: 'at', atMs }` through the
    existing `cron/create`|`cron/update`.
  - `formatSchedule` displays "One-shot: <local time>" for `at` tasks
    (`cron.scheduleFormat.onceAt`, replacing the old `once` copy without time information;
    the `once` key is deleted, with this as its only reference).
- `ui/src/i18n/en.ts`, `zh.ts`: bilingual (en/zh) `cron.atOption`/`atLabel`,
  `cron.errors.atTimeRequired`/`atTimeFuture`, and `cron.scheduleFormat.onceAt`.
- `ui/e2e/cron-tasks.spec.ts`: adds an offline spec — the one-shot option appears, the
  `datetime-local` field is visible after selection, and a past-time submission is blocked by
  future-time validation (no create request; no real provider dependency; the existing main
  spec remains provider-gated).

## What was explicitly not done (established in-line prerequisites; untouched)

- Catch-up for missed schedules — the text says to wait for CH-0.
- Outbound `payload.deliver/channel/to` channels — wait for CH-0.
- An independent approval-governance surface for cron runs — a separate proposal.

## Board

The "at form entry point" sub-item on the UI-CRON-P2 line in `docs/TODO.md` is closed;
the overall line remains OPEN (catch-up/outbound sub-items still wait for CH-0), and the line
note is updated.
