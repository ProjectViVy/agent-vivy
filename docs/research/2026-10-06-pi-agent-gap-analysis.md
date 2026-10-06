# VIVY CODE vs Pi Agent — Capability Gap Analysis

Date: 2026-10-06
Source: `earendil-works/pi` @ `23cf2b9` (formerly badlogic/pi-mono), `ProjectViVy/agent-vivy` @ `f34f3ce`
Standard: VIVY CODE must reach 100% of pi agent's capability surface.

## 1. Executive summary

Pi is a **minimal, self-extensible agent harness**: a thin core (~agent loop + 8 builtin tools) surrounded by a runtime extension system (TypeScript extensions, skills, prompt templates, themes, MCP, custom providers, virtual models) and a polished terminal UX. Its strength is *surface area through runtime extensibility*.

VIVY is a **gateway/species system**: a durable kernel (Journal + writer fencing + run state machine on Eino ADK) with channels, faces, HITL approvals, a pack/generation distribution model, and a Bubble-Tea TUI face (`vivy-code`). Its strength is *kernel durability, governance, and product breadth*.

Verdict: VIVY already exceeds pi in the **kernel/governance plane** (durable journal, HITL, sandbox policy, plan mode, subagents, channels, cron, observability). The gap to 100% pi parity is concentrated in three planes:

| Plane | Parity estimate | Main gap |
|---|---|---|
| Agent engine + tools | ~75% | steering mid-turn, tool-exposure model, codemode/tool_search meta-tools, overflow recovery depth |
| Model/provider plane | ~45% | OAuth login, 7-level thinking, virtual models/router, native API coverage (Responses/Bedrock/Vertex/Azure), classifier+image models, live catalog, cache warming, llama.cpp |
| Session/context plane | ~55% | tree navigation, branch summaries, clone/import/export/share, JSONL portability, compaction extras |
| CLI + embedding | ~25% | no CLI flags, no print/JSON/RPC subprocess modes, no embeddable session SDK |
| Extensibility runtime | ~20% | no runtime extensions, no event hooks, no user slash-templates, no packages ecosystem |
| TUI surface | ~60% | no themes, no custom keybindings, no inline images, no transcript search, no external editor, less chrome |

Overall weighted parity: **~45–55%**. Sections below give the per-feature matrix.

Legend: ✅ present · 🟡 partial/weaker · ❌ missing · ➕ VIVY exceeds pi

## 2. Agent loop and tool plane

