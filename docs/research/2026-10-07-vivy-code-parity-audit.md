# VIVY CODE vs Pi Agent — Post-VCP Self-Audit

Date: 2026-10-07
Source: `earendil-works/pi` @ `23cf2b9`, `ProjectViVy/agent-vivy` @ `feat/vivy-code-parity` (`c8490b5`)
Scope: item-by-item re-verification of the 2026-10-06 gap matrix after all 25 VCP stories landed.

Status legend: ✅ implemented (evidence cited) · 🟡 partial/weaker · ❌ still missing · ⏸️ deferred by owner · 🚫 rejected by owner (doctrine) · ➕ vivy exceeds pi

## 0. Scoreboard

| Plane | Pre-VCP | Post-VCP |
|---|---|---|
| Agent engine + tools | ~75% | ~95% (codemode/runtime-ext by decision) |
| Model/provider | ~45% | ~80% within Anthropic+OpenAI scope |
| Session/context | ~55% | ~90% (branch-summaries + JSONL export remain) |
| CLI + embedding | ~25% | ~100% of committed surface |
| Extensibility runtime | ~20% | N/A — rejected by doctrine (see §6) |
| TUI | ~60% | ~90% (mermaid, IME, drag-select remain) |

Overall vs pi's *agreed* scope (decisions applied): ~95%. vs pi's *raw* surface including rejected items: ~80%.

## 1. Agent loop and tools

| Capability | Pi | VIVY post-VCP | Status | Evidence |
|---|---|---|---|---|
| Streaming loop + tool calls | yes | Eino ADK | ✅ | `internal/runtime` |
| Parallel tool calls | yes | yes | ✅ | — |
| Steering mid-run | Enter | Enter→steer lane, delivered after current turn | ✅ | B1 `Service.Steer`, `run.started`/steering RPC; B3 GUI |
| Follow-up queue + dequeue | Alt+Enter/Alt+Up | identical keymap (`follow_up` alt+enter/ctrl+q, `dequeue` alt+up) | ✅ | `sdk/tui/view/keymap.go` |
| Abort | Esc | esc cancel + gates | ✅ | keymap `cancel` |
| Retry w/ backoff knobs | `retry.*` | failover/titler routes; no user retry knobs | 🟡 | unchanged |
| Overflow compact+retry | yes | auto-compact + provider-error-triggered compact-and-retry | ✅ | D2 `internal/runtime/compaction` + recovery |
| File tools | r/w/edit/grep/find/ls | read/write/patch/multiedit/search/glob/list_dir | ✅ | protected 11 |
| Shell tool env injection + output spill + prefix | PI_* / shellCommandPrefix / truncate→file | VIVY_SESSION_ID/RUN_ID/PROVIDER/MODEL/THINKING, `shell_command_prefix`, spill → `.vivy/tool-output/` | ✅ | E1 |
| `!` / `!!` shell | both | `!` governed shell, `!!` no-context | ✅ | E1 |
| Web/task/notes/plan/subagent/workflow/jobs tools | via ext / absent | builtin | ➕ | internal/tools |
| Tool exposure model | direct/model-only/codemode/deferred/hidden | exposure levels + toolhost `ErrToolNotActive` gating (codemode lane absent by deferral) | ✅ | E2 |
| `tool_search` discovery | BM25 builtin ext | search→declare→activate | ✅ | E3 |
| `codemode` JS sandbox | QuickJS ext | — | ⏸️ | deferred (owner) |
| Nested tool calls `ctx.executeTool` | yes | agent-tool pattern only | 🟡 | unchanged |
| Custom tools w/o rebuild | registerTool | compile-time Module only | 🚫 | doctrine |
| Registerable tool renderers | registerToolRenderer | fixed internal renderers (G3) | 🚫 | runtime ext rejected |
| File-mutation queue | withFileMutationQueue | Journal durability instead | 🟡 different model | unchanged |

## 2. Model/provider plane (scope: Anthropic+OpenAI per owner)

