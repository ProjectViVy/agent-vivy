# S1 Task 3 independent review

Date: 2026-10-15. Scope: coverage-preserving reconciliation on the existing ACTMEM owner only.

**Spec compliance: APPROVED for the Task 3 source handoff.** The frozen change implements revision 3 C1 and S1 Task 3. **Quality, security and data integrity: APPROVED within this scope.** No substantiated Critical, Important or Minor finding was identified. This is not S1 completion, native Windows durability acceptance, Task 4 compatibility acceptance or S6 product acceptance.

## Reviewed candidate and authority

- Laputa base: `4c7c6b20859375d0d764d3ba2a0188926cf73c9e`.
- Laputa candidate: `9ec9baa4db117c3ea3b4a0949fece14999904895`.
- Read the Task 3 brief first, then its report. Read the supplied `review-reconcile-4c7c6b2..9ec9baa.diff` once in full; no replacement diff was generated. Its four files contain 14 production additions, two production deletions and 324 new test lines (338 additions overall).
- Checked the consumer and Laputa instructions, the selected #58 policy refinement, authoritative revision 3 C1, S1 Task 3/4 boundaries, and the prior qualified owner-recovery review. The newer selected Recap policy governs this task; it does not authorize changes to WORLD, Persona, worker permissions or other authorities.
- Confirmed the source checkout HEAD is the frozen candidate. Each of the four working-file SHA-256 values matches the verification manifest, and each file is byte-identical to its candidate commit blob. Future consumer S2 work was not reviewed.

## Spec and implementation assessment

1. **Correct transition on the same owner.** `laputa/actmem/recap.go:176-190` initializes from current coverage and epoch and admits reconcile only when both range endpoints equal current coverage, the opaque receipt is nonblank, and `RestoredFrom` is zero. It allocates the ordinary next revision without changing coverage/epoch. `recap_format.go:65-68` enforces matching persisted metadata; `recap_history.go:157-188` enforces binding, revision, prior digest, unchanged epoch/coverage, endpoints and reconstructed operation digest. The chain decodes each selected revision before transition validation, including revision one (`recap_history.go:91-137`). No parser for the host envelope, source-verification engine, second progress coordinate, owner or persistence path is added.

2. **Exact text and opaque receipt survive the existing format and hash contract.** UTF-8 validation remains in `recap.go:128-129`; request hashing still covers the complete typed request (`recap.go:141,233-240`). The schema stays `actmem/v3`, and encoding appends the unnormalized Markdown body (`recap_format.go:12,19-26`). `recap_reconcile_test.go:20-67` checks Chinese/CRLF/fences/delimiters, zero/nonzero coverage, preserved post-clear epoch, complete request hashing, unchanged prior revision bytes, exact reopen and reachable history. A deliberately unknown envelope version with surrounding whitespace is accepted as opaque owner data. Interpreting that envelope remains the trusted host's responsibility.

3. **Existing mutations, CAS and replay remain intact.** Ordinary summary still requires a nonempty range. Edit/restore/clear still require empty Sources and retain their original text/epoch rules (`recap.go:181-219`). Replay follows owner recovery and precedes stale-base rejection (`recap.go:137-155`). The new tests reject invalid endpoints, blank receipts, restore metadata and unknown causes without file changes; test certificate reset on each mutation; replay the original full receipt after edit/restore/clear; reject changed text or receipt bytes; and prevent a stale reconcile from overwriting clear (`recap_reconcile_test.go:71-147`). Validator fixtures recompute integrity hashes so rejection tests semantics rather than only stale checksums (`recap_reconcile_test.go:152-196`). Historical plain receipts and their hashing remain unchanged.

4. **The existing durability qualifications are preserved.** Reconcile reaches the same `persistRecap` path (`recap.go:160-169`) and the same exclusive-owner/recovery gate (`recap_history.go:19-58`). The new fault tests exercise pre-replacement failure/cancellation, post-replacement uncertainty, lost response, four continuing recovery failures, persistent live-owner freeze after fault removal, exact receipt replay after reopen, immutable bytes and orphan exclusion (`recap_reconcile_test.go:198-324`). The reused assertion helper explicitly checks zero-data errors across read, positive/negative lookup, history, revision and Commit replay (`recap_recovery_test.go:36-58`). The zero-coverage pre-replacement branch retains unsupported Windows missing-head negative resolution as `recap_commit_unknown`; an uninitialized Read is not elevated into a durable negative receipt.

5. **Compatibility and architectural restraint are appropriate.** DTOs, schema identity, hash construction and public methods are unchanged. The base strict decoder has no reconcile case and its unchanged default rejects unknown causes; fail-closed old-reader behavior is a source-inspection conclusion, not an old-binary run. No Eino capability is being reimplemented: the revision 3 capability check assigns inference to the existing runtime summarization APIs, while this task adds only storage transition semantics. No new dependency, model import, authority or scheduler appears in the four-file change.

## Recorded verification checked independently

Parsed the three raw JSONL files under `docs/logs/2026-10-15-issue58-subject-binding/`, recomputed their SHA-256 values and byte lengths, and counted named test/subtest terminal events separately from package results. All matched `s1-task-3-verification.json`:

| Recorded run | Named pass / fail / skip | Package result | Bytes |
| --- | --- | --- | --- |
| Focused RED | 8 / 18 / 0 | fail, 0.014s | 30,792 |
| Focused GREEN | 35 / 0 / 0 | pass, 0.068s | 31,659 |
| Full ACTMEM race | 114 / 0 / 0 | pass, 2.867s | 98,145 |

The RED errors show the expected missing-feature rejection, `invalid Recap cause, source range or restore payload`; the eight passing cases are negative admission checks. GREEN records all four reconcile test families, including their validation and recovery cases. The full ACTMEM output also records the existing crash, owner exclusion, recovery, legacy fencing, replay and concurrent-CAS families passing. Counts include parent tests and subtests, not that many independent scenarios.

The report identifies the commands and Linux Go toolchain; the raw output verifies the reported outcomes. The reported initial non-JSON RED, intermediate fixture repair and whitespace checks are reported history, not independently rerun evidence. No test suite, Windows cross-compile or old binary was executed for this review: inspection and the existing raw output resolved the in-scope questions.

## Remaining gates

- **Native Windows:** still pending and required. The new platform-neutral cases are suitable for the existing native fault seams, but Linux results and zero reported Linux skips do not execute Windows-only tests or establish Windows durability. Preserve the qualified absence/publication split from the prior recovery review.
- **Task 4:** prove bound-facade exact text/receipt and error propagation, omitted/equal binding behavior and foreign/worker/closed/unarmed denial; run its Garden/Laputa/consumer gates; publish the exact reviewed owner and coordinate module sums/lock identities before host activation. No consumer pin or facade acceptance follows from this review.
- **S3/S6:** downstream pending-request/evidence retention and recovery, full integration, required `just ci`, native platform, real-model, UI/install/restart and same-candidate product acceptance remain open. Revision zero, unsupported negatives, visible coverage and unknown outcomes cannot authorize evidence retirement.
- **Rollback:** after reconcile heads exist, disable extended admission while retaining a compatible owner and all heads/history. Do not downgrade to the strict incompatible reader, rewrite old receipts or reactivate legacy writers.

Only this requested review file was written. Source, indexes, HEADs and pins were not mutated; no publication, push, deployment or additional reviewer was used.
