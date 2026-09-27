# T2 implementation report — bounded stable history storage

## Accepted predecessor and review state

- T1 accepted implementation: `9a1c5c1`.
- Supervisor acceptance/status: `94fd4b9`.
- Initial T2 implementation: `76cdf6018e46fe32e423a1b6574af6bbd957f023`.
- An independent task review graded that implementation **FAIL** and recorded
  five verified implementation findings plus one evidence gap in
  `t2-review.md`.
- Fix-round commit: `145d5d9` (`fix(storage): bound history query cuts and
  scans`).
- The scoped re-review at `145d5d9` remained **FAIL** with R1-R4 Important
  and R5 Minor in `t2-review-fix.md`. This final-fix round addresses only
  those storage/conformance and evidence findings.
- T1 and T2 Go evidence remains unverified because the required `go` command
  is not installed on `PATH`; this report does not treat static inspection as
  compilation or passing tests.

## Changed files

The complete T2 implementation/fix range changes:

- `internal/storage/history.go`
- `internal/storage/contracts.go`
- `internal/storage/conformance/history.go`
- `internal/storage/sqlite/history.go`
- `internal/storage/sqlite/messages.go`
- `internal/storage/sqlite/history_mutations.go`
- `internal/storage/sqlite/conformance_test.go`
- `internal/storage/postgres/history.go`
- `internal/storage/postgres/messages.go`
- `internal/storage/postgres/history_mutations.go`
- `internal/storage/postgres/conformance_test.go`
- `internal/storage/migrations/sqlite/024_history_positions.sql`
- `internal/storage/migrations/postgres/024_history_positions.sql`
- `internal/storage/migrations/manifest_test.go`
- `internal/storage/migrations/runner.go`
- `internal/storage/migrations/runner_test.go`

The final-fix round modifies:

- `internal/storage/history.go`
- `internal/storage/sqlite/history.go`
- `internal/storage/postgres/history.go`
- `internal/storage/conformance/history.go`
- `.superpowers/sdd/2026-09-21-session-continuity/t2-brief.md`
- `.superpowers/sdd/2026-09-21-session-continuity/t2-report.md`
- `.superpowers/sdd/2026-09-21-session-continuity/t2-review.md`
- `.superpowers/sdd/2026-09-21-session-continuity/t2-review-fix.md`

## Migration, writers, and position visibility

Migration `024_history_positions` is paired in the centralized manifest. It
adds `messages.position`, `sessions.next_message_position`, deterministic
legacy backfill ordered by `(created_at, id)`, session counter backfill, and
the unique `(session_id, position)` index.

Position allocation remains in the insertion transaction for
`AppendMessage`, `AppendMessageIfAbsent`, `CommitSessionEdit`, and
`CommitSessionFork` on both backends. A deterministic projected-message
duplicate is checked before allocation and retains its existing position.

The SC-D4 visibility resolution is backend insertion position, not
`created_at` or message ID. Rewind/edit markers hide the closed position range
between their two message anchors. A later insertion receives a position
after the marker tail and remains visible even when its timestamp sorts
earlier. The fix round computes this with a per-candidate SQL `EXISTS` range
predicate; it never expands a long hidden interval into a per-position map.

## Query, cut, and budget semantics

`HistoryQueryStore` captures, in one read transaction, per-session message
position ceilings and per-run `(session_id, run_id, event_seq)` ceilings.
Cuts allow at most 100 sessions and 256 runs; wider run selection returns
`ErrHistoryNarrowScope`.

Message pages use stable `(session_id, position)` keysets. Run-event pages use
stable `(run_id, seq)` keysets. Both apply their captured ceiling, so rows or
events appended after capture are excluded. Current rewind/edit visibility is
applied to a run event only when that specific event has its deterministic
projected message in a hidden position range: `tool.requested` uses slot 1,
while `tool.finished` and `model.completed` use slot 0. Events without that
direct projection are not hidden because another message in the run is
hidden. Run-event candidates contain the correct
session/run/event reference, event type, payload version, and only a bounded
payload; oversized or malformed payloads expose unavailable/truncated metadata
without raw JSON.

