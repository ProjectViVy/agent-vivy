# DeepSeek Harness and agent-vivy Capability Comparison and Gap Analysis

> Status: **research report** (an analysis document, not a product contract; it does not add or rewrite decisions).
> Date: 2026-08-16
> Purpose: compare the actual capabilities of DeepSeek Harness (DSH) and agent-vivy (species `vivy.exe`) dimension by dimension,
> and mark the direction, nature, and source of each gap for citation in capability proposals and architecture discussions.
> Evidence sources: direct reading of source code and documentation on both sides (see the evidence index in Appendix B); DSH is taken from
> `.workspace/deepseek-harness/deepseek-harness` (`HEAD` `47f9438`, `0.1.0-rc.5`).
> Related: `AGENT-VIVY-DIRECTION.md`, `SELF-EVOLVING-GATEWAY.md`,
> `VIVY-GATEWAY-AND-STUDIO.md`, `VIVY-STUDIO.md`, `prd-agent-vivy-v0.md`.

---

## 0. Summary (TL;DR)

DSH and agent-vivy are not the same kind of product. The capability gap is first an **identity gap**, and only secondarily a
catalog gap:

- **DSH** is a developer-preview **TypeScript / Cordis plugin OS**: everything is a plugin,
  even the agent loop itself can be unloaded and replaced; profiles/bundles/overlays are layered at startup;
  its model tool surface has 30+ tools; it has fail-closed three-platform process sandboxes, context compaction,
  spill, SQLite FTS session search, more than six subagent transports (spawn/fork/ACP/dsh-sdk/
  Codex/Claude Code), model-authored workflows, self-mounted plugins (`tool-cordis`), and more. It makes
  "composability" a product.
- **agent-vivy** is a **Go single-EXE personal gateway**: Journal is a first-class citizen; it has a six-state
  run state machine, exactly-once terminal state, first-writer-wins for approvals and Ask User, restart
  recovery, environment-variable keys, compile-time plugins (`vivy-sdk pack`), and an independent Studio for managing
  development and distribution. It makes "replayability, auditability, recoverability, and the human gate" a product.

Both sides share the same engineering disciplines: **event-sourced local session history, model-visible ≡ recorded,
capability seams, policy/hooks/Plan Mode attached to the tool pipeline, a JSON-RPC control plane, and a thin UI**.
The gap is concentrated in capabilities DSH has that Vivy intentionally or temporarily does not: **context compaction and search,
OS-level sandboxing, hot/dynamic plugin loading, subagent/workflow orchestration, browser automation, LSP,
multiple interfaces (ACP/CLI/Python SDK/Codex/Claude Code hooks), and telemetry**. Conversely,
agent-vivy is stronger than DSH out of the box—or DSH has no equivalent at all—in **terminal-state uniqueness,
human-gated approvals, restart recovery, generational ledgers, and air-gapped evaluation**.

Conclusion: most gaps are **intentional gaps** (the philosophical anchor and the kernel are never pluginized), while a few are
**gaps to fill later** (compaction, subagent orchestration, sandboxing, and search—all candidates for V1+ capability proposals).
This document does not argue that Vivy should chase DSH's capability surface; it argues that the gaps should be handled in three
separate categories: "should learn / should reject / can be added later."

---

## 1. Positioning Comparison: Two Products, Two Identities

| Dimension | DeepSeek Harness | agent-vivy (species) |
|---|---|---|
| Product identity | Open-source agent harness, plugin framework + reference implementation (`README.md:5-7`) | Personal gateway application (`prd-agent-vivy-v0.md:76-86`) |
| In one sentence | "everything is a plugin" (`docs/architecture.md:9-13`) | "the only body for daily life; the only writer to Journal" (`VIVY-STUDIO.md:352-356`) |
| Language/runtime | TypeScript, pnpm monorepo, Node ≥22 (`package.json`) | Go 1.26 single module, single binary + `go:embed` UI (`go.mod:1`, `ui/embed.go:17-18`) |
| Host framework | In-house (vendored Cordis, paper “Spatiotemporal Composability”) | Third-party Eino v0.9.13, isolated in `internal/runtime` + `internal/provider` (D-007) |
| Kernel status | No privileged kernel; the loop is also an unloadable plugin (`docs/architecture.md:9-13`) | The kernel is never pluginized (Journal/policy/keys/identity, `SELF-EVOLVING-GATEWAY.md:159-169`) |
| Plugin model | Hot-attach/hot-unload at runtime; the model can author plugins (`packages/extensions/tool-cordis`) | Compile time: source → `vivy-sdk pack` → new EXE (NG-11, `SELF-EVOLVING-GATEWAY.md:226-249`) |
| Development status | Developer preview `0.1.0-rc.5`, breaking changes allowed (`README.md:9-11`) | V0/V1 assembly complete; Studio bootstrap complete (`README.md:12-15`) |
| Relationship to the other | The first development engine for Vivy Studio (`VIVY-STUDIO.md:183-191`) | DSH's resident product; does not embed Node or depend on DSH (NG-5) |

