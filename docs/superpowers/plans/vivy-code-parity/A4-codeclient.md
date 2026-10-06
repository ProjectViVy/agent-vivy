# A4 — `sdk/codeclient` Go embedding client

**Goal:** Go package that embeds a `vivy-code --mode rpc` child process: typed commands + event subscription.
**Epic:** A. **Requirements:** RQ-SDK. **Predecessor:** A3 (the wire contract it speaks).
**Spec:** [VCP-D1](../../specs/2026-10-06-vivy-code-parity-design.md) §5.1.

## Scope

**Files:** `sdk/codeclient/` — `client.go` (spawn, pipe management), `commands.go` (typed methods), `events.go` (subscription fan-out), `client_test.go` (against a fake rpc-mode binary script).

## Tasks

- [ ] `New(Config{Binary, Args, Dir}) (*Client)` → spawns, waits for `session` event.
- [ ] Methods mirroring A3 surface: `Prompt, Steer, FollowUp, Abort, ClearQueue, NewSession, SwitchSession, SetSessionName, GetState, GetMessages, GetSessionStats, SetModel, CycleModel, Models, SetThinkingLevel, ThinkingLevels, SetQueueModes, Compact, SetAutoCompaction, SetAutoRetry, Fork, Clone, Tree, Entries, Export, LastAssistantText, Commands, Close`.
- [ ] `Subscribe(func(Event))` — ordered events until Close; auto-read drains stdout (bounded buffer, drop counter if consumer stalls).
- [ ] Id correlation; `Close` sends shutdown + kills child on timeout.
- [ ] Tests: fake rpc binary (shell script emitting canned lines) verifies correlation, events, disposition handling, close semantics.
- [ ] `go test ./sdk/codeclient`; `just ci`.
- [ ] Commit `feat(sdk): codeclient embedding package`.

## Boundary

Subprocess embedding only. In-process embedding (link `internal/app`) is a different product decision — not in scope.

## Acceptance

A Go test program drives prompt→steer→abort through a real `vivy-code --mode rpc` end-to-end.
