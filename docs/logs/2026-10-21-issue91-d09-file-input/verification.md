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


Independent Linux candidate15cbf7f6: wiring PASS; kernel failed only storage
conformance CN-18 because SQLite's driver scans both empty BLOB and NULL into
nil []byte. Runtime tests passed. The script stopped before real PostgreSQL,
Face/Web and strict conformance; those checks were not run by that attempt.

A native storage-local regression confirmed SQL had persisted a non-NULL zero-
length BLOB while LastFileAccess returned unavailable. Read `read_content IS NOT
NULL` explicitly in both backends and normalize known-empty content after scan.
Actual NULL remains unavailable. Native regression and CN-18 PASS2.311s after
this correction; matching PostgreSQL NULL defense is included in the real matrix.
No schema change or migration rewrite is needed for this driver-boundary fix.


The next Linux kernel attempt at371363e0 passed CN-18 but failed once in unchanged
TestCognitiveNotifyInputFeedsWindow with active-run conflict. Production and test
bytes for that owner were unchanged from6080b447. Isolated Linux repetition30/30
passed, so no deterministic reproduction or production root-cause fix is claimed.
Inspection found the test invokes a synchronous admission immediately after
Notify has already woken the automatic loop. Change this test to await the actual
automatic workflow and assert exactly one created/completed run and its exact
input window. Native corrected fixture repetition30/30 PASS24.889s. Cognitive
production code, upstream APIs and D10's gated CAS contract are unchanged.

The script again stopped before PostgreSQL/Face/Web/conformance on that failed
kernel attempt; those checks remain pending until the resumed command completes.


Final independent Linux evidence at exact candidate
`e992029509dab70a52e70e7d786beced3f3fbcbb`:

- Independent ext4 checkout and its own clean Laputa pin
  `ff3936f44ff8cf08c12af2cf698c194cfe474fd3`; Go1.26.4/Node24.21.0.
- `just wiring-ci`: PASS16 Node and105 Go cases, zero skips.
- `just kernel-ci`: PASS1788 executed tests; one existing optional
  `TestBackendGrepRipgrepHonorsGitignore` skip. Minimal Pack/Inspect/boot PASS1.
- Real disposable PostgreSQL16 via `just test-postgres`: PASS129, zero skips.
  The private container is removed by the script trap; no production DSN used.
- `just d02-ci`: PASS81 runtime/RPC,175 shared Face/client,1 causal App,
  13 shutdown/headless App and9 standalone headless cases, zero skips.
- `just d02-web-ci`: PASS UI typecheck,102 Web cases across5 files and2 actual
  selected-UI artifact Pack checks, zero skips.
- `go test ./sdk/internal/conformance -run
  '^TestCheckedInProviderConformanceMatchesExecutedSuites$' -count=1
  -timeout=35m -v`: PASS all23 actual Provider suites (67.580s).
- Script exit0 and final clean-worktree assertion passed.

The first file-input implementation slice is verified within that scope.
Cursor/reference/presentation work, D09 Story acceptance and owner-deferred3015
smoke remain open. The earlier failures above remain recorded as failed attempts.