| Capability | Pi | VIVY | Status | Evidence |
|---|---|---|---|---|
| Providers breadth | ~35+ | 46 vendors yaml | 🟡 | unchanged (count ok) |
| Wire protocols | Anthropic + OpenAI Comp+Responses + Azure + Vertex + Bedrock + Mistral | OpenAI-compat + Anthropic | 🚫 scoped | owner: anthropic/openai only; rest → future provider plugins |
| OAuth `/login` + auth.json + `!command` keys | yes | env keys + registry UI | ⏸️ | deferred → plugins/provider |
| Thinking levels | 7 + clamp + budgets + per-level sampling | `/thinking [auto|on|off|minimal|low|medium|high|xhigh|max]` + per-model clamp + `ThinkingSampling` per-level overrides | ✅ | F1 |
| Model picker | fuzzy + Ctrl+P + scoped + save | `/model` + Alt+P cycle + `/scope-model` + `scoped_models`/`project_defaults` settings | ✅ | F3 |
| Virtual models/router | registerVirtualModel | — | ⏸️ | deferred-locked |
| Classifier / image models | via codemode | — | ⏸️ | tied to codemode |
| Local models | llama.cpp router | `plugins/infra/llm`: discover/status/models/pull across ollama/llamacpp/lmstudio/vllm; start/stop honestly `spawn_unsupported`+hints | 🟡✅ | I1 — see §7 note |
| Cache warming | cacheWarming | `cache_warming off|streaming|idle` + min-savings floor + Anthropic breakpoints | ✅ | F2 |
| Live model catalog | pi.dev overlay + update --models | static vendored yaml | ❌ | **remaining gap** |
| Usage/cost | per-model + cache tokens | /stats + cache token accounting | ✅ | parity |
| Per-request sampling params | samplingParams | provider options schema (temperature/top_p/max_tokens) + ThinkingSampling | 🟡 | F1 partial |

## 3. Session/context plane

| Capability | Pi | VIVY | Status | Evidence |
|---|---|---|---|---|
| Storage | JSONL tree | Journal + fencing | ➕ durability; portability 🟡 | — |
| Resume/continue | --continue/-r, picker | `--continue/--resume/--session/--fork/--no-session` + /sessions + newest-on-launch | ✅ | A1 |
| Session tree browser | /tree | `session/tree` RPC + `/tree` navigator + GUI | ✅ | C1/C2/C3 |
| Fork/clone/import | /fork /clone /import | identical + pi-JSONL `ImportSession` | ✅ | C-series, `internal/runtime/session_copy.go` |
| Branch summarization on nav | auto summary | none | ❌ | **remaining gap** |
| Auto-compaction | reserveTokens/split-turn/file-tracking/instructions/hooks | TriggerPercent+KeepRecent+per-model overrides + `/compact [instructions]` TUI+GUI + file-tracking manifest | ✅ | D1–D3; hooks = ext events 🚫 |
| Context files | AGENTS+CLAUDE+overrides+SYSTEM.md | AGENTS.md only | 🚫 | D6 decision |
| Skills | Agent Skills | skillhost + marketplace | ✅ | parity |
| Prompt-template slash commands | ~/.pi/agent/prompts | fixed registry | 🚫 | runtime ext rejected |
| Export/share/copy/bug/debug | all | `/export` HTML (+`exports/read` RPC), `/copy`, `/bug`, `/debug`; /share deferred; **JSONL export absent** | 🟡 | C3 |
| Cross-project sessions | --session-dir grouping | `--session-dir` exists | 🟡 | A1 |
| Title generation | manual /name | auto titler | ➕ | — |

## 4. CLI + embedding

| Capability | Pi | VIVY | Status | Evidence |
|---|---|---|---|---|
| Flag surface | ~40 flags | full parity set incl. `--print --mode --model --provider --thinking --api-key --tools/--exclude-tools/--no-tools/--no-builtin-tools --continue/--resume/--session/--session-id/--fork/--session-dir/--no-session --list-models --name --export --approve/--no-approve --offline --theme/--no-themes --skill --no-skills --system-prompt/--append-system-prompt --prompt-template --tui-mode --verbose --debug-tools --version`, @file, `--` | ✅ | A1 `cmd/vivy-code/args.go` |
| Print mode | -p | `--mode print` + non-TTY auto-fallback | ✅ | A2 facerun |
| JSON event stream | --mode json | JSONL projection | ✅ | A2 |
| RPC subprocess | --mode rpc + RpcClient | pi-compatible JSONL stdin/stdout, ~15 verbs; unimplemented ops answer `success:false`+reason; bash refused (governance) | ✅ | A3 |
| Embeddable SDK | createAgentSession | `sdk/codeclient` + real-binary E2E | ✅ | A4 |
| pkg-manager subcommands | pi install/mcp/auth | `vivy-code mcp list|status|add|remove` + `config get|set`; `mcp login`→deferral notice; install/update ⏸️ | 🟡 | A5 |
| Headless face | — | `faces/headless` module | ✅ | — |

