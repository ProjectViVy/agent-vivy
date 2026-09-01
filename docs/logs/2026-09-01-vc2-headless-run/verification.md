# Verification — VC-2 headless `vivy run`

All commands run in the worktree `agent-vivy-vc0` on `feat/vc1a-bash-tool`.

## Gate

- `just ci` — PASS (gofmt check, `go vet ./...`, full Go test suite,
  UI vitest 24 files / 195 tests, UI vite build).
- Intermediate: `go build ./...` PASS; `gofmt -l` on touched packages clean;
  `go vet ./internal/app/` clean after test-file fixes.

## Package tests (new)

- `go test ./internal/app/` — PASS. New tests:
  - `TestHeadlessSinkRendersStream` — delta/completed dedupe, tool notices,
    run-failed payload decode.
  - `TestHeadlessTurnCompletesWithScriptedModel` — completed turn: stdout
    carries assistant text, journal run.started decodes `face=headless`,
    user message effective source `headless`.
  - `TestHeadlessTurnApprovalFailsLoudly` — approval block: loud stderr,
    run cancelled, stdout empty, no lingering pending approval, exactly one
    terminal, approval_required event journalled.
  - `TestHeadlessTurnQuestionFailsLoudly` — question block: same shape,
    question_required journalled.
  - `TestResolveHeadlessSession` — continue-newest, empty-board error,
    fresh-session title truncation (60 runes + `...`).

## Real-path smoke (`vivy run` CLI)

Built `go build -o /tmp/vivy-smoke.exe ./cmd/vivy`, ran against a throwaway
`VIVY_USER_HOME=/tmp/vivy-hl-home` (production data untouched):

- `vivy run --help` — usage on stdout, exit 0.
- `vivy run` (no prompt, non-interactive stdin) — usage on stdout, exit 1.
- `vivy run --bogus-flag` — `vivy run: unknown flag --bogus-flag` on
  stderr + usage, exit 1.
- `vivy run "say hi"` with an empty home (no provider configured) —
  stderr: `vivy: run failed (provider_error): ...`, exit 1; **stdout stayed
  empty** (the config-defaults WARN also went to stderr via the bootstrap
  logger). This exercises the full real path: session mint, run started,
  engine drive, provider failure, terminal mapping — everything short of a
  live model completion.

## Smoke exception (recorded)

The completed-turn path (`vivy run` against a real provider returning
assistant text, exit 0) was not exercised: no provider key exists in this
environment. Coverage for that path is the scripted-model package test
`TestHeadlessTurnCompletesWithScriptedModel` (same composition, same sink,
same terminal mapping). Recorded per rulebook as the reason for skipping
the live-key smoke; not a `just ci` slice skip.
