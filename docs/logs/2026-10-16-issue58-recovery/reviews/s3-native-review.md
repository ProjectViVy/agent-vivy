# Independent S3 native Recap review

## Decision

**Changes required: two Important findings. No Critical or Minor findings.** This is independent review of the frozen native Task 2 production candidate and its direct Task 1 effect, not integrated S3 or Issue #58 acceptance. No candidate source files were edited, no commits were created, and no agents were delegated.

## Exact source and scope

- Worktree: `/workspace/scratch/5dda642e718d/agent-vivy-issue58-s3-native-validation`.
- Reviewed HEAD: `b26ee856cea6bb1fd10916b44fe28bcb12491841`; tree: `982f37f5bc4f534097b3ac07da04d95c387fb6bd`. `git status --porcelain` was empty before and after inspection/probes.
- Diff baseline: accepted Task 1 `96adc4f7b49f25f4f67b1c583597d035ab2cf371`. Historical audit-document additions were not treated as new source behavior.
- Root-supplied published exact-tree counterpart: `ca99c2f1f66ab2128a7f2fe366de1c0d6de73749`, draft PR #70. Main `5715950` is not the review source. Remote equivalence was supplied by root, not independently fetched here.
- Verified local dependency: sibling Laputa `bb57e93a61cfb41460524d00a557a3a9f2cce1e7`, clean; Go `go1.26.4 linux/amd64`; local Eino `v0.9.20`. Built `ui/dist` was supplied for app embedding.
- Read root `AGENTS.md`, `.agents/skills/vivy-eino/SKILL.md`, review brief, current recovery-root S3 plan including pending-intent and rowless-observation rulings, design revision 3, author Task 2 report, relevant production/tests and pinned native implementation.
- Root subsequently supplied fixture-only `f372626df642116fa3b8a96f0a30260828432558`. Its inspected delta adds `svc.deps.ToolEvidence = b.Blobs()` in `internal/runtime/subject_evidence_test.go`. It changes no reviewed production code. All tests below remain explicitly qualified to b26, not f372 or a later candidate.

## Findings

### Important 1 — Exact correction subsets are emitted as incomplete live tool exchanges

**Location:** `internal/runtime/recap_compaction.go:267–294`, specifically `projectFeedWithContextBudget(..., selected, ...)` and direct append at 290–294. The unchanged projector in `internal/runtime/context.go:291–305` preserves `RoleTool` as `schema.ToolMessage` and assistant tool calls as live `ToolCalls`.

**Evidence:** `populateRecapFold` correctly selects only captured `target.MessageIDs`, but those IDs need not contain a whole tool exchange. A rewind/edit can target the retained tool result alone. The summary input then contains a `role=tool` message whose `tool_call_id` has no preceding assistant call. A call-only subset similarly leaves an unanswered tool call before the receipt/system message. No native summarization hook repairs this input: `GenModelInput` returns `f.input` unchanged.

Reviewer overlay `TestReviewNativeCorrectionToolSubsetHasValidWire` ran an actual governed tool Run through the local OpenAI HTTP adapter, captured/sealed its real retained originals, seeded the covered head, and recorded a rewind marker selecting only that tool-result row. The actual native summary HTTP request failed the structural assertion:

> actual summary HTTP has orphan tool result at message 3, id="review-call"

This is a real outbound-wire defect, not an inference from a scripted return value. The fixture accepted it because `newIssue58Wire` only checks `stream` and returns a response; its acceptance does not establish provider protocol conformance.

**Impact:** Required reconciliation for legitimate partial tool-message corrections can be rejected by a conforming provider, preventing both explicit correction and subsequent LIFE admission. A permissive endpoint can let the test pass and even commit a receipt despite the malformed protocol shape. Existing tool-result bytes remain retained; this finding does not claim data loss.

**Smallest correction:** Project the exact authorized correction fragments into explicitly delimited historical data with full role/call-ID/name/arguments/body fields, rather than emitting selected fragments as active tool protocol turns. Preserve exact canonical/view mapping and actual-used receipt hashing; do not widen references to unrelated originals or invent missing tool calls/results. Reuse the current native summarizer and projector/provenance seams. Add strict actual-wire cases for result-only, call-only, terminal-only, and mixed/subset parallel-tool corrections, including Notebook exclusion.

### Important 2 — Explicit native summaries are outside Service cancellation, drain and idle fences

