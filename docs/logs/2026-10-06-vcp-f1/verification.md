# VCP F1 — verification

Date: 2026-10-06. Branch: `feat/vivy-code-parity`.

## Plan test command (required)

```
go test ./internal/provider ./internal/modelhost ./internal/rpc -run 'Thinking' -count=1
ok  agent-vivy/internal/provider  0.039s
ok  agent-vivy/internal/modelhost 0.002s [no tests to run]
ok  agent-vivy/internal/rpc       0.277s
```

## Wider suites

```
go build ./...                          clean
go test ./internal/provider ./internal/rpc ./internal/domain ./internal/app/settings -count=1
ok  provider 0.286s / rpc 16.59s / domain 0.005s / settings 0.593s
go test ./internal/runtime ./internal/modelhost -count=1
ok  runtime 47.372s / modelhost 0.002s
go test ./sdk/tui/... -count=1
ok  all packages (command/face/i18n/live/stream/view)
go test ./sdk/internal/conformance -count=1
ok  70.621s (digest re-pinned → 797714daa7b0468247…)
cd ui && npx vitest run                 76 files / 589 tests pass
cd ui && npx tsc --noEmit               clean
```

## New tests appended for F1

- `internal/provider/resolving_thinking_test.go`
  - `TestThinkingLevelClampAndDefault` — clamp (`max`→`medium` on a
    `[minimal,low,medium]` model), `on`/`auto` → declared default,
    `off` → off, `on` with levels but no default → highest declared,
    undeclared model honors an explicit level verbatim.
  - `TestThinkingLevelClaudeBudget` — each of the six levels lands as
    `thinking:{type:enabled, budget_tokens:N}` on the Anthropic wire.
  - `TestThinkingLevelOpenAIReasoningEffort` — level → verbatim
    `reasoning_effort`; above-clamp request resolves to the declared max.
  - `TestThinkingLevelSamplingParamsReachBody` — `thinking_sampling`
    temperature/top_p reach the request body; undeclared level sends none.
  - `TestThinkingDeepSeekStaysBinary` — an explicit level on a DeepSeek
    endpoint maps to the canonical enabled + high request.
  - Migrated `TestDecideThinkingRuleTable` to the `domain.ModelInfo`
    signature; options-count expectation updated for the max_tokens
    companion option.
- `internal/rpc/control_thinking_test.go`
  - `TestModelThinkingSetPersistAndReport` — set persists into
    `settings.yaml`, bare call reports it, `auto` clears the overlay.
  - `TestModelThinkingRejectsUnknownLevelAndReadOnly` — bogus level →
    InvalidParams; no SettingsPath → CodeConflict.
  - `TestModelThinkingLevelsAnswersDeclaredSurface` — non-thinking model
    answers an empty surface.
- `sdk/tui/view/command_test.go` — `/thinking xhigh` accepted;
  `Ctrl+T` cycles `on → minimal` (level surface).
- `sdk/tui/command/command_test.go` — `/thinking max|xhigh` valid,
  `/thinking brain` rejected; help usage updated.
- `internal/provider/vendor_test.go` — `reflect.DeepEqual` for the
  unannotated-model assertion (Model gained a slice field).

## Found and fixed during verification

- Sampling overrides never reached the OpenAI adapter — `decideThinking`
  returned early before the sampling lookup. The openai case now falls
  through and sampling is keyed on `shape.effective`.
- `xhigh`/`max` claude requests were refused client-side by the Anthropic
  SDK's non-streaming guard (max_tokens > 128000/6) — now carry a
  documented `WithRequestTimeout(30m)` bypass when the combined budget
  crosses the ceiling.
- The go-openai fork rejects `temperature`/`top_p` on o1/o3/o4/gpt-5
  model ids client-side — real `vendors.yaml` declares no
  `thinking_sampling` for those families; the wire test uses a neutral
  model id.
