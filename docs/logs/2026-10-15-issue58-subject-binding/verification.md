# Verification

Environment: Go 1.26.4 via
`PATH=/workspace/scratch/243d1cbb859f/toolchains/bin:$PATH`.
Base: `aead1e86fbf6e5bde9bf27bc3bbad72a8be30555`.
Counts below are Go JSON terminal test events, including subtests/group events.
Every command below returned exit 0 on its final run.

| Command | Result |
|---|---|
| `go test ./internal/storage/... ./internal/app ./internal/rpc -run '^TestIssue58' -count=1 -json` | Storage 1, migrations 1, SQLite 19, App 9, RPC 10 passed; PostgreSQL 4 skipped |
| `go test ./internal/storage/migrations ./internal/storage/sqlite ./internal/storage/postgres -count=1 -json` | Migrations 39 passed/5 skipped; SQLite 204 passed; PostgreSQL 1 pure test passed/32 server tests skipped |
| `go test ./internal/runtime -run '^(TestDeleteSession\|TestRunShell\|TestShellAvailability\|TestShellStateReference\|TestShellApprovalHash\|TestRecoverRejectsCrossRunShell\|TestRecoverGarbageCollectsTerminalShell)' -count=1 -json` | Runtime 23 passed; ordinary delete, shell execution/recovery and NoContext behavior covered |
| `go test ./internal/app -run '^(TestEinoImportsQuarantined\|TestNoReferenceMaterialDependencies\|TestPluginWindowCannotImportInternal)$' -count=1 -json` | 3 passed |
| `git diff --check` | Passed |

The final focused and runtime gates ran after the pre-tombstone deletion fix.
The storage/migration gate ran before that runtime-only fix; its storage code and
SQL are unchanged, apart from comment/import formatting. The mandatory storage
and focused gates were not replaced with a raw root `go test ./...`.

PostgreSQL has no legitimate `VIVY_POSTGRES_TEST_DSN` here. The four Issue58 suites
(order/dedup, isolation, admission boundaries, continuity replay) are attached to
the real PostgreSQL harness but SKIPPED. All 32 PostgreSQL server tests and five
PostgreSQL released-cron migration cases skipped. Only the DB rebind pure test
ran in that backend package. Static 044 logical parity, real SQLite fresh install,
23/43 upgrades, reopen/reapply and injected failure rollback ran. No claim of real
PostgreSQL migration/transaction parity is made.

## Executable red and green evidence

The same focused commands are reproducible by their test names. Initial missing
APIs were made compilable with minimal unarmed/absent consumers, then behavior
was observed failing before implementation. An initial WaitIdle bool assertion
compile error was corrected before the singleton behavioral red run; it is not
counted as a useful red assertion.

- Identity: `go test ./internal/app -run '^TestIssue58Subject' -count=1`
  failed both initial identity cases with `storage: subject binding mismatch`;
  stable restart/presentation and canonical owner-path cases are now green.
- Index: `go test ./internal/storage/sqlite -run '^TestIssue58' -count=1`
  initially failed `backend does not implement trusted subject metadata index`;
  the same suite now proves contiguous same-time ordering, idempotency, isolation,
  retention and fixed pagination ceilings.
- Singleton: `go test ./internal/app -run '^TestIssue58SingletonAdmission$' -count=1`
  failed `storage: not found` when the admission consumer returned no binding;
  it now validates default main policy, ordinary contribution isolation,
  conversion denial, missing resolver and changed-grant rejection.
- RPC: `go test ./internal/rpc -run '^TestIssue58ClientSubjectClaimsRejected$' -count=1`
  failed all nine claim subcases with `client authority claim accepted`; all now
  return InvalidParams before create/turn mutation.
- CAS: `go test ./internal/app -run '^TestIssue58SubjectIdentityConcurrentFirstRegistration$' -count=1`
  failed a real two-handle first registration with
  `storage: snapshot CAS "organism/subject/v1": database is locked (5) (SQLITE_BUSY)`;
  single-statement CAS plus bounded backend contention handling now converges.
- Cancellation: `go test ./internal/storage/sqlite -run '^TestIssue58SnapshotContentionCanceled$' -count=1 -v`
  with driver 5000 ms wait returned deadline exceeded after 5.019632932s and failed
  `driver ignored prompt cancellation`. The final driver 25 ms/local-retry path
  returned deadline exceeded after 56.710668ms and passed. Timeout configuration
  on a replacement connection is also asserted. These measurements are test
  observations, not performance promises.
