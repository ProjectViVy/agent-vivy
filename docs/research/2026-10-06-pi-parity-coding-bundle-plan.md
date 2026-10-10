# VIVY CODE — Pi Parity Closure Plan (decision-adjusted)

Date: 2026-10-06
Basis: pi @ `23cf2b9`, agent-vivy @ `f34f3ce`, gap report v1
Owner decisions applied:
- D-A: CLI/embedded surface → full implementation
- D-B: runtime extension system → NOT built; compile-time Module/Port/Recipe is the doctrine
- D-C: model plane → Anthropic + OpenAI only; OAuth deferred to a later plugin; cache warming IN; llama.cpp/ollama → research note for decision
- D-D: session/context plane → full; `/compact` must be reachable from both TUI and GUI
- D-E: tool plane → enhance
- D-F: TUI → enhance
- D-G: all parity work lands in one **coding plugins bundle** (Modules + Ports + Recipe + faces), not scattered patches

## 1. Where pi parity lands in VIVY architecture

Vivy's sealed model already has homes for almost every pi surface. Mapping:

| Pi surface | VIVY home | Port / layer |
|---|---|---|
| Builtin tools (read/bash/edit/grep/write/find/ls) | `vivy/protected-tools` + tool modules | `std/tool@v1` |
| MCP tools | MCPHost → dynamic catalog | `std/tool-world@v1` |
| codemode / tool_search meta-tools | new module | `std/tool@v1` (static meta-tool over ToolWorld catalog) |
| Tool exposure levels + activation | new ordered middleware or ToolHost visibility flag | `std/middleware/pre-tool@v1` / ToolHost |
| Skills (`/skill:`) | SkillHost + skills marketplace | `std/skill-source@v1` |
| Prompt templates → slash commands | new context-source + face command plumbing | `std/context-source@v1` + face |
| AGENTS.md/CLAUDE.md/overrides | extend project_instructions into a context-source | `std/context-source@v1` |
| Provider registry / OAuth plugin | provider profiles; OAuth later as module-owned control-action flow | `std/provider-profile@v1` + `std/control-action@v1` |
| Session tree / clone / import / export / share | kernel storage + control actions or RPC | `std/control-action@v1` or core RPC |
| Compaction settings + manual compact | existing CompactionService | core RPC `context/compact` |
| Interactive TUI | `vivy/tui` face | `std/face@v1` |
| print/JSON/RPC subprocess modes | code face multimode or headless face variants | `std/face@v1` |
| Web UI extras (compact button, tree, export) | new UI module | `std/ui-extension@v1` |
| Themes / keybindings | TUI face internal theme+keymap loader; web via `std/ui-extension` | face internals |
| Cache warming | provider adapters + ModelHost scheduler | `internal/provider` + `internal/modelhost` (closed surface — kernel work) |
| Steering mid-run | loop window + Journal event | `internal/runtime` (kernel work) |
| llama.cpp/ollama | optional module: control-actions (spawn/list/pull) + provider-profile | `std/control-action@v1` + `std/provider-profile@v1` |
| Telemetry/export diagnostics | observers | `std/observer/*@v1` |

**Closed surfaces (kernel work, cannot be plugins):** executable model provider, steering injection into the run loop, session storage queries, Journal read/write, face options plumbing, compaction service internals, Policy evaluator. The bundle covers everything selectable; kernel deltas are listed in §6.

## 2. The coding plugins bundle

Today coding capability is scattered: `internal/tools/*` (~40 tools, mixed protected/internal), `plugins/lsp`, `sdk/tui` face, `internal/codeface` launcher, `faces/headless`, skill/mcp hosts, plan/goal/workflow/subagent tools. The bundle makes the "coding product" one Recipe + one module family:

```
recipes/vivy-code.vivy.yml          # already exists; becomes the bundle definition
plugins/coding/
  coding-tools/                     # std/tool@v1: tool_search, codemode(goja), find/ls parity extras
  coding-exposure/                  # std/middleware/pre-tool@v1: exposure levels + dynamic activation
  coding-context/                   # std/context-source@v1: CLAUDE.md, *.override.md, SYSTEM/APPEND_SYSTEM, prompt templates
  coding-session-extras/            # std/control-action@v1: tree/clone/import/export_html/share/bug-report
  coding-local-llm/                 # optional: control-actions + provider-profile (llama.cpp/ollama) — gated on decision
  coding-ui/                        # std/ui-extension@v1: compact trigger, session tree view, export/share buttons
faces/
  tui/                              # existing vivy/tui — enhance in place
  headless/                         # existing — extend to full print/json/rpc multimode
```

