# Capability Proposal — Eino-first context loop, session goal, session query

> Status: **proposal** (not approved entry; does not change ADR/NG until
> accepted). Date: 2026-08-29.
> Why now: plugin-anchored DSH gap research named compaction / goal / query
> as the next real species debts
> (`docs/research/DSH-PLUGIN-ANCHORED-VIVY-GAPS.md` §4.1 L-COMPACT /
> L-GOAL / L-QUERY). Prior loop research already verified Eino hooks
> (`docs/research/AGENT-LOOP-PORT-COMPARISON.md` §4.3-E2, §7.1).
> Companion: `docs/v1-minimal-agent-proposal.md` (MA-1..4 closed shape);
> `docs/AGENT-VIVY-ARCHITECTURE-V0.md` ADR-009/010 (history feed; compaction
> deferred until this proposal is accepted).
> **Board:** `docs/TODO.md` §0.1 **ACTIVE** lane — ids `EIN-0`, `EIN-A*`,
> `EIN-B*`, `EIN-C*` (opened 2026-08-30).

---

## 0. One-line intent

Ship the next harness slice by **wiring Eino ADK capabilities we already
depend on**, then add only the Vivy-owned surfaces Eino does not provide
(Journal events, session goal object, FTS query). Do **not** invent a second
agent loop, do **not** port DSH Cordis plugins, do **not** open MEM-1.

---

## 1. Why — verified product gaps

| Gap | User-visible failure | Evidence |
|---|---|---|
| **Context pressure** | Long sessions hit hard caps or lose early tool detail; no compact event, no graceful “summarize then continue” | ADR-009/010 defer compaction; `buildRunContext` byte/count trim only; `tooladapter` single-shot head/tail; `internal/runtime/compaction` meter/pruner **unwired**; engine Handlers today = tool-selection + optional skill only (`engine.go`) |
| **Goal object** | UI GoalBar fakes progress from `task_*` in_progress text; no revisioned objective, pause/block/complete, or round driver | `docs/TODO.md` UI-GOAL; DSH `goal/*` has no Eino twin |
| **Session query** | Trajectory / review search stay demo or full scan; model cannot FTS prior turns | UI-TRAJ demo; no FTS migration (IMPLEMENTATION-PLAN avoided FTS in V0) |

Secondary loop hygiene already designed against Eino (same research batch):
summary-only reward pass, empty-output classification, unknown-tool handler.
This proposal **folds them into Track A** so one engine PR does not thrash
`Handlers` order twice.

---

## 2. Eino reuse matrix (source of truth)

Verified against `.workspace/eino` (module `github.com/cloudwego/eino`
v0.9.13 in `go.mod`) and current Vivy wiring.

### 2.1 Adopt / wire (Track A core)

| Eino piece | Path | What it gives Vivy | Vivy bridge (required) |
|---|---|---|---|
| **`reduction` middleware** | `adk/middlewares/reduction` | Deterministic two-phase compress: truncate oversized tool results; clear old tool rounds before model call (`BeforeModelRewriteState`) | `Backend` → existing run workspace fs backend (`filesystem_backend.go`); `ReadFileToolName: "read_file"`; `ClearPostProcess` → emit Journal `context.compacted`; config thresholds under `runtime.loop.reduction.*`; **Journal `tool.finished` stays full inline** (no D16 artifact rewrite) |
| **`summarization` middleware** | `adk/middlewares/summarization` | LLM summary when token/count trigger fires; replaces in-loop history | `Model` from Vivy provider stack (same quarantine as chat); `EmitInternalEvents: true` → mapper → budget ledger; `Callback`/`Finalize` → `context.compacted` + optional transcript path **inside run workspace only**; never write tenant Journal blobs from middleware |
| **`MaxIterations`** | `ChatModelAgentConfig` | Already mapped (MA-4) | Unchanged; reward-pass must run **before** hard fail semantics (see order) |
| **`UnknownToolsHandler`** | `ToolsNodeConfig` / react tools node | Graceful unknown tool name → corrective message instead of hard fail | One-liner + scripted test (research P5) |
| **`ChatModelAgentMiddleware` chain** | `adk.ChatModelAgentConfig.Handlers` | Ordered hooks already used for tool-selection + skill | Append reduction → summarization → reward-pass; **document order** |
| **`CustomizedOutput` / internal events** | `adk.AgentEvent` | Observe summarize/reduce without scraping logs | `mapper.go` branch → new `EventType`s |
| **Checkpoint store** | Runner `CheckPointStore` | In-run state already bridged | Compacted message list must remain resume-safe; add recovery test |

