# S2 Task 1 — scoped re-review, fix round 1

**Spec compliance: PASS for the reviewed Task1 fixes.**

**Code quality: Approved.**

Reviewed fix base `0813109ddc64c7b70139687a93d7acbdbdc0c775` to head `d54fa92e180f55c70963348ed44118f1ff899cba`. Both original Important findings are addressed; no new Critical, Important, or Minor findings in the ten-file fix diff.

## Original findings

### I1 — ADDRESSED

`storage.SameSubjectSourceBinding` compares both bindings using the same JSON encoding used for persisted metadata (`internal/storage/subject_history.go:81–92`). This consistently handles RawMessage compaction and HTML escaping without modifying the producer's snapshot or normalizing user evidence. Runtime admission/read authorization and SQL membership/run-source comparisons now use that helper (`internal/runtime/subject_binding.go:54`, `internal/storage/subjectsql/subjectsql.go:95`, `:202`). The existing persisted-string comparisons already use the same encoding and do not require a separate normalization path.

The regressions exercise real first admission, persistence, idempotent replay, authorized exact C2 reading, and another admission with the same formatted/HTML-sensitive snapshot. They also reject a genuinely changed decision and assert that producer bytes remain unchanged (`internal/storage/conformance/subject_history.go:301–342`, `internal/app/subject_admission_test.go:240–310`). The shared replay regression is wired to both backends; PostgreSQL execution remains pending.

### I2 — ADDRESSED

The guard now tests the selected normal Run path immediately after subject resolution and before queue/allocation/persistence work (`internal/runtime/service.go:989–994`). An eligible normal Run cannot reach the non-atomic fallback merely because Continuity is wired. Availability checking was removed from the general resolver, keeping binding/read authorization separate from the selected write path (`internal/runtime/subject_binding.go:49–58`). Actual continuity, custom persistence, and Goal branches retain their existing respective boundaries.

The regression uses actual Service.Run with narrow RunStore exposure and an observed Journal/model, rather than directly testing a predicate. It requires zero messages, runs, Journal writes, provider calls, and persisted source membership on rejection. Controls verify unchanged unshared fallback behavior and real eligible continuity pin persistence (`internal/app/subject_admission_path_test.go:20–124`).

## New findings

- Critical: **0**.
- Important: **0**.
- Minor: **0**.

## Evidence and review boundaries

- Read the fix brief, original review, appended implementation report, and all 614 lines of the fix package in bounded passes. No broader source inspection or unrelated pin-writer changes were included.
- Inspected the four recorded I1/I2 red/green logs. I1 red output shows unchanged-grant replay/read failing with `ErrSubjectBinding`; green output passes both regressions. I2 red output shows the unintended 2 messages, 1 run, 6 Journal writes, and 1 provider call; green passes the rejection and both supported-path controls. No unexpected warning noise appears in these logs.
- The report records the covering Issue58 gate as 46 passing test events and 5 PostgreSQL skips, plus four relevant SQLite conformance events. These were not rerun. No unresolved code doubt required an additional test.
- No git commands, subagents, source edits, or test execution. Only this re-review report was written.

## Out-of-scope observations

- Real PostgreSQL execution and final normal-pin integration/central CI remain pending. Local-source verification against the reported Laputa recovery tree is not exact-pin acceptance.
- External #66 producer agreement and S2 Task2 lossless tool/steering/partial repair and eligible shell completion remain the previously identified separate integration obligations.
- This scoped approval closes I1/I2 and assesses the fix diff; it does not claim those outstanding integration gates have passed.
