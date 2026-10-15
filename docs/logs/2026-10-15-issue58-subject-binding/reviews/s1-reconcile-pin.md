# S1 Task 4 independent review

Date: 2026-10-15. Frozen consumer range:
`4658e263746b4e10059627c896f8d07628b50405..61b7684e14a410de2600f729945b4b372a501c67`.
Source/test commit: `2936b6d5d59cb0c776bf834a3c7a3afa5ec5e1d6`.

**Spec compliance: APPROVED for the Task 4 published-pin and bound-facade handoff.**
**Quality, security and data integrity: APPROVED within this scope.**
Findings: **0 Critical, 0 Important, 0 Minor**. No corrective change is required.
This is not native Windows durability acceptance, S1 unconditional completion,
S3 activation approval, or final S6 product acceptance.

## Scope and method

Read the Task 4 brief and handoff first, then its report; read the complete supplied
50,775-byte review package without regenerating the Git diff. Checked applicable
repository instructions, the repository's `vivy-plugin` review guidance, current
S1 Task 4 and authoritative revision 3 C1. The frozen change consists of three
requirement updates, six added checksum lines, the full-SHA source lock, one test
in the existing facade test file, and five report/evidence files. Later or uncommitted
parent S3/spec/status work is outside this verdict.

The already reviewed Task 3 owner implementation and S2 Task 3 implementation were
not reviewed again. Narrow reads of the unchanged facade, factory and Garden denial
tests addressed the named compatibility risks: whether the new test reaches the
same real owner, and whether existing authorization precedes cause handling.
No test suite, archive download, broad runtime/storage check or network verification
was rerun. Verification below independently checks retained artifacts and recorded
outputs; it is not a claim of fresh execution.

## Spec and behavior evidence

- `go.mod:18-19,63`, `go.sum:779-784` and `laputa-source.lock.json:3` agree on
  `v0.0.0-20261015124034-bb57e93a61cf` and full publication SHA
  `bb57e93a61cfb41460524d00a557a3a9f2cce1e7`. All three cached normal `.info`
  records have that Origin.Hash and timestamp `2026-10-15T12:40:34Z`.
  The six archive/go.mod sums match the download output, checked-in sums,
  resolution manifest and cached sum.golang.org lookup records. Git objects confirm
  the published commit and independently reviewed local
  `9ec9baa4db117c3ea3b4a0949fece14999904895` share tree
  `654b8aa8c81251b8843356ebf3c91b00157ba4c5`.
- `internal/modules/diva-cognitive/recap_test.go:163-230` opens the existing
  real bundle with a temporary data directory and trusted binding. It exercises
  coverage zero and nine, including the post-clear epoch, exact Chinese/CRLF text
  and opaque future-version receipt, unchanged coverage/epoch, omitted binding,
  equal-binding replay, foreign subject/domain denial with typed error and zero
  head, changed-receipt conflict and invalid range. Foreign assertions reuse the
  valid operation ID, so the test also detects receipt leakage before scope checks.
- `recap_test.go:231-290` places a fresh certificate before each later mutation,
  checks edit/clear/restore reset, correct text and epoch semantics, original
  receipt replay after every mutation, lookup, revision/history and reopen.
  Restore reads certified historical text without inheriting its certificate.
  Equal binding also accompanies fresh reconcile commits, not only replay.
- The unchanged `factory.go:47-66` opens Garden and obtains `owner.BindRecap()`;
  `recap.go:7` exposes that handle. `internal/cognitivecontract/recap.go:10-15`
  aliases owner DTOs. Garden `agentapi/recap.go:28-48,74-85` holds the lifecycle
  read lock, checks the complete grant, rejects foreign binding, stamps the trusted
  binding and calls the same Store without a cause switch. Existing
  `agentapi/recap_test.go:142-211` checks worker, foreign, closed and unarmed
  denials across reads, lookup, history, revision and commit with no data. Their
  recorded focused/full runs pass; adding duplicate reconcile-specific upstream
  authorization machinery is unnecessary.
