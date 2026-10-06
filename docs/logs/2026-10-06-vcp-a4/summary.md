# A4 — `sdk/codeclient` Go embedding client

**Commit:** `feat(sdk): codeclient embedding package`
**Depends on:** A3 (the wire contract it speaks)

## What landed

`agent-vivy/sdk/codeclient` — a Go package that spawns
`vivy-code --mode rpc` as a child process and speaks its JSONL protocol:

- `New(Config{Binary, Args, Dir, Env, Stderr, EventBuffer})` → spawns
  `--mode rpc`, handshakes via `get_state` (30s) before returning. `--mode`
  overrides in Args are rejected.
- `Call(ctx, command, params)` → id-correlated request; response `data` on
  success, error with the child's error text otherwise.
- Typed surface mirroring §5.3: `Prompt, FollowUp, Steer, Abort, ClearQueue,
  NewSession, SwitchSession, SetSessionName, GetState, GetMessages,
  LastAssistantText, GetSessionStats, Models, SetModel, CycleModel,
  SetThinkingLevel, ThinkingLevels, SetSteeringMode, SetFollowUpMode,
  SetAutoCompaction, SetAutoRetry, AbortRetry, Compact, Fork, Clone, Tree,
  Entries, Export, Commands` — deferred commands return the child's
  `success:false` error today and work when the backend lands (no API break).
- `Subscribe()` → ordered `Event` stream (type + full record). Bounded
  per-subscriber channel (default 256); a stalled consumer drops records and
  counts them on `Dropped`.
- `Close()` closes stdin (the child's graceful EOF shutdown), waits 5s,
  then kills.
- Child inherits parent env plus `Config.Env`; `Config.Dir` sets the child's
  session storage root.

## Boundary

Subprocess embedding only — in-process linking against `internal/app` is a
different product decision, out of scope.
