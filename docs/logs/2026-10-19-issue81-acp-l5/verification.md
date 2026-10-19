# Verification — issue #81 E8 / L5 ACP

## Environment

| Fact | Value |
|---|---|
| Host | macOS arm64 |
| Go | `GOTOOLCHAIN=go1.26.4` (host 1.27.1 breaks the pinned toolchain closure; the repo pins 1.26.4 in `README.md` and every module) |
| Caches | `GOPATH`/`GOCACHE`/`PNPM_HOME` inside the lane `.workspace/` (the sandbox cannot write `~/go`, `~/Library/Caches/go-build`, `/opt/homebrew`) |
| Proxy | `GOPROXY=https://goproxy.cn,direct`; the owner proxy `127.0.0.1:7897` was not needed after that |
| pnpm | 12.10.1 (repo pins 10.33.0) with `npm_config_manage_package_manager_versions=false` |
| UI dist | `ui/dist` built locally because `ui/embed.go` embeds it and it is untracked |

## Gate

`just ci` was attempted with a `just` 1.59.0 binary downloaded from the
project's release page into `.workspace/bin` (`go install` cannot fetch it:
the project tags releases without the `v` prefix, and the sandbox blocks
Homebrew's `/opt/homebrew` writes). See the `just ci` section for the
step-by-step result: the serial gate stops at `ui-ci` on 39 pre-existing UI
environment failures, so the remaining recipes were run individually.

## Commands and results

| Command | Result |
|---|---|
| `go build ./cmd/vivy` | PASS |
| `go test ./internal/faceprocess ./internal/codeface ./cmd/vivy ./internal/config` | PASS |
| `go test ./internal/rpc` | PASS (16.6 s) |
| `go test ./internal/app -count=1 -timeout 30m` | PASS, every test (verbose log kept out of the tree; the package needs the longer bound because the memory-loop crash tests wait out a ~24 s lease each) |
| `go test ./plugins/acp -count=1` | PASS |
| `go test ./plugins/acp -count=1 -race` | PASS |
| `go test ./sdk/internal/testdata/acp-compat -count=1 -race` | PASS against the pinned fork |
| `go test ./sdk/internal -run '^TestPackACPSelectedOverlay$'` | PASS (overlay stages the TUI stub, `go list -deps` omits the TUI organs, the selected build links) |
| `go test ./sdk/internal -run '^TestACPRecipeSelection$'` | PASS (real `recipes/acp.vivy.yml` + real `plugins/acp`: T2, `projectvivy.acp`, `rpc.client`, pin == manifest == tree hash, TUI stub staged, no other Recipe selects ACP) |
| `go test ./sdk/internal -run '^TestPrepareBuildModfile'` | PASS (external version replacement carried into the build modfile; conflicting pin and directory replacement fail closed) |
| `VIVY_ACP_ARTIFACT_E2E=1 go test ./sdk/internal -run '^TestACPPackedArtifactScriptedPeer$' -v` | PASS in 4.3 s with a warm build cache. Evidence: `Pack` of `recipes/acp.vivy.yml` + `InspectArtifact` shows `projectvivy/acp` T2 in the manifest; the packed binary answers `initialize` (`protocolVersion 1`, `agentInfo.name projectvivy/acp`), `session/new` returns a session id, `session/prompt` without a configured model returns the governed error `{code:-32603, data:{reason:INTERNAL_FAILURE}, message:"run failed"}`, `session/cancel` is accepted, stdin EOF exits 0 |
| `go run ./sdk/internal/cmd/source-hash plugins/acp <declared>` | new digest `b92be2a2fad8a8c39db81449f642f21afcf141db2e66c089e4d14480835b2e91`; re-running with that digest is stable |
| `go list -deps ./plugins/acp` | only `github.com/mastwet/acp`, `/conn`, `/internal/jsonrpc`, `/transport`, `/transport/stdio`; no WS/HTTP/Hertz/Gin transport in the closure |
| `gh api repos/mastwet/acp/commits/780fb4dd4b98` | fork commit `780fb4dd4b98b3936fe572f61ae83b3abc2f918d`, 2026-10-07T07:17:07Z, "feat(conn): expose agent connection options and opt-in wire error sanitization" |

## Not verified on this host

- No Docker, PostgreSQL, or Windows `.exe` run (see `ACP-WINDOWS-ACCEPTANCE` in
  `docs/TODO.md`).
- `diva-recall-test` and the memory-loop second pass depend on the DIVA Recipe
  and were not exercised beyond the gate attempt below.
- No live model was configured for the ACP artifact: the peer script proves the
  protocol and the governed failure path, not a model-backed turn. Approval and
  question flows are proven against a scripted ACP client over a real SDK
  connection (`plugins/acp/interactions_test.go`, `lifecycle_test.go`), not by a
  human answering a dialog.

## `just ci`

Run from the lane root with the environment above plus
`GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null` — the host's global
`url.https://gh-proxy.com/https://github.com/.insteadOf` rule makes
`git remote get-url` print the proxied URL, which `ensure-laputa` correctly
refuses; disabling global git config for the gate is the least invasive fix
(the lane's `../laputa` is at the locked commit `4b2bec2`).

| Step | Result |
|---|---|
| `ensure-laputa` | PASS after the origin/`insteadOf` issue above |
| `bootstrap-test` | PASS (11 node tests) |
| `fmt-check` | PASS (1251 Go files formatted) |
| `ui-ci` (`ui-core`) | **FAIL**: 39 `ui` tests fail because `window.localStorage` is undefined under the pinned `happy-dom 20.14.0` with host Node 26.11.0. Reproduced unchanged on a pristine `origin/main` `e9609296` worktree (`pnpm install --frozen-lockfile`, `ui/dist` built). Recorded as `UI-VITEST-LOCALSTORAGE`; unrelated to this batch. |
| `vet` (run separately) | PASS |
| `headless-compile` (run separately) | PASS (`cmd/vivy`, `cmd/vivy-code`, `ui`) |
| `plugin-ci` (run separately) | PASS, 23 modules including `plugins/acp` |
| `diva-recall-test` (run separately) | PASS, 6/6 memory-loop recalls |
| `test` (run separately): `go test -p 1 -timeout 35m -skip '^TestMemoryLoop' ./...` | 83 packages `ok`; two failures, both outside this batch. (1) `sdk/codeclient` `TestClientAgainstRealVivyCode`: `handshake get_state: protocol stream ended`, reproduced on pristine `main` (recorded as `CODECLIENT-PERSONA-STEER`). (2) `sdk/internal` `TestLegacyPluginSurfaceIsPhysicallyRemoved`: the sandbox forced `GOPATH` inside the repo (`.workspace/go`), so the repository-wide scan flagged third-party module-cache files as removed surface; with `GOPATH` moved outside the tree the test and the whole `sdk/internal` package PASS (`ok 138.7 s`). |
| `test` second line: `node scripts/memory-loop.mjs` | PASS ("memory-loop: passed"; the DIVA overlay app suite inside it took 742 s) |

So the only gate steps that do not pass on this host are the UI environment
failure and the pre-existing `codeclient` failure; every kernel, SDK, plugin,
headless, DIVA and ACP step passes.
