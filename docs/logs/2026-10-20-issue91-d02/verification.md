# Verification

## Baseline and ownership

- Base: codex/issue91@f5ce1c66, with stable unchanged by this task.
- Laputa: independent sibling checkout at ff3936f44ff8cf08c12af2cf698c194cfe474fd3.
- Go 1.26.4 is added to process PATH only. No dependency versions, global Go
  settings, database schema or production state are changed.
- Eino capability check: pinned v0.9.21 adk.NewTypedChatModelAgent,
  adk.NewTypedRunner, schema.AgenticMessage and existing native stream
  aggregation remain. The added helper only normalizes Vivy event metadata;
  it does not duplicate an Eino runner, middleware or transport.

## Commands and observed results

| Check | Result |
| --- | --- |
| New runtime/RPC/stream/facerun protocol tests before implementation | Expected RED: mapper emitted v2, historical projection checked digest, replay kept v2, live Faces rejected v3 |
| Web store test before implementation | Expected RED: wrong byte count did not set a protocol error |
| Null-delta regression | Expected RED, then fixed: null must not disappear as empty text |
| Empty selected_tools regression | Expected RED, then fixed: empty tool metadata serializes an array, not null |
| Repeated-sequence facerun regression | Expected RED (duplicate text and count mismatch), then GREEN after sequence deduplication |
| Focused runtime/RPC/app/SDK/stream/live suite | Passed: runtime 81.713s, RPC 30.281s, app 24.162s; shared Face packages passed |
| Final protocol/subscription/request subset | Passed: runtime 0.099s, RPC 5.183s, stream 0.041s, facerun 0.040s |
| Complete SDK facerun/stream/live packages | Passed; additional final facerun run 0.267s after restoring its dependency boundary |
| Independent faces/headless module | Passed, 0.104s; no Bubble Tea dependency added |
| Web framing/store/transcript subset | Passed, 92 tests; final store/framing/transcript/ChatView recheck passed, 98 tests |
| just ci (normal OS TEMP) | Failed at UI production build: existing esbuild temporary-file deletion Access denied. The final repeat also failed at the same build step; bootstrap/fmt/typecheck and 654 Vitest tests passed first |
| Dedicated OS TEMP subdirectory diagnostic | Same esbuild large-transform Access denied failure |
| just ui-ci (TEMP/TMP scoped to checkout-local UI build scratch only) | Final repeat passed: 83 files / 654 tests, typecheck, real Vite build (3.70s), i18n completeness and cross-Face checks |
| just vet (normal OS TEMP) | Passed again after review fixes |
| just test (normal OS TEMP) | Failed: initial attempt had 30 top-level failures in six packages; it overlapped final source fixes/UI rebuild and is not final-tree acceptance evidence. See dispositions below |
| just headless-compile | Passed again after review fixes |
| just plugin-ci | Passed, including independent headless and TUI modules |
| Independent read-only review | Reviewed f5ce1c66..85afcc07: three failure-path findings; all addressed in one RED/GREEN fix pass, detailed below. No second review or full-gate acceptance is claimed |

Focused command:

```text
go test ./internal/runtime ./internal/rpc ./internal/app ./sdk/tui/stream ./sdk/tui/live ./sdk/facerun -run 'TestModelCompletedV3|TestCompletion|TestReplayNormalizes|TestMapper|TestMessageProjector|TestTrajectory|TestModelCallPayloadSchemas|TestObservedCall|TestBudget|TestToolHook|TestNudgeAcceptance|TestRecoverAcrossModelGeneration|TestHeadless|TestAppRunsWithoutDigestAudit|TestProjection|TestInterpret|TestPackedLiveWire|TestRunSubscription|TestDigestModelRequest' -count=1 -timeout 35m
```

The command above records the original focused run; the request tests were
later renamed TestDescribeModelRequest*. Final renamed request/tool-trace,
subscription and protocol cases were rerun successfully.

Final added history checks cover real run/subscribe output consumed by the
shared strict stream core, and v1 persisted-message reads with explicit raw
replay rejection. D01's causal App regression additionally asserts emitted
request v4 contains neither digest keys nor the file nonce, and completion
v3 contains only the expected byte count. Existing accounting/recovery tests
exercise the preserved >=3 request charging path with new v4 emissions.

## Browser real path

Used testing-vivy-ui's frozen environment provider session with a fake key,
127.0.0.1:9911 scripted HTTP/SSE provider, backend 127.0.0.1:8788 and Vite
127.0.0.1:3015. VIVY_CONFIG is explicit and VIVY_USER_HOME points only inside
this checkout's ignored smoke directory. Only read_file is enabled.

The fresh home required Persona initialization; completed it through the
browser. The first default-workspace attempt failed to locate nonce.txt;
created a separate session explicitly bound to the isolated workspace and
repeated the test. The real tool succeeded and the subsequent provider
request contained d02-call and the fresh file nonce. The page showed one
preamble, successful tool, Unicode final answer and completed/idle state.
Reload retained the same transcript. A transient Work control binding error
on the API-created session cleared after its run and refresh; no fix or
cross-session Work acceptance is claimed.

All three smoke processes were stopped. See [browser screenshot](browser-smoke.png).
No live provider or production Journal was used.

## Evidence and residues