- Replay: `go test ./internal/storage/sqlite -run '^TestIssue58ContinuityReplayBinding$' -count=1`
  failed `continuity replay accepted changed grant: <nil>`; revalidation before
  receipt replay now rejects the changed snapshot.
- Shell: `go test ./internal/app -run '^TestIssue58ShellFirstEligibleFailsBeforeSideEffects$' -count=1`
  failed with the wrong generic `runtime: governed shell is unavailable` before
  the subject boundary existed; the explicit subject-unavailable boundary now
  runs before workspace allocation, run creation or execution.
- Exact digest: `go test ./internal/storage -run '^TestIssue58EvidenceDigestRejectsInvalidText$' -count=1`
  failed `invalid UTF-8 was silently normalized by digest: <nil>`; invalid text
  is now rejected rather than hashed after JSON byte substitution.
- Delete: `go test ./internal/app -run '^TestIssue58RetainedDeleteDoesNotTombstoneSingleton$' -count=1`
  failed both before-admission delete (`<nil>`) and after-admission usability
  (`rejected deletion tombstoned main: storage: not found`); both now pass,
  including durable protection without a restarted resolver.

Additional evidence checks cover complete Service.Run text seal/read with equal
digests; missing assistant projection; shortened tool evidence; unfinished deltas;
excluded sources; attachment-byte mutation; retained archive/read; foreign
subject/domain/count denial; malformed child/strategy/restricted bindings;
transaction rollback of source registration; same-conversation internal segments;
and a real two-handle eight-seal race. A deliberate mutation disabling the replay
digest comparison made TestIssue58SealedSourceTamperRejectedOnRetry fail with
`idempotent retry accepted changed original: <nil>`; restoring the comparison
made it pass. This mutation check supplements, rather than represents, test-first
implementation of that branch. Permanent/unknown/CAS/uncertain/context errors are
asserted to run only one attempt.

## Eino and remaining gates

Inspected pinned Eino 0.9.13 `adk/runner.go` (`RunnerConfig.CheckPointStore`) and
`compose/checkpoint.go` (`CheckPointStore`, `WithCheckPointStore`). They checkpoint
engine execution, not core organism identity, Host grants, retained source
membership or subject-serialized SQL sequence allocation. A thin core storage
and existing Service.Run admission adapter is required by Vivy's Journal and
policy invariants; no custom model/agent loop or Eino type escapes were added.
Existing Eino import quarantine and ordinary shell runtime tests pass. S3 retains
the separate authoritative summarization capability check.

The controller owns independent review, full `just ci`, DIVA Recipe/pack and
artifact verification. Those were intentionally not run from this dev worktree
(the known Go 1.26 .git-file stamping issue is not bypassed). No browser/UI change
or #66 producer is included, so no split UI smoke is claimed. S2 Task2 lossless
tool/steering/interrupted-delta and direct-shell admission remains pending.


## Accepted review fixes, round 1

Fix base: `0813109ddc64c7b70139687a93d7acbdbdc0c775` (C1 documentation atop
Task1). Only I1/I2 and their regressions changed; no schema, dependency pin,
Laputa, UI or Task2 changes. These focused runs are intermediate local-source
evidence, not final exact-pin integration acceptance.

- I1: `go test ./internal/storage/sqlite ./internal/app -run '^TestIssue58FormattedGrant' -count=1 -v`
  initially failed storage replay with `unchanged formatted/HTML grant replay
  rejected: storage: subject binding mismatch` and authorized read with
  `unchanged formatted/HTML grant rejected after persistence: storage: subject
  binding mismatch`. Both now pass, including genuine changed-decision denial.
  SameSubjectSourceBinding compares the JSON representation used for persisted
  metadata, so RawMessage formatting/HTML escaping cannot change the grant on
  reload. No user/original evidence normalization was introduced.
- I2: `go test ./internal/app -run '^TestIssue58ContinuityOnlyCompositionUsesSelectedAdmission$' -count=1 -v`
  initially allowed the normal eligible Run and recorded 2 messages, 1 run,
  6 Journal writes and 1 provider call. It now rejects before any such side
  effect. Its two controls pass: ordinary unshared Run retains the fallback;
  an actual continuity request commits the subject pins through ContinuityStore.
  The guard now checks the selected normal admission branch at Service.Run.
- Covering gate: `go test ./internal/storage/... ./internal/app ./internal/rpc -run '^TestIssue58' -count=1 -json`
  returned exit 0: storage 1, migrations 1, SQLite 20, App 14 and RPC 10 passed;
  PostgreSQL 5 skipped without a test DSN. Counts include subtests/group events.