**Location:** new selected-policy branch `internal/runtime/compaction_service.go:145–164`; standalone observation setup `internal/runtime/recap_compaction.go:501–527`. Relevant existing consumers: `service.go:725–745` (`ChangeModelWhenIdle`), `service.go:1895–1934` (`CancelAll`/`WaitIdle`), `compaction_service.go:499–544` (engine reload), and `internal/app/app.go:1680–1727` (drain, then Garden/storage close).

**Evidence:** The branch briefly checks `active`/`pending` and invokes native generation synchronously, but never registers its cancellation or lifetime with Service bookkeeping and never observes the stopping admission fence. Its rowless Journal identity is not itself a registered operation. `WaitIdle` only waits on `wg`/`goalWG`, and `CancelAll` only sees registered active/pending work.

Two independent reviewer overlay regressions blocked the actual summary HTTP response while exercising the existing host-facing gates:

- `TestReviewNativeExplicitSummaryParticipatesInDrain`: **FAIL**, `Service.WaitIdle reported drained while explicit native summary HTTP was still blocked`.
- `TestReviewNativeExplicitSummaryBlocksModelSwitch`: **FAIL**, `model switch accepted during explicit native HTTP: err=<nil> persisted=true`.

The tests released and settled the operation afterward, so their cleanup did not fabricate an unfinished worker. Engine reload has the same `active`/`pending` omission by source inspection; no engine-reload race execution is claimed.

**Impact:** Normal `App.CloseContext` can report Service drained and close Garden/SQL while the explicit model observer is still generating or settling usage/finish events. That breaks the required observer-drain-before-owner-reopen boundary and can lose required settlement records. Settings can also persist a model change/rebuild while one summary is using an older engine/mapper; later route/capacity checks then read mutable current settings. The existing App reopen test closes only after compaction has returned, so it does not exercise this cut.

**Smallest correction:** Put explicit compaction inside the existing Service admission/cancel/drain/idle bookkeeping, atomically excluding new starts once stopping begins. Pin its engine and effective route for the operation and ensure model/settings idle guards include it. Keep it rowless: no RunStore row, user/tool message, terminal event, C2 seal, scheduler or second runtime. Add a blocked-provider cancellation/shutdown test proving usage/finished settlement precedes owner/storage close, plus model-switch/reload admission checks.

## Architecture and integrity observations

Implemented and supported by inspection/focused tests:

- Actual Eino `summarization.New` / concrete `TypedMiddleware[*schema.Message].Summarize` is used. Local upstream `summarization.go:281–332` confirms generation → custom Finalize → Callback → optional after-event → return. `GenModelInput` supplies the selected input; custom Finalize validates free text and commits C1 before consumed admission reconstruction.
- `EmitInternalEvents=false` is consistent with admission/explicit scopes lacking ADK Run event context. The real host Callback journals `context.compacted`; its error is recovered through exact OperationID lookup. No native after-summary transport emission is asserted.
- Native fallback is bounded, runs through the existing observed summary model, uses route-specific capacity and the 3072 output request/reservation. Foreground summary uses the active run ledger; explicit summary uses the accepted rowless control Journal contract. Existing usage queries omit those rowless records; the ordinary usage UI is not qualified by this review.
- Complete-prefix selection is continuous and only shrinks at interaction boundaries; the newest complete interaction is retained. No 200 KiB legacy break or duplicate SQL cursor was added. Post-commit foreground input is rebuilt through `prepareRecapContext`, not directly installed from native returned summary messages.
- Correction metadata is scoped to exact captured canonical/view mappings and committed with the versioned receipt. Whole-interaction provenance classification precedes subset selection. The protocol issue above remains despite that sound provenance ordering.
- Current authorization/capture and revision/epoch checks precede provider and Finalize boundaries. CAS/disposition conflicts permit one fresh recomputation. Original evidence stays in existing stores.
- Pending metadata uses the existing BlobStore/listing capability, canonical strict decoding, immutable operation-key digest, read-back verification and exact exposed C1 receipt-field comparison. Positive receipts survive advisory cleanup failure; unknown/error retains intent. Native owner `recapUnknown` freezes authoritative operations until same-owner lifecycle reopen. No second store/parser/owner/recovery API was found.
- Existing selected-policy provider-leg permission/epoch enforcement and Coding/worker isolation are retained by the production diff. This review did not rerun the entire separately accepted Task 1 matrix.

## Executed validation

