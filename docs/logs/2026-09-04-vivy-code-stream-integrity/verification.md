# Verification

Commands were run from the repository root unless noted otherwise.

- `go test ./internal/runtime -run 'TestMapper(EmitsReasoning|ReasoningOnlyChunksDoNotEmitEmptyDeltas|EmitsStall)' -count=1 -timeout=60s` — passed.
- `go test ./internal/tui/... -count=1` — passed.
- `cd faces/tui; go test ./... -count=1` — passed.
- `go test -race ./internal/tui -run 'TestLive(KeepsReasoning|EventInbox|DetectsSequence|TurnStreams)' -count=1 -timeout=90s` — passed.
- `cd faces/tui; go test -race . -run 'TestLive(KeepsReasoning|EventInbox|EventQueue|DetectsSequence|TurnStreams)' -count=1 -timeout=90s` — passed.
- `git diff --check` — passed before the final gate.
- `just ci` — first run reached the full Go suite but failed the unrelated
  timing-sensitive `TestCronAtJobDeletesAfterSuccessfulRun`; the same test
  passed immediately in isolation. A complete second `just ci` run passed,
  including UI typecheck/197 tests/build, Go vet/tests, headless builds, and
  every plugin/face module.
- After the final review-driven retry-race fixes, `just ci` was run again and
  passed completely; this is the final gate for the committed code.
- A GPT-5.6-LUNA MAX read-only reviewer rechecked the retained-gap and delayed
  initial-subscription races after their regression tests landed and reported
  no remaining blocker.

No production or tenant data path was accessed.