D01's conformance blocker remains. The current producer binds all internal
source/test files, so D02 changes that identity again. The headless source
also changed through its actual standalone tests; the canonical source-hash
tool updated both existing Go/YAML declarations to
c2168c5bf67d96348a6ba7b0e73ff8764b7439a1df9740a2affd79a6bf922381.
That declaration is not a conformance pass claim. The checked-in producer
results and its older source expectations remain pending a successful real
producer run; no passing rows were fabricated or manually relabeled.

Old content_sha256 is read only as an ignored historical v2 key in the
normalizer and explicit legacy/negative test fixtures. ModelCompletionTracker
remains only for byte framing, with no hash state. V3 Verify functions check
byte counts and UTF-8 only. Checkpoint/prompt/approval/source hashes outside
this selected protocol remain owned by later #91 cuts. Necessary actionhost
Journal auditing and nudge/warming/steering are unchanged. Generated UI files
are produced only by existing staging commands, never hand-edited.

## Full-run dispositions and final-source checks

The initial full Go attempt failed in internal/app, internal/runtime,
sdk/codeclient, sdk/internal, sdk/internal/conformance and ui. The
[failure inventory](ci-failures.txt) preserves its 30 top-level names. This
attempt overlapped final Face/source declaration changes and the UI rebuild;
its moving-tree source mismatches and missing embedded index are verification
contamination, not established product regressions. No full final-tree Go
suite is claimed passing, and no broad baseline/environment exemption is made.

Final-source retries:

- TestServiceRunHappyPath and the observer-schema test had old v2/v3
  expectations; updated to completion v3/request v4 and explicit rejection
  of legacy requests by the live schema. Both now pass.
- TestServiceApprovalResumePersistsChunkBeforeProviderEOF passes in the final
  targeted run; the earlier full-run timing failure remains recorded.
- TestEmbeddedHandlerAnswers passes after rebuilding actual UI assets, 0.107s.
- The ui-root-conflict SDK matrix subtest actually executes and passes, 2.484s,
  after source declarations stop moving. This does not accept other pack cases.
- The final combined regression command passed runtime (10.604s), RPC
  (11.761s), SDK/stream/live and ui, but timed out the causal App test while
  running packages together. The same unchanged App test passed alone in
  12.45s with nonce, call ID, byte framing, final answer, one terminal and
  WaitIdle assertions. Its 15s wait was not increased; contention/timing
  remains an open limitation, not erased by the isolated pass.
- Full shared facerun/stream/live suites pass after review corrections;
  independent plugin-ci, including headless/TUI, passes again.
- Windows echo/bash assumptions, codeclient spawn/handshake failures,
  garden.db cleanup locks, approval-expiry timing and remaining SDK pack
  failures have not all been diagnosed. D01's baseline comparison reproduced
  the two shutdown cleanup failures, not every failure in this attempt.

The real conformance producer was run on the final source tree. It fails
before suite execution: computed internal identity
80aa4859cf6ec8e27a35c06c4c3a10eb0e14de9f9d255906c15653e531a4e8a0
versus checked-in
58e65c9389ffe8eda3349e2a02e8004487c7d5e0b8de604af989250fbd96b621.
This is a D01/D02 source-change consequence, not an environment-only failure.
The artifact and older producer source expectations remain untouched; a real
successful regeneration is still required before acceptance.

## Independent review and consolidated correction

The fresh reviewer inspected f5ce1c66..85afcc07 read-only and ran no duplicate
suites. Findings were accepted by the lead and fixed in one pass:

1. Standalone facerun ignored run/stream_error and could wait indefinitely.
   A Host-path regression first failed on context timeout. It now reports a
   subscription-scoped local error, preserves text, ignores foreign failures
   and never sends run/cancel or fabricates a backend terminal. Synchronous
   replay failure before the subscribe response is handled with bounded
   pending errors, matching the existing TUI recovery pattern.
2. Web and TUI retried permanent malformed/unsupported event formats. New
   regressions observed the loop first. RPC now marks these failures with
   optional retryable:false; absent means the existing recoverable behavior.
   Faces preserve the diagnostic and stop automatic recovery. The Web store
   regression also preserves backend status, prior text and queued work.
3. Catch-up replay returned silently on normalization failure. The bus-drop
   regression first timed out. Initial, live and catch-up paths now all emit
   classified subscription failures; the backend Run remains active.

Review RED logs reproduced all three findings. The correction subset passed
RPC 5.512s, facerun 0.119s, live 0.202s and stream 0.139s. Final Web subscription,
store, framing and transcript checks pass 102 tests. Full shared Face suites,
standalone modules, UI/i18n, vet and headless compilation were rerun after it.
The optional retryable flag is an additive field on existing run/stream_error;
there is no new RPC method, Port, dependency or execution authority.

The review declined D01 correctness, operation/request-ID ownership changes,
database/restart conversion guarantees, approval/checkpoint migrations,
source-hash deletion, #93 extraction and unrelated expiry/cleanup failures.
Ruling: these remain with their existing owners; D02 changes no such inputs,
authority or stored formats and accepts none of those broader guarantees.

D02 and full #91 acceptance remain open. Final diff/format/link checks pass;
there is no release, push, issue closure or claim of full CI success.
