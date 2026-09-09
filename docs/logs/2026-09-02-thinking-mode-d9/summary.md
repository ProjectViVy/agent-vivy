# Summary — end-to-end thinking-mode wiring (feasible UI-COMPOSER / UI-CHAT-TOOLBAR portion)

## What changed

The chat box's thinking mode is now wired from UI-only state to real kernel capability,
gated under D9 (model metadata is managed by provider/model, with no separate data source):

- **domain**: add `ThinkingMode` (auto/on/off) and carry it in run-level context
  (`WithThinkingMode` / `ThinkingModeFromContext`); add `SupportsThinking` to `ModelInfo`
  (conservatively false at the zero value).
- **provider**: annotate thinking support per model in the Anthropic catalog (Claude 3.7
  Sonnet and later generations are true); `resolvingChatModel` reads run context on each
  call and injects `einoclaude.WithThinking` only when thinking=on and catalog metadata says
  it is supported (budget 4096, below the protocol max_tokens 8192). Unknown models and
  non-Anthropic backends never send the parameter — structurally matching the attachment's
  SupportsImages gate (zero value defaults do not break custom gateways).
- **runtime**: validate `RunOptions.Thinking` (empty = auto) through
  `normalizeThinkingMode`, then write it to runCtx; reject invalid values before persisting
  any data (`ErrInvalidThinkingMode`).
- **rpc**: add a `thinking` parameter to `turn/start` (invalid value → InvalidParams); add
  `thinking_supported` to `session/context` (the D9 gate surface from which the UI decides
  whether the selector is visible).
- **UI**: render the `ChatInput` thinking selector only when
  `context.thinking_supported` is true (D9 gate — dead controls never appear); carry the
  preference through both send paths (direct send and queue): ChatInput → ChatView →
  store.startRun → `api.startTurn` → turn/start.

## Explicitly not done

- **The `reasoning_effort` control on the OpenAI-compatible path**: the eino openai
  adapter has `WithReasoningEffort`, but the "off" semantics for the o-series/gpt-5 are not
  uniform (`minimal` is valid for only some models); it is not wired in this generation. All
  metadata is false, so the selector does not appear. Open a separate item if needed later.
- **Forced 'off'**: Anthropic does not think by default, and auto/off both mean "do not send
  the parameter"; for always-on reasoning models (the o-series), off falls back to the
  provider default.
- **AutoDream / question mode / desktop companion / voice**: remain stubs (AutoDream, etc.,
  MEM-1 DEFERRED; question mode has no kernel semantics).
- **Persistence of thinking preferences**: this round follows Crush semantics with a
  per-turn choice; there is no session-level persistence.

## Filing

- Updated the TODO-line notes for UI-COMPOSER / UI-CHAT-TOOLBAR (the feasible portion is
  delivered; remaining items include their blocking reasons).
