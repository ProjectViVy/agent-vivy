# A1 — `vivy-code` flag parser + extended face.Options

**Goal:** `vivy-code` accepts the pi-equivalent flag surface; `face.Options` carries mode/model/session selectors; unknown args still exit 2 with usage.
**Epic:** A. **Requirements:** RQ-CLI.
**Spec:** [VCP-D1](../../specs/2026-10-06-vivy-code-parity-design.md) §5.1. **Baseline:** `f34f3ce`.

## Scope

**Files:** `cmd/vivy-code/main.go` (flag parsing, usage text), `sdk/port/face/face.go` (Options extension), `internal/codeface/launch.go` (options → face runner plumbing), `internal/config/config.go` (additive fields only if needed for persistence), `sdk/tui/face` / `faces/tui` (accept Options, dispatch `Mode`; non-text modes may return a clear "not yet implemented" until A2/A3), `faces/headless` (reuse, unchanged unless signature drift). Faces are separate Go modules — their go.mod requires stay intact.

**Interfaces:** `face.Options` extended exactly as spec §5.1. The Port contract text in `VIVY-PORT-CATALOG.md §5` gets one line noting mode dispatch is face-internal (no new Port).

## Tasks

- [ ] Write `cmd/vivy-code` flag table covering spec §5.1 fields; `-h/--help` prints full usage including modes; unknown flag → stderr usage + exit 2; `--version` prints build version var.
- [ ] Extend `face.Options`; update `codeface.Prepared`/`Run` to pass them; TUI mode = default when `Mode=="text"`.
- [ ] `Mode=="print"`/`json`/`rpc` route to `faces/tui` headless runner stubs that return a typed `ErrModeUnavailable` for now (A2/A3 fill them).
- [ ] Tests: flag parse table tests (each flag, combined forms, `--flag=value` vs `--flag value`, prompt positional capture); Options round-trip through codeface into a fake face runner.
- [ ] Run `go test ./cmd/vivy-code ./internal/codeface ./sdk/...` (+ face module tests from `faces/tui`).
- [ ] Commit `feat(vivy-code): add pi-parity CLI flags and face mode plumbing`.

## Boundary

No mode behavior beyond dispatch (A2/A3 own it). No subcommands (A5). `--api-key` sets a process-scoped override never written to config/logs. `--session-dir` is accepted and mapped to the codeface instance root mechanism.

## Acceptance

`vivy-code --help` documents the full surface; each flag parses to a typed Options value; unsupported modes fail with a named error, not a panic; `just ci` green.
