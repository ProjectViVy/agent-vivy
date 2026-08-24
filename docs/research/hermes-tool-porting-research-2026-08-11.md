---
stepsCompleted: [1, 2, 3, 4, 5, 6]
inputDocuments:
  - "../../../../.workspace/hermes-agent/pyproject.toml"
  - "../../../../.workspace/hermes-agent/toolsets.py"
  - "../../../../.workspace/hermes-agent/tools/registry.py"
  - "../../../../.workspace/hermes-agent/tools/file_tools.py"
  - "../../../../.workspace/hermes-agent/tools/session_search_tool.py"
  - "../../../../.workspace/hermes-agent/tools/skills_tool.py"
  - "../../../../.workspace/hermes-agent/tools/todo_tool.py"
  - "../../../../.workspace/hermes-agent/tools/approval.py"
  - "../../../../.workspace/hermes-agent/tools/checkpoint_manager.py"
  - "../../../../.workspace/hermes-agent/tools/terminal_tool.py"
  - "../../../../.workspace/hermes-agent/tools/process_registry.py"
  - "../../internal/tools/tools.go"
  - "../../internal/runtime/tooladapter.go"
  - "../../internal/runtime/isolation.go"
  - "../../internal/storage/sqlite/sqlite.go"
  - "../../internal/storage/sqlite/messages.go"
research_type: "technical"
research_topic: "Hermes Agent tool portability into Vivy"
research_goals: "Identify Hermes tools that can be ported into the Go/Vivy runtime without browser-class external dependencies, fully port Hermes file editing and Skills support, and design HUMAN IN THE LOOP approval for every effectful mutation."
user_name: "mastwet"
date: "2026-08-11"
web_research_enabled: true
source_verification: true
---

# Hermes Agent Tool Portability into Vivy

**Date:** 2026-08-11  
**Author:** mastwet  
**Research Type:** Technical feasibility and implementation planning

## Research Overview

This research compares the local Hermes Agent checkout at `../../.workspace/hermes-agent` (relative to the `agent-vivy` repository root) with the current Go/Vivy runtime. The objective is selective portability, not a wholesale reimplementation: browser automation, remote sandboxes, media generation, messaging adapters, and provider-specific integrations remain out of scope, while the reviewed MCP/HTTP/search/process contracts are now included behind Vivy-owned policy boundaries.

The analysis combines local source inspection with current Hermes documentation. The revised scope treats the complete local file toolset (`read_file`, `write_file`, `patch`, and `search_files`) and complete Skills support (`skills_list`, `skill_view`, and `skill_manage`) as required portability targets. HUMAN IN THE LOOP is a cross-cutting requirement: read-only tools may run automatically, while every durable or external mutation must pause before execution and expose a reviewable request/diff to a human.

## Executive Summary

