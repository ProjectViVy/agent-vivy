# Acceptance (how a person can confirm it works)

Prerequisite: `just dev` (or `just run` + `cd ui; pnpm dev`), with a browser open at
`http://127.0.0.1:3015`.

1. **It is no longer a demo page**: the left navigation item "Scheduled tasks" opens
   `/cron-tasks`; the orange "Demo / local simulation" banner is absent; the subtitle
   is "Create a scheduled task and automatically run it once in its dedicated session
   when due."
2. **Creation schedules immediately**: create a task (for example, name it "Daily
   report", use Cron expression `0 9 * * *`, and enter any task content). After saving,
   "Next run" in the details shows the concrete time calculated by the backend, not
   "—". Change the system timezone or expression and save again; "Next run" changes
   accordingly.
3. **The Run now loop is complete**: click "Run now"; the button briefly changes to
   "Running", then the task badge and "Last status" become "Completed", and "Last run"
   updates to a time within the current minute. If the model API key is wrong or
   cleared, the status becomes "Failed" and "Recent error" shows the specific reason.
4. **The dedicated session is traceable**: click "View session" to open the chat page;
   the session title is "Cron: task name", and it contains the task message as a user
   message and the agent reply. Each triggered run accumulates here without interfering
   with regular chat.
5. **It runs automatically when due**: create and enable a task with a "Fixed interval
   0.25 hours" (or temporarily change the expression to every minute, `* * * * *`),
   wait one cycle without clicking anything, and "Last run / Last status" update
   automatically. Disabling the toggle stops automatic triggers; re-enabling restores
   them.
6. **One-off tasks clean themselves up**: through the API (`cron/create`,
   `schedule.kind=at` + `delete_after_run=true`), create a one-off task a few seconds
   in the future; after it succeeds, the task disappears from the list.
7. **Persistence**: refresh the page or restart `vivy.exe`; the task and run state are
   still present (SQLite). Starting a second process is rejected by the organism lease,
   so no double trigger occurs.
8. **Configuration toggle**: set `runtime.cron.enabled: false` in `config.yaml` and
   restart; the task list remains manageable, but no automatic trigger occurs.

Regression evidence: `ui/e2e/cron-tasks.spec.ts` automatically covers the core paths in
steps 1/2/3/4/7; smoke screenshots (kept beside this directory on the verification
machine in `vivy-cron-smoke/`) show the actual details page and task-session rendering.
