# S2 Task 1 independent review

## Spec Compliance

- ❌ Issues found: two Important defects prevent trusting immutable grant replay and fail-closed source admission. See I1 and I2 below.
- ⚠️ Real PostgreSQL execution and centralized CI remain pending, as explicitly reported in `docs/logs/2026-10-15-issue58-subject-binding/verification.md:24`. The new PostgreSQL harness skips without a DSN (`internal/storage/postgres/subject_history_test.go:14`); compilation and logical schema comparison do not establish database parity.
- ⚠️ The external #66 producer agreement and installed flow remain outside this increment (`internal/cognitivecontract/main_conversation.go:9`). Lossless tool/steering/partial repair and completing eligible shell remain Task2 responsibilities; their explicit unavailable/incomplete boundaries are consistent with the narrowed task and are not findings here.

## Strengths

- Stable identity uses the existing SnapshotStore absent-version CAS and rejects a changed canonical owner path (`internal/app/subject_binding.go:36`, `:49`). The existing factory callback carries SubjectID independently of ProfileID (`internal/app/assembly_cognitive.go:49`); no alternate owner or body store is introduced by this diff.
- Both backends share metadata transaction logic. Source registration and run pins are placed inside admission transactions, and head serialization precedes sequence allocation and index insertion (`internal/storage/subjectsql/subjectsql.go:69`, `:134`, `:207`). Paired append-only migration 044 contains metadata rather than transcript bodies (`internal/storage/migrations/sqlite/044_subject_history.sql:1`, `internal/storage/migrations/postgres/044_subject_history.sql:1`).
- The deletion guard executes before runtime tombstoning/cancellation and rechecks durable membership under projectionMu (`internal/runtime/service.go:766`, `:780`). Backend guards inspect the subtree before delete statements (`internal/storage/sqlite/sessions.go:179`, `internal/storage/postgres/sessions.go:191`).
- Text evidence checks completion hashes and refuses tool/steering evidence pending the designated repair task (`internal/storage/subjectsql/evidence.go:112`, `:122`). Read and duplicate-seal paths recompute the digest (`internal/storage/subjectsql/subjectsql.go:238`, `:354`).
- SQLite CAS is a single SQL statement, and the bounded retry helper restricts retries to typed BUSY/LOCKED while preserving cancellation and uncertain outcomes (`internal/storage/sqlite/snapshot.go:43`, `internal/storage/sqlite/subject_contention.go:17`). Added tests exercise real database handles, cancellation, transactional rollback, and actual Service.Run text evidence rather than only mocked return values (`internal/storage/sqlite/subject_contention_test.go:17`, `:73`, `internal/app/subject_admission_test.go:144`).

## Issues

### Critical

- None.

### Important

**I1 — An unchanged valid grant snapshot can fail every admission/read after its first persistence.**

- Evidence: `internal/storage/subject_history.go:66` accepts ordinary valid JSON objects, including whitespace; `internal/storage/subjectsql/subjectsql.go:101` marshals the binding, which compacts its `json.RawMessage` and applies JSON escaping. Loading at `:164` therefore reconstructs a different byte slice. Both `internal/runtime/subject_binding.go:55` and `internal/storage/subjectsql/subjectsql.go:96` compare that slice with the producer's original using `reflect.DeepEqual`.
- Example: a trusted producer consistently returns `Snapshot: json.RawMessage("{ \"decision\": \"allow\" }")`. First admission persists the compact object. The next admission, or `AuthorizeSubjectHistory`, rejects the very same producer decision with `ErrSubjectBinding`; a storage-level replay also fails. JSON containing literal HTML-sensitive characters has the same problem through escaping. No policy, version, or decision has changed.
- Fix: define one stable representation for the snapshot and normalize it consistently before capture, persistence, and comparisons, or preserve the original bytes losslessly. Keep rejection of genuinely changed decisions. Add a real first-admission/replay/read regression using a formatted JSON grant (and escaped characters), with the same producer snapshot across calls.

**I2 — Continuity availability incorrectly authorizes the non-atomic ordinary Run fallback for an eligible source.**

- Evidence: `internal/runtime/subject_binding.go:59` accepts an eligible source when `Continuity != nil` even if both `Admission` and `PrimaryRuns` are nil. In `internal/runtime/service.go:1377`, continuity persistence is selected only when `options.Continuity != nil`. A normal Run in that otherwise permitted composition reaches the fallback at `internal/runtime/service.go:1550`, which separately appends the message, creates the run, and appends run.started without consuming `admission.Subject` or registering retention/run pins.
- Impact: a headless/embedded composition exposing the continuity store but only the narrow ordinary storage interfaces can successfully execute a trusted eligible normal Run without the required atomic source protection or immutable grant snapshot. A restart without the resolver then has no persisted source membership to protect those originals. The new guard claims to reject precisely this unsupported admission situation but does not reject the actual selected path.
- Fix: validate the selected admission path before any side effect, or unconditionally reject a non-nil subject at the non-atomic fallback. Continuity is sufficient only for an actual continuity admission. Add a focused composition test with Continuity wired, ordinary atomic admission interfaces absent, and a normal eligible Run; require rejection before any message/run/event write (or route it through an atomic subject-aware path).

### Minor

- None.

## Assessment

**Task quality: Needs fixes.**

The overall identity/index architecture and scoped Task2 deferrals follow the brief. The two defects above affect the new immutable authorization and pre-terminal retention guarantees, so this increment should be corrected before acceptance; PostgreSQL and central CI remain separate outstanding gates.

## Review checks and scope

- Read the brief first, then handoff/report and the local Superpowers task-reviewer prompt. Reviewed base `aead1e8` to head `e1732e1` through all 52 files in bounded diff passes. One initially truncated documentation span was recovered with smaller ranges.
- The Service.DeleteSession and runWithAdmissionGate hunks ended mid-function. Read only their relevant continuation spans (`internal/runtime/service.go:760–880`, `:1360–1620`) to assess pre-tombstone ordering and which persistence branch actually consumes Subject. No whole-file reread or broad code crawl.
- One focused outside-diff check: risk that the new evidence loader/digest omitted message fields or misclassified file content as text. Inspected the `Attachment`, `FileContext`, and `Message` declarations in `internal/domain/session.go:115–159`; the loader covers those fields and file bytes remain `[]byte`. No additional finding from that check.
- No git commands, subagents, source edits, or test reruns. Reported tests were assessed as implementer evidence, not independently re-executed. Only this review report was written.
- Finding count: **0 Critical, 2 Important, 0 Minor**.
