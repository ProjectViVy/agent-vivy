# Independent review: S5 native C5 legacy-consumer compatibility

## Verdict and immutable scope

**Changes required: 0 Critical, 2 Important, 0 Minor findings.** Both Important findings have independent executable RED probes. The producer is not ready for acceptance/pin activation until the consolidated fix and unchanged-probe verification. No other substantiated finding was identified in this scope.

- Reviewed local native candidate: `3c365207d295221c4a3c5667fc71a6a465ee59f8`.
- Exact tree: `1ca98be9a8cffb18d242bf3c1aa40a613d0bbb7f`.
- Baseline: ordinary merge `1ce1c1e322ff278db258044dfddb640dcf29101c`, tree `a6d4515158008ffc96dc51f7ee546e13115d3745`, preserving the accepted bb57 native recovery producer and current f44 policy documentation.
- Parent supplied remote tree-equivalent source: `1edf7b2379d7887a016c57b78e2060ad9dae1588`; remote publication/tree verification is the parent's evidence, not an independent remote operation by this reviewer.
- Isolated clone: `/workspace/scratch/5dda642e718d/laputa-issue58-native-review`.
- Increment: 12 files: five production Go files, four Go test files, three English records. All 12 and 15 supporting files were hashed against committed HEAD blobs; all 27 match. The candidate worktree/index remain clean.

This reviews the native producer only, against the assigned brief, applicable native AGENTS, ADR-0018/0012 and approved Vivy C5. It does not repeat the completed S4 App fix review or accept BML activation, all S5, or whole-candidate S6. No production/test source in the clone, active native checkout, Vivy source, dependency pin, or GitHub state was modified. Independent tests were added only through external Go test-file overlays. Receiving/requesting-review and verification-before-completion guidance were applied; no subagents were launched.

## I1 — Important: absent/null original revision is converted into a false applied receipt

**Locations:** `garden/internal/ingest/capture_activity.go:121-124,364-367`; consequence at `:178-186`. `activityIntent.Before` uses the value-typed `actmem.AppendSnapshot`, whose `Revision` is `uint64` (`laputa/actmem/capture_pair.go:16-18`).

The new v3 recovery branch checks the original SHA's shape and revision overflow, but cannot tell whether `before.revision` was present and non-null in the persisted JSON. Go decoding maps both a missing revision and JSON `null` to zero. Exact positive archived entries do not supply the missing original legacy revision: v3 archive lookup deliberately returns `Revision=0` to mean there is no current legacy head revision.

The independent probe creates genuine native entries at legacy revision 3, folds them, preserves an unknown attempt whose actual original snapshot was revision 2, and commits a valid v3 C1. It then removes only `before.revision`, or sets it to `null`, retaining the genuine snapshot hash and exact native pair. `projectActivity` returns success and stores:

```text
actual_native_revision=3 original_before_revision=2
status=applied revision=1 entries=[two actual native IDs]
source.user_text="" error=""
```

Thus incomplete original proof manufactures the wrong receipt revision and clears the original uncertain-attempt evidence instead of retaining uncertainty with a separate policy disposition (or rejecting the malformed record). This is structural completeness, not a claim that capsules can cryptographically reconstruct a tampered but well-shaped historical SHA. The committed producer summary explicitly promises a structurally complete original snapshot.

**RED:** `probes/reviewer_snapshot_test.go`, `TestReviewerS5OriginalSnapshotCompleteness/{missing_revision,null_revision}`. Both fail. The same probe's explicit complete revision-zero and complete revision-two controls pass, returning actual native revisions 1 and 3 respectively. Preserve legitimate zero.

**Smallest fix:** retain presence/non-null information when decoding the original snapshot and require a structurally complete original revision/hash before invoking positive receipt recovery. Reject incomplete metadata, or preserve the old unknown effect and only record an independent proven policy closure; never infer a missing revision as zero. Do not replace this check with `Revision > 0`. Maintain error/source/Before evidence when no complete original snapshot exists.

**Classification:** new v3 compatibility recovery defect. The generic value-decoding behavior existed earlier; this increment introduces the v3 positive-archive-to-receipt path and its explicit snapshot qualification promise.

## I2 — Important: the shared decoder accepts invalid trailing delimiters and bypasses malformed-record fences

**Location introduced by this increment:** `garden/internal/ingest/capture_activity.go:121-124`. **Underlying existing helper:** `laputa/evolution/contracts.go:82-94`, especially `:91`.

The old ingestion decoder performed a second Decode and required `io.EOF`. The increment replaces it with `evolution.DecodeStrictJSON`. That helper uses `Decoder.More()` after decoding the root object. `More()` checks for another array/object member; an extra closing `}` or `]` returns false instead of proving end-of-input. The duplicate-key walk also stops after the first root value.

The independent probe creates a real valid v3 policy closure, then appends a single extra `}` or `]` to its persisted `activity_json`. For each invalid document, all these paths accept it:

- `ActivityProjection` returns no error.
- `HighWatermark` returns row sequence 1 with no error.
- `ArchiveSession` returns success.
- Real `Start` returns success and omits the already-closed consumer.

This defeats the increment's explicit rule that malformed metadata cannot suppress recovery or advance the source fence. It does not demonstrate a forged native C1 proof or broader authority escalation; the concrete defect is accepting a malformed durable document as a valid closure.

