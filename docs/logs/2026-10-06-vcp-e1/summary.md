# VCP-E1 — shell/fs tool upgrades

Story: `docs/superpowers/plans/vivy-code-parity/E1-shell-tool-upgrade.md`
Scope: env injection, `shell_command_prefix`, `!!` no-context variant,
oversized output spill-to-file. T1 internal edits only.

## What landed

- **`VIVY_*` env injection.** `CommandBackend.resolveCommandContext` appends
  `VIVY_SESSION_ID`, `VIVY_RUN_ID`, `VIVY_PROVIDER`, `VIVY_MODEL`,
  `VIVY_THINKING` after `safeCommandEnv` sanitization, flattened to one line
  and skipped when empty. Values come from the run context: session/run ids
  and the provider/model labels bound at run admission (`service.go`),
  `shellContext`, and orchestration resume via the new
  `domain.WithRunLabels`/`RunLabelsFromContext` (`internal/domain/runlabels.go`).
  No secrets are ever injected.

- **`shell_command_prefix`.** New `runtime.shell_command_prefix` config
  (plus `runtime.tool_output_spill_bytes`) wired through
  `internal/modules/sandbox` into `CommandBackendOptions`. `bash` prepends
  it to the `-c` script after the deny-table classifier runs on the caller
  script alone; `commandline` (`ApplyShellPrefix`) wraps argv as
  `bash -c '<prefix> "$@"' vivy-shell-prefix <cmd> <args...>` so argv
  survives verbatim; the plain `execute` path stays unprefixed. The
  embedded-shell fallback (Windows/no-bash hosts, direct shell) recognizes
  the `vivy-shell-prefix` marker and flattens the wrap back into one
  single-quoted script via `shellQuoteJoin`. The prefix is trusted operator
  config; it must end with a shell separator.

- **Output spill-to-file.** `internal/tools/spill.go` adds `spillWriter`, a
  lazy tee that keeps the head in memory until a stream exceeds the
  threshold (default = 64 KiB inline bound, overridable via
  `tool_output_spill_bytes`), then opens
  `<workspace>/.vivy/tool-output/<toolCallID>.<stream>.txt` and backfills.
  `JobSpec` gained `SpillDir`/`SpillID`/`SpillBytes`; both the exec path and
  the in-process `spec.Run` path wrap stdout/stderr. `CommandResult` reports
  `StdoutSpillPath`/`StdoutTotalBytes`/`Stderr…` and marks the stream
  truncated. Files live inside the run workspace so workspace teardown
  removes them — no separate lifecycle.

- **`!!` no-context shell.** `command.Parse` now classifies `!!script` as a
  `ShellInvocation` with `NoContext` (was a `!` escape); `@` keeps its `@@`
  escape. The flag flows surface → live controller (`queuedTurn` replays it
  when busy) → `client.startShell` → `shell/start` param `no_context` →
  `Service.RunShell(..., ShellRunOptions{NoContext})` → `run.started`
  payload `no_context`. `projectedMessages` returns nil for runs flagged
  `no_context`, so the tool call/result pair stays in the transcript and
  journal but never enters the model feed. `!` is unchanged. GUI is out of
  scope for the shell surface (plan T1).

## Notes

- `payloadRunStarted` gained `NoContext` (`omitempty`) — legacy journals
  decode to `false`, ordinary runs are unaffected.
- `RunShell` gained a variadic `ShellRunOptions`; all existing callers and
  tests compile unchanged.
- `boundedCommandOutput` is gone — the job-registry tail buffer plus the
  spill writer own bound + full-output duty now.
- Fixed a config test broken since D1 (`CompactionConfig` gained a map and
  became non-comparable; now `reflect.DeepEqual`).

## Files

- `internal/tools/spill.go` (new), `internal/tools/jobs.go`,
  `internal/tools/command.go`
- `internal/runtime/command_backend.go` (commandScope/commandValidated,
  appendRunLabels, prefix wrap, spill plumbing),
  `internal/runtime/{service,orchestration,shell,payloads,message_projector}.go`
- `internal/domain/runlabels.go` (new)
- `internal/config/config.go`, `internal/modules/sandbox/module.go`
- `internal/rpc/control.go` (`no_context` param)
- `sdk/tui/{command,surface,view,live}` (`!!` parse + flag plumbing)
- tests: `internal/tools/spill_test.go`,
  `internal/runtime/command_backend_e1_test.go` (new),
  `internal/runtime/{shell_test,command_backend_test}.go`,
  `internal/rpc/control_test.go`, `sdk/tui/{command,live,view}_test.go`,
  `internal/config/config_test.go`
- `sdk/internal/assembly/conformance_results.json` re-pinned
  (`53e27d20…45e`)