DSH is "a compositional framework that can assemble an agent"; agent-vivy is "an agent that can get through the night."
They do not occupy the same competitive position, but Vivy's architectural direction (seams, log invariants, and dual event surfaces)
explicitly uses DSH as a benchmark (`SELF-EVOLVING-GATEWAY.md:87-112`).

---

## 2. Size and Form Overview

| Dimension | DSH | agent-vivy |
|---|---|---|
| Package/module count | ~50 npm packages + 3 bundles + 6 examples + apps/cli + apps/web + python/sdk + native/landlock-run (`packages/README.md`) | 1 Go module; 13 packages under `internal/` + `cmd/vivy` + `cmd/vivy-studio` + `sdk/` + `ui/` (`README.md:39-58`) |
| Order of magnitude of lines | Hundreds of thousands of lines of TS (reference: 64k TS/TSX across projects in the full `.workspace`) | Medium-sized Go monolith (about 60+ source files, including tests) |
| Configuration model | Four-layer composition: cordis.yml + profile + bundle + overlay (`docs/architecture.md:15-37`) | Single `config.yaml`, strict decoding (`internal/config/config.go:1-13`) |
| Event vocabulary | Merge-extensible `SessionEventMap`, one JSON Schema per event (`docs/persistence-catalog.md`) | 34 `RunEvent` types + a payload schema for each type (`internal/domain/event.go:8-43`) |
| Persistence | Event log + dual JSONL(zstd)/SQLite backends (`docs/subsystems/persistence.md:231-237`) | Single Journal + SQLite backend (pure-Go modernc) (D-031) |
| Interfaces | Web GUI (:3080) + headless CLI + ACP + JSON-RPC SDK (TS/Python) (`apps/cli/README.md`) | Built-in Web UI (:8787) + JSON-RPC control plane (`internal/rpc`) |
| Testing culture | Per-file 100% coverage gate, keyless snapshots, browser snapshot CI (`docs/testing.md:9-49`) | `just ci` + CN-01..16 conformance suite + Playwright real-process smoke (`justfile:22-25`) |

---

## 3. Capability Matrix (Dimension-by-Dimension Comparison)

### 3.1 Architecture and Extension Model

| Capability | DSH | agent-vivy | Gap direction |
|---|---|---|---|
| Composition unit | Cordis plugin (Service + inject + reversible effect, `docs/cordis-primer.md:7-44`) | Kernel (not composable) + built-in units (loop/world/provider/tool) + user plugins (`VIVY-ASSEMBLY.md:42-66`) | Different paradigms: runtime tree vs compile-time generation |
| Capability seam | Service Definition / Provider / Consumer triplet, ~55 `ctx` keys (`docs/capability-seams.md:412-469`) | Named seams such as `provider` / `tool-world` / `loop` (`VIVY-ASSEMBLY.md:47-66`) | Vivy has the seam concept, but with coarse granularity and no runtime rebinding |
| Assembly | bundle → profile → home → `--patch` overlay; inspectable with `--dump-config` (`docs/architecture.md:15-37`) | `vivy.generation.yml` recipe + compile-time overlay via `vivy-sdk pack` (`VIVY-ASSEMBLY.md:74-113`) | Equivalent concept, different execution point |
| Swappable loop | agent-loop is an unloadable plugin (`docs/architecture.md:48-52`) | loop is a recipe-swappable assembly unit, privileged within the species (`VIVY-ASSEMBLY.md:146`) | Intentional gap (kernel not pluginized) |
| Self-modification | `tool-cordis`: the model defines/mounts/unmounts its own plugins (VM sandbox, opt-in, `packages/extensions/tool-cordis`) | **Rejected**: no runtime loading; `execute` pointing at source is an "ungated self-rewrite" to be sealed (S8, `SELF-EVOLVING-GATEWAY.md:307-309`) | Intentional gap (NG-11; DSH itself also states this is not a security boundary) |

### 3.2 Sessions, Events, and Persistence

