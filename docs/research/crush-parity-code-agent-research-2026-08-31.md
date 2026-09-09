# Crush Benchmark Research: Gaps and Roadmap for Vivy's CODE AGENT Route

- Date: 2026-08-31
- Research subject: `.workspace/crush/` (Charm Crush, Go terminal AI coding assistant, FSL-1.1-MIT)
- Comparison baseline: this repository's `internal/` (vivy.exe runtime) + `ui/` (Web UI), as of 2026-08-30 main
- One-sentence conclusion: **Feasible. Vivy's governance foundation (HITL approval / checkpoint / Journal audit / resident server) leads Crush in multiple dimensions; the real gaps are four areas—"coding tool surface + LSP + diff presentation + headless surface." Filling them across five stages, VC-0…VC-4, reaches feature parity without giving up Vivy's differentiated positioning.**

---

## 1. What Crush Is

Crush is Charm's terminal coding agent (TUI-first, with server mode added later), consisting primarily of:

- **Agent loop**: `charm.land/fantasy` unified LLM abstraction + `fantasy.NewAgent` loop; `StopWhen` has two conditions (token level triggers automatic summarization; more than 5 repeats of the "call+result" signature within a 10-step window declares a dead loop); stream-write each step to the database; queue/cancellation race handling (AcceptedRun/RunID); automatic re-authentication retry on 401.
- **27+ built-in tools**: bash / edit / multiedit / write / view / glob / grep / ls / fetch / download / web_fetch / web_search / agentic_fetch / sourcegraph / todos / question / job_output / job_kill / crush_info / crush_logs / 8 lsp_* tools / 2 MCP resource tools / agent (task subagent) / MCP wrapper tools. All tool descriptions are `.md` / `.md.tpl` template files that can be `go:embed`-ed.
- **LSP integration**: lazy-start manager + automatic discovery (powernap); after editing, feed LSP diagnostics back into tool results (the model directly sees lint errors), with 8 LSP tools.
- **MCP**: three transports—stdio / http / sse—+ OAuth 2.1 (including dynamic client registration); merge tools into the tool surface as `mcp_{server}_{tool}`.
- **Permissions**: allowlist (at `tool` or `tool:action` granularity) + persistent per-session authorization + yolo; bash has a 60+ read-only command allowlist (no prompt) and ~70 blocked commands (curl/sudo/…).
- **Hooks**: currently only one event, `PreToolUse`; protocol = stdin JSON payload + env injection + exit code (2 blocks the tool / 49 halts the whole turn) + stdout JSON envelope (`decision: allow|deny`, `reason`, `context`, shallow merge of `updated_input`); run in parallel, aggregate in configured order, execute **before** permission checks, and support Claude Code output format.
- **Sessions**: SQLite (dual modernc/ncruces drivers) + goose migrations + sqlc; parent/child sessions (subagent costs roll up to the parent session); file-version history table; filetracker records "which files each session has read and when" for stale-read protection.
- **Runtime forms**: TUI (Bubble Tea v2) / headless `crush run` / resident server (Unix socket / Windows named pipe + SSE event stream), with multiple clients sharing the same workspace (grouped by `--cwd`).
- **Skills**: Agent Skills open standard (agentskills.io), multi-path discovery (`~/.claude/skills`, `.agents/skills`, `.cursor/skills`…), 3 builtin skills (crush-config / crush-hooks / jq), and frontmatter support for `user-invocable` / `disable-model-invocation`.
- **Context files**: automatically inject workspace AGENTS.md / CLAUDE.md / CRUSH.md / .cursorrules and similar into the system prompt; global `~/.config/crush/CRUSH.md`; layer `.crushignore` over `.gitignore`.
- **Provider**: fantasy multi-protocol (anthropic / openai / openai-compat / bedrock / vertex / ollama / …) + automatic updates from the Catwalk remote model catalog (context window, pricing, reasoning metadata).
- **Configuration**: primarily `crushrc` (an embedded bash interpreter runs a configuration DSL consistently across platforms), with legacy compatibility for `crush.json`.

## 2. Vivy Current Baseline (Relevant to Coding Agents)

**Already present, and mostly stronger than Crush**:

