# Verification

## Completed

- `go test -race ./sdk/tui/stream ./internal/tui -count=3 -timeout=180s` — PASS during implementation; focused recovery/overflow cases also passed ten consecutive race runs.
- `cd faces/tui; go test -race ./... -count=3 -timeout=180s` — PASS during implementation.
- First `just ci` — UI install/typecheck (201 tests)/build, vet, and all TUI packages passed. The full Go suite failed only at unrelated `TestCronRecoveryPastDueRecurringJobFiresOnceOnWakeAndSkipsStorm`: the job had already completed with `LastStatus=ok`, while its 200 ms next-run state advanced beyond the test's short wanted window. Captured as `TFLAKE-CRON-RECOVERY` in `docs/TODO.md` §0.1.

## Final gate and smoke

- A second `just ci` again passed formatting, UI typecheck/201 tests/build, vet, all changed TUI/RPC packages, and every reported package except the unrelated `TestCronAtJobDeletesAfterSuccessfulRun` wall-clock canary. Its settled row was disabled but had not yet been deleted after 30 seconds. A current-tree isolated rerun passed in 1.142s. The recurrence is captured as `TFLAKE-CRON-DELETE-RECURRENCE`.
- The already tracked `TestCronAtJobDisablesAfterRun` also reproduced under a five-count isolation batch, while its state was already disabled; it remains `TFLAKE-CRON-AT-DISABLE`.
- Final current-tree equivalent gate: `go vet ./...`; `go test ./... -skip '^(TestCronAtJobDeletesAfterSuccessfulRun|TestCronAtJobDisablesAfterRun|TestCronRecoveryPastDueRecurringJobFiresOnceOnWakeAndSkipsStorm)$'`; `just headless-compile`; `just plugin-ci` — PASS.
- Final changed-package race gate: `go test -race ./internal/rpc ./sdk/tui/stream ./internal/tui -count=2 -timeout=180s`, then `cd faces/tui; go test -race ./... -count=2 -timeout=180s` — PASS. The hung-subscribe, overflow, and recovery tests are included.
- `go test ./internal/runtime -run '^TestCronRecoveryPastDueRecurringJobFiresOnceOnWakeAndSkipsStorm$' -count=10` — PASS. A combined five-count cron canary run reproduced only the existing delete/disable timing family; no TUI package failed.
- `go run ./sdk verify faces/tui` — PASS.
- `go run ./sdk pack --face tui --out .workspace/tui-stream-driver-pack-20260904` — PASS; generation `gen_12e28f071c976e6e`, artifact SHA-256 `5c119702a9388eaecd570753ac80a91c85206d2a879b97862bef4bdd5fc1bb6d`.
- Real PTY smoke at 80×24 with `VIVY_USER_HOME=.workspace/stream-driver-smoke-home` — PASS. The independent `vivy-code` screen opened, `/help` displayed the real shared command overlay, and Ctrl+C exited cleanly.
- Three GPT-5.6-LUNA MAX specialists re-audited REPL convergence, subscription lifecycle, and bounded rendering. Their raised P1/P2 findings were fixed and re-reviewed; all three final audits reported no remaining P1/P2. The remaining `model.completed.content` projection work stays explicitly tracked as `TUI-STREAM-N3`.

No provider credential, Studio state, production tenant Journal, `data/vivy.db`, `data/demo`, or `data/workspaces` was read or written.
