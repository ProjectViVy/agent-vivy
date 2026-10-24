# Verification

Date: 2026-10-24
Host: macOS 27.0.0 arm64 (Apple Silicon)
Worktree: `.worktrees/run-failure-repair`, branch `fix/run-failure-repair`

## Toolchain note

The system Go is 1.27.1, which fails to build the pinned grpc v1.64.1
(`undefined: http2.TrailerPrefix`) on both this worktree and the root tree.
Every Go command below ran with `GOTOOLCHAIN=go1.26.4`, matching `go.mod`.
The same override is required for `pnpm typecheck`, whose staging script builds
the Go UI-assembly binary.

A fresh worktree has no `ui/dist`; a placeholder `ui/dist/index.html` was
created only to satisfy `ui/embed.go` for `go build ./...` and removed before
commit, together with the temporary `ui/node_modules` symlink.

## Go gates

```text
gofmt -l cmd/vivy internal/runtime internal/tools internal/app
  (no output)
GOTOOLCHAIN=go1.26.4 go build ./...
  pass
GOTOOLCHAIN=go1.26.4 go vet ./internal/runtime/ ./internal/tools/ ./internal/app/ ./cmd/vivy/
  pass
GOTOOLCHAIN=go1.26.4 go test ./internal/runtime/ ./internal/tools/ ./internal/app/ ./cmd/vivy/
  ok  agent-vivy/internal/runtime   77.696s
  ok  agent-vivy/internal/tools      2.697s
  ok  agent-vivy/internal/app       17.606s
  ok  agent-vivy/cmd/vivy            0.578s
```

The final `app` / `cmd/vivy` pass was rerun with the embed placeholder recreated
after the first combined invocation reported `ui/embed.go: pattern all:dist: no
matching files found` for `cmd/vivy`; the placeholder was removed again
immediately afterwards.

New regression tests:

- `TestToolAdapterPreflightArgErrorRefusesInsteadOfSuspending`
- `TestToolAdapterPreflightCapabilitySkipsPrepare`
- `TestToolAdapterPreflightSandboxDenialRefusesAsPolicyDenied`
- `TestToolAdapterPreflightWiringFailureStaysRunFatal`
- `TestCommandBackendCwdRejectionsAreCorrectable`
- `TestSkillPreflightValidatesWithoutStagingRevision`
- `TestSkillManageToolPreflightReachesBackendWithoutStaging`
- `TestPairToolTurnsDefersSteerPersistedInsideToolGroup`
- `TestPairToolTurnsKeepsSteerOrderOutsideGroups`
- `TestPairToolTurnsDropsOrphanCallWithoutLosingUserRow`
- `TestBuildRunContextProjectsSteerAfterToolResults`
- `TestCompactForOverflowStabilizesPendingToolGroups`
- `TestServiceRunFailedProviderHTTPStatus` (400/401/429/500/transport table)
- `TestTerminalEventChildProviderHTTPStatus`
- `TestIncompatibleStoreHintNamesTheDataDirectory`
- `TestIncompatibleStoreHintCoversUpgradeRequired`
- `TestIncompatibleStoreHintStaysQuietForOtherFailures`

## UI gates

```text
cd ui && pnpm vitest run src/lib/failure.test.ts
  Test Files 1 passed, Tests 8 passed
GOTOOLCHAIN=go1.26.4 pnpm typecheck
  pass
```

The full `pnpm vitest run` reports 45 failed / 646 passed. Every failure
starts with `TypeError: Cannot read properties of undefined (reading 'clear')`
at `window.localStorage.clear()`; Node v24.18.0 exposes a global `localStorage`
only with `--localstorage-file`, which shadows happy-dom's window object. The
same file fails identically in the untouched root checkout
(`/Users/mastwet-macmini/pjvv/agent-vivy/ui`), so this is the environment issue
already tracked as `UI-VITEST-NODE25` on the TODO board, not a regression from
this change. All run/chat/settings suites, including `src/lib/failure.test.ts`,
pass.

## Unrun kernel lanes

`just wiring-ci` and `just kernel-ci` are the Linux-filesystem/Windows recipes
(they require the pinned WSL/Linux Laputa and the platform shell). Neither is
runnable on this macOS host, so they are **not run** here. The candidate still
needs those gates from a Linux-filesystem checkout before it is accepted.

## Real-path smoke

Setup: backend built from this worktree, a local OpenAI-compatible SSE mock, a
disposable `VIVY_USER_HOME=/tmp/vivy-smoke-run/home`, and a scripted
`turn/start` over the RPC WebSocket. No provider key was used.

### Approval preflight (Fix 1)

Run `run_62058970767dcf80` journal:

```text
tool.requested   commandline {"command":"git","args":["rev-parse","--show-toplevel"],"cwd":"../../../../.."}
policy.evaluated decide=prompt "effectful tool requires user approval"
policy.evaluated decide=deny   "tool argument \"cwd\": must stay inside the run workspace"
tool.finished    outcome=refused result="… commandline did not run: tool argument
                 \"cwd\": must stay inside the run workspace. Choose a different tool or arguments."
model.request    (second call, run continues)
run.completed
```

No `run.failed` event and no `could not be paused` line in the backend log for
that run: the run stayed interactive instead of terminating.

### Provider status split (Fix 3)

Run `run_58b86602580edd32` (mock returns HTTP 400 because the prompt contains a
marker):

```text
run.failed {"cause_category":"internal_error",
            "message":"模型服务拒绝了本次请求（HTTP 400）。请新建会话后重试；若仍出现，请附运行日志反馈。"}
```

The old `无法连接！请检查供应商配置！` is gone for a request-shaped 400.
Transport failures still map to it (unit table).

### Steer inside a tool group (Fix 2)

A deterministic live reproduction needs a tool call to stay in flight while the
steer is journaled; the mock answers instantly, so this was not reproduced on
the real path. Coverage is the projection unit tests above: the journal order
`assistant(call), assistant(call), user(steer), tool(result), tool(result)`
now projects as `assistant(call), assistant(call), tool(result),
tool(result), user(steer)`, and the overflow tail test covers the same shape
for the pending turn.

### Daily install (Fix 4)

- `VIVY_USER_HOME=/tmp/vivy-smoke-run/stable-home ./vivy` reached the listening
  state; `GET /rpc/bootstrap` returned 200.
- `VIVY_USER_HOME=/Users/mastwet-macmini/.vivy ./vivy` (default home) exited 1
  with the pre-existing `composition failed … migration 36 name drift` and the
  new line:

```text
data directory belongs to a different Vivy build
  data_dir=/Users/mastwet-macmini/.vivy
  hint=data directory /Users/mastwet-macmini/.vivy belongs to a different Vivy build;
       set VIVY_USER_HOME to a separate directory for this build, or run the build
       that created this data, and do not delete the existing database without a backup
```

`~/.vivy/vivy.db` was untouched: `sha256
859097f4e1574205e76092d1a7033be9bc6ffec4c9c7a8f33793fe96da99b7b6`, mtime
`1792828067`, size `2240512`, and no `-wal`/`-shm` sidecar was created.