| Capability | Pi | VIVY | Status |
|---|---|---|---|
| Streaming agent loop w/ tool calls | Yes, pi-agent-core | Eino ADK engine | ✅ |
| Parallel tool calls per message | Yes | Yes | ✅ |
| Steering messages mid-run (inject after current turn) | `Enter` during work | Queue only; no interject found in `internal/rpc/control.go` | ❌ |
| Follow-up queue + dequeue back to editor | `Alt+Enter`, `Alt+Up`, `steeringMode`/`followUpMode` | `queuedTurn` local queue, `/queue clear` | 🟡 queue yes, no steering/dequeue split |
| Abort run | Esc | `/cancel`, gate handling | ✅ |
| Agent-level retry w/ backoff + provider retry knobs | `retry.*` settings | Some failover (titler/route); no documented retry settings | 🟡 |
| Context-overflow recovery (compact+retry on provider error / stopReason length) | Yes | Auto-compaction on `TriggerPercent` exists; provider-error-triggered recovery not observed | 🟡 |
| Builtin file tools | read/write/edit/grep/find/ls | read_file/write_file/patch/multiedit/search_files/glob/list_dir | ✅ (patch+multiedit richer) |
| Shell tool | `bash`, `powershell`, env injection (`PI_SESSION_ID`, `PI_MODEL`…), `shellPath`, `shellCommandPrefix`, aliases file, 1MiB capture to codemode, truncated→file | `bash`/`commandline`/`execute` w/ governed classifier + deny-globs | 🟡 governance ➕, env-injection + output-to-file ❌ |
| `!` / `!!` user shell input | Yes | `!script` → server shell seam | 🟡 no `!!` (run-without-context) variant |
| Web tools | none builtin (via MCP/extensions) | `network_search`, `web_fetch`, `http_request`, `download` | ➕ |
| Task/notes tools | via extensions | `task_*`, `write_note/read_note/list_notes` | ➕ |
| Plan mode | deliberately absent | `enter_plan_mode`, `submit_plan`, `*_goal` | ➕ |
| Subagents | deliberately absent | `agent`, `child_inbox`, `reply_parent` | ➕ |
| Workflow orchestration | none | `workflow` tool | ➕ |
| Background jobs | none | `job_output`, `job_kill` | ➕ |
| Tool exposure model (direct/model-only/codemode/deferred/hidden + namespaces + MCP annotations) | Full model | Flat enabled list in config | ❌ |
| Dynamic tool activation + `tool_search` (BM25 discovery→declare) | Builtin ext | None | ❌ |
| `codemode`: sandboxed JS that calls tools + non-LLM models, script store, `models.classify/generateImages` | Builtin ext (QuickJS) | None | ❌ |
| Nested tool calls (`ctx.executeTool`, parentToolCallId, nestedCalls record) | Yes | Agent tool pattern only | 🟡 |
| Custom tools without rebuild | `pi.registerTool()` (TS, TypeBox, outputSchema) | Go module compile/pack-time only | ❌ |
| Tool renderers (custom TUI/HTML render per tool) | `renderCall`/`renderResult`, `registerToolRenderer` | Fixed surface renderers | ❌ |
| File-mutation serialization | `withFileMutationQueue()` | Not observed (Journal handles durability) | 🟡 |

## 3. Model/provider plane

| Capability | Pi | VIVY | Status |
|---|---|---|---|
| Builtin providers | ~35+ incl. subscription providers (Anthropic, OpenAI/Codex, Copilot, ZAI, OpenCode, Radius…) | 46 vendors in `vendors.yaml` (mostly CN mirrors + majors) | 🟡 count ok, mostly one protocol family |
| Wire protocols | Anthropic Messages, OpenAI Completions **and Responses**, Azure Responses, Google GenAI/Vertex, Bedrock Converse, Mistral Conversations | OpenAI-compatible + Anthropic endpoint (`deepseek/anthropic`) | ❌ Responses/Bedrock/Vertex/Azure native |
| OAuth subscription login (`/login`, device/browser flow, headless paste-back, refresh) | Yes | None — env keys + UI registry only | ❌ |
| `auth.json` credential store + `!command` key resolution + precedence chain (`--api-key` > auth.json > models.json > env) | Yes | env keys / config only | ❌ |
| Custom endpoints | `models.json` + `modelOverrides` | vendors.yaml + provider registry UI | 🟡 equivalent reach, different UX |
| Live model catalog (pi.dev overlay, offline cache, `pi update --models`, fuzzy model search) | Yes | Static vendored yaml | ❌ |
| Thinking levels | 7 (`off…max`), per-model clamp, per-model defaults, `thinkingBudgets`, `samplingParamsByThinkingLevel` | `auto|on|off` | ❌ |
| Model picker UX | `/model` fuzzy picker, `Ctrl+P` cycle, `/scoped-models`, save default | `/model [filter]` picker | 🟡 no cycling/scoped/save-per-model |
| Virtual models (per-request router w/ state, sticky retry routing, physical↔virtual split) | `pi.registerVirtualModel` | `modelhost_routing` covers failover/route, not user-visible virtual models | ❌ |
| Classifier models (Jev/Clef via codemode + `modelRegistry.classify`) | Yes | None | ❌ |
| Image-generation models (via codemode + `generateImages`) | Yes (OpenRouter etc.) | None (media pipeline is inbound only; CH-MEDIA-N1 open) | ❌ |
| Local models | llama.cpp router integration + compatible endpoints | `vllm` vendor entry only; ollama discovery is OPEN (VC-4) | ❌ |
| Prompt-cache warming (`cacheWarming`, lifetime metadata, miss notices) | Yes | None | ❌ |
| Usage/cost accounting | Per-model usage+cost, footer, `/session`, cache tokens | `/stats`, tokenstats, sidebar; CN-priced | ✅ (parity, TUI-USAGE-ACCOUNTING open edge) |
| Embedding/sampling params per request | `samplingParams`, per-level overrides | Not observed | ❌ |

