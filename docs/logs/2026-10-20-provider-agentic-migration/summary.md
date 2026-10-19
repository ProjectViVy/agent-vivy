# Provider agentic migration

## What changed

Vivy's model seam moved from Eino's text generation (`schema.Message` /
`model.ToolCallingChatModel`) to the agentic generation
(`schema.AgenticMessage` / `model.AgenticModel`), and the provider deferral
machinery that existed only to describe the un-executable Responses family was
deleted.

Scope, per the approved decisions:

- **D1 — one executable adapter set, no deferral.** `openai-responses`,
  `ErrAdapterDeferred`, `Adapter.State`, `modelhost.CapabilityState`,
  `Capabilities`, `Capability`, `CapabilitySupported`,
  `CapabilityDeferredIndefinite`, `ProfileDeferredIndefinite`, the reconcile
  deferral allowance, the RPC `catalogEndpointResult.Executable/State` wire
  fields, the UI adapter-state type/badge and the schema enum member are gone.
  The two remaining families are `openai-completions`
  (`agenticopenai.NewChatModel`) and `anthropic-messages` (`agenticclaude.New`).
  Runtime profile states `COMPILED/UNCONFIGURED/READY/UNAVAILABLE`,
  `ResolveExecutable`, `Statuses` and the fail-closed `ErrAdapterUnknown` stay.
- **D2 — one generation, no bridge.** Every runtime seam reads and builds
  `schema.AgenticMessage`. `modeladapter.go` is now the *domain*↔Eino
  projection (`domain.ChatModel` → `model.AgenticModel`), not a text-message
  bridge: it still speaks agentic messages, and the text-generation
  `WrapModel` bridge is deleted. No second generation and no conversion
  interval exists in the product path.
- **D3 — request-time tool binding.** Tools travel through `model.WithTools`;
  `resolvingChatModelWithTools` and both `WithTools` methods on the resolving
  wrapper are deleted.
- **D4 — the seven-level thinking policy is preserved per protocol.**
  `agenticopenai.WithExtraFields` carries the DeepSeek `thinking` object,
  `reasoning_effort` and the level sampling overrides;
  `agenticclaude.WithExtraFields` carries the Anthropic thinking object and
  `agenticclaude.WithRequestTimeout` covers the long non-streaming budget.
  Claude keeps its protocol-required `max_tokens` floor and its prompt-cache
  breakpoint. Outbound request bodies are asserted in
  `internal/provider/resolving_thinking_test.go` and
  `internal/provider/claude_test.go`.
- **D5 — journal schema unchanged.** Only the projection moved: reasoning
  blocks still reach the existing `model.reasoning_delta` domain event, and
  usage counters keep exact main/summary attribution.
- **D6 — typed entry points.** Compaction uses `reduction.NewTyped` and
  `summarization.NewTyped`; dynamic tool search uses `toolsearch.NewTyped`;
  skills use `skill.NewTyped`; AGENTS.md uses `agentsmd.NewTyped`. Vivy-owned
  middlewares (prompt budget, nudge, plan guidance, tool visibility, steering)
  implement `adk.TypedBaseChatModelAgentMiddleware[*schema.AgenticMessage]`.
- **D7 — the titler speaks agentic.** `internal/provider/titler.go` builds
  `schema.AgenticMessage` prompts and reads assistant text blocks.
- **D8 — the Eino quarantine holds.** Eino imports remain only in
  `internal/runtime/` and `internal/provider/`.

## Checkpoint generation

`checkpointEnvelope` gained a `Generation` field (`agentic` for new payloads).
An envelope with no field predates the migration and therefore reads as `chat`;
`VersionedCheckpointStore.Get` refuses it with the new
`ErrCheckpointGenerationMismatch`. Restart recovery routes that sentinel through
the existing `failLostCheckpoint` path, so a paused run is closed with the
existing engine-changed message and its pending approval/question/Plan review is
cancelled. Tests: `TestVersionedCheckpointStoreRefusesTheOtherGeneration`
(synthetic old-format envelope) and
`TestRecoverAcrossModelGenerationCancelsPendingApproval` (recovery path).

## Behaviour-preserving details worth knowing

- **Steering.** The text generation injected queued steer text through
  `adk.WithHistoryModifier`, whose signature is `[]*schema.Message` and whose
  internal state assertion does not match a typed agentic agent. Steering now
  rides a Vivy-owned typed middleware (`internal/runtime/steer_middleware.go`):
  the resume leg binds the steer text on the run context and the middleware
  appends it once, before a trailing assistant tool-call message, so the pinned
  `ToolNode` still runs the pending calls. It stays one run; `turn.steered` is
  still the single continuity marker.
- **Finish reason.** `AgenticResponseMeta` has no unified finish reason. The
  empty-length-stop overflow heuristic reads the per-protocol signal: the
  Anthropic `max_tokens` stop reason, the OpenAI incomplete reason, and the
  chat-completions component's own response-meta extension.
- **Prompt caching.** `agenticclaude.Config.CacheControl` applies the same
  top-level "last cacheable block" breakpoint the previous generation used. The
  parameter's `Type` must be set explicitly, because the SDK's `omitzero`
  encoding drops it when only the default tag would have filled it.
- **`codex`-style raw ids.** Gateway model ids still travel verbatim.

## What was explicitly not done

- `openai-responses` was **deleted**, not migrated. The decision removed the
  family rather than lifting it out of `DEFERRED-INDEFINITE`.
- No bridge, no dual-generation interval, no compatibility shim at the seam.
- `just ci`, Docker, Postgres, real-provider smoke and browser e2e were not run:
  those are the Lead's final gate.
- The three `internal/app` suite failures this Windows host reproduces on the
  untouched baseline (`schema_meta`/SQLite temp-dir locking) and the three UI
  suites that cannot resolve generated plugin sources are environment issues,
  not migration regressions; evidence is in `verification.md`.

## Documentation touchpoints

- `docs/TODO.md`: `PROVIDER-AGENTIC-MIGRATION` moved to `docs/COMPLETE.MD`;
  `DEEPSEEK-REASONING-CONTENT` closed (the upgraded agentic OpenAI client maps
  `reasoning_content` into reasoning blocks, which the mapper already projects).
- `sdk/ui/src/module.ts`: the Face provider vocabulary lost the retired adapter
  and the adapter capability state, keeping the SDK contract test green.
