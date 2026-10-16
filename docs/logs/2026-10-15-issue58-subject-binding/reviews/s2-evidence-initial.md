# Issue58 S2 Task2 independent review

Reviewed base `4e57f3ea0954ef20ad598b33245c06cc7fa1f119` through stable head `4a914f531b341202211e8df8764060190a16ae29` (implementation `2cbb9aefc5fe395a05664430e87071741017cd77`). Review is scoped to Task2, its accepted narrow rulings, and the supplied complete 6,075-line review package. It does not review the separate in-flight Eino dependency remediation.

## Verdict

**Spec compliance: changes required.** The principal retention, shell admission, immutable provenance, archive and evidence-failure requirements are implemented with substantial behavioral coverage. However, PostgreSQL's new exact-copy writer cannot execute its SQL, and the native question-answer success path does not satisfy the new evidence reader's mandatory receipt contract. These are source defects, separate from acknowledged environmental acceptance gates.

**Code quality/security/data integrity: changes required.** No Critical findings. Two Important findings. No Minor findings. Current-grant rechecks, fail-closed artifact integrity, original/private argument separation, and transactionally protected retention are sound in the reviewed paths. The two concrete integration defects below prevent accepting the increment as a complete producer.

## Strengths and requirement coverage

- Authorized raw tool result and error are captured at `toolAdapter.invoke` before hook/error conversion or model compaction; original requested arguments use deterministic existing BlobStore IDs. Versioned receipt expansion binds run/operation, artifact hash/length, original result/error hash/length and stable projection. Ordinary descriptor-looking strings remain opaque. Missing/corrupt result or argument artifacts deny continuation and do not reexecute tools.
- Governed shell admission uses the existing atomic run-admission transaction with source/run pins and protection before execution, without inventing a user message. `NoContext` remains excluded. The narrow trusted first-use exception verifies actual governed shell shape and absence of user/model activity; it does not infer authority from a title or UI face alone.
- Paired append-only 045 preserves existing 044 metadata/FKs and supplies nullable shell message coordinates plus exact copy-to-parent metadata. SQLite populated-upgrade, DDL rollback, reopen, FK protection and duplicate-content provenance fixtures directly exercise the claimed behavior. New nested copies do not rely on content matching; historical missing links are explicitly unavailable.
- Original readers validate deterministic Journal/message projection and completion hashes, expand attachments/file snapshots and tool originals, and keep rewind disposition separate from canonical evidence digest. Retained archive preserves addressability and hard-delete protection. Default ordinary tool behavior has an explicit negative control.
- Plan candidate capture preserves the native typed interruption and Work's mutation/dedup authority. Named revise/execute-once restart tests assert a single submission/decision and final seal. Missing `tool.finished` can be repaired from a completed exact receipt with `journal_partial` provenance.
- The final prefix-gap guard validates other terminal unsealed sources inside the head-allocation transaction. Its conformance test shows a later seal rejected at head zero, repair allowing sequences 1 and 2, and idempotent retry. Active and unpinned activity are outside this scan. Already sealed retry handling precedes new allocation. This closes the carried final self-review risk at the source level.
- Automatic terminal sealing was moved after active-run cleanup while preserving the existing projection lock; ordinary no-resolver runs bypass the additional source read. The final reported diagnostic concurrency run includes existing cancellation idempotence.

## Critical

None identified.

## Important

### I1 — PostgreSQL forks/clones use SQLite placeholders on a raw SQL transaction

**Location:** `internal/storage/postgres/history_mutations.go:119` and `:122`; transaction construction at `:86`.

**Evidence:** `CommitSessionFork` calls `b.db.SQL.BeginTx`, producing a native `*sql.Tx`, and the surrounding existing queries use PostgreSQL `$1`, `$2`, etc. The new parent verification uses `WHERE session_id=? AND id=?`, followed by `VALUES(?,?,?)` for the copy-source insert. These calls bypass the backend's placeholder-translating DB/Tx wrapper. The runtime fork producer now supplies `CopySource` for every copied message, so normal nonempty forks and clones enter this branch.

**Consequence:** PostgreSQL rejects the new statements and rolls back the fork/clone transaction. Exact copy provenance and existing fork/clone functionality are broken on that backend. The registered PostgreSQL conformance tests were skipped, so compilation and the green SQLite gate do not cover this defect. This is a concrete source error, not a finding inferred merely from the missing PostgreSQL acceptance gate.

**Smallest correction:** Use `$1,$2` and `$1,$2,$3` in these two raw-transaction statements, retaining the existing transaction/rollback boundary. Keep the new backend conformance test and execute it against a legitimate PostgreSQL DSN when available; add a focused placeholder/driver-level regression if the source fix must precede that environment.

### I2 — Successful native question answers never receive the receipt required for sealing

**Location:** `internal/runtime/tooladapter.go:735–740` and `internal/storage/subjectsql/evidence.go:259–260`; amplification through `internal/storage/subjectsql/subjectsql.go:471`.