## 4. Session/context plane

| Capability | Pi | VIVY | Status |
|---|---|---|---|
| Session storage | JSONL tree files, `id/parentId`, v3 + migration | SQLite/Postgres Journal, writer fencing | ➕ durable, ✖ portability |
| Resume/continue | `--continue`, `--resume`, picker, `/resume`, `/new`, `/name` | `/sessions`, `/session <id>`, `/new`, `/rename`, `/delete`, newest-on-launch | ✅ |
| Session tree browser (`/tree` w/ filters, labels, fold, prompt jump) | Yes | `/fork <msg_id>`, `/rewind` only; no tree UX | ❌ |
| Fork/clone/import | `/fork`, `/clone`, `/import <jsonl>` | `/fork`, rewind; no clone/import | 🟡 |
| Branch summarization on navigation | Auto summary of abandoned branch | None | ❌ |
| Auto-compaction | threshold `reserveTokens`, `keepRecentTokens`, per-model overrides, split-turn handling, iterative summary w/ file tracking (read/modified lists), `/compact [instructions]`, compaction hooks | `TriggerPercent`/`KeepRecent`/`MaxTokens`, `/compact` | 🟡 core yes; split-turn/file-tracking/instructions/hooks ❌ |
| Context files | AGENTS.md/CLAUDE.md chain w/ overrides, ancestors walk, agent-dir files, `SYSTEM.md`/`APPEND_SYSTEM.md` | AGENTS.md via eino middleware (@imports, transient) | 🟡 AGENTS only (by design D6), no CLAUDE.md/override/SYSTEM.md |
| Skills | Agent Skills spec, user+project+packages, `/skill:name`, `disable-model-invocation`, `allowed-tools` | skillhost + marketplace (skills.sh), `/skills`, budgets, mount provenance | ✅ parity-ish (different distribution) |
| Prompt templates → user slash commands w/ arg substitution | `~/.pi/agent/prompts/*.md`, `$1`, `${@:N:L}` | None (fixed command registry) | ❌ |
| Session export (HTML/JSONL), share link, `/copy`, `/bug` report, `/debug` dump | Yes | None | ❌ |
| Session name → title generation | Manual `/name` | Auto titler model | ➕ |
| Cross-project sessions / session-dir override | `--session-dir`, grouping by cwd, cross-project fork offer | Per-instance private journals | 🟡 |

## 5. Interfaces and embedding

| Capability | Pi | VIVY | Status |
|---|---|---|---|
| Interactive TUI | Fullscreen+regular modes | Fullscreen Bubble Tea | ✅ |
| Print mode (one-shot, stdout) | `-p`/`--print` | `vivy run` headless face exists but no CLI flag (vivy-code accepts no args; `vivy --help` broken per TODO) | ❌ |
| JSON event-stream mode | `--mode json` → JSONL events | None | ❌ |
| RPC subprocess mode (JSONL stdin/stdout, ~40 commands) | `--mode rpc` + `RpcClient` | JSON-RPC control plane over HTTP/WS (`vivy.rpc.v1`) | 🟡 different transport; covers control, not drop-in subprocess embedding |
| In-process embeddable agent SDK (`createAgentSession`, SessionManager, custom resource loader/tools/providers) | `@earendil-works/pi-coding-agent` | vivy-sdk builds Generations/packs; no "embed a session" Go API surface | 🟡 different model |
| Browser/web UI | via community (OpenClaw) or web-ui components | Embedded React UI + Studio IDE | ➕ |
| Channels (Slack/IM bots) | pi-mom (Slack only) | feishu/telegram/discord/qq/dingtalk + governance | ➕ |
| Multi-face/multi-client attach to one kernel | Experimental pi-server/protocol (CBOR) | Production RPC control plane | ➕ |
| Headless/scriptable face | print/JSON/RPC | headless face module | 🟡 |

