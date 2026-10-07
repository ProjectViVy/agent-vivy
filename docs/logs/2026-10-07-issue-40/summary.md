# Issue #40 — Faithful core task data and advisory Nudge

Issue: https://github.com/ProjectViVy/agent-vivy/issues/40

Delivery branch: `feat/issue-40-faithful-data`; the original implementation was also pushed on `devin/1791405630-issue-40-faithful-data`.
Baseline: `1df9ae5f893a1c2fee3b565f775a034bab0af4e7`.

## Delivered

- Removed the core logging redactor, generic tool argument guard, their production call sites, and the repetition-based Run stop. No no-op compatibility APIs remain.
- Authorized tool arguments/results/errors, hook projections, model inputs, Journal/replay/history, reviews, Context/Skill/Status/Observer/Action Hosts, and RPC/diagnostics now preserve task text and literal markers. Existing structural/output/context budgets remain.
- Nudge still correlates calls, marks typed failures, settles batches deterministically, waits for durable results, and wakes cancellation waiters. Only matching failure counts 3 and 5 schedule reminders; sixth and later repetitions do not terminate Runs.
- Both model paths skip an unfit reminder without input mutation, Journal emission, or error. Fitting reminders are stable on provider retry. Failed reminder persistence aborts the leg, preventing an unrecorded retry injection.
- SQLite/Postgres review arguments are projected from `json.RawMessage`, retaining `9007199254740993` and literal `[REDACTED]` rather than round-tripping through floating point.
- The Action Host preserves unrelated diagnostic text even after a Secret was resolved, while retaining the explicit actual-Secret authority boundary.
- Bash classification now scopes deny-table inspection to each AST command: literal traversal, home, remote-script, and fork-bomb-shaped output remains data across pipelines/sequences, while real escaping redirects, substitutions, executables, and path operations remain denied.
- `plugins/exp/redaction` exposes an explicitly invoked `std/tool@v1` tool; `plugins/exp/argument-guard` exposes an explicitly selected `std/middleware/pre-tool@v1` Provider. Neither is registered by default. `plugins/exp/loop-detection` is indefinitely deferred independent algorithm source, not a runnable observer/intervention Module.
- Development go-host snapshots seal tracked plus non-ignored untracked source, omit deleted entries, and exclude only untracked SDK-owned `.vivy-pack-*` staging; explicitly tracked prefix files remain sealed.
- Updated canonical architecture docs and marked historical Nudge plans/evidence as revision-bound. No new dependency, database schema, runtime, agent loop, discovery scan, or default EXP flag was introduced.

## Architecture / Eino capability check

Pinned Eino remains `v0.9.13`. The implementation retains `adk.ChatModelAgentMiddleware.WrapModel`, the existing `model.BaseModel[*schema.Message]` wrapper, `compose.GetToolCallID`, ADK Runner/checkpoint recovery, and `compose.IsInterruptRerunError`. The small Vivy-owned Nudge state is necessary to enforce Journal-before-model ordering and durable batch settlement, which a model callback alone cannot enforce. Its migration boundary is an upstream primitive providing those same durability contracts; no parallel producer/consumer loop was added. Eino imports remain quarantined to runtime/provider.

## Owner-approved turn-policy follow-up

The owner explicitly expanded the original Issue #40 scope to move the entire
eight-turn policy to EXP, rather than raising it to 10,000. Core no longer has
`runtime.max_tool_turns`, `EngineConfig.MaxToolTurns`, the child eight-turn clamp,
or their terminal-message special cases. Strict config decoding rejects the
retired key instead of silently ignoring it.

`plugins/exp/turn-limit` preserves the old default, configuration validation and
parent/child clamp policy as independent, tested source. It is deferred, not a
runnable Module: current public Ports cannot apply a Run-terminal iteration cap.
No Port, Descriptor, default selection, dependency or registration was added.

Eino capability check: `adk.ChatModelAgentConfig.MaxIterations` and the ordinary
and agentic React state constructors in the pinned `adk/react.go` treat all
non-positive values as twenty. Native `adk/chatmodel.go` already uses
`compose.WithMaxRunSteps(math.MaxInt)`. The thin runtime adapter therefore uses
the same platform-maximum sentinel for `MaxIterations`; there is no custom loop
or counter and no hidden replacement twenty/10,000-turn product policy. This is
not literal infinite computation: shared model/tool/event/retry resource
accounting, cancellation, context/output limits and workflow structural limits
remain authoritative and unchanged.

Pinned Eino has no supported resume setter for the iteration count retained in
opaque pre-follow-up checkpoints. Checkpoint compatibility version 2 therefore
rejects absent/older runtime epochs before Eino resume; pending work created by
the old policy requires an explicit new Run. Current checkpoints continue to
resume normally without mutating Eino's deprecated internal state.

Red regression evidence captured configuration still exposing the policy,
parent completion failing at Eino's default twenty and child completion failing
at eight. Fresh follow-up verification and acceptance are recorded separately
below the original iteration evidence; the earlier nine-call smoke used an
explicit sixteen-turn fixture and does not prove this follow-up.

## Verification and delivery state

See [verification](verification.md) and [acceptance](acceptance.md). Automated product-entry proof used the split Go/Vite pair and an isolated synthetic provider, not a tenant install or real provider credential. Owner UI E2E and Windows-native release acceptance are separate, unclaimed gates.

The implementation was committed and pushed with the human owner's author/committer identity after publication approval. PR #41 was closed because the integration created it with a bot author despite an explicit human author request. Human-authored PR publication is pending; no merge, issue closure, or release was performed.