### 2.2 Already adopted (do not re-propose)

| Eino piece | Vivy status |
|---|---|
| `plantask` backend | `todo_backend.go` + `task_*` tools (ET-03) |
| `skill` middleware | `engine.go` + `skills_backend.go` |
| `filesystem` backend pattern | `filesystem_backend.go` |
| Runner `Run(messages)` history feed | MA-1 / ADR-009 |
| Interrupt + `ResumeWithParams` | Approval path |
| Tool adapters as `einotool.BaseTool` | `tooladapter.go` |

### 2.3 Explicitly **not** adopted from Eino

| Eino piece | Why not |
|---|---|
| `prebuilt/deep`, `planexecute`, `supervisor` | Product-shaped agents; anti-clone (D-001/D-005); research E8 |
| `transfer_to_agent` / Sequential/Parallel/LoopAgent | Eino marks transfer family NOT RECOMMENDED; swarm later via optional `NewAgentTool` only |
| `patchtoolcalls` | Overlaps `pairToolTurns`; research E7 |
| Building a second loop outside `ChatModelAgent` | D-007 quarantine; species loop stays Eino |
| GraphTool as production workflow engine | ET-06 tests only; prd non-goal for graph workflow |
| Turning compaction offloads into Journal canonical artifacts | Conflicts event model (research D16) |

### 2.4 Eino has **no** package for

| Need | Consequence |
|---|---|
| DSH-style **session goal** (revision, phase, round driver) | Track B is **Vivy domain + tools + RPC**; may *use* plantask only as optional checklist under a goal, never as the goal itself |
| **SQLite FTS** session search | Track C is storage + RPC (+ optional read-only tool); not an ADK middleware |
| OS process sandbox / PTY / ACP | Out of this proposal (SBX-OS, terminal, ACP-1 stay own tracks) |

---

## 3. Tracks and acceptance

### Track A — In-loop context governance (**Eino-first**, highest priority)

**Goal.** While a run is alive, context stays inside budget via official
middlewares; pressure is observable; max-iterations fails more usefully;
unknown tools do not explode the turn.

| ID | Work | Eino? | Acceptance |
|---|---|---|---|
| **A1** (`EIN-A1`) | Wire `reduction.New…` into `Engine` Handlers | Yes | Scripted long tool result → truncated/cleared in **model input**; workspace offload readable via `read_file`; Journal still has full `tool.finished` payload (or existing bounded form—**no silent shrink of durable tool.finished** without explicit ADR) |
| **A2** (`EIN-A2`) | Wire `summarization` with Vivy `Model`, `EmitInternalEvents` | Yes | Scripted token/count trigger → summary replaces in-agent messages; at least one `context.compacted` (or mapped internal) event in Journal; `BudgetLedger` counts summary model call |
| **A3** (`EIN-A3`) | Map internal compact/summarize signals in `mapper.go` | Thin Vivy | New event types in `domain` + schema under `schemas/events/`; UI can ignore initially |
| **A4** (`EIN-A4`) | Summary-only reward pass middleware | Vivy middleware on Eino hook | On last iteration budget, `BeforeModelRewriteState` clears `ToolInfos`, forces one prose pass; emits `loop.summary_pass`; then MA-4 fail path still works if empty |
| **A5** (`EIN-A5`) | `UnknownToolsHandler` | Yes | Unknown name → tool error string to model; run can continue; test |
| **A6** (`EIN-A6`) | Config `runtime.loop.*` | Vivy | Parse/validate tests; defaults safe (reduction on with conservative thresholds; summarization **opt-in or high threshold** until soak) |
| **A7** (`EIN-A7`) | Downgrade `internal/runtime/compaction` | Vivy | `meter` used by preflight/pressure warning; `pruner` not primary engine (fallback only if middlewares disabled) |

**Handler order (normative for A):**

```text
1. toolSelectionMiddleware          // existing
2. skill middleware                 // existing, optional
3. reduction                        // truncate/clear before model
4. summarization                    // optional LLM compact
5. summaryRewardPass                // last-chance prose-only
```

Skill stays after tool-selection (existing comment in `engine.go`). Reduction
before summarization (cheap deterministic first). Reward pass last among
rewrite hooks so it sees already-reduced history.

**Invariant bridge (non-negotiable):**

1. **Model-visible ⇔ logged (derived):** any message list mutation that
   affects the next provider call must leave a Journal fact
   (`context.compacted` and/or `model.request` summary already exists).
2. **Exactly-one-terminal** unchanged.
3. **Secrets:** summarization Model uses same env_key path; compacted
   payloads run existing redact path.
