# agent-diva AGENT-LOOP and agent-vivy Loop Comparison and Porting Assessment

> Status: **research report** (an analysis document, not a product contract; it does not add or rewrite decisions).
> Date: 2026-08-26 (revision: the official compaction middleware, native subagents, and TurnLoop were all reclassified as
> "adopt the complete version")
> Purpose: compare `agent-diva`'s AGENT-LOOP (`agent-diva-agent`'s turn-loop implementation),
> and assess mechanism by mechanism what is worth porting to Vivy's loop (the Eino wiring behind `internal/runtime`).
> Two lenses: **(1) whether each mechanism is native to Eino (available by wiring) or must be implemented by us**;
> **(2) a reverse catalog—capabilities provided natively by Eino that agent-diva does not have and Vivy is not using,
> and which are worth using (§4)**. This document provides only comparison and conclusions; it does not implement anything.
> Evidence sources: direct reading of source code and documentation on both sides (see the evidence index in Appendix A); for agent-diva,
> `agent-diva-agent/src/agent_loop.rs` (3609 lines) and the `src/agent_loop/` module family;
> for Vivy, `internal/runtime/` and Eino v0.9.13 (`github.com/cloudwego/eino@v0.9.13`),
> with a complete adk directory survey, package-by-package checks of callbacks / components / flow / compose,
> and close reading of the summarization/reduction middleware source.
> Related: `v1-minimal-agent-proposal.md`, `VIVY-ASSEMBLY.md`,
> `SELF-EVOLVING-GATEWAY.md`, `DSH-VS-AGENT-VIVY-CAPABILITY-GAP.md`.

---

## 0. Summary (TL;DR)

agent-diva's AGENT-LOOP is a **governed turn lifecycle**: admission → context assembly →
iteration → tool step → terminal state, with budgets (iteration count / token), compaction (reactive + micro-compaction),
empty-output fallback, a runtime control channel, and a tool-capability matrix by plan stage. Vivy's current loop is
the ReAct inner loop of Eino's `ChatModelAgent`, governed externally by Service: the budget ledger, approval
human gate, policy gate, and preflight are all in place, but **inside the loop** there is only one hard limit, `MaxToolTurns`.

Conclusions (three categories by source):

- **Native to Eino and requiring only wiring (zero work)**: iteration limit (D3), approval interrupt/recovery barrier
  (D11). Vivy already has both wired.
- **Not in Eino and requiring our own implementation (= the DIVA porting catalog, first batch)**: summary-only
  reward pass (D4), empty-output fallback (D5), loop observability events, plus the readily available
  **`UnknownToolsHandler`** (P5: turns a hallucinated tool name from a hard run failure into one round of self-correction).
- **Native to Eino but unused by both DIVA and Vivy (reverse catalog §4; this round decides to adopt the complete version)**:
  - **Official compaction middleware** (`summarization` + `reduction`): close source reading confirmed that the configuration surface
    has sufficient bridge hooks (summary model supplied by the caller, `EmitInternalEvents`, `Callback`,
    `ClearPostProcess`)—**adopt the complete version + bridge** (P3 upgraded); demote the custom
    compaction package to metering/fallback;
  - **TurnLoop preemptive conversation**: **use it**, in the second batch (interaction-model upgrade: users can interrupt
    an in-progress turn);
  - **Native subagents (`NewAgentTool`)**: **make them compatibly optional** (service-level child workers
    as the main mode, native/swarm as advanced modes), consistent with the hot-swappability principle, in the second batch;
  - Semantic retries via `ModelRetryConfig` and direct tool responses via `ReturnDirectly`: revisit later (candidates).
- **Not in Eino, but Vivy already has an equivalent**: context assembly (D8), token budget ledger (D18),
  and admission-style entry throttling (D2).
- **Memory family (MEMRULES/ACTMEM/experience log)**: **do none of it**. Vivy's memory design
  (MEM-1) is more advanced than agent-diva's memory mechanisms, is outside AGENT-LOOP scope, and is not a later candidate.

One-sentence conclusion: **Vivy will not port agent-diva's loop skeleton; the first batch will implement four missing pieces
(reward pass, empty-output fallback, observability events + the readily available UnknownToolsHandler), use Eino's official
complete middleware with a bridge for compaction, and use TurnLoop plus optional native subagents in the second batch to upgrade
the interaction and orchestration forms.**

---

## 1. agent-diva AGENT-LOOP Architecture Overview

agent-diva's turn loop has a five-layer structure: **facade → turn orchestration → turn stages → control plane → tool surface**
(`agent_loop.rs` is the facade, and the `agent_loop/` module family carries the stages):

```text
agent_loop.rs (facade, 3609 lines)
  ├─ turn orchestration        loop_turn.rs / loop_runtime_control.rs
  ├─ turn stages               turn/{admission, context, iteration, tool_step, finalize}.rs
  │                            stage-contract: each stage has StageContract{ShouldRun, Run}
  ├─ turn policy               turn/policy.rs (TurnMode: Agent | Plan | Ask)
  ├─ turn prompt               turn/prompt.rs
  └─ tool surface              loop_tools.rs
```

Key mechanism catalog (the numbers are referenced below):

| # | Mechanism | Location | In one sentence |
|---|---|---|---|
| D1 | stage-contract turn pipeline | `agent_loop/`, each stage | Five stages per turn; each stage can independently decide should-run |
| D2 | admission control | `turn/admission.rs` | Rejection circuit breaker + 100 calls/hour throttling + execution-conflict check |
| D3 | IterationBudget (default 20) | `turn/iteration.rs` | Per-turn iteration limit, with accounting across turns |
| D4 | summary-only reward pass | `turn/iteration.rs` | Add one "summarize only, do not call tools" pass when the budget is exhausted |
| D5 | Empty-output fallback classification | `turn/finalize.rs` | OutputTruncated / InputPressure / EmptyStop |
| D6 | Reactive compaction (one rebuild) | `turn/context.rs` | Rebuild once when the context exceeds its limit |
| D7 | Micro-compaction of tool results | `turn/context.rs` | Proportionally trim long tool results |
| D8 | Budgeted context assembly | `turn/context.rs` | History/CanonicalCheckpoint/ToolResultInline/ActiveTail/DEFERRED |
| D9 | Automatic compaction + manual /compact | `loop_tools.rs` | Expose a compaction command on the tool surface |
| D10 | Runtime control channel | `loop_runtime_control.rs` | StopSession/ResetSession/SetThinking/SetApprovalPolicy/CompactSession/UpdateNetwork/UpdateMcp |
| D11 | Plan approval lifecycle barrier | `loop_turn.rs` | stop_after_AwaitingApproval; stop before approval |
| D12 | Plan-stage tool capability matrix | `turn/policy.rs` | Fail-closed tool allowlist for each stage |
| D13 | MEMRULES write gate | Memory subsystem | Gate memory writes |
| D14 | ACTMEM pulse/retrospective + idle folding | Memory subsystem | Periodic experience consolidation |
| D15 | Media handling | Message surface | Model visibility of media such as images |
| D16 | Canonical tool result (sha256 artifact reference) | Tool surface | Do not inline the result; reference an artifact |
| D17 | Experience log | Memory subsystem | Post-session experience consolidation |
| D18 | Token budget ledger | Inside the loop | Account for turn tokens |