## 6. Extensibility runtime — the structural gap

Pi's defining capability: **runtime TypeScript extensions** (`ExtensionAPI`) — `pi.registerTool/Command/Shortcut/Flag/Provider/McpServer/VirtualModel`, ~40 lifecycle/context/tool/provider/compaction/trust/queue events, `ctx.ui` (select/confirm/input/editor/notify/status/widget/custom components), `appendEntry` durable state, `sendMessage` custom messages, renderer registration, hot `/reload`. Plus **Pi packages** (npm/git/local) bundling extensions+skills+prompts+themes.

| Pi capability | VIVY nearest | Status |
|---|---|---|
| Runtime-loadable extension modules (no rebuild) | Go plugins are compile/pack-time (Module→Port→Recipe→Generation) | ❌ architectural |
| Event/hook system (~40 event types incl. `before_agent_start`, `tool_call` block/mutate, `context` transform, `session_before_compact`, `provider_stream_event`, `cache_warming_decision`) | Internal events/observerhost — internal, not an extension API | ❌ |
| Register commands/shortcuts/CLI flags at runtime | Fixed registry | ❌ |
| `ctx.ui` interactive components over TUI+RPC | HITL gates only | ❌ |
| Custom renderers per tool/entry | Fixed | ❌ |
| Custom session entries (`appendEntry`) | Journal internal | ❌ |
| Packages ecosystem (install npm/git bundles) | skills marketplace only | ❌ |
| Hot reload of resources | `/reload` equivalent absent | ❌ |
| Configurable keybindings (`keybindings.json`, ~80 named actions) | Fixed keys | ❌ |
| Themes (JSON schema, system/dark/light, `light/dark` auto, OKLCH, hot reload) | Fixed styles (explicitly out of scope so far) | ❌ |
| Per-project resource dirs (`.pi/` skills/extensions/prompts/themes/settings/mcp) + trust gate | Workspace sandbox model; no project-local resource dirs | ❌ (different trust model: commandpolicy+HITL ➕) |
| Custom provider extensions (`registerProvider`, `streamSimple`, `refreshModels`, OAuth flows) | provider registry + adapters (Go) | 🟡 config-level only |

## 7. TUI surface detail

| Capability | Pi | VIVY CODE | Status |
|---|---|---|---|
| Markdown + syntax highlight + mermaid streaming render | Yes | Streaming markdown exists; mermaid ❓ | 🟡 |
| Command palette / fuzzy `/` discovery | Yes | `command_palette.go` exists | ✅ |
| @file completion | Yes | Yes (vivy-code-file-completion) | ✅ |
| Image attach | paste/drag + `read` of images, inline display (kitty/iterm2), auto-resize | `/image` attach, paste-guard chip | 🟡 no inline terminal image display |
| Editor: emacs/word ops, undo, yank, history nav, multi-line | Full readline set | Standard bubbletea editor | 🟡 |
| External editor (`Ctrl+G`) | Yes | No | ❌ |
| Transcript search (`Ctrl+Shift+F`) | Yes | No | ❌ |
| Prompt-jump navigation, message markers | `Ctrl+↑/↓` | No | ❌ |
| Copy last message (`Ctrl+X`), copy-on-select | Yes | No | ❌ |
| Session picker w/ search/rename/delete/sort | Yes | `/sessions` list | 🟡 |
| Tree navigator | Yes | No | ❌ |
| Mouse events (click, drag-select) + wheel | Full | Wheel scroll; partial | 🟡 |
| Status footer (cwd/session/model/context%/cost) + thinking-level border | Yes | Footer w/ queue count; sidebar rail (MCP/skills/files) | 🟡 different chrome |
| Startup resource listing | Yes | No | ❌ |
| Inline images in transcript | kitty/iterm2 protocols | No | ❌ |
| OSC8 hyperlinks, OSC 9;4 progress, truecolor detection, terminal-theme-adaptive colors | Yes | Fixed styles | ❌ |
| IME/hardware-cursor positioning | Yes | Standard | ❌ |
| i18n | English | en/zh | ➕ |
| Notifications/toasts | `ctx.ui.notify` | gate/notice system | 🟡 |

