# Independent scoped Eino remediation review

Reviewed 2026-10-15. Base `4a914f531b341202211e8df8764060190a16ae29`; frozen head `15e79419ccfa302ad3f8330fe72448c88796a5fb`; source/pin/test candidate `d0f1e487baa3ba74d4b0ec7471a7547592b2809c`.

**Spec verdict: PASS for the authorized official-pin remediation slice.**
**Quality verdict: PASS for this slice, with the existing failed broad app diagnostic and remaining integration gates explicitly retained.**
**New findings: 0 Critical / 0 Important / 0 Minor.** No substantiated in-scope defect requires a correction. This verdict does not accept Task2 source, whole-candidate CI, the DIVA Recipe, or S3/S6 delivery.

## Scope and method

Read `.superpowers/sdd/EINO/upgrade-brief.md` first, the complete upgrade report, root AGENTS, local kernel/Eino guidance, and the original Task2 gate descriptions. Reviewed the complete frozen diff package `.superpowers/sdd/S2/review-4a914f5..15e7941.diff`, its manifest and recorded outputs. Its changed-file set and every addition/deletion match the native Git diff; the package additionally contains its header and expanded context. No concurrent working-tree Task2 changes are part of this verdict.

Outside-diff inspections were limited to named risks: official cached module provenance/checksums and native converter/checkpoint/summarization code, original RED commands/logs, before/after graph evidence, raw outputs named in the manifest, and the changed persona fixture's surrounding assertions. No suites were rerun. No source, dependency, test, plan index, or ledger was edited. This review writes only this report. PostgreSQL placeholder/native Question receipt findings belong to the separate Task2 review and are not re-reviewed here.

## Evidence checks

### Official pin and dependency scope

- `go.mod:14` changes only Eino `v0.9.13` to `v0.9.20`; `go.sum:132–133` adds exactly its module and go.mod checksums. Previous sums remain. There is no fork, vendor tree, replacement for Eino, test exclusion, serialized-tool workaround, or production source change in the frozen diff.
- Cached normal Go origin metadata identifies `https://github.com/cloudwego/eino`, tag `refs/tags/v0.9.20`, commit `b9539ec114a14c32aa568f05eda5cde1e83fcfc1`, time `2026-09-20T09:34:37Z`. Cached `sum.golang.org` signed lookup contains the exact two new sums, and the module ziphash matches. Every one of the 385 zip entries matches the extracted module cache; all ten source fingerprints in the manifest match. No patched cached source was used as review evidence.
- Eino v0.9.13/v0.9.20 module files are byte-identical. Recorded before/after graph hashes match their raw files. The changed edge sets are exactly the root Eino edge and its version-qualified outgoing edges, with identical child versions. Existing Inofy/Laputa minimum edges remain unchanged; no EinoExt/transitive version or local replacement changed. Recorded module selection changes only Eino among the five inspected modules; the full graph comparison supplies broader closure evidence.
- The report accurately distinguishes the failed root `go mod verify` (existing local replacement ziphash limitation) from isolated official-module verification. The preserved isolated log says `all modules verified`. This review did not rerun verification or claim root verification succeeded.
- Both reviewed commits have the required human author/committer identity. Frozen changed-source hashes match the manifest exactly.

### Native capability and ordering

- `/root/go/pkg/mod/github.com/cloudwego/eino@v0.9.20/compose/tool_node.go:1365–1369` uses closure-local `parts, convertErr := tr.ToMessageInputParts()` and assigns the parts after success. It removes the captured shared `err` write implicated by both original race reports. The report correctly distinguishes the remaining sequential Invoke conversion.
- Native `toolStreamCheckpointTracker` uses a mutex for completed outputs and rerun IDs. `ErrStreamCanceled` marks unfinished calls; merged completion constructs native rerun state in original call order. `compose/checkpoint.go:332–361` recognizes the typed stream-checkpoint error, restores producer input, and preserves the existing interrupt persistence path. No Vivy orchestration authority was added.
- `adk/middlewares/summarization/summarization.go:281–357` confirms summary generation → Finalize → Callback → optional after-summary event → returned messages; `BeforeModelRewriteState` replaces a copied state only after success. Its diff from v0.9.13 contains only the two error returns preserving `ctx`. Existing GenModelInput and native checkpoint/resume seams remain available. The amendments retain the important S3 obligation to resolve durable receipts after post-Finalize errors.
- Two unchanged upstream native cancellation/checkpoint regressions pass in the saved race log. The disposable test modfile avoids rewriting the product module closure for upstream test-only dependencies. This evidence supports the named changed ordering, not all upstream behavior.

