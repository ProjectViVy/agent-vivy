# MR-1: restore compilation

Defects: D1–D4 (duplicate merge artifacts, dropped event vocabulary, stale
migration pins).

## Steps

1. `internal/rpc/control.go`: collapse the duplicated `ControlDeps` field block
   and the duplicated `RunWithOptions` literal; keep the union
   (`HumanAdmission: true, Continuity: params.continuity`).
2. `internal/app/app.go`: remove the duplicated `Crons/Channels/Titles` block.
3. `internal/domain/event.go`: restore `EventToolMounted` in `EventTypes`
   (vocabulary must hold 47 entries).
4. `internal/storage/migrations/runner_test.go`: remove the stale duplicated
   assertion; pin to manifest version 33 / count 33.
5. Fix `sdk/internal/assembly/channel_capability_test.go` module path
   (`example.com/vivy/plugins/...` → `agent-vivy/plugins/...`).

## Evidence

`go build ./...` clean; `internal/domain` and `internal/storage/migrations`
tests green.