## 8. Security/governance (vivy mostly exceeds)

| Capability | Pi | VIVY | Status |
|---|---|---|---|
| Project trust gate for project-local resources | Yes (trust.json, `--approve`) | N/A — no project-local resources; workspace sandbox instead | 🟡 different model |
| Approval/HITL for tool calls | Only via extensions (annotations hints) | Built-in: `/permission` presets, sandbox policy, deny globs, network allowlist, approval timeout + smart preset, review center | ➕ |
| Governed shell (command classifier, deny execs) | None builtin | `commandpolicy` deny/shell-escape classifier | ➕ |
| Sandbox guidance | Docs only (no builtin) | Workspace-root + network sandbox config | ➕ |
| Secrets handling | `!command` keys, auth.json, no plaintext logs | env refs only (D-010), secret redaction audit | ➕ |

## 9. Infra/distribution

| Capability | Pi | VIVY | Status |
|---|---|---|---|
| Install/update (`install.sh`, `pi update`, nix) | Yes | justfile/docker; no self-update | ❌ |
| Package manager subcommands (`pi install/remove/list/config/update`, `pi mcp`, `pi auth`) | Yes | None | ❌ |
| Session JSONL import/export portability | Yes | Journal-bound | ❌ |
| Remote execution env (SSH tool backend, pi-env daemon) | Experimental | None | ❌ |
| Durable execution harness (commit-before-show, mid-turn crash resume) | Experimental pi-durable | Production Journal + checkpoints/interrupts | ➕ |
| Telemetry contracts | pi-telemetry | logging split + OBS suite | ➕ |
| Eval harness | vitest-evals + docker doc-lift | internal/eval + conformance | 🟡 |
| Windows/macOS/termux/ssh support notes | Full docs | Windows-first dev; Go binary portable | 🟡 |

## 10. Prioritized backlog to reach 100% pi parity

Ordered by user-visible impact, not effort. Groups ≈ Epic-sized.

**P0 — CLI + embedding surface (currently biggest hole)**
1. `vivy-code` CLI flags: `--print`, `--mode text|json|rpc`, `--continue/--resume/--session/--session-id/--fork/--session-dir/--no-session`, `--model/--provider/--thinking/--api-key`, `--tools/--exclude-tools/--no-tools`, `--list-models`, `--name`, `--export`, `--approve/--no-approve`, `--offline`, `--version`. (Also fix open TODO `CLI-HELP-EXIT`.)
2. JSON event-stream mode (JSONL out) + RPC subprocess mode (JSONL stdin/stdout command protocol). The existing `vivy.rpc.v1` WebSocket plane covers control; the gap is a *subprocess* protocol like pi's.
3. Embeddable session SDK surface (go equivalent of `createAgentSession`): prompt/steer/followUp/abort/subscribe/getMessages/setModel/setThinking/compact/fork/export.

**P1 — session + context**
4. Session tree model + `/tree` navigator, `/clone`, `/import`; branch summarization on navigation.
5. Steering: mid-turn inject (deliver-after-current-turn) vs follow-up, `Alt+Up` dequeue, `steeringMode`/`followUpMode`.
6. `/export` (HTML+JSONL), `/copy`, `/share`, `/bug`, `/debug` diagnostics dump.
7. Compaction depth: `/compact [instructions]`, split-turn handling, read/modified-file tracking in summary, pre-compact hooks.
8. Context files: CLAUDE.md + `*.override.md` chain + `SYSTEM.md`/`APPEND_SYSTEM.md` (decide against D6 single-file rule if parity demands).

