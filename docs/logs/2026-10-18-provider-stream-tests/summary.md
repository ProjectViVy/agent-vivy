# Provider stream tests; DeepSeek reasoning row closed; B3/B4 recorded

## What changed

- **Tests only, no runtime change.** `internal/provider/provider_stream_test.go`
  streams hand-written SSE through `NewResolvingChatModel` (the model the
  runtime uses) for both sealed families: complete reply, tool call, provider
  HTTP 500, DeepSeek `reasoning_content`, and a cut-off stream.
- **`DEEPSEEK-REASONING-CONTENT` closed as already working**
  (`TestDeepSeekStreamSurfacesReasoningContent`; the runtime mapper projects
  `ReasoningContent` into `model.reasoning_delta`, covered by
  `internal/runtime/observability_test.go`). Row moved to `docs/COMPLETE.MD`.
- **`STREAM-TRUNCATION-SILENT-SUCCESS` stays a reported bug.** A guard that
  failed streams with no finish reason was built and dropped by owner decision:
  OpenAI-compatible servers that never send `finish_reason` look exactly like a
  cut-off stream, and the guard broke `sdk/codeclient`'s
  `TestClientAgainstRealVivyCode`. `TestStreamCutOffIsAcceptedKnownBug` pins
  today's behavior with a comment to flip it when fixed. Row is back in
  `docs/TODO.md` §0.1 as `OPEN · REPORTED BUG`.
- **Owner decisions** in provider-agentic S4: B3 and B4 accepted.
- `EINOEXT-CLAUDE-STREAM-RACE` note: the Anthropic cases reproduce it under
  `-race`.

## Not done

- No fix for cut-off streams (owner: report output-affecting defects as bugs
  first; safety issues considered separately).
- The `claude@v0.1.25` stream race is upstream.
