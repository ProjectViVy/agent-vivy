# Acceptance — end-to-end thinking-mode wiring

## How to prove it works to a human

1. **Gate off (default environment with no provider)**: open the chat page at
   `http://127.0.0.1:3015`, create a session — the toolbar has **no** thinking-mode lightbulb
   button (hidden by the D9 gate), while the attachment button remains. The permanent
   regression in `ui/e2e/thinking-gate.spec.ts` covers this behavior.
2. **Gate on (Anthropic thinking-generation model)**: in Settings → Model, choose the
   anthropic bundle and switch the model to `claude-sonnet-4-5` (or any 3.7+/4+ generation),
   then return to the chat page — the toolbar shows the thinking-mode selector; choose "On"
   and send a message, and the model can return reasoning content (`model.reasoning_delta`
   event, already rendered in the TUI/bubbles).
3. **The parameter is actually sent**: kernel-level proof is in
   `internal/provider/resolving_thinking_test.go` — a local Anthropic-shaped server asserts
   that outbound JSON carries `thinking: {"type":"enabled","budget_tokens":4096}`
   (only on + supported generations; auto/off/unknown models never include the thinking key).
4. **Invalid values are rejected**: `turn/start` with `thinking:"execute"` → InvalidParams
   (`TestTurnStartThinkingRoute`); reject before execution, with no residual run in the
   Journal.

## Boundary behavior

- Queued messages retain their individual thinking preferences and are sent with the choice
  made when they entered the queue.
- An old client that omits thinking → auto → behavior is byte-for-byte identical to before.