**P2 — model plane**
9. `/login` OAuth flows + `auth.json` + `!command` key resolution + credential precedence chain.
10. 7-level thinking (`off…max`) w/ per-model clamp/defaults + `thinkingBudgets` + per-level sampling params.
11. Virtual model/router registration (per-request routing w/ state).
12. Native wire protocols: OpenAI Responses, Azure Responses, Bedrock Converse, Google GenAI/Vertex, Mistral.
13. Live model catalog refresh + offline cache + `vivy update --models`.
14. Classifier + image-model types reachable by tools; `models.*` equivalent.
15. llama.cpp/ollama local-model discovery (already queued in VC-4).
16. Prompt-cache warming + cache-miss notices.

**P3 — extensibility runtime (architectural decision required)**
17. Runtime extension mechanism: the sealed-Go-pack model cannot drop in code at runtime. Options: (a) JS/WASM extension sandbox (QuickJS plugin — matches codemode need too), (b) out-of-process extension protocol over stdio/JSON-RPC (keeps sealed kernel, extensions as managed child processes — closest fit to Vivy's durability/sealed story), (c) accept divergence and document Go-pack-only extensibility. A decision record is needed before any work.
18. On top of 17: `registerTool/Command/Shortcut/Flag/Provider/McpServer/VirtualModel`, event bus (~40 event types), `ctx.ui` bridge to face protocol, `appendEntry`, renderers, hot `/reload`.
19. Tool exposure model (direct/model-only/codewmode/deferred/hidden) + namespaces + `tool_search` + `codemode` sandbox tool.
20. Pi-package equivalent: resource bundles (extensions+skills+prompts+themes) from npm/git/local + `vivy install/remove/list/config/update`.
21. Prompt templates → user slash commands w/ arg substitution.
22. Themes + `keybindings.json` (named actions).

**P4 — TUI chrome**
23. Themes (incl. light/dark pair + system-adaptive), configurable keybindings.
24. Transcript search, prompt-jump, copy-on-select/`Ctrl+X`, external editor, double-Esc→tree, `/hotkeys`.
25. Inline terminal images (kitty/iterm2), OSC8 hyperlinks, terminal-adaptive colors, startup resource listing.
26. Mermaid streaming render.

**P5 — infra**
27. `vivy mcp add/list/login/logout` shell subcommands + MCP OAuth 2.1 (already OPEN).
28. Shell env injection (`VIVY_SESSION_ID`, `VIVY_PROVIDER`, `VIVY_MODEL`, `VIVY_REASONING_LEVEL`) + `shellCommandPrefix` + `!!` no-context variant.
29. Remote execution env (SSH tool backend) — pi-env equivalent (experimental upstream; lowest priority).
30. `vivy update` self/packages + installer script.

## 11. Not required for parity (vivy already exceeds)

Channels (feishu/telegram/discord/qq/dingtalk), cron scheduler, durable Journal + writer fencing + checkpoints, HITL approval center + commandpolicy, plan mode, subagents+inbox, workflow tool, notes/tasks, web/search/download tools, diagnostics/observability suite, Studio IDE + embedded web UI + headless face, sealed Generation/pack distribution, Postgres storage option, en/zh i18n, auto-titler.

## 12. Recommendation

100% parity is achievable but the binding constraint is architectural: **pi's extensibility is runtime-dynamic; Vivy's is sealed build-time**. Before P3 work, land a decision record on the extension mechanism (recommended: out-of-process extension protocol — preserves the sealed kernel + Journal authority while matching pi's capability set). P0–P2 are independent of that decision and close ~60% of the remaining gap; start there.
