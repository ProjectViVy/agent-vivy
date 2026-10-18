# E9-05 Bounded Diagnostic Continuation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans after implementation authorization. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Selectively recover #32 R3 bounded physical reads and final encoded record limits while retaining main's R1 append/platform fixes.

**Architecture:** Repair existing Diagnostics reader/cursor/clip as one concern. Count reads before materialization, carry oversize-line discard state across pages, and validate final JSON envelope size.

**Tech Stack:** Go filesystem/JSON/UTF-8; existing Diagnostics/RPC, no new dependencies.

**Spec:** Old #32 P5.1 at `6045c0a...`, especially implementation `57bbe2ce3385a7eaf74ebe480bd14babb678b437`; architecture.md D-DIAG. Status: package.md.

## Global Constraints

500 records/page; <=8192 serialized bytes/record; <=4096000 physically read bytes/request, including anchors. These are existing contract limits. Preserve current `fileIdentity(info,f)` platform behavior and source/date/containment checks. R2 generic redaction is superseded; do not revive it. No production logs or Journal are opened for tests.

## Review Focus

One huge physical line; JSON escapes/multibyte envelope fields; page budget ending inside that line; partial trailing UTF-8 line; normal append versus replacement/truncate-regrow. Tests must bound actual reads and final encoded bytes, not only a counter after ReadBytes.

## Files and interfaces

Modify `internal/logging/diagnostics.go`; read existing platform identity files and keep current signatures. Recover branch-only `internal/logging/diagnostics_p5_test.go`; extend `internal/logging/diagnostics_test.go`, `internal/rpc/diagnostics_test.go` only for missing coverage.

Preserve `Diagnostics.Read(ctx context.Context, q DiagnosticQuery) (DiagnosticPage, error)` and public DTOs. Reuse private old P5.1 `diagCursorV2{Date string,FileID uint64,Size int64,Offset int64,Anchor string,DiscardLine bool}`, `diagCursorV2Encode`, `diagCursorV2Decode`, `diagAnchor(f *os.File, offset int64, budget *int64) (string,error)` and counted reader. `v2.` raw-URL JSON decode capped at 2048 bytes, unknown fields/negative sizes/offset beyond size fail. Anchors hash at most prior 64 bytes and count against scan budget. Legacy five-part token resets once with Gap.

## Tasks

- [ ] Recover `TestDiagnosticsOversizeContinuationKeepsLineAlignment`, `TestDiagnosticsScanBudgetCountsPhysicalReads`, `TestDiagnosticsEveryEnvelopeFieldBounded`, `TestDiagnosticsIncompleteLineWaitsForNewline`, `TestDiagnosticsRotationAndTruncateRegrowGap`, legacy/malformed/replacement tests. Retain already-fixed append continuation assertions without reintroducing old file identity.
- [ ] Construct line longer than 4096000 followed by `tail`; assert first request bounded, continuation discards remainder until newline, no midline fake record and no repeated clipped record. Verify counted reads include incoming/outgoing anchors. Incomplete final JSON split inside UTF-8 waits at original line start, then emits once after append.
- [ ] Put 16KiB into component/level/message/structured fields with escapes/multibyte text; assert JSON valid, UTF-8 valid, `Truncated=true`, `len(encoded)<=8192`. Same-identity append returns only new rows, actual replacement/shrink/regrow reports Gap. Run `go test ./internal/logging -run '^TestDiagnostics' -count=1 -v` red before repair; distinguish R3 failures from existing R1 passes.
- [ ] Port bounded fragments/discard cursor/anchor logic from old P5.1 into current reader. Preserve descriptor stat/platform identity and validate resolved/opened file identity. Never replace current Unix/Windows implementation with old branch's inode-only helper. Bound materialization to raw prefix and counted limited reads.
- [ ] Port final envelope clipping: drop fields first, bound all envelope strings as valid UTF-8, include Truncated flag and JSON escaping in final marshalled-byte check. Do not slice by unvalidated negative/oversized indexes or assume message length equals serialized length.
- [ ] Run logging full/race and actual RPC suites: `go test ./internal/logging -count=1`, `go test -race ./internal/logging -count=1`, `go test ./internal/rpc -run '^TestDiagnostics' -count=1`. RPC appends one GUI record, reads, appends another, continues and sees only the latter. On native Windows, execute replacement/descriptor tests; cross-compilation alone is pending native evidence.
- [ ] Run `just ci`. Record measured byte counts, cursor reset behavior and platform results in future Story log. Independent reviewer checks allocation/read bound, stale token/error behavior, final encoding and main platform fixes; commit explicit paths with human attribution.

**Acceptance:** R3's physical and serialized envelopes hold under oversize/escape/continuation stress; append remains continuous; rotation and incomplete lines remain honest. No raw payload redaction requirement is added.

**Stop conditions:** Cannot preserve platform identity with old helper; public cursor compatibility requires a broader contract decision; unavailable native platform. Retain completed Linux/RPC checks, name blocked native row and avoid acceptance inflation.
