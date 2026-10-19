# S2 — Local `httptest` Minimal Closed Loop

**Goal:** prove upstream component/typed-agent feasibility with **zero external
network access**, before any production edit, and produce a pass/fail matrix
that S4 can decide on. This is not Vivy's product end-to-end acceptance.

**Status:** `DONE 2026-10-17`. §1–§8 are the harness spec as reviewed; the
matrix and findings are §9.

**Depends on:** S1. **Blocks:** S3 (an inventory written against an unexercised
contract is guesswork).

---

## 1. Why a closed loop, and why it is local

Three things are being proven, and none of them needs a real provider:

1. That an `AgenticModel` from each of the three bindings drives an ADK typed
   agent — the migration's premise.
2. That text, tool call, reasoning and usage survive the component-to-ADK loop,
   not yet Vivy's event projection, history reconstruction or UI.
3. That the failure paths fail closed.

A local fake is also the only way to test the failure paths at all: a real
provider will not reliably return a malformed SSE stream, and unit tests must be
deterministic with no live network (`AGENTS.md`).

## 2. Harness shape

One throwaway Go module in the **lane worktree**, gitignored, never under
`internal/` — an untracked file under `internal/` rewrites the `internal` source
digest and breaks the sealed-Generation gate (`docs/TODO.md:116`,
`PROVIDER-PROFILE-DIGEST-PIN`). Recommended location:
`.worktrees/provider-agentic/.spike/`, which is already inside a gitignored path
(`.gitignore:62,70`).

```
.spike/
  go.mod                      # module spike; requires eino v0.9.21 + the two components
  internal/harness/fake.go    # three httptest servers
  internal/harness/sse.go     # hand-written SSE fixtures
  cmd/loop/main.go            # one ADK typed agent per protocol, driven twice
```

Nothing from `.spike/` is committed and nothing from it is imported by the
repository. The harness does not traverse Vivy's modeladapter, Journal,
compaction, checkpoint store or UI; it cannot detect regressions in those
consumers. Their separate migration acceptance is S3 §2.7.

## 3. The three fakes

Each is an `httptest.Server` whose handler returns **hand-written** SSE. No
fixture is produced by the code under test (§5).

| Protocol | Endpoint | Fixture contents |
|---|---|---|
| `openai-completions` | `POST /chat/completions` | text delta; a `tool_calls` delta with `index`, `id`, `function.name`, partial `function.arguments`; a `reasoning_content`-style field; a final chunk carrying usage |
| `openai-responses` | `POST /responses` | a reasoning summary item producing a `schema.Reasoning` block; a `function_call` item with `call_id`, `name`, arguments; text output; usage |
| `anthropic-messages` | `POST /messages` | a `thinking` content block (`thinking` + `signature`); `text` block; `tool_use` block with `id`, `name`, `input`; usage |

Each server must also assert the **outbound request** it received (§5): base URL
path, the `model` field carrying this repository's raw model id (no
gateway-invented `provider/` prefix — root `AGENTS.md`, "Provider model IDs"),
the presence of the tools array when tools were bound, and the presence of the
reasoning control when it was requested.

## 4. The loop

Per protocol, two drives of the same agent:

1. **Plain reply.** One user message; expect assistant text.
2. **Tool round trip.** One user message; expect a tool call, feed a tool result
   back, expect a final assistant text.

Agent shape per protocol:

```go
agent, err := adk.NewTypedChatModelAgent[*schema.AgenticMessage](ctx,
    &adk.TypedChatModelAgentConfig[*schema.AgenticMessage]{
        Model: <the binding's AgenticModel>,
        // tools bound at request time via the model, options via adk
    })
runner := adk.NewTypedRunner[*schema.AgenticMessage](adk.TypedRunnerConfig[*schema.AgenticMessage]{Agent: agent})
```

The exact option plumbing (where `model.WithTools` is passed on the agentic
generation) is a S1 output; S2 consumes it.

## 5. Test discipline (transferred, not invented here)

These four rules come from the audit of the Python project investigated for this
decision, where their absence produced three verified silent-fallback hazards.
They are acceptance criteria here.

1. **Fixtures are hand-written.** Never generate an SSE fixture with the
   converter or codec under test. A generated fixture proves the code is
   self-consistent, not that it conforms.
2. **"The SDK parsed it" ≠ "the protocol conformed".** Assert the **outbound**
   request body directly — path, `model`, tools, reasoning controls. A client
   that sends a slightly wrong body and parses its own fake successfully is the
   failure mode this rule exists to catch.
