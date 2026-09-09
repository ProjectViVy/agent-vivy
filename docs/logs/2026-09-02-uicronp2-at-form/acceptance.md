# Acceptance — UI-CRON-P2 (at form)

How to manually confirm this slice works (development environment `just run` + `cd ui; pnpm dev` →
http://127.0.0.1:3015/cron-tasks):

1. Cron-tasks page → New task: the schedule-mode dropdown shows a third option,
   "One-shot (at time)" (the English UI label).
2. Select "One-shot (at time)" and a "Trigger time" field (`datetime-local`) appears; enter
   a time in the past and submit, and the form reports "Trigger time must be later than the
   current time." without sending a create request.
3. Submit a future time + name + content successfully; the task's schedule mode in the list
   shows "One-shot: <local time>"; the existing scheduler triggers it at the appointed time
   (matching backend behavior, with no new backend logic).
4. Edit an existing `at` task: the schedule mode correctly displays "One-shot (at time)"
   and the trigger time is restored in the original moment's local format.
5. Creation, editing, enabling/disabling, and deletion of existing cron-expression and fixed-
   interval tasks is unchanged (`ui/e2e/cron-tasks.spec.ts` main-spec regression).
