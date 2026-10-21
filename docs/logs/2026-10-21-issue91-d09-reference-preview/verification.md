# Verification

Base9dbe75a9 contains the independently verified shared receipt slice93f385f6.
Only isolated/disposable stores/checkouts were used; root and tenant Journals
remain untouched.

Observed RED before corrections:

- Altered preview text, altered positions and missing expected records were all
  accepted without the exact-record comparison.
- Source deletion/rewind injected after read but before operation commit both
  committed a stale copy; source expectations now fence that transaction.
- New visibility metadata was read once under revoked source scope; authority
  now precedes that read (spy regression).
- Actual PostgreSQL16 parent/child delete interleave produced SQLSTATE40P01.
  A separate interleave deleted newly added tree members instead of refusing.

Corrected deterministic PostgreSQL probe: both lock/membership tests PASS3.351s.
The probe used a temporary ext4 clone, its own clean pinned Laputa, private
PostgreSQL16 container and no tenant DSN. Its trap removed clone/pin/container.
No skipped DSN case is counted. This is targeted RED/GREEN evidence; exact
candidate's full PostgreSQL matrix is still required below.

Native Go1.26.4:

- Initial affected history/reference/continuity suite PASS runtime24.380s,
  domain1.189s,RPC2.998s,tools0.093s,App real coding loop11.994s.
- Source-race correction selector PASS runtime3.719s/SQLite1.846s.
- Broader source-guard suite PASS runtime52.324s,domain0.057s,RPC2.503s,
  tools0.079s,SQLite179.838s and App coding loop10.406s.
- Final authority/reference/source-expectation selector PASS runtime22.384s,
  RPC3.547s,tools0.082s,SQLite2.427s and App coding loop10.859s; domain
  selected no cases in that final command and is not new test evidence.
- Affected runtime/domain/RPC/tools/App/all storage vet PASS.

UI: real Recipe staging/typecheck PASS. Four affected files/81 tests PASS2.18s,
including refused-page recovery and queued expected-record immutability.
SDK Face module24 tests and actual SDK typecheck PASS940ms after fixing existing
missing mock fields and duplicate keys. An earlier SDK attempt lacked local React
resolution and ran no tests; a second ran24 tests but failed mock typecheck.
The final check temporarily linked this same checkout's pinned UI dependencies;
the junction was removed afterward. No dependency version/lockfile changed.

Independent review confirmed exact comparison, authority ordering, source fences,
ReferenceID versioning and the lock/membership correction, with no remaining
finding in those corrections. Review did not replace execution. Exact candidate fd824b05e1c2cfa523fd22a9b24d80cc09bd4194 was then checked
from the independent ext4 Linux checkout with its own pinned Laputa
ff3936f44ff8cf08c12af2cf698c194cfe474fd3:

- `just wiring-ci`:16 Node +105 Go cases PASS.
- `just kernel-ci`:1832 cases PASS; one existing optional ripgrep case skipped;
  minimal Recipe pack/Inspect/boot:1 PASS.
- Real disposable PostgreSQL16 matrix:132 cases PASS, zero skips. Container
  removed by helper trap; no production DSN used.
- `just d02-ci`:81 runtime/RPC,175 shared Face,1 causal App,13 shutdown,
  9 standalone cases PASS.
- `just d02-web-ci`:staged UI typecheck,102 tests and2 real UI packs PASS.
- Affected reference components:3 files/16 tests PASS1.16s; Face SDK24 tests
  PASS422ms and actual SDK typecheck PASS. Temporary dependency link removed.
- Strict actual23 Provider conformance suites PASS69.780s.
- Relevant runtime/SQLite history, reference, delivery and continuity race
  suite PASS58.570s/102.975s.
- Candidate clean before and after; check script exit0.

Standard3015 smoke remains owner-deferred. Durable presentation/download closure
and D09 Story acceptance remain open; D14 final acceptance is not claimed.
