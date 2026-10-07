# A2A-01 verification

Environment: Go 1.26.8, just 1.43.1, `GOPROXY=https://goproxy.cn,direct`,
`GIT_CONFIG_GLOBAL=/dev/null`.

## A2A-01.1 contract

```text
go test ./sdk/port/channel -run TestTaskHostContract -count=1   ok
for m in telegram dingtalk discord feishu qq; do go -C plugins/$m test ./...; done
ok on all five channel Modules — no source compatibility change.
```

## A2A-01.2 wiring (red first, then green)

```text
go test ./internal/app -run 'ChannelTask|TaskHost|ChannelCapability' -count=1
  red: undefined BindChannelsWithModuleIDs (expected red)
go test ./sdk/internal/assembly -run ChannelModuleIDs -count=1
  red: missing emitted map / no ambiguity error (expected red)

After implementation:
go test ./internal/app -count=1                                ok
go test ./sdk/internal/assembly -count=1                       ok
go test ./sdk/internal/conformance -count=1                    ok (digest re-pinned)
go test ./internal/channelhost ./sdk/internal/... -count=1     ok
```

Matrix covered: grant absent / taskless Host -> no TaskHost assertion;
authorized real task Host -> all five calls + TaskServiceInfo reach the
underlying Host; sealed Module ID on ModuleID(); missing/empty module
identity -> bind + app validation errors; ambiguous provider claim ->
generator error; typed-nil task Host -> assertions succeed without
touching the nil receiver; ordinary Channels unchanged (existing
BindChannels/conformance callers still valid).

## just ci (Story boundary)

```text
just ci
red — single failure: sdk/codeclient TestClientAgainstRealVivyCode
  "persona is not initialized" steer rejection
```

Documented pre-existing main regression from `cc54ee27` (identical on a
clean origin/main worktree; proven again at `eaf9ad76`). Fix is in
flight as PR #37 (branch `devin/1791350615-fix-code-face-steer`,
`"face":"code"` on queued-turn params), awaiting owner merge. Not a
branch-introduced break: this Story adds no codeclient/steer surface.

One flake observed: `TestPlanGoalIntegratedRoundLimitBlocksDurably`
reported `no such table: schema_meta` under full-package parallel load;
green in isolation and on full-package rerun — BML/laputa embedded init
race, unrelated to this change.

Secondary evidence: `go generate ./internal/generated/assembly`
regenerated `zz_default.go` deterministically (ChannelModuleIDs present);
`git diff` on the generated file shows only the new field/map.