**RED:** `probes/reviewer_encoding_test.go`, `TestReviewerS5ClosureRejectsTrailingDelimiters`, both delimiter cases fail with all four observed acceptances. Original output is retained unchanged.

**Smallest fix:** require exactly one complete JSON document, including end-of-input validation, while retaining unknown-field and duplicate-key rejection. A focused guard in the ingestion decoder can keep the fix local; alternatively correct the shared helper with appropriate affected-consumer regressions. A second Decode expecting `io.EOF` (or an equivalent complete-document check) must reject both extra closing delimiters. Keep all Start/drain/watermark/archive paths on the same validated decoder.

**Classification:** new ingestion regression from reusing an existing deficient helper. The helper defect is inherited; replacing the previous explicit EOF check makes it newly reachable in this increment's metadata/fence paths.

## Verified behavior and review observations

The independent fixed-source focused runs pass 50 tests/subtests across three package results. Together with source inspection, these establish the tested boundaries outside the two findings:

- Actual `runtimecore.Open` installs the separately resolved trusted `RecapBinding` before ingest Start. Legacy `ProfileID` is not reused as new C1 identity. Real Open/restart and observation beyond the retry interval pass.
- Policy closure consults the existing native `ReadRecap` and requires positive current revision. Prefix-only, foreign/incomplete/IO-invalid owners do not authorize closure. Native live-unknown refusal and close/reopen qualification tests pass. The existing same-byte recovery publication remains intact.
- Archive-only lookup returns the actual native IDs/provenance and explicit legacy revision zero. Complete original zero and nonzero snapshots recover correctly. Missing/partial/conflicting evidence retains the original uncertain effect separately from policy disposition; archive IO/malformed errors remain visible in the tested cases.
- Already applied receipts take precedence over current policy. Failed durable closure persistence does not acknowledge or remove the fence. Raw accepted/running/spooled processing remains independently eligible.
- Start, drain, watermark and archive read ordered SQL snapshots through `activityRows`; rows are closed before native owner IO. The tested earlier unresolved row continues to fence the watermark. Concurrent drain/archive/watermark coverage passes in the selected normal run. No additional lock-order defect was substantiated; no new race execution is claimed.
- Session finalization under confirmed v3 avoids the denied FoldSession and does not claim a new capsule. Existing v1/v2 capture recovery and all tested legacy v3 mutator denials preserve authority/archive bytes.

## Independent execution and reproducibility

Every test invocation used `/tmp/issue58-localonly-runner.py`, archived verbatim as `probes/localonly-runner.py`: offline dependency resolution, credential-bearing environment names removed, local-only fixtures, and a non-forwarding external deny proxy. All four runs recorded zero external proxy requests and zero forwarding. Go was `go1.26.4 linux/amd64`, `-p 1`, `-count=1`; no browser, external model or tenant data was used.

Native modules retain their declared dependencies. Garden resolves sibling Laputa/Mentle through existing local replacements. Native Laputa declares Eino `v0.9.13` and Inofy `v0.0.0-20260930141905-71e2c9bbe47d`; Vivy's separate Eino `v0.9.20` is not substituted or claimed for this execution. go.mod/go.sum hashes are included.

Counts below are Go JSON terminal events, including parent tests and subtests. Package outcomes are separate; parent failures are not additional distinct defects.

| Log stem (prefix `issue58-s5-review-`) | Test pass / fail / skip | Package pass / fail | Exit | Seconds |
| --- | ---: | ---: | ---: | ---: |
| `snapshot` | 2 / 3 / 0 | 0 / 1 | 1 | 2.478 |
| `encoding` | 0 / 3 / 0 | 0 / 1 | 1 | 1.224 |
| `garden-focused` | 36 / 0 / 0 | 2 / 0 | 0 | 6.696 |
| `actmem-focused` | 14 / 0 / 0 | 1 / 0 | 0 | 0.720 |

`results.json` and each `raw/*.json` retain exact commands/candidate/runner metadata. `raw/*.log` are unchanged original Go JSON streams, including every failed assertion; no compile/interrupted test attempts occurred. The two overlay mappings are external-only and point at immutable probe files. Reproduce RED by running the recorded commands from their recorded Garden clone through the runner. Root may relocate only overlay target paths to a new fixed clone while retaining original probe bytes.

The author separately reported normal/race passes of Garden161+ACTMEM120 tests per lane. Those working-source logs name merge HEAD; the author's per-file hashes bind their source to the checkpoint. They are supporting author evidence, not executions performed by this reviewer or grounds to relabel our RED results.

## Handoff and pending gates

Root should assign one consolidated author TDD pass for I1 and I2, then rerun these unchanged probes against a newly fixed immutable candidate and run affected normal/race qualification. This reviewer does not perform a redundant second review of that same fix diff. Full native module/published-pin qualification, actual Windows execution, Vivy consumer activation and whole-candidate S6 remain separate root-owned gates. Windows-specific source was inspected but NOT executed; no Windows, PostgreSQL, deployment or live-model acceptance is claimed.

`source-hashes.json` records source identity and clean status; `raw/source-final.json` records the final source freeze. `hash-manifest.sha256` covers this report, source evidence, probes and raw logs (excluding the manifest itself). No additional finding is inferred solely from explicitly pending downstream work.