`HistoryQueryOptions.Effective` caps caller-supplied candidate work at 2,000
records and 4 MiB while preserving smaller positive limits. Metadata queries
use only `effective CandidateRecords + 1`. SQLite measures message and event
bytes with `length(CAST(... AS BLOB))`; PostgreSQL retains `octet_length`.

Storage-owned metadata policy caps identities at 512 UTF-8 bytes and
role/event-type labels at 128 UTF-8 bytes. SQL `CASE` projections enforce
those bounds before values reach either driver. Unsafe identities return
`ErrHistoryMalformed` without truncating cursor keys; unsafe labels produce
empty-text unavailable/truncated candidates and still advance safely.

`BytesInspected` never exceeds the effective byte budget. A record that does
not fit the remaining budget advances `Next` and becomes unavailable when
visible. `ScanIncomplete` reports budget-limited scanning; `HasMore` reports a
known later metadata row, including the cap+1 record-budget lookahead. `Next`
remains the last actually scanned key. A later page starts after that unavailable record,
so the following bounded record can still be returned.

All capture/query SQL uses context-aware operations, and metadata loops check
the context explicitly. Authorized session/run values remain parameters; no
payload is loaded before bounded byte metadata is checked.

## Conformance coverage

Shared SQLite/PostgreSQL conformance now covers:

- equal-timestamp message capture, delayed earlier-timestamp projection, and
  limit-one stable paging;
- current deletion/rewind visibility and a post-marker insertion with an
  earlier timestamp;
- 2,000-record and 4 MiB scan ceilings;
- a greater-than-4-MiB embedded-NUL message, asserting unavailable output,
  cursor progress, and `BytesInspected <= 4 MiB`;
- remaining-byte exhaustion followed by successful continuation on the next
  page;
- hard-cap and smaller-limit behavior;
- two run events, capture, one late event, and limit-one paging that returns
  event sequences `1,2` exactly once;
- run-event exclusion when the run's projected message is rewound;
- event-specific projection visibility in a multi-event run, including an
  unprojected event and a post-tail projection with an earlier timestamp;
- oversized, malformed JSON, and syntactically valid JSON containing invalid
  UTF-8 without exposed raw content, followed by successful continuation;
- a 1 MiB message role, a 129-byte event type, and a 513-byte captured run
  identity in cap+1 lookahead, with bounded unavailable progress on the next
  page and bounded failure for the oversized captured run identity;
- message and run-event record-cap lookahead with both continuation flags and
  cursor progress asserted;
- more than 256 runs, cancellation including an empty run-event stream,
  hostile session IDs, and all current message writers.

Both backend registrations expose this suite through
`TestHistoryConformance`, so the required `TestHistory` regex selects the new
tests. Backend registrations are selected by
`TestBackendConformance|TestHistoryConformance`; migration owner gates run via
the unfiltered migrations package command.

## Tests and verification

| Command | Result |
| --- | --- |
| `go test ./internal/storage/... -run 'TestHistory|TestContinuityMigration' -count=1` | **UNVERIFIED** — `/bin/bash: line 1: go: command not found` (exit 127). |
| `go test ./internal/storage/migrations/... -count=1` | **UNVERIFIED** — `/bin/bash: line 1: go: command not found` (exit 127). |
| `go test ./internal/storage/sqlite ./internal/storage/postgres -run 'TestBackendConformance|TestHistoryConformance' -count=1` | **UNVERIFIED** — `/bin/bash: line 1: go: command not found` (exit 127). |
| SQLite `:memory:` predicate/metadata projection probe | PASS (exit 0): event hidden flags `(false,true,false,false)` and oversized role projected as `NULL`. |
| `git diff --check` | PASS (exit 0) before the final-fix commit. |

`VIVY_POSTGRES_TEST_DSN` is unset, so PostgreSQL integration would also skip
after a Go toolchain is provided. No compile/test pass is claimed from static
inspection. All three Go commands were attempted after the final-fix changes
and could not start because the binary is absent.

## Scope and follow-up

No T3 tool/HistoryService/RPC/UI code, T4 admission/receipts/workspace cleanup
or `session_work_events`, external delivery action, inline DDL, second
migration registry, or new dependency was added.

T4 must still implement its atomic admission and receipt semantics separately;
it must not repurpose the bounded history projection as a second Journal
authority.
