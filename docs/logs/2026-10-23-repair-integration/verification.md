# Integration verification

Executed on 2026-10-24 (Asia/Shanghai). Initial combined merge:
`855fe683d04a4fd3d4bb60fc328abc3f8b4a6858`. Final executable difference
from that merge is the three-line RPC canceled-context guard described in
`summary.md`; no kernel-owner source changed during verification.

## Environment

Independent Linux-filesystem checkout:
`/home/mastwet/work/vivy-repair-integration-20261024/agent-vivy`.
Own clean sibling Laputa at `9acc999301d5d575da2cea902a1c6d213763f381`;
Go 1.26.4, Node 24.21.0, pnpm 10.33.0, process-local `GOFLAGS=-p=2`
and official Go proxy. No Windows node_modules or runtime data were reused.

The Windows environment check returned a mismatched sibling Laputa and could
not enumerate WSL under the sandbox. Authorized WSL access found Debian.
The independent Laputa clone initially retained a local-source origin URL;
the bootstrap correctly rejected it. Setting that isolated clone's origin
to the lockfile's canonical repository resolved the setup failure without
changing its source pin or the Windows sibling. `just doctor` passed:
4 checks, no warnings or failures.

## Required gates and affected packages

| Command | Result |
| --- | --- |
| `just wiring-ci` | PASS: 16 Node tests and 111 Go tests, no skips. |
| `just kernel-ci` | PASS: vet, 1,986 executed kernel tests and one skip; minimal Recipe pack/Inspect/boot passed separately (one test). |
| `just d02-ci` | PASS on initial merge and after RPC correction: 83 runtime/RPC, 175 Face/CODE, one causal App, 13 shutdown, nine headless tests; no skips. |
| `go test -tags=vivy_headless ./internal/app ./internal/app/settings ./internal/provider ./internal/runtime ./internal/rpc ./internal/storage/... -count=1 -timeout=180s` | Initial run FAILED only in RPC's canceled project-file request regression. App 15.714s, settings 0.630s, provider 0.351s, runtime 90.430s, storage 0.009s, migrations 4.168s, PostgreSQL package 0.005s, SQLite 12.176s passed. Conformance directory has no directly executed tests. |
| `go test -tags=vivy_headless ./internal/rpc -run '^TestTurnStartUsesLiveContextForProjectFiles$' -count=10 -timeout=90s` | RED: ten failures before correction. GREEN: all ten repetitions passed after correction (0.939s). |
| `go test -tags=vivy_headless ./internal/rpc -count=1 -timeout=180s` | PASS after correction, 24.415s. Together with the unchanged packages' results above, completes the affected-package acceptance. |
| `pnpm --dir ui install --offline --frozen-lockfile` | PASS; independent Linux dependency tree from the local cache. |
| `go build -tags=vivy_headless -o .workspace/vivy-smoke ./cmd/vivy` | PASS on the final executable sources. |
| `node ui/scripts/stage-ui-assembly.mjs` and `pnpm --dir ui dev --host 127.0.0.1` | PASS; SDK-generated UI projection and Vite split path. |
| `gofmt -l internal/rpc/control.go` | PASS in Linux, no output. Windows command was unavailable on PATH; no Windows formatting acceptance is claimed. |
| `git diff --check` | PASS on merged changes and delivery records. |

The kernel skip was `TestBackendGrepRipgrepHonorsGitignore`: Linux `rg` was
absent, matching the source batches' known host limitation. The PostgreSQL
package result does not certify real PostgreSQL integration. No code or test
was changed to suppress a failing assertion. After the RPC-only correction,
the full RPC package and D02 were rerun; unchanged kernel-owner suites were
not needlessly repeated.

## Combined real-path smoke

Candidate binary bound `127.0.0.1:8787`; Vite bound `127.0.0.1:3015` and
proxied public RPC. Explicit disposable user home:
`/home/mastwet/work/vivy-repair-integration-20261024/smoke-home`.
The only inference destination was a loopback mock at `127.0.0.1:18437`.
No live provider credentials or tenant Journals were accessed.

1. Through Vite's public RPC proxy, upserted custom provider `Integration Local`
   with default `integration-model` absent from the entered list. Confirmed
   default reconciliation and selected that model successfully.
2. Created a disposable Session, started `INTEGRATION FIRST` on the CODE face,
   and held mock inference. Queued `INTEGRATION SECOND` and `INTEGRATION THIRD`
   with high/low thinking. Queue RPC reported two pending items;
   `background/recover` returned conflict `-32009` while the Run was active.
3. Opened the actual browser at `http://127.0.0.1:3015`, selected the Session,
   refreshed, and reselected it. The active request and both queued inputs
   remained visible. Evidence: `queued-smoke.png`.
4. Released the mock. RPC observed three user messages, three assistant replies,
   and no pending follow-ups. The browser showed all three exchanges and
   completed status. Evidence: `completed-smoke.png`.
5. Mock observations confirmed exactly three outbound requests, raw model
   `integration-model` on every request, and FIRST/SECOND/THIRD input order.
   The first observer assertion incorrectly assumed string content; adapting
   only the temporary observer to accept the adapter's valid structured text
   format made the assertion pass against the same recorded requests.

Raw scoped logs are retained in the independent checkout's `.workspace/`.
Temporary launchers, transfer archive, mock/client scripts, processes, and
disposable smoke data are removed during delivery cleanup. The test checkout
and its verification logs remain available.

## Review and limits

Independent read-only review found no merge-specific issue and no actionable
issue in the RPC compatibility guard. The inherited idle-queue restart gap is
recorded as `QUEUE-IDLE-RESTART-DRAIN`; it was established by control-flow
inspection and remains uncorrected. Orphan-blob GC remains a separate open item.

Manual/live-provider acceptance belongs to the owner. Full-product `just ci`,
full race, real PostgreSQL, Docker, default/optional Module certification,
native Windows packaged acceptance, and UI unit suites were not run in this
local integration scope. No frontend source changed. No push or release was
performed; a separate release record is therefore omitted.