3. **No defaulting `else` at a dispatch seam.** Every protocol dispatch is
   explicit; an unrecognised protocol is an error, never a fallback to another
   protocol's parameters, counters, or SDK.
4. **Lifecycle is asserted, not assumed.** After each drive, every
   `*schema.StreamReader` is closed and the stream reaches `io.EOF`; a cancelled
   or failed run leaves nothing open and leaks no goroutine that keeps writing.

## 6. Assertion channels

| # | Channel | Assertion |
|---|---|---|
| A1 | Text | Assistant text arrives, incrementally, and concatenates to the fixture's text |
| A2 | Tool call | Tool-call id, name, and arguments round-trip through the tool result and the second drive |
| A3 | Reasoning | A `schema.Reasoning` block matches the fixture; where the protocol carries signature or opaque/redacted reasoning, verify the required content survives the subsequent tool-round-trip outbound request, not merely inbound parsing |
| A4 | Usage | Exact fixture token counts and relevant cached/reasoning counters reach the ADK-readable result; product route attribution and duplicate suppression remain S3 §2.7 checks |
| A5 | Failure | For each protocol exercise HTTP 500, malformed/truncated SSE and `ctx` cancellation. Each surfaces the intended failure/cancellation; none reports silent successful empty output |
| A6 | Isolation | The three protocols can run in one process, sequentially and interleaved, without sharing state |

A5 and A6 are the ones most likely to fail, and are therefore the most valuable
part of the matrix.

## 7. Output

A pass/fail matrix over A1–A6 × the three protocols, plus:

- Every assertion that **cannot be expressed** on the agentic generation, listed
  explicitly. Each becomes an S4 `GO-CONDITIONAL` blocker or a `NO-GO` reason —
  not something to work around in the spike.
- The captured outbound request bodies for all three protocols, as the evidence
  for §5's rule 2.
- Any place where a required capability was reachable only by a Vivy-owned
  adapter, recorded so S3 can inventory it.

## 8. Explicit non-goals

- No live provider calls and no API keys. The harness is loopback-only.
- No production code change, and no `go.mod` change in this repository (D8).
- No fix is written here for a failure the harness finds. A failure is data for
  S3 and S4.

## 9. Result (2026-10-17)

### 9.1 Harness as built

Throwaway module in the session scratchpad (not in the repository, not under
`internal/`), Go 1.26.4, `eino v0.9.21`, `agenticopenai v0.2.4`,
`agenticclaude v0.1.7`. Three `httptest` servers serve hand-written SSE
(`openai-completions` `POST /chat/completions`, `openai-responses`
`POST /responses`, `anthropic-messages` `POST /v1/messages`), one tool turn
and one final turn each, and record every outbound body. One
`adk.NewTypedChatModelAgent[*schema.AgenticMessage]` + `adk.NewTypedRunner`
(`EnableStreaming: true`) per protocol; per-request options through
`adk.WithChatModelOptions`. Every dispatch `switch` ends in `t.Fatalf`, never a
default. Each case closes its fake and checks that goroutines return to
baseline. Not committed, per §2; its full description is in the iteration
log's `notes.md`.

Two harness bugs were fixed during the run: a zero-value
`anthropic.CacheControlEphemeralParam{}` (omitted on the wire, so it was
switched to `anthropic.NewCacheControlEphemeralParam()`), and a goroutine check
that ran before the isolation test's fakes were closed. No fixture was changed
to make a component pass.

### 9.2 Matrix (`go test -race -count=1`)

