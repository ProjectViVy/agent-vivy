# ND-0 verification

## Performed

Built `internal/runtime/nudge_contract_test.go` (~1300 lines, test-only):

- `contractState`: test-local mirror of the §4 nudgeState surface —
  `Register`/`Complete`/`Abort`/`Await`. `Await` blocks until the batch
  covering the awaited call ids is registered **and** sealed (all results
  durably appended), then returns the batch's notice plus `first=true`
  exactly once per batch (peek semantics).
- `contractBarrier`/`contractWrappedModel`: a test-only
  `adk.ChatModelAgentMiddleware` implementing `WrapModel`. Before the inner
  `Generate`/`Stream` it extracts the trailing tool-call ids from the model
  input, waits on `Await`, then appends the scheduled reminder to a **copy**
  of the input (the request slice is never mutated).
- `contractJournal`: a `storage.Journal` decorator over the real sqlite
  backend. It feeds the state only after `inner.Append` lands (the seal
  boundary is durability, not emission), with pause/fail/hold hooks keyed
  on `"<event type>:<call id>"`.
- `contractCaptureModel` (records mode + full input per inner entry,
  scripted replies, per-entry injection failures) and `contractTool`
  (records `compose.GetToolCallID(ctx)` + raw args per invocation).
- Harness wires a real `adk.NewChatModelAgent` + `adk.NewRunner` (same
  adapter construction as `NewEngine`: `newToolAdapter` /
  `newEnhancedToolAdapter`) behind a `NewService` whose `Journal` is the
  decorator. The state is shared through a pointer holder — the only seam
  available across resume legs since `resumeRun` rebuilds ctx from
  `context.Background()`.

## Exact Eino APIs exercised

- `adk.ChatModelAgentMiddleware` via `*adk.BaseChatModelAgentMiddleware` +
  `WrapModel(ctx, m model.BaseModel[*schema.Message], mc *adk.ModelContext)`.
- `adk.NewChatModelAgent` + `ChatModelAgentConfig{Model, Handlers,
  ToolsConfig, MaxIterations, ModelRetryConfig}`.
- `adk.ToolsConfig`/`compose.ToolsNodeConfig{Tools []tool.BaseTool}`;
  `adk.NewRunner` + `RunnerConfig{Agent, EnableStreaming,
  CheckPointStore}`.
- `compose.GetToolCallID(ctx)` inside `tools.Tool.InvokableRun`.
- `schema.{Assistant,Tool,User,System}Message`, `schema.Pipe`,
  `model.ToolCallingChatModel` (Generate+Stream+WithTools).
- `adk.ModelRetryConfig{MaxRetries, BackoffFunc}` — default `ShouldRetry`
  retries any error.
- `adk/middlewares/summarization` + `adk/middlewares/reduction` via the
  production `buildCompactionHandlers` with a separate summary model.

## Results

```
go test -timeout 20m ./internal/runtime -run '^TestNudgeContract' -count=1 -v
--- PASS: TestNudgeContract (1.02s)
    PASS OrdinaryAdapterBatchOrderAndDurability   (0.25s)
    PASS EnhancedAdapterBatchOrderAndDurability   (0.22s)
    PASS BaselineNoBarrierMayEnterBeforeDurability(0.05s)
    PASS GeneratePathRegisteredBeforeResults      (0.06s)
    PASS FailureMetadataCorrelatesByCallID        (0.06s)
    PASS JournalAppendFailureAbortsBarrier        (0.05s)
    PASS CancelWhileAwaitingSeal                  (0.06s)
    PASS ProviderRetrySeesIdenticalPreparedInput  (0.07s)
    PASS ApprovalInterruptResumeFreshState        (0.18s)
    PASS CompactionRunsBeforeBarrierSummaryBypassesIt (0.00s)
```

```
go test -race -timeout 20m ./internal/runtime -run 'TestNudge|TestToolFailure' -count=1
FAIL — one subtest: EnhancedAdapterBatchOrderAndDurability, upstream race
(see "Upstream race" below). All other subtests pass under -race.
```

```
just ci — fmt-check + ui-ci + vet + test + headless-compile + plugin-ci
PASS.
```

