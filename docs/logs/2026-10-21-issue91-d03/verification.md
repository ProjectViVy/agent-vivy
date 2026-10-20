# Verification

Accepted prerequisite: stable
`e2c1518e9388cca091828c85df5aadc4f59406fa` (D02).
The D03 candidate is the focused commit containing this record. Its runtime
source identity is reproduced by the checked-in conformance bundle, regenerated
only after all owning suites passed on frozen source.

## Venue and gates

Authoring: Windows Server, root stable, one sequential writer. Verification:
`/home/mastwet/work/vivy-issue91-stable/agent-vivy` on Debian WSL2's Linux
filesystem, with its own clean pinned Laputa
`ff3936f44ff8cf08c12af2cf698c194cfe474fd3`.
Go 1.26.4, Node 24.21.0, pnpm 10.33.0, just 1.40.0; process-local
`GOFLAGS=-p=2`. Headless tags were command-local, never global.

The owner explicitly replaced Windows full-product CI reruns with scoped WSL
verification. No Windows full-product, Docker packaging or full release
acceptance is claimed.

| Command/check | Observed result |
| --- | --- |
| `go test -tags=vivy_headless ./internal/storage/... -count=1 -timeout 35m -json` with disposable DSN | 330 actual passes, zero skips/failures; SQLite 4.623s, PostgreSQL 83.975s |
| Final source/preflight subset and explicit legacy PG fixture conversion | 22 passes, zero skips/failures |
| `go test -race -tags=vivy_headless ./internal/storage/sqlite ./internal/storage/postgres -run 'TestToolOperation\|TestBackendConformance/(CN-38\|CN-39)' -count=1 -timeout 35m -json` | 12 passes, zero skips/failures; both named conformance cases actually executed |
| Complete runtime before final lead review corrections | 959 passes, one existing optional ripgrep skip, zero failures; 49.741s |
| Runtime flight/replay/cached sibling/prepared approval/crash race subset | 16 passes, zero skips/failures; 11.500s |
| Strict approval denial and shell declaration/boundary subset | 48 passes, zero skips/failures |
| SQLite wildcard-source and cached multimedia correction subset | 24 passes, zero skips/failures |
| Live fatal/cancellation redaction plus typed replay and cached multimedia under `-race` | 12 passes, zero skips/failures; 8.132s |
| `just wiring-ci` | 16 Node and 105 Go passes; zero skips/failures |
| Final `just kernel-ci`, after review fixes and evidence refresh | vet passed; 1712 Go passes, one optional ripgrep skip; real minimal Pack/Inspect/boot passed separately |
| `just test-postgres` | 118 actual passes, zero skips/failures |
| Actual guarded conformance producer, update mode | All 23 owning suites passed, 70.947s; artifact published only afterward |
| Strict conformance reproduction, ordinary read-only mode | All 23 owning suites passed, 80.231s |
| `git diff --check` and final staged whitespace check | Passed |

The sole skipped case is `TestBackendGrepRipgrepHonorsGitignore`: this WSL host
has no external ripgrep executable. It is not counted as a pass, and none of
the required D03, race, PostgreSQL or minimal artifact cases skipped.

PostgreSQL was PostgreSQL 16.15 in the task-owned WSL Docker container
`vivy-d03-postgres-20261020`, port 55404 on the WSL private address. The real
disposable DSN was supplied explicitly. Native Windows connection attempts
from the earlier investigation are not acceptance evidence.

The four actual process-death cases executed:

- `admitted_before_claim`
- `claimed_before_invoke`
- `effect_before_completion`
- `completion_before_graph_checkpoint`

## Review corrections

Independent read-only review found two defects, both reproduced before repair:

1. SQLite `LIKE 'sqlite_%'` hid legal application tables/views because
   underscore is a wildcard. Four cases (unknown-only/current namespace,
   table/view) failed; literal-prefix GLOB filtering passed while retaining
   source schema/data.