| Channel | `openai-completions` | `openai-responses` | `anthropic-messages` |
|---|---|---|---|
| A1 text (incremental, exact concat) | PASS | PASS | PASS |
| A2 tool call (id, name, args, executed, result replayed) | PASS | PASS | PASS |
| A3 reasoning inbound | PASS | PASS | PASS (text + signature) |
| A3 reasoning replayed in turn 2 | PASS (`reasoning_content`) | **FAIL**: reasoning item replayed by id only; `encrypted_content` not carried | PASS (`thinking` + `signature`) |
| A4 usage | PASS (prompt/completion/total/cached/reasoning) | PASS | **PARTIAL**: total 12 ≠ 18 when `message_delta` has output tokens only; PASS with cumulative delta usage |
| A5 HTTP 500 | PASS (error) | PASS (error, after SDK's own retries ≈1.25 s) | PASS (error, after SDK retries) |
| A5 truncated SSE (clean EOF mid-stream) | **FAIL**: partial text reported as success | **FAIL**: same | **FAIL**: same |
| A5 ctx cancel mid-stream | PASS (error ≤300 ms) | PASS | PASS |
| A6 isolation (3 rounds × 3 protocols, concurrent, `-race`) | PASS | PASS | PASS |
| Lifecycle (streams closed, goroutines settle) | PASS | PASS | PASS |

Outbound bodies (rule 2), all PASS: path, raw `model` id with no prefix,
`stream: true`, tools bound on both turns; completions carries
`thinking:{"type":"enabled"}` and `reasoning_effort:"high"` from per-request
`WithExtraFields` plus `stream_options.include_usage`; Responses carries
`reasoning:{"effort":"high","summary":"auto"}`, no `store` field (server
default); Claude carries per-request
`thinking:{"budget_tokens":2048,"type":"enabled"}`, `max_tokens`, top-level
`cache_control`.

Extra probes:

- **C3 deferred tools** (direct `Stream` with `WithTools` + `WithDeferredTools`):
  Responses sends `defer_loading:true` + `{"type":"tool_search"}`; Claude
  sends `defer_loading:true`; **completions drops the deferred tool silently**.
- **R10** (`EINO-TOOLSNODE-ERR-RACE`): 20 runs of two parallel *enhanced
  streamable* tools on the agentic tools node under `-race` — **no race**.
  Consistent with the source: v0.9.13's shared `err` write
  (`compose/tool_node.go:1253`) is a local `convertErr` in v0.9.20+
  (`:1365`).

### 9.3 Findings

1. **Truncated stream = silent partial success, on all three agentic
   components.** **Not a regression**: the same fixture on the *current*
   components at `main`'s exact pins (eino v0.9.13, `openai v0.1.13`,
   `acl/openai v0.1.17`, `go-openai v0.1.2`, `claude v0.1.25`) also returns
   `"Sunny "` with no error. None of the components checks for the terminal
   event (`[DONE]` / `response.completed` / `message_stop`). Fails T3's A5 as
   written; it is a pre-existing product defect, recorded on the board.
2. **Responses streaming drops `encrypted_content`.** Multi-turn reasoning
   then depends on OpenAI keeping the response (`store` defaults to true).
   Stateless mode (`store:false` + `include:["reasoning.encrypted_content"]`)
   cannot replay reasoning on the streaming path with `v0.2.4`.
3. **Claude `TotalTokens`** is assembled by per-field max over the
   `message_start` and `message_delta` chunks, so it comes out wrong
   (`input+1`) when the delta carries output tokens only. Prompt, completion
   and cache counters are exact either way.
4. **Completions drops `WithDeferredTools` silently** (C3). Harmless while Vivy
   filters tools client-side.
5. **Side finding, current production component:** `components/model/claude
   v0.1.25` (and the newest `v0.1.26`) races in `(*ChatModel).Stream`: the
   stream goroutine assigns the function-scope `err`
   (`claude.go:446` `message, err = schema.ConcatMessages(...)`). The race
   detector reports it on a plain streamed reply, on the branch taken when an
   empty chunk such as `message_start` is buffered before content.
   `agenticclaude` does not show it. Recorded on the board.
6. **Correction to S3 §2.6 / C6:** reasoning is **already** surfaced by the
   current stack. At `main`'s exact pins, `openai v0.1.13` returns DeepSeek
   `reasoning_content` in `Message.ReasoningContent`, and `claude v0.1.25`
   returns thinking there too. Vivy's mapper projects that field into
   `model.reasoning_delta` (`internal/runtime/mapper.go:490-495`, tests at
   `observability_test.go:15,224`). "No adapter produces reasoning content
   today" does not hold at the component level; `DEEPSEEK-REASONING-CONTENT`
   looks stale and needs a product-path check before it is closed.

7. **User message shape changes on chat completions.** `agenticopenai`'s
   chat model always sends user `content` as an array of parts
   (`[{"type":"text","text":…}]`); the current `components/model/openai
   v0.1.13` sends a plain string (both captured against the same fake). The
   library collapses a single text part to a string for tool results
   (`libs/acl/openai agentic_convert.go:273-274`) but not for user messages,
   and exposes no option for it. OpenAI accepts both forms; some
   OpenAI-compatible gateways accept only strings. Not provable with a local
   fake — needs an authorized live check on DeepSeek (the default provider) and
   the other compatible vendor families.

### 9.4 Assertions that could not be expressed

None. Every channel was expressible on the agentic generation; the failures
above are behavior, not missing API.

### 9.5 Capabilities reached only through a Vivy-owned adapter

None beyond the thin per-request option projections listed in S1 §9.4.