- Relevant conformance: `go test ./internal/storage/sqlite -run '^TestBackendConformance$/^(CN-03|CN-33|CN-34)' -count=1 -json`
  returned exit 0: Snapshot expected-version CAS, atomic Goal and atomic first
  primary admission all passed (4 terminal test events including the parent).
  `git diff --check` passed. Unchanged broad migration/CI gates were not repeated.

Tests: internal/app/subject_admission_test.go (formatted grant, real first Run,
authorized exact read, next admission and changed-grant rejection),
internal/app/subject_admission_path_test.go (real narrow continuity-only
composition, write/provider observations and controls), and shared
internal/storage/conformance/subject_history.go (first persistence/replay and
changed-decision rejection), wired into both backend subject_history_test.go
files. PostgreSQL remains pending; its harness registration is not a server pass.

Dependency evidence: root pins remain at
`v0.0.0-20261015094757-5f26de678a99`. Existing local replacements consumed the
reviewed recovery tree `0d728106f295dd28db2dc86d446af186cc43e76a`, initially
local commit `853ced6b41feda072d67c229fc95fa9512f15b6a`, then metadata-aligned
published commit `4c7c6b20859375d0d764d3ba2a0188926cf73c9e`; both commits have
that identical source tree. Final focused/conformance runs used the latter.
No ensure-laputa/bootstrap/just ci or source-owner operation was run. Parent owns
normal pin coordination, independent scoped re-review and final exact-pin CI.


## Task2 verification (BASE 4e57f3ea0954ef20ad598b33245c06cc7fa1f119)

Go 1.26.4 and normal checksums; Eino remains v0.9.13 and published Laputa remains 4c7c6b20859375d0d764d3ba2a0188926cf73c9e. Counts are terminal Go JSON test entries, including subtests/group entries. The exact commands, source hashes, actual before/after manifests, all skip names and output hashes are in `task-2-verification.json`.

| Final command | Actual result |
|---|---|
| `go test ./internal/runtime ./internal/storage/... -run '^TestIssue58' -count=1 -json` | Exit 0: Runtime 39, Storage 1, migrations 2, SQLite 24 passed; PostgreSQL 8 skipped |
| `go test ./internal/runtime ./internal/storage/... -count=1 -json` | Exit 0: Runtime 1130 passed/1 skipped, Storage 21 passed, migrations 40 passed/5 skipped, SQLite 209 passed, PostgreSQL 1 passed/36 skipped |
| `go test ./internal/app -run '^TestIssue58' -count=1 -json` | Exit 0: 14 passed |
| Required race/concurrency subset including all Issue58 and existing projector/operation/cancel coverage | Exit 1: pinned Eino converter data race in retained Plan review; exact output in `task-2-required-race.jsonl` |
| Existing unretained `TestPlanGoalProbeSubmissionFencesLaterToolInSameBatch` under `-race` | Exit 1: same upstream data race; exact output in `task-2-upstream-baseline-race.jsonl` |
| Diagnostic race subset with only that Plan scenario excluded, plus all relevant new source/copy tests, existing projector/crash/approval/cancel checks | Exit 0: Runtime 52, SQLite 24 passed. Diagnosis only; does not substitute for the failed required race gate |
| `git diff --check` | Passed |

The full final gate ran after the prefix-gap guard, exact copy metadata and ordinary terminal cleanup adjustment. Fresh install, populated-044 upgrade to 045, restrictive child FKs, rollback after real DDL, `foreign_key_check`, reopen/reapply, exact source archive and ordinary parent deletion all ran in SQLite. Paired PostgreSQL code and conformance compile; real PostgreSQL execution is SKIPPED without VIVY_POSTGRES_TEST_DSN, not a parity pass. The sole runtime skip is `TestReportFeedbackSnapshot` (its external GitHub gating remains unchanged).

Meaningful RED assertions preceded the corresponding fixes: full output disappeared before compaction (3 initial required-gate failures including shell and NoContext); populated044 could not admit a shell without a fake user; raw partial result was discarded on tool error; invalid UTF8 became replacement text; completed receipt without tool.finished omitted known terminal output; enhanced media lost its image/reference; post-invoke Plan interrupt stranded a claimed operation; excluded shell disabled first singleton use; later seal advanced past missing evidence; NoContext source admission was accepted; missing original-argument artifact allowed replay continuation; and missing exact copy writer returned no parent identity. The exact-copy RED armed the new read/schema seam with its producer absent, then restored the atomic producer. A mistaken equal-time cutoff/count and test API typos were corrected and are not counted as behavioral RED evidence.

