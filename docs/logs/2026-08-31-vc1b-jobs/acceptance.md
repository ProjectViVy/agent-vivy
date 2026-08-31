# Acceptance — VC-1b background jobs

How a human can tell it worked:

1. Start the split pair (`just dev`) and open http://127.0.0.1:3015.
2. Ask the model to run something long in the background, e.g. "start a
   background bash job that sleeps 30 seconds then prints done" — the
   model calls `bash` with `run_in_background: true` and immediately
   gets a job id instead of the run hanging for 30 seconds.
3. Ask it to check on the job — `job_output` returns any new output plus
   running/completed status without re-running anything.
4. Start any long foreground command (e.g. a slow build with
   `timeout_ms` set low): when the budget is exceeded the tool result
   says the command moved to a background job and gives the job id —
   work is not lost on timeout.
5. Ask it to kill the job — `job_kill` stops it; the run journal shows
   the killed status.
6. When the run ends (final reply), any still-running background jobs
   are reaped: no orphan processes survive the conversation turn.

Rollback: revert the single VC-1b commit; `bash` loses the
`run_in_background` parameter and `job_output`/`job_kill` leave the
default enabled surface with it.
