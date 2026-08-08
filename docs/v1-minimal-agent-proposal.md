# V1 Minimal Agent Layer — Capability Proposal

> Status: **approved entry** into V1 scope (RK-2/RK-4, IMPLEMENTATION-PLAN
> §9). V0 closed with a deliberately narrow surface; this proposal is the
> first capability re-entry. It benchmarks the *shape* of `.workspace/pi`
> (read-only reference) without cloning any of its code (D-001/D-005).
> Companion document: `AGENT-VIVY-ARCHITECTURE-V0.md` (ADR baseline,
> closes SR-1/D-035; ADR-009 records the MA-1 decision).

## 1. Why — the verified gap ("goldfish brain")

Vivy today answers every turn as if it were the first turn.

Verified against eino v0.9.13 source (not assumed):

- `adk.Runner.Query(ctx, text)` wraps the single user message into one
  fresh `Run`; the Runner keeps **no cross-Query memory**.
- `internal/runtime/service.go` `drive()` calls exactly that: one
  `userText` per run.
- The journal already persists the full transcript (every `user` and
  `assistant` message, in `seq` order) — the model simply never sees it.

Result: anaphora breaks across turns ("save it again" cannot resolve
"it"), stated facts evaporate, and the persisted history is product-
invisible. This is a product defect against the logs-as-first-class
anchor (§5.0.2): history exists but does not work.

Secondary verified facts that shape this proposal:

- `adk.Runner.Run(ctx, messages, opts...)` accepts an arbitrary message
  list, and `WithCheckPointID` applies to it — so history can be fed
  without touching the checkpoint/resume contract.
- `ChatModelAgentConfig.MaxIterations` defaults to 20 and exceeding it
  surfaces `ErrExceedMaxIterations` — a ready hook for loop guardrails.
- `Instruction` is static agent configuration; there is no per-run
  instruction option — so dynamic context must enter as leading messages.
- `write_note` currently appends to an in-process `[]string`; notes do
  not survive a restart.

## 2. Reference shape — `.workspace/pi` (`packages/agent`)

Read-only benchmark. What pi's minimal agent layer looks like, and what
Vivy's equivalent is (or will be):

| pi concept | pi location | Vivy today | Vivy after this batch |
|---|---|---|---|
| Turn loop (assistant → toolCalls → execute → result back → repeat) | `agent-loop.ts` (~750 lines) | eino `ChatModelAgent` + `Runner` already loop inside one run | Unchanged — the in-run loop stays Eino's (D-007) |
| Message / tool / event protocol | `types.ts` (AgentContext / AgentTool / AgentEvent) | Vivy-owned `ToolSpec`, ten-type `RunEvent` vocabulary (ADR-004/006) | Unchanged |
| Context transform hook | `transformContext` | None — single static `Instruction` | MA-2 per-run prompt preamble |
| Stop hook | `shouldStopAfterTurn` | Implicit eino `MaxIterations` (default 20, unmapped) | MA-4 explicit `MaxToolTurns` with classified failure |
| Session history | harness feeds conversation into each turn | Journal persists it; engine never receives it | MA-1 history rebuild feed (ADR-009) |
| Memory / notes | harness-tier persistence | `write_note` in-memory list | MA-3 persisted notes trio |

The comparison is structural only: Vivy's approval gate (two-layer
checkpoint bridge, six-step approval ordering, ADR-006) is already
stronger than pi's steering queue and is not touched.

## 3. Goals and acceptance — MA-1..MA-4

### MA-1 — Session history feed (highest priority)

- **Goal.** Every run enters the engine via `Runner.Run` carrying the
  session transcript rebuilt from the message store (`user` →
  `schema.UserMessage`, `assistant` → `schema.AssistantMessage`, in
  `seq` order).
- **Acceptance.** Scripted-model test: the second run's model input
  contains the previous turn's user+assistant pair plus the new user
  message; cross-session isolation holds; replay/recovery/cancel suites
  stay green.
- **Decision recorded:** ADR-009 (text pairs only; tool exchanges stay
  out of cross-turn context).

### MA-2 — Per-run prompt composition

- **Goal.** A pure-text composer (no eino imports) assembles a run
  preamble: persona + current date + available-tool guidance generated
  from the registered `ToolSpec`s. The preamble is injected as the first
  message of the run's message list; the static `Instruction` line stays.
- **Acceptance.** Composer unit tests (date, tool listing,
  determinism); service-level assertion that the first fed message is
  the preamble; secret-boundary guards (E3) stay green — the composer
  never touches env/credentials.

### MA-3 — Persisted notes trio

- **Goal.** Migration 003 adds a `notes` table; a `NoteStore` contract
  (+ SQLite backend, bounded by the existing `noteContentLimit`) replaces
  the in-memory list. `write_note` becomes durable; two read-only tools
  join: `list_notes` (count + bounded summaries, auto-execute) and
  `read_note` (full text by id, auto-execute). Recent note digests enter
  the MA-2 preamble.
- **Acceptance.** Store CRUD tests; contract tests for both new tools
  including `*ArgError` branches; approval chain green; a note written
  through approval survives a restart.

### MA-4 — Loop guardrails

- **Goal.** `EngineConfig.MaxToolTurns` (default 8, configurable via
  `runtime.max_tool_turns`) maps onto `ChatModelAgentConfig.
  MaxIterations`; exceeding it is classified as `run.failed` with a
  bounded `internal_error` cause (no internals leaked).
- **Acceptance.** A scripted tool-loop model breaches the cap → exact
  terminal state and cause asserted; runs inside the cap complete
  unchanged.

## 4. Anti-clone statement (D-001/D-005)

This proposal borrows **architecture shape only** from `.workspace/pi`:

- No TypeScript source is translated, paraphrased, or vendored. pi's
  code cannot enter the Go dependency graph anyway; the AST-level
  import-lint gate already bans `.workspace`/`agent-diva` imports and
  runs on every commit.
- `.workspace/pi` and `agent-diva/` remain read-only; nothing under them
  is modified.
- Vivy's loop stays Eino's run-internal loop behind the D-007 quarantine;
  this batch adds feed, composition, persistence, and guardrails around
  it — not a second agent loop.

## 5. Out of scope (stay deferred, TODO §10)

- Context compaction / summary compression (history is fed raw and
  bounded by session length for now).
- Memory / RAG retrieval.
- Multi-agent orchestration, MCP, plugin systems.
- Additional providers, filesystem-journal backend.
- Any change to the approval/checkpoint contract.

## 6. Delivery shape

Six atomic commits, each gate-green (gofmt, vet, build,
`go test -race -count=1 ./...`) before the next; no pushes. Order:
docs (this proposal + ADR baseline) → MA-1 → MA-2 → MA-3 → MA-4 →
closure (real-gateway walkthrough evidence + TODO updates + full
regression).
