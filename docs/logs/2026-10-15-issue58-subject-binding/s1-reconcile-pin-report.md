# S1 Task 4: published reconcile owner and bound compatibility

Date: 2026-10-15. Status: implementation and requested integration gates complete;
ready for independent scoped review. S1 durability and S6 product acceptance remain
qualified. Consumer base: `4658e263746b4e10059627c896f8d07628b50405`.
Source/test commit: `2936b6d5d59cb0c776bf834a3c7a3afa5ec5e1d6`.
Author and committer: `mastwet <1136523089@qq.com>`. This report and its verification
manifest are committed separately as evidence for that exact source/test commit.

## Published identity and change

The controller published independently approved owner source
`9ec9baa4db117c3ea3b4a0949fece14999904895` as
`bb57e93a61cfb41460524d00a557a3a9f2cce1e7`, timestamp
`2026-10-15T12:40:34Z`, identical tree
`654b8aa8c81251b8843356ebf3c91b00157ba4c5`. Normal module metadata for **all three**
Garden/Laputa/Mentle modules confirms that full Origin.Hash, timestamp and
`v0.0.0-20261015124034-bb57e93a61cf`. The source lock records the full published SHA.
The clean sibling was already at that exact SHA/tree; no upstream source was edited.

The exact API is authoritative design revision 3 C1: existing owner DTO aliases and
bound `ReadRecap`, `CommitRecap`, `LookupRecapOperation`, `ListRecapHistory`, and
`ReadRecapRevision` methods are unchanged. `Cause="reconcile"` has zero-width source
coordinates equal to current coverage, an opaque nonblank receipt and no restore
payload, preserving coverage and epoch. The unchanged cognitive facade forwards to
Garden's existing bound handle and the SAME owner. No forwarding gap was found.

Source changes are limited to three requirements in `go.mod`, six authenticated
archive/go.mod lines in `go.sum`, `laputa-source.lock.json`, and one focused real
Garden-owner regression in existing `internal/modules/diva-cognitive/recap_test.go`.
All old sums, local replacements, Eino v0.9.20/EinoExt pins, S2 source, persona
fixtures, design/plans and existing reports remain intact. There is no new Store,
DTO, public Port, runtime engine or UI surface. This is dependency integration and
adapter verification, not a new inference capability; the existing C1 revision 3
Eino capability check remains applicable.

## Behavioral proof

`TestIssue58RecapBoundReconcile` opens the actual cognitive bundle/Garden owner for
coverage zero and nine. It verifies exact Chinese/CRLF Markdown and an opaque
future-version receipt, unchanged coverage and epoch (including a post-clear epoch),
omitted binding stamping and equal-binding replay/new commits. Foreign subject or
domain assertions return typed `*agentapi.Error` with `recap_scope_denied` and a zero
head before a matching operation can reveal its receipt. Changed receipt bytes
return `recap_operation_conflict`; invalid endpoints preserve `recap_invalid_range`.

Later edit, clear and restore each reset a fresh certificate. Clear increments the
epoch once; restore of the certified historical revision restores only its text,
not its certificate, while preserving current coverage/epoch. The original reconcile
operation still returns its exact receipt after all three later mutations; lookup,
revision/history and same-owner-namespace reopen remain correct. Existing unchanged
Garden tests cover worker/grant/unarmed/closed/lifecycle denial before cause handling;
no duplicated cause-specific authorization implementation or upstream test was added.

Meaningful RED used old **published** requirements
`v0.0.0-20261015104519-4c7c6b208593`, with only the three Laputa replacements dropped
in a temporary root modfile. Both `/0` and `/9` fail on the first valid reconcile
with typed `recap_invalid_range`, zero head; there is no build/environment error.
GREEN changes those normal requirements/sums to the reviewed publication, using the
same no-Laputa-replacements modfile. The focused regression passes (one top-level,
two subtests). Raw RED/GREEN are retained beside this report. Assertions were also
tightened for later reconciling epoch and edit text before final GREEN/package runs.

## Normal module authentication

All acquisition/test commands used Go 1.26.4 linux/amd64 and
`PATH=/workspace/scratch/243d1cbb859f/toolchains/bin:$PATH` unless the explicitly
reported initial bootstrap invocation below. Checksum settings were normal:
`GOSUMDB=sum.golang.org`, `GOPROXY=https://proxy.golang.org,direct`, empty
`GOPRIVATE`, `GONOSUMDB` and `GONOPROXY`. No checksum bypass or fabricated sum.

A temporary minimal three-requirement modfile, without **any** replacements, ran:

```text
go mod download -modfile=/tmp/issue58-reconcile-pins.mod -json github.com/ProjectViVy/laputa/garden github.com/ProjectViVy/laputa/laputa github.com/ProjectViVy/laputa/mentle
go mod verify -modfile=/tmp/issue58-reconcile-pins.mod
```

