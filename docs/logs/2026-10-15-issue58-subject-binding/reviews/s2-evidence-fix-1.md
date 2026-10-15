# Issue58 S2 Task2 — scoped re-review, fix round 1

## Scope

Reviewed fix BASE `15e79419ccfa302ad3f8330fe72448c88796a5fb` through HEAD `d1a7c325261d50689908cea9c3a9eaebfd159afe`, implementation/evidence commit `333a0bf0cd5def56d9f6f90e31d2d220058ab05b` plus report-only follow-up. The scope is the two Important findings in `task-2-review.md`, the accepted `task-2-fix-1-brief.md`, and direct breakage introduced by this fix. The complete supplied 1,248-line packaged diff was read, including its three source/test files, RED/GREEN and required-race logs, verification manifest and documentary updates.

The separately reviewed Eino v0.9.20 remediation was preserved and was not reviewed again. Current uncommitted architecture/plan amendments, observer redesign and other implementation work were not read as new requirements or modified.

## Prior findings

### I1 — ADDRESSED

**Location:** `internal/storage/postgres/history_mutations.go:119` and `:122`.

Both new statements executed through the existing raw `*sql.Tx` now use native PostgreSQL parameters: `$1/$2` for exact parent lookup and `$1/$2/$3` for the copy-source insert. The correction leaves transaction creation, rollback, exact parent validation and metadata ownership unchanged. It introduces no SQL wrapper, driver or dependency. Shared backend conformance remains registered.

This closes the identified source defect. It does **not** establish PostgreSQL backend parity: no legitimate PostgreSQL DSN was available, and the reported real-server tests remain skipped. The fix brief explicitly retained the real backend contract and rejected a source-string test that merely mirrors the corrected syntax; the fix follows that direction.

### I2 — ADDRESSED

**Location:** `internal/runtime/tooladapter.go:738–762`; covering regression `internal/runtime/subject_question_test.go:40–185`.

The existing native Question resume branch still requires `isTarget && hasData`. For a currently authorized retained source, it now admits the exact original request through the existing operation coordinator, then completes evidence for the actual native resumed answer before returning to enhanced normalization/model continuation. The callback does not invoke `ask_user`; it records an already committed Question outcome. Explicit `originalToolOutcome` capture preserves the complete answer even when its first bytes equal the normal untrusted-output header. Existing artifact/receipt validation, ownership and operation states are unchanged.

Pending Question still follows the existing native interruption path without fabricating an answer receipt. An ordinary unshared resume still returns the actual answer directly. Authorization/persistence errors propagate; the fix does not weaken the evidence validator or reinterpret an interrupt as a successful result. Question storage remains the human-decision authority, while the operation receipt retains evidence of that outcome.

The new regression provides concrete closure of the original consequence:

- `subject_question_test.go:63–86` uses the actual service, native question flow, persistent checkpoint and SQLite close/reopen/recovery; it asserts no pending answer receipt or seal.
- `:87–106` submits a 38,043-byte answer beginning with the literal untrusted-output header, verifies the authoritative stored answer and rejects a duplicate answer through existing Question semantics.
- `:107–150` verifies a completed receipt, artifact/result SHA256 and byte length, exact original requested arguments, original-reader expansion, repair/idempotence and exactly one sealed interaction.
- `:151–183` checks ordinary unshared receipt exclusion, shortened model projection with an exact reference only for retained sources, a subsequent same-domain interaction reaching head 2 (ordinary remains head 0), and one terminal event for the resumed run.

The supplied meaningful RED reproduces the missing completed receipt (`storage: not found`) and unsealed evidence on the retained branch while the ordinary control passes. The same test's GREEN includes both branches. The earlier expiry-fixture setup failure is clearly disclosed and is not counted as behavioral RED.

## New findings introduced by this fix

None identified: **0 Critical, 0 Important, 0 Minor**.

The production change remains limited to the two raw PostgreSQL parameter corrections and the retained-only native Question success boundary. The test exercises actual restart/recovery and domain progress rather than just successful run status. No additional transcript authority, operation state, question engine, tool reinvocation, dependency change or validator relaxation is introduced.

## Verification reviewed

No suite was rerun, no source was edited, and no additional execution test was necessary to resolve a concrete doubt in this narrow correction. I read the complete packaged diff and supplied outputs/manifest; only this requested review report was written. The first large raw-log display truncated part of its output, so that portion was reread as decoded individual log entries. A decode helper stopped when it reached the subsequent pretty-printed JSON manifest; that manifest and all remaining diff lines were then read directly. No git diff recomputation or broad repository crawl occurred.

The fix report and `task-2-fix-1-verification.json` identify exact commands, source/output hashes, per-package counts and every skip. Counts below are reported terminal Go JSON test events, including subtests/groups, not distinct top-level tests or newly reproduced review results.

| Check | Reviewed result |
|---|---|
| New retained Question RED | Exit 1: retained subtest/group fail; ordinary control passes (2 fail, 1 pass) |
| Same Question GREEN | Exit 0: 3 pass; exact 38,043-byte answer manifest present |
| Focused Issue58 runtime/storage/App/RPC | Exit 0: 93 pass, 8 PostgreSQL skips |
| Original required race expression, unchanged | Exit 0: Runtime 58 + SQLite 24 = 82 pass; no skips/failures |
| Full runtime/storage | Exit 0: 1,404 pass, 42 skips |
| Native Question/Plan/interrupt/cancellation race regressions | Exit 0: 19 pass; no skips/failures |
| Diff whitespace check | Reported pass |

The complete required-race log includes retained Plan revise/execute-once, the new retained and ordinary Question cases, existing projector checks, operation crash matrix, approval replay, concurrent cancellation and the prefix/copy conformance tests. It ends with both packages passing and contains no race failure. This is the original required expression at the already upgraded native Eino v0.9.20, not the earlier diagnostic exclusion. The previous v0.9.13 failed evidence remains historical; it is superseded for this fix candidate by the supplied exact-gate pass without rereviewing the dependency change.

The 42 full-gate skips are 36 PostgreSQL server tests, 5 PostgreSQL migration tests and the existing external-GitHub runtime feedback snapshot test. The single pure PostgreSQL unit pass and package compilation are not server acceptance.

## Outside-scope observations and remaining gates

These are nonblocking for this scoped correction review and do not reopen unrelated source work:

- Real PostgreSQL conformance/parity, native Windows, live provider/product and final HTTP/UI acceptance, #66 producer integration and centralized S6 `just ci`/artifact acceptance remain pending.
- Per-session `RepairSubjectHistory` is not domain-wide restart closure. Current-authorized pending-domain metadata discovery/repair, including archived sources, and covered-source supersession integration remain the controller's S3/S5 dependencies. This fix neither claims to close them nor adds a full-session scan to the hot path.
- The source review accepts these two fixes only; it does not turn task package passes into final Issue58/product acceptance.

## Round verdict

**Spec compliance: PASS for the scoped fix round.** I1 and I2 are both **ADDRESSED** with the accepted minimal corrections and the required meaningful native Question RED→GREEN coverage.

**Code quality/security/data integrity: PASS for the scoped fix round.** No new fix-diff finding remains. The existing native decision and interruption owners, current grant checks, receipt/artifact integrity, ordinary exclusion and exact-copy transaction semantics are preserved. The external acceptance gates above remain explicit.