| Capability | DSH | agent-vivy | Gap direction |
|---|---|---|---|
| Session history model | Append-only `SessionEvent` log; model history is **derived** (`docs/subsystems/session.md:5`) | Journal + Message projection; history rebuilt into the Eino feed (ADR-010) | Isomorphic |
| Model-visible ≡ recorded | Runtime invariant + request-header snapshot `request/header` (`docs/architecture.md:92-96`) | `model.request` summary event + message projection (ADR-010) | Isomorphic; DSH is stronger (the full request can be replayed) |
| Unique terminal state | No such concept (a turn can be interrupted/retried/cancelled, `docs/agent-lifecycle.md`) | **Exactly-once terminal state**, with a double safeguard at the Journal layer (`internal/storage/sqlite/journal.go:38-46`) | **Vivy is stronger** (DSH has no equivalent) |
| Restart recovery | Synthesizes `turn/end {interrupted}` after a crash (`docs/subsystems/persistence.md:13-17`) | Before startup, `Service.Recover` settles every non-terminal run; approvals/questions can resume (`internal/app/app.go:273-276`) | Isomorphic; Vivy makes "recovery" a startup gate |
| Context compaction | Compaction seam: summary + surface replacement + tool-result pruner (`docs/subsystems/compaction.md`) | **None** (explicitly deferred, ADR-009/010) | **DSH has it; Vivy does not** (V1+ proposal candidate) |
| Oversized output | Spill-to-file + opaque locator (`docs/subsystems/spill.md`) | Bounded head/tail + `[UNTRUSTED TOOL OUTPUT]` folding (`internal/runtime/tooladapter.go:182-202`) | Isomorphic, different mechanism |
| Session search | SQLite FTS5 full-text search + tracing (`packages/session-query/session-query-sqlite`) | None (FTS is proposal-only, `docs/research/hermes-tool-porting-research`) | **DSH has it; Vivy does not** |
| Persistence backend | Swappable JSONL + SQLite (`docs/subsystems/persistence.md:231-237`) | SQLite-only backend (fsjournal V1+ probe, D-031) | Small gap; Vivy is protected by a conformance suite |

### 3.3 Model / Provider Layer

| Capability | DSH | agent-vivy | Gap direction |
|---|---|---|---|
| Adapter seam | `ctx.llm`: register any adapter, parse once per request (`docs/subsystems/llm-streaming.md:627-702`) | `ProviderRef` + pre-baked YAML bundle (`internal/provider/bundle.go`) | Isomorphic; DSH allows open registration, Vivy uses a curated catalog |
| Available providers | Official DeepSeek + two pi-ai library adapters; any provider can be added (`packages/llm/llm-deepseek`) | OpenAI-compatible (wired) + **Anthropic not connected** + mock (`internal/provider/catalog.go:37`) | **DSH has it; Vivy does not** (provider breadth is proposal-driven) |
| Streaming protocol | Unified closed union of `StreamChunk` + BlockAssembler (`docs/subsystems/llm-streaming.md:154-182`) | Domain streaming interface + Eino mapping (`internal/runtime/mapper.go`) | Isomorphic |
| Keys | Credential ref (the value is never persisted), resolved per operation (`docs/subsystems/credentials.md`) | `env_key` stores only the name and reads it at runtime (D-010, `internal/config/config.go:29-31`) | Isomorphic; DSH supports files/multiple sources, Vivy only env |

### 3.4 Tool Catalog

