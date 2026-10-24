# W1-1 verification

Base: `fc559e6b` (frozen origin/main). Branch: `feat/wails-migration`.

## Test commands

- `go test -race -timeout 25m ./sdk/host/v1 ./internal/embedded ./internal/app ./internal/logging`
  → all packages `ok` (sdk/host/v1 11.2s, internal/embedded 5.6s,
  internal/app 48.7s, internal/logging 1.1s). 14 sdk boundary tests,
  9 embedded Next tests, 3 embedded + 2 app close tests pass under -race.
- External-consumer compile proof: `TestExternalModuleConsumesPublicAPI`
  writes a temp module (`vivy-external-consumer-test`) with
  `require agent-vivy v0.0.0` + `replace agent-vivy => <repo>` plus every
  local `replace` harvested from the repo go.mod, `main.go` importing only
  `agent-vivy/sdk/host/v1`, then `go mod tidy` + `go build`. Green → the
  public surface compiles from outside the module and no `internal/*`
  symbol is reachable through it.
- `just ci` — result recorded below once complete (fmt-check, ui-ci,
  vet, `go test -timeout 35m ./...`, headless-compile, plugin-ci).

## RED evidence (TDD)

- `sdk/host/v1` boundary tests were written before the package existed;
  `go test ./sdk/host/v1` failed with "no non-test Go files" (compile
  failure = missing feature) before `host.go` was implemented.
- The external-consumer test cannot pass without the public package.

## Rulings ledgered during implementation

- **LAPUTA_REF is stale on the frozen base.** `internal/modules/diva-cognitive`
  (merged at `6ff82eae`) requires `garden/agentapi` symbols
  (`EvolutionPorts`, `HumanClient`) that only exist in laputa commits
  `a6eaeca`/`0200b16`, merged as laputa `6f2eed2` — *after* the CI-pinned
  `c9c5efe`. Local verification used laputa `6f2eed2`. Consequence if
  wrong: CI's pinned checkout cannot compile agent-vivy main at all —
  i.e. main's CI gate is already broken independent of this work; the pin
  bump is an owner-side repo fix, not part of W1.
- **Close semantics preserved for CLI/C ABI.** `Close()` still blocks the
  caller for the whole teardown; `CloseContext` is additive. Cost if
  wrong: the console 5 s CTRL_CLOSE budget path changes → kept the wrapper
  semantics byte-for-byte.
- **Teardown goroutine + stage reporting.** Both `embedded` and `app`
  expose `CloseContext` that returns a named-stage timeout while teardown
  proceeds. A timed-out close retains the process claim in `sdk/host/v1`
  (contract: never reopen over live resources); release happens only on a
  `nil` result.
- **Test seams**: `Host.teardownHook` / `App.closeHook` (unexported,
  invoked at teardown start) let deadline tests hold teardown open
  deterministically. Cost if wrong: none — no production path sets them.
- **Peer join**: `Peer.ServeDone()` closes when `Serve` unwinds (read loop
  + writer joined); embedded close joins it with a 5 s bound instead of
  racing the pump.
- **Logging**: initialized once per process via `sync.Once`
  (`logging.Setup` → `slog.SetDefault`), sinks retained until process
  exit — closing them at Host.Close would strand a later owner.
  `Stdout` follows config (`logging.stdout`); the CLI path is unchanged.
- **Next limit policy**: `sdk/host/v1` validates 0..500 (0 = default 500),
  `invalid_input` outside; `embedded.Next` keeps the legacy clamp for the
  C ABI `Poll` semantics.
- **Claim ordering**: config-path + sealed-manifest validation happen
  *before* the owner claim, so a bad-input `Open` never even momentarily
  holds the slot; composition failure after the claim always releases.

## Known limitations / deferred

- `not_ready` kind is defined but currently unproduced (no pre-ready call
  state exists in this embedding model); documented for W2 review.
- `FrozenCore`/restart semantics for the sealed generation are verified in
  W5, not asserted fixed here.
- The consumer test materializes local `replace` directives by parsing the
  repo go.mod (the W2 modfile problem); a shipped SDK pin will need the
  published-modfile story from W2.
