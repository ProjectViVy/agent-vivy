# Notes — provider agentic spike raw evidence

## Harness (not committed, per S2 §2)

Scratch module `spike` (Go 1.26.4; eino v0.9.21, agenticopenai v0.2.4,
agenticclaude v0.1.7, plus openai v0.1.13 / claude v0.1.25 for comparisons):

- `sse.go` — hand-written SSE: completions (`reasoning_content`, `tool_calls`
  split across two deltas, usage chunk, `[DONE]`); Responses (`reasoning`
  item with summary deltas and `encrypted_content`, `function_call` with
  argument deltas, `output_text` deltas, `response.completed` with usage);
  Anthropic (`thinking` + `signature_delta`, `tool_use` + `input_json_delta`,
  `text`, `message_delta` usage). Fixture facts: text `"Sunny in Paris."`,
  reasoning `"think hard"`, call `call_1`/`get_weather`/`{"city":"Paris"}`,
  usage 11/7/18, cached 3, reasoning tokens 2, cache write 4. `truncate()` cuts
  before the first `"Paris"`.
- `harness_test.go` — recording `httptest` fake (scripted replies, 500,
  hang-until-cancel), `weatherTool`, protocol table, typed agent/runner
  builder, stream drain + `ConcatAgenticMessages`, goroutine settle check.
- `loop_test.go` — `TestPlainReply`, `TestToolRoundTrip` (outbound rule 2 +
  replay checks), `TestFailures` (500/truncated/cancel), `TestIsolation`
  (3×3 concurrent), `TestParallelEnhancedTools` (R10).
- `compare_test.go`, `current_reasoning_test.go`, `deferred_test.go` — Claude
  cumulative-usage variant, current-generation truncation/reasoning, C3.
- A second module pinned to `main`'s exact current versions for the
  like-for-like current-stack checks.

The harness can be re-created from this description and S2 §9; it is kept out
of the repository so nothing under `internal/` or the module graph changes.

## Outbound bodies, tool round trip (captured by the fakes)

```text
openai-completions request 2
{"messages":[{"content":"be brief","role":"system"},
  {"content":[{"text":"weather in Paris?","type":"text"}],"role":"user"},
  {"reasoning_content":"think hard","role":"assistant","tool_calls":[{"function":{"arguments":"{\"city\":\"Paris\"}","name":"get_weather"},"id":"call_1","type":"function"}]},
  {"content":"18C sunny","role":"tool","tool_call_id":"call_1"}],
 "model":"deepseek-chat","reasoning_effort":"high","stream":true,
 "stream_options":{"include_usage":true},"thinking":{"type":"enabled"},
 "tools":[{"function":{"name":"get_weather",…},"type":"function"}]}

openai-responses request 2
{"input":[{"content":"be brief","role":"system"},{"content":"weather in Paris?","role":"user"},
  {"id":"rs_1","summary":[{"text":"think hard","type":"summary_text"}],"type":"reasoning"},
  {"arguments":"{\"city\":\"Paris\"}","call_id":"call_1","id":"fc_1","name":"get_weather","status":"completed","type":"function_call"},
  {"call_id":"call_1","output":[{"text":"18C sunny","type":"input_text"}],"type":"function_call_output"}],
 "model":"gpt-5","reasoning":{"effort":"high","summary":"auto"},"stream":true,
 "tools":[{"name":"get_weather",…,"type":"function"}]}
 # no "encrypted_content" on rs_1; no "store"/"include" fields

anthropic-messages request 2
{"cache_control":{"type":"ephemeral"},"max_tokens":4096,
 "messages":[{"content":[{"text":"weather in Paris?","type":"text"}],"role":"user"},
  {"content":[{"signature":"sig_abc","thinking":"think hard","type":"thinking"},
              {"id":"call_1","input":{"city":"Paris"},"name":"get_weather","type":"tool_use"}],"role":"assistant"},
  {"content":[{"content":[{"text":"18C sunny","type":"text"}],"tool_use_id":"call_1","type":"tool_result"}],"role":"user"}],
 "model":"claude-sonnet-4-5","stream":true,"system":[{"text":"be brief","type":"text"}],
 "thinking":{"budget_tokens":2048,"type":"enabled"},
 "tools":[{"input_schema":{…},"name":"get_weather","type":"custom"}]}
```

Current `components/model/openai v0.1.13` at `main`'s pins, same fake:
`"messages":[{"role":"system","content":"be brief"},{"role":"user","content":"weather in Paris?"}]`
(string content — compare the array above; S2 §9.3.7).

## Deferred tools on the wire (C3)

```text
completions: [{"function":{"name":"get_weather",…},"type":"function"}]            # hidden_tool absent
responses:   [{…"get_weather"…},{"defer_loading":true,"name":"hidden_tool",…},{"type":"tool_search"}]
anthropic:   [{…"get_weather"…},{"defer_loading":true,"name":"hidden_tool",…}]
```

## Truncated stream results

```text
agentic   openai-completions  -> assistant text "Sunny ", err nil
agentic   openai-responses    -> assistant text "Sunny ", item status "in_progress", err nil
agentic   anthropic-messages  -> assistant text "Sunny ", err nil
current   openai v0.1.13      -> text "Sunny ", err nil
current   claude v0.1.25      -> text "Sunny ", err nil
```

## Claude usage

```text
output-only message_delta: prompt 11, completion 7, total 12, cached 3, cache_write 4
cumulative message_delta:  prompt 11, completion 7, total 18, cached 3, cache_write 4
```

## Data race (current Claude component)

```text
WARNING: DATA RACE
  github.com/cloudwego/eino-ext/components/model/claude.(*ChatModel).Stream.func2()
      claude@v0.1.25/claude.go:446
  github.com/cloudwego/eino-ext/components/model/claude.(*ChatModel).Stream()
      claude@v0.1.25/claude.go:419
v0.1.26 claude.go:446: message, err = schema.ConcatMessages(append(waitList, message))   # unchanged
```

## ToolsNode race source (R10)

```text
eino@v0.9.13 compose/tool_node.go:1253  ret[index].UserInputMultiContent, err = tr.ToMessageInputParts()
eino@v0.9.21 compose/tool_node.go:1365  parts, convertErr := tr.ToMessageInputParts()
```
