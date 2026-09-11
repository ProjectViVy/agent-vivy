# Eino/upstream reuse inventory: what the VC track does not need to build itself (2026-08-31)

- Date: 2026-08-31
- Scope definition (user decision): **Align the functional surface with Crush; do not add functionality that Crush does not have.** This inventory covers only the Crush comparison surface (`docs/research/crush-parity-code-agent-research-2026-08-31.md` §3/§5 VC-0..VC-4 + existing merged items); any eino/upstream capability beyond that surface is either an internal implementation detail or explicitly "not exposed" (see the guardrails in §3).
- Verification baseline: eino v0.9.13 (locked in go.mod, read directly from the module cache); eino-ext components verified to exist through `go list -m @latest`; Crush upstream dependencies read from its go.mod (`.workspace/crush/`, FSL-1.1-MIT for reference only, no code copied).
- One-sentence conclusion: **About 60% of the VC track's "new tool surface/loops" work can be delivered with existing components—native eino middleware + the same upstream libraries as Crush (MIT/BSD, directly usable as dependencies); the parts that truly must be built in-house are governance hooks (archiving/approval/audit) and Vivy semantic components (masks/budget/Journal), which are precisely the differentiators.**

---

## 1. Three-tier reuse classification overview

Classification tiers: **A already in use** (zero work) / **B direct reuse** (wire it in; evaluation points are listed) / **C thin in-house build** (Crush also wrote it itself, with no upstream component to take, but most are small) / **D governance build** (Vivy semantics that should not be outsourced in the first place).

### 1.1 Native eino (v0.9.13, no new dependencies)

| Capability (Crush equivalent) | Tier | Notes and evaluation points |
|---|---|---|
| checkpoint/interrupt recovery (Crush resume) | A | `adk.CheckPointStore`; Vivy's VersionedCheckpointStore already consumes it. Pure runtime state, unrelated to file rollback (RB-1 L1/L2) |
| AGENTS.md injection (D6 context file) | B | `middlewares/agentsmd`: 5-level recursive @import, total-byte limit, transient model-call injection (not added to session state or summaries). **D6 is a free pass-through**; `vivy init` still needs to generate AGENTS.md itself |
| File/search tool registration surface (seven tools—ls/read_file/write_file/edit_file/glob/grep/execute—with Chinese and English descriptions) | B | Native registration in `middlewares/filesystem`; Vivy's `EinoFilesystemBackend` already implements `einofs.Backend`, so connecting the registration layer provides the grep/glob tools. Evaluation point: tool-name mapping (eino `edit_file` vs. Vivy `patch`—name them `edit`/`multiedit` to match the Crush surface; eino tool names can be customized through `selectToolName`); policy annotations/approval go through Vivy's existing pipeline |
| read supports images/PDF (VC-3 read_file images) | B | `filesystem.MultiModalReader` protocol slot is ready; only the backend implementation is needed |
| execute background flag | B (partial) | `Shell`/`StreamingShell` + `RunInBackendGround` protocol slots are native; **native job retrieval/termination management is absent** (see C) |
| Orphaned tool-call repair | B (evaluation) | `middlewares/patchtoolcalls`—resume/compaction-boundary hygiene; an internal component, not a product surface. May replace the in-house repair after evaluation |
| Compaction/skills/tool discovery | A | `summarization`/`reduction`/`skill`/`dynamictool(toolsearch)` is already used by Vivy or has an equivalent |
| Gitignore-aware glob (`**`) | B | eino itself uses `bmatcuk/doublestar/v4` (see 1.2); Vivy's `matchesGlob` is currently handwritten with filepath.Match and has no `**` recursion—switching to doublestar fills the gap |

### 1.2 Upstream libraries also used by Crush (MIT/BSD, directly usable as dependencies—"porting" means importing the library, consistent with FSL behavioral-alignment requirements)

