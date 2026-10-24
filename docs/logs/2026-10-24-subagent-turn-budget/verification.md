# Verification

Host: macOS arm64 (16 GiB). Module pins Go 1.26.4, so every Go command used
`GOTOOLCHAIN=go1.26.4 GOPROXY=https://goproxy.cn,direct`.

## Commands and results

| Command | Result |
| --- | --- |
| `go build ./...` | PASS |
| `gofmt -l` on the five changed Go files | PASS (no output) |
| `go test -count=1 ./internal/config` | PASS 0.467s |
| `go test -count=1 -run 'TestChildViewConstrainsToolsAndRemovesParentContextMiddleware\|TestServiceMaxToolTurns\|TestServiceBudgetCircuitBreakerStopsToolTree\|TestAgentTool\|TestAgentChildFailureKeepsParentRunAlive' ./internal/runtime ./internal/app ./internal/tools` | PASS |
| `go test -p 1 -count=1 ./internal/runtime ./internal/app ./internal/tools ./internal/config ./internal/storage/...` | PASS: runtime 68.177s, app 18.039s, tools 2.817s, config 0.113s, storage 0.306s, migrations 3.030s, postgres 0.418s, sqlite 9.209s |

`-p 1` is deliberate: parallel package builds exceed the 16 GiB host and were
already observed to terminate test processes in the previous iteration.

## Regression coverage

- `TestChildViewConstrainsToolsAndRemovesParentContextMiddleware` now sets the
  parent cap to 24 and asserts the child engine carries exactly 24 — the old
  constant would have failed this at 8.
- `TestLoadValid` asserts the new runtime defaults (10000 turns, 100000
  events, 10000 model calls, 20000 tool calls).
- Existing loop-guard tests keep passing with explicit caps (2 and 8), so the
  breach classification and the within-cap path are unchanged.

## Not run

- `just wiring-ci` + `just kernel-ci`: the gate requires an independent
  Linux-filesystem checkout; this host is macOS.
- Split-pair browser smoke at `http://127.0.0.1:3015`: needs a live provider
  key and an owner-run session. The deterministic runtime/app tests above cover
  the cap inheritance and the child-failure path.
