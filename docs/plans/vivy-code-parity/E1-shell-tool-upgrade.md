# E1 — Protected shell/fs tool upgrades

**Goal:** env injection, `shell_command_prefix`, `!!` no-context variant, oversized output spill-to-file. T1 internal edits only.
**Epic:** E. **Requirements:** RQ-TOOL.
**Spec:** [VCP-D1](../../specs/2026-10-06-vivy-code-parity-design.md) §5.6. **Baseline:** `f34f3ce`.

## Scope

**Files:** `internal/tools/bash.go`, `commandline`, `execute`, `internal/config/config.go` (`shell_command_prefix`, `tool_output_spill_dir`/threshold fields), `internal/runtime` (env values source: session/provider/model/thinking labels at admission), `sdk/tui` (`!!` prefix handling in composer → passes `include_in_context:false` flag on the turn/execute path — decide exact wire shape: likely a `!`/`!!` parse in the face calling different submission).

## Tasks

- [ ] Shell tools prepend `shell_command_prefix` (config `runtime.shell_command_prefix`, e.g. `set -euo pipefail &&` or nix-shell enter) to every `bash`/`commandline` execution; `execute` exempt unless configured.
- [ ] Inject `VIVY_SESSION_ID`, `VIVY_PROVIDER`, `VIVY_MODEL`, `VIVY_THINKING`, `VIVY_RUN_ID` into spawned-process env (after env sanitization; never inject secrets).
- [ ] Output spill: when tool output exceeds bound (default ~64KiB), write full output to `<instance>/tool-output/<tool_call_id>.txt`, return tail excerpt + path + byte count.
- [ ] `!!` prefix: face-side parse → submission flagged `no_context` → runs the shell command, result lands in transcript but is NOT appended to model context (Journal event marks it). `!` (single) keeps today's behavior.
- [ ] Tests: env vars visible in child (`bash -c 'echo $VIVY_SESSION_ID'`), prefix applied, spill file written + tail truncation, `!!` result excluded from next model request but present in transcript, denylist unaffected.
- [ ] `go test ./internal/tools ./internal/runtime -run 'Bash|Execute|Command|Spill'`; `just ci`.
- [ ] Commit `feat(tools): env injection, shell prefix, no-context shell, output spill`.

## Boundary

`!!` semantics: the command output goes to transcript only — equivalent to pi's `!!` "without context". No new tools; exposure model is E2.

## Acceptance

All four behaviors observable in one TUI session; spill files cleaned with instance teardown; `commandpolicy` still gates everything.