`recipes/vivy-code.vivy.yml` then selects: `vivy/loop, vivy/model, vivy/tool-host, vivy/storage, vivy/checkpoint, vivy/credential, vivy/sandbox, vivy/protected-tools, vivy/mcp-host, vivy/face-host, vivy/tui, coding/*`.

Protected-tool rule check: `bash`, `read_file`, `write_file`, `patch`, `multiedit`, `execute`, `search_files`, `list_dir`, `ask_user`, `skills_list`, `skill_view` are T1-only — env-injection and output-file changes to `bash`/`execute` are **kernel edits**, not bundle modules. New tools (`tool_search`, `codemode`, `find`, `ls`) are free IDs and become public modules.

## 3. D-A: CLI/embedded surface (full)

pi surface to match: `-p/--print`, `--mode text|json|rpc`, `--continue/--resume/--session/--session-id/--fork/--session-dir/--no-session`, `--model/--provider/--thinking/--api-key`, `--tools/--exclude-tools/--no-tools`, `--list-models`, `--name`, `--export`, `--offline`, `--verbose`, `--version`; subcommands `install/remove/list/config/update/auth/mcp`.

**Design:** one face Provider with internal mode dispatch (keeps the `std/face@v1` 0..1 rule). `cmd/vivy-code` grows a real flag parser feeding `face.Options` (already has `Prompt`, `ContinueNewest`, `Out`, `Err` — extend with Mode, Model, Provider, Thinking, ToolSet, SessionID, SessionDir, Export).

| pi mode | VIVY equivalent |
|---|---|
| `text` (interactive) | existing `vivy/tui` face |
| `print` (`-p`) | headless face path: prompt → stream text → stdout. Exists today (`faces/headless`) but unreachable via flag |
| `json` | new: emit Journal/run events as JSONL on stdout (`session`, `agent_start`, `turn_start`, `message_start/update/end`, `tool_execution_*`, `turn_end`, `agent_end`, `compaction_start/end`, `auto_retry_*`, `error`). Source = the face's `Host.OnEvent` stream → stable event names |
| `rpc` | new: JSONL stdin commands / stdout responses+events. Implement pi's command set over `vivy.rpc.v1`: `prompt, steer, follow_up, abort, clear_queue, new_session, get_state, get_messages, set_model, cycle_model, get_available_models, set_thinking_level, cycle_thinking_level, get_available_thinking_levels, set_steering_mode, set_follow_up_mode, compact, set_auto_compaction, set_auto_retry, abort_retry, bash, abort_bash, get_session_stats, export_html, switch_session, fork, clone, get_fork_messages, get_entries, get_tree, get_last_assistant_text, set_session_name, get_commands` |

Subcommands: `vivy-code mcp list/add/remove/login` → control actions over MCPHost (login deferred per D-C); `vivy-code config get/set` → settings overlay; `vivy-code update` → deferred (no self-update channel exists; needs separate decision).

**Embedding SDK:** pi exports `createAgentSession`. VIVY analogue is a Go package wrapping the control plane (like `faces/headless` does via `faceport.Host.Call`): `sdk/codeclient` — `NewClient(opts)`, `Prompt`, `Steer`, `FollowUp`, `Abort`, `Subscribe`, `SetModel`, `SetThinking`, `Compact`, `Fork`, `Export`. It talks `vivy.rpc.v1`; no kernel import. Cheap, consistent with face contract.

## 4. D-D: session/context plane

**Compaction — current state (verified):**
- Kernel: `Service.CompactSession(ctx, sessionID)` (no instructions param), auto-compaction via `CompactionPolicy{TriggerPercent, KeepRecent, MaxTokens}`, `context.compacted` events, `session/compactions` listing.
- TUI: `/compact` — exists, no args.
- GUI: `CompactionSettingsCard` calls `compactSession` → `context/compact`. Chat-level trigger absent.
- Gaps vs pi: `/compact <instructions>` arg (TUI + RPC param + kernel signature + GUI input), per-model overrides, split-turn handling, file read/modified tracking inside summaries, compaction hooks (observers get `context.compacted` already — enough), branch summarization on tree nav.