| Capability | DSH | agent-vivy | Gap direction |
|---|---|---|---|
| Filesystem | read/write/edit/read_image + str_replace_editor + glob/grep (`docs/tool-catalog.md:16-28`) | read_file/search_files/write_file/patch (`config.example.yaml:74-76`) | Comparable; DSH additionally has read_image/str_replace |
| Shell | bash / pwsh / persistent PTY bash + background jobs (`docs/tool-catalog.md:21-28`) | execute/commandline (allowlist, workspace-scoped, no shell syntax, `internal/runtime/command_backend.go`) | **DSH is stronger**: persistent terminals, background jobs, and both shells |
| Web | web_search/web_fetch (multi-provider seam, `docs/subsystems/web.md`) | network_search (5 providers, read-only); http_request (GET/HEAD allowlist, `internal/tools/http_request.go:39-48`) | Comparable; Vivy is more conservative (read-only) |
| MCP | MCP client bridge, with tools scoped by server (`packages/mcp/mcp-client/README.md`) | mcp_list_tools/mcp_call (approval gate, `internal/tools/mcp.go`) | Comparable; Vivy treats it as a "configuration dependency," not a plugin (NG-19) |
| Terminal/PTY | terminal_open/list/read/send/signal/close (`docs/tool-catalog.md:28`) | **None** | **DSH has it; Vivy does not** |
| LSP | lsp (goToDefinition/references/impl/hover, `docs/subsystems/lsp.md`) | **Not in the species** (only in the Studio toolchain, `VIVY-STUDIO.md:187`) | **DSH has it; Vivy does not** (on the species side) |
| Tasks/todos | todo_write (`packages/todo/tool-todo`) | task_create/get/update/list (`internal/tools/todo.go`) | Comparable |
| Planning | Plan mode soft guidance + exit_plan_mode (`docs/subsystems/plan.md:5-35`) | Plan Mode **physically rejects** effectful tools (H3, `GOAL-AGENT-HARNESS-ROADMAP.md:71-74`) | Isomorphic but different strength: Vivy is harder |
| Ask user | ask_user_question (pauses tool call, `docs/tool-catalog.md:18`) | ask_user (independent QuestionStore, answer = data, `internal/tools/askuser.go`) | Isomorphic |
| Code execution | run_code (model writes a TS program, Code Mode, `docs/tool-catalog.md:19`) | **None** (sequential_thinking is reasoning assistance) | **DSH has it; Vivy does not** |
| Skills | skill tool + layered providers + catalog discovery (`docs/subsystems/skills.md`) | skills_list/view/manage (HITL revision, `internal/tools/skills.go`) | Comparable; Vivy management is stricter (HITL) |
| Self-inspection | cordis_inspect_* (runtime introspection, `docs/tool-catalog.md:23`) | species/inspect (read-only identity, `internal/studio/inspect.go`) | Isomorphic; different scope (DSH introspects its own plugin tree) |

### 3.5 Security and Sandboxing

| Capability | DSH | agent-vivy | Gap direction |
|---|---|---|---|
| Process sandbox | Fail-closed: Linux bwrap/Landlock (native C), macOS Seatbelt, Windows ACL restricted-token (`docs/subsystems/sandbox.md`; `native/landlock-run`) | **No OS-level sandbox**; only per-run directory isolation + fail-closed traversal/symlink handling (`internal/runtime/isolation.go`) | **DSH has it; Vivy does not** (explicitly a V0 non-goal, `prd-agent-vivy-v0.md:66`) |
| Sandbox modes | read-only / workspace-write / danger-full-access (`docs/subsystems/sandbox.md:11-21`) | Policy profiles: default/plan/read_only/full_auto (`internal/domain/policy.go:6-11`) | Isomorphic concept |
| Approvals | allowed-once/rejected/cancelled/unavailable, fail-closed (`docs/subsystems/approval.md:21-29`) | Six-step write ordering + first-writer-wins + expiry + auditable proposal (`internal/runtime/service.go:1035-1142`) | Isomorphic; Vivy is more complete (proposal, expiry, Review Center) |
| Permission presets | Permission preset bundles sandbox + approval (`docs/subsystems/permission-presets.md:11-27`) | Policy profile is semantically equivalent (`internal/runtime/policy.go:47-53`) | Isomorphic |
| Argument safety | ToolDefinition output schema + validation (`docs/subsystems/tools.md:9-23`) | ValidateArgs + dangerous-argument gate in security.go (`internal/tools/security.go:41-71`) | Isomorphic |
| Output sanitization | Spill locator + credential scrub (`docs/defensive-patterns.md:27-33`) | Key/email redaction + untrusted marker + bounded folding (`internal/runtime/tooladapter.go:182-202`) | Isomorphic |

### 3.6 Approvals, Interaction, and Human–Machine Collaboration

| Capability | DSH | agent-vivy | Gap direction |
|---|---|---|---|
| Approval audit | Paired approval/asked + decided events (`docs/subsystems/approval.md:11-19`) | 6 approval event types + replayability (`internal/domain/event.go:18-22`) | Isomorphic |
| Cross-session queue | None (approval is in-session) | **Review Center**: cross-session review/list/get/respond (`docs/architecture/hitl-review-center.md`) | **Vivy is stronger** |
| Ask User separated from approval | user-questions and approval are separate (`docs/subsystems/user-questions.md:5-27`) | question is independent of approval (`internal/tools/askuser.go:12-13`) | Isomorphic |

### 3.7 Context Management

