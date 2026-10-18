# S3 Task 2 native Recap implementation report

Source baseline: `96adc4f7b49f25f4f67b1c583597d035ab2cf371`. Task 1 remains separately accepted; this report does not claim final S3 acceptance, installed #66 producer, Windows durability, live-model quality, PostgreSQL execution, or full CI.

## Checkpoints

- `340bc959a282118c031d2416ed4e34dac886c617`, tree `1eb88a3f392d5e856030e9a1088d2b2a308eee52`: native fold, zero-range reconcile, existing BlobStore pending intent and explicit compact routing.
- `b26ee856cea6bb1fd10916b44fe28bcb12491841`, tree `982f37f5bc4f534097b3ac07da04d95c387fb6bd`: foreground reconciliation and pressure admission, synthetic explicit observer, CAS/crash/reopen/provenance/capacity/integrity tests. Fixed-source gate is running in detached sibling checkout `agent-vivy-issue58-task2-verify`.

## Native APIs actually inspected

- Repository pin: Eino `v0.9.20`, Laputa/Garden `bb57e93a61cf`, Go `1.26.4`.
- `/root/go/pkg/mod/github.com/cloudwego/eino@v0.9.20/adk/middlewares/summarization/summarization.go`: `New` returns the ADK middleware interface; concrete `*TypedMiddleware[*schema.Message]` exposes `Summarize`. `GenModelInput` replaces the inference input; custom `Finalize` fully controls returned state; Callback executes after Finalize and is read-only. Model retries and configured fallback remain native. The adapter requests zero primary retries, one configured fallback and uses existing `observeSummaryModel`.
- Native `Summarize` calls Finalize, then Callback, then optional `ActionTypeAfterSummarize` transport. `/adk/handler.go` `TypedSendEvent` requires a ChatModelAgent Run/Resume execution context and errors outside it. Admission/explicit scopes precede/outside Runner, so `EmitInternalEvents=false`; the existing governed Callback -> Journal `context.compacted` seam propagates errors instead. No native after-summary transport emission is claimed.
- Existing native observer `observeGenerate` applies `model.WithMaxTokens` from the trusted requested cap and provider adapters apply it after thinking translation. Summary uses its own 3072 bound/reserve and actual route scope, separate from foreground 4096. Catalog currently declares no positive `MaxOutputTokens`; attempting a fixture field for one produced a preserved compile failure, not invented physical metadata.
- Existing C1 bound port is passed unchanged; no owner parser, native OperationDigest hash copy, second summarizer, acknowledgement store, coverage cursor, public API or migration was added. Runtime adapts stable direct `actmem.Error` and Garden `agentapi.Error` codes.

## Material implementation rulings

- One native adapter supplies exact previous head, oldest contiguous complete prefix, typed source-view dispositions and actual correction subset. Covered references are never in the new fold range. Latest complete interaction and current unsealed turn are retained. Prefix capacity splitting occurs only at interaction boundaries. No legacy 200 KiB transcript break is used.
- C1 Commit occurs inside custom Finalize after current complete capture/grant/base checks. Only returned/matching positive C1 commit or exact OperationID lookup authorizes coverage retirement. CAS/disposition change consumes at most one fresh recompute. Unknown errors preserve exact pending request and pause.
- Existing `ServiceDeps.ToolEvidence` BlobStore + `BlobPrefixLister` holds intent metadata, exact RecapCommit and reference/input digests, never a transcript copy or success receipt. Canonical strict decoding and a content digest in the immutable per-operation key protect request identity across restart. Unsupported capability or malformed/partial/mutated intent fails closed.
- Authoritative negative lookup after a typed CAS rejection permits rejected-intent cleanup and bounded fresh recomputation. Other unresolved attempts remain pending. A supported negative on renewed owner may replay only the exact original request after the same source/fingerprint/base/input/capacity validations; unknown/busy/unavailable stays paused.
- Live unknown has no hot restart path. Runtime never closes Bundle/Store or opens a second owner. Existing App/host lifecycle stops/drains Service and closes/reopens the same real Garden owner. Tests inject unavailable/lost-response at the RecapPort boundary and exercise actual App.CloseContext + Garden close/reopen; they do not claim new owner-internal replacement-fault injection (that remains the separately reviewed Laputa suite).
- Standalone native Summarize return is disposable. The consumed provider state is reassembled through existing `prepareRecapContext` from a confirmed committed head plus fresh captured exact suffix/current. No unused tail expansion was added to the native returned slice.
- Explicit compact reuses existing rowless synthetic session-control Journal semantics (`toolExposureRunID` precedent), with fresh scoped `sessctl_recap_<session-hash>_<id>` identity, existing mapper, bounded BudgetLedger, actual v4 request/usage/finished events and synthetic failure propagation. It creates no RunStore row, user/tool message, terminal event, C2 seal or automatic-ingest candidate. Foreground summaries charge their active run ledger before Runner starts.
- Existing usage listings INNER JOIN RunStore, so rowless explicit summary records are omitted from session/model usage UI queries. Raw Journal and call budget remain authoritative. No usage-query redesign is included.