Current corresponding state on the Vivy side (see §6 for the detailed comparison):

- Loop skeleton: the ReAct inner loop of Eino `adk.ChatModelAgent` + `adk.Runner`
  (`internal/runtime/engine.go:88-116`), driven by Service
  (`internal/runtime/service.go`).
- Turn budget: a single hard limit, `MaxToolTurns` → `MaxIterations`
  (`engine.go:98-103`); over the limit → `run.failed` (MA-4).
- Context assembly: `buildRunContext` truncates by bytes/count (`internal/runtime/context.go`),
  and there is also an unwired `internal/runtime/compaction/` package (meter + pruner, pure functions, tested).
- Tool surface: the InvokableRun gate in `tooladapter.go` (selection → validation → policy → hooks →
  approval/question interrupt → execution → redaction → header annotation → result compaction).
- Control plane: RPC layer (`internal/rpc/control.go`), with run/session/approval/question/
  review/child/generations/evals surfaces, but no context/budget/compaction surface.

---

## 2. Comparison Method: Why "Port Mechanisms, Not the Skeleton"

agent-diva's AGENT-LOOP is a custom Rust implementation; Vivy's loop is Eino's library-level ReAct.
The difference is not implementation language, but **who owns the skeleton**:

- agent-diva owns the entire turn pipeline (stage-contract is its product code), so it can compile
  admission, budgets, compaction, and the control channel directly into the pipeline.
- Vivy's turn pipeline belongs to Eino (the ChatModel loop in `adk/react.go`), and Vivy intervenes through
  three seams: middleware (`adk.ChatModelAgentMiddleware`), run-local values,
  and event consumption (`internal/runtime/mapper.go`).

