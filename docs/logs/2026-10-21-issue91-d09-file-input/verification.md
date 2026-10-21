# Verification

Base is integrated D06/D12 commit6080b447 plus verification record29a58434.
Root checkout and production Journals remain untouched. Native Go1.26.4 and
existing pinned Laputa ff3936f44ff8cf08c12af2cf698c194cfe474fd3 are used.

Observed RED before fixes:
- Same-size restored-time change and empty-file deletion bypassed timestamp reads.
- Missing/NULL approved before-bytes authorized an empty baseline.
- A fresh native Eino ToolNode call inherited a previous approval's private data.
- Reviewed replacement of binary content newly failed preparation.
- No-op native/ToolWorld writes reported success after bytes changed; ToolWorld
  also ignored revoked write Grant. Three deterministic no-op cases failed RED.
- Before-state parser errors lost their cause; wrapped errors now preserve cause
  without exposing raw private input in their public message.

Native focused checks before the final no-op correction:
- Runtime D09/file-history/approved-state cases, App ToolWorld adapter and SQLite
  file versions: PASS (4.034s/0.585s/2.955s); migration selector PASS3.869s.
- Runtime file/download/D03 approval and durable-operation selector: PASS23.895s;
  protected tool selector PASS0.059s.
- Full TUI stream/view/live packages: PASS0.926s/3.452s/7.969s.
- UI typecheck with real SDK staging: PASS. Existing Web approval and store suites:
  74 tests across two files,zero skips,PASS3.11s.
- PostgreSQL package compiles and native fence case PASS1.303s. This is not real
  PostgreSQL acceptance; the actual DSN matrix remains required.

Further exact-candidate results follow when run. Required Linux wiring/kernel,
affected Face/Web and real disposable PostgreSQL checks remain open until their
results are recorded. The owner explicitly deferred3015 browser smoke for later;
no skipped check is represented as passing.


Final native correction checks:
- D09/file-history/ToolWorld selector PASS: runtime1.326s,App0.320s,
  migrations0.281s. Includes the three previously failing no-op cases.
- Full migration package PASS8.563s with paired038 and following-head assertions.
- Affected runtime/tools/App/storage/conformance/SQLite/PostgreSQL `go vet`
  PASS (vivy_headless tag).
- Independent read-only reviewer confirmed original findings closed after the
  correction, with no remaining issue in the reviewed first slice. Review did
  not replace test execution. Cursor/reference/presentation owners were excluded.
