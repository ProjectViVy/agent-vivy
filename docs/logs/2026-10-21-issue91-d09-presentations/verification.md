# Verification

RED evidence on the previous runtime: an otherwise valid old-protocol read
succeeded after replacing the continuity owner with a store reporting no retained
bytes; presentation JSON still carried sha256. The new contract rejects missing
captures as unavailable and emits no digest/private-content fields. Independent
review reproduced an offset0,length3 then offset0,length6 retry incorrectly
returning another window; the deterministic regression failed with nil error
before enforcing equality of the effective returned window.

Native focused Go final run passed:
`go test -tags=vivy_headless ./internal/runtime ./internal/domain ./internal/tools
./internal/app ./internal/rpc ./internal/exportfile -run
'TestD09Presentation|TestDeliverable|TestPresentFiles|Test.*Continuity.*|TestExport|TestDiagnosticsBundleRPC|TestWrite|TestRead|TestAppCodingLoop'
-count=1 -timeout=10m`:runtime18.387s,domain0.094s,tools0.162s,App11.705s,
RPC6.111s,exportfile0.144s. It covers equal-size/restored-time stale content,
immutable later chunks, known-empty restart, transfer close, concurrent chunk
and close, exact duplicate windows and actual App continuity downloads.

Earlier integration attempts exposed remaining domain and App fixture references
to removed fields; those fixtures now compare actual bytes/identity. A temporary
RPC failure accepted retired export input; strict decoding now rejects it.
These failed attempts are not passing evidence. Native symlink cases explicitly
skip on Windows and still require the Linux run.

UI RED/GREEN covered foreign TransferID switching, page/preview bounds and late
results after session changes. Final focused UI:5 files/99 cases PASS1.97s.
SDK:29 cases across2 files and actual SDK TypeScript PASS1.26s; temporary
same-checkout dependency junction removed. Direct UI TypeScript PASS. Two native
staged typecheck attempts failed on a mapped generated assembly.ts file before
compiler execution; independent Linux regeneration/typecheck remains required.
The manifest change is generated from the authoritative session-tree catalog,
not hand edited. No dependency versions/lockfiles changed.

Storage native owner/conformance PASS0.764s and full migrations PASS27.437s.
Lifecycle cases cover direct edit, CommitSessionRewind, CommitSessionEdit and
actual CommitRunAdmission(Edit), including rollback after prune. A PostgreSQL
compile-only check PASS2.188s ran zero tests and is not PostgreSQL evidence.
Affected runtime/domain/tools/RPC/App/all-storage/exportfile vet PASS.
Full native SQLite PASS190.476s (same storage/migrations/SQLite command, exit0).
The shared operation-name follow-up derives the runtime constant from its storage
owner; focused storage1.593s/runtime7.847s presentation tests PASS.
Exact candidate Linux wiring/kernel, real PostgreSQL, Face/Web, actual Provider
and race outcomes are pending below.

Independent read-only review found no remaining substantiated defect after the
PostgreSQL admission-pruning and exact retry-window corrections. It covered
runtime capture/transfer ownership, private transactional retention,
deletion/truncation parity, export publication, strict RPC and Face consumers.
Review does not replace execution. Standard3015 browser smoke is owner-deferred,
not passing evidence. No production journals or data reset were used.