Conclusion: **porting the stage-contract turn pipeline would amount to rewriting Eino's ReAct loop**
(requiring `adk.TurnLoop` or even a custom Runner), with costs disproportionate to the benefits, and would violate
the D-007 isolation in `v1-minimal-agent-proposal.md` ("Vivy's loop stays Eino's
run-internal loop behind the quarantine") and the "loop factory default = eino" in `VIVY-ASSEMBLY.md`.
The unit of porting is therefore the **mechanism** (the problems agent-diva solves inside the loop),
not the **skeleton**.

Decision criteria (used consistently below):

- **Port**: the gap is real (agent-diva has it, Vivy does not, and the absence is harmful), the Eino seam has been verified as feasible,
  the slice can be delivered independently, and it does not violate Journal/policy invariants.
- **Free adoption / adopt the complete version (specific to §4)**: Eino provides it natively, neither DIVA nor Vivy uses it,
  and wiring is low-cost or does not break invariants (with a bridge layer where necessary).
- **Defer**: the mechanism itself is valuable, but is constrained by seams, lacks sufficient benefit evidence, or depends on an independent slice
  (generation change / multi-tenancy) coming first.
- **Reject / do not do at all**: it belongs to another slice (the memory family, for which Vivy has a more advanced design), conflicts with Vivy
  invariants, or is simply unnecessary for agent-diva's form.

---

## 3. Primary Lens One: Native to Eino vs Requiring Our Own Implementation (DIVA Mechanism Classification)

The core comparison question is: **is each agent-diva mechanism native to Eino v0.9.13 (available by wiring),
or must we write it ourselves?** The table below classifies all mechanisms (see §5 seam verification and
Appendix A evidence).

| # | Mechanism | Native to Eino v0.9.13? | Vivy status / conclusion |
|---|---|---|---|
| D3 | Iteration limit | **Native**: `MaxIterations`, internal State's `RemainingIterations`, and `ErrExceedMaxIterations` on overflow | Already wired (`MaxToolTurns`→`MaxIterations`, `engine.go:98-103`). **Zero work** |
| D11 | Approval interrupt/recovery barrier | **Native**: interrupt (InterruptCtx/AwaitingApproval) + `ResumeWithParams` + CheckPointStore | Already wired (mapper extractInterrupt, service handleInterrupt, engine Resume). **Zero work** |
| D4 | summary-only reward pass | **Not native** (the iteration limit only hard-fails; there is no "reward pass" concept) | **Requires our own implementation**: middleware (§6.1 P1) → port |
| D5 | Empty-output fallback classification | **Not native** (terminal-message semantics belong to the consumer) | **Requires our own implementation**: mapper/service (§6.1 P2) → port |
| D7 | Micro-compaction of tool results | **Natively available (official middleware)**: `reduction` (two-stage deterministic compaction) + `summarization` (LLM summary), with bridge hooks (§4.3-E2) | Not wired → **adopt the complete version + bridge** (§6.1 P3) |
| D9 | Loop observability events | **No native event surface** (only the `CustomizedOutput` channel + each middleware's EmitInternalEvents/Callback hooks, with semantics defined by the consumer) | **Requires our own implementation**: new mapper branch + new event types (§6.1 P4) → port |
| D6 | Reactive retry on context overflow | **Partially covered by official components**: `reduction`'s Clear stage performs in-place slimming when over limit | Reassess after the P3 official components land (see §6.2-D6) |
| D10 | Runtime control channel | **Partially native**: TurnLoop's Stop (graceful/immediate) covers the StopSession family | RPC surface requires our own implementation → defer (depends on P1/P3; see TurnLoop in §4.3-E3) |
| D12 | Plan-stage tool capability matrix | **Not native**: the tool set is statically bound at assembly (ToolsConfig); runtime tool-surface swapping requires `TurnLoop`. The native seam `BeforeModelRewriteState` + `state.ToolInfos` can filter per round | Equivalent already exists (policy.go gates by profile, plan→Deny) → defer |
| D2 | Admission throttling/circuit breaker | **Not native** | Equivalent already exists: budget ledger (budget.go) + preflight (preflight.go) → defer (single-user form) |
| D8 | Budgeted context assembly | **Not native** (`Runner.Run` receives caller messages; assembly belongs to the caller) | Already exists: `buildRunContext` (context.go) → do not port |
| D18 | Token budget ledger | **Partially native**: `TokenUsage` reported per call (`ResponseMeta`); no cross-turn ledger | Already exists: `budget.go` → do not port |
| D15 | Media handling | **Partially native**: `schema.Message` MultiContent supports media parts; visibility at the model surface depends on the provider | Text-only surface currently → independent capability slice, not part of AGENT-LOOP |
| D16 | Canonical tool result (sha256 artifact) | **Not native** (reduction's storage is an internal loop recovery artifact and does not change Journal; see §4.3-E2) | Conflicts with Journal event-by-event replay → reject |
| D13 | MEMRULES write gate | **Not native** | **Do none of it**: Vivy has a more advanced memory design (MEM-1), outside AGENT-LOOP |
| D14 | ACTMEM pulse/retrospective/idle folding | **Not native** | **Do none of it** (same as above) |
| D17 | Experience log | **Not native** | **Do none of it** (same as above) |
| D1 | stage-contract turn pipeline | **Partially native**: the ReAct loop is an equivalent skeleton; stage contracts (admission/context/…) are not native | The skeleton belongs to Eino; do not port (§2) |

Conclusions from the table:

1. **Vivy already has both mechanisms that are "native"**—the iteration limit and approval barrier.
2. **Compaction (D7) is upgraded to "official component + bridge"**—after close source reading in this round, the decision changed;
   we will no longer build custom compaction middleware. The truly necessary custom gaps narrow to the reward pass, empty-output fallback,
   and observability events.
3. **The three memory-family items are the only category that is "do none of it"**—not deferred, not candidates, but explicitly excluded.

---

## 4. Primary Lens Two (Reverse Catalog): Eino-Native Capabilities Unused by Both DIVA and Vivy

This section reverses the question: **which capabilities provided natively by Eino v0.9.13, and not included in the §1 D1–D18 mechanism catalog,
are worth Vivy using?** Basis: a complete adk directory survey (including `adk/middlewares/*` and
`adk/prebuilt/*`), package-by-package checks of callbacks / components / flow / compose,
and close reading of the summarization/reduction source (Appendix A).

### 4.1 Baseline: Eino Surfaces Vivy Already Uses

To avoid misclassifying something as "unused," first list Vivy's actual usage (grep across all non-test source in the repository):

- adk core: `NewChatModelAgent`/`NewRunner`/`ToolsConfig`/`WithCheckPointID`/
  `ResumeParams`/`AgentEvent` stream consumption/`CancelError` classification
  (`engine.go`, `service.go`, `mapper.go`, `checkpointadapter.go`).
- Three official adk middleware packages are already used: `middlewares/filesystem` + `middlewares/plantask`
  (`todo_backend.go`), `middlewares/skill` (`skills_backend.go`), and the
  `adk/filesystem` Backend (`filesystem_backend.go`).
- One custom middleware: toolSelection (whole-turn tool filtering in BeforeAgent,
  `toolselection_middleware.go`).

All other adk surfaces (all those below) are currently unused.

### 4.2 Free Adoption in the First Batch

#### E1. `UnknownToolsHandler` — Graceful Fallback for Hallucinated Tool Names (Strongly Recommended, First Batch)

| Item | Content |
|---|---|
| What it is | `compose.ToolsNodeConfig.UnknownToolsHandler` (`compose/tool_node.go:206`, directly embedded in `adk.ToolsConfig`; Vivy can configure it in one place in `engine.go`) |
| What happens without it | When the model calls a tool name outside the catalog, the entire ToolsNode fails (`"tool %s not found in toolsNode indexes"`, `tool_node.go:819-824`), a **hard run failure** |
| What happens with it | The handler's string return value is fed back to the model as a **normal tool result** (`tool_node.go:868`), and the model self-corrects on the next round—for example, replying "that tool does not exist; available tools are …" |
| Current state on both sides | DIVA's custom loop has no native component; Vivy has not configured it (`engine.go:94-96` only sets `Tools: wrapped`) |
| Decision | **Free adoption in the first batch (§7.1 P5)**. One configuration point + prompt-copy design + deterministic test; direct resilience benefit |

### 4.3 Adopt the Complete Version (Decided This Round)

#### E2. Official Compaction Middleware `summarization` + `reduction` — Complete Version + Bridge (P3 Upgraded)

> Decision-change note: the previous version rejected this on the grounds that "summary calls are not accounted for / full transcript storage conflicts with Journal."
> After close reading of both middleware implementations in this round, we confirmed that **the configuration surface has sufficient bridge hooks, so the previous conflict can be resolved with a bridge**. The decision is changed to adoption under the principle "if a complete capability exists, use the complete version."

Close-reading conclusions (`adk/middlewares/summarization/summarization.go:56-151`,
`adk/middlewares/reduction/reduction.go:54-146`):

| Component | What it does | Bridge hooks (key) |
|---|---|---|
| `reduction` | Two-stage deterministic compaction: (1) Truncation—truncate + store oversized results after tool execution (default 50000); (2) Clear—before sending to the model, slim older tool calls/results round by round when over budget (default 160k tokens), retain the most recent N rounds (`ClearRetentionSuffixLimit`), and use `ClearAtLeastTokens` to preserve the prompt cache | `ClearPostProcess` (post-slimming hook, can emit events); `ClearMessageRewriter` (rewrite as a user message, such as a system-reminder style); per-tool `ToolConfig`/exclusion table; replaceable `TokenCounter`; `Backend` **may be nil** (placeholder only, no storage) |
| `summarization` | When triggered by token/count threshold (default 160k), use a **summary model** to compact the conversation history, replace the history, and retain `TranscriptFilePath` pointing to the full transcript | `Model` **is supplied by the caller** (through Vivy's provider stack); `EmitInternalEvents` (three observable events: BeforeSummarize/GenerateSummary/AfterSummarize); `Callback` (read-only hook for pre/post-compaction state); `Finalize` (controls the output history); `TokenCounter`; `Retry`/`Failover` |

Bridge plan (preserving Vivy's invariants):

1. **Model-visible ≡ recorded**: emit a `context.compacted` event (P4) from the
   `ClearPostProcess` / `Callback` hooks, recording the IDs of slimmed messages, before/after sizes, and storage path;
   the official component's history rewrite is persisted with the checkpoint and can be replayed.
2. **Account for the budget**: pass a model wrapped by modelbroker as summarization's `Model` (keys/
   routing go through Vivy's stack); `EmitInternalEvents=true` makes the summary call reach
   the mapper as events, and service.consume accounts for it as usual (MaxModelCalls can no longer be bypassed).
3. **Storage location**: use the run workspace root's `adk/filesystem`
   backend (`filesystem_backend.go`, already present in Vivy) for `reduction.Backend`—stored items land in that run's
   workspace, and the placeholder message points to a readable tool via `ReadFileToolName`. Journal still inlines
   the full `tool.finished` result, **without changing the event model** (the D16 rejection remains valid: this is an internal
   loop recovery artifact, not Journal artifactization).
4. **Demote the custom package**: `internal/runtime/compaction` is no longer the compaction engine—
   retain `meter.go` (ContextBreakdown) for preflight/observability (context-budget estimation),
   and retire `pruner.go` or use it as a deterministic fallback when the official components are not enabled.

**Decision: adopt the complete version + bridge; P3 is upgraded from "custom wiring" to "official component + bridge configuration"**
(§6.1 P3, §7.1). Design note: reduction's Clear and the P1 reward pass use the same
`BeforeModelRewriteState` hook; the ordering of the official and custom middleware must be explicitly set at assembly
(compact first, then decide on the reward pass).

#### E3. `TurnLoop` Preemptive Conversation — Use It (Second Batch)

| Item | Content |
|---|---|
| What it is | Push-based turn loop (`turn_loop.go`): `Run/Push/Stop/Wait` + `WithPreempt(SafePoint)` preemption (the user can speak or change instructions during a turn; at safe points after the ChatModel or a tool batch, the loop yields), graceful/immediate Stop, and checkpointed continuation (`Store+CheckpointID`) |
| Current state on both sides | DIVA has no corresponding component; Vivy does not use it—the user can currently act only between runs and cannot interrupt an in-progress turn |
| Value | "User interruption of an in-progress turn" is an interaction-model upgrade, and native Stop covers the StopSession family of the D10 control channel |
| Integration cost | Engine layer: replace or wrap the interactive session's Runner in TurnLoop; change the event surface from an `AgentEvent` iterator to the `TurnLoopConfig.OnAgentEvents` callback → add an adapter path in mapper; coexist with approval interrupts (two checkpoint layers: TurnLoop is the outer session loop, approval is the inner graph interrupt) |
| Decision | **Use it (user decision), second batch (§7.2)**. Depends on first-batch P1–P5; TurnLoop is a newer API, so the implementation batch should first build a compatibility PoC with the existing checkpoint/resume |

#### E4. Native Subagent `NewAgentTool` — Compatibly Optional Advanced Mode (Second Batch)

| Item | Content |
|---|---|
| What it is | adk's in-run subagent delegation (`agent_tool.go`): wrap an Agent as a tool on the parent agent's tool surface; `ToolsConfig.EmitInternalEvents` bubbles up subagent events; approval interrupts propagate through `tool.CompositeInterrupt`; configure each subagent with `WithAgentToolRunOptions`/`DesignateAgent` |
| Current state on both sides | DIVA has no native component; Vivy has service-level child workers (parent/child budget scopes, child.* events, approval authority, child surface in `rpc/control.go`)—governance is complete, but each delegation is a separate and relatively heavy run |
| User direction | Compatibly optional advanced mode: swarm or native, consistent with the hot-swappability principle |
| Assessment | **Agree to make it compatibly optional**. Design it as a switchable delegation backend: `children.mode: service (default, fully governed current mode) | native (in-run, lightweight and fast, suitable for swarm-style fan-out—multiple AgentTools on the parent tool surface, with model-driven parallel delegation)`**. Constraint: in native mode, child-agent tools must still pass through the same tooladapter gate (no bypass of policy/hooks/approval); map subagent events into the child.* vocabulary and include them in the budget ledger. Both modes share the child.* events and RPC surface, transparent to upper layers |
| Decision | **Adopt as a design goal, second batch (§7.2)**. It changes engine assembly + mapper + budget, so it is not first batch |

### 4.4 Candidates, Not Urgent (Revisit Later)

#### E5. `ModelRetryConfig` / `ModelFailoverConfig` — Semantic Retries and Model Switching

`adk.ModelRetryConfig` (`retry_chatmodel.go:222`): `ShouldRetry` receives
`RetryContext` and returns `RetryDecision`—it can rewrite the error shown to the model, modify input messages,
add model options by attempt count, and customize backoff; `ModelFailoverConfig` switches models on failure.
Vivy currently only **consumes** `WillRetryError` events (provider.retry); the engine layer does not configure
retry policy—retries are hidden in the provider implementation and cannot be changed from the governance surface.
**Candidate (provider-resilience slice), revisit later.**

#### E6. `ToolsConfig.ReturnDirectly` — Direct Tool Responses

Declare per tool that "this result is the final answer" (`react.go:520-553`) and skip one model synthesis step.
This can reduce latency for query tools (read notes, inspect status); it requires a product decision per tool.
**Candidate, revisit later.**

### 4.5 Not Adopted After Evaluation

#### E7. `patchtoolcalls` Middleware — Placeholder Messages for Unanswered Tool Calls

Vivy already has equivalent handling in feed assembly (`pairToolTurns` in `buildRunContext` discards
unpaired tool rows), and the checkpoint guarantees pairing on interrupt/recovery paths. **Partially duplicates the current state,
so it is not adopted**; reassess if a malformed case is later found in Eino state.

#### E8. Bundled Prebuilts and Multi-Agent Orchestration Components — Product Shape Mismatch

- `prebuilt/deep` (DeepAgent), `prebuilt/planexecute`, `prebuilt/supervisor`:
  bundled agent products; adopting them wholesale conflicts with Vivy's own species positioning (D-001/D-005 anti-cloning).
  **Do not adopt them wholesale** (individual ideas such as write_todos are already covered by plantask).
- The transfer family (`SetSubAgents`/`transfer_to_agent`/deterministic_transfer) and
  `SequentialAgent`/`ParallelAgent`/`LoopAgent`: Eino itself marks these NOT RECOMMENDED,
  and they exceed Vivy's single-agent shape. **Do not adopt.** (E4's native delegation uses `NewAgentTool`,
  not the transfer family.)

#### E9. Small or Duplicative Components

| Component | Conclusion |
|---|---|
| `WithChatModelOptions` (`WithModel` model switching / `WithToolChoice` + allowedToolNames) | Switch models per run without rebuilding the agent. Overlaps with modelbroker's selection responsibility; reassess if per-session model switching is implemented |
| `WithCallbacks` + `utils/callbacks.NewHandlerHelper` | Component-level latency telemetry seam. Vivy already has its own event-stream telemetry; connect it if finer component latency is needed |
| `ExitTool` / `SendToolGenAction` + `NewExitAction` | Explicit model finalization / tool-triggered loop action. Overlaps semantically with the P1 reward pass; P1 takes priority, observe for now |
| `agentsmd` / `dynamictool` (toolsearch) middleware | Duplicates the responsibilities of the existing preamble notes digest and `tools.Selector`; do not adopt |
| Prompt templates (FString/GoTemplate/Jinja2 + MessagesPlaceholder) | Vivy's Go-side composer (prompt.go) is sufficient; do not introduce a template layer |

### 4.6 Confirmed Absent Capabilities (Avoid Repeated Searching)

This round's survey confirms that Eino v0.9.13 has **none** of the following components (do not spend more time looking for them):

- An independent tokenizer / token-counting package—only the `TokenCounter` hook in summarization/reduction exists;
  its default implementation estimates "~4 characters/token," using the same heuristic as Vivy's custom
  `compaction/meter.go`. P3's estimation convention can be injected through the hook.
- Cache helper packages (there is no cache layer at all).
- Generic `WithJSONResponse` / ResponseFormat options (response-format control lives in each eino-ext
  provider implementation, not in the core package).

---

## 5. Eino v0.9.13 Seam Feasibility (Porting Prerequisite)

This round directly read the Eino v0.9.13 source to verify the following seams (all are landing points for porting/adoption candidates):

1. **`BeforeModelRewriteState` runs before every model generation** (`adk/chatmodel.go`),
   can rewrite `state.Messages` / `state.ToolInfos`, and the rewrite is persisted with the checkpoint.
   Setting `state.ToolInfos = nil` is equivalent to giving that generation an empty tool list—this is the landing point for
   the **summary-only reward pass** (D4), and also the attachment point for the official reduction Clear stage and
   summarization (both are middleware; their order is set at assembly).
2. **`SetRunLocalValue` / `GetRunLocalValue`** (`adk/react.go`) store gob-serializable
   values that survive interrupt/recovery—the landing point for "cross-iteration accounting" (reward-pass trigger determination).
3. **`AgentOutput.CustomizedOutput` + `adk.SendEvent`** (`adk/interface.go`)
   allow middleware to inject custom events; the consumer (`internal/runtime/mapper.go:94-96`)
   currently ignores events with `ev.Output.MessageOutput == nil`—**loop observability
   events** require a new custom-event branch in mapper. The official summarization
   `EmitInternalEvents` also uses the event surface and is mapped in the same branch.
4. **`RemainingIterations` decreases on each iteration in the internal State**; when `<= 0`,
   it reports `ErrExceedMaxIterations` (`adk/react.go`)—the reward pass must trigger **before** this error
   (take over after the penultimate generation).
5. **`UnknownToolsHandler`** (`compose/tool_node.go`): without it, an unknown tool name causes the
   entire ToolsNode to fail (§4.2-E1); with it, the handler's return value is sent back to the model as a tool result.
6. **Official middleware bridge hooks**: summarization's `Model` (supplied by the caller)/
   `EmitInternalEvents`/`Callback`/`Finalize`; reduction's
   `ClearPostProcess`/`ClearMessageRewriter`/per-tool `ToolConfig`/nullable
   `Backend` (the §4.3-E2 table).
7. **Middleware chains can be composed**: the existing `newToolSelectionMiddleware()` at `engine.go:93`
   only filters the tool set for the whole turn in BeforeAgent; new middleware can coexist with it. The official middleware
   (filesystem/skill/plantask) has already been verified to coexist with the custom chain (todo/skills backend).
8. **TurnLoop event surface**: `TurnLoopConfig.OnAgentEvents` callback (not an iterator),
   Store+CheckpointID checkpointed continuation, and `WithPreempt(SafePoint)` preemption (§4.3-E3).
9. **Native subagents**: `NewAgentTool` + `ToolsConfig.EmitInternalEvents` +
   `CompositeInterrupt` approval propagation + `WithAgentToolRunOptions`/`DesignateAgent`
   (§4.3-E4).

These verification conclusions are recorded in the "feasibility" fields in §6 for direct reference by implementation batches; this document does not implement anything.

---

## 6. Mechanism-by-Mechanism Comparison and Decision

### 6.1 Port (First Batch: Turn-Lifecycle Governance)

#### P1. summary-only Reward Pass (agent-diva D4)

| Item | Content |
|---|---|
| agent-diva | `turn/iteration.rs`: reserve one reward pass before IterationBudget is exhausted; the model is asked only for a final summary and is given no more tools (budget + prompt dual constraint) |
| Native to Eino? | **Not native**. `MaxIterations` immediately yields `ErrExceedMaxIterations` when exhausted; there is no "reward pass" concept → **requires our own implementation** |
| Vivy current state | When `MaxToolTurns` is exhausted, it immediately becomes `run.failed` (`engine.go:98-103`, `service.go` terminalEvent). Once the budget is reached, the model cannot even say "I have done this much; here is the conclusion" |
| Gap | Long tasks **hard-fail** at the iteration limit instead of **soft-finishing**; the user receives only a "limit of tool-call turns" failure, not partial results |
| Feasibility | Verified: check a run-local counter in `BeforeModelRewriteState` (`SetRunLocalValue`), set `state.ToolInfos = nil` on the penultimate round, and rewrite the system prompt to "summarize only"; the model then emits only a final message and proceeds normally to END. Note the ordering with official compaction middleware hooks (compact first, then decide on the reward pass) |
| Decision | **Port (custom implementation)**. Directly fixes the largest gap where "the loop can fail after spinning" while remaining compatible with MA-4 semantics (retain the hard limit, but allow one soft finish at the boundary) |

#### P2. Empty-Output Fallback (agent-diva D5)

| Item | Content |
|---|---|
| agent-diva | `turn/finalize.rs`: when the terminal message is empty, classify it as OutputTruncated / InputPressure / EmptyStop and handle each differently (retry / prompt the user / clean stop) |
| Native to Eino? | **Not native**. Terminal-message semantics belong to the consumer → **requires our own implementation** |
| Vivy current state | `model.completed` may contain empty Content (`mapper.go:184-189`), and Service records `run.completed` as usual; an empty reply is shown directly to the user, without classification or fallback |
| Gap | Occasional empty model replies (truncation, input pressure, empty stop) are treated as normal completion and cannot be distinguished by the user |
| Feasibility | Verified: empty-output detection can be added to the final-message branch of `mapper.onMessageEvent` and `onTurnEnd` (`mapper.go:234-241`); classification and fallback actions belong in the Service layer (`service.go` consume), without touching Eino |
| Decision | **Port (custom implementation)**. A pure consumer-side change with no Eino risk, and an observable difference at the user-experience level |

#### P3. In-Loop Compaction (agent-diva D6/D7 Combined) — Official Components + Bridge (Upgraded)

| Item | Content |
|---|---|
| agent-diva | `turn/context.rs`: proportionally trim long tool results (D7); rebuild once when the context exceeds its limit (D6) |
| Native to Eino? | **Natively available (official middleware)**: two-stage deterministic compaction in `reduction` (single-result truncation + history slimming, equivalent to D7 and covering most of D6) + LLM summarization in `summarization` (complete version). Bridge hooks are complete (§4.3-E2) |
| Vivy current state | Tool results have only **single-pass** head/tail/tombstone compaction (`compactToolResult` in `tooladapter.go`, constrained by `MaxToolResultBytes`), with no cumulative cross-iteration compaction; `buildRunContext` truncates only by bytes from newest to oldest (`context.go`), which may discard an entire early useful tool result; the custom `internal/runtime/compaction` package is written and tested but has zero references |
| Plan | **Official components + bridge** (§4.3-E2 four points): (1) emit `context.compacted` events (P4) into Journal from `ClearPostProcess`/`Callback`; (2) route summarization's `Model` through modelbroker and set `EmitInternalEvents=true` so summary calls enter the budget ledger; (3) use the run workspace for `reduction.Backend` (reuse the adk/filesystem backend), leaving the Journal event model unchanged; (4) demote the custom package—retain meter for preflight/observability, and retire pruner or use it as fallback |
| Decision | **Adopt the complete version + bridge** (decision changed this round; the original "custom wiring" plan is void). Wire the thresholds (MaxLengthForTrunc/MaxTokensForClear/Trigger/retained rounds) to `config.yaml` runtime.loop.* |

#### P4. Loop Observability Events (agent-diva D10 Event Surface / D9 Visibility)

| Item | Content |
|---|---|
| agent-diva | Loop state (budget, compaction, control actions) is visible in the session stream, so the user can perceive "what the loop is doing" |
| Native to Eino? | **No native event surface** (`CustomizedOutput` channel + each middleware's EmitInternalEvents/Callback hooks, with semantics defined by the consumer) → **requires our own implementation** |
| Vivy current state | 34 RunEvents (`internal/domain/event.go`) cover model/tools/approvals/subagents, but there are **no "loop's own actions" events** |
| Gap | Internal loop governance actions are not observable; during troubleshooting it is impossible to distinguish "the model is spinning" from "the loop is finishing" |
| Feasibility | Verified: inject custom events through `adk.SendEvent` + `AgentOutput.CustomizedOutput` (emit `loop.summary_pass` when the P1 reward pass triggers); map the official components' events/hooks for `context.compacted` and summary calls (P3 bridge). Add a custom-event branch at `mapper.go:94-96`, add event types to the domain, and the RPC/UI subscription surfaces receive them automatically through the event stream |
| Decision | **Port (custom implementation)**. Event sources are the two paths from P1 (custom middleware) and P3 (official-component bridge) |

#### P5. Graceful Fallback for Hallucinated Tool Names (Eino-Native Free Adoption, No DIVA Equivalent)

| Item | Content |
|---|---|
| What it is | `UnknownToolsHandler` from §4.2-E1: change an unknown tool name from a "hard run failure" into "one corrective tool result; the model self-corrects" |
| Vivy current state | No handler is configured at `engine.go:94-96`; a wrong tool name from the model directly becomes `run.failed` |
| Decision | **Free adoption in the first batch**. Deliver it with P1–P4; minimal implementation |

### 6.2 Defer (Direction Exists, Do Not Implement Yet)

#### D6. Reactive Retry on Context Overflow (One Rebuild)

agent-diva rebuilds once and runs again when the context exceeds its limit. **Note: after P3 adopts official reduction,
its Clear stage (round-by-round slimming when over budget + the `ClearAtLeastTokens` threshold) already provides "in-place
mitigation of overflow"**—the DIVA-style "rebuild the entire segment once" is more coarse-grained than reduction's progressive slimming.
This item is **downgraded to: inspect residual gaps after P3 lands**; if reduction + summarization is still insufficient
(for example, if it triggers too frequently), reassess rebuild-style retry.

#### D2. Admission Throttling / Circuit Breaker / Execution Conflict

agent-diva has 100 calls/hour throttling + rejection circuit breaker + execution-conflict checking. **Eino has no native support**.
Vivy is a **single-user personal gateway**, naturally low frequency, and already limits entry through its budget ledger
(`budget.go`) and preflight (`preflight.go`). **Reason to defer**: throttling thresholds are fictitious parameters for a personal gateway;
if multi-tenancy or batch Studio evaluation appears later, extend from `budget.go` as the foundation.

#### D10. Runtime Control Channel (StopSession/ResetSession/CompactSession/…)

agent-diva has in-loop control commands. Vivy's control plane is in the **RPC layer** (`internal/rpc/control.go`:
run/interrupt, approval, question, review, child), which covers human-gate needs; what is missing are commands such as
"compact/change budget/stop a session while it is running." **Reason to defer**: an independent slice (new RPC methods +
runtime intervention channel + recovery semantics); TurnLoop (§4.3-E3) natively covers the StopSession family,
and CompactSession is partially covered by P3's official-component trigger/threshold configuration. Design the remaining commands after the two batches land. **Candidate after the second batch.**

#### D12. Plan-Stage Tool Capability Matrix

agent-diva gives each TurnMode a fail-closed tool allowlist. **Eino has no native support**. Vivy already has a
**profile-based policy engine** (`policy.go`: default→Prompt, plan→Deny,
read_only→Deny, full_auto→Allow); denying all tools in the plan stage is stricter than an "allowlist by tool."
**Reason to defer**: the security objective is already met; refining this into a per-phase allowlist requires product evidence and is a policy-level
change. List it as a generation-change candidate.

### 6.3 Reject / Do Not Do at All

| # | Mechanism | Reason for decision |
|---|---|---|
| D1 | stage-contract turn pipeline | The skeleton belongs to Eino; rewriting it would abandon D-007 isolation and the `loop: eino` factory default (§2) |
| D13 | MEMRULES write gate | **Do none of it**. Memory-related; Vivy has a more advanced memory design (MEM-1), outside AGENT-LOOP scope |
| D14 | ACTMEM pulse/retrospective + idle folding | **Do none of it** (same as above) |
| D17 | Experience log | **Do none of it** (same as above) |
| D15 | Media handling | A media capability (images, etc.); Vivy's current model surface is text-only. Independent capability slice, not part of AGENT-LOOP |
| D16 | Canonical tool result (sha256 artifact reference) | Conflicts with Journal event-by-event replay / the event model in which tool results are inlined in `tool.finished`. Note the difference from E2: reduction storage is an internal loop recovery artifact (Journal unchanged), while D16 changes the Journal event model—the latter remains rejected |
| D18 | Token budget ledger | **Not rejected**: Vivy already has the equivalent `budget.go`, with the same direction; no port needed |
| D8 | Budgeted context assembly (DEFERRED layering) | `buildRunContext` already truncates by bytes/count and pairs tool messages; after P3 adopts the official components, layered assembly is handled inside the loop by reduction/summarization. Do not port the layering |
| D11 | Plan approval lifecycle barrier (stop_after_AwaitingApproval) | Equivalent already exists: Eino native interrupt + `ResumeWithParams` is wired |
| D9 | Automatic compaction + manual /compact | Automatic compaction = the P3 official components themselves; manual /compact depends on the D10 control channel (deferred), so revisit with the second batch |

---

## 7. Recommended Batches and Priorities

### 7.1 First Batch (Turn-Lifecycle Governance) — The Only Slice Recommended for Immediate Work After This Research

| Item | Content | Landing point | Nature |
|---|---|---|---|
| P1 | summary-only reward pass | New custom middleware (`BeforeModelRewriteState` + run-local counter + `ToolInfos=nil`) | Custom implementation |
| P2 | Empty-output fallback | Final-message branch in `mapper.go` + classification in `service.go` consume | Custom implementation |
| P3 | In-loop compaction (D6+D7) | **Official `reduction` + `summarization` + bridge** (event accounting/budget accounting/workspace Backend/configured thresholds); demote the custom compaction package to metering observability + pruner fallback | **Eino-native complete version + bridge** (§4.3-E2) |
| P4 | Loop observability events | Custom-event branch in `mapper.go` + new `loop.summary_pass`/`context.compacted` domain events (including official-component event mapping) | Custom implementation |
| P5 | Graceful fallback for hallucinated tool names | Configure `UnknownToolsHandler` in `engine.go` + corrective copy + tests | **Eino-native free adoption** |
| Supporting | Add `loop.*` to `config.yaml` runtime (reward-pass switch/thresholds, reduction/summarization trigger thresholds and retained rounds) | `internal/config/config.go` + `config.example.yaml` | Custom implementation |

Any new configuration requires parsing/validation tests; all mechanisms require deterministic scripted-model tests (failure path +
normal path; the summary model uses a scripted model with fixed output). Verification runs through `just ci`; UI smoke runs at
`http://127.0.0.1:3015` (split Vite).

### 7.2 Second Batch (Adopt the Complete Version, Decided This Round) — Start After the First Batch Lands

| Item | Content | Key points |
|---|---|---|
| E3 | TurnLoop preemptive conversation | Replace the interactive session with TurnLoop at the engine layer; `OnAgentEvents`→mapper adapter; `WithPreempt` safe-point preemption = user interruption of an in-progress turn; graceful/immediate Stop covers D10's StopSession family; first build a checkpoint/resume compatibility PoC |
| E4 | Optional native subagent compatibility | `children.mode: service (default) | native`; native = `NewAgentTool` + `EmitInternalEvents` + `CompositeInterrupt`, with the child tool surface still passing through the tooladapter gate; swarm = multiple AgentTools on the parent tool surface; both modes share child.* events and RPC surface |

### 7.3 Later Candidates (Revisit Later)

- **E5 Semantic retries/model switching** (`ModelRetryConfig`/`ModelFailoverConfig`): bring
  retry decision authority into the Vivy governance layer (provider-resilience slice).
- **E6 Direct tool responses** (`ReturnDirectly`): save one model-synthesis hop for query tools; requires a product decision per tool.
- Remaining commands in the **D10 runtime control channel** (change budget/compaction commands): design after TurnLoop and P3 land.
- **D12 Plan-stage capability matrix**: policy-level; list as a generation-change candidate.
- **D2 Admission throttling**: extend from `budget.go` only if multi-tenancy / batch evaluation appears.

### 7.4 Explicitly Do Not Do

- **Memory family (D13/D14/D17) → do none of it**: Vivy has a more advanced memory design (MEM-1);
  memory mechanisms are outside AGENT-LOOP scope and are not later candidates.
- **E8 bundled prebuilts and multi-agent orchestration components** (DeepAgent/planexecute/supervisor/
  transfer family/Sequential-Parallel-LoopAgent): product shape mismatch (§4.5-E8).
- **E7 patchtoolcalls**: partially duplicates `pairToolTurns` (§4.5-E7).
- **D16 Journal artifactization**: leave the Journal event model unchanged (see the difference from E2 storage in
  §6.3).
- Media (D15) → independent capability slice; requires a product decision.
- stage-contract skeleton (D1) → retain `loop: eino` as the factory default.

---

## 8. Delivery Discipline

- When this research record is placed with the corresponding iteration log in
  `docs/logs/2026-08-26-agent-loop-port-comparison/`, record the conclusions in `summary.md` and attach this file.
- Open a separate iteration log for each implementation batch (§7.1 first batch, §7.2 second batch), following the
  `just ci` gate and UI smoke gate.
- Do not touch `data/vivy.db`, `data/demo/`, or `data/workspaces/` (air-gap).
- This document adds no decisions; keep `docs/TODO.md` §0.1 as-is. If an implementation batch is accepted, register it
  as a TODO item at that time.

---

## Appendix A: Evidence Index

### agent-diva (read-only reference, `agent-diva-agent/src/`)

- `agent_loop.rs` — facade and turn orchestration (3609 lines)
- `agent_loop/loop_runtime_control.rs` — D10 control commands
- `agent_loop/loop_tools.rs` — D9 compaction tools
- `agent_loop/loop_turn.rs` — D11 plan barrier
- `agent_loop/turn/admission.rs` — D2 admission
- `agent_loop/turn/context.rs` — D6/D7/D8 context assembly and compaction
- `agent_loop/turn/finalize.rs` — D5 empty-output classification
- `agent_loop/turn/iteration.rs` — D3/D4 iteration budget and reward pass
- `agent_loop/turn/policy.rs` — D12 capability matrix
- `agent_loop/turn/prompt.rs` — turn prompt
- `agent_loop/turn/tool_step.rs` — tool step
- `agent_loop/turn/mod.rs` — stage contract

### Vivy (`internal/runtime/`)

- `engine.go:88-116` — Eino ChatModelAgent/Runner assembly; `98-103` MaxToolTurns→MaxIterations; `94-96` ToolsConfig (UnknownToolsHandler not configured)
- `service.go` — turn orchestration, budget ledger, terminalEvent classification, recovery, interrupts, child workers
- `mapper.go:94-96` — custom events currently ignored; empty-output branches at `184-189`/`234-241`
- `context.go` — buildRunContext truncation by bytes/count, tool-message pairing (pairToolTurns)
- `tooladapter.go` — InvokableRun gate and single-pass compactToolResult compaction
- `policy.go` — profile-based policy engine (plan→Deny)
- `budget.go` — budget ledger (MaxEvents/MaxModelCalls/MaxToolCalls/MaxRetries)
- `preflight.go` — ready/warning/blocked before a turn
- `compaction/meter.go`, `compaction/pruner.go` — unwired compaction package (pure functions, tested; after the P3 decision change, demoted to meter observability + pruner fallback)
- `toolselection_middleware.go` — existing custom middleware
- `todo_backend.go` (middlewares/filesystem + plantask), `skills_backend.go`
  (middlewares/skill), `filesystem_backend.go` (adk/filesystem) — official components already in use
- `domain/event.go` — 34 RunEvent types
- `config/config.go`, `config.example.yaml` — runtime configuration section (max_tool_turns, etc.)
- `rpc/control.go`, `rpc/protocol.go` — control-plane method catalog

### Eino v0.9.13 (`github.com/cloudwego/eino@v0.9.13/adk/`, etc.)

- `adk/chatmodel.go` — ChatModelAgentMiddleware hooks, ToolsConfig (embedding
  ToolsNodeConfig: UnknownToolsHandler/ToolAliases/ToolCallMiddlewares, etc.,
  EmitInternalEvents/ReturnDirectly), WithChatModelOptions,
  ModelRetryConfig/ModelFailoverConfig attachment points
- `adk/react.go` — ReAct loop, RemainingIterations, ErrExceedMaxIterations,
  SetRunLocalValue, ReturnDirectly branch, SendToolGenAction
- `adk/handler.go` — AgentOutput.CustomizedOutput, adk.SendEvent
- `adk/retry_chatmodel.go` / `failover_chatmodel.go` — RetryDecision/FailoverContext
- `adk/agent_tool.go` — NewAgentTool subagent delegation, CompositeInterrupt propagation
- `adk/turn_loop.go` — TurnLoop push/preemption (Run/Push/Stop/Wait, WithPreempt,
  TurnLoopConfig.OnAgentEvents, Store+CheckpointID)
- `adk/middlewares/summarization/summarization.go:56-253` — TypedConfig (Model
  supplied by caller/EmitInternalEvents/Callback/Finalize/Trigger/TokenCounter/
  Retry/Failover), TriggerCondition
- `adk/middlewares/reduction/reduction.go:54-221` — TypedConfig (two stages:
  MaxLengthForTrunc truncation + MaxTokensForClear clearing; ClearPostProcess/
  ClearMessageRewriter/ClearRetentionSuffixLimit/ClearAtLeastTokens/per-tool
  ToolConfig/Backend may be nil)
- `adk/middlewares/{plantask,skill,filesystem,agentsmd,dynamictool,patchtoolcalls}` — remaining official middleware family
- `adk/prebuilt/{deep,planexecute,supervisor}` — bundled agents
- `compose/tool_node.go:206,819-824,868` — UnknownToolsHandler semantics
- `callbacks/` + `utils/callbacks/template.go` — callback surface and typed handler builder
- `components/model/option.go` — WithToolChoice/WithModel/WithMaxTokens, etc. (no
  WithJSONResponse)
- `flow/` — agent/react (legacy), retriever/indexer helpers (no agentops, no rag)

### Decision Anchors

- `docs/v1-minimal-agent-proposal.md` — MA-1..MA-4, D-007 isolation
- `docs/architecture/VIVY-ASSEMBLY.md` — loop as a first-class assembly unit, factory default eino
- `docs/architecture/SELF-EVOLVING-GATEWAY.md` — loop direction (builtin | generation)
- `docs/research/DSH-VS-AGENT-VIVY-CAPABILITY-GAP.md` — style and evidence conventions from the previous comparison