| Capability | DSH | agent-vivy | Gap direction |
|---|---|---|---|
| Context budget | token-meter + pressure events (`docs/subsystems/token-meter.md`) | max_context_bytes / max_history_messages (`internal/config/config.go:109-111`) | Vivy has only hard limits, with no pressure signal |
| Compaction | Automatic compaction-basic + `/compact` (`docs/subsystems/compaction.md`) | None | **DSH has it; Vivy does not** |
| Context injection | Queued injection through agent.inject() (`docs/architecture.md:120`) | Pre-prompt assembly (MA-2, `internal/runtime/prompt.go`) | Isomorphic |
| Cross-session references | session-reference + session-query (`docs/subsystems/session-reference.md`) | None | **DSH has it; Vivy does not** |

### 3.8 Orchestration (Subagents / Workflow / Goal / Jobs / Schedule)

| Capability | DSH | agent-vivy | Gap direction |
|---|---|---|---|
| Subagents | 6+ transport types (in-process spawn/fork, ACP, dsh-sdk, Codex, Claude Code) + continuable child sessions + control tools (`docs/subsystems/subagent.md`) | `vivy worker` child runs in the same binary: the parent governs model/tools/approvals/policy/budget (`internal/worker/supervisor.go`; GOAL-3..6) | Isomorphic (parent governs child); DSH has more forms |
| Workflows | Workflow scripts + worker-thread engine + ralph (`docs/subsystems/workflow.md`) | **No graph workflow** (the child tree is the only orchestration, `prd-agent-vivy-v0.md:65`) | **DSH has it; Vivy does not** |
| Goals | Same-session goal (revisioned phase + turn limit, `docs/subsystems/goal.md`) | None | **DSH has it; Vivy does not** |
| Background tasks | Generic jobs registry + job_* controls (`docs/subsystems/jobs.md`) | Background runs: list/attach/logs/recover/cancel (H8) | Isomorphic |
| Scheduling | schedule (session-local reminders, `docs/subsystems/schedule.md`) | None (scheduler deferred, `IMPLEMENTATION-PLAN.md:464`) | **DSH has it; Vivy does not** |
| Budget | No explicit budget ledger (has timeout/guard) | **Budget ledger**: events/model_calls/tool_calls/retries, shared by parent and child (`internal/runtime/budget.go`) | **Vivy is stronger** |

### 3.9 Interfaces

| Capability | DSH | agent-vivy | Gap direction |
|---|---|---|---|
| CLI | `dsh --profile` / headless one-shot tasks (`apps/cli/README.md:7-16`) | `vivy.exe` has no subcommand surface; `vivy-sdk` / `vivy-studio` are independent binaries (ADR-017) | Different positioning |
| Web GUI | Full product UI: settings/models/workspace/plugin catalog (`docs/user/guide/index.md:5-23`) | Thin UI shell: sessions/streaming/approvals/Review Center (`ui/src/features/*`) | Large gap (feature surface) |
| Automation protocol | ACP server (fresh sessions, `packages/acp/acp/README.md:5-81`) | ACP **proposal only** (`docs/architecture/ACP-REMOTE-CONTROL-PROPOSAL.md:3`) | **DSH has it; Vivy does not** |
| SDK | JSON-RPC SDK (TS + Python, `packages/sdk`; `python/sdk`) | JSON-RPC control plane (`internal/rpc`), no public SDK | Medium gap |
| External agent bridge | Codex / Claude Code hooks bridge (`packages/hooks`) | None (hook semantics are built into the policy/hook chain) | **DSH has it; Vivy does not** (Vivy has no use for it) |
| RPC types | Typert type graph generation (`docs/api-gateway.md`) | Handwritten JSON-RPC methods (`internal/rpc/control.go`) | Medium gap (engineering) |

### 3.10 Observability and DX

| Capability | DSH | agent-vivy | Gap direction |
|---|---|---|---|
| Telemetry | session-telemetry + OTel, off by default (`docs/subsystems/session-telemetry.md`) | All events land in Journal; no OTel export | Medium gap |
| Feedback | message-feedback + /feedback (`docs/subsystems/feedback.md`) | No user feedback surface | **DSH has it; Vivy does not** |
| Session title | Automatic titles (`docs/subsystems/session-title.md`) | Session title field (`internal/domain/session.go`) | Vivy has no automatic generation |
| Projection | session-projection folding units (`docs/subsystems/session-projection.md`) | ReviewItem projection (`internal/domain/review.go`) | Isomorphic, different scope |
| Runtime invariants | ctx.invariants registered by each package (`docs/subsystems/invariants.md`) | ADR guard tests + conformance | Isomorphic |

### 3.11 Maturity and Engineering Discipline