**Evidence:** The native `ToolInteractionQuestion` branch returns `answer, nil` directly from Eino's resume context before the operation-admission block at `tooladapter.go:742`. It does not call the retained outcome coordinator. The new `EventToolFinished` evidence branch rejects a successful result whenever `toolOperation` finds no receipt (`else if p.Error == "" { return ... ErrSubjectEvidenceIncomplete }`). Thus the existing supported native answer path cannot produce the exact receipt/artifact now required by the Task2 reader. The new retained Plan and shell tests do not cover retained question/answer resume.

**Consequence:** A retained run can resume successfully with a human answer and finish, but exact evidence validation and sealing reject it. Once terminal, that run also blocks later same-domain seals through the new prefix-gap guard. For a long answer, normal enhanced-result projection may shorten the returned text without the exact retained result reference. This violates completion of exact call/result evidence and preservation of existing interactive tool capability; treating the successful answer as an error or weakening artifact validation would not satisfy the requirement.

**Smallest correction:** Adapt the existing native question-resume boundary to retain the exact original request and returned answer through an owned durable evidence seam before model projection, without reinvoking a tool or replacing Eino's interruption semantics. Either complete an authorized receipt at that boundary or explicitly resolve the already-owned exact question/answer authority in the evidence reader. Add one retained question suspend/restart/resume case (including an over-budget answer), assert exact expansion, native resume behavior and one seal, then assert that a subsequent interaction can advance the same domain head.

## Minor

None identified.

## Actual review checks and evidence assessment

1. Read `task-2-brief.md` first, then `task-2-review-handoff.md`, the complete implementation report, and the producer `task-2-handoff.md`.
2. Read the supplied complete diff in bounded passes, including all 40 implementation/test/migration files, documentary changes, verification manifest, and both raw race reports. Oversized raw-log output was reread in bounded decoded-output passes to recover truncated portions. No git diff recomputation or broad repository crawl was performed.
3. One focused current-grant check read `resolveSubjectAdmission`: the concern that `SubjectRecent` only checks an error is cleared, because an existing retained source whose current resolver returns nil or changed pins causes `ErrSubjectBinding`. It does not silently accept revoked membership.
4. One focused PostgreSQL transaction-seam check established that the fork writer uses `b.db.SQL.BeginTx` rather than the translating wrapper. No PostgreSQL server test was run or claimed.
5. A bounded question-path lookup plus the supplied adapter/evidence hunks establish I2's direct return before receipt admission. An attempted lookup of guessed question fixture filenames found no such files; it was not treated as evidence of missing functionality. A separate focused cancellation-path read was not expanded into a finding because its downstream settlement contract was not fully inspected.
6. No reported test suite was rerun. No new execution test was added or claimed. Source files were read only; this review report is the sole written artifact.

The implementation report's final focused gate records Runtime 39, Storage 1, migrations 2 and SQLite 24 passing, with PostgreSQL 8 skipped. Its full runtime/storage gate records Runtime 1130 passing/1 skipped, Storage 21 passing, migrations 40 passing/5 skipped, SQLite 209 passing, and PostgreSQL 1 passing/36 skipped. The app Issue58 gate records 14 passing. These are reported terminal Go JSON test-event counts, not newly reproduced review results. The manifest and report explicitly place the final non-race gates after the prefix/copy/cleanup corrections.

The required race output shows Eino v0.9.13 `compose/tool_node.go:1253` writes in two enhanced converters. The supplied existing unretained Plan baseline reproduces that upstream location. The required gate is **FAILED** (Runtime 47 pass/2 fail, SQLite 21 pass); the diagnostic exclusion run is additional localization only (Runtime 52/SQLite 24 pass), not replacement acceptance. The separate dependency-remediation writer and any later in-flight result are outside this stable review.

## Outstanding acceptance and integration gates

- Fix I1 and I2 and independently review the focused corrections with appropriate behavioral evidence.
- The required race/concurrency gate must pass on the separately reviewed official native fix and final candidate. This Task2 review does not convert the known upstream failure into a source finding or declare it resolved.
- Real PostgreSQL execution/parity, native Windows checks, live provider/product acceptance, the #66 producer contract and final LIFE HTTP/UI integration remain pending. Centralized S6 CI/artifact acceptance remains pending. Green package results and skips cannot close these gates.
- `RepairSubjectHistory(ctx, sid)` at `internal/runtime/subject_history.go:47` is a **per-session repair primitive**, not demonstrated domain-wide restart closure. If archived authorized source A has a terminal unsealed interaction after restart, repairing B or reading B's sealed index does not discover A. The prefix guard prevents advancement over incomplete A, but does not itself repair or index A. The controller's S3 admission/S5 observer handoff must provide current-authorized domain pending-run discovery/repair before capture, including archived/main sources. Prefer a bounded metadata query over scanning all sessions on every turn. The existence of `ListSessionsForRecovery` supplied by the controller is an integration seam, not proof this task wires it. This is carried as a producer/consumer readiness dependency, not an unrelated Task2 observer rewrite request.
- Historical copies without exact metadata intentionally remain unavailable. Missing/corrupt originals require authorized repair; rollback must stop new admission/sealing while retaining schema, protected sessions and original evidence.