2. Direct/broker callback errors could return unredacted live text although
   the committed failure was redacted. Fatal and wrapped-cancellation tests
   failed; the existing redacting wrapper now preserves `errors.Is` and
   claim/control-flow semantics without leaking the canary.

Lead integration also reproduced lost media in the missing-tool.finished
cached-sibling window. The existing enhanced normalization now reconstructs
native text/image/audio/video/file projections, including base64 field names.
The real native checkpoint producer/recovery regression passed without another
tool invocation. No replacement executor was introduced.

The reviewer traced approval/denial authority, unknown-effect fences, shell
state, native cached siblings, atomic completion, migration refusal and digest
closure. Narrow correction review and final evidence review found no remaining
substantiated issue. Review was independent and read-only; executed-test and
browser observations belong to the lead.

## Real split-browser acceptance

Venue: Windows in-app browser at `http://127.0.0.1:3015`, WSL Vite and a
source-built headless backend at :8787. Home:
`/tmp/vivy-d03-smoke-NSjVd3Jj/home`. The local streaming mock was at :9911,
using the embedded DeepSeek adapter and valid catalog model
`deepseek-flash`. No real provider key or network model was used.
Persona was initialized through the ordinary session-bound Action RPC.

| Case | Run | Observed behavior |
| --- | --- | --- |
| Approve | `run_442372ae0c4829fd` | UI approval wrote the exact nonce once to the Run workspace; success/unknown persisted |
| Deny | `run_10ce04f881537ffd` | UI denial created no file; refused/user_denied/not_executed persisted |
| Restart while pending | `run_a758ecf3fae62981` | SIGKILL of only the owned backend, 35s natural lease wait, same home/new process; same approval recovered in a fresh browser page, then approved and wrote once |
| Private canaries | `run_bf99115877f588fb` | Original request and decoded provider data retained test canaries; UI preview redacted them; public Journal, approval/list, review/list, review/get and runtime logs were canary-free; denied without writing |

For the first three cases, a read-only SQLite inspection and the captured mock
requests verified the exact tool call ID and normalized result in the
**immediate next model request**, one terminal event per Run, completed status,
no active Runs and no pending approvals. Later historical transcript projection
is a different existing path and was not mistaken for that immediate handoff.
Default workspaces are scoped under `home/workspace/<run_id>/`.

The browser reload encountered a transient in-app error-page navigation issue;
a fresh localhost tab loaded successfully and reconstructed the same pending
Run/approval from the server. No browser security interstitial was bypassed,
no lease row was changed, and no approval was recreated.

Screenshots: [approval and denial](browser-smoke.png),
[recovered approval completed](browser-restart.png),
[private canaries redacted](browser-privacy.png).
The mock, Vite, backend and database resources were task-owned; production
Journals were never accessed. The owned servers were stopped and the temporary
home and disposable PostgreSQL container removed after evidence collection.

## Residual identities and conversion boundary

| Remaining occurrence | Owner/reason |
| --- | --- |
| Operation digest names in released 031, 036 DROP statements, preflight and old-source/rejection fixtures | Immutable history and explicit format refusal; no runtime producer |
| File PreconditionHash and file/history/reference identities | D09 |
| Policy/admitted snapshots and checkpoint identities | D08 |
| Work/child/workflow/notes business identities | D11 |
| Migration checksums/metadata | D12 |
| Source/Assembly/Generation evidence | D06, retained until that cut; refreshed by actual producer here |

Retired operation/request/approval argument digest identifiers and producers
are absent from the changed live operation/approval/shell paths. Historical
event bytes remain readable and unchanged. Paired 036 leaves every baseline
operation's original input and new classifications NULL.

D03's offline approval fragments and copy/rollback fixtures are delivered.
D08 must compose checkpoint/interaction/unique terminal settlement in the same
target transaction; D12 owns final metadata; D14 owns the finite upgrade tool.
This delivery does not authorize upgrade publication or modify a source store.