A pending-shell test also exposed a connection-pool deadlock: eligibility lookup occurred after opening the one-connection Journal iterator. The lookup now precedes Replay and the original cancellation test is green. A diagnostic ordinary-cancel failure exposed seal I/O between terminal status and active cleanup; sealing moved after existing cleanup (still under projection lock) and unshared runs without trusted subject resolvers bypass it. The final diagnostic cancel/concurrency gate passes.

Pinned Eino source evidence: compose/tool_node.go:1253 writes captured outer `err` from two enhanced stream converters (`ret[index].UserInputMultiContent, err = tr.ToMessageInputParts()`). Both race reports identify only upstream writes at that line. No dependency patch/vendor, serialization restriction, race suppression or capability removal was made. The controller owns a separately scoped official native-fix pin review. No unrelated root `go test ./...`, `just ci`, live model call, browser/tenant smoke, push or deployment ran; centralized candidate CI remains S6 per dispatch.

## Official v0.9.20 final-pin amendment

The historical Task2 v0.9.13 failures above remain accurate for their tested pin.
At official v0.9.20, the exact required race gate passes with retained Plan review
intact; the same unretained Plan submission baseline passes. Final runtime/storage
non-race coverage is 1401 pass / 42 skip, with real PostgreSQL and the existing
external GitHub prerequisite still unavailable. Focused Issue58 runtime/storage/app/
RPC coverage also passes. The broader app/persona compatibility check initially
exposed an unchanged old/new-pin nil SnapshotStore fixture failure; the authorized
real-store fixture correction preserves product behavior and original assertions.
All exact commands, raw-output hashes/counts/skips, final corrected compatibility
results, official module provenance and limits are in
[eino-upgrade-report.md](eino-upgrade-report.md) and `eino-upgrade-verification.json`.
No raw root `go test ./...`, just ci, installed product/provider/Windows acceptance,
push, merge, deployment or tenant journal operation was performed by this increment.

The corrected broad app race diagnosis returns exit 1 (31 pass / 1 fail): the
existing MemoryLoop live-delete RPC deadline expired once during overlapping
runs. The exact case passes when rerun alone after the runs drain; the failed
command remains failed. Final corrected persona authority, native cancellation/
checkpoint and codeface consumer checks pass. Timing cause is unproven; the
separate candidate-bound DIVA Recipe gate remains pending, with the open
`TFLAKE-MEMORYLOOP-LIVE-DELETE` record in docs/TODO.md.


## Task2 fix round1 verification (BASE 15e79419ccfa302ad3f8330fe72448c88796a5fb)

Meaningful Question RED: real suspend, DB reopen, native recovery and successful answer reached terminal with no completed exact receipt; ordinary control passed. The early test fixture omitted the existing five-minute interaction expiry and failed at recovery; that setup failure was corrected and is not counted as behavioral RED. The narrow retained-only answer receipt then made the exact same regression green (3 test entries). Complete RED/GREEN outputs are task-2-fix-1-red.jsonl and task-2-fix-1-green.jsonl.

Final unchanged required race regex passes at Eino v0.9.20: Runtime58 and SQLite24 entries (82 total), including retained Plan plus the new Question. Exact output: task-2-fix-1-required-race.jsonl. Targeted existing native Question/Plan/interrupt/cancellation race regressions pass19 entries, including the former unretained Plan race baseline. Focused Issue58 runtime/storage/App/RPC passes93 entries (Runtime42, storage1, migrations2, SQLite24, App14, RPC10) with8 PostgreSQL skips. Full runtime/storage passes1404 entries (Runtime1133, storage21, migrations40, SQLite209, PG1) with42 skips: Runtime1 external GitHub, migrations5 PostgreSQL, PostgreSQL36. git diff --check passes. Exact commands, output hashes, fixture manifest and all skip names are task-2-fix-1-verification.json.

The two raw PostgreSQL statements use $1/$2 and $1/$2/$3; existing shared backend copy provenance/rollback conformance remains registered. No real PostgreSQL DSN is present, so PG compilation/skips are not parity acceptance. Native Windows, live provider/product acceptance, #66, HTTP/UI integration and final just ci stay pending at S6 per dispatch. No dependency pin, current snapshot constants/Persona fixture, parent plan/index, tool capability or Eino converter was changed by this fix.