Both exited zero; verification output is `all modules verified`. The six tooling
sum lines were copied exactly to root `go.sum`. Full normal `.info` metadata with
Origin.Hash/time, all six sums and the modfile are embedded in
`s1-task-4-verification.json`. `s1-task-4-final-manifest.json` records source-file,
RED/GREEN and verification hashes, plus root module resolution proving all three
modules came from published cache paths without Replacement fields.

The checked-in root intentionally retains three historical local Laputa replacements.
For actual published-module consumer proof `.superpowers/sdd/S1/task4-root.mod`
removes **only those three**; all other application-local replacements remain. This
is distinct from the ordinary root test at the clean exact published sibling.
Default root `go mod verify` still exits one with the existing 19 `agent-vivy/...`
local v0.0.0 placeholder **missing ziphash** conditions. This is neither an archive
checksum mismatch nor a root verification pass; the complete output is retained in
the verification record. It was observed before the pin edit, whose unchanged
replacement architecture does not resolve that pre-existing limitation.

An exploratory `go list -m` while the new minimal modfile lacked sums returned
`updates to go.sum needed, disabled by -mod=readonly`; after normal download,
explicit `-mod=mod` metadata lookup succeeded. No root dependency or checksum policy
was relaxed to resolve this bookkeeping condition.

## Required gates and actual results

`-json` only records Go events; selections/counts match the requested commands.
No named test skipped or failed in the final gates. Top-level test counts exclude
subtests; package-only no-test results are separately identified below.

| Working directory | Command | Result |
| --- | --- | --- |
| Exact published sibling `laputa/garden` | `go test -json ./agentapi -run '^TestIssue58' -count=1` | exit 0; 6 top-level tests |
| Same Garden | `go test -json ./agentapi ./internal/runtimecore/... -count=1` | exit 0; 89 top-level + 10 subtests, 2 packages |
| Exact published sibling `laputa/laputa` | `go test -json ./... -count=1` | exit 0; 102 top-level + 61 subtests, 5 tested packages |
| Consumer, published modules | `go test -modfile=.superpowers/sdd/S1/task4-root.mod -json ./internal/modules/diva-cognitive ./internal/cognitivecontract -count=1` | exit 0; 30 top-level + 10 subtests |
| Consumer, ordinary root/exact sibling | `go test -json ./internal/modules/diva-cognitive ./internal/cognitivecontract -count=1` | exit 0; 30 top-level + 10 subtests |
| Consumer root | `just bootstrap-test` | exit 0; actual existing ensure-laputa/dependency-closure suite, 11 passed, 0 skipped |
| Consumer root | `git diff --check` and staged diff check | exit 0 |

Laputa `cmd/eino_smoke` and `evolution/testkit`, and consumer `internal/cognitivecontract`,
report `[no test files]`; these are package-only results, not skipped test cases.
The verification JSON contains exact package results/counts, command/cwd, raw output
paths/hashes and bootstrap output. The focused bound test has durable raw output;
full already-established gate logs remain in `/tmp/issue58-reconcile-*.jsonl`.

The first direct Node bootstrap invocation omitted the toolchain PATH, so `pwsh`
was not found: 2 dependency checks passed and 9 spawn-based bootstrap checks failed
with null child status. No product/bootstrap change was made. The corrected actual
`PATH=... just bootstrap-test` ran all **11/11** successfully. Initial failure and
corrected full output are recorded, not counted as skipped tests.

## Review, qualification and handoff

Self-review checked the full source/test/pin diff, exact six normal sums, module
Origin/full lock agreement, selected published-module resolution and all gate events.
No remaining in-scope issue was identified. Independent Task 4 review is still the
controller's next gate; this report is not its verdict. Parent-owned index, S2 review,
S3 plan and design clarification changes are excluded from these commits.

Task 3's independently approved producer evidence remains separate: its focused
reconcile suite passed 35 events; full ACTMEM race passed 114 with zero failed/skipped;
fault-seam crash/recovery coverage is synthetic, not actual power-loss testing. Task
4's full Laputa module run also passes existing legacy/v1/v2 tests. There is no new
native Windows durability evidence and no claim that cross-compilation qualifies it.

Native Windows, actual S3 provider integration, BML/Mentle integration acceptance,
issue #66, real PostgreSQL, prior broad MemoryLoop timeout resolution and final exact
candidate S6 whole-product CI/product acceptance remain pending. No unrelated broad
runtime/storage/full CI/high-count run, tenant-data access, push, merge or deploy was
performed. Passing a package that can use a mock is not actual-provider acceptance.

S3/S4 may consume this exact compatible C1 revision/published pin after the controller's
independent review and S2 Task 3/domain handoff. Compilation of an unknown cause is
not capability negotiation. Keep the exact pending request/operation/range/evidence
until a durable positive receipt; revision zero, visible coverage, unknown/busy or
unsupported negatives never permit retirement.

Schema identity remains `actmem/v3`; this semantic new cause requires a compatible
owner. Old strict readers reject it fail-closed (source-inspection conclusion, not a
separate old-binary run). Rollback disables extended admission while retaining all
heads/history and a compatible owner; never downgrade a reconciled head to an
incompatible owner or reactivate legacy writers. No receipt rewrite or migration.