### Authorized fixture and current-version constants

- `internal/app/default_persona_test.go:28–34,76` opens one disposable SQLite backend under `t.TempDir()`, registers cleanup, and supplies the same real SnapshotStore for factory open/reopen. Existing uninitialized-authority, WORLD exclusion, session freezing, saved persona, and new-session projection assertions remain unchanged. No production nil-store fallback or identity relaxation exists in this diff.
- Both recorded old/new-pin persona runs fail at the same pre-model factory initialization with `storage: subject binding mismatch`; the corrected exact persona race run passes. This is consistent with the accepted S2 dependency-fixture ruling, not an Eino workaround or an assertion weakened to obtain green.
- The only other test edits are the two existing explicitly current-pin constants in `internal/app/realsmoke_test.go:29` and `internal/codeface/launch_test.go:22`. No historical fixture was globally rewritten, and no version-string mirror test was added. Corrected app/codeface evidence covers these test-only changes.

### Recorded gates and honesty of reporting

All 13 manifest test records were checked against their actual available raw output: SHA256, terminal per-test pass/fail/skip counts, and package terminal results match. Durable files were used where committed; the remaining exact scratch paths still existed and were inspected. Counts include group/subtest terminal entries.

| Recorded gate | Verified recorded outcome |
|---|---|
| Exact original required race command | 79 pass, 0 fail; retained Plan revise and execute-once included |
| Original unretained Plan baseline race | 1 pass |
| Full runtime/storage | 1401 pass / 42 skip |
| Focused Issue58 runtime/storage/app/RPC | 90 pass / 8 skip |
| Broad compatibility runtime package | 128 pass; its app package still failed on the old fixture |
| Native cancellation/checkpoint race | 2 pass |
| Corrected persona race | 1 pass |
| Codeface current-pin race | 3 pass |
| Corrected broad app compatibility | **FAILED: 31 pass / 1 fail** |
| Later isolated MemoryLoop cancellation race | 1 pass; does not supersede the failed broad command |

The exact required and baseline command strings match the original Task2 report. Original RED log hashes also match; the retained Plan revise/parent and unretained baseline fail with upstream `tool_node.go:1253` evidence. They remain preserved rather than reclassified.

The 42 full-gate skips are 36 real PostgreSQL server cases, five migration checks without a PostgreSQL DSN, and the existing external-GitHub feedback snapshot prerequisite. Focused PostgreSQL contributes eight skips. These are not parity or product acceptance.

The corrected broad app failure is recorded at `eino-app-compatibility-corrected.jsonl:52`: `TestMemoryLoopActivityDeleteDrainsLiveCancellation`, `memory_loop_activity_test.go:147`, `context deadline exceeded`. `eino-upgrade-report.md:223–233` explicitly preserves the failed command, calls overlap/load a hypothesis, and leaves candidate-bound DIVA Recipe acceptance pending. The open TODO records the concern. No timeout relaxation or repeated broad rerun hides it.

### Documentation and history

Current Eino guide title/cache path and S3 current tech stack match v0.9.20. Dated design/S3/S6 amendments explicitly preserve the originally inspected v0.9.13 baseline and native post-Finalize ordering obligations. Historical RED statements in the development record are followed by clearly labeled final-pin amendments. Three duplicate upstream race TODOs move to a qualified local-completion record with historical evidence retained. No documentation claims full CI, real PostgreSQL, provider/product, Windows, or release acceptance.

`git diff --check` on the frozen range passes. The full report correctly describes its evidence-only companion commit and does not pretend that companion SHA was the tested binary.

## Remaining gates and boundaries

The existing broad MemoryLoop cancellation timeout is a real unresolved compatibility concern, not a new finding concealed by this PASS verdict. It requires the already assigned candidate-bound DIVA Recipe/integration gate; its isolated pass does not prove the root cause or close that gate. Continue to carry `TFLAKE-MEMORYLOOP-LIVE-DELETE` and the failed broad output into S6.

Centralized final-candidate `just ci`, real PostgreSQL parity, native Windows, provider/installed artifact/browser LIFE/#66 and DIVA/Mentle acceptance remain pending. Separate Task2 source review corrections remain with their owner. S3 implementation and whole-Issue58 acceptance are outside this slice. No push, merge, deployment, tenant data access, or release packaging was performed or approved by this review.
