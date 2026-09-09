# Verification — thinking mode (feasible UI-COMPOSER / UI-CHAT-TOOLBAR portion)

Date: 2026-09-02. All commands ran from the repository root (`agent-vivy`).

## Targeted kernel and RPC tests

```text
go test ./internal/provider -run 'Thinking' -count=1
  → ok  agent-vivy/internal/provider  0.385s
go test ./internal/runtime -run 'Thinking' -count=1
  → ok  agent-vivy/internal/runtime   2.031s
go test ./internal/runtime -run 'Thinking' -race -count=1
  → ok  agent-vivy/internal/runtime   2.936s
go test ./internal/rpc -run 'TestTurnStartThinkingRoute' -count=1
  → ok  agent-vivy/internal/rpc       1.070s
```

Coverage:

- `TestResolvingModelInjectsClaudeThinking` / `TestResolvingModelWithToolsInjectsThinking` —
  a local Anthropic-shaped httptest service asserts that the **outbound request body** contains
  `thinking: {"type":"enabled","budget_tokens":4096}` (both the bound-tool and bare-call
  paths).
- `TestResolvingModelOmitsThinkingWithoutRequest` — auto / off / unset all omit the
  `thinking` key (byte-for-byte consistent with existing traffic).
- `TestResolvingModelThinkingGatedOnMetadata` — even when requested on,
  claude-3-5-sonnet does not inject it (metadata gate active).
- `TestNormalizeThinkingMode` / `TestRunWithOptionsRejectsInvalidThinkingMode` —
  `""→auto` normalization; invalid values such as `execute` are rejected before persistence
  (`ErrInvalidThinkingMode`).
- `TestRunWithOptionsCarriesThinkingModeToTheModel` — a capture model confirms that
  `ThinkingModeFromContext` in the run-scoped context reads `on` throughout (a default run
  reads `auto`).
- `TestTurnStartThinkingRoute` (RPC integration) — invalid thinking on `turn/start` →
  InvalidParams; `on` → normal run creation; `session/context` reports the
  `thinking_supported` field.

## UI

```text
cd ui; pnpm typecheck
  → passed (no type errors)
pnpm test -- --run
  → all 24 test files / 197 tests passed
```

- `store.test.ts` adds 'carries the thinking preference through the queue': messages queued
  during a run retain their per-message thinking preference, and after the run completes
  `startTurn` receives `('s1','think hard','normal',undefined,undefined,'on')`.
- Existing queue tests were updated to exact six-argument matching (`vi.waitFor` strict
  argument count).

## Product gates

```text
just ci
  → CI-EXIT:0 (run in the background and confirmed from the log tail; includes
    gofmt/build/vet, all go tests, ui build + vitest, the headless build tag, and plugin-ci
    for six plugins)
```

## e2e smoke (split Vite real path)

```text
just ui-e2e
  → first round: the new spec's "thinking-mode button count 0" assertion passed, but
    locating the "attachment button" failed — the attachment control is
    `<label aria-label="附件">` (not a button role), so this was a test-locator mistake,
    not a product defect; the page snapshot confirmed that the toolbar, input box, and send
    key all rendered normally.
  → after changing it to `getByLabel('附件')` plus a textbox assertion, rerun:
    thinking-gate.spec.ts passed, with no regression in the full e2e suite (final result
    below).
```

Rerun result: the full e2e suite passed (18 passed + the new spec, 0 failed), `E2E-EXIT:0`.

## Explicitly outside this round's verification scope

- The OpenAI-family `reasoning_effort` is not wired (explicitly not done in the decision), so
  it cannot be verified.
- Outbound behavior against the real Anthropic upstream is represented by outbound-JSON
  assertions against a local Anthropic-shaped service; no real API key was consumed.