| Dimension | DSH | agent-vivy | Gap direction |
|---|---|---|---|
| Release stage | Developer preview, breaking changes allowed (`README.md:9-11`) | V0/V1 assembly complete, Studio switch complete (ST-6) | Both are early; Vivy makes a stronger stability commitment |
| Coverage | Per-file 100% gate (`docs/testing.md:10`) | go test -race + CN suite + Playwright | Both are strong, different forms |
| Snapshot testing | Keyless session snapshots + browser snapshot CI (`docs/testing.md:12-13`) | Playwright mock-session smoke (`ui/e2e/smoke.spec.ts`) | DSH is more systematic |
| Generated documentation | Generated tool/config/persistence catalogs + freshness gate (`docs/graph-atlas.md`) | Handwritten schema + README | DSH is stronger |
| Incident culture | postmortem + agent notes (`docs/postmortem/`) | Acceptance logs + ADRs (`docs/logs/`) | Isomorphic |

---

## 4. Gap Catalog (by Gap Type)

### 4.1 DSH Has It, agent-vivy Does Not (and There Is Currently No Implementation)

| # | Capability | Corresponding Vivy status | Evidence |
|---|---|---|---|
| G1 | Context compaction | None; raw but bounded feed | `AGENT-VIVY-ARCHITECTURE-V0.md:144-146` |
| G2 | Full-text session search (FTS5) | None | `hermes-tool-porting-research` (proposal only) |
| G3 | OS-level process sandbox | None (only directory isolation + policy) | `prd-agent-vivy-v0.md:66` |
| G4 | Persistent PTY / terminal tools | None | `config.example.yaml` (no terminal in the tool catalog) |
| G5 | LSP semantic navigation (on the species side) | None (only in the Studio toolchain) | `VIVY-STUDIO.md:187` |
| G6 | Code execution (run_code / Code Mode) | None | `docs/tool-catalog.md:19` |
| G7 | Multiple subagent forms + continuable child sessions | Same-binary worker child only | `GOAL-AGENT-HARNESS-ROADMAP.md:124-145` |
| G8 | Model-authored workflow / ralph | None | `docs/subsystems/workflow.md` |
| G9 | Same-session goal management | None | `docs/subsystems/goal.md` |
| G10 | Scheduled reminders (schedule) | None | `IMPLEMENTATION-PLAN.md:464` |
| G11 | ACP automation protocol | Proposal only | `ACP-REMOTE-CONTROL-PROPOSAL.md:3` |
| G12 | Public SDK (TS/Python) | None | `packages/sdk`, `python/sdk` |
| G13 | Codex / Claude Code hooks bridge | None (not applicable) | `packages/hooks` |
| G14 | OTel telemetry export | None | `docs/subsystems/session-telemetry.md` |
| G15 | User feedback surface | None | `docs/subsystems/feedback.md` |
| G16 | Automatic session titles | None | `docs/subsystems/session-title.md` |
| G17 | Cross-session reference/search injection | None | `docs/subsystems/session-reference.md` |
| G18 | Provider breadth (>2) | Only openai-compatible is available | `internal/provider/catalog.go:37` |
| G19 | Multiple persistence backends | SQLite only | `prd-agent-vivy-v0.md:450` |
| G20 | Full request replay (request/header snapshot) | Summary events only | `docs/subsystems/session.md:152-176` |

### 4.2 DSH Has It, and agent-vivy Has It in a Different Form

| Capability | DSH form | Vivy form | Description |
|---|---|---|---|
| Event sourcing | Session log + derived history | Journal + message projection | Isomorphic; Vivy has unique terminal state |
| Approvals | One-time allowed-once | proposal + expiry + first-writer-wins + Review Center | Vivy is more complete |
| Sandbox modes | 3 file-effect modes | 4 policy profiles | Conceptually equivalent |
| Plugins | Hot-attached at runtime | Compile-time pack | Intentional gap |
| Background tasks | Jobs registry | Background run management | Isomorphic |
| Budget | timeout/guard | Explicit ledger (stronger) | Vivy is stronger |
| Tool safety | schema + scrub | schema + security gate + redaction | Isomorphic |

### 4.3 DSH Has It, and agent-vivy Intentionally Rejects It (Philosophy, Not Debt)

| Capability | DSH | Why Vivy rejects it | Decision |
|---|---|---|---|
| Everything is a plugin (including loop/log) | Core selling point | The environment cannot be a population member | NG-7 |
| Runtime self-modification | tool-cordis (opt-in) | Production instances must not perform ungated self-rewrite; this is not a security boundary | NG-11; `SELF-EVOLVING-GATEWAY.md:307` |
| Community plugin marketplace | dsh-plugin discovery | Curated-catalog anchor | PRD §5.0.3 |
| Multi-tenancy/hosting | None (but extensible) | Personal-gateway anchor | D-016 |
| Microservices/mesh | None (but extensible) | The local machine is not a cluster | NG-8 |
| Node in the hot path | It is Node itself | Single-EXE Windows one-click experience | NG-2 |

