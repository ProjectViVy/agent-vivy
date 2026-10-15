# Issue 58 S2 Task 3 — independent producer review

Date: 2026-10-15. Scope: specification compliance and quality/security/data integrity of the frozen producer increment.

- Base: `8d7c23cd81afa85c7fba84b600aa5e5a2668dadf`
- Source: `3787f6252a69205dd0c65ab23603041930a00da9`
- Reviewed head: `738a43074352ec8771020923df62dd614f559f6d`
- Frozen package: `.superpowers/sdd/S2/review-8d7c23c..738a430.diff`, 181,458 bytes, 21 files (9 source/test files and 12 evidence/report files).

Read the regenerated Task 3 brief first, then the producer report, complete frozen source/test patch and evidence (the JSONL log was parsed in full and its output inspected), applicable repository instructions, and authoritative revision 3 C2/C3. Narrow unchanged-code inspection was limited to the called SQL rebinding, identity/source/run pin, evidence/prefix-guard, current Host authorization and projection helpers, plus backend transaction wiring. Later working-tree changes are outside this review. No suites were rerun; no source, index, HEAD or pin state was changed.

## Specification compliance verdict

**Producer implementation complies with the reviewed Task 3 contract. No blocking specification defect found.** Acceptance evidence has the minor fixture gap M1 below and the explicitly pending real PostgreSQL and downstream integration gates. This verdict does not declare S3, S6, full CI or provider acceptance complete.

| Contract | Reviewed implementation/evidence |
| --- | --- |
| Same-store API and unchanged history meaning | `internal/storage/subject_history.go:94` extends `SubjectSourceStore` with capture and pending metadata; original four C2 signatures, existing capture and terminal sequence/digest semantics are unchanged. No schema or secondary transcript/cursor/acknowledgement authority was added. |
| One coherent SQL cut | `internal/storage/subjectsql/subject_context.go:19` uses a read-only transaction and explicit PostgreSQL REPEATABLE READ. Identity, pending guard, ceiling, complete source membership, markers, durable message ranges, target/run pins and nested exact-copy resolution share that transaction. Iterators close before dependent queries. |
| Complete membership and current authorization | `internal/storage/subjectsql/subject_context.go:65` reads every persisted domain source binding without grant, archive, marker or suffix filtering. `internal/runtime/subject_context.go:63` resolves and compares every captured source using `SameSubjectSourceBinding`, then checks marker/target/copy participants and rechecks the whole-domain reader. All failures return the zero snapshot. The archived no-marker revoke/restore/version fixture demonstrates the central A/B authorization counterexample. |
| Exact scoped dispositions | `internal/storage/subjectsql/subject_context.go:150` resolves closed ranges by durable position; canonical resolution at line 235 follows exact metadata, validates run identity, detects cycles and bounds traversal at 128. Missing anchors/mappings fail closed. Canonical/view message pairs preserve copied-view scope; fork provenance is excluded. Nested copy fixtures preserve the original digest and parent view. |
| Stable fingerprint | `internal/storage/subjectsql/subject_context.go:128` hashes only version, binding and complete sorted eligible dispositions/targets. Neither the moving ceiling nor `Sources` is serialized into the hash. Concurrent/equal/backward marker fixtures verify exact targets and coherent post-seal marker visibility. New distinct-source stability is statically supported but not directly asserted; see M1. |
| Domain repair before capture | `internal/runtime/subject_context.go:24` holds the existing projection lock, authorizes and repairs all selected projections before starting the sealing phase, reauthorizes each selected run, and delegates evidence validation and sealing to existing owners. Archived and pinned-main sources are included. No second recovery queue, scheduler, fake outcome or status repair is introduced. |
| Interphase terminal and inconsistent status | `internal/storage/subjectsql/subject_context.go:45` repeats the pending query inside the capture cut. The query at line 288 selects terminal-status or terminal-Journal evidence, rejects missing/duplicate terminal coordinates, and excludes genuinely active/unpinned runs. Existing evidence validation rejects stale active status before a seal. Failure fixtures cover missing terminal Journal, stale active status, projection failure and revoked pending grant; the barrier fixture proves a newly terminal run cannot silently disappear between repair selection and capture. |
| PostgreSQL dialect and lease | Every new raw transaction query routes through `s.query` or `s.one`; the latter also rebinds. The shared helper replaces placeholders with native `$n`. Both new PostgreSQL backend entry points retain `b.db.guard()`. This is source-level qualification, not real-server execution proof. |

The Service checks immutable admission pins against current Host decisions rather than treating retained membership as permission. Targeted run checks do not enumerate all historical runs. Denial does not delete, reconcile, reclassify or reset retained facts. The pending projector can read existing bodies for selected repair work; complete capture itself remains metadata-only.