## Attempt evidence (all preserved)

Each attempt uses `/tmp/issue58-localonly-runner.py <absolute-worktree> <label> go test ...`; the runner strips credential variables, rejects external requests via a non-forwarding proxy, and sets `GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local`. Local HTTP fixtures use fake credentials. Meta candidate is HEAD at launch; pre-b26 attempts ran uncommitted working-source iterations and are not fixed-source qualification. Raw `.log` and `.json` are under `task2/`.

| Attempt | Exit | External forwarded | Command |
|---|---:|---:|---|
| issue58-task2-admission-green | 0 | 0 | `go test ./internal/runtime -run ^TestIssue58(CoveredSupersessionAdmission|ReconcileCertificateReset|DispositionCaptureRace|ContinuousRangeCompaction|ZeroNewReconcile)$ -count=1 -json` |
| issue58-task2-admission-red | 1 | 0 | `go test ./internal/runtime -run ^TestIssue58(CoveredSupersessionAdmission|ReconcileCertificateReset|DispositionCaptureRace)$ -count=1 -json` |
| issue58-task2-b26-focused | 1 | 0 | `go test ./internal/runtime ./internal/app -run ^TestIssue58 -count=1 -json` |
| issue58-task2-budget-red | 1 | 0 | `go test ./internal/runtime -run ^TestIssue58SummaryBudgetBound$ -count=1 -json` |
| issue58-task2-capacity-green | 0 | 0 | `go test ./internal/runtime -run ^TestIssue58(AdmissionCapacityUsesSharedFold|SummaryHardOutputBound)$ -count=1 -json` |
| issue58-task2-capacity-red | 1 | 0 | `go test ./internal/runtime -run ^TestIssue58(AdmissionCapacityUsesSharedFold|SummaryHardOutputBound)$ -count=1 -json` |
| issue58-task2-capacity-red2 | 1 | 0 | `go test ./internal/runtime -run ^TestIssue58(AdmissionCapacityUsesSharedFold|SummaryHardOutputBound)$ -count=1 -json` |
| issue58-task2-cas-crash-red | 1 | 0 | `go test ./internal/runtime -run ^TestIssue58(SummaryCASRecompute|CompactionCrashCuts|SummaryRouteCapacityAndFallback) -count=1 -json` |
| issue58-task2-control-evidence | 0 | 0 | `go test ./internal/runtime ./internal/app -run ^TestIssue58(ExplicitCompactionControlLane|SummaryBudgetBound|CompactionCrashCuts) -count=1 -json` |
| issue58-task2-control-green | 1 | 0 | `go test ./internal/runtime ./internal/app -run ^TestIssue58(CoveredSupersessionAdmission|ReconcileCertificateReset|DispositionCaptureRace|ContinuousRangeCompaction|ZeroNewReconcile|SummaryBudgetBound|SummaryCASRecompute|CompactionCrashCuts|SummaryRouteCapacityAndFallback) -count=1 -json` |
| issue58-task2-control-green2 | 0 | 0 | `go test ./internal/runtime ./internal/app -run ^TestIssue58(CoveredSupersessionAdmission|ReconcileCertificateReset|DispositionCaptureRace|ContinuousRangeCompaction|ZeroNewReconcile|SummaryBudgetBound|SummaryCASRecompute|CompactionCrashCuts|SummaryRouteCapacityAndFallback) -count=1 -json` |
| issue58-task2-copy-clear | 0 | 0 | `go test ./internal/runtime -run ^TestIssue58(ReconcileCopiedViewScope|ClearInvalidatesResume)$ -count=1 -json` |
| issue58-task2-first-green | 1 | 0 | `go test ./internal/runtime -run ^TestIssue58(ContinuousRangeCompaction|ZeroNewReconcile)$ -count=1 -json` |
| issue58-task2-first-green2 | 1 | 0 | `go test ./internal/runtime -run ^TestIssue58(ContinuousRangeCompaction|ZeroNewReconcile)$ -count=1 -json` |
| issue58-task2-first-green3 | 0 | 0 | `go test ./internal/runtime -run ^TestIssue58(ContinuousRangeCompaction|ZeroNewReconcile)$ -count=1 -json` |
| issue58-task2-integrity-green | 0 | 0 | `go test ./internal/runtime ./internal/app -run ^TestIssue58(PendingIntentIntegrity|CompactionCrashCuts|CommittedCorrectionProvenance)$ -count=1 -json` |
| issue58-task2-integrity-red | 1 | 0 | `go test ./internal/runtime -run ^TestIssue58PendingIntentIntegrity$ -count=1 -json` |
| issue58-task2-observed-green | 1 | 0 | `go test ./internal/runtime -run ^TestIssue58(CoveredSupersessionAdmission|ReconcileCertificateReset|DispositionCaptureRace|ContinuousRangeCompaction|ZeroNewReconcile|SummaryBudgetBound|CompactionCrashCutsCallback)$ -count=1 -json` |
| issue58-task2-observed-green2 | 1 | 0 | `go test ./internal/runtime -run ^TestIssue58(CoveredSupersessionAdmission|ReconcileCertificateReset|DispositionCaptureRace|ContinuousRangeCompaction|ZeroNewReconcile|SummaryBudgetBound|CompactionCrashCutsCallback)$ -count=1 -json` |
| issue58-task2-observed-green3 | 1 | 0 | `go test ./internal/runtime -run ^TestIssue58(CoveredSupersessionAdmission|ReconcileCertificateReset|DispositionCaptureRace|ContinuousRangeCompaction|ZeroNewReconcile|SummaryBudgetBound|CompactionCrashCutsCallback)$ -count=1 -json` |
| issue58-task2-provenance | 0 | 0 | `go test ./internal/runtime -run ^TestIssue58CommittedCorrectionProvenance$ -count=1 -json` |
| issue58-task2-red-corrected | 1 | 0 | `go test ./internal/runtime -run ^TestIssue58(ContinuousRangeCompaction|ZeroNewReconcile)$ -count=1 -json` |
| issue58-task2-red | 1 | 0 | `go test ./internal/runtime -run ^TestIssue58(ContinuousRangeCompaction|ZeroNewReconcile)$ -count=1 -json` |

