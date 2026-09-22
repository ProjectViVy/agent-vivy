FAIL — T2 scoped fix-round re-review

Review date: 2026-09-22. Original implementation: `76cdf6018e46fe32e423a1b6574af6bbd957f023`. Fix: `145d5d91c899b50278d29b888f587793452f210f` (also inspected HEAD). Original parent: `f2099a6`. Accepted T1 predecessor: `9a1c5c1`, with acceptance recorded in `94fd4b9`.

Independent inspection covered the original implementation and its direct storage/writer/migration/Journal context, the complete fix range, T2 plan and brief, SC-D4, the shared contract ledger, accepted T1 contracts, and the original FAIL evidence in `t2-review.md`. The appended fix claims and `t2-report.md` were treated as assertions to verify, not acceptance evidence.

No production code, tests, plans, or existing evidence were modified. No subagents, commits, external writes, or production databases were used. This report is the only file written.

## Findings

No Blocker finding is established. The Important findings below prevent acceptance; the final Minor finding is non-blocking by itself.

### R1 — Important: malformed UTF-8 event payloads are returned as available raw text

Locations: `internal/storage/sqlite/history.go:243`; `internal/storage/postgres/history.go:241`.

Both event paths mark a candidate available and copy `string(body)` whenever `json.Valid(body)` succeeds. Unlike the message paths, neither checks `utf8.Valid(body)`. A payload with bytes `7b 22 78 22 3a 22 ff 22 7d` (an object containing a quoted raw `0xff`) passes the standard encoding/json syntax scanner but is invalid UTF-8. The writer accepts arbitrary payload bytes, so this does not require bypassing Journal.Append. BLOB/BYTEA can retain them. The raw malformed body is returned with `Unavailable=false` and `Truncated=false`.

