# P5: Diagnostics and mandatory settlement implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Keep diagnostic continuation bounded and truthful, and make every mandatory model settlement failure reach the caller.

**Architecture:** Repair the existing Diagnostics reader/cursor together so bounded scans retain physical-line alignment. Repair the existing model observer wrapper and Service persistence seam; Eino remains the model/stream provider, and Journal remains the sole durable owner.

**Tech Stack:** Go `1.26.4`, JSON, filesystem descriptor identity, Eino `v0.9.13` StreamReader/Pipe, existing Journal.

**Spec:** [P5 contract](../../specs/2026-10-09-issue32-remediation-design.md#p5-diagnostics-and-mandatory-model-settlement).

P5.1 evidence: [summary](../../../logs/2026-10-09-issue32-p5.1-diagnostics-continuation/summary.md),
[verification](../../../logs/2026-10-09-issue32-p5.1-diagnostics-continuation/verification.md),
[acceptance](../../../logs/2026-10-09-issue32-p5.1-diagnostics-continuation/acceptance.md).

## Global Constraints

- Preserve one Service/Journal/policy path; no second model loop or persistence service.
- Keep Eino at `v0.9.13`; only runtime/provider import Eino.
- No built-in automatic redaction or generic argument veto; R2 is superseded.
- Diagnostic ceiling: 500 records, 8192 serialized bytes per record, 4096000 scanned bytes per request.
- Use temporary files/stores and synthetic errors; never read tenant Journal data.
- Run product `just ci` and relevant actual adapter smoke after implementation.

## Review Focus

- Same-inode truncate/regrow reaches the old size: anchor mismatch reports a gap (P5.1).
- Budget stops inside an oversized line: next page discards its tail instead of parsing a fake row (P5.1).
- Unicode/JSON escaping and oversized envelope fields: final encoded record never exceeds 8192 bytes (P5.1).
- Provider failure plus settlement failure: errors.Is retains both causes and End occurs once (P5.2).
- Consumer closes before settlement completes: no blocked producer or automatic model replay (P5.2).

---

## Prerequisites and file responsibilities

P0 supplies baseline and R2 disposition. P5.1 owns all cursor/reader/clip behavior
in Diagnostics. P5.2 owns model terminal propagation and an error-returning
helper in the existing Service path. Sequence P5.2 with P2/P6 Service edits.
No production probe files are copied into internal source; create genuine
repaired-behavior regressions in the established test files.

### Task P5.1: Stable bounded physical-line continuation (R1, R3)

**Files:**
- Modify: `internal/logging/diagnostics.go`; review existing `diagnostics_filestat_unix.go` and `diagnostics_filestat_windows.go` for descriptor validation without weakening their platform identity semantics.
- Test: `internal/logging/diagnostics_test.go`.
- Test actual RPC: `internal/rpc/diagnostics_test.go`.

**Interfaces:**
- Preserve `Diagnostics.Read(ctx context.Context, q DiagnosticQuery) (DiagnosticPage, error)` and all public DTO fields.
- Add private `diagCursorV2{Date string, FileID uint64, Size int64, Offset int64, Anchor string, DiscardLine bool}`. Encode as `v2.` plus raw-URL-base64 JSON; bound token decoding to 2048 bytes and reject unknown fields/negative sizes/offsets beyond recorded size.
- Add `diagCursorV2Encode(c diagCursorV2) string`, `diagCursorV2Decode(raw string) (diagCursorV2, error)` and `diagAnchor(f *os.File, offset int64, budget *int64) (string, error)`.
- Anchor hashes at most the 64 bytes immediately before offset with SHA-256 and consumes that physical-read budget. Use descriptor `Stat` and existing platform file identity; use path/descriptor same-file validation before accepting the opened file.
- Preserve `clipDiagRecord(record DiagnosticRecord) DiagnosticRecord`; make its final serialized output satisfy the exact ceiling.

- [x] **Step 1: Write cursor/line regressions.** `TestDiagnosticsAppendContinuationStable` reads "one", appends "two" -> Gap=false and only "two". `TestDiagnosticsRotationAndTruncateRegrowGap` covers same-file overwrite/regrow; `TestDiagnosticsReplacementFileReportsGap` covers replacement, and existing `TestDiagnosticsCursorResumeAndGap` covers shrink. `TestDiagnosticsOversizeContinuationKeepsLineAlignment` has a line larger than 4096000 followed by "tail" -> bounded cursor with persisted discard state and no middle fragment. `TestDiagnosticsIncompleteLineWaitsForNewline` splits a UTF-8 character inside JSON -> no partial duplicate or lost suffix. Malformed v2 and one-time legacy reset are covered.
- [x] **Step 2: Write serialization/budget regressions.** `TestDiagnosticsEveryEnvelopeFieldBounded` puts 16KiB into component, level, message and structured fields, including multibyte characters/control escapes -> `len(json.Marshal(record)) <= 8192`, valid UTF-8, Truncated=true. `TestDiagnosticsScanBudgetCountsPhysicalReads` verifies the counted reader reaches exactly 4096000 bytes including both cursor anchors; the oversize-page test verifies offset 4095936 plus the 64-byte outgoing anchor equals 4096000. Existing empty and filter-only page tests remain in place. No redaction assertion was added.
- [x] **Step 3: Verify red.** The baseline focused run failed append continuation (old cursor marked an append as Gap), same-file overwrite (no Gap), v2 cursor generation, incomplete-line waiting, final JSON clipping (163905 bytes), and one-time legacy reset.
- [x] **Step 4: Implement bounded continuation.** Open/validate descriptor before identity decisions. Growth with matching identity/anchor continues; replacement/shrink/anchor mismatch resets with Gap. The counted limited reader includes incoming/outgoing anchors in its 4096000-byte ceiling. Bounded fragments retain at most an 8192-byte raw prefix. Budget exhaustion persists discard state and emits at most one clipped record; continuation skips to the next newline. Incomplete final lines retain their start offset. Legacy five-part cursors reset once; malformed v2 tokens return `ErrDiagnosticQuery`. Symlink and source/date restrictions remain.
- [x] **Step 5: Implement final JSON clipping.** All envelope strings are valid UTF-8 and bounded; fields drop first; JSON escaping and the `Truncated` flag are included in the exact 8192-byte size check. Message prefix reduction is performed against marshaled output.
- [x] **Step 6: Verify green and actual RPC.** `go test ./internal/logging -count=1`, `go test -race ./internal/logging -count=1`, and `go test ./internal/rpc -run '^TestDiagnostics' -count=1` passed. RPC regression performs GUI append, logs read, GUI append, cursor continuation read and observes only the appended row. Windows test binary cross-compiles; native Windows file-identity execution and aggregate `just ci` remain pending.
- [x] **Step 7: Commit.** Code and regressions committed as `57bbe2ce3385a7eaf74ebe480bd14babb678b437` (`fix(logging): bound diagnostic continuation and record envelopes`). Exact measured budget is recorded in the P5.1 verification log.

### Task P5.2: Journal settlement errors on model terminal paths (R4)

**Files:**
- Modify: `internal/runtime/model_stream_observer.go`, `internal/runtime/service.go`.
- Test: `internal/runtime/model_stream_observer_test.go`; create `internal/runtime/model_settlement_failure_test.go` if real-Journal fault fixtures need a focused file.
- Read: pinned Eino model/callback/stream code and existing model retry/provider middleware.

**Interfaces:**
- Preserve `modelCallObserver.End(ctx context.Context, meta modelCallMeta, result modelCallResult) error`.
- Preserve existing observeGenerate/observeStream signatures and Pipe(8) backpressure.
- Add `Service.persistAndPublishErr(ctx context.Context, sessionID domain.SessionID, re domain.RunEvent) error`; existing `persistAndPublish(...) bool` wraps that same implementation. Preserve publication, projection and classified terminal behavior; never create a parallel appender.
- Add private `modelSettlementError{Err error}` with Error/Unwrap and a non-retryable classification at the existing retry seam; joined errors retain the provider cause as well as the Journal cause.

- [x] **Step 1: Write wrapper terminal regressions.** `TestObservedGenerateSettlementFailure` returns a sentinel End error after successful provider output -> caller errors.Is sentinel, no success. `TestObservedStreamSettlementFailure` emits chunks, then End fails -> caller sees settlement error before EOF. `TestObservedErrorsJoinSettlement` covers Generate/Stream setup/provider/chunk errors -> both sentinels survive and End count=1. `TestObservedCloseDuringSettlementDoesNotBlock` closes/cancels the downstream and asserts the pump/upstream close complete through deterministic channels, with no duplicate End.
- [x] **Step 2: Write real persistence failure regressions.** `TestModelSettlementJournalFailure` injects failure specifically at final usage and model.call.finished in the real runModelCallObserver. `TestModelSettlementFailureReachesGenerate` exercises a real Generate wrapper and Journal failure; `TestModelSettlementFailureIsNotRetried` configures the actual retry path -> provider call count=1. Main/child/summary routes, detached cancellation settlement, bounded terminal attempts, no successful Run, and quota exemptions are covered.
- [x] **Step 3: Verify red.** The pre-fix focused run reproduced the defect: successful Generate/Stream ignored End failures, joined provider/settlement causes were lost, actual runModelCallObserver discarded storage causes, and the retry path could treat a settlement failure as provider overflow. The new regressions failed before implementation.
- [x] **Step 4: Implement the existing adapter's terminal behavior.** Generate returns errors.Join(call/chunk error, End error). Stream has one terminal closure: settle once, send any joined terminal error before close, and respect downstream closure/cancellation so Send cannot block forever. The pinned Eino v0.9.13 Pipe/StreamReader path was inspected and covered by a deterministic close-during-settlement test; no alternate model loop was added.
- [x] **Step 5: Preserve the actual persistence cause.** Moved the existing append/publish body into `persistAndPublishErr`; the bool wrapper compares its result to nil. The same projection mutex and best-effort terminal policy remain. Mandatory End returns the real Append/deletion cause wrapped as `modelSettlementError`, and the existing retry seam refuses replay. A permanently unavailable Journal still cannot provide a fabricated finish record; the run is classified failed where the existing terminal append succeeds.
- [x] **Step 6: Verify green.** Focused observer/settlement tests, `go test ./internal/runtime -run 'Test(Observed|Model|Cognitive)' -count=1`, `go test -race ./internal/runtime -run '^Test(Observed(GenerateSettlementFailure|StreamSettlementFailure|ErrorsJoinSettlement|CloseDuringSettlementDoesNotBlock)|ModelSettlement)' -count=1`, and `go test ./internal/runtime -count=1` passed. `TestWorkflowProductCancelRun` passed 10 repetitions with child cancellation, list/detail parity, and duplicate-start refusal.
- [x] **Step 7: Commit.** Code and regressions committed as `900ca7db` (`fix(runtime): propagate mandatory model settlement failures`). Fault positions, caller outcomes and remaining Journal limits are recorded in the P5.2 evidence logs.

## Phase exit

Run integrated VIVY `just ci` and existing model/observer SDK conformance.
Record actual adapter continuation and synthetic Journal-failure outcomes.
P7 regenerates the final source-bound evidence; R2 remains superseded throughout.