4. **Plan mode / policy:** middlewares must not register effectful tools;
   reward pass only removes tools, never adds them.
5. **D-007:** imports of `reduction`/`summarization` stay inside
   `internal/runtime` (+ provider), never UI/storage.

**Out of Track A:** manual `/compact` command RPC (needs commands registry);
cross-session memory; changing ADR-009 feed rules for *cross-run* history
(A mutates *in-run* Eino state; cross-run feed stays user/assistant pairs
unless a follow-up ADR says otherwise).

---

### Track B — Session goal object (**Vivy-native**; Eino-assisted only)

**Goal.** One durable **goal** per session (or explicitly one active goal),
with compare-and-set revision and phases `active|paused|blocked|complete`,
model tools + RPC, UI GoalBar verbs. **Not** a rename of `task_*`.

| ID | Work | Eino? | Acceptance |
|---|---|---|---|
| **B1** (`EIN-B1`) | Domain: `Goal`, `GoalRef{id,revision}`, phase, block reason | No | Pure `internal/domain` + tests |
| **B2** (`EIN-B2`) | Store: session-scoped rows + CAS update | No | SQLite migration; first-writer-wins on revision conflict |
| **B3** (`EIN-B3`) | Tools: `goal_create` / `goal_get` / `goal_update` (names bikeshed OK if stable) | No (plain Vivy tools) | Mirror DSH verb set at *product* level; args validated; effectful create/update approval policy TBD (default: update content may be auto if session-local meta; prefer **readonly get**, prompt on create/complete) |
| **B4** (`EIN-B4`) | RPC: `session/goal` get + optional human update | No | UI-GOAL can bind without scraping todos |
| **B5** (`EIN-B5`) | Optional round driver | Thin | After run terminal, if goal active and not complete, expose hint in preamble (MA-2) — **no** hidden auto-run loop without user/turn start |
| **B6** | Relation to `task_*` | plantask already | Docs + UI: tasks are checklist under work; goal is objective; do not dual-write |

**Why not Eino `planexecute` / deep:** those own the whole agent graph and
prompt product. Vivy already owns run lifecycle, approval, and Journal.
Goal is a **projection + tool**, not a second Runner.

**Why not overload plantask:** plantask is file-shaped task CRUD (already
mapped). Goal needs revision CAS, blocked reason codes, and UI lifecycle
independent of todo standing-plan clear-on-turn semantics (DSH todo vs goal
are separate packages for the same reason).

---

### Track C — Session query / FTS (**storage-first**; no Eino)

**Goal.** Read-only search over durable session text for UI trajectory and
optional model tool, without LLM calls (Hermes/DSH lesson).

| ID | Work | Eino? | Acceptance |
|---|---|---|---|
| **C1** (`EIN-C1`) | Spike: modernc sqlite FTS5 availability in our build tags/OS matrix | No | Doc result in verification; go/no-go |
| **C2** (`EIN-C2`) | Migration: FTS virtual table over message/event projections | No | Incremental index on append; rebuild on migrate |
| **C3** (`EIN-C3`) | `session/search` RPC (bounded, redacted) | No | Powers UI-TRAJ / HITL history; empty query rejected |
| **C4** (`EIN-C4`) | Optional tool `session_search` readonly | No | Auto-exec; results marked untrusted; no path escape |
| **C5** (`EIN-C5`) | Trace by run_id / tool_call_id | No | Subset of DSH session-query; enough for inspector |

**Non-goals for C:** semantic embedding RAG (MEM-1); cross-tenant search;
replacing Journal replay.

---

## 4. Suggested delivery batches

| Batch | Tracks | Depends | Notes |
|---|---|---|---|
| **Batch 1** | A1, A3 (reduction-only), A5, A6 (reduction flags), A7 meter hook | none | Zero extra model cost; biggest long-tool win; proves Handler order + events |
| **Batch 2** | A2 summarization, A4 reward pass, full A6 | Batch 1 | Needs scripted dual-model or same model with captured summarize prompts; budget tests |
| **Batch 3** | B1–B4 | none (parallelizable with 1–2) | Unblocks UI-GOAL |
| **Batch 4** | C1 spike → C2–C4 | none (parallel) | Spike may DEFER FTS if modernc story fails → LIKE fallback explicitly worse |
| **Batch 5** | B5–B6 polish, C5, UI smoke | 3–4 | split Vite `3015` |

Each batch: `just ci`; scripted-model tests; no live network in unit tests.
User-visible: Browser smoke on compaction event and goal strip when UI lands.

---

## 5. Config sketch (Batch 1–2)