RED evidence includes the initial Task1 reconciliation refusal, admission budget bypass, hard-capacity failure, and valid-JSON pending mutation replay. Compile/fixture failures (duplicate fixture session, native concrete middleware name, incorrect main binding field, unsupported provider metadata field, first App fixture missing Sink) and the invalid attempt to journal onto a terminal source Run are retained separately; corrected assertions were rerun. No failed attempt is represented as acceptance.

## Remaining verification / limits

- Fixed `b26ee856` focused gate, required race gate and broader runtime/app tests are pending at this writing. Root owns integrated `just ci`, independent review and publication.
- Additional remaining proof cuts are being assessed: complete interaction over-200-KiB segmentation, arbitrary correction subsets, post-commit recapture/source-read failure and synthetic usage/cancellation details. Findings will be separate source commits and require new fixed-source qualification.
- No PostgreSQL DSN is configured. PostgreSQL skips are not execution; no Windows/live-model/#66 installed claim. Disk currently has about 9.7 GiB free; no source/evidence/cache deletion performed by this agent.

## Fixed-source checkpoint updates

- `f372626df642116fa3b8a96f0a30260828432558`, tree `4a679659bffa6d201780329f98e81d752d6348f6`: fixture-only addition of the existing `b.Blobs()` dependency to the selected resident producer fixture. It preserves mandatory pending-recovery capability refusal.
- Fixed b26 focused gate exited1: app clean-checkout build prerequisite `ui/dist` missing, and `TestIssue58AuthorizedOrdinaryAndLifeKeepTools/resident-true` omitted the newly mandatory existing blob dependency. Both failures preserved; b26 is not accepted.
- Prepared validation-only embedded UI prerequisite by copying existing `ui/dist` from the implementation worktree. Git UI tree is exactly unchanged between96 andf372: `6eea09cee77d11aadd13d26a80c5f314b0fdb5ea`. Root integrated CI must rebuild normal product artifacts.
- Fixed f372 focused gate exited1: runtime Issue58 tests pass, but three older app fixture cases remain RED: `TestIssue58ContinuityOnlyCompositionUsesSelectedAdmission/eligible_continuity_request_commits_pins`, `TestIssue58ExactTextInteractionSealsThroughServiceRun`, `TestIssue58FormattedGrantAuthorizedRead`. Two shared fixture constructors omit ToolEvidence. Production App already wires existing blobs. These fixture fixes are held for the consolidated independent-review TDD pass.
- Fixed f372 required race gate exited0 in75.151s,22 passing events (including package events),0failed/skip/race diagnostics; external rejected0/forwarded0. Exact command is in `task2/issue58-task2-f372-race.json`.
- A new working-source test proves the independent review's tool-subset protocol finding: a correction consisting only of one historical tool result is sent as an orphan live `role=tool` message. `task2-subset-red` captures actual HTTP. Production remains frozen awaiting the consolidated review pass.
- Working-source test-only additions for a220KiB prefix split at whole interaction boundaries, actual overlapping inference/CAS, and post-commit source expansion failure all pass (`task2-additional-cuts`); these are not yet committed/fixed-source qualification.


