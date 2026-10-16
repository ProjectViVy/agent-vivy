# Actual Windows execution and bounded remediation ruling

Run38825120165 executed branch554c668391852ad6cf2b9fc189c56be18b76e1a6 merged
with main5715950 as actual267a0151acc09c533ace5767001ce5a9e2424df5. Native
identity confirmed Go1.26.4 windows/amd64 on Windows10.0.26100 and actual pinned
Laputa249a756e04c5f28af781e4dadea9e9086689bde0. This precedes46a/f6; it is not
acceptance of those later sources. Root remotely fetched six implicated source
files at actual CI/native revisions and byte-compared them with immutable local
counterparts. The original native artifact ZIP SHA256 is
`d6b2bbc3ba372572b6d55fcbf9ba99700a5f029a723e691313efc396b77f8f09`.

| Lane | Actual result | Qualification |
|---|---|---|
| UI | PASS707 tests/93files, typecheck/build/i18n | Intermediate fixed fixture source only |
| BML native1.56 | FAIL:153pass/6fail/0skip,40.277s | Six old Home fixtures omit Close; Windows TempDir cannot remove open DB |
| ACTMEM native | FAIL:88pass/25fail events/1skip,5.727s | Shared helper reads the deliberately exclusively locked root coordination file; fail count includes parents |
| Garden declared native graph | BUILDFAIL3packages/0test bodies,17.342s | hnsw0.6.1 calls renameio.TempFile, absent on Windows |
| Old selected full-UI browser | PASS2 tests | Selected process stop/wait also completed |
| New Recap browser | SETUP FAIL before test/render | Nested quoted bare pnpm launcher looked under wrong cwd; exact shim mechanism remains high-confidence inference |
| Backend | CANCELLED after real runtime FAIL89.385s | CrashCuts zero=false/before-commit expects unsupported Windows negative resolution; cancellation did not erase the failure |
| Aggregate | FAIL | No whole CI/Windows/browser acceptance |

Backend runtime failure was logged23:50:53; later checkpoint cancellation was
23:56:44. PostgreSQL package `ok` in this partial log does not establish a real
server/DSN run. The new browser produced no screenshots/trace because setup never
reached a browser; absence was retained, not replaced with a fabricated image.

## Root ruling for the next serialized writer

1. BML: register error-reporting Home cleanup immediately in exactly the six old
fixtures, before any Fatal path. Keep lazy-open/CAS/tombstone/GC/index assertions.
2. Native ACTMEM: snapshot all data/history bytes; treat only exact root
`.owner.lock` as a coordination entry with presence/type metadata, never read
its locked bytes or disable/release the lock to inspect it. Other read errors
still fail. Register immediate fatal-safe cleanup for opened recovery handles,
retaining explicit close/reopen order. Preserve owner/alias/subprocess/crash and
Windows positive controls. Publish a reviewable test-only native checkpoint and
update the root source lock/module pins through normal verified resolution.
3. Garden: preserve the actual raw declared graph BUILDFAIL. The bounded required
product lane may execute **Garden with Vivy's shipped Windows compatibility**
using a temporary copied native mod/sum and an explicit replacement of only
renameio with the one existing pinned root `third_party/renameio` implementation.
No dependency upgrade, new vendored copy or permanent cross-repository replace is
authorized. Force GOWORK off; record original/effective declarations, resolved
module graph/paths, exact replacement/diff, all11 compatibility-source hashes,
commands and original outputs. Verify native declarations unchanged. Label this
artifact compatibility graph separately from standalone native Garden, whose raw
Windows support remains unavailable. The compatibility lane must fail on any
test/setup error; do not skip failed tests or claim new atomicity guarantees.
4. Browser: first set absolute `PNPM_EXE` from `Get-Command pnpm.cmd -CommandType
Application` through the config's existing override, alongside actual GO_EXE.
Preserve predev/stage-ui, proxy, readiness, no-server-reuse, assertions and failure
propagation. Actual Windows rerun must test the launcher hypothesis; direct
Node/Vite with the same staging is only a fallback if this fails.
5. Runtime test: retain the exact missing-head before-commit case on Windows and
assert existing ErrRecapCommitPending/nativeunknown, immutable pending bytes/op,
revision0/coverage0/no head, retained source hashes/blobs and no further HTTP.
Repeated admission/resolution stays paused. Keep Linux retry and all positive
commit cuts. Add a separate real durable edit-head recovery control that replays
the same exact pending request after renewal and obtains a positive receipt.
This follows explicit revision3 Windows absence semantics; do not change the
production denial/retry logic, seed away the original case or turn it into SKIP.

These fixes change harness/CI qualification against already-approved contracts,
not the product design or memory authority. Source work remains serialized;
Task1a stays frozen for its independent review. Run the smallest relevant local
checks, preserve every attempt, commit before actual Windows rerun, and then
retain the candidate while the required jobs finish. No passing local parser or
collection is a Windows or rendered-UI result.

Read-only diagnosis packets are independently hash-verified (native19 original
artifacts/18 source files plus11 shipped compatibility files; browser7 artifacts;
runtime9 immutable sources plus original log). Source-only diagnostics do not
count as execution. Archives:

- `evidence/s6-native-windows-diagnosis.tar.gz`: `02a2a43a31395d8b057e6c3c3b02d1c71570e7e1fbf19fd7b1b2c5076e7c43fd`
- `evidence/s6-browser-windows-diagnosis.tar.gz`: `0e8ee95e33c0d844a393b1a60a8378f8f558f20c2b35378d3318c2aab11691f4`
- `evidence/s6-runtime-windows-diagnosis.tar.gz`: `1502a7c764b1bf50a087e4174578bb11d6352f3df8ff552dfc4bd6bfc6307a07`
- `evidence/s6-windows-original-38825120165.tar.gz`: `76d60a4d8a116c167e6aeeb93c5f18b878e745ceb301b215594573799e98e32c` (initial observation; finalized backend/cancellation follow-up is in runtime packet)

Latest remote main recheck still5715950; #66 is open/unscheduled, not an installed
singleton producer. Current owner instructions supersede its older mode wording.
No sidebar implementation, actual tenant migration, merge or deployment occurred.