Hermes currently exposes roughly 70+ registered tools grouped into toolsets, including file, terminal, skills, todo, memory, session search, cronjob, code execution, delegation, browser, and provider integrations. The official tool reference confirms that `session_search` is SQLite/FTS5-backed and makes no LLM calls, while the file and terminal toolsets are local execution primitives with separate safety and environment concerns ([Hermes built-in tools reference](https://hermes-agent.nousresearch.com/docs/reference/tools-reference/), [Hermes toolsets reference](https://hermes-agent.nousresearch.com/docs/reference/toolsets-reference)).

Vivy already has the important control-plane seams: a Go-owned `Tool` contract and deterministic registry, request-scoped selection, schema validation, read-only/effectful policy, approval suspension and resume, bounded/redacted tool output, per-run workspace isolation, durable SQLite sessions/messages/runs, and parent-owned child-run authority. The recommended design is therefore an adapter-shaped port that plugs into these seams rather than copying Hermes' Python registry or backend abstractions.

### Decision

The revised scope prioritizes the following order:

1. **P0 — unified HUMAN IN THE LOOP contract**, with server-side approval, review payloads, expiry, deny, cancellation, and restart recovery.
2. **P0 — complete file toolset**: `read_file`, `search_files`, `write_file`, and `patch`; writes are approval-gated and show a diff before execution.
3. **P0 — complete Skills toolset**: `skills_list`, `skill_view`, and `skill_manage`; all skill mutations are staged and human-approved.
4. **P1 — `session_search` read-only recall**, after an FTS5 capability spike.
5. **P1 — `todo`**, preferably durable per session rather than Hermes' in-memory-only store.
6. **P2 — local `terminal`/`process` and `cronjob`**, still subject to the same HITL contract.

Do not port `delegate_task` or `clarify` as Hermes features: Vivy already has durable parent/child workers and the `ask_user` question interaction. Do not port Hermes' multi-backend terminal, browser, code execution, memory providers, or messaging integrations in the first wave. This is an external-dependency decision, not a reduction of the file/Skills/HITL scope.

## 1. Technical Research Scope Confirmation

### Scope

- **Architecture analysis:** Hermes registry/toolset model versus Vivy's existing registry, broker, policy, and workspace boundaries.
- **Implementation approaches:** Go standard library and existing SQLite backend first; no new Python runtime or browser automation stack.
- **Technology stack:** Hermes Python modules and dependency groups, Vivy's Go 1.26.4 module and `modernc.org/sqlite` backend.
- **Integration patterns:** tool schemas, request-scoped selection, durable storage, approval/resume, output truncation, and JSON-RPC/UI exposure.
- **Performance and safety:** bounded results, index maintenance, path escape prevention, prompt-injection signals, lifecycle/restart behavior.

### Methodology

Local Hermes source was inspected statically, including tool registrations, implementation modules, dependency declarations, and license. Vivy source and roadmap/TODO documents were checked to distinguish implemented code from deferred design. Current public claims were verified against Hermes' official documentation and Eino's official documentation/repository pages. Confidence levels below refer to portability conclusions, not to benchmarked runtime performance.

## 2. Source and Technology Landscape

### Eino-native boundary assessment

The current Vivy module is pinned to `github.com/cloudwego/eino v0.9.13`. That version already contains more relevant primitives than a plain tool-calling API:

| Capability | Eino native support | What Vivy still owns | Current incremental difficulty |
|---|---|---|---:|
| Tool schema/execution | `BaseTool`, `InvokableTool`, `ToolsNode`, tool-call middleware | Vivy registry, selection, argument policy, audit, redaction | Low |
| Agent loop | `ChatModelAgent`, `Runner`, streaming events, iteration loop | Vivy run state, journal mapping, app lifecycle, provider boundary | Low; already integrated |
| HITL interrupt | `Interrupt`, tool interrupt state, targeted resume context | Approval policy, durable proposal, UI review, first-writer-wins, expiry, stale-target checks | Medium; mechanism already proven |
| Checkpoint/resume | `CheckPointStore`, `WithCheckPointID`, `ResumeWithParams`, serialization | Version envelope, backend storage, migration, recovery classification, security binding | Medium; adapter already integrated |
| File operations | `adk/middlewares/filesystem` injects `ls`, `read_file`, `write_file`, `edit_file`, `glob`, `grep`; `filesystem.Backend` is pluggable | Workspace containment, protected paths, atomic/diff semantics, approval, output/redaction | Medium |
| Skill discovery/loading | `adk/middlewares/skill` with `Backend.List`/`Get`, filesystem backend, progressive disclosure | Trusted roots, injection warnings, provenance, CRUD/staging, approval, rollback | Low for read-only; High for management |
| Task planning | ADK/DeepAgent plan-task patterns and todo support | Vivy durable session semantics and journal/UI contract | Low-medium |
| Session full-text search | No Vivy-compatible session/FTS store | SQLite migration, FTS query API, indexing, result shaping | Medium |
| File/Skill review UI | No product-level review surface | JSON-RPC/UI, diff display, approve/reject/cancel controls | High |

This changes the implementation strategy: use Eino middleware as the execution **shape**, but put Vivy-owned backends and adapters underneath it. The recommended file path is a custom `filesystem.Backend` backed by `WorkspaceManager`; the recommended Skill path is a custom `skill.Backend` backed by the configured Skills root. For the existing Vivy architecture, preserving the Vivy tool names and `ToolSpec` policy boundary is safer than exposing Eino's default middleware tools directly.

Eino does not turn filesystem or Skill middleware into a security policy. Its `filesystem.Backend` exposes read/search/write/edit operations, but workspace authority, sensitive-path denial, precondition hashes, diff generation, human approval, and rollback remain application responsibilities. Likewise, Eino Skill middleware loads `List`/`Get` content; it does not provide Hermes-style `skill_manage` CRUD, staged revisions, provenance, or approval.

### Additional Eino-native tool surfaces

Beyond filesystem and Skill middleware, the pinned Eino v0.9.13 source contains several tool surfaces that are relevant to Vivy:

| Eino surface | What it provides | Vivy recommendation | Difficulty |
|---|---|---|---:|
| `adk/middlewares/plantask` | `task_create`, `task_get`, `task_update`, and `task_list`, backed by a small storage interface | Add as the first native replacement for Hermes `todo`, but adapt the backend to Vivy session/journal storage instead of raw task files | Low-medium |
| `adk/middlewares/dynamictool/toolsearch` | A `tool_search` meta-tool, deferred tool metadata, and progressive tool visibility | Add when the registry grows; keep Vivy's allowlist, request-scoped selection, and audit as the authority | Low-medium |
| Filesystem `StreamingShell` | Optional `execute` tool alongside the filesystem tools | Keep P2; expose only through the existing terminal policy and HITL gate | High |
| `adk.NewAgentTool` | Wrap an Agent as a callable tool, including nested checkpoint/streaming behavior | Do not duplicate now; Vivy already has parent-owned child runs and worker orchestration | Low to reuse, no immediate feature |
| Enhanced tool interfaces | Structured text/image/audio/video/file results, including streaming variants | Useful for future attachments, generated files, and multimodal results; requires UI/result transport support | Low-medium |
| Tool middleware | Tool-call interception, argument repair, output reduction/summarization, callbacks | Reuse as runtime hardening, not as user-facing tool types; all mutation policy remains Vivy-owned | Low-medium |

The official `eino-ext` repository adds general-purpose implementations, but these are extensions rather than Eino core and are not all pinned in Vivy's current `go.mod`. The agreed target includes MCP, HTTP request, command line, Bing/Google/DuckDuckGo/SearXNG search, Wikipedia, and sequential thinking. Basic network search is required and must use search APIs or HTTP providers; it must not depend on browser automation. Browser-use is explicitly excluded.

Eino documentation also names HTTP, database, calculator, and code-executor tools as examples of what can be modeled as a Tool. That is a capability pattern, not a native Vivy-ready implementation. Database access, calculator policy, code sandboxing, session search, memory, cron, and process management still require Vivy-owned contracts and controls.

### Agreed scope decision

- Add all reviewed Eino core surfaces and the reviewed `eino-ext` tool families except Browser Use.
- Add basic network search as a first-class capability through a provider-neutral search contract and API-backed adapters; browser search/automation is out of scope.
- Treat GraphTool as a test-only fixture for validating nested workflows, interrupt propagation, checkpoint behavior, and tool wrapping. Do not expose GraphTool as a production Vivy user capability; it may become a future Vivy differentiator.
- Keep every external or effectful extension behind Vivy's ToolAdapter, provenance, output limits, network policy, and HUMAN IN THE LOOP rules. EinoExt availability does not bypass those controls.

### Hermes technology shape

- Local checkout version: `0.15.1` in `pyproject.toml`; the upstream repository is MIT-licensed ([Hermes Agent repository](https://github.com/NousResearch/hermes-agent)).
- Core Python dependencies include HTTP/client, CLI, YAML, cron, JWT, timezone, and `psutil` support. Browser, voice, provider, messaging, MCP, remote sandbox, and image/video features are optional or lazy-installed in the local `pyproject.toml`.
- `toolsets.py` models tools as named bundles and composites. `tools/registry.py` discovers modules through top-level `registry.register(...)` calls and stores handler, schema, availability checks, output limits, and metadata.
- Hermes' core tool list includes `terminal`, `process`, four file tools, skills, todo, memory, session search, clarify, code execution, delegation, cronjob, browser, web, media, and platform integrations. The current official tool reference documents the same broad grouping ([Built-in Tools Reference](https://hermes-agent.nousresearch.com/docs/reference/tools-reference/)).

### Vivy technology shape

The current implementation evidence is in:

- [`internal/tools/tools.go`](../../internal/tools/tools.go): Go-owned `Tool` interface, registry, deterministic request selection, and argument validation.
- [`internal/runtime/tooladapter.go`](../../internal/runtime/tooladapter.go): schema publication, safety validation, policy evaluation, approval/question interruption, result redaction and compaction.
- [`internal/runtime/isolation.go`](../../internal/runtime/isolation.go): validated per-run workspace allocation and path containment boundary.
- [`internal/storage/sqlite/sqlite.go`](../../internal/storage/sqlite/sqlite.go): pure-Go SQLite backend with versioned migrations and one-writer configuration.
- [`internal/storage/sqlite/messages.go`](../../internal/storage/sqlite/messages.go): append-only session message persistence.
- [`docs/GOAL-AGENT-HARNESS-ROADMAP.md`](../../docs/GOAL-AGENT-HARNESS-ROADMAP.md) and [`docs/TODO.md`](../../docs/TODO.md): current harness scope and deferred memory/RAG, MCP/plugins, extra providers, and filesystem-journal work.

The main compatibility gap is not Eino tool invocation. It is the Vivy-owned control plane around the Eino primitives: filesystem safety and mutation review, Skills management/staging, HITL persistence and review UI, FTS5 search tables for session recall, and possibly a durable todo table.

## 3. Candidate Portability Matrix

| Hermes capability | External dependency weight | Vivy fit | Risk | Recommendation |
|---|---:|---:|---:|---|
| `session_search` | Low; local SQLite/FTS5 | High; sessions and messages already exist | Medium: migration/index/query semantics | **P1** after FTS5 spike |
| `read_file` | Low; filesystem only | High; workspace manager exists | Medium: path/symlink/size handling | **P0** |
| `search_files` | Low; filesystem + search implementation | High if bounded and workspace-scoped | Medium: regex cost and result volume | **P0** |
| `skills_list` | Low; local directories/YAML frontmatter | High once a skills root is configured | Medium: trust root and discovery policy | **P0** |
| `skill_view` | Low; local files | High with untrusted-content handling | High: instructional content injection | **P0**, warnings required |
| `skill_manage` | Low; local files/YAML | Medium-high | High: create/edit/patch/delete effects | **P0**, staged HITL |
| `todo` | None; Hermes store is per-agent in memory | Medium | Low-medium: session semantics and persistence | **P0/P1** durable per session |
| `write_file` | Low external weight | High after approval/staging | High: destructive workspace effect | **P0**, diff + HITL |
| `patch` | Low external weight | High after safe patch engine | High: fuzzy replacement and accidental edits | **P0**, diff + HITL |
| Unified HITL approval | No external dependency; runtime control plane | Already partially present in Vivy | High: stale/replayed approvals must fail closed | **P0 prerequisite** |
| local `terminal` | OS process only | Medium | Very high: arbitrary command execution | **P2**, workspace-only first |
| `process` | `psutil` in Hermes; OS APIs in Go | Medium | High: process tree, kill, restart semantics | **P2**, only with terminal |
| `cronjob` | `croniter` in Hermes; scheduler/persistence in Vivy | Medium | High: durable lifecycle and delivery | **P2** |
| `memory` | Local persistence, but broad prompt injection surface | Low currently | High and deliberately outside current Vivy scope | Defer |
| `tool search` | No heavy dependency; Eino has dynamic tool middleware | High once the registry grows | Low-medium: must preserve Vivy selection and audit | **P0/P1** |
| basic network search | EinoExt search providers or HTTP APIs | High; required capability | Medium: credentials, rate limits, untrusted results, provider drift | **P1**, API-backed only |
| MCP tools | MCP client/adapter in EinoExt | Medium-high | High: remote tool provenance, arbitrary side effects, lifecycle | **P1**, approval by default |
| GraphTool | Eino workflow wrapping pattern | Test fit only | Medium: nested interrupt/checkpoint behavior | **Test-only**, no production exposure |
| `clarify` | None | Already covered by `ask_user` | N/A | Do not duplicate |
| `delegate_task` | No browser dependency, but large orchestration surface | Already implemented differently | N/A | Do not port |
| `execute_code` | Sandboxing/RPC/runtime | Low | Critical | Exclude |
| browser-use | Browser runtime and external automation dependencies | Explicitly excluded | High | Exclude |
| messaging/Home Assistant/Spotify/Feishu/Yuanbao | Platform SDKs and credentials | Low | High | Exclude first wave |

## 4. Recommended Port Designs

### 4.1 `session_search` — highest-value candidate

Hermes provides three read-only shapes: discovery by query, scroll around a message anchor, and browse recent sessions. The official implementation uses SQLite FTS5, returns actual messages, and does not call an LLM ([Sessions and `session_search`](https://github.com/NousResearch/hermes-agent/blob/main/website/docs/user-guide/sessions.md), [session storage design](https://github.com/NousResearch/hermes-agent/blob/main/website/docs/developer-guide/session-storage.md)).

**Vivy mapping:** add a storage-level search contract rather than exposing SQLite to the tool package. Add a versioned migration containing an FTS virtual table over `messages.content`, plus insert/delete maintenance. The tool should accept a query, bounded result limit, optional session/role filters, and an anchor/window for scroll. Keep the result read-only and pass it through the existing untrusted-result header, redaction, and output compaction.

**Required spike:** confirm that the pinned `modernc.org/sqlite` build supports `CREATE VIRTUAL TABLE ... USING fts5` and the required `bm25`/`snippet` functions on the target Windows build. This must be a small storage test before committing to the migration. If FTS5 is unavailable, use a bounded `LIKE` fallback only as an explicit degraded mode; do not silently claim FTS ranking.

**Why first:** it directly addresses the current context-recall gap without introducing a network service, embeddings, a memory provider, or long-term prompt injection. It also uses data Vivy already persists.

### 4.2 Full file toolset — read, search, write, and patch

The required Hermes file surface is the complete four-tool bundle: `read_file`, `search_files`, `write_file`, and `patch`. The local implementation includes pagination, binary detection, bounded results, path write deny lists, line-ending and UTF-8 BOM preservation, atomic writes, unified diffs, fuzzy replacement, and optional syntax/LSP diagnostics. Hermes' security guide also describes a safe-root boundary and hard-blocked credential paths ([Hermes file-write security](https://hermes-agent.nousresearch.com/docs/user-guide/security/), [file mutation checkpoints](https://hermes-agent.nousresearch.com/docs/user-guide/checkpoints-and-rollback/)).

**Vivy mapping:** implement the four tools as one capability family but keep each tool's schema and audit event explicit. All paths resolve against the run workspace and are revalidated immediately before the filesystem operation. The read/search tools are read-only and can auto-execute. `write_file` and `patch` are effectful and must pause for HITL before `InvokableRun` reaches the mutation.

Eino can reduce the plumbing here: `adk/middlewares/filesystem` already defines the backend protocol and injects file tools, and its backend interface covers list/read/grep/glob/write/edit. There are two viable integration modes:

1. **Recommended:** implement Vivy's `filesystem.Backend` adapter, then wrap or expose the operations through Vivy's existing `ToolSpec`/policy adapter. This preserves Hermes-compatible names and Vivy's approval/audit boundary.
2. **Fast prototype:** configure Eino filesystem middleware directly with a workspace backend and replace the default tool definitions. This is useful for a spike, but direct middleware exposure risks bypassing Vivy's per-tool selection and mutation proposal semantics.

Eino supplies the execution path; it does not supply the final security contract. The production difficulty is therefore **medium**, with most work in the Vivy backend and HITL layer rather than in Eino integration.

The write path should include:

- path containment, symlink checks, NUL/traversal rejection, credential/secret deny rules, and configurable safe roots;
- atomic temp-file plus replace semantics, preserving line endings and BOM when applicable;
- patch modes with unique-match default, explicit `replace_all`, fuzzy matching only within a bounded algorithm, and a unified diff preview;
- post-write re-read/hash verification, bounded syntax diagnostics where a local checker exists, and a mutation-verifier event when the requested change did not land;
- optional pre-mutation checkpoint through Vivy's existing generation/blob mechanism rather than a second shadow Git store.

The user must approve a structured proposal containing tool, path, operation, old/new size, diff preview, risk findings, and the exact run/tool-call binding. Approval is not permission to skip the final path and content revalidation.

### 4.3 Full Skills support — list, view, and manage

Hermes skills use a directory containing `SKILL.md`, optional references/templates/scripts/assets, YAML frontmatter, and progressive disclosure: list metadata first, load full content only when needed ([Skills System](https://hermes-agent.nousresearch.com/docs/user-guide/features/skills/)). The full required surface is `skills_list`, `skill_view`, and `skill_manage`; `skill_manage` supports create, edit, patch, delete, supporting-file write, and supporting-file removal.

**Vivy mapping:** define a configured Skills root and a Skills store/reader with:

- frontmatter parsing and bounded metadata (`name`, `description`, platform, prerequisites, version, and tags);
- platform/prerequisite readiness checks without executing skill commands;
- linked-file access restricted to the Skill directory and known subtrees (`references/`, `templates/`, `scripts/`, `assets/`);
- prompt-injection and credential/exfiltration signals surfaced as warnings, while all loaded Skill text remains untrusted data;
- create/edit/patch/delete/write-file/remove-file operations through one mutation planner and one approval gate;
- provenance (`source`, owner, created-by, last-approved revision), content hashes, atomic writes, rollback, and pending-revision cleanup on restart.

Eino's `adk/middlewares/skill` already covers the read-only `Backend.List`/`Get` contract and progressive disclosure. Use it for prompt/context loading only after the Vivy Skills backend has applied root, platform, size, and untrusted-content checks. Do not use Eino's read-only Skill backend as the mutation API: `skill_manage` remains a Vivy-owned pending-revision service.

Hermes' current documentation explicitly separates the content scanner from the write-approval gate: `skills.write_approval` stages every agent Skill write for review, and staged writes survive restarts until approved or rejected ([Skills write approval](https://hermes-agent.nousresearch.com/docs/user-guide/features/skills/), [Hermes configuration](https://github.com/NousResearch/hermes-agent/blob/main/website/docs/user-guide/configuration.md)). Vivy should adopt this stronger default: `skill_manage` never writes directly from a model call; it creates a pending revision and waits for human approval.

### 4.4 HUMAN IN THE LOOP — first-class runtime capability

HITL must be a runtime invariant, not a prompt instruction. Hermes' security model already demonstrates the useful separation: protected file paths are hard-denied, while human approval handles destructive actions; its Skills system stages writes for approve/reject review. Hermes also has an open request for general per-tool/per-toolset approval, which confirms that approval policy should be centralized instead of implemented only for shell commands ([Hermes security guide](https://hermes-agent.nousresearch.com/docs/user-guide/security/), [per-tool approval issue](https://github.com/NousResearch/hermes-agent/issues/33905)).

**Vivy policy:**

- read-only tools: auto-allow by default;
- file mutations, Skill mutations, terminal/process, cron, and future external writes: `always_ask` by default;
- hard-denied operations: never enter approval, such as escaping the workspace, touching credential paths, invalid patch targets, or malformed Skill structure;
- user questions: remain the separate `ask_user` control-flow interaction, not a disguised approval;
- any “allow always” policy must be explicit, scoped to a tool/profile/path rule, auditable, and never inferred from natural language.

**Approval payload:** `approval_id`, `run_id`, `tool_call_id`, tool and action, normalized target, precondition/hash, diff or command preview, policy profile/hash, risk findings, expiry, and requested decision. The actual mutation rechecks all of these bindings after resume.

**Lifecycle:** model call → policy decision → durable pending proposal → run suspended → UI/JSON-RPC review → first-writer-wins approve/deny → exact checkpoint resume or cancellation. Pending proposals must survive restart; stale, expired, changed-target, changed-file, duplicate, and unknown decisions fail closed.

### 4.5 `todo` — small, but make it durable

Hermes' `TodoStore` is intentionally small: four statuses (`pending`, `in_progress`, `completed`, `cancelled`), ordered items, replace/merge writes, and a rule that only one item is in progress. It is in-memory per session and is re-injected after context compression.

**Vivy adaptation:** use a session-scoped durable table or a journal-backed projection. Keep `todo` effectful and approval policy explicit if the tool is model-invoked; alternatively treat it as a control-plane plan update with a dedicated event type rather than a generic file write. Enforce item count/content limits and the single-`in_progress` invariant server-side. Do not inject completed items back into prompts.

### 4.6 Eino extension tool bundle — required scope

The required network-facing bundle should be exposed through Vivy-owned interfaces rather than registered directly from EinoExt:

- **Basic network search:** define one normalized `SearchProvider` contract and add API-backed adapters for Bing, Google, DuckDuckGo, SearXNG, and Wikipedia. Return bounded title/URL/snippet/source-provider records, mark all remote text as untrusted, and keep browser automation out of the design.
- **HTTP request:** support bounded, policy-checked requests for the approved read-only surface first. Any external write, credential-bearing request, or non-allowlisted host enters the HITL path or is hard-denied.
- **MCP:** load remote tools with server/tool provenance, capability selection, timeout, output limits, disconnect/reconnect behavior, and per-call policy. MCP tools must enter the same ToolAdapter and ToolSearch catalog as local tools.
- **Command line / execute:** implement only after the workspace-scoped process policy exists; reuse the terminal HITL contract and never inherit Eino's unrestricted local backend defaults.
- **Sequential thinking:** add as a bounded local planning aid; keep its state and output out of the user-visible reasoning contract unless explicitly needed.
- **GraphTool:** build conformance tests only. Test nested tool calls, interruption propagation, checkpoint/resume, cancellation, and error boundaries, but do not register GraphTool as a production Vivy tool.

The current `go.mod` does not pin all EinoExt tool modules. Each adapter needs a version compatibility check against Eino v0.9.13, dependency/license review, and a contract test before inclusion.

## 5. Capabilities requiring separate controls or future product decisions

### Local terminal and process management

Hermes' terminal implementation is not a small local shell wrapper: it supports local, Docker, Modal, SSH, Singularity, and Daytona backends, with process tracking and backend-specific cleanup. The official security documentation presents the backends as different isolation/security profiles ([Hermes security guide](https://github.com/NousResearch/hermes-agent/blob/main/website/docs/user-guide/security.md), [environment variables](https://github.com/NousResearch/hermes-agent/blob/main/website/docs/reference/environment-variables.md)).

Only a local, workspace-scoped command runner is a plausible later Vivy slice. It must be a new capability proposal with command allow/deny policy, environment allowlisting, bounded stdout/stderr, cancellation, process-tree cleanup, audit events, and restart behavior. Do not import the multi-backend concept or silently run commands in the host working directory.

### Cronjob

Hermes uses `croniter` and durable job management. Vivy would need scheduler ownership, persistence, startup recovery, timezone semantics, duplicate-fire prevention, cancellation, and delivery to a session/run. This is not a simple tool port; defer until a scheduler contract exists.

### Memory

Hermes distinguishes persistent facts from procedural skills and injects memory across sessions ([Hermes FAQ](https://github.com/NousResearch/hermes-agent/blob/main/website/docs/reference/faq.md)). Vivy's current capability proposal deliberately keeps Memory/RAG/long-term injection out of scope and already has a notes trio. Porting Hermes memory now would duplicate semantics and introduce a larger trust boundary.

### Tool search and registry discovery

Hermes' tool search is primarily progressive schema disclosure for MCP and non-core plugin tools; built-in core tools remain loaded directly ([Tool Search](https://hermes-agent.nousresearch.com/docs/user-guide/features/tool-search)). Vivy already has deterministic request-scoped selection and startup validation. Add Eino ToolSearch as a visibility/catalog layer, not as a second authority: the Vivy registry, request scope, policy, and audit remain authoritative.

### GraphTool boundary

GraphTool is deliberately test-only in this roadmap. It should validate that Vivy can embed and resume nested Eino workflows without making GraphTool the product's public workflow model. Future Vivy-specific workflow/graph capabilities may use these conformance tests as a compatibility baseline, while keeping the production API and semantics Vivy-owned.

### Clarification and delegation

Vivy already has `ask_user` as a durable question suspension and already implements parent-owned child runs, worker loops, policy, budget, and workspace authority. These are architectural equivalents, not missing Hermes tools.

## 6. Architecture and Integration Plan

```text
Model tool call
      |
      v
Vivy ToolAdapter
  - schema validation
  - prompt/path safety
  - policy + approval/question control flow
  - hooks, redaction, output budget
      |
      +--> read/search filesystem --> WorkspaceManager --> per-run root
      |
      +--> write/patch filesystem --> MutationPlan --> HITL approval --> atomic apply
      |
      +--> session_search -------------> Storage contract --> SQLite FTS5 adapter
      |
      +--> skills_list/view ------------> Trusted skills-root reader
      |
      +--> skill_manage ----------------> Pending revision --> HITL approval --> Skills root
      |
      +--> todo ------------------------> Session projection/journal
      +--> tool_search ------------------> Allowlisted tool catalog --> Vivy selection/policy
      +--> network_search ---------------> SearchProvider adapters --> untrusted result normalizer
      +--> http_request / MCP -----------> External policy --> HITL or hard deny
      +--> execute / commandline --------> Workspace process policy --> HITL
      |
      v
Durable pending approvals + run events + UI/JSON-RPC review/replay
```

The tool implementation should remain independent of Eino, as current `internal/tools` does. Runtime should remain the authority for policy, workspace identity, HITL state, and final revalidation. Storage-specific features such as FTS5 should stay behind a storage interface; the existing storage documentation explicitly treats FTS as an adapter detail. File and Skill writes should share the mutation-plan/approval machinery but retain separate domain validators.

## 7. Implementation Roadmap

### Wave P0-A: HITL and capability spikes

1. Freeze the mutation approval contract: proposal, diff/preview, policy snapshot, target hash/precondition, expiry, first-writer-wins decision, deny/cancel, restart recovery, and final revalidation.
2. Add the HITL review surface over the existing approval/checkpoint path; prove approve, deny, expiry, stale-target, duplicate-decision, cancel, and restart cases.
3. FTS5 support probe against the pinned `modernc.org/sqlite` build on Windows.
4. Spike an Eino `filesystem.Backend` adapter over `WorkspaceManager`; verify Eino middleware tool injection does not bypass Vivy policy/selection.
5. Spike an Eino `skill.Backend` adapter over a configured Skills root; verify progressive disclosure and untrusted-content handling.
6. Define the full workspace file contract and test traversal, symlink escape, protected paths, binary files, oversized files, atomic replacement, line endings/BOM, patch mismatch, and cancellation.
7. Define the Skills root/provenance contract and test frontmatter, linked-file containment, malformed content, injection warnings, pending revision persistence, and rollback.

### Wave P0-B: complete file capability

1. Implement the Vivy filesystem backend over Eino's backend interface; preserve Vivy's tool naming and policy boundary.
2. Implement `read_file` with line pagination and binary/size metadata.
3. Implement `search_files` with bounded file glob/content search; start with literal search and add regex only with an explicit timeout/size strategy.
4. Implement `write_file` with atomic replacement, safe-root/deny rules, precondition capture, and HITL diff review.
5. Implement `patch` with unique/fuzzy matching, unified diff, atomic apply, post-write verification, and HITL review.
6. Add tool schemas, selection keywords, result bounds, mutation audit events, mutation-verifier events, and JSON-RPC/UI review surfaces.

### Wave P0-C: complete Skills capability

1. Configure a trusted Skills root and implement Vivy's Eino `skill.Backend` for metadata discovery and full content loading.
2. Implement bounded `skills_list`/`skill_view` behavior with frontmatter/platform checks, linked-file reads, and prompt-injection warning signals.
3. Implement `skill_manage` create/edit/patch/delete/write-file/remove-file as pending revisions, never direct model writes.
4. Add human diff/review/approve/reject, provenance, content hashes, atomic apply, rollback, and restart recovery.

### Wave P0-D: native Eino surfaces and test-only GraphTool

1. Add `plantask` through a Vivy session/journal backend and enforce durable todo invariants.
2. Add Eino ToolSearch over the allowlisted Vivy registry; verify selection, policy, audit, and prompt-cache behavior.
3. Add enhanced ToolResult mapping for text/image/audio/video/file outputs without weakening redaction or output limits.
4. Add GraphTool conformance tests for nested workflow calls, interrupts, checkpoint/resume, cancellation, and failure propagation; do not expose GraphTool in the production tool catalog.

### Wave P1: session search, network search, and remote tools

1. Add the storage contract and FTS migration for `session_search` discovery, scroll, and browse shapes.
2. Define the provider-neutral network search contract and add bounded adapters for Bing, Google, DuckDuckGo, SearXNG, and Wikipedia; browser automation is not included.
3. Add EinoExt HTTP Request as a read-only, host-allowlisted tool; route external writes through HITL.
4. Add EinoExt MCP with server/tool provenance, dynamic catalog integration, timeouts, output bounds, lifecycle handling, and per-call approval.
5. Add EinoExt Sequential Thinking with bounded state and output tests.

### Wave P2: effectful local capabilities

Treat each item as a separate proposal and commit: Eino `execute`/EinoExt `commandline`, local `terminal`, `process`, then `cronjob`. Every proposal must reuse the P0 HITL contract and include approval behavior, cancellation, restart/recovery, output limits, audit events, and end-to-end tests before implementation.

## 8. Risks and Mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| FTS5 unavailable or behavior differs in the pure-Go SQLite build | Blocks session recall or produces false confidence | Capability spike; versioned adapter; explicit degraded mode |
| File tool escapes workspace through symlink/relative path | Host data disclosure or mutation | Resolve-and-contain checks; reject symlinks; run-scoped root only |
| Approval is shown before the target changes, then the target changes before apply | Human approved a different operation | Persist target hash/diff; revalidate path, file hash, policy, and args immediately before apply |
| Large or sensitive file diff is exposed to the reviewer/model | Secret leakage or unusable review | Secret/path redaction, bounded diff, head/tail marker, hard deny for credential paths |
| Pending Skill revision is applied twice or lost on restart | Duplicate or unreviewed procedural behavior | Durable revision IDs, content hashes, first-writer-wins decision, idempotent apply/rollback |
| Skill text contains prompt injection | Model behavior hijack | Treat as untrusted data; warnings; no automatic execution/injection |
| Search/terminal output overwhelms context | Cost, latency, model confusion | byte/line/record caps; head/tail tombstone; pagination |
| Tool semantics are copied too literally from Hermes | Architectural duplication and policy bypass | Map behavior into existing Vivy contracts; no copied Python runtime |
| Hermes changes upstream | Drift or license/source mismatch | Pin reviewed source snapshot, preserve MIT notices for copied code, record provenance |
| Durable todo and session search mutate current V0 schema | Migration/recovery regressions | additive migrations, reopen tests, conformance tests, no domain leakage of SQLite details |

## 9. License and Porting Hygiene

The local Hermes checkout carries the MIT License and upstream marks the repository MIT. A source translation or direct code reuse must retain the Nous Research copyright and license notice in accordance with MIT terms. The safer default for Vivy is to port the externally visible behavior and invariants, reimplementing in Go behind Vivy-owned interfaces; any copied implementation fragment needs explicit provenance and attribution in the commit/documentation.

## 10. Research Quality and Source Verification

### Local evidence

- Hermes: `toolsets.py`, `tools/registry.py`, `tools/file_tools.py`, `tools/file_operations.py`, `tools/path_security.py`, `tools/tool_output_limits.py`, `tools/session_search_tool.py`, `tools/skills_tool.py`, `tools/skill_manager_tool.py`, `tools/approval.py`, `tools/checkpoint_manager.py`, `tools/todo_tool.py`, `tools/terminal_tool.py`, `tools/process_registry.py`, `pyproject.toml`, and `LICENSE`.
- Vivy: `internal/tools`, `internal/runtime`, `internal/storage/sqlite`, `docs/GOAL-AGENT-HARNESS-ROADMAP.md`, and `docs/TODO.md`.
- Eino: `go.mod` pins `github.com/cloudwego/eino v0.9.13`; the local module contains `adk/middlewares/filesystem`, `adk/middlewares/skill`, `adk/middlewares/plantask`, and the ADK checkpoint/interrupt APIs used by the runtime.

### Public sources

- [Hermes built-in tools reference](https://hermes-agent.nousresearch.com/docs/reference/tools-reference/)
- [Hermes toolsets reference](https://hermes-agent.nousresearch.com/docs/reference/toolsets-reference)
- [Hermes sessions and session_search](https://github.com/NousResearch/hermes-agent/blob/main/website/docs/user-guide/sessions.md)
- [Hermes SQLite session storage](https://github.com/NousResearch/hermes-agent/blob/main/website/docs/developer-guide/session-storage.md)
- [Hermes skills system](https://hermes-agent.nousresearch.com/docs/user-guide/features/skills/)
- [Hermes security and file-write safety](https://hermes-agent.nousresearch.com/docs/user-guide/security/)
- [Hermes Skills write approval](https://github.com/NousResearch/hermes-agent/blob/main/website/docs/user-guide/configuration.md)
- [Hermes checkpoints and rollback](https://hermes-agent.nousresearch.com/docs/user-guide/checkpoints-and-rollback/)
- [Hermes per-tool approval proposal](https://github.com/NousResearch/hermes-agent/issues/33905)
- [Hermes tool search](https://hermes-agent.nousresearch.com/docs/user-guide/features/tool-search)
- [Hermes security guide](https://github.com/NousResearch/hermes-agent/blob/main/website/docs/user-guide/security.md)
- [Hermes environment variables](https://github.com/NousResearch/hermes-agent/blob/main/website/docs/reference/environment-variables.md)
- [Hermes repository and license](https://github.com/NousResearch/hermes-agent)
- [Eino overview and ADK capabilities](https://www.cloudwego.io/docs/eino/overview/)
- [Eino Agent Runner, checkpoint persistence, and resume](https://www.cloudwego.io/docs/eino/core_modules/eino_adk/agent_extension/)
- [Eino interrupt and checkpoint primitives](https://www.cloudwego.io/docs/eino/core_modules/chain_and_graph_orchestration/checkpoint_interrupt/)
- [Eino ToolsNode and tool-call middleware](https://www.cloudwego.io/docs/eino/core_modules/components/tools_node_guide/)
- [Eino filesystem middleware](https://www.cloudwego.io/docs/eino/core_modules/eino_adk/eino_adk_chatmodelagentmiddleware/middleware_filesystem/)
- [Eino filesystem Backend interface](https://www.cloudwego.io/docs/eino/core_modules/eino_adk/eino_adk_chatmodelagentmiddleware/filesystem_backend/)
- [Eino Skill middleware](https://www.cloudwego.io/docs/eino/core_modules/eino_adk/eino_adk_chatmodelagentmiddleware/middleware_skill/)
- [Eino ToolSearch middleware](https://www.cloudwego.io/docs/eino/core_modules/eino_adk/eino_adk_chatmodelagentmiddleware/middleware_toolsearch/)
- [Eino filesystem and DeepAgent tools quick start](https://www.cloudwego.io/docs/eino/quick_start/chapter_04_tool_and_filesystem/)
- [Eino tool creation and EinoExt/MCP integration](https://www.cloudwego.io/docs/eino/core_modules/components/tools_node_guide/how_to_create_a_tool/)
- [Eino Graph/Agent tool composition](https://www.cloudwego.io/docs/eino/overview/graph_or_agent/)
- [EinoExt tool implementations](https://github.com/cloudwego/eino-ext/tree/main/components/tool)
- [Eino official examples, including human-in-the-loop and Skill middleware](https://github.com/cloudwego/eino-examples)

### Confidence

- **High:** `session_search`, the complete file toolset, complete Skills toolset, todo, registry, and terminal shapes are present in the inspected local checkout and documented upstream.
- **High:** Vivy already owns the policy, approval, workspace, storage, and worker seams described above; these are verified in source.
- **Medium:** FTS5 feature availability and exact query-function compatibility in Vivy's pinned pure-Go SQLite build; requires the P1 spike.
- **Medium:** Exact UI/JSON-RPC review ergonomics for large file/Skill diffs; the durable approval/checkpoint semantics are already proven, but the new review payloads need implementation verification.
- **Low/not assessed:** performance under large databases, regex worst-case behavior, and production UX for new JSON-RPC/UI surfaces; these are implementation verification tasks.

## Conclusion

Hermes offers useful ideas, and the revised target is not merely a read-only subset. The full local file toolset, full Skills toolset, Eino-native task/tool-discovery surfaces, and the reviewed EinoExt tool bundle are all in scope except Browser Use. Basic network search is required through API-backed providers; browser search/automation is excluded. GraphTool is test-only so that nested workflow behavior can be validated without preempting a future Vivy-specific workflow feature. All effectful local, remote, MCP, HTTP, and Skill operations must remain behind a uniform HUMAN IN THE LOOP gate.

The implementation wave is complete in the current worktree: file/Skills families,
durable tasks, tool search, enhanced results, GraphTool conformance tests,
API-backed network search, bounded HTTP, MCP, sequential thinking, and
workspace-scoped execute/commandline are covered by Vivy contracts and tests.
Browser Use remains excluded, and GraphTool remains a conformance-test fixture
only. Remaining product work is UI-level review ergonomics and future
Vivy-native workflow features, not an unguarded external tool path.

**Technical Research Completion Date:** 2026-08-11  
**Source Verification:** Current Hermes and Eino public documentation plus local source inspection  
**Overall Confidence:** High for prioritization; medium for FTS5 implementation details