## Consolidated review fix checkpoint

`3e13019b0373d8f6344bb84469fd72598f4c11ab`, tree `f482293e05f14a955f7d0f6b5ec6644b973af3fe`, addresses both accepted Important findings from the independent review and the two existing App fixture omissions. No production capability relaxation, API or DDL was added.

- I1: selected correction messages are explicit historical data with exact message ID, role, call ID, tool name, raw arguments string, body and error. No result-only/call-only subset is emitted as live tool protocol, and unrelated original turns are not added. Actual HTTP cases cover result-only, call-only, terminal-only, parallel mixed and Notebook exclusion. Assertions parse each fragment envelope and compare all fields with the retained targets. Shared receipt normalization runs before constructing/hashing summary input; historical receipt refs reconstruct the same input order without later live marker ordering.
- I2: explicit compaction registers a scoped rowless control atomically under the existing projection/startup fence and Service mutex. It pins engine and route; registration checks stopping and existing active/pending/shell-pending work. Observer and one bounded ledger are installed once across retries. The control map contains only cancellation bookkeeping; it creates no RunStore, queue identity, message, snapshot, terminal or C2 seal. The existing toolkit admission reservation covers the registration window. Finishing removes the control only after native observer settlement.
- Lifecycle audit: CancelAll cancels controls; WaitIdle captures the current control-idle channel under the same mutex, so no new zero-count WaitGroup Add/Wait pattern is introduced. ChangeModelWhenIdle, ScheduleEngineReload/applyPendingEngineReload, RecoverBackground and toolkitDrainState include controls. Engine scheduling takes the existing startup fence. Run-specific cancellation, approval resume, queue/session-active checks, goal/child/workflow activation and terminal cleanup still consume only real run maps; controls never enter those maps and cannot create conversation ghosts. StopAutomaticWork closes the existing stopping gate before App CancelAll/WaitIdle. Capture/native inference occur after releasing projectionMu.
- The real App shutdown test blocks actual HTTP, calls CloseContext, and probes the still-open Journal immediately before Garden owner close. It proves cancelled model.call_finished persisted with response_complete=false, no synthetic conversation terminal/RunStore row, and compaction cancellation settlement before owner/SQL closure.
- That stronger test revealed a payload bug: passing event.Payload ([]byte) to the synthetic JSON marshaler encoded it as a base64 string. The observer now passes json.RawMessage, retaining object payloads and original request version4; request/finish/usage object assertions protect the wire-to-Journal boundary. The failed attempt and inspection payload remain preserved.
- Working-source gates `issue58-task2-review-fixes-green` and `issue58-task2-lifecycle-consumers-green` exit0, external forwarded0. They precede fixed-commit qualification and are not relabeled as it. Fixed3e130 normal/race and broader default runtime/App gates follow.

