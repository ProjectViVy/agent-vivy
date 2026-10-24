# Agent tool promoted to the always-disclosed core — verification

## Environment

- Host: macOS arm64, worktree `.worktrees/agent-tool-fault-isolation`, branch
  `fix/agent-tool-fault-isolation`.
- Go commands ran with `GOTOOLCHAIN=go1.26.4 GOPROXY=https://goproxy.cn,direct` because the host Go
  is 1.27.1 and the pinned grpc does not build under it.

## Commands and results

| Command | Result |
| --- | --- |
| `gofmt -l internal/tools/toolactivation.go internal/tools/toolactivation_test.go` | no output |
| `go test ./internal/tools -count=1` | ok |
| `go test ./internal/runtime -run 'ToolSearch\|Exposure\|ToolActivation\|Agent' -count=1` | ok |
| `go test -p 1 ./internal/runtime -count=1` | ok (`68.946s`, serial) |
| `go test ./internal/app ./internal/tools -count=1` | ok |
| `git diff --check` | clean |

One run that executed `internal/runtime`, `internal/app` and `internal/tools` concurrently on this
16 GiB host failed `TestCacheWarmFailureRemainsDiagnostic` with a context-interrupted journal append;
the same test passed three consecutive focused runs and the package passed with `-p 1`, so the
failure is host contention, not the exposure change. The serial package run is the recorded result.

## What the tests pin

- `TestResolveToolExposureDefaults`: the fixed core is 12 tools, `agent` resolves to `direct`
  without an explicit declaration, and an explicit `Exposure: deferred` still overrides the core
  default.
- Existing `tool_search` coverage (`internal/runtime/tool_search_e2e_test.go`) still passes with
  `echo_info` as the deferred candidate, so the deferred path itself is unchanged.

## Unrun lanes

- `just wiring-ci` + `just kernel-ci` require an independent Linux-filesystem checkout and were not
  run on this macOS host. They must run at the candidate before merge.
- Browser smoke (`just run` + `cd ui && pnpm dev`, `http://127.0.0.1:3015`) is owner-run; the
  schema change is visible in the request payload rather than in a dedicated UI surface.
