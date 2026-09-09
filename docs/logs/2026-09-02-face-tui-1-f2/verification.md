# verification — FACE-TUI-1 F2

Date: 2026-09-02. All commands ran from the repository root
`C:\Users\Administrator\Desktop\morediva\diva-go\agent-vivy` (except smoke in %TEMP%).

## Unit / component tests

| Command | Result |
| --- | --- |
| `cd faces/headless && go vet ./... && go test -race ./...` | `ok example.com/vivy/faces/headless 1.508s` (7 tests: completed streaming/non-streaming, approval-block cancellation, question-block cancellation, `--continue` taking an existing session, failed terminal state, and empty-prompt dial rejection) |
| `go test ./sdk/...` | `ok agent-vivy/sdk/internal 28.105s` (all existing cases for checkFaceManifest/hasNewFace/seam dispatch green) |
| `go test -run 'TestRunFace\|TestLoopback\|TestGatewayless' ./internal/app/` | `ok agent-vivy/internal/app 3.860s` (added TestRunFaceWithoutOrganFails and TestRunFaceServesGatewaylessControlPlane; no F1 regression) |
| `go build ./...` (including the cmd/vivy face.Register branch) | passed |
| `gofmt -l sdk internal cmd faces` | no output (nothing unformatted) |

## Five pack steps (real artifact)

| Command | Result |
| --- | --- |
| `go build -o vivy-sdk.exe ./sdk` | passed |
| `./vivy-sdk.exe verify faces/headless` | `ok ...\faces\headless` (seam-face validation path active) |
| `./vivy-sdk.exe pack --face headless` | `gen_7dbd0d2922738a21`, EXE built; generation.json: `recipe.face: "headless"` + `face{name: headless, kind: headless, grants: [tty argv rpc.client], tree_hash: ca80bb…}` |

## Real EXE smoke (air-gapped: everything in isolated `%TEMP%\f2-smoke`; config/storage/workspace all temporary; repository data/ untouched)

1. `ANTHROPIC_API_KEY=dummy vivy.exe run --continue "smoke probe"` →
   `vivy run: headless: no sessions to continue; run without --continue to start one`, EXIT:1.
   **Branch proof**: the `headless:` error prefix comes from the organ (if the face branch
   were not wired, kernel RunHeadless would use the `app:` prefix).
2. `ANTHROPIC_API_KEY=dummy vivy.exe run "smoke probe"` →
   `vivy: run failed (internal_error): The model run could not be completed...`, EXIT:1.
   **Full-flow proof**: the organ completes initialize → session/create → turn/start →
   run/subscribe → run.failed event rendering → terminal state → process exit-code mapping.
   Provider failure with the dummy key is expected (no real network dependency).

The scratch directory was deleted after smoke testing.

## `just ci`

- First round: `CI-EXIT:1` — two existing load-sensitive flakes in the `test` step
  (`TestCronAtJobDeletesAfterSuccessfulRun` hit the canary budget at 30.96s,
  `TestServiceGrepToolEndToEnd` took 14.98s, recorded on the TFLAKE-CRON and TEST-2
  lines); this slice touched zero files under `internal/runtime`.
- Isolated rerun: `go test -run 'TestCronAtJobDeletesAfterSuccessfulRun|TestServiceGrepToolEndToEnd' -count=1 ./internal/runtime/`
  → `ok 2.656s` (reproduced only under full load, consistent with the board record).
- Second full round: `CI-EXIT:0` (fmt-check [including faces], ui-ci, vet, test,
  headless-compile, plugin-ci [plugins + faces module roots]).

## Process lessons

- The first faces/headless test failure exposed a semantic mismatch between the fake env and
  the real control plane: after run/cancel, the server pushes a `run.cancelled` terminal
  event; the fake env now models this faithfully (the organ waits for the terminal event,
  not for the cancellation call itself).
- An accidentally deleted `for` line during pack.go editing was found and fixed immediately;
  it entered neither tests nor a commit.
