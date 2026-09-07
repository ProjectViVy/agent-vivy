# Verification

Date: 2026-09-08

## Targeted checks

- `go test ./internal/runtime ./internal/rpc -count=1 -run '<wave-1 tests>'`
  — PASS; covers projected count, dynamic auth state, bounded handshake error,
  recovery, retired-entry isolation, RPC state priority, nullable count, and
  catalog-only fallback.
- `go test ./sdk/tui/live ./sdk/tui/view -count=1` — PASS.
- `go test -race ./internal/runtime -count=1 -run '<MCP concurrency tests>'`
  — PASS; covers concurrent initialize, failed-session replacement, and
  retired status writes.
- `gofmt -l <changed Go files>` — no output.
- `git diff --check` — PASS. Git emitted only the existing Windows
  LF-to-CRLF working-copy notice for `docs/TODO.md`.

## Product gate

- `just ci` — PASS, exit 0: fmt-check, UI typecheck, 25 UI test files / 210
  tests, production UI build, Go vet, full `go test ./...`, headless compile,
  and every plugin/face vet+test slice completed successfully.
- `just vivy-code` — PASS; the headless-tagged `vivy-code.exe` built.
- `vivy-code.exe --help` — PASS, exit 0; the independent terminal product
  printed its expected private-instance contract.

## Real path

- A native PTY launch was attempted with isolated
  `VIVY_USER_HOME=.workspace/tui-sidebar-n1-smoke-home`, but the execution
  host failed before process startup with `CreateProcessW` OS error
  `-1073283067` (`FormatMessageW` error 317). No interactive smoke is claimed.
  The backend-to-RPC mapping, nullable wire semantics, shared live projection,
  rendered status/details, unsafe-text filtering, and width bounds are covered
  by the passing in-process tests above.
