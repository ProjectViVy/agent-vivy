# Acceptance — S11-E cutover (gates G1–G8)

- **G1 sole engine**: `StartINOFYWorkflow` is the only workflow execution
  route; `compose.NewWorkflow` is production-unreachable (source audit in
  verification.md). No translation, selector, or cross-engine resume exists.
- **G2 governed nodes**: every `vivy.child-task@1` node executes as one
  deterministic `workflow_child_*` child run under VIVY authority — verified
  in runtime tests and the live smoke (2 children completed, `child_run_id`s
  inspectable).
- **G3 durable outcomes**: terminal state is written only by INOFY commits;
  host code has no terminal emitter. A run that cannot decide durably settles
  as `recovery_required`, never a fabricated status.
- **G4 restart/recovery**: running→`recovery_required` classification,
  admitted→idempotent resume, terminal/waiting settled — all proven in
  focused tests; authority re-verified (policy hash, sandbox, tool ceiling,
  program/host binding digests) before any resume.
- **G5 legacy exclusion**: schema-1 rows untouched by recovery, rejected by
  `workflow/get`, absent from `workflow/list`; evidence rows preserved.
- **G6 contract boundary**: RPC/UI surface `definition`/`engine_status`/
  `nodes`/`outputs`; legacy `{descriptor:…}` fails `-32602` at the edge.
- **G7 inspection**: single inspector backed by the committed projection +
  journal replay; outputs resolved from committed result blobs through
  declared bindings; unresolved ops surface as `blocked`.
- **G8 cancellation**: cancel propagates to children and yields the engine's
  honest `recovery_required`; the app worker exits polling instead of
  hanging or inventing a cancelled terminal.

Skipped: none. PostgreSQL suite ran against a live `VIVY_POSTGRES_TEST_DSN`.