Every test command ran only through `/tmp/issue58-localonly-runner.py`; metadata records credential stripping by name, offline dependencies and zero external forwarding/rejected external requests. Reviewer tests were supplied by Go's `-overlay`; candidate files stayed unchanged. The overlay replaces only the test file with its exact b26 contents plus the reviewer regressions. The final overlay copy contains all three new tests; the earlier drain invocation used the same first test before the later tests were appended.

Commands (worktree argument for each is `/workspace/scratch/5dda642e718d/agent-vivy-issue58-s3-native-validation`):

```text
python /tmp/issue58-localonly-runner.py <worktree> review-native-focused go test ./internal/runtime ./internal/app -run '^TestIssue58(ContinuousRangeCompaction|ZeroNewReconcile|ReconcileCertificateReset|DispositionCaptureRace|SummaryCASRecompute|CompactionCrashCuts|CommittedCorrectionProvenance|PendingIntentIntegrity|SummaryRouteCapacityAndFallback|ExplicitCompactionControlLane|ClearInvalidatesResume)' -count=1 -json
python /tmp/issue58-localonly-runner.py <worktree> review-native-drain go test -overlay=/tmp/review-native-overlay/overlay.json ./internal/runtime -run '^TestReviewNativeExplicitSummaryParticipatesInDrain$' -count=1 -json
python /tmp/issue58-localonly-runner.py <worktree> review-native-correction-wire go test -overlay=/tmp/review-native-overlay/overlay.json ./internal/runtime -run '^TestReviewNativeCorrectionToolSubsetHasValidWire$' -count=1 -json
python /tmp/issue58-localonly-runner.py <worktree> review-native-model-switch go test -overlay=/tmp/review-native-overlay/overlay.json ./internal/runtime -run '^TestReviewNativeExplicitSummaryBlocksModelSwitch$' -count=1 -json
```

| Label | Result | Actual selection | Elapsed |
|---|---|---|---:|
| review-native-focused | PASS, exit 0 | 45 test/subtest runs; 47 pass events including 2 packages; zero fail/skip | 30.166 s |
| review-native-drain | RED, exit 1 | 1 intended test executed and failed on drain assertion | 12.524 s |
| review-native-correction-wire | RED, exit 1 | 1 intended test executed and failed on actual-wire orphan assertion | 12.601 s |
| review-native-model-switch | RED, exit 1 | 1 intended test executed and failed on idle-model-switch assertion | 13.721 s |

Raw logs/runner metadata, complete reviewer overlay and SHA-256 manifest are in adjacent `evidence/`. This is not a surface PASS from an empty test selection.

## Remaining limits and acceptance ownership

No full `just ci`, independent race run, native Windows execution, real-provider/model quality test, installed #66 integration or UI acceptance was performed by this reviewer. No PostgreSQL DSN was configured; the focused selection did not execute PostgreSQL tests, and separately reported PG skips are not qualification. Over-200-KiB interaction segmentation, every pending-artifact I/O fault, cancellation settlement at active host close, concurrent summary sources and post-commit recapture faults are not all independently exercised here. Root/author wider runs are separate evidence and are not relabeled as reviewer execution.

The author/root are already retaining a full-focused RED due to missing BlobStore wiring in older positive fixtures, with subsequent fixture fixes and source-qualified rechecks. This report does not hide that RED or accept later trees without verification. Fix the two production findings, then qualify the exact consolidated source and required gates. Root retains architectural and integrated acceptance; whole S3/Issue #58 completion is not claimed.

## Late integration observation (not an additional finding)

The author/root raised receipt reconstruction ordering after the main review. Source confirms that `populateRecapFold` hashes correction references/expanded input in its current in-memory order, whereas `encodeRecapReceipt` sorts a JSON copy of references, targets and aligned message pairs. I did not establish a real captured C2 case with differing order before review close, so this is not counted as a third finding. In the consolidated fix, verify that the committed historical envelope alone determines the exact expansion/hash order after pending metadata is deleted; canonicalize before model input/hash through the shared codec if needed. Do not recover ordering from later live markers.

No concrete ordinary-Coding behavior regression was established from the earlier observer-context construction. The inspected ordinary admission path only folds existing stored compaction records; `foldToCompactionRecord` is called by explicit compact or later overflow recovery, so the hypothesized new pre-run Coding summary call was not demonstrated. No extra finding or redesign request is made on that basis.
