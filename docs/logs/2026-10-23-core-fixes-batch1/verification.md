# Verification

Source base: `7496f35f4c2e54cb95d037433d2d3cae1fcc863b`.
Independent Linux checkout:
`/home/mastwet/work/vivy-core-fixes-batch1/agent-vivy`, with its own clean Laputa
`9acc999301d5d575da2cea902a1c6d213763f381`. Go 1.26.4, Node 24.21.0,
process-local `GOFLAGS=-p=2` and official Go proxy. The Windows sibling Laputa
was not changed. No tenant Journal was accessed.

## Regression evidence

- Before the boundary fixes, `TestBackgroundRecoverRefusesLiveRun` failed:
  recovery closed the active Run. `TestTurnFilesUseTargetSessionWorkspace`
  failed because the process project was used instead of the Session selection.
- Before the recovery drain fix, `TestQueueRecoveryAdmitsSavedRequestAfterRestart`
  failed because the saved request never reached inference.
- Focused Linux storage/runtime/RPC regressions pass, including literal paths,
  enqueue failure, immutable full request replay, FIFO, separate follow-up Runs,
  atomic tail transfer before inference, and restored-request admission.
- Native Windows: `go test ./internal/storage/sqlite -run
  TestOpenPreservesLiteralFilename -count=1 -timeout=2m` passed. Windows forbids
  the `?` filename case; Linux executes that case.

## Required gates

- `just wiring-ci`: passed, 16 Node and 111 Go tests, zero skips.
- First `just kernel-ci`: passed, 1978 executed tests and one host-related skip
  (`TestBackendGrepRipgrepHonorsGitignore`, Linux `rg` absent); minimal artifact
  pack/inspect/boot passed separately.
- First `just d02-ci`: passed, 83 runtime/RPC tests, 175 Face/CODE tests,
  one causal App test, 13 shutdown tests, and nine headless Face tests.
- Final focused race: `go test -race -tags=vivy_headless ./internal/runtime
  ./internal/rpc -run 'TestQueue|TestQueued|TestFollowUp|TestSteer|TestClearQueue|
  TestDequeue|TestBackgroundRecoverRefusesLiveRun|TestTurnFilesUseTargetSessionWorkspace'
  -count=1 -timeout=180s` passed (runtime 43.309s; RPC 7.121s).
- Byte comparison confirmed every changed executable/test/schema file in the
  independent Linux verification checkout exactly matches this working tree.
- Final `just kernel-ci`: passed, 1981 tests; the same single missing-rg
  host skip remains. Minimal artifact pack/inspect/boot passed (one test).
- Final `just d02-ci`: passed, 83 runtime/RPC + 175 Face/CODE + one causal
  App + 13 shutdown + nine headless tests, zero skips.
- `gofmt -l` on all changed Go files produced no output; `git diff --check`
  and `git diff --cached --check` passed.
- The disposable split services were stopped and task launcher/mock scripts
  removed. The independent verification checkout retains raw scoped-gate logs
  under `.workspace/core-*.log`; its disposable home, workspace, UI dependency
  tree, and split executable were removed.

## Real-path smoke and limits

- Started a separately installed Vite tree and headless split backend with
  `VIVY_USER_HOME=/home/mastwet/work/vivy-core-fixes-batch1/smoke-home` and a
  loopback-only mock provider. No live provider credentials or network inference.
- Public RPC through the Vite proxy selected a disposable Session workspace,
  started a CODE Run with a context snapshot, and queued two distinct requests
  with high/low thinking. Recovery returned conflict `-32009` while active.
- Browser refresh at `:3015` showed the active Run and both queued requests;
  after settlement it showed three separate completed exchanges. This smoke
  preceded the final failed-steer/FIFO, shell admission, and deletion refinements;
  those failure paths are covered by the final tests and scoped gates.
- No UI source changed, so frontend unit suites and `d02-web-ci` were not
  required. Full-product `just ci`, default/optional Module certification,
  real PostgreSQL integration, Docker, and native Windows whole-product
  acceptance remain unrun; scoped kernel gates do not certify those lanes.
- Cleanup failures after a committed private-snapshot deletion are logged;
  a process crash between BlobStore preparation and Journal commit can leave an
  unreachable private blob. This does not acknowledge or resurrect a request.