## 5. Extensibility runtime

Whole plane **🚫 rejected by doctrine** — compile-time Module→Port→Recipe→Generation is the chosen extensibility story ("编译性插件一切皆插件物种理念很先进"). Runtime TS extensions, event hooks, ctx.ui, appendEntry, packages ecosystem: never to be ported. Keybindings/themes (pi implements them as resources) were ported anyway as builtin features (G1/G2).

## 6. TUI surface

| Capability | Pi | VIVY | Status | Evidence |
|---|---|---|---|---|
| Markdown + highlight streaming | yes | yes; **mermaid ❌** | 🟡 | — |
| Command palette + fuzzy / | yes | yes | ✅ | — |
| @file completion | yes | yes | ✅ | — |
| Image attach + inline display | paste/drag + kitty/iterm2 | /image + kitty/iTerm2/WezTerm transmit+place | ✅ | G4 |
| Editor readline ops | full | standard editor | 🟡 | unchanged |
| External editor | Ctrl+G | ctrl+e | ✅ | G3 |
| Transcript search | Ctrl+Shift+F | ctrl+f | ✅ | G3 |
| Prompt-jump nav | Ctrl+↑/↓ | ctrl+up/down | ✅ | keymap |
| Copy last / on-select | Ctrl+X | alt+c + /copy (OSC52) | ✅ | G3 |
| Session picker | yes | ctrl+s + /sessions | ✅ | — |
| Tree navigator | yes | /tree | ✅ | C2 |
| Mouse | click+drag+wheel | press-click + wheel only | 🟡 | no drag-select |
| Startup resource listing | yes | hero counts (skills/tools/mcp/sessions) | ✅ | G3 |
| OSC8 / truecolor | yes | OSC8 linkify + COLORTERM=truecolor | ✅ | G3 |
| OSC 9;4 progress | yes | ❌ | ❌ | minor gap |
| IME positioning | yes | standard | ❌ | unchanged |
| Themes | JSON + auto | `sdk/tui/theme` loadable JSON + dark/light pair | ✅ | G1 |
| Keybindings | keybindings.json ~80 actions | YAML keymap ~30 named actions, unknown-action warnings | ✅ | G2 |
| /hotkeys viewer | yes | /hotkeys | ✅ | — |
| i18n | en | en+zh | ➕ | — |

## 7. Governance (unchanged: vivy exceeds)

HITL approvals, sandbox policy, deny globs, commandpolicy classifier, secret redaction, durable Journal — all ➕. No regressions.

## 8. Remaining real gaps (not decided away)

1. **Live model catalog refresh** — static vendored yaml; no update mechanism. (→ plugin-able later)
2. **Branch summarization on navigation** — pi summarizes abandoned branch when navigating the tree; absent.
3. **JSONL session export** — /export is HTML-only; inbound pi-JSONL import exists but no symmetric export.
4. **Mermaid streaming render**, OSC 9;4 progress, IME positioning, mouse drag-select — TUI polish gaps.
5. **Agent-level retry knobs** (`retry.*`) — failover exists, user-facing retry config doesn't.

## 9. Deferred inventory (owner decisions)

| Item | Destination |
|---|---|
| `/share` public link | deferred — no upload target chosen |
| codemode (JS sandbox meta-tool) | deferred — goja approved as future impl |
| OAuth `/login` + auth.json | `plugins/provider/*` (future category) |
| Self-update `vivy update` + installer | deferred |
| Virtual models/routers | deferred-locked |
| Native wire protocols beyond Anthropic/OpenAI | rejected (CN usage); future provider plugins |
| Runtime extensions/packages/prompt-templates | rejected — compile-time doctrine |
| CLAUDE.md/SYSTEM.md context chain | rejected — D6 AGENTS.md-only |
| Remote exec (pi-env equivalent) | not scheduled |
