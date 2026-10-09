# P5: Diagnostics and mandatory settlement implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Keep diagnostic continuation bounded and truthful, and make every mandatory model settlement failure reach the caller.

**Architecture:** Repair the existing Diagnostics reader/cursor together so bounded scans retain physical-line alignment. Repair the existing model observer wrapper and Service persistence seam; Eino remains the model/stream provider, and Journal remains the sole durable owner.

**Tech Stack:** Go `1.26.4`, JSON, filesystem descriptor identity, Eino `v0.9.13` StreamReader/Pipe, existing Journal.

**Spec:** [P5 contract](../../specs/2026-10-09-issue32-remediation-design.md#p5-diagnostics-and-mandatory-model-settlement).

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

- [ ] **Step 1: Write cursor/line regressions.** `TestDiagnosticsAppendContinuationStable` reads "one", appends "two" -> Gap=false and only "two". `TestDiagnosticsRotationAndTruncateRegrowGap` covers replace, shrink and same-file overwrite/regrow -> Gap=true. `TestDiagnosticsOversizeContinuationKeepsLineAlignment` has a line larger than 4096000 followed by "tail" -> per-call byte counts stay bounded and "tail" appears exactly once, never a middle fragment. `TestDiagnosticsIncompleteLineWaitsForNewline` appends a split UTF-8/JSON line -> no partial duplicate or lost suffix. Include malformed v2 and one-time explicit legacy reset.
- [ ] **Step 2: Write serialization/budget regressions.** `TestDiagnosticsEveryEnvelopeFieldBounded` puts 16KiB into component, level, message and structured fields, including multibyte characters/control escapes -> `len(json.Marshal(record)) <= 8192`, valid UTF-8, Truncated=true. `TestDiagnosticsScanBudgetCountsPhysicalReads` uses a counted reader and a huge line -> actual bytes read including anchor <=4096000, bounded retained prefix, <=500 records. Include exact-boundary and empty/filter-only pages. Preserve dummy nested text under #40; no redaction assertion is added.
- [ ] **Step 3: Verify red.** Run `go test ./internal/logging -run '^TestDiagnostics(AppendContinuationStable|RotationAndTruncateRegrowGap|OversizeContinuationKeepsLineAlignment|IncompleteLineWaitsForNewline|EveryEnvelopeFieldBounded|ScanBudgetCountsPhysicalReads)$' -count=1`. Expected: baseline append marks gap and envelope/scan cases exceed bounds.
- [ ] **Step 4: Implement bounded continuation.** Open/validate descriptor before identity decisions. Growth with matching identity/anchor continues; replacement/shrink/anchor mismatch resets with Gap. Wrap file reads in an io.LimitedReader whose remaining budget includes anchor reads; bounded fragments keep at most an 8192-byte record prefix. At scan exhaustion retain offset/discard state without an extra Peek read. Emit an oversized prefix once with Truncated=true, then skip its remaining fragments across calls. Preserve the start offset of a normal unfinished line. Legacy five-part cursors reset explicitly once; malformed tokens still return ErrDiagnosticQuery. Do not add path/glob input or weaken symlink rejection.
- [ ] **Step 5: Implement final JSON clipping.** Bound every string/envelope field on UTF-8 boundaries and account for escaping/Truncated overhead after dropping excess fields. Re-marshal while reducing bounded content to guarantee final bytes <=8192; keep the required identity/message/truncated envelope. This is size clipping, not pattern filtering.
- [ ] **Step 6: Verify green and actual RPC.** Run the focused command, `go test -race ./internal/logging -count=1`, `go test ./internal/rpc -run '^TestDiagnostics' -count=1`, and a diagnostics/logs read-append-read through the actual control adapter. Expected: no old-row replay, bounded reads/records and unchanged family/path authorization. Exercise Windows file identity in native CI; mark it pending if unavailable locally.
- [ ] **Step 7: Commit.** Commit `fix(logging): bound diagnostic continuation and record envelopes` and record exact measured byte bounds in the phase log.

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

- [ ] **Step 1: Write wrapper terminal regressions.** `TestObservedGenerateSettlementFailure` returns a sentinel End error after successful provider output -> caller errors.Is sentinel, no success. `TestObservedStreamSettlementFailure` emits chunks, then End fails -> caller sees settlement error before EOF. `TestObservedErrorsJoinSettlement` covers Generate/Stream setup/provider/chunk errors -> both sentinels survive and End count=1. `TestObservedCloseDuringSettlementDoesNotBlock` closes/cancels the downstream and asserts the pump/upstream close complete through deterministic channels, with no duplicate End.
- [ ] **Step 2: Write real persistence failure regressions.** `TestModelSettlementJournalFailure` injects failure specifically at final usage and model.call.finished in the real runModelCallObserver. Assert the actual storage sentinel survives to Generate/Stream, a successful Run is not reported, mandatory terminal attempts remain bounded and quota exemptions still apply. `TestModelSettlementFailureIsNotRetried` configures the actual retry path -> provider call count=1. Cover child/summary routes and cancellation, not just a fake observer.
- [ ] **Step 3: Verify red.** Run `go test ./internal/runtime -run '^Test(Observed(GenerateSettlementFailure|StreamSettlementFailure|ErrorsJoinSettlement|CloseDuringSettlementDoesNotBlock)|ModelSettlement(JournalFailure|FailureIsNotRetried))$' -count=1`. Expected: baseline ignores End or hides raw Journal cause; the provider call may appear successful.
- [ ] **Step 4: Implement the existing adapter's terminal behavior.** Generate returns errors.Join(call/chunk error, End error). Stream has one terminal closure: settle once, send any joined terminal error before close, and respect downstream closure/cancellation so Send cannot block forever. Do not replace Eino stream machinery or start another consumer loop. Run the pinned Eino capability check recorded in the spec before adding the adapter changes.
- [ ] **Step 5: Preserve the actual persistence cause.** Move the existing append/publish body into persistAndPublishErr; the bool wrapper compares its result to nil. Keep the same projection mutex and best-effort terminal policy. Return the real Append/deletion cause from mandatory End, wrap it as modelSettlementError, and prevent the existing retry seam from treating it as a provider retry. A permanently unavailable Journal remains an explicit evidence gap; do not fabricate a finish record or replay model effects.
- [ ] **Step 6: Verify green.** Run focused tests, `go test ./internal/runtime -run 'Test(Observed|Model|Cognitive)' -count=1`, and the affected streaming/cancellation race checks. Expected: both causes unwrap, End=1, no goroutine leak, no automatic replay, and finish/settlement ordering remains before downstream terminal observation.
- [ ] **Step 7: Commit.** Commit `fix(runtime): propagate mandatory model settlement failures`. Record fault positions, caller outcomes and unrecoverable Journal limits.

## Phase exit

Run integrated VIVY `just ci` and existing model/observer SDK conformance.
Record actual adapter continuation and synthetic Journal-failure outcomes.
P7 regenerates the final source-bound evidence; R2 remains superseded throughout.
