# Execute Timeout Ceiling Summary

Date: 2026-08-26
Status: complete
Branch: `feat/execute-timeout` (worktree `../agent-vivy-execute-timeout`)

## Outcome

The execute/commandline ceiling is no longer a hardcoded 30s constant.
`runtime.execute_max_timeout_seconds` (default 30) now bounds one local
process run, so real work such as `go test ./...` or `git clone` can finish
when the operator raises it. An unconfigurable hard cap of 10 minutes stays
in the runtime so a single execute call can never hang a run for hours.

## Delivered

- `internal/runtime/command_backend.go`: `NewEinoCommandBackend` takes a
  `maxTimeout` argument. Non-positive falls back to the 30s default; values
  above `hardMaxCommandTimeout` (10m) are clamped. Request timeouts above
  the effective ceiling are clamped as before.
- `internal/config`: new `runtime.execute_max_timeout_seconds` field,
  default 30, validated to 1–600 (600 mirrors the runtime hard cap; a value
  above it would be silently clamped, so config rejects it up front with an
  actionable error).
- `internal/app/app.go` passes the configured ceiling into the backend.
- `config.example.yaml`: documents the new field (range, hard cap, why to
  raise it) and extends the `execute_allowed_commands` comment with per-project
  extension examples (node/pnpm, python/uv, just).
- Tests: table-driven ceiling coverage in `command_backend_test.go`
  (request clamped by ceiling, default fallback, hard-cap clamp) and
  parse/validate tests in `config_test.go` (parse, omitted-keeps-default,
  zero / negative / above-cap rejected, default asserted).

## Explicitly not done

- No mock-provider scenario that drives an execute tool call end to end;
  recorded as TEST-1 in `docs/TODO.md` §0.1. The real-path smoke therefore
  covers startup config acceptance/rejection on the real binary, and the
  clamping matrix is covered at the backend unit level.
- Per-command timeout overrides (e.g. long allowlist entries with their own
  ceilings) — one global ceiling is enough for the current pain.
- No UI surface for the knob; it is an operator config field.
