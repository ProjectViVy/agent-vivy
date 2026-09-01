# Verification — VC-2 hooks (D8)

All commands run in the worktree `agent-vivy-vc0` on `feat/vc1a-bash-tool`.

## Gate

- `just ci` — PASS (gofmt check, `go vet ./...`, full Go test suite,
  UI vitest 24 files / 195 tests, UI vite build).

## New tests

- `internal/config`: `TestHookConfigValidation` — hook entry parses
  (matcher/command/timeout_ms/approved) and the four rejection paths
  (empty command, negative timeout, timeout above 1h cap, bad matcher
  glob) fail startup with targeted messages.
- `internal/runtime`: 12 `TestScriptHook*` tests — allow passthrough
  (plus stdin payload shape), exit-2 deny with stderr reason, exit-2
  default reason, fail-closed on other exits / spawn failure / timeout,
  envelope rewrite with shallow merge (override + surviving keys),
  envelope deny, non-object updated_input fail-closed, non-JSON stdout
  allow, matcher chain-skip (no invocation, no events), and a
  real-process integration through the platform shell (`exit 2` denies,
  `exit 0` allows).
- `internal/app`: `TestScriptHooksArmingGate` — only approved entries arm;
  the unapproved entry produces exactly one loud warning naming
  `pre_tool_use[1]` and its command word.

## Real-path smoke (`vivy run` binary)

Built `go build -o /tmp/vivy-smoke.exe ./cmd/vivy`; scratch home with a
config.yaml registering `matcher: bash, command: "guard --strict"` (no
`approved`):

- Run reached the expected provider failure (no key in this environment);
  the file sink log carries
  `"msg":"hook registered but not approved","detail":"hook pre_tool_use[0]
  (guard) is registered but not approved; it will not run until
  runtime.hooks.pre_tool_use[0].approved is true in config.yaml"` — the
  ask-gate warning on the real startup path.

## Smoke exception (recorded)

A live run where an armed hook actually denies a tool call mid-run needs a
provider key; coverage for that path is the real-process integration test
(`TestScriptHookRealProcessExit2`) plus the chain deny tests
(`TestToolHookChainDenyFailsClosed` — pre-existing) wired through the same
chain the engine uses. Provider-key smoke deferred, same exception as the
headless slice.
