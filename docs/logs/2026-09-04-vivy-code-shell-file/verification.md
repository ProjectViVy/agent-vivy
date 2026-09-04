# Verification

Commands run from the shell/file worktree before the final product gate:

- `C:\Program Files\Go\bin\gofmt.exe -w` over all changed Go files — passed.
- Targeted RPC/runtime/storage/shared-view/internal/packed-face tests for
  project context, security, persistence, accounting, request digests, and
  retry restoration — passed.
- `git diff --check` — passed.

- `go test ./internal/rpc ./internal/runtime ./internal/storage/sqlite
  ./internal/storage/postgres ./sdk/tui/... ./internal/tui -count=1
  -timeout=300s` — passed.
- `go test -race` across the changed RPC/runtime/storage/shared/internal TUI
  cases — passed.
- `go test ./... -count=1 -timeout=300s` from `faces/tui` — passed.
- `go run ./sdk verify faces/tui` — passed.
- `go run ./sdk pack --face tui --out
  .workspace/tui-file-context-pack-20260904-v1` — passed; generation
  `gen_9d9a61841603f3ce`, SHA-256
  `f2cbd69e530246138730213c49e344c59af6a992beb192074f8511e0f068dc7b`.
- `just ci` — passed on the final unchanged implementation: UI typecheck,
  197 tests and build; Go formatting, vet, complete tests, headless compile,
  and every plugin/face gate. The preceding attempt hit the known flaky
  `TestCronAtJobDeletesAfterSuccessfulRun`; its immediate isolated rerun
  passed, followed by the complete green gate.
- `vivy-code` real PTY smoke at 80x24 — passed. `inspect @README.md` rendered
  `[file: README.md]`, reached the real turn path, and then displayed the
  expected missing-provider error. `!echo SHELL_SHOULD_NOT_RUN` returned
  `rpc error -32601: method not found: shell/start`; no local shell ran and
  Ctrl+C exited cleanly.
- Two GPT-5.6-LUNA MAX read-only audits (file/TUI surface and
  storage/security/compaction contracts) — PASS after the repair rounds; no
  remaining P1/P2.

No live provider/network call, production tenant Journal, or Studio state was
used or read.
