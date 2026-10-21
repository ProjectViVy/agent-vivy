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
finding in those corrections. Review did not replace execution. Exact candidate
Linux wiring/kernel, real PostgreSQL, affected Face/Web, actual Provider suites
and race outcomes will follow after running them. Standard3015 smoke remains
owner-deferred; D09 Story and D14 final acceptance are not claimed.