Eino capability check: this increment introduces no model, summarization, provider, callback or orchestration implementation. It reuses the existing Service projection/Journal and storage owners. The pinned Eino integration and import quarantine are unchanged; downstream C3 adapter/provider work is outside this producer review.

## Quality, security and data-integrity findings

Critical: **0**. Important: **0**. Minor: **1**.

### M1 — distinct-source fingerprint stability lacks its explicit fixture

- **Location:** `internal/storage/conformance/subject_context.go:228–234`; related accepted test requirement in `.superpowers/sdd/S2/task-3-brief.md`.
- **Evidence:** The stability fixture creates `context-unrelated` using the existing source `s`, then asserts the digest is unchanged as Through advances. It therefore covers a new unrelated interaction, but does not change the distinct `Sources` membership. The runtime grant fixture also does not add a third unrelated authorized source after the baseline capture. The implementation at `internal/storage/subjectsql/subject_context.go:128–132` correctly omits `Sources` from serialization.
- **Consequence:** No present producer defect is demonstrated. The supplied passing tests do not directly establish the accepted property that newly persisted, independently authorized source membership alone leaves the disposition fingerprint unchanged, or catch an accidental future inclusion of `Sources` in that hash.
- **Smallest correction:** Extend an existing fixture with one distinct authorized source Session and no applicable disposition; recapture through Service, assert that the new source is present/authorized and that the prior `DispositionDigest` is unchanged. Keep this as a narrow acceptance assertion, not another suite or subsystem. Parent whole-branch triage/downstream integration may carry this minor item; no automatic implementation loop was started during review.

No substantiated implementation correctness, security or data-integrity issue was found in the reviewed range. No unrelated Task 2, C1 or Eino findings are asserted.

## Evidence audit

Independently checked SHA-256 and byte lengths of all 10 raw logs against `task-3-verification.json`, and SHA-256 of all 9 source/test files read from the frozen head against its source manifest: **all matched**. Parsed every affected JSONL event and checked the report's totals. The archived/edit and nested-view source/disposition manifests agree across focused, race and affected outputs.

| Supplied command/evidence | Verified result and scope |
| --- | --- |
| `go test ./internal/storage/sqlite ./internal/runtime -run '^TestIssue58Subject(Context\|DomainPendingRepair)' -count=1 -v` | 11 top-level cases plus 4 failure subcases PASS; SQLite 0.325s, runtime 0.478s. |
| Same command with `-race` | Same 11 + 4 PASS; SQLite 8.711s, runtime 12.149s; no recorded race report. |
| `go test ./internal/storage/... ./internal/runtime -run '^TestIssue58' -count=1 -json` | 84 passing test events, 13 skipped test events, no failures; 5 package passes and 3 packages without tests. The PostgreSQL package process pass includes skipped database cases. |
| `go test ./internal/storage/postgres -run '^TestIssue58SubjectContext' -count=1 -v` | All 5 cases SKIP because `VIVY_POSTGRES_TEST_DSN` is unset. Zero real database cases executed. |
| RED evidence | Capability absence; no-marker archived-source grant bypass; terminal status without terminal Journal; terminal Journal with stale active status; unpinned-row exclusion; and terminal arrival between preflight/capture each show the specific intended failure. These are meaningful observed failures, not unrelated build errors. |
| Whitespace | Producer report records clean `git diff --check`; no separate raw whitespace-command transcript is supplied. This review did not recompute the source diff or rerun that command. |

The report records Go 1.26.4 and ordinary `sum.golang.org`; logs/manifests substantiate the stated test outcomes, not an independently recreated execution environment. The frozen report copy agrees with the supplied Task 3 report. No full CI or full-runtime suite result is claimed.

## Remaining gates and limits

1. **Real PostgreSQL / S6:** Run the supplied five database cases against the externally configured disposable PostgreSQL instance, including the concurrent snapshot fixture. Explicit isolation and rebinding are inspected, but live server behavior, lease interaction and isolation under concurrency remain unverified. Do not count SKIP or package exit status as database proof.
2. **S3 actual provider acceptance:** This is the metadata producer, not an HTTP/provider/resume admission boundary. Current grants, disposition recapture, exact suffix/head reconciliation and rechecks at each actual provider/resume boundary remain downstream obligations. No scripted snapshot fixture proves those integrations.
3. **S6 cost and whole-branch acceptance:** Measure the complete source-membership/current-Host checks and disposition/copy work at relevant scale before adding caches. The pending query is metadata-only but this review makes no query-plan or latency claim. Full product CI and final integration acceptance remain separate gates. Preserve M1's exact remaining case in that acceptance work.

Rollback remains disabling the new capture/admission consumer while retaining originals, existing markers, index and original APIs. Reader-session-only fallback is not an acceptable substitute for complete domain capture.
