# Issue #40 — Faithful core task data and advisory Nudge

Issue: https://github.com/ProjectViVy/agent-vivy/issues/40

Delivery branch: `feat/issue-40-faithful-data` (same implementation as `devin/1791405630-issue-40-faithful-data`).
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

Pinned Eino remains `v0.9.13`. The implementation retains `adk.ChatModelAgentMiddleware.WrapModel`, the existing `model.BaseModel[*schema.Message]` wrapper, `compose.GetToolCallID`, ADK Runner/checkpoint recovery, and `compose.IsInterruptRerunError`. Eino's native iteration budget remains independent of repetition. The small Vivy-owned Nudge state is necessary to enforce Journal-before-model ordering and durable batch settlement, which a model callback alone cannot enforce. Its migration boundary is an upstream primitive providing those same durability contracts; no parallel producer/consumer loop was added. Eino imports remain quarantined to runtime/provider.

## Verification and delivery state

See [verification](verification.md) and [acceptance](acceptance.md). Automated product-entry proof used the split Go/Vite pair and an isolated synthetic provider, not a tenant install or real provider credential. Owner UI E2E and Windows-native release acceptance are separate, unclaimed gates.

The implementation was committed and pushed with the human owner's author/committer identity after publication approval. PR #41 was closed because the integration created it with a bot author despite an explicit human author request. Human-authored PR publication is pending; no merge, issue closure, or release was performed.
