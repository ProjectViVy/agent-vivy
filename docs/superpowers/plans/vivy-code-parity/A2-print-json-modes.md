# A2 — `--mode print` and `--mode json`

**Goal:** non-interactive headless output modes inside the `vivy/tui` face module.
**Epic:** A. **Requirements:** RQ-CLI, RQ-JSON. **Predecessor:** A1 (Options.Mode + runner seam).
**Spec:** [VCP-D1](../../specs/2026-10-06-vivy-code-parity-design.md) §5.1. Pattern: `faces/headless` already proves FaceEnv-driven non-interactive runs.

## Scope

**Files:** `faces/tui` (or `sdk/tui/face` wherever the runner lives) — new `headless_runner.go` + `json_events.go`; `cmd/vivy-code` wiring already lands via A1.

## Tasks

- [ ] `print`: prompt → `turn/start` → stream assistant text deltas to `Out`; tool/diagnostic noise to `Err`; exit code = run status (0 completed, 1 failed/blocked — blocked approvals print a clear cause, never wait forever).
- [ ] `json`: emit JSONL per spec RQ-JSON record names — `session`, `agent_start`, `turn_start`, `message_start/update/end`, `tool_execution_start/update/end`, `turn_end`, `agent_end`, `compaction_start/end`, `auto_retry_*`, `error`, `agent_settled`. Map from Journal/run events via `Host.OnEvent`; record schema versioned (`"v":1`).
- [ ] Session flags honored (`--continue/--resume/--session/--fork/--no-session/--session-dir`, A1).
- [ ] Tests: event-name mapping table coverage; print exit codes incl. approval-block; json output parses line-by-line and carries run correlation ids; `--no-session` leaves no Journal session.
- [ ] `go test ./faces/tui` + a packed-generation smoke driving `--mode json` end-to-end on a fake provider.
- [ ] Commit `feat(vivy-code): print and json headless modes`.

## Boundary

`rpc` mode is A3. `--export` flag (A1) may route to C1 export once it lands; until then it errors clearly.

## Acceptance

`vivy-code --mode json -p "hi"` produces the full ordered event sequence on a real generation; `print` returns assistant text only on stdout.