| Capability | Vivy implementation | Crush comparison |
|---|---|---|
| Resident server + event stream | A resident gateway by nature (HTTP :8787 + WebSocket JSON-RPC `run/subscribe`), shared by Web/TUI/headless builds | Crush's later-added serve mode (one-way SSE); Vivy's architecture is more native |
| HITL approval | Three layers (sandbox `read_only/workspace_write/danger_full_access` × approval policy `ask/never/auto` × session switch) + persisted approvals + checkpoint recovery + automatic rejection on timeout | Crush has only allowlist + yolo, with no persisted approval recovery |
| Audit | Journal event sourcing (~40 RunEvent types persisted before push) | None |
| Budget | BudgetLedger circuit breaker, shared by parent/child and tightened without widening | None |
| Context compaction | In-run reduction + summarization + session-level CompactSession + UI meter | Token-level summary trigger (simpler) |
| Session storage | SQLite (16 migrations) / Postgres dual backend | SQLite-only backend |
| Resume | Checkpoint bridge + restart recovery (FR-8) | Replay session messages |
| Skills | Runtime skills_root + Eino skill middleware + skills marketplace (skills.sh) + prompt-injection scanning | Standard discovery + 3 builtins, no marketplace |
| Plugins | vivy-sdk pack compile-time plugin ABI (tool_world / channel seam) | None |
| Asking questions | Independent `ask_user` question stream + UI response | `question` tool (equivalent) |
| Todo | `task_*` five-piece set (persistent + dependencies) | Full replacement by `todos` |
| MCP client | Streamable HTTP (JSON-RPC + SSE decoding, session, Bearer, size limit, settings hot-reload) | Three transports + OAuth, no hot-reload |
| File tools | `read_file` (line-numbered) / `write_file` / `patch` (exact replacement) / `search_files` (literal) / `list_dir` | More complete (see §3 matrix) |
| Controlled execution | `execute` (basename allowlist, no shell composition, always approval) + three sandbox tiers | `bash` (embedded POSIX shell + background jobs) |
| Network | `http_request` (host allowlist + private-network rejection) + `network_search` (5-engine fallback) | `fetch` / `web_search` |

**Key gaps** (Crush has them; Vivy does not):

1. No regex grep, no independent glob, no multiedit; `patch` has no whitespace-tolerant fallback; no stale-read protection.
2. `execute` cannot run compound commands (pipes/`&&`/`git diff`), and there are no background jobs—a hard deficiency for a coding agent.
3. No LSP.
4. Neither UI nor TUI renders diffs or previews files; approvals have no inline diff highlighting.
5. No named subagent / `agent` tool (the underlying child run exists but is not exposed as a model tool).
6. No workspace context-file injection (AGENTS.md/CLAUDE.md).
7. Hooks have an interface (`ToolHookChain`) but no implementation and are not configurable.
8. No headless one-shot run surface (`vivy run "prompt"`)—FACE-0 is in TODO.
9. Anthropic-native provider is not wired (the catalog explicitly reports "not wired yet").
10. No monetary cost accounting (token statistics exist); model metadata (context window) is a hard-coded small table.
11. MCP uses HTTP transport only; no stdio and no OAuth.

## 3. Full Benchmark Matrix

Legend: ✅ aligned / 🟡 partially aligned / ❌ missing / ➖ not recommended to copy.

### A. Tool Surface

| Crush tool | Vivy current state | Decision | Track |
|---|---|---|---|
| `bash` (compound commands + background jobs + read-only allowlist + timeout-to-background) | `execute` (basename allowlist, no shell, always approval) | ❌ largest gap | VC-1 |
| `edit` (exact old/new replacement + whitespace-tolerant fallback + stale-read protection + diff in history) | `patch` (exact replacement, no tolerance, no protection) | 🟡 | VC-1 |
| `multiedit` | None | ❌ | VC-1 |
| `write` (stale-read protection) | `write_file` | 🟡 add protection | VC-1 |
| `view` (line numbers/images/skills virtual paths) | `read_file` (line numbers) | 🟡 (low priority for image reading) | VC-3 |
| `grep` (rg first, pure-Go fallback, respects .gitignore/.crushignore) | `search_files` (literal only) | ❌ | VC-1 |
| `glob` | Only the glob filter parameter of `search_files` | ❌ | VC-1 |
| `ls` | `list_dir` | ✅ | — |
| `todos` | `task_*` five-piece set (stronger) | ✅ | — |
| `question` | `ask_user` | ✅ | — |
| `job_output` / `job_kill` | None | ❌ | VC-1 |
| `fetch` / `web_search` | `http_request` / `network_search` | ✅ (stricter) | — |
| `agentic_fetch` / `sourcegraph` | None | ➖ optional | VC-4 |
| `lsp_*` ×8 + diagnostic feedback after editing | None | ❌ core to coding quality | VC-3 |
| 2 MCP resources / direct `mcp_{server}_{tool}` tools | `mcp_list_tools` / `mcp_call` (approval + proposal, indirect call) | 🟡 | VC-4 |
| `agent` (task subagent, child session + cost rollup) | Underlying child run exists, no model-visible tool | ❌ | VC-2 |
| `crush_info` / `crush_logs` | Partial `tool_search`; no self-diagnostic tool | 🟡 low priority | VC-4 |

### B. Agent Architecture and Loop

| Crush | Vivy current state | Decision | Track |
|---|---|---|---|
| Multi-agent coordinator (coder/task) | Single agent (ADK ChatModelAgent, Name always "vivy") | ❌ | VC-2 |
| System prompt template + GitStatus + ContextFiles injection | Static Instruction + per-run preamble | 🟡 | VC-1 |
| Context files (AGENTS.md/CLAUDE.md/global) | None | ❌ | VC-1 |
| Dead-loop detection (signature deduplication) | `MaxToolTurns` only | ❌ | VC-2 |
| Automatic token-level summarization | Compaction (reduction+summarization, stronger) | ✅ | — |
| 401 re-authentication retry | None | 🟡 low priority | VC-4 |
| Cost accounting (pricing metadata) | Token statistics without monetization | ❌ | VC-2 |
| filetracker + stale-read protection | None | ❌ | VC-1 |

