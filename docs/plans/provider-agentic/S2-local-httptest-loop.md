# S2 — Local `httptest` Minimal Closed Loop

**Goal:** prove the design end to end with **zero network access**, before any
production edit, and produce a pass/fail matrix that S4 can decide on.

**Status:** `UNSCHEDULED`. This document specifies the harness; it does not
contain its results.

**Depends on:** S1. **Blocks:** S3 (an inventory written against an unexercised
contract is guesswork).

---

## 1. Why a closed loop, and why it is local

Three things are being proven, and none of them needs a real provider:

1. That an `AgenticModel` from each of the three bindings drives an ADK typed
   agent — the migration's premise.
2. That the four channels the product depends on (text, tool call, reasoning,
   usage) survive the generational change.
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
repository.

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
| A3 | Reasoning | A `schema.Reasoning` content block appears, with signature where the protocol carries one, and its text matches the fixture |
| A4 | Usage | Token counts are non-zero and land where the run path can read them |
| A5 | Failure | One failure path per protocol: HTTP 500, malformed/truncated SSE, and `ctx` cancellation. Each returns an error; **none** returns a silently empty assistant message |
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