| Crush use | Upstream library (license) | Vivy destination |
|---|---|---|
| Embedded POSIX shell (bash tool) | `mvdan.cc/sh/v3` (BSD-3) + `mvdan.cc/sh/moreinterp` (extended command interpreter, also used by Crush) | VC-1 bash tool core. Windows can run POSIX syntax without WSL; compatibility with sandbox path constraints is a VC-1 acceptance point |
| Unified diff generation | `aymanbagabas/go-udiff` (MIT) | VC-1 backend diff (write/patch/execute proposals and results output unified diff + addition/deletion counts); replaces Vivy's handwritten single-hunk pseudo-diff `boundedDiff` |
| `**` recursive glob | `bmatcuk/doublestar/v4` (MIT) | Upgrade glob-tool + search_files glob filtering |
| LSP client | `charmbracelet/x/powernap` (MIT) | VC-3 LSP manager (lazy startup/auto-discovery/diagnostics). Fallback: write a minimal jsonrpc2 client (1–2k lines, retained by the existing decision) |
| Small diff utilities | `pmezard/go-difflib` (BSD) | Miscellaneous tasks such as addition/deletion line counts (where go-udiff is insufficient) |
| Ripgrep first | External `rg` binary detection (same strategy as Crush) | grep tool is rg-first: use rg when available (native gitignore awareness), otherwise use a pure-Go fallback (the backend `GrepRaw` regex path has already been audited) |
| MCP client | `eino-ext/components/tool/mcp` v0.0.9 (Apache-2.0) + `mark3labs/mcp-go` v1.0.0 (MIT) | MCP slice is complete: Eino `GetTools` handles tools/schema; the official `client.NewStreamableHttpClient` prefers modern discovery with `mcp.LATEST_PROTOCOL_VERSION` and falls back to legacy; the mcp-go typed client handles list/call/resources/prompts/Close; Vivy retains `mcp_list_tools` as a read-only control-plane catalog, `isError`, untrusted/bounds, lifecycle, and MCPHost → ToolWorld → ToolHost governance. The former model-visible `mcp_call`/`PrepareMCPCall` path was retired in P4. stdio/OAuth/continuous listening are not implemented; typed plumbing is removed only after future Eino coverage includes resources/prompts/lifecycle while retaining `isError` |
| Native Anthropic | `eino-ext/components/model/claude` v0.1.25 (verified to exist) | VC-2, the established §8.5 decision, six-item implementation checklist |

### 1.3 UI side (browser components; do not build rendering in-house)

| Crush use | Upstream | Vivy destination |
|---|---|---|
| Unified/split diff view | react-diff-view / diff2html / primevue diff, etc. (select during implementation, MIT family) | VC-1 UI diff rendering (D10 presentation behavior aligned with Crush, using an existing component) |
| Syntax highlighting for file preview | shiki / prism (select during implementation) | VC-3 UI file preview |

## 2. C/D tiers: must be built in-house (Crush also writes these itself, or they are Vivy-specific semantics)

| Item | Tier | Notes |
|---|---|---|
| multiedit | C | eino has only single edit; Crush writes this itself. Small item |
| Patch whitespace-tolerant fallback | C | Write the same logic as Crush's `normalizedReplace` (behavioral alignment) |
| stale-read filetracker + file_versions storage + recovery RPC | D | RB-1 L1/L2: governance core (storage migration + approval + Journal); Crush's version chain also has no recovery consumer. Decision on 2026-09-01: the recording side (file_versions + filetracker) lands with the VC-3 tail payment; recovery RPC is deferred (RB-L2-DEFER, revisit after MVP) |
| Background job registry (job_output/job_kill, move to background on timeout) | C | eino has only the protocol slot; Crush writes it itself. Attach it inside the bash tool implementation |
| Infinite-loop detection (signature deduplication) | C | Crush writes it itself; Vivy attaches it beside MaxToolTurns |
| Cost-accounting metadata table (context window/prices) | C | D9 decision synchronized with web provider/model management; Crush uses remote Catwalk, which we explicitly do not bring in |
| Headless `vivy run` (FACE-0) | C | Crush writes the CLI surface itself; Vivy puts it in face assembly |
| Hooks engine (PreToolUse protocol) | C | Crush writes it itself; Vivy attaches the existing `ToolHookChain` |
| 401 re-authentication retry | C | Crush writes three branches itself; Vivy aligns with chatmodel retry (eino has native retry_chatmodel/failover—the evaluation point is that `adk` retry/failover ChatModel may cover most of it) |
| Automatic session title | C | Small item |
| Message queuing + two-phase cancellation | C | UI interaction item |
| Image-attachment path | C | UI + provider workaround |
| LSP diagnostic backfill | C | After powernap obtains diagnostics, the glue layer that backfills write/patch results is part of Vivy's governance surface |
| Pure-Go gitignore fallback audit | C | Fallback when rg is absent; `sabhiram/go-gitignore` (MIT) can be evaluated to reduce the work |
| Sandbox/approval/budget/Journal/masks | D | Vivy-specific differentiators that should not be outsourced in the first place |