### 4.4 agent-vivy Has It, DSH Does Not or Is Weaker

| Capability | Vivy | Current DSH status | Description |
|---|---|---|---|
| Exactly-once terminal state | Double safeguard at the Journal layer | No such concept | DSH allows turns to be interrupted/retried |
| Restart recovery gate | Settles every run before startup | Crash-log repair (does not settle runs) | Vivy is more productized |
| Approval integrity | proposal + expiry + cross-session queue | One-time + ask/never | Vivy is stronger |
| Budget ledger | Shared by parent and child, cannot be widened | Only timeout/guard | Vivy is stronger |
| Generational ledger | Studio Generation/EvalRun/Release/Install | No cross-generation fitness | `VIVY-STUDIO.md:196-240` |
| Air-gapped evaluation | Studio launches candidate with an independent data directory | None | NG-24 |
| Key discipline | env-only, audit tests | Multiple sources but no OS keychain (also deferred) | Equally strong |
| Compile-time plugin provenance | Hash + generation.json | None | `sdk/internal/pack.go:179-197` |

---

## 5. Structural Gap: Plugin OS vs Personal Gateway

This is not a catalog difference, but a fork between two architectural philosophies:

1. **Where composability lives**. DSH puts "dynamic composition" at runtime (Cordis reversible effects +
   reactive coeffects); Vivy puts "composition" at compile time (packing a generation into an EXE).
   The former enables hot-unloadable plugins and model self-evolution; the latter ensures Journal/policy/keys can never
   be unloaded and that a failed mutant cannot dismantle daily life (NG-3).
2. **What the trust root is**. DSH's trust root is "composition correctness" (no privileged kernel); Vivy's
   trust root is "human gate + replayability" (the kernel is never pluginized, `SELF-EVOLVING-GATEWAY.md:159-169`).
3. **The evolution path**. DSH evolves by "attaching parts to a live process" (gone on restart; officially declared not a
   security boundary, `SELF-EVOLVING-GATEWAY.md:94`); Vivy evolves through "new EXE + Studio evaluation + human release"
   (NG-15, NG-25).
4. **The overnight promise**. DSH is a developer preview and allows breaking changes; Vivy's philosophical anchor requires
   the resident product to be usable, auditable, and recoverable today (`prd-agent-vivy-v0.md:76-99`).

Therefore, the correct reading of the gap analysis is: **DSH owns most capabilities in G1..G20 in a "plugin/composition"
form; if Vivy introduces them, it should rebuild them as "first-class citizens within the species," rather than importing DSH's
capability surface** (NG-7: learn the discipline, reject the identity). `SELF-EVOLVING-GATEWAY.md:118-136`
provides a quantitative assessment: by pillar, the species implements only 35–45% of DSH's core ideas; after adding the Studio
engine, the "effect on humans" is about 70–80%, while "overnight-accumulating evolution" can exceed DSH out of the box.

---

## 6. Conclusions and Recommendations

### 6.1 Handling the Three Gap Categories

| Category | Items | Recommendation |
|---|---|---|
| Should learn (discipline) | Model-visible ≡ recorded (ADR-010 completed); seam naming; dual event surface; guard/invariants | Adopted; continue evolving according to DSH discipline (NG-7) |
| Can be added later (capability proposal candidates) | G1 compaction, G2 search, G3 sandbox, G7 subagent forms, G12 SDK, G14 telemetry | Initiate each item through the capability proposal process (`AGENT-VIVY-DIRECTION.md`); prioritize G1/G3/G7 for V1+ |
| Intentionally rejected | Hot plugins, self-modification, plugin marketplace, Node in the hot path, multi-tenancy | Maintain NG-2/NG-7/NG-11/NG-15 unless separately decided |

### 6.2 Positioning Conclusions for Vivy

- DSH is the reference ceiling for a **compositional framework**, not a competitor for a **personal gateway**; Studio can use it as
  an engine, while the species does not need DSH's capability surface.
- Vivy's **real advantages** relative to DSH (unique terminal state, human-gated approvals, restart recovery, budget ledger,
  and generational air-gapped evaluation) should continue to strengthen its species identity rather than filling out DSH's catalog.