### C. Governance (Vivy's lead; do not copy verbatim)

| Crush | Vivy | Assessment |
|---|---|---|
| allowlist + yolo + per-session authorization | three-layer sandbox/policy/session switch + persisted approvals + checkpoint recovery + timeout scheduling | ✅ ahead |
| Hooks: only PreToolUse, configurable shell scripts | `ToolHook` interface + empty chain, not configurable | ❌ Add configurable script hooks (VC-2), aligned with the Claude Code protocol |
| bash read-only command allowlist (60+ entries in safe.go) / blocked-command table | None | ❌ Introduce with VC-1 bash support (fits directly into Vivy's approval policy: allowlist=auto, everything else=ask) |

### D. Sessions / persistence

| Crush | Vivy | Assessment |
|---|---|---|
| SQLite + sqlc + goose | SQLite/Postgres + Journal event sourcing | ✅ ahead |
| Child sessions + cost roll-up to parent | child run has parent/child budgets, no cost roll-up | 🟡 (VC-2) |
| File-version history (view/restore old versions) | None | 🟡 low priority | VC-3 |
| `--continue` / `--session` resume | checkpoint + `session/*` RPC | ✅ |

### E. Runtime shape

| Crush | Vivy | Assessment | Track |
|---|---|---|---|
| TUI (Bubble Tea v2 + glamour rendering + themes) | `vivy tui` (--live fullscreen / --plain REPL, skeletal) | 🟡 | FACE-TUI-1/2 (existing TODOs) |
| headless `crush run` (stdin pipe, --continue, exit-code semantics) | None | ❌ | VC-1 (FACE-0 has an existing TODO) |
| server mode + multi-client workspace | resident gateway + bidirectional WS by design | ✅ ahead | — |
| diff rendering (tool metadata + TUI) | Neither UI nor TUI has a diff component | ❌ | VC-1 (UI) / VC-3 (TUI approval highlighting=FACE-TUI-2) |
| Desktop notifications / clipboard / self-update / telemetry | None | ➖ optional | VC-4 (do not copy telemetry: Vivy's local Journal is the audit trail) |

### F. Provider

| Crush | Vivy | Assessment | Track |
|---|---|---|---|
| fantasy multi-protocol support (native anthropic, etc.) | openai-compatible ✅ / anthropic native ❌ (not wired) | ❌ | VC-2 (use eino-ext/claude instead; see the §8.5 decision) |
| Catwalk model catalog (pricing/window/reasoning metadata) | hard-coded small context-window table | 🟡 Reuse its metadata structure, not the remote catalog | VC-2 |
| Local model discovery (ollama/lmstudio/…) | None | ➖ optional | VC-4 |

### G. Skills / configuration

| Crush | Vivy | Assessment |
|---|---|---|
| Agent Skills standard + multi-path discovery + builtin skills | skills_root + Eino middleware + marketplace + injection scanning | ✅ (could add builtin coding skills: git/vivy-code usage) |
| crushrc bash configuration DSL | settings RPC + live UI updates | ➖ Do not copy (settings is a better fit for a web product) |
| `.gitignore` + `.crushignore` hierarchical ignore | `list_dir`/`search_files` built-in exclusion list (.git/node_modules) | 🟡 VC-1 standardizes on gitignore awareness + `.vivyignore` |

## 4. Parts Vivy should not copy (differentiation)

1. **The three governance mechanisms are a moat, not baggage**: persisted approvals + checkpoint recovery + Journal auditing + budget circuit breakers, none of which Crush has. Making the agent coding-oriented should not weaken them; instead, bash's tiered policy ("read-only allowlist=auto, compound commands=ask") creates a finer-grained permission model than Crush.
2. **Resident server architecture**: Crush's serve mode was added later; Vivy is Web-first by design, so coding can reuse RunInspector/Review Center without becoming TUI-first.
3. **Configuration shape**: crushrc (bash DSL) solves cross-platform configuration for terminal users; Vivy has settings RPC + live UI updates, so copying it would be a step backward.
4. **Telemetry**: Do not introduce PostHog; the Journal is the local audit trail.

## 5. Roadmap (VIVY-CODE track; use VC-* numbering)

> Existing TODOs grouped here: FACE-0 (headless surface), FACE-TUI-1/2 (TUI/approval diff highlighting), SBX-OS/SBX-GLOB (sandbox upgrades), HITL-P1-* (proposal editing/scoped allow), CMP-1/2/3 (deeper compaction), TEST-1 (execute mock scenarios) — these are not new work; they are existing parts of this track.

### VC-0 Decisions and skeleton (about 0.5 weeks)

- Finalize the product shape: **the "code" face inside vivy.exe** (reuse the same runtime / Journal / approvals / skills), not a separate binary. FACE-0's `face` assembly is the entry point; UI Masks evolve from simple badges into a real run mode (tool surface + prompt injection switch with the face).
- Open the `docs/TODO.md` §0.1 VIVY-CODE track and attach VC-1…VC-4.
- Decision point: the boundary between bash tools and governance (see §6 risk 1).

### VC-1 Tool-surface parity (core, about 2–3 weeks) — minimum viable coding agent

1. `bash` tool: embedded POSIX shell (evaluate `mvdan.cc/sh/v3`, usable on Windows without WSL) + read-only command allowlist (port Crush's `safe.go` idea, allowlist→auto_approve, everything else→ask) + blocked-command table + head/tail output truncation.
2. Background jobs: `job_output` / `job_kill`, automatically move timed-out jobs to the background.
3. `grep` (rg first / pure-Go fallback) + `glob` tools, with gitignore awareness + `.vivyignore`.
4. `multiedit`; whitespace-tolerant `patch` fallback; stale-read protection for `write_file`/`patch` (add filetracker storage, record on read, validate before write).
5. Context-file injection: workspace AGENTS.md / CLAUDE.md / VIVY.md → run preamble; `vivy init` generates AGENTS.md.
6. UI diff rendering: generate unified diffs from write/patch/execute proposals and results (Crush uses go-udiff + additions/removals counts); render diff views in MessageBubble and Review Center; also close FACE-TUI-2's approval diff-highlighting gap.
7. Acceptance: complete the offline mock-provider e2e path "read code → grep → multiedit → run tests with bash → inspect diff" (add the TEST-1 execute scenario).

### VC-2 Loop and agentization (about 2 weeks)

1. `agent` (task subagent) tool: visible to the model and mapped to the existing child-run RPC; child session + cost roll-up to the parent run; subagents default to NonInteractive and do not trigger hooks (matching Crush semantics).
2. Infinite-loop detection: deduplicate the "call + result" signature over the latest N steps (Crush: window of 10 steps, >5 occurrences).
3. Cost accounting: model metadata table (context window / input-output prices / reasoning support) → monetize token statistics; add a cost column to the UI token panel.
4. Headless surface: `vivy run "prompt"` (stdin pipe, `--continue`, exit-code semantics) — land in FACE-0.
5. User-configurable hooks: `runtime.hooks.pre_tool_use[]` (matcher/command/timeout), aligned with Crush/Claude Code (stdin JSON + exit 2 block + stdout envelope + shallow merge of `updated_input`), running on the existing `ToolHookChain`; decisions still enter the Journal.
6. Anthropic-native provider wiring: **use the eino-ext `components/model/claude` component**, dropping the custom `vivy/anthropic` adapter milestone (see the decision and landing checklist in §8.5).

### VC-3 LSP coding intelligence (largest item, about 3–4 weeks)

1. LSP manager: lazy startup + match by file type/root marker + automatic discovery (gopls, typescript-language-server, pyright, etc.) + unavailable cache.
2. Post-edit diagnostics: append LSP diagnostic text to write/patch/multiedit results (the model sees lint/type errors directly — a key mechanism behind Crush's coding quality).
3. Land the `lsp_*` tool family in batches: diagnostics → definition/references/symbols → rename/replace_symbol (the latter goes through write approval).
4. File-version history (archive before editing, with view/restore support).
5. UI file preview + syntax highlighting; `read_file` supports images.
6. Note: put LSP code in `internal/lsp` or `internal/tools`; not importing Eino avoids the quarantine boundary. The implementation can learn from Crush's powernap wrapper; subject to license compliance, evaluate direct use of `charmbracelet/x/powernap`.

### VC-4 Ecosystem and productization (ongoing)

- MCP: stdio transport + OAuth 2.1 + resources (list/read_mcp_resource) + `mcp_{server}_{tool}` direct tools (reuse Vivy's approval annotations).
- Sandbox upgrades (existing SBX-OS/SBX-GLOB items) + editable bash deny globs for auto_approve.
- Builtin coding skills (git workflow / vivy-code usage / just-ci).
- Local model discovery (ollama, etc.); 401 re-authentication retry; desktop notifications; a `crush_info`-style self-diagnostic tool.
- Explicitly out of scope: crushrc DSL, the remote Catwalk catalog, PostHog, and copying the TUI theme system.

## 6. Risks and decision points

1. **bash tools vs. governance philosophy (greatest tension)**: Vivy's current execute is a basename allowlist + always-approve; introducing real bash opens compound shell commands. Suggested tiers: read-only allowlist=auto, workspace writes=ask, blocked table (sudo/curl|sh, etc.)=deny directly in the policy-engine rules (policy.go already supports per-tool/field rules). Keep the sandbox directory constraint as a backstop.
2. **Embedded shell choice**: `mvdan.cc/sh/v3` (BSD-3, the same choice as Crush) is a mature way to run POSIX syntax on Windows without WSL; compatibility with Vivy's sandbox path constraints must be verified.
3. **LSP library license**: powernap is MIT (Charm) and usable; a minimal hand-written client (jsonrpc2 + a small number of methods) would be about 1–2k lines as a fallback.
4. **Anthropic wiring**: ~~license risk~~ has been verified clean (anthropic-sdk-go MIT, aws-sdk-go-v2 Apache-2.0; the P3-1 claude-code upstream is unrelated to this adapter). The path is decided: use the eino-ext `components/model/claude` component instead of building a custom adapter (see §8.5).
5. **UI diff effort**: use a mature diff-view component rather than writing one by hand; backend diff generation (unified diff + additions/removals counts) is small.
6. **Crush is FSL-1.1-MIT**: its source can be studied, but **code cannot be copied directly** into Vivy (FSL is not OSI open source and converts to MIT only after two years). Every "port" in the roadmap means behavioral/protocol parity with a fresh implementation. This must be written into the acceptance note for every VC task.

## 7. Feasibility conclusion

- **Feature parity is feasible**: the gap is concentrated in the tool surface and presentation layer, with no architectural blocker; Vivy's ADK loop, compaction, approvals, and storage are more complete than or equivalent to Crush's counterparts.
- **Recommended order**: VC-1 (tool surface) → VC-2 (loop) is the threshold for "can it do coding work?"; VC-3 (LSP) separates "does it do it well?"; VC-4 as needed.
- **Total effort**: VC-0…VC-2 can reach "Crush core-experience parity" in about 5–6 weeks (bash/grep/glob/edit family + diff presentation + subagent + headless + hooks + cost); VC-3 adds 3–4 weeks for "LSP-enhanced coding" parity.
- **Vivy's end-state positioning** is not "another Crush" but a **strongly governed coding agent**: it can do the same work, while every file write, shell command, and cent has approval, audit, and budget controls — capabilities Crush lacks and Vivy already has.

---

## 8. Second-pass verification addendum (same day; corrections and omissions after a three-way source review)

### 8.1 Corrections to earlier conclusions

| Earlier wording | Verification result |
|---|---|
| "server mode (unidirectional SSE)" | The command is `crush server` (not `serve`); the client/server architecture is enabled by `CRUSH_CLIENT_SERVER`; TUI is only one client; server includes complete OpenAPI documentation (`internal/swagger`); **only local Unix sockets / Windows named pipes, with no authentication layer**. Includes stale-server version negotiation, spawn flock single-flight, and `/v1/health` readiness probing. |
| "multi-agent coordinator (coder/task)" | Only two **hard-coded** agents (`SetupAgents`); `Config.Agents` is marked `json:"-"` — **users cannot define named agents in configuration** (unlike the old Crush agents configuration). |
| (not expanded) task subagent tool surface | Read-only subset: `glob/grep/ls/view/lsp_definition/lsp_symbols/lsp_call_hierarchy/sourcegraph`; empty `AllowedMCP` map = **no MCP**; prompt is only 16 lines and is unaware of skills/context files/git. |
| (easy point of confusion) compact | Crush **has no `/compact` command**; `compact_mode` is a compact TUI layout and unrelated to context compaction. The summary entry point is the command palette's "Summarize Session" plus automatic triggering at the token threshold (see §3B). |
| (additional) theme system | There are actually only 2 themes (Pantera default / Hypercrush brand), mapped by provider. Fewer than the first-pass impression suggested. |

### 8.2 First-round omissions

**A. CLI machine interface surface** (`internal/cmd/`)

- `crush session list/show/last/delete/rename`: complete `--json` machine interface, with cost / tokens / **loaded skills metadata** / full message parts in the output; hash-prefix parsing + ambiguous candidates; `show` uses the same renderer as TUI + pager.
- `crush stats`: **self-contained HTML dashboard** (not a terminal table) — aggregates by day/model/hour/weekday, average response duration, **tool-call counts** (extracted from messages.parts with `json_each`), and a heat map; `--crawl-dir`/`--all` aggregates across projects.
- `crush projects`: `projects.json` project registry (path / data_dir / last_accessed, supporting stats --all).
- `crush models`: one `provider/model` per line when not on a TTY (machine-readable and consumable by `crush run --model`).
- `crush login/logout`: Hyper and GitHub Copilot **device-code OAuth flows**; Copilot can import an existing token from VS Code `apps.json`.
- `crush logs --follow/--tail`, `crush dirs` (visualize configuration directories), `crush update-providers --source=catwalk|hyper`, and `crush schema` (hidden; dynamically injects provider-type enum entries, including locally registered providers).

**B. TUI interaction surface** (the coding agent's "feel and safety net", `internal/ui/`)

- **bang mode `!`**: execute shell directly — execute on the server, stream the echo as chat items, interrupt with esc, add to prompt history, and **persist in session records**.
- **Message queueing**: prompts automatically queue while the agent is busy (queue pill shown); esc is two-stage (clear queue → press again to cancel).
- **Attachment path**: clipboard image / `ctrl+f` file picker / pasted image path / `@` file completion (including **MCP resource completion**); 5MB limit and image-type allowlist; **image support gated by model metadata `SupportsImages`**, with a provider workaround for tool results carrying images (only anthropic/bedrock allow images in tool results; others downgrade to a placeholder + a FilePart added to the user message).
- Three-state reasoning view (collapsed → trailing 200 lines → fully expanded); REFUSED banner.
- **Unified/split diff viewer modes** (the permissions dialog switches with `options.tui.diff_mode`).
- Four notification backends (native / OSC99 / OSC777 / bell) + notify only when unfocused; render images with the Kitty graphics protocol; indeterminate terminal progress bar; ANSI16 output remapping; switch to yolo while running with `ctrl+y`; external `$EDITOR` with `ctrl+o`.
- Command palette includes one-click start/stop for Docker MCP Catalog (automatic discovery).

**C. Prompt templates and behavior rules** (`internal/agent/templates/`, 8 templates — this is the "agent behavior" layer)

- `coder.md.tpl` key rules: **never commit unless the user explicitly says so** (commits follow the attribution format); default output <4 lines, no emoji; references use `file:line`; **LSP-first editing strategy** (replace_symbol/rename before text edit); at least 2–3 recovery strategies for errors; bash's description parameter is required; do not use bash to run curl (use fetch); parallel tool calls.
- Runtime injection: git branch / status(--short head 20) / log(-3) snapshots, platform, date, context files rendered as `<project_context>`/`<user_preferences>`, and the skills XML directory (`crush://skills/...` virtual paths read natively by the view tool).
- `initialize.md.tpl`: generates `options.initialize_as` (AGENTS.md by default); rejects empty directories; detects existing rule files such as `.cursor/rules`, `.cursorrules`, and `.github/copilot-instructions.md`; principle: "record only non-obvious knowledge".
- `title` generation: small→large fallback chain, `/no_think` + empty `<think></think>` anti-thought-leak technique, 40-token limit, shell sessions named `"$ cmd"`, and title usage also billed.
- `summary.md`: fixed 5 sections (Current State / Files & Changes / Technical Context / Strategy & Approach / Exact Next Steps); on **resume, change the summary message role to User, truncate all prior history, and zero PromptTokens**; if automatic summarization interrupts a turn containing a tool call, requeue it with a rewritten prompt.
- Inject an empty-todo reminder in `<system_reminder>` on the first user message; sanitize invalid JSON tool arguments to `{}` and return an error message.

**D. Provider / cost / cache details** (the hidden bulk of parity)

- **Anthropic prompt caching**: mark the last system message + last 2 messages with ephemeral `cache_control`; `CRUSH_DISABLE_ANTHROPIC_CACHE` switch; `x-session-id` / `x-session-affinity` session-affinity headers.
- **Large reasoning/thinking parameter-mapping switch** (`coordinator.go`, ~200 lines): openai `reasoning_effort`, anthropic `thinking{budget_tokens}`, google `thinking_config`, openrouter `reasoning{enabled,effort}`, plus provider-specific `extra_body` cases for ZAI/DeepSeek/Fireworks/MiniMax/Alibaba/Baseten and others.
- Four-part pricing formula: cache_creation / cache_read / input / output; **estimated usage is forced to zero cost**; OpenRouter's response `usage.cost` overrides local pricing + the `:exacto` model suffix; Hyper balance is read from a response-metadata side channel.
- 15 provider types; `aws_auth_refresh` (run a command and retry in place when Bedrock credentials expire); `flat_rate`; `system_prompt_prefix`.
- `discover_models`: five local enrichers (ollama / omlx / lmstudio / llamacpp / litellm) probe their endpoints to fill in context window and other data, **only filling zero-valued fields** (explicit user configuration wins); litellm is the only one that fills pricing.
- **OAuth token refresh uses cross-process flock single-flight**, with refresh-token rotation to prevent revocation (adopt the peer's new token / retry with the peer's new refresh_token); 401 → three OnAuthRefresh branches (OAuth refresh / AWS SSO / reparse the API-key template); revoked refresh token → block while waiting for interactive re-login.

**E. MCP superset** (not just tools)

- **prompts capability** → automatically wrap as command-palette entries (send the retrieved text directly as a user message).
- **resources capability** → built-in `list_mcp_resources` / `read_mcp_resource` tools + change-notification listener.
- **Experimental channels**: hidden flag `--channels server:webhook`; an MCP server can actively push `claude/channel` events into a session.
- Docker MCP Catalog automatic discovery and registration; no sampling support.

**F. Session / data / engineering governance**

- **Per-session file-version history** (`internal/history`, SQLite version chain) — lightweight undo infrastructure paired with filetracker.
- **Configuration hot reload** (crushrc / hook / model each support reload, copy-on-write + pubsub).
- **Embedded gojq**: no external jq binary needed in the bash environment (benefits Windows).
- **VCR test infrastructure** (`charm.land/x/vcr` records and replays LLM interactions) — directly reusable testing strategy.
- herdr terminal multiplexer status reporting (Unix-socket JSON-RPC: idle / working / blocked, best-effort and non-blocking).
- Android/Termux compatibility (DNS resolver replacement); env/flag surface including `CRUSH_SERVER_READY_TIMEOUT` and `--channels`.

### 8.3 Capabilities Crush explicitly lacks (= Vivy's differentiation/opportunity list)

- No session sharing/export; no ACP (mentioned only in comments as planned); no IDE extension (only Copilot token import); no manual compaction command (Vivy's `CompactSession` RPC is **stronger** here); no watch mode; no cron/scheduled tasks (Vivy already has cron_scheduler); no multi-root workspace; no MCP sampling; no remote authentication layer for server (local socket only); no hard-deny permission layer ("visible but always refused", planned in FUTURE.md — Vivy's policy deny is implemented).
- Roadmap exposed by official `docs/*/FUTURE.md`: agent edits runtime configuration live (with permission gating), separates `state.json` state/configuration, returns hook `UserPromptSubmit` events and `context_files`, and offers `include_sub_agents` subagent hook opt-in.

### 8.4 Additions to the VC roadmap

**VC-0/VC-1 additions**:

- The code-face system prompt directly follows coder.md.tpl's rule set (never commit without permission, <4-line default output, file:line references, LSP-first editing, multiple error-recovery strategies) — this is zero-code "behavioral parity".
- Merge filetracker and file-version history into **one storage design** (both are read_files/versions family tables).
- UI attachment baseline: image paste/upload enters the message path (composer attachment stub already exists), with image support gated by model metadata + provider workaround for tool results carrying images.
- Message queueing + two-stage cancellation (UI safety net corresponding to Crush's queue pill / esc semantics).

**VC-2 additions**:

- Execute the Anthropic wiring per the §8.5 decision (eino-ext/claude component); acceptance items: prompt caching works (record the component breakpoint strategy = system + tools + last message, with the known difference from Crush's system + last 2 messages), thinking/reasoning parameters, and four pricing data sources (`CachedTokens` / `GetCacheCreationInputTokens`).
- Automatic session titles (small→large fallback chain).
- Narrow the subagent (agent tool) surface to a read-only subset + no MCP according to Crush semantics — consistent with Vivy worker's PolicySnapshot philosophy and directly mappable to implementation.
- Session machine interface: Vivy RPC already covers most of it; add equivalent `--json` fields for cost / skills / message parts; add cache-hit and cost dimensions to the token statistics panel (matching the crush stats field set).

**VC-3 additions**:

- Diff component selection must support unified/split modes; reasoning blocks have three collapsed states.

**VC-4 additions**:

- MCP resources (list/read tools) + prompts (map into Vivy's command/skill system); list channels as an experimental observation item, not urgent.
- Custom commands (markdown + `$NAMED` parameters) and user-invocable skills merge into one mechanism in Vivy (the skills marketplace is already a strength).
- When `vivy init` generates AGENTS.md, use the initialize-template principles (record only non-obvious knowledge, detect existing .cursor/copilot rule files).
- Test infrastructure: evaluate VCR-style LLM recording/replay and align it with the existing scriptedmodel mock.
- Active differentiators (which Crush lacks): cron, policy hard-deny, manual CompactSession, and potentially server authentication and remote multi-client support — selling points rather than burdens for coding.

### 8.5 Decision record: switch the Anthropic backend to eino-ext/claude (decided 2026-08-31)

**Decision**: drop the custom `vivy/anthropic` adapter milestone and wire Anthropic through `github.com/cloudwego/eino-ext/components/model/claude`; keep the eino-ext openai component unchanged for the OpenAI-compatible surface (gateway/DeepSeek/ZAI/Kimi/custom providers); do not introduce dedicated deepseek/qwen/gemini components.

**Decision basis** (verified against source that day):

1. **The original premise is outdated**. The `internal/provider/bundle.go` comment and `fixtures/provider/anthropic.yaml` provenance state the custom-build reason as "no official Eino Anthropic component exists" (recorded 2026-08-07); eino-ext now has `components/model/claude`, whose go.mod depends on eino v0.9.1 and is compatible with vivy's locked v0.9.13 within the same minor version.
2. **The component directly satisfies VC-2 acceptance items**: `AutoCacheControl` (automatic cache_control breakpoints for system/tools/trailing messages, TTL 5m/1h, plus manual `SetMessageBreakpoint`/`SetToolInfoBreakpoint` breakpoints); usage reporting through `CachedTokens` + `GetCacheCreationInputTokens` (four pricing data sources); round-trip thinking-block signatures (`WithThinking`/`GetThinking`+signature); native Bedrock (AWS credential chain/profile) and Vertex (service-account JSON) support; Anthropic protocol strictness handling such as `mergeAdjacentToolResults`; and exponential 429/5xx backoff built into anthropic-sdk-go.
3. **The license is clean**: anthropic-sdk-go MIT, aws-sdk-go-v2 Apache-2.0. P3-1 (the claude-code upstream LICENSE) is unrelated to this adapter and is no longer a prerequisite.

**Architectural impact**: the blast radius is limited to internal/provider — the `Ref` seam (`ref.go`, returning `model.ToolCallingChatModel`) is unchanged, and runtime/modeladapter/ADK/mock are unaffected; Eino quarantine (only internal/runtime and internal/provider may import eino) is unaffected.

**Implementation checklist** (in VC-2):

1. Add `claudeRef` (following the `openai.go` template, mapping `ModelSpec{ID, APIKey, BaseURL}` → component `Config`); add `BackendEinoClaude` to the backend switch in `catalog.go`; add the backend enum value to `schemas/providers.bundle.schema.json` — the bundle schema is a product contract (D-022..D-025, strict parsing), so follow the rule with `just ci` + an outbound `model`-field assertion test.
2. Correct stale records: the comment in `bundle.go`, the `anthropic.yaml` provenance note, and the `catalog.go` comment containing "no official Eino Anthropic component exists".
3. D-010 protection: `claudeRef` first raises `KeyMissingError` when the key is empty (matching the openaiRef pattern), and Model is always passed explicitly; add a test asserting that the component's `ANTHROPIC_API_KEY` / `ANTHROPIC_MODEL` environment-variable fallback paths are not triggered.
4. MaxTokens: required by the Anthropic protocol, while the current vivy openai path treats "0 = API decides"; add MaxTokens to `ModelSpec` or give claudeRef a model-level default to avoid sending 0 and failing.
5. Dependency surface: the component unconditionally imports bedrock/vertex branches, so aws-sdk-go-v2, Google auth, and related dependencies enter go.sum and the binary even when unused; licenses are clean, and the supply-chain audit goes in this iteration's verification.md.
6. Record the cache-strategy difference: component automatic breakpoints = system + tools + last message; Crush = system + last 2 messages. Both are within Anthropic's four-breakpoint best practice; note this during acceptance.

**Explicitly not doing**: introduce dedicated eino-ext deepseek/qwen components for OpenAI-compatible vendors (the openai component + BaseURL already cover them and are shorter); evaluate a native gemini component only when there is real demand (the Google GenAI SDK dependency is heavy).

### 8.6 Decision record: capability layers and persona model (decided 2026-08-31, user)

The following two corrective decisions for §3/§5 take effect with the VIVY-CODE track (`docs/TODO.md` §0.1 VC-0..VC-4):

1. **Capability layers (mainline kernel / code face / plugin)**: the base tool surface (bash, background jobs, grep/glob, multiedit, patch tolerance, stale-read protection, context-file injection, etc.) is **mainline kernel capability**, synchronized with the mainline rather than code-face-specific — correct §5's placement of all VC-1 work under the code-face roadmap accordingly. Code-specific capabilities (LSP, etc.) are **optional** on the mainline, so **LSP is a plugin** and does not enter the default EXE surface. Assign every VC task to one of the three layers before taking it up.
2. **Persona model (no Crush coordinator)**: Vivy **explicitly will not** implement Crush's named agents / coordinator (the hard-coded coder/task pair). Vivy's persona model follows diva: **one primary supervisor persona** that can wear a mask (mask/face); the mask does not affect the vivy kernel (governance/approvals/auditing remain in the kernel). A subagent (a child run triggered by the `agent` tool) is a **masked, kernel-less, context-clean** task-scoped worker that carries neither the primary persona's kernel state nor the parent session history. Rewrite the §3B "multi-agent coordinator ❌→VC-2" line accordingly: what is needed is not a coordinator but masked subagents.

### 8.7 Decision record: VC decision list D1..D11 (decided 2026-08-31, user)

| # | Decision | Ruling |
|---|---|---|
| D1 | code-face product shape | Per research §5: "code" face inside vivy.exe, reusing runtime/Journal/approvals/skills, not a separate binary |
| D2 | FACE-0 | Adopt `VIVY-FACE-PACK.md` (`face: web\|tui\|headless` as first-class assembly + user `seam: face`); close TODO §0.1 FACE-0 accordingly |
| D3 | bash governance boundary | Tier per research §6.1: read-only allowlist→auto_approve, everything else→ask, blocked table (sudo/curl into shell, etc.)→deny in the policy engine |
| D4 | LSP assembly shape | Standalone vivy-sdk module plugin (tool_world seam), no LSP in the default EXE, following the CH-C4..C7c precedent "default EXE does not link protocol SDKs" |
| D5 | Subagent boundary | Approvals join the parent session (no HITL exemption); mask permissions follow diva and remain editable |
| D6 | Context-file injection | Read-only AGENTS.md (do not introduce multi-file CLAUDE.md/VIVY.md precedence) |
| D7 | Ignore contract | Use .gitignore; do not introduce .vivyignore |
| D8 | Hook governance | Hook configuration changes enter the Journal; first registration of a hook script requires ask |
| D9 | Cost/model metadata | Synchronize with the web side (provider/model management); do not create another data source |
| D10 | UI diff presentation | Match Crush diff behavior (unified/split modes + additions/removals counts); write a fresh implementation under the FSL constraint |
| D11 | Headless surface | Follow D2: land `vivy run` in face assembly (FACE-0 adopted) |

**Additional rulings (same day)**:

1. **vivy code has no channel**: code-face assembly excludes channel ears; channel remains a mainline (web) capability.
2. **ACP is not being done**: reclassify TODO §0.1 ACP-1 from DEFERRED to WONT-DO.
3. **Rollback research initiative (RB-1)**: research "how rollback works and whether code rollback is actually supported" — checkpoint bridge (FR-8 session recovery) vs. file-version history (VC-3 archive before editing) vs. git semantic rollback; output = whether Vivy promises code rollback capability and where it belongs.

Note: D4's pluginization turns the integration of "post-edit diagnostic injection" (how plugin diagnostics join kernel write/patch tool results) into an implementation design point; see TODO §0.1 VC-3.