**Steering — the big kernel item.** Pi semantics: `Enter` while busy → steering message delivered after the *current turn* finishes, steering the next turn inside the same run; `Alt+Enter` → follow-up, delivered after the run settles; `Alt+Up` → dequeue back to editor; `steeringMode`/`followUpMode` = `all|one-at-a-time`. VIVY today: single `queuedTurn` FIFO drained only when the run ends = follow-up only. Needed: `turn/steer` RPC + loop-window injection point in `internal/runtime` (after current turn's tool batch settles, before next model call) + Journal event `turn.steered`. This is the single largest kernel delta in the program.

**Session tree.** Storage already supports it: `session_truncations` markers (rewind/edit/fork, tail-anchored, append-only) + `fork_session_id` links + messages never deleted. Missing: `session/tree` read API (sessions × fork-graph), `session/clone` (copy visible view to new session id), `session/import` (JSONL import → new session), TUI `/tree` navigator, GUI tree view. Both TUI `/fork`/`/rewind` and GUI per-message rewind/fork already exist.

**Export/share/report.** `session/export` → standalone HTML (render messages+tool calls; Journal is the source — kernel RPC or control-action with Journal read). `/share` needs an upload target decision (gist vs self-host — mark open). `/bug` → diagnostics zip (logs+session redacted). `/debug` → local log dump (mostly exists via diagnostics).

**Context files.** `DiscoverProjectInstructions` walks launch dir → git root for AGENTS.md only. Pi parity wants: `AGENTS.md` + `CLAUDE.md` chain, `*.override.md` sibling overrides, ancestor accumulation, `~/.vivy/AGENTS.md`, `SYSTEM.md`/`APPEND_SYSTEM.md`. Conflicts with D6 "AGENTS.md only" decision — needs explicit owner call, else mark partial-by-policy.

## 5. D-E: tool plane

| Item | Mechanism | Home |
|---|---|---|
| Shell env injection (`VIVY_SESSION_ID/PROVIDER/MODEL/THINKING`) + `shellCommandPrefix` + `!!` no-context variant | T1 edit to `bash`/`execute` | `internal/tools` (protected) |
| Large-output capture to file + tail truncation (pi's output-file pattern) | T1 edit | `internal/tools` |
| Tool exposure model (direct/model-only/codemode/deferred/hidden + namespaces) | ToolHost visibility flag OR ordered middleware `coding-exposure` | bundle + ToolHost flag |
| `tool_search` (BM25 → declare) | new static tool over ToolWorld catalog | `coding-tools` |
| `codemode` (sandboxed JS calling tools) | **DEFERRED** (goja remains candidate engine) | — |
| Dynamic activation (`setActiveTools`) | ToolHost + face command | kernel touch |
| Nested tool calls w/ parent linkage | Journal field + `child_inbox` precedent | kernel touch |
| `find`/`ls` parity | already covered by `glob`/`list_dir` | — |
| Tool renderers per tool | face-side render registry | `vivy/tui` |

## 6. Kernel deltas (cannot live in the bundle)

1. `turn/steer` + loop injection point + `turn.steered` Journal event (steering).
2. `CompactSession(ctx, id, instructions)` + RPC param + TUI arg parse + GUI field.
3. Provider-error context-overflow recovery (compact-and-retry on `stopReason: length` / overflow error class).
4. Session tree/clone/import/export storage + RPC methods.
5. Cache warming: per-model cache-lifetime metadata on provider-profile; ModelHost scheduler (`off|streaming|idle`, default streaming, cost gate); Anthropic adapter already emits `cache_control: ephemeral` breakpoints — warming = prefix-refresh call, usage counted, not added to context.
6. Thinking levels: `off|minimal|low|medium|high|xhigh|max` map + per-model clamp/defaults on provider-profile + `samplingParamsByThinkingLevel` (Anthropic `thinking.budget_tokens`, OpenAI `reasoning_effort`).
7. `session/export` HTML, `session/tree`, `session/clone`, `session/import` RPC.
8. Face options extension (mode, model, thinking, tools, session flags, export).
9. ToolHost: exposure/activation bits (or done in middleware — decide in design review).
10. Provider-profile field additions: `apiKeyEnv`, endpoint class limited to `anthropic|openai` per D-C.

## 7. D-F: TUI enhancements (Bubble Tea feasibility)

| Pi feature | Feasibility in bubbletea/lipgloss | Note |
|---|---|---|
| Themes (JSON, light/dark pair, system-adaptive) | ✅ `sdk/tui/view/styles.go` → theme loader + `keybindings`-style config file | faces read config; no Port needed |
| Configurable keybindings (~80 named actions) | ✅ refactor hardcoded keys → action map + `keybindings.yaml` | |
| Transcript search | ✅ filter view over transcript model | |
| External editor (`Ctrl+G` → $EDITOR) | ✅ trivial | |
| Tree navigator (`/tree`) | ✅ alt-screen graph view; needs §6.4 storage first | |
| Inline images (kitty/iTerm2) | 🟡 custom escape-sequence writer; bubbletea won't render — write raw to tty | medium-hard |
| Mermaid | 🟡 render to image→kitty, or ASCII fallback | hard, low value |
| Copy-on-select / `Ctrl+X` | ✅/🟡 terminal-dependent | |
| Prompt-jump nav (`Ctrl+↑/↓`) | ✅ | |
| Startup resource listing | ✅ | |
| OSC8 hyperlinks, OSC 9;4 progress | ✅ escape sequences | |
| Steering keys (Enter/Alt+Enter/Alt+Up) | ✅ once kernel steering lands | |
| Mouse drag-select | 🟡 bubbletea mouse events; clipboard is terminal's job | |

## 8. llama.cpp / ollama — decision brief (D-C)

Pi's model: `/llama` manages an external `llama-server --jinja` router process (spawn/health/model load-unload/list); GGUF files discovered under a models dir; loaded models appear in `/model`. Ollama/LM Studio/vLLM/SGLang are plain OpenAI-compatible endpoints via config (`apiKey: "ollama"` dummy is enough).

VIVY equivalent, fully as a module (no kernel work):
- `coding-local-llm` module:
  - `std/provider-profile@v1`: OpenAI-compatible profiles `local/llamacpp` (http://127.0.0.1:8080/v1), `local/ollama` (http://127.0.0.1:11434/v1), `local/lmstudio`, `local/vllm`.
  - `std/control-action@v1`: `local_llm.status`, `local_llm.discover` (probe ollama `/api/tags`, llama-server `/v1/models`), `local_llm.start/stop` (spawn via GrantProcSpawn, needs proc grant — modules can request `GrantProcSpawn`, cf. `plugins/lsp`), `local_llm.models` (list GGUF/ollama models), optional `local_llm.pull` (ollama pull).
  - `std/status-source@v1`: server health for sidebar.
- Cost: small-medium; the only question is whether process supervision belongs in a plugin (pi treats router as an external process too — consistent).
- Risk: GGUF layout/params differ; keep to OpenAI-compat surface.
- **Recommendation: build.** It's ~1 module, no kernel change, and it is the only local-inference path; deferring leaves Chinese-local-model users without an answer. If deferred, minimum viable = one provider-profile + docs (`OLLAMA_BASE_URL` style vendor entry exists already).

## 9. GUI (web face) additions

Via `coding-ui` (`std/ui-extension@v1`): chat-level compact button (settings card exists today), session tree page, export/share buttons, compact instructions input, steering-mode switcher, thinking-level selector (7 levels vs auto/on/off). Web face keybindings/themes already fall under `std/ui-extension` full-code authority.

## 10. Phasing

| Phase | Content | Kernel touch |
|---|---|---|
| CC-0 | Design review: bundle layout, face multimode, steering injection point, exposure mechanism choice | — |
| CC-1 | CLI flags + face modes (print/json/rpc) + `codeclient` SDK | face options, RPC |
| CC-2 | Steering + queue modes (the hard one) | runtime loop, Journal event |
| CC-3 | Session tree/clone/import/export/share + `/tree` TUI + GUI tree | storage, RPC |
| CC-4 | Compaction: instructions arg end-to-end + per-model policy + file-tracking summary | compaction service |
| CC-5 | Tools: env injection, output-to-file, exposure middleware, `tool_search`, `codemode` | protected tools edit |
| CC-6 | Model host: thinking levels, cache warming, overflow recovery | provider/modelhost |
| CC-7 | TUI: themes, keybindings, search, external editor, tree nav, steering keys | face only |
| CC-8 | Bundle consolidation: move non-protected coding tools + lsp + faces under `plugins/coding/*`, finalize `vivy-code.vivy.yml` | refactor |
| CC-9 | `coding-local-llm` compat plugin (approved) | none |

Estimated: CC-1,3,4,5,7 are face/module-scale (days each); CC-2, CC-6 are kernel-scale (the two risky items). Total program ≈ a full parity Epic; sequencing CC-1→CC-2→CC-3 first gives users the visible pi-parity surface fastest.

## 11. Owner decisions — RESOLVED 2026-10-06

1. Context files: **AGENTS.md only** (D6 stands; CLAUDE.md/override/SYSTEM.md explicitly rejected). The `coding-context` module shrinks to prompt-template support only; ancestor/override features are out of scope.
2. `/share`: **deferred** — `/export` produces a local HTML file; no upload target.
3. `codemode`: **deferred** — `tool_search` stays in scope; the JS-sandbox meta-tool is revisited later (goja remains the candidate engine).
4. Local-LLM: **build** `coding-local-llm` as a compat plugin (control-actions + provider-profile, §8).
5. `vivy-code update/install`: **deferred** — no self-update channel.
6. Steering keys: **pi scheme** — `Enter`=steer, `Alt+Enter`=follow-up, `Alt+Up`=dequeue to editor.

Scope consequences: codemode rows in §2/§5 move to deferred; `/share` removed from `coding-session-extras` (export stays); context-file expansion removed (D6); CC-9 local-llm is scheduled work, not pending.
