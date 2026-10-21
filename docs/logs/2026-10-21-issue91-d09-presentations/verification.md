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
Exact candidate a48b54951e262a1f58a45ca79d378114360a9bce then passed in the
independent ext4 checkout with its own pinned Laputa ff3936f44ff8cf08c12af2cf698c194cfe474fd3:

- `just wiring-ci`:16 Node +105 Go cases PASS.
- `just kernel-ci`:1847 cases PASS; one existing optional ripgrep case skipped;
  minimal Recipe pack/Inspect/boot1 case PASS.
- Real disposable PostgreSQL16 matrix:137 cases PASS, zero skips. It exercises
  the new private-byte transaction, actual admission-edit cleanup, reopen,
  conversion rollback and SQLite parity contracts. Container trap cleanup ran.
- `just d02-ci`:81 runtime/RPC,175 shared Face,1 causal App,13 shutdown,
  9 standalone cases PASS.
- `just d02-web-ci`:regenerated/staged UI typecheck,106 cases and2 real UI
  packs PASS. This resolves the native mapped-file staging limitation.
- Full UI84 files/672 cases PASS7.30s, including authoritative Module tests.
- SDK2 files/29 cases PASS323ms and actual SDK TypeScript PASS; dependency link
  removed afterward.
- Strict actual23 Provider suites PASS65.104s.
- Relevant runtime/SQLite/RPC/exportfile race checks PASS69.082s/96.795s/
  18.678s/1.195s. Linux symlink refusal cases execute here without Windows skip.
- Check script exit0; candidate clean before/after. No tenant DSN or journals.

This accepts the automated candidate scope, not standard3015 smoke or human
Story acceptance. Later D08 integration is checked separately.

Independent read-only review found no remaining substantiated defect after the
PostgreSQL admission-pruning and exact retry-window corrections. It covered
runtime capture/transfer ownership, private transactional retention,
deletion/truncation parity, export publication, strict RPC and Face consumers.
Review does not replace execution. Standard3015 browser smoke is owner-deferred,
not passing evidence. No production journals or data reset were used.
