# T2 task review — FAIL

Review target: 76cdf6018e46fe32e423a1b6574af6bbd957f023
Base: f2099a6
Reviewer: independent task reviewer (Epicurus); review was interrupted after full file inspection, before a formal report file was written.

## Verified findings

1. Critical — SQLite metadata byte count uses length(content) in internal/storage/sqlite/history.go. The messages column has BLOB affinity, but the existing writer passes Go Message.Content as a string; SQLite can therefore store TEXT, and length(TEXT) stops at an embedded NUL. A 5 MiB string beginning with NUL reports zero while CAST(content AS BLOB) reports 5242880. This can bypass the 4 MiB pre-allocation budget.
2. Critical — hidden rewind ranges are materialized into a map by looping every position between anchor positions in both backend history.go files. A large retained range can allocate without the 2,000-record/4 MiB scan budget; the loop also lacks an explicit context check.
3. Important — custom ContinuityLimits are not capped to the mandatory storage budget; metadata fetch uses CandidateRecords+1 before the loop, so a caller can request an oversized scan envelope.
4. Important — when remaining CandidateBytes is less than a normal candidate, the page emits unavailable/truncated and breaks, so it may not continue scanning to a later bounded record even though the contract requires progress through a bounded scan.
5. Important — captured run event ceilings are stored but QueryHistoryPage rejects run-event positions and only pages message positions. The storage surface does not yet make the run-event cut effective; verify whether T2 must implement run-event candidates or explicitly narrow the contract without claiming the seam is complete.
6. Evidence gap — shared conformance registration exists, but the requested TestMigration|TestConformance command does not match the actual test names; Go is unavailable, so compile/runtime proof remains UNVERIFIED.

## Finding requiring semantic resolution

The reviewer flagged position-based rewind visibility versus the prior created_at/id ListMessages order. SC-D4 explicitly requires backend insertion order as the visibility boundary, and the T2 plan requires message positions, so position-based visibility is currently the intended direction. The fixer must state this resolution and ensure late projections after a marker remain visible by position, while still fixing the unbounded interval expansion.

## Verification limits

Go tests could not run because go is not installed. PostgreSQL DSN is not configured. `git diff --check` passed for the implementation commit. No acceptance is granted.

## Fix round evidence

The shared-worktree fix round addressed every verified finding:

1. SQLite message metadata now uses `length(CAST(content AS BLOB))`; the
   shared fixture stores an embedded-NUL message larger than 4 MiB and asserts
   unavailable/truncated output, cursor progress, and an inspected-byte count
   no greater than 4 MiB.
2. Both backends removed per-position hidden maps. Message metadata computes
   visibility with a parameterized SQL `EXISTS` predicate over cutoff/tail
   positions, so a long hidden range consumes constant Go memory per scanned
   candidate. SQL calls and scan loops propagate context cancellation.
3. Storage-owned hard caps clamp candidate work to 2,000 records and 4 MiB,
   while preserving smaller positive limits. Both message and run-event
   metadata queries receive only the effective record cap plus one.
4. Remaining-byte exhaustion emits bounded unavailable metadata for the
   current visible record, advances `Next`, sets `ScanIncomplete` and
   `HasMore` when later metadata exists, and allows the next page to return a
   later record under a fresh byte budget.
5. SQLite and PostgreSQL now page run events by `(run_id, seq)` under captured
   per-run ceilings. Conformance inserts events 1 and 2, captures, appends
   event 3, pages at limit one, and asserts only 1 and 2 appear without
   duplicates. Current rewind/edit visibility is applied through the same SQL
   range semantics when a run has a hidden projected message. Oversized and
   malformed event payloads remain unavailable and never expose raw content.
6. The added cases run under each backend's `TestHistoryConformance`, which is
   selected by the T2 `TestHistory` regex. The earlier claim that
   `TestMigration|TestConformance` selected backend and migration-owner gates
   was incorrect. The corrected backend selector is
   `TestBackendConformance|TestHistoryConformance`; migration owner checks use
   an unfiltered `go test ./internal/storage/migrations/... -count=1`.

Semantic resolution: SC-D4's backend insertion-position rule is authoritative.
Rewind/edit visibility is the closed position interval between marker anchors;
a later insertion after the marker tail remains visible even when its
`created_at` value is earlier. Shared conformance now asserts that behavior.

The final-fix evidence commands were attempted after the test changes:

- `go test ./internal/storage/... -run 'TestHistory|TestContinuityMigration' -count=1` — UNVERIFIED; exit 127, `/bin/bash: line 1: go: command not found`.
- `go test ./internal/storage/migrations/... -count=1` — UNVERIFIED; exit 127, `/bin/bash: line 1: go: command not found`.
- `go test ./internal/storage/sqlite ./internal/storage/postgres -run 'TestBackendConformance|TestHistoryConformance' -count=1` — UNVERIFIED; exit 127, `/bin/bash: line 1: go: command not found`.

Runtime and PostgreSQL parity therefore remain UNVERIFIED. The prior incorrect
regex is not treated as passing or selecting evidence. `git diff --check`
passed during the final-fix round.