The [official Go scanner source](https://go.dev/src/encoding/json/scanner.go) confirms that `Valid` invokes the byte scanner; `stateInString` rejects unescaped control bytes below `0x20` but does not validate UTF-8 sequences. This is a source-based finding, not a Go execution result.

Contract violated: T2 brief's malformed-record-to-bounded-unavailable requirement and this re-review's explicit malformed UTF-8/JSON gate. Correct syntax rejection of `{"broken":` does not cover this case.

Minimal fix direction: require both UTF-8 validity and JSON validity before returning event Text, retaining empty-text unavailable/truncated metadata otherwise. Add the invalid-byte-inside-quoted-string fixture followed by a valid event to the shared suite; assert flags, empty Text, and continuation on both backends.

### R2 — Important: metadata row count is capped, but metadata byte allocation is not

Locations: `internal/storage/sqlite/history.go:275`, `:305`, `:334`, `:368`; `internal/storage/postgres/history.go:275`, `:306`, `:337`, `:372`.

The metadata SELECTs retrieve entire message IDs/run IDs/roles and event types into Go strings before the candidate loop and before any byte check. `CandidateRecords+1` caps slice cardinality, not these variable-length values. The original schemas use unrestricted TEXT for these columns; Journal.Append only checks commit/terminal conditions and does not bound `EventType` (`internal/storage/sqlite/journal.go:18`, `:65`; PostgreSQL `journal.go:18`, `:69`). Message insertion likewise does not impose a role/ID byte bound.

A run event with a 5 MiB `Type` and two-byte `{}` payload is accepted by this storage seam. Its metadata row alone transfers 5,242,880 bytes while the recorded candidate payload size is 2. It can be the extra lookahead row, so it is fetched even when `CandidateRecords=1` and the loop never inspects that event. An in-memory SQLite SQL probe confirmed the row-width counterexample. The corresponding Go `rows.Scan` necessarily receives that complete string; PostgreSQL has the same unbounded SELECT shape. This is not a claim of measured Go heap usage or a demonstrated public-model entry point.

Contract violated: the T2 bounded enumeration/pre-allocation invariant, specifically the requested gate that cap+1 metadata must not cause unbounded Go allocation. The content/payload fixes do not close this separate allocation route.

Minimal fix direction: bound variable-width metadata before it crosses the SQL/driver boundary, including lookahead and captured identifiers. Use bounded/safe projections or a bounded explicit error for corrupt identifiers; never truncate an identity into a different valid cursor. Preserve unavailable metadata for malformed non-key fields. Add a small-record-budget/oversized-lookahead-metadata regression. Writer validation alone is insufficient for retained malformed rows.

### R3 — Important: one hidden projected message hides every event in its run

Locations: `internal/storage/sqlite/history.go:336`–`:344`; `internal/storage/postgres/history.go:339`–`:347`.

The event hidden predicate correlates `projected.run_id = e.run_id`, but not the event sequence or its own projection. Therefore a single hidden message makes the predicate true for all events of the run, including events whose projected messages are outside the marker interval.

Concrete counterexample: run R has completion events 1 and 2 with their normal deterministic projected message IDs (`internal/runtime/message_projector.go:138`, `:193`–`:201`), at positions 1 and 2. A rewind hides only position 2. The message query correctly retains position 1. The event query nevertheless marks both event 1 and event 2 hidden. The in-memory SQLite probe returned message hidden flags `(1,false),(2,true),(3,false)` (position 3 was a post-marker earlier-timestamp insertion), but event hidden flags `(1,true),(2,true)` under the captured seq=2 ceiling. PostgreSQL uses the same predicate. The shared fixture at `internal/storage/conformance/history.go:254` tests only a fully hidden one-message run and cannot detect this.

Contract violated: SC-D4's record visibility, the closed-range/complement-of-union contract in `internal/storage/contracts.go:397`–`:403`, and T2's consistent message/event visibility requirement. Whole-run suppression is an additional semantic choice, not a consequence of insertion-position ordering.

Minimal fix direction: make event visibility respect the existing event-to-projection relationship and marker interval, without a timestamp boundary or a second registry. Where legacy provenance cannot be resolved, expose an explicit bounded unavailable outcome rather than silently treating every event as hidden. Add a partially rewound multi-event run and post-tail projection case to both backend conformance registrations.

### R4 — Important: the second required test regex still selects neither conformance nor migration-owner gates

Locations: `internal/storage/sqlite/conformance_test.go:12`, `:33`; `internal/storage/postgres/conformance_test.go:19`, `:44`; `internal/storage/migrations/runner_test.go:14`, `:76`, `:92`; `internal/storage/migrations/manifest_test.go:34`. The contrary claim is in `t2-report.md` under Conformance coverage and in the appended fix-round evidence in `t2-review.md`.

`TestMigration|TestConformance` matches neither `TestBackendConformance` nor `TestHistoryConformance`. It also does not select the owner checks named `TestEmbeddedManifestHasCanonicalPairs`, `TestApplyRejectsChecksumDrift`, or `TestApplyRollsBackFailedMigration`. `TestHistory|TestContinuityMigration` does correctly select both history registrations and the new SQLite position-backfill test; that does not run the other claimed gates.

Contract violated: the brief's exact-command/evidence requirement and the original FAIL evidence's unresolved regex gap. A future exit-zero run of the second command is not proof that these suites ran.

Minimal fix direction: correct the evidence and make the required command actually select the intended tests (or explicitly agree and record a corrected command). For example, `TestBackendConformance|TestHistoryConformance` selects the backend suites; run the migrations package without a name filter for its owner gates. Confirm test discovery with `go test -list` once Go is available. Do not call either command passing now.

### R5 — Minor: record-budget exhaustion leaves HasMore false despite known lookahead

Locations: `internal/storage/sqlite/history.go:135`–`:137`, `:205`–`:207`; `internal/storage/postgres/history.go:133`–`:135`, `:203`–`:205`.

At the cap+1 row, these branches set only `ScanIncomplete`. With two visible one-byte messages, `CandidateRecords=1` and `Limit=2`, page one has a valid advancing Next and `ScanIncomplete=true` but `HasMore=false`, although the second row is already known to exist. The same applies to events and hidden-prefix scans. Byte-budget and result-limit branches set HasMore for known remaining metadata, so the flags have inconsistent semantics.

Contract affected: honest continuation metadata and the fix report's stated meaning of HasMore. A caller using `HasMore || ScanIncomplete`, as the shared tests do, still progresses; this is not an infinite-loop finding and is not independently blocking.

Minimal fix direction: set HasMore on the known-lookahead record-cap branch, preserve Next at the last actually scanned key, and assert both flags plus the next page in each stream. The existing 2,001-hidden-row test checks only ScanIncomplete and a nonzero cursor.

## Explicit SC-D4 position-based rewind resolution

The position-based resolution is CORRECT for the T2 boundary under SC-D4 line 156. The message SQL implements the union of closed `[cutoff.position, tail.position]` intervals for rewind/edit markers. A message inserted after the tail is outside that interval even if its CreatedAt is earlier. Fork provenance markers do not filter; stale/unresolvable anchors are ignored consistently with the existing storage contract.

Do not revert this storage rule to legacy `ListMessages` CreatedAt/id ordering, and do not expand this fix into a rewrite of legacy runtime/UI behavior. R3 concerns the additional whole-run event suppression, not the validity of insertion-position visibility.

## Required-gate disposition

| Gate | Independently inspected result |
| --- | --- |
| True payload byte metadata | SQLite uses `length(CAST(m.content AS BLOB))` and the equivalent payload expression; PostgreSQL uses `octet_length` for both. A TEXT embedded-NUL probe returned `length=0`, blob length `4194369`. These expressions close the original content-length bypass. R2 remains for other selected strings. |
| Payload allocation guards | Both streams check remaining CandidateBytes and ResultItemBytes before SELECTing body bytes, within one page read transaction. SQLite uses its transaction snapshot; PostgreSQL explicitly uses repeatable read. Oversized bodies never get a raw prefix. Invalid message UTF-8 becomes empty unavailable metadata; syntactically malformed event JSON does too, but R1 remains. |
| Hidden-range memory/cancellation | Per-position maps and interval-expansion loops are removed. SQL EXISTS returns one boolean per candidate; no Go marker list grows with retained history. SQL calls use context, capture/metadata/candidate/event loops check cancellation, including entry to an empty event stream. Predicate construction and cut validation loops have fixed 100-session/256-run bounds. Database work is not claimed constant-time. |
| Caller-controlled budgets | Effective hard-caps CandidateRecords at 2000 and CandidateBytes at 4 MiB, preserves smaller positives, caps ReadPageMax at 100, ReadPageDefault at 20, and ResultItemBytes at 8 KiB. Negative relevant limits are rejected. Limit above effective ReadPageMax is rejected. Payload scan limits cannot be enlarged through these fields. Metadata slices have capacity at most 2001, but R2 prevents a total-memory-bound claim. Public ResultPageBytes/envelope projection remains T3's responsibility. |
| Run-event cut and keyset | Both backends really implement run_event. Capture reads `r.id,r.session_id,MAX(e.seq)` in the same transaction as message ceilings, limits run enumeration to 257, and rejects >256. Page predicates bind run AND session AND seq ceiling; `(run_id,seq)` keysets exclude late events. Valid nonzero foreign/out-of-cut cursors and explicit stream mismatches are rejected. SourceRef session/run/seq/kind/time and EventType/PayloadVersion map to the selected columns. R3 prevents visibility acceptance. |
| Message cut/writers | Equal timestamps do not affect ordering. All eight production message INSERT sites across AppendMessage, AppendMessageIfAbsent, edit and fork allocate positions transactionally. PostgreSQL allocation updates/locks the session row; SQLite uses its serialized writer transaction. Duplicate projections preserve the existing position and roll back any racing allocation. Migration 024 is paired, deterministic by `(created_at,id)`, and has a unique session/position index. No duplicate-producing query join is introduced. T4 atomic admission remains deferred. |
| Progress/deletion | Next is assigned before skipping hidden/oversized/remaining-byte records. Remaining-byte exhaustion consumes the scan allowance and returns continuation flags when more metadata exists, allowing a fresh-budget next page; it need not continue fetching bodies in the exhausted page. Iterating on HasMore OR ScanIncomplete does not repeatedly revisit a hostile record. Current committed session deletion removes message/run/event rows, yielding an empty storage page without resurrected content. T3 still owns public reauthorization/source-unavailable status. See R5 for record-cap flag inconsistency. |
| Test reality and compilation | The fixtures are real helper functions, both backend registrations call the same RunHistoryQuerySuite, openBackend exists, storage.Commit has Events, RunEvent has the used fields, and EventModelDelta/EventToolFinished/SourceKindEvent exist. No invalid field, missing helper, or nonexistent constant was identified in the inspected changes. This is NOT compilation proof. R4 and the coverage gaps below remain. |
| Scope | Fix diff changes exactly `internal/storage/history.go`, both backend `history.go` files, and `internal/storage/conformance/history.go`. Original T2 changes are confined to storage contracts/backends/central migrations and their tests. No T3/T4/UI/receipts/workspace-cleanup implementation, session_work_events, new dependency, inline DDL, or second migration registry was introduced. |

The current suite does not test foreign/out-of-cut cursors, cross-run keyset transitions, partial-run visibility, invalid UTF-8 inside valid JSON syntax, or oversized metadata lookahead. Its second-session fixture is inserted only after the paging assertions (`conformance/history.go:60`–`:62`), so it does not actually prove cross-session exclusion. The migration test named “Reopens” reapplies on the same DB handle rather than closing/reopening it. Cancellation tests use already-cancelled contexts, not interruption during expensive metadata SQL. These are evidence limitations, not passing assertions.

Eino capability check: this fix stays in deterministic host-owned storage. SC-D4 section 3 records why pinned Eino tool/runner APIs do not own Vivy's bounded SQL/Journal/visibility semantics. No new orchestration, retriever, Eino adapter, or competing source of truth is added.

## Verification actually performed

- `go test ./internal/storage/... -run 'TestHistory|TestContinuityMigration' -count=1` — UNVERIFIED; exit 127, `/bin/bash: line 1: go: command not found`.
- `go test ./internal/storage/... -run 'TestMigration|TestConformance' -count=1` — UNVERIFIED; same exit 127. Independently, the regex mismatch above remains.
- `VIVY_POSTGRES_TEST_DSN` is unset (presence checked without printing credentials). PostgreSQL execution/parity is UNVERIFIED, not passed or successfully skipped.
- `git diff --check 76cdf60..145d5d9` and `git diff --check 76cdf60^..145d5d9` — exit 0, no whitespace errors.
- Read-only repository searches verified writer sites, test declarations, helper/type/constant definitions, paired migration ownership, and the four-file fix scope.
- Ephemeral `python3`/SQLite `:memory:` probes (no repository/database files written) exercised the byte-count and visibility SELECT expressions and the oversized metadata row. They confirmed the outputs cited in R2/R3 and the gate table. They are SQL-level evidence only: not the Go backend suite, not modernc driver verification, and not PostgreSQL execution.
- Regex inspection confirmed the specific selected/unselected test names in R4. Go test discovery itself remains UNVERIFIED.

Remaining acceptance gates include actual Go compilation, corrected test discovery, both backend suites with adversarial regressions, PostgreSQL DSN-backed parity, real reopen/upgrade/checksum/rollback checks, and cancellation/concurrency behavior through the owning Go seams. No runtime PASS is granted. Fixing R1-R4 requires another scoped review; R5 should be resolved or explicitly documented before consumer integration.

## Final-fix implementation evidence (not acceptance)

The implementation round after this review added the requested R1-R5 code and
shared conformance changes. This appendix records implementer evidence only;
it does not change the review's FAIL disposition or claim an independent
re-review.

- R1: both event payload paths now require `utf8.Valid(body) && json.Valid(body)`; the shared fixture places invalid UTF-8 inside otherwise valid JSON and then pages to a valid event.
- R2: storage-owned 512-byte identity and 128-byte label policies are enforced by SQLite/PostgreSQL `CASE` projections before driver scan. Oversized identities return `ErrHistoryMalformed`; oversized role/type labels become bounded unavailable candidates. Shared coverage exercises message/event cap+1 lookahead and captured run IDs.
- R3: event hidden predicates now match `msgp_<runID>_%020d_<slot>` for `tool.requested` slot 1 and `tool.finished`/`model.completed` slot 0. Shared coverage hides only one event in a four-event run, retaining a post-tail earlier-`CreatedAt` projection and an event with no deterministic projection.
- R5: both cap+1 branches set `HasMore=true` while preserving `Next` at the last scanned key; both streams assert continuation.
- R4 evidence commands were corrected and attempted exactly. All are **UNVERIFIED** with exit 127 and `/bin/bash: line 1: go: command not found`:
  - `go test ./internal/storage/... -run 'TestHistory|TestContinuityMigration' -count=1`
  - `go test ./internal/storage/migrations/... -count=1`
  - `go test ./internal/storage/sqlite ./internal/storage/postgres -run 'TestBackendConformance|TestHistoryConformance' -count=1`

`VIVY_POSTGRES_TEST_DSN` was unset. An ephemeral SQLite `:memory:` probe passed
for the event-specific hidden flags and SQL-side oversized-label projection;
this is not Go compilation, modernc-driver, or PostgreSQL evidence.