- Eino capability check: this range adds no inference, context coordinator,
  scheduler, checkpoint or model adapter. The revision 3 design's existing
  `summarization.New`/`Summarize`, `GenModelInput`, `Finalize`, `Callback` and
  `BeforeModelRewriteState` allocation remains applicable to S3. Eino v0.9.20
  and EinoExt are unchanged. There is no new DTO, Store, public Port, RPC schema,
  dependency choice or backend implementation in this range.

## Independent evidence checks

Recomputed SHA-256 hashes of the four source/pin files from the frozen source
commit, the committed verification JSON, durable RED/GREEN outputs, all five
retained full/focused gate logs, archive download output and initial bootstrap
failure output. They match the manifest/verification records. Parsed every retained
Go JSON event to independently reproduce counts, package results and absence of
named skips. The local report mirror agrees with the durable report, apart from
its explicitly appended stable handoff identity.

| Recorded gate | Independently checked result |
| --- | --- |
| Old published-pin focused regression | Both coverage cases fail at first valid reconcile with `recap_invalid_range` and zero head; not a build/environment failure |
| New published-pin focused regression | 1 top-level + 2 subtests pass |
| Garden focused Issue58 | 6 top-level tests pass, including existing denial/lifecycle families |
| Garden agentapi/runtimecore | 89 top-level + 10 subtests pass; 2 packages |
| Laputa module | 102 top-level + 61 subtests pass; 5 tested packages |
| Published-module consumer | 30 top-level + 10 subtests pass |
| Ordinary-root consumer/exact sibling | 30 top-level + 10 subtests pass |
| Corrected actual `just bootstrap-test` | Embedded complete output records 11 passed, 0 failed, 0 skipped; recipe matches the two existing Node suites |

The package-only skip events are `[no test files]` for Laputa `cmd/eino_smoke`,
`evolution/testkit` and consumer `internal/cognitivecontract`; they are not skipped
named tests. Existing legacy/v2 preservation families are present in the passing
Laputa output. The initial bootstrap output's hash and 2-pass/9-fail counts agree
with the report; it is retained separately from the corrected 11-pass invocation.

Compared the temporary consumer modfile with root `go.mod`: it removes exactly
the three Laputa replacements and nothing else. The manifest resolves all three
modules from published module-cache directories with no `Replace` fields.
The minimal three-module verification input has no replacements and records
`all modules verified`. Root's distinct 19 historical local v0.0.0 missing-ziphash
conditions remain correctly reported as a limitation, neither checksum corruption
nor successful root verification. The normal checksum environment is recorded;
there is no checksum bypass in the evidence. The sibling is clean at the exact
published SHA, independently confirming the ordinary-root test identity.

## Handoff and retained qualifications

Task 4 supplies the reviewed publication and compatible bound facade for S3/S4,
subject to the separate S2 Task 3/domain handoff. Compilation alone does not
negotiate owner capability. Preserve the exact pending operation/request/range and
evidence until a durable positive receipt; revision zero, visible coverage,
busy/unknown/unavailable and unsupported negatives do not authorize retirement.

Task 3's 114 passing race events remain historical producer evidence, not this
candidate's whole-product CI. Fault-seam testing is not actual power-loss evidence.
Native Windows durability, real PostgreSQL, actual S3 provider behavior,
BML/Mentle integration, issue #66, broad MemoryLoop timeout resolution and final
exact-candidate S6 CI/product acceptance remain pending.

Rollback must disable extended admission while retaining compatible ownership and
all heads/history. Do not downgrade reconciled heads to the old strict decoder,
rewrite receipts or reactivate legacy writers. Old-reader rejection remains the
previous source-inspection conclusion, not a new old-binary run.

Only this review file was written. Source, index, commits, pins and sibling were
not changed; no push, merge or deployment was performed.