Gate note: the first `just ci` run surfaced the known artifact coupling —
`TestCheckedInProviderConformanceMatchesExecutedSuites` hashes every file
under `internal/`, so adding `internal/runtime/nudge_contract_test.go`
moved the internal source digest to
`632e27dc6c59be2a05204d643d060cd5cf83c362290879df7d5e8c4b6836f4a5`
(recomputed via `go run ./sdk/internal/cmd/source-hash internal ""`).
The five `internal`-rooted `sourceSha256` entries in
`sdk/internal/assembly/conformance_results.json` are re-pinned to that
value in this commit, and the conformance reproduction suite re-executed
cleanly (51.9s).

## Resolved design assumptions (ND-0 evidence for ND-2/ND-3)

1. **Barrier can hold the inner model out until durability.** With the
   journal append for c1's `tool.finished` paused, the engine re-entered
   the wrapper, `Await` blocked, and `inner.Stream` was never called —
   while c2's `tool.finished` (earlier in event order) journaled normally.
   Releasing the append sealed the batch, emitted exactly one scheduling
   action, and the next input carried exactly one result per requested id
   plus the reminder as tail message. No deadlock: the consumer path does
   not depend on the engine.
2. **Take must be peek/idempotent per batch.** adk/wrappers.go invokes
   `WrapModel` and the wrapped model once per retry attempt with the same
   input; the barrier therefore produced an identical prepared input and
   emitted once for both attempts (asserted via DeepEqual on the two
   captured inputs).
3. **The barrier must key on the input's trailing tool-call ids, not on
   "the latest registered batch."** The engine can reach the next model
   call before the consumer finishes journaling the request events — the
   baseline subtest measured `earlyEntry=true` (inner model entered with
   the journal append still paused). `Await` waits for the awaited ids to
   be registered **and** sealed; registration by journal observation may
   lag the engine.
4. **Resume needs a non-ctx state seam.** `resumeRun` builds the resume ctx
   from `context.Background()`, so ctx-attached state cannot reach resume;
   the contract swaps state through a shared holder, mirroring ND-2's
   "fresh state per drive/resume leg". On resume, the replayed
   `tool.finished` for the approved call sealed a singleton batch in the
   fresh state and the next model call proceeded; no stale notice was
   carried over.
5. **Both Generate and Stream paths carry the contract.** The
   `EnableStreaming=false` runner exercised `Generate` end-to-end with
   identical ordering/id-fidelity evidence.
6. **Compaction ordering pins as designed.** The barrier observed the
   final, already-rewritten request (summary content present, original
   feed gone); the summarization-internal model call bypassed `WrapModel`
   entirely (wrap count equals agent model calls; summary inputs carried
   no reminder) — compaction-internal requests get no nudges.
7. **Journal failure aborts the wait.** A failing `tool.finished` append
   released the wrapper via `Abort`, inner model call count stayed at 1,
   the run failed, no waiter leaked.
8. **Cancellation while waiting releases promptly.** With the seal
   withheld, `svc.Cancel` ended the run cancelled; `Await` returned
   `ctx.Err()` and no waiter leaked.

## Upstream race (finding, not a design defect)

`compose/tool_node.go:1253` (eino v0.9.13): inside `(*ToolsNode).Stream`,
the enhanced-path converter closure does
`ret[index].UserInputMultiContent, err = tr.ToMessageInputParts()` where
`err` is the function-scope `var err error` declared at line 1154. Two
parallel enhanced tool calls run their converters on separate goroutines
and both write that shared `err` — a genuine upstream data race, hit on
any parallel enhanced tool batch (production wraps every tool with
`newEnhancedToolAdapter`). The nudge design does not depend on this path
being race-free; the barrier is orthogonal. Not fixed here (upstream
module); tracked in docs/TODO.md §0.1.

## Not performed

- `TestToolFailure*` does not exist yet — that suite belongs to ND-1.
- No production code changed; the producer (ND-1) and the real nudgeState
  (ND-2) remain unproven at product level. ND-0 passing is not product
  acceptance.
- Smoke at 127.0.0.1:3015 belongs to ND-4.
