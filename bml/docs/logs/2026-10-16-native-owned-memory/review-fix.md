# Consolidated native review fix

This increment addresses the two Important findings from the independent review
of native producer 92048bccda52875a798ade1bccc4e44823a564fb. It preserves the
original review probes and their RED evidence.

A schema-1 read-only handle now pins its actual schema check and legacy data
query to one SQLite read transaction. A schema-1 snapshot remains legacy-readable;
a snapshot already upgraded by another supported writer returns the explicit
`schema_changed` error and requires reopen. Get, List, FTS Search/SearchVisible
and SupersededTargetIDs use this boundary. Cached bound capabilities remain
conservative. Reopened schema-2 handles keep ownership predicates before limits
and ranking. There is no reader-driven migration or global handle registry.

A bound tombstone now requires exactly one Supersedes target matching its native
TargetRecordID and target CAS. Rejected extra targets leave canonical records,
ownership, operation receipts, FTS, supersedes and counters unchanged. Valid
single-target operations and ordinary unbound multi-target tombstones retain
their existing behavior.

## Verification

Go1.26.4 on Linux/amd64, unchanged offline local-only runner, `-p=1 -count=1`.
All runs recorded zero external requests and zero forwarding. Counts include Go
JSON parent/subtest terminal events; package results are separate.

| Check | Test pass/fail/skip | Package pass/fail | Result |
| --- | ---: | ---: | --- |
| Author regressions before fix | 1/3/0 | 0/1 | Expected semantic RED |
| Author regressions after fix | 4/0/0 | 1/0 | PASS |
| Three unchanged independent probes, relocated overlay only | 3/0/0 | 1/0 | PASS |
| Native SQLite1.56 complete normal | 159/0/0 | 1/0 | PASS |
| Native SQLite1.56 complete race | 159/0/0 | 1/0 | PASS |
| Root MVS SQLite1.59 native package normal | 159/0/0 | 1/0 | PASS |
| Native `go vet -p=1 ./...` | n/a | n/a | PASS |

Normal: `go test -json -p=1 ./... -count=1 -timeout=4m` in bml.
Race: `go test -json -race -p=1 ./... -count=1 -timeout=5m` in bml.
Root MVS: `go test -json -mod=readonly -p=1
github.com/ProjectViVy/agent-vivy/bml -count=1 -timeout=4m` at root.
Original probes: `go test -json -overlay=<relocated mapping> -p=1 ./...
-run '^TestReviewerBML' -count=1 -timeout=3m` in bml. Only overlay target paths
were relocated; the three original reviewer Go files were not edited.

The permanent regressions also cover byte-unchanged schema-1 read-only access,
concurrent supported upgrade/query, limit-1 reads, explicit stale-handle failure,
reopen and Home L1 isolation, extra-target failure atomicity and the positive
single-target/legacy multi-target controls. The original cancellation probe
continues to verify receipt-insert cancellation rollback and exact retry.

## Scope and acceptance limits

Only native BML code, tests and contract documentation change. Existing root CI
corrections, schema version, dependencies and native Laputa pins are preserved.
Actual native Windows execution is NOT RUN. Root owns fixed-commit unchanged
probe qualification, publication and later integration gates. No App, adapter,
consumer activation or whole S5/S6 acceptance is claimed.
