# T2 implementation brief — bounded history storage

## Dispatch

- Repository: ProjectViVy/agent-vivy
- Worktree: /workspace/scratch/370b2dd4ebcc/agent-vivy-session
- Branch: feat/issue-51-session-continuity
- Final-fix base revision: 145d5d91c899b50278d29b888f587793452f210f
- Story plan: docs/superpowers/plans/session-continuity/T2.md
- Report to write: .superpowers/sdd/2026-09-21-session-continuity/t2-report.md
- Required final-fix commit message: fix(storage): harden history metadata visibility

You are the fresh T2 implementer. Read the T2 Story plan, the shared Session Continuity contract ledger, the SC-D4 spec, the accepted T1 implementation and the current storage contracts before editing. Do not infer acceptance from this brief or from planning checkboxes. The supervisor will review your commit separately.

## Accepted predecessor and authority

T1 is accepted for implementation purposes at 9a1c5c1, with supervisor status recorded in 94fd4b9. T1 supplies bounded typed domain contracts, effective limits, HistoryScope authority and payload schemas. Its Go compilation/tests are explicitly unverified because the environment has no go binary; do not report them as passing.

The current main storage migration owner is already integrated. Both dialects end at 023_workspace_path.sql; migration 024 is the next available logical number. Use one paired, centrally discovered SQLite/PostgreSQL migration for this Story. Do not reserve or consume a number for T4.

## Required outcome

Implement a stable, bounded HistoryQueryStore on SQLite and PostgreSQL. History pages must be deterministic across equal timestamps, use a durable per-session message position, expose an opaque/keyset cursor and a consistent capture cut, and preserve current deletion/rewind visibility rules. Reads must never load unbounded payload data merely to enumerate history.

The capture cut must be obtained in one consistent read transaction and must contain the message-position ceiling plus the relevant run event-sequence ceilings. Later pages apply current visibility/deletion rules while never admitting rows inserted after the captured ceilings. Equal CreatedAt values are ordered by the durable position, not by a timestamp-only tie break.

Oversized or malformed raw records must become bounded unavailable/truncated metadata and advance the cursor; one hostile record must not starve progress. Enforce the scan ceiling of 2,000 records and 4 MiB. Limit+1 is only for has-more within that ceiling. Parameterize every source/session/run predicate before any payload read.

## Scope — permitted files and seams

Implement only the T2 boundary. The expected files are:

- internal/storage/history.go
- internal/storage/sqlite/history.go and history_test.go
- internal/storage/postgres/history.go and history_test.go
- internal/storage/conformance/history.go
- the existing SQLite and PostgreSQL messages.go, sessions.go and history_mutations.go files
- the existing centralized migration manifest and exactly one paired 024 migration

Modify internal/storage/contracts.go only if needed to declare HistoryQueryStore and expose it through the existing Engine/app composition surface. Keep the existing public storage abstractions compatible; do not refactor unrelated stores or create a second authority.

The position allocation invariant must cover every current message writer: AppendMessage, AppendMessageIfAbsent, CommitSessionEdit and CommitSessionFork. A deterministic duplicate projection returns the existing row without changing its position. Leave the future atomic admission writer as a reusable storage seam for T4, but do not implement T4 admission, receipts or workspace cleanup in this Story.

## Explicit exclusions

Do not implement or edit T3 tools, HistoryService, RPC dispatch or model-facing inspection; T4 ContinuityStore/admission, operation receipts, session_work_events or workspace rollback; T5+ reference/delivery/UI work; unrelated schema refactors; raw model-facing records; new dependencies; external push, PR, issue or deployment actions.

Do not replace the Journal as source of truth. Do not add inline DDL or a second migration registry. Do not use wall-clock timestamps as the cut boundary. Do not make ListMessages or legacy runtime behavior depend on a new unverified UI contract.

## Tests and evidence

Add real backend/conformance coverage, not comments or planned assertions. The shared helper must include the T2 plan sequence: two same-timestamp messages, capture, a later insert with an earlier timestamp, limit-one paging, duplicate detection, delayed assistant projection and a second session. It must assert the stable order m1,m2, exclusion of m3 from the captured cut, bounded BytesInspected and no duplicate IDs.

Also cover legacy migration/reopen and idempotency, source deletion between pages, rewind visibility, more than 256 run selections, cancellation, malicious source IDs, empty-but-scan-incomplete pages, oversized payload progress, the 2,000-record/4 MiB budget, SQLite/PostgreSQL parity and rebuild ordering. Validate the centralized manifest/checksum tests.

Run and record exact results for:

- go test ./internal/storage/... -run 'TestHistory|TestContinuityMigration' -count=1
- go test ./internal/storage/migrations/... -count=1
- go test ./internal/storage/sqlite ./internal/storage/postgres -run 'TestBackendConformance|TestHistoryConformance' -count=1
- git diff --check

If go, PostgreSQL or another prerequisite is unavailable, record UNVERIFIED with the exact command and failure/skip reason. A Postgres skip is not a pass. Do not weaken tests or claim compilation from static inspection.

## Handoff report

Write t2-report.md with: accepted predecessor revisions; changed-file list; migration number/name; HistoryQueryStore and cursor/cut semantics; every writer covered; test commands and exact results; unavailable prerequisites; known follow-up seams for T4; and confirmation that no excluded Story was implemented. Leave the worktree clean after the required commit.