- The largest gaps (context management G1/G2, orchestration G7/G8, and sandbox G3) are precisely candidate proposals for the V1
  Operate stage's "make daily life easier"; they should be proposed as "first-class citizens within the species," each following the
  evidence → proposal → contract → implementation → real-path validation process (`GOAL-AGENT-HARNESS-ROADMAP.md:50-56`).

---

## Appendix A: Tool Catalog Comparison (Model-Visible Surface)

### DSH (30+, generated catalog in `docs/tool-catalog.md`)

ask_user_question · run_code · exit_plan_mode · bash · pwsh · bash(persistent) ·
cordis_define/run/stop/undefine/inspect_* · str_replace_editor · read/write/
edit/read_image · glob/grep · terminal_open/list/read/send/signal/close ·
create_goal/get_goal/update_goal · schedule_create/list/delete · lsp ·
workflow · ralph · skill · session_event_read/search/trace · session_search/
trace · subagent/subagent_fork · interrupt_agent/list_agents/send_message ·
report · job_kill/list/output · todo_write · web_search/web_fetch ·
mcp__<server>__<raw>

### agent-vivy (24, `config.example.yaml:66-90`)

echo_info · write_note · list_notes · read_note · ask_user · read_file ·
search_files · write_file · patch · http_request · mcp_list_tools · mcp_call ·
sequential_thinking · execute · commandline · skills_list · skill_view ·
skill_manage · task_create · task_get · task_update · task_list ·
network_search · tool_search

---

## Appendix B: Evidence Index

### DSH (relative to `.workspace/deepseek-harness/deepseek-harness`)

- Positioning: `README.md:5-11`; preview statement `README.md:9-11`
- Architecture: `docs/architecture.md:9-37,39-52,63-96,98-104`
- Cordis: `docs/cordis-primer.md:7-44`; vendor `vendor/README.md`
- Seam table: `docs/capability-seams.md:412-469`
- Sessions/persistence: `docs/subsystems/session.md`, `persistence.md`,
  `persistence-catalog.md`
- Compaction/spill/search: `docs/subsystems/compaction.md`, `spill.md`, `session-query.md`
- LLM: `docs/subsystems/llm-streaming.md`; `packages/llm/llm-deepseek/README.md`
- Tools: `docs/tool-catalog.md` (generated catalog)
- Sandboxing/approvals: `docs/subsystems/sandbox.md`, `approval.md`, `permission-presets.md`;
  `native/landlock-run/README.md`
- Orchestration: `docs/subsystems/subagent.md`, `workflow.md`, `goal.md`, `jobs.md`, `schedule.md`
- Interfaces: `apps/cli/README.md`, `packages/acp/acp/README.md`, `packages/sdk/README.md`,
  `python/README.md`, `packages/hooks/README.md`, `docs/api-gateway.md`
- Engineering: `docs/testing.md`, `.github/workflows/ci.yml`, `BENCHMARK.md`
- Explicit POC/partial: `packages/e2b/README.md`, `packages/code-runtime/code-runtime/README.md`

### agent-vivy (relative to repository root)

- Positioning/philosophy: `prd-agent-vivy-v0.md:76-141`; `AGENT-VIVY-DIRECTION.md`
- Architecture/ADRs: `docs/AGENT-VIVY-ARCHITECTURE-V0.md` (ADR-001..018)
- Worldview/Studio: `docs/architecture/VIVY-WORLDVIEW.md`,
  `VIVY-STUDIO.md`, `SELF-EVOLVING-GATEWAY.md`, `VIVY-GATEWAY-AND-STUDIO.md`
- Assembly/plugins: `VIVY-ASSEMBLY.md`, `VIVY-PLUGIN-SPEC.md`, `sdk/plugin/plugin.go`,
  `sdk/internal/pack.go`
- Runtime: `internal/runtime/service.go`, `engine.go`, `tooladapter.go`,
  `budget.go`, `policy.go`, `preflight.go`, `checkpoint.go`
- Domain/events: `internal/domain/event.go`, `run.go`, `policy.go`;
  `schemas/events/run-event.schema.json`
- Storage: `internal/storage/contracts.go`, `internal/storage/sqlite/*`
- Tools: `internal/tools/*.go`, `config.example.yaml`
- Control plane: `internal/rpc/protocol.go`, `control.go`, `websocket.go`
- Worker: `internal/worker/supervisor.go`, `server.go`; `internal/app/worker.go`
- Studio: `cmd/vivy-studio/main.go`, `internal/studiocore/service.go`,
  `internal/eval/*`
- Roadmap/catalog: `docs/GOAL-AGENT-HARNESS-ROADMAP.md`, `docs/IMPLEMENTATION-PLAN.md`,
  `docs/TODO.md`