```yaml
runtime:
  loop:
    reduction:
      enabled: true
      max_length_for_trunc: 50000
      max_tokens_for_clear: 160000
      clear_retention_suffix_limit: 1
      # offload under run workspace; Backend wired in engine
    summarization:
      enabled: false          # soak reduction first
      trigger_tokens: 160000
      emit_internal_events: true
    reward_pass:
      enabled: true
      # fires when MaxToolTurns-1 iterations consumed
    unknown_tools:
      enabled: true
```

Exact field names finalized in implementation; this proposal requires
**parse/validate tests** for every new key (AGENTS rulebook).

---

## 6. Event vocabulary additions (Track A)

| EventType | When | Payload (sketch) |
|---|---|---|
| `context.compacted` | reduction clear and/or summarization finalize | `run_id`, `kind: reduction|summarization`, `before_tokens`, `after_tokens`, `offload_paths[]` (workspace-relative), `message_count_before/after` |
| `loop.summary_pass` | reward pass engaged | `run_id`, `iteration`, `reason` |

Schemas under `schemas/events/`; mapper never drops unknown Eino custom
outputs silently (log + skip or map).

---

## 7. Anti-clone and architecture anchors

- **D-007:** Eino types only in `internal/runtime` + `internal/provider`.
- **D-001/D-005:** No copy of DSH package source; no pi/diva TS port; shape
  only (middleware on the loop, goal as session object, FTS search).
- **NG-7 / NG-11:** Compaction is **kernel pipeline**, not a hot plugin the
  model mounts.
- **NG-15:** No runtime “install reduction plugin”; feature flags in config
  + generation recipe later if tools change.
- **Journal trust root:** Middlewares may offload to **run workspace**;
  durable truth remains Journal + Message store.
- **MEM-1** stays deferred: goal ≠ long-term memory; query ≠ RAG.

---

## 8. Out of scope (explicit)

- OS sandbox (SBX-OS), persistent PTY, ACP implementation, external SDK freeze
- workflow/ralph/graph engine, tool-cordis, community marketplace
- Softening Plan Mode to DSH soft-guidance (keep physical deny)
- Replacing `task_*` with `todo_write`
- Manual slash-command framework (may follow Track B UI)
- Changing approval six-step ordering
- Provider breadth / Anthropic

---

## 9. Risks and mitigations

| Risk | Mitigation |
|---|---|
| Summarization hidden cost / loops | Default off; budget ledger; max summarize attempts via middleware Retry config capped |
| Clear breaks prompt cache / quality | `ClearAtLeastTokens`; retention suffix; scripted eval fixtures |
| FTS5 unavailable on modernc | C1 spike gate; fallback deferred or external `sqlite_fts` build tag documented |
| Goal vs todo user confusion | UI copy + docs B6; separate RPC namespaces |
| Handler order bugs with skill | Single table-driven engine test asserting middleware names/order |
| Compaction vs cross-run ADR-009 | A only touches in-run state; document that session feed still strips tools across runs until separate ADR |

---

## 10. Decision asked of the maintainer

Please choose one:

1. **Approve Batch 1 only** (reduction + events + unknown tools) as next
   implementation entry; hold summarization/goal/query as written follow-ons.
2. **Approve Batches 1–3** (context loop + goal domain/RPC); query spike
   parallel.
3. **Reject / revise** — especially if compaction must wait for a full ADR
   rewrite of cross-run tool history (larger than this proposal).

Until approval, this file is research+design only: **no** engine behavior
change is authorized by its mere presence.

---

## 11. References

| Doc | Role |
|---|---|
| `docs/research/DSH-PLUGIN-ANCHORED-VIVY-GAPS.md` | Plugin-anchored gaps L-COMPACT / L-GOAL / L-QUERY |
| `docs/research/DSH-VS-AGENT-VIVY-CAPABILITY-GAP.md` | G1/G2/G9 |
| `docs/research/AGENT-LOOP-PORT-COMPARISON.md` | E2 reduction+summarization bridge; §7.1 P1–P5 |
| `docs/v1-minimal-agent-proposal.md` | Proposal tone; MA history feed precedent |
| `docs/AGENT-VIVY-ARCHITECTURE-V0.md` | ADR-009/010 defer compaction |
| `docs/eino-capability-verify.md` | Checkpoint/interrupt facts |
| `.workspace/eino/adk/middlewares/{reduction,summarization}` | Implementation contracts |
| `internal/runtime/engine.go` | Current Handlers list |
| `docs/TODO.md` | UI-GOAL, UI-TRAJ, SBX-OS (untouched by this proposal’s scope) |
