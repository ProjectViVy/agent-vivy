# VC-1b: background job registry (job_output/job_kill, automatic backgrounding on timeout)

## Delivery scope

The bash tool gained background execution, with two management tools:

- **`bash.run_in_background`** (boolean parameter): starts the script immediately as a background job,
  returns `job_id` + `status: running` directly in the tool result, and still sends the script through
  the deny list and tiered approval (background execution relaxes no security semantics).
- **Automatic backgrounding on timeout**: when a foreground bash call exceeds `timeout_ms`, it no longer kills the process and loses output;
  it adopts the still-running process as a background job. The result includes `timed_out: true` +
  `job_id` + partial output available so far; the model continues harvesting with `job_output`.
- **`job_output`** (readonly, auto-approved): reads **new output since the previous read** by job_id
  (the server maintains a read cursor) and reports lifecycle status
  running/completed/failed/killed, exit code, and duration. The output stream is a bounded 64KiB
  tail window; when the head is squeezed out, it reports `stdout_gap_bytes`/`stderr_gap_bytes`.
- **`job_kill`** (mutating, uses existing approval): terminates a running job; for an already-finished
  job it idempotently returns the current status; an unknown id is an error.

**Lifecycle semantics: jobs bind to the run context.** When a run ends or is canceled, its background jobs
are killed as well (the service.go terminal path releases the run context → exec.CommandContext terminates
the process); there are no cross-run orphan processes. To obtain a long-task result before the run ends,
the model must poll `job_output` to completion within the same run.

**Ownership and wiring:** `tools.JobRegistry` (internal/tools/jobs.go) owns all
process mechanics (spawn/reader/finalizer/kill). `EinoCommandBackend` implements
`tools.JobOperations` and registers `job_output`/`job_kill` (the tools are automatically registered, including in the
tool_search index, when the commands backend implements both interfaces); the default enabled surface adds the two
new tool names. At most 16 jobs may be registered concurrently (when full, terminal jobs are evicted first; if still full,
the operation fails closed); 64 terminal jobs are retained for reading.

## Explicitly not done (see docs/TODO.md)

- Jobs that survive across runs (which would require persisting job handles in the Journal) are outside this slice.
- On Windows, `Process.Kill` kills only the bash process; bash grandchildren may remain until
  natural completion (`cmd.WaitDelay=2s` ensures kernel-side cleanup does not hang); process-tree killing
  is deferred to SBX-OS.
- No independent job-output budget tuning or `job_output` pagination offset parameter (the current server-side
  cursor is sufficient for a single-consumer model).

## Behavior alignment

- Background jobs + output harvesting + kill are existing Crush behavior (aligned with the
  run_in_background / bash_output / kill_shell semantics: incremental output, status queries, and terminal-state idempotency).
- Automatic backgrounding on timeout is the wording established in research §5 VC-1 ("automatically move to background"); it is
  implemented as the default and described to the model in the tool description.
- The native eino `Shell`/`RunInBackendGround` flags are not consumed directly (the eino-reuse inventory concludes that native eino
  has flags but no job-management tools); the implementation follows "adopt behavior without adopting the dependency" with its own backend.
