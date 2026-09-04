# Verification

Commands are run from the isolated `feat/tui-stream-continuity` worktree.

- `go test ./internal/runtime ./internal/rpc ./sdk/tui/view -count=1
  -timeout=240s` — runtime and RPC passed; the first view run exposed two
  incorrect minimum-width expectations in the new fixture, which were fixed.
- `go test -race ./internal/runtime -run
  'TestMapper(Publishes|Splits|Reasoning)' -count=1 -timeout=120s` — passed.
- `go test -race ./internal/rpc -run
  'TestPeer(NotifyContext|RejectsOverloaded)' -count=1 -timeout=120s` — passed.
- `go test ./sdk/tui/view -count=1` — passed after word-boundary,
  ANSI/control, trailing-newline, CJK, whitespace, and emoji coverage.
- `go test ./internal/runtime -count=1` — the first full run hit the existing
  timing-sensitive `TestCronAtJobDeletesAfterSuccessfulRun`; its isolated
  rerun passed, and the final `just ci` runtime run also passed.
- A pre-review `just ci` passed. After the race fixes, the first rerun caught
  the real `tool.finished`/next-delta ordering regression; the tool barrier
  fixed it. Two current-tree reruns then passed formatting, 201 UI tests, UI
  production build, Go vet, RPC, and every reported package except the
  unrelated load-sensitive `TestCronAtJobDisablesAfterRun`, which timed out at
  its fixed 5s poll budget with the job already disabled but settlement still
  pending. Its isolated rerun passed in 1.275s. The newly exposed sibling flake
  is tracked as `TFLAKE-CRON-AT-DISABLE` in `docs/TODO.md` §0.1.
- `go test ./... -skip '^TestCronAtJobDisablesAfterRun$'` — passed on the
  final tree, including runtime/RPC/storage and all other packages.
- `just headless-compile` and `just plugin-ci` — passed on the final tree for
  headless binaries, every plugin module, and both face modules.
- `go test -race ./internal/runtime -run
  'TestService(ApprovalApproveFlow|ApprovalResumePersistsChunkBeforeProviderEOF|PersistsFirstModelChunkBeforeProviderEOF)'
  -count=10 -timeout=240s` — passed; covers exactly-once raw chunks, resumed
  pre-EOF durability, and tool-result causal ordering.
- `go test -race ./internal/rpc -run
  'TestRunSubscription(ResponseCloseCleansPendingEntry|ContextCancelCleansPendingEntry|StopsWhenPeerClosesWhileIdle)'
  -count=20 -timeout=180s` — passed; covers all subscription shutdown windows.
- `go run ./sdk verify faces/tui` — passed.
- `go run ./sdk pack --face tui --out
  .workspace/tui-live-stream-pack-20260904` — passed; generation
  `gen_5421d6c234d802c1`, artifact SHA-256
  `378796e37e742c944d29593b5469fd0f7ae0a49c40647dc49d403ef7f23ba320`.
- Built `.workspace/vivy-code-stream-smoke.exe`, launched it in a PTY with
  `VIVY_USER_HOME=.workspace/stream-smoke-home`, observed the VIVY CODE local
  project screen, and exited cleanly with Ctrl+C. No provider was configured;
  the real Service + SQLite test
  `TestServicePersistsFirstModelChunkBeforeProviderEOF` covers pre-EOF
  durability with a gated provider stream.
- GPT-5.6-LUNA MAX specialists audited the runtime/RPC event chain, shared TUI
  reducer/renderer, and local Crush source. Final renderer and event-chain
  re-reviews both returned PASS with no remaining P1/P2.

No live network, provider credential, Studio state, or tenant Journal was
accessed.
