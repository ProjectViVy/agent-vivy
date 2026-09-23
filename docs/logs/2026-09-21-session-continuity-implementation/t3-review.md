# T3 independent review artifact

Date: 2026-09-23. Reviewer: independent read-only subagent (shared contract, `T3.md` review focus, T3 diff range `3519c73`..`a4d8996a`). Scope: governed history projection, model tools and inspection RPC.

## Verdict (initial pass)

MATERIAL FINDINGS — acceptance blocked until fixed. Clean areas confirmed by the reviewer: projection allowlist, redaction-before-matching, cursor signing/binding/size guards, canonical selection digest encoder, page bounds, and the absence of an HTTP/RPC actor-spoof path.

## Findings

1. **Material — modern v2 assistant content unreachable.** `projectHistoryCandidateWithLimit` skipped every `msgp_`-prefixed message row, while `model.completed` v2 events projected an empty truncated placeholder. Because the message projector persists verified assistant text for both v1 and v2 completions as `msgp_<run>_<seq>_0` rows, modern assistant turns could never be found or read. The fixture masked this by seeding only a v1 completion and a legacy row.
   **Fix:** `msgp_` assistant rows are now the canonical modern assistant text (projected, sanitized like any message). Known-version `model.completed` events (v1, v2) are suppressed to keep the no-duplication rule; unknown versions keep the honest truncated placeholder. `msgp_` tool rows stay skipped because `tool.finished` events remain canonical. Fixture seeds persisted `msgp_` rows for both a v1 and a v2 completion and asserts each text is reachable exactly once with message provenance.

2. **Material — legacy compaction fabricated precision.** `context.compacted` payloads without `before_tokens`/`after_tokens` rendered `compaction <mode>: 0 -> 0 tokens`, inventing measurements the journal never recorded.
   **Fix:** token counts render only when both fields are present; legacy payloads render `compaction <mode> (legacy record; token counts unavailable)`. Fixture seeds a token-less legacy compaction event and asserts the reduced-precision text.

3. **Minor — dead code.** `sortHistoryItems` had zero callers; `projectHistoryCandidate` was a test-only wrapper around `projectHistoryCandidateWithLimit`.
   **Fix:** both deleted; unit tests call `projectHistoryCandidateWithLimit` with `domain.DefaultContinuityLimits().ResultItemBytes` directly. The stale `TestHistoryProjectionSkipsModernProjectedMessage` was replaced by `TestHistoryProjectionProjectsModernAssistantRow` and `TestHistoryProjectionSkipsProjectedToolRowAndCompletionEvent`.

4. **Minor — RPC capabilities fallback fabricated support.** `historyCapabilities` returned a hardcoded kinds/filters map when the injected service did not implement `tools.HistoryCapabilitiesOperations`, violating "the GUI never fabricates missing support".
   **Fix:** the fallback now fails closed with a `MethodNotFound` error (`history capabilities are not supported`). The shipping `HistoryService` implements the interface, so real deployments are unaffected.

## Re-review of fixes

Date: 2026-09-23. Reviewer: independent read-only subagent. Scope: fix commits `2fa25e3b` (findings 1–4), `50dc5ed3` (secret-audit comment), `a320f55f` (user-directed v2-only `model.completed` contract), `d3f4878b` (second-pass blocker fixes).

### First scoped re-review (of `2fa25e3b` + `a320f55f`)

All four original findings verified RESOLVED with file/line evidence: assistant `msgp_` rows emitted and known-version completion events suppressed (no duplication, honest placeholders for v0/v1/v3+); pointer-field compaction rendering; dead code deleted with zero residual references; RPC capabilities fail closed with `MethodNotFound`.

The same pass found two blockers and two minors introduced by the v1-removal commit `a320f55f`:

1. **Blocker** — `sdk/tui/live` still delivered a v0 content-bearing `model.completed` (`controller_test.go` `TestPackedLiveWireProjectsAuthoritativeCompletionAndToolCallIdentity`); suite red.
2. **Blocker (material)** — `ReconcileSessionMessages` hard-failed any session containing a pre-v2 completion event, making legacy journals unreadable through `session/get` / `session/messages`.
3. **Minor** — stale fixture comment in `history_service_test.go` claiming the projector persists rows for both completion versions.
4. **Minor** — `ui/src/lib/run-rows.ts` kept a v1 `content` fallback and `ChatView.test.tsx` seeded a v1-shaped payload.

### Second scoped re-review (of `d3f4878b`) — all four items RESOLVED

1. Live wire test streams `model.delta "completed only"` then a v2 completion; reviewer independently recomputed `sha256 = 28c5b3c0…cd6f`, 14 bytes — both match; original projection intent (assistant content + tool identity) preserved; test passes.
2. Reconcile tolerance is sentinel-scoped: `errUnsupportedCompletedVersion` wraps only the `PayloadVersion != 2` branch; digest/byte-length/field/decode errors remain plain errors and still abort reconcile (`TestMessageProjectorRejectsV2HashMismatch`, `TestMessageProjectorRejectsNonCanonicalV2Metadata` pass). The run-completion path (`projectRunMessagesLocked`) propagates the sentinel with no skip — still fails closed for fresh runs. `TestMessageProjectorToleratesLegacyV1Run` asserts legacy content is not freshly projected while the pre-persisted row stays readable (stronger than the test it replaced).
3. Fixture comment corrected; fixture digest `sha256("hash verified completion text") = 5ad22b08…`, 29 bytes, verified against the seeded seq-5 v2 event.
4. UI fallback removal verified safe: the v2 writer emits full-content `model.delta` events even for non-streaming completions (`internal/runtime/mapper.go`), so an empty buffer at completion means genuinely empty output. 14/14 UI tests pass.

Reviewer nit (immaterial, not blocking): the live `fakeEnv.deliver` sends `payload_version: 0` for deltas while the real backend emits 1; the validator checks versions only on `model.completed`.

### Verdict

**Acceptable for T3 acceptance.** No blockers remain. Reviewer-confirmed clean areas from the initial pass (projection allowlist, redaction-before-matching, cursor signing/binding/size guards, canonical selection digest, page bounds, no actor-spoof path) were untouched by the fix commits.