## 2026-09-06 superseded note

The `middlewares/dynamictool/toolsearch` row above records the earlier
pre-implementation state. It is superseded for the current tool-search
track: Vivy now uses the pinned Eino v0.9.13 core middleware
`github.com/cloudwego/eino/adk/middlewares/dynamictool/toolsearch` for
allowlisted deferred active tools, while keeping Vivy adapters, policy,
HITL, audit, and Skill-mount projection around business tools. The retired
custom search implementation and its registry entry are removed; historical
research conclusions are otherwise unchanged.

## 3. Guardrails: eino/upstream capabilities beyond the Crush surface—disposition table (do not add them unilaterally)

| eino/upstream capability | Present in Crush? | Disposition |
|---|---|---|
| `adk/prebuilt/deep` (DeepAgents), `planexecute` | No | **Do not adopt or expose.** Rationale recorded after the user challenge on 2026-08-31: (1) `planexecute` = plan–execute–replan loop, absent from the Crush surface and excluded under "do not add unilaterally"; (2) `deep` is a complete DeepAgents stack with built-in task_tool/plan files, beyond Crush's task semantics and parallel to Vivy's governance (approval/budget/Journal/masks), so integrating it creates two tracks; (3) all three forms of "coordination" happen inside the eino graph, whereas Vivy child-agent governance (WorkerChildAuthority/budget/approval merged into the parent session D5/PolicySnapshot mask) is entirely in the runtime service layer—using a prebuilt requires dismantling it first and reconnecting it to the pipeline, which is more work than a thin wrapper around the existing child-run RPC. The Crush surface actually needs only the `agent` tool = thin wrapper. **Reopening condition:** if a planning–execution capability proposal is approved in the future, the prebuilt can be reevaluated as a kernel candidate (through the proposal process) |
| `adk/prebuilt/supervisor` | No (we also rejected Crush's hard-coded coder/task) | Do not adopt; the prebuilt itself is "a central agent coordinating a group of child agents" (verbatim supervisor.go package comment)—exactly the Crush coordinator pattern rejected by the 2026-08-31 decision; vivy = a single persona wearing a mask, with supervisor semantics handled by the product persona rather than an orchestration component. A child agent = a thin wrapper with Vivy child-run semantics |
| `middlewares/plantask` (task_*) | Equivalent exists (todos) | Do not switch or expose both; keep Vivy task_* |
| `middlewares/dynamictool/toolsearch` | No | Vivy already has tool_search (keep the existing capability as-is; do not use eino to expand the surface) |
| `middlewares/filesystem` large_tool_result | Internal component | Internal hygiene; evaluate adoption, but it does not constitute a product surface |
| Future eino middleware/components | — | Always filter through the "Crush surface" first: product-visible capability violates the guardrail; internal implementation details may be evaluated |

## 4. Impact on VC workload (correction to research §5)

- **VC-1 has the largest reduction**: the grep/glob/execute tool-definition layer, unified diff backend, `**` glob, and POSIX shell all become "import a library/connect middleware"; in-house work concentrates on multiedit, whitespace tolerance, filetracker/version chain, job registry, and approval-annotation glue. Rough estimate 2–3 weeks → **1.5–2 weeks**.
- **VC-2**: import the Anthropic claude component (established); hooks/headless/cost remain in-house; evaluate native eino to reduce retry/failover work. Rough estimate 2 weeks → **1.5 weeks**.
- **VC-3**: do not build the LSP library in-house (powernap); in-house work concentrates on diagnostic-backfill glue and the read side of the version chain. Rough estimate 3–4 weeks → **2.5–3 weeks**.
- The RB-1 (rollback) conclusion is unaffected: eino has no file-version primitive, so L1/L2 land on the Vivy side as described in §5.