## Explicit deferred reporting limitations

ContextStatus still reports legacy per-session pressure and CompactSession returns Folded/Skipped with zero token totals; these are not measured unified LIFE context totals. S4/S6 must derive selected-LIFE status from committed/pinned admission data or explicitly mark it unavailable, without labeling zero as measured. Rowless synthetic controls remain absent from current RunStore-joined usage listings; raw durable Journal and bounded budget are required and tested. Installed #66 producer/sidebar remains external.

The fixed f372 broad aggregate exited1 after605.156s: runtime package passed, App hit the default600s package timeout. 1436 test pass events and5 skips are partial evidence only. The named MemoryLoop test had only9s active when the aggregate timed out; this is not evidence of a deadlock in that test. External rejected2/forwarded0. Final broader verification separates default (`-skip '^TestMemoryLoop'`, timeout35m) from repository-supported DIVA/MemoryLoop script; root owns integrated full CI.


## Fixed3e130 verification

- Detached checkout: `/workspace/scratch/5dda642e718d/agent-vivy-issue58-task2-verify`, exact source commit3e13019b, treef482293e; generated UI embed prerequisite unchanged as documented above.
- `issue58-task2-3e130-focused`: `go test ./internal/runtime ./internal/app -run '^TestIssue58' -count=1 -timeout 35m -json`, exit0 in29.579s;228 test/subtest pass events plus2 package pass events,0fail/skip; external rejected0/forwarded0.
- Root independently reran all three original unchanged reviewer assertion probes via an isolated overlay on exact3e130. Parent reports exit0,3 test passes plus1package,16.962s, external0. Root owns those independent raw artifacts and qualification.
- Source checkpoint published by root as remote `cdc2e16cd8f00ee06ced6ba46c4baffca5fbdeff` with exact tree `f482293e05f14a955f7d0f6b5ec6644b973af3fe`.
- Full Issue58 race and broader default runtime/App outcomes are recorded below once complete. Root owns final DIVA/MemoryLoop script and integrated justci.

`task2-evidence-index.json` indexes preserved attempt metadata, package/test events, failures/skips and race diagnostics without treating a partial run as complete. All Task1 evidence remains unchanged.


- `issue58-task2-3e130-race`: `go test -race ./internal/runtime ./internal/app -run '^TestIssue58' -count=1 -timeout 35m -json`, exit0 in269.214s;228 test/subtest passes plus2 package passes,0fail/skip, no race diagnostics; external rejected0/forwarded0. This is the full Issue58 race set, broader than the original required subset.
- Broader fixed-source default gate started: `go test ./internal/runtime ./internal/app -skip '^TestMemoryLoop' -count=1 -timeout 35m -json`, safe-runner label `issue58-task2-3e130-broad-default`, exec session80723. The writer is released while this immutable detached candidate continues; parent owns completion/archival and any subsequent follow-up. Until metadata records exit0, this gate is RUNNING and not acceptance. Raw output `/tmp/issue58-task2-3e130-broad-default.log`; final metadata will be `/tmp/issue58-task2-3e130-broad-default.json`.

Source/test implementation is complete at3e130; no source changes follow that checkpoint in this lane. Remaining integration risks/limits: reporting limitations above, rowless usage aggregation, external #66 producer/sidebar, unavailable PostgreSQL DSN (no execution claim), and root-owned final DIVA/justci/Windows qualification. No tests justify a deployment or merge claim.
