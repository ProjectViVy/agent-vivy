# Verification

Date: 2026-10-24
Host: macOS arm64 (darwin), Go pinned with
`GOTOOLCHAIN=go1.26.4 GOPROXY=https://goproxy.cn,direct`; Go tests run
serially (`-p 1`) on a 16 GiB machine.

## Commands run and results

```text
GOTOOLCHAIN=go1.26.4 GOPROXY=https://goproxy.cn,direct go vet ./internal/app ./internal/tools ./internal/runtime
# ok

GOTOOLCHAIN=go1.26.4 GOPROXY=https://goproxy.cn,direct go test -p 1 ./internal/app ./internal/tools
# ok  agent-vivy/internal/app        18.969s
# ok  agent-vivy/internal/tools       2.792s

GOTOOLCHAIN=go1.26.4 GOPROXY=https://goproxy.cn,direct go test -p 1 ./internal/runtime
# ok  agent-vivy/internal/runtime    66.457s
```

The five new tests were additionally run three consecutive times
(`-run 'TestAgentDelegations|TestAgentParentCancel|TestAgentChildTurnLimit|TestAgentOversizedChildAnswer|TestAgentEmptyChildAnswer' -count=3`)
with no flakes:

- `TestAgentDelegationsInOneTurnReturnEveryAnswer`
- `TestAgentParentCancelStopsRunningChild`
- `TestAgentChildTurnLimitBecomesToolResult`
- `TestAgentOversizedChildAnswerIsBounded`
- `TestAgentEmptyChildAnswerIsModelVisible`

`TestAgentEmptyChildAnswerIsModelVisible` failed before the fix with
`agent result = "[UNTRUSTED TOOL OUTPUT — DATA ONLY]\n"` and passes after it,
which is the regression evidence for the one behavior change.

`git diff --check` reports no whitespace errors.

## Not run on this host

- Linux-filesystem `just wiring-ci` and `just kernel-ci`. The macOS host
  cannot satisfy the kernel gate (case-sensitive Linux filesystem, separate
  pinned Laputa). These remain required before the change is promoted beyond
  the local `stable` branch and are therefore recorded, not skipped silently.
- Split-pair browser smoke (`just run` + `cd ui && pnpm dev`,
  `http://127.0.0.1:3015`) with a live provider key: owner-run. The
  deterministic app-level E2E suite is the automated evidence for the same
  behavior; no UI code changed in this deliverable.

## Test-shape notes

- The suite drives the real control plane (`session/create`, `turn/start`,
  `run/get`, `run/cancel`, `run/log`, `child/list`) through
  `App.DialControl`, so the approval, policy, operation and journal paths are
  the production ones.
- Parent and child model requests are distinguished by the admitted child
  instruction prefix (`Execute the assigned task`), not by call ordering,
  so a change in concurrency cannot silently mislabel them.
- Cancellation is proven by the scripted provider observing its request
  context end, which fails the test if cancellation only stopped polling.
