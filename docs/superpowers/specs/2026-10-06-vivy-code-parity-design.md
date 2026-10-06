# VCP-D1 — VIVY CODE Pi Parity Design

> Status: proposed. Baseline: `f34f3ce` (main, 2026-10-06). Reference target: `earendil-works/pi` @ `23cf2b9`.
> Goal: close the pi-agent capability gap inside the compile-time Module/Port doctrine. This spec owns requirements, architecture choices, and exclusions for the `vivy-code-parity` initiative.

## 1. Intent

The owner standard: VIVY CODE must carry 100% of pi agent's capability surface **within the owner-approved scope below**. Pi parity is measured on capability, not mechanism: a capability counts when a user or embedding program can reach the same outcome, even if Vivy delivers it through a Module, Recipe, or control action instead of a runtime extension.

## 2. Owner decisions (binding)

| ID | Decision |
|---|---|
| VCP-O1 | No runtime extension system. Pi's TypeScript extensions are never ported; compile-time Module/Port/Recipe is the species doctrine. All parity work consolidates into a "coding plugins bundle". |
| VCP-O2 | Model wire protocols: Anthropic + OpenAI families only. Responses/Bedrock/Vertex/Azure are out until revisited. OAuth login is deferred to a later plugin (its own proposal). |
| VCP-O3 | Cache warming: build. llama.cpp/ollama: build as a compatibility plugin (`coding-local-llm`). |
| VCP-O4 | `/share` upload: deferred — export produces local HTML only. |
| VCP-O5 | `codemode` JS sandbox tool: deferred (goja remains the candidate engine). |
| VCP-O6 | Self-update (`vivy-code update/install`): deferred. |
| VCP-O7 | Context files stay AGENTS.md-only (D6 stands; CLAUDE.md/override/SYSTEM.md rejected). |
| VCP-O8 | Steering keys follow pi: `Enter`=steer, `Alt+Enter`=follow-up, `Alt+Up`=dequeue to editor. |

## 3. Requirements

| ID | Requirement (observable) | Epic |
|---|---|---|
| RQ-CLI | `vivy-code` accepts a pi-equivalent CLI: `-p/--print`, `--mode text|json|rpc`, `--continue/--resume/--session/--session-id/--fork/--session-dir/--no-session`, `--model/--provider/--thinking/--api-key`, `--tools/--exclude-tools/--no-tools`, `--list-models`, `--name`, `--export`, `--offline`, `--verbose`, `--version` | A |
| RQ-JSON | `--mode json` emits run activity as JSONL records on stdout: session, turn/message start/update/end, tool_execution_start/update/end, turn_end, run_end, compaction_start/end, auto_retry_*, error | A |
| RQ-RPC | `--mode rpc` accepts JSONL commands on stdin and replies JSONL responses/events on stdout, covering the pi command set mapped to `vivy.rpc.v1` semantics (§5.3) | A |
| RQ-SDK | `sdk/codeclient` provides a Go embedding client spawning/driving `vivy-code --mode rpc` (prompt, steer, followUp, abort, subscribe, model/thinking setters, compact, fork, export) | A |
| RQ-SUB | `vivy-code mcp list/add/remove` and `vivy-code config get/set` subcommands operate via control actions without entering the TUI | A |
| RQ-STEER | Dual-track message queue: steer messages deliver at the next turn boundary inside the active run; follow-ups deliver after the run settles; `Alt+Up` returns the queued head to the editor; `steering_mode`/`follow_up_mode` ∈ {`all`,`one-at-a-time`}; queue is Journal-durable (survives restart); identical semantics on TUI, GUI, and RPC | B |
| RQ-SESS | Session tree read model (sessions × fork/rewind graph), `session/clone`, `session/import` (JSONL), `session/export` (standalone HTML), `/tree` navigator in TUI, tree page + export in GUI | C |
| RQ-CMP | `context/compact` accepts optional `instructions`; `/compact <instructions>` in TUI; chat-level compact control in GUI; per-model compaction overrides; compaction summary tracks read/modified file lists; provider-overflow/length-stop triggers compact-and-retry once per run | D |
| RQ-TOOL | Shell tools inject `VIVY_SESSION_ID`, `VIVY_PROVIDER`, `VIVY_MODEL`, `VIVY_THINKING`; `shell_command_prefix` config; `!!` prefix sends without model context; oversized tool output is captured to a file with tail excerpt; tool exposure levels `direct|model-only|deferred|hidden` with runtime activation; `tool_search` meta-tool discovers deferred tools; TUI renders tool calls through a per-tool renderer registry | E |
| RQ-MDL | Thinking levels `off|minimal|low|medium|high|xhigh|max` with per-model clamp, per-model default, and `sampling_params_by_thinking_level`; cache warming `off|streaming|idle` (default `streaming`) gated on model cache-lifetime metadata; model cycling and scoped-model set in TUI; model choice persistable as default | F |
| RQ-TUI | JSON theme files (system/light/dark pair, auto-detect terminal); `keybindings.yaml` mapping named actions to keys; transcript search; prompt-jump navigation; copy-last-message; external editor (`$EDITOR`); startup resource listing; OSC8 hyperlinks; session tree navigator | G |
| RQ-GUI | Chat-level compact control (button + instructions input), steer/follow-up split send, session tree page, export download | C,D,B |
| RQ-LLM | `coding-local-llm` module: provider-profiles for `local/llamacpp` (127.0.0.1:8080/v1), `local/ollama` (11434/v1), `local/lmstudio`, `local/vllm`; control actions `local_llm.status|discover|start|stop|models|pull`; status source for sidebar | I |
| RQ-BND | Coding capability consolidated: non-protected coding tools, lsp, session extras, and both faces organized under `plugins/coding/*`; `recipes/vivy-code.vivy.yml` is the bundle definition; Inspect reports the full inventory | H |

## 4. Exclusions

VCP-O1..O8 plus: mermaid transcript rendering (defer to a theme-era follow-up), remote SSH execution env (pi-env equivalent — pi side is itself experimental), classifier/image model types (no consumer without codemode), prompt-template slash commands (deferred: no context-source slot for them under O7 — revisit if templates are wanted as a `coding-context` module).

## 5. Architecture

### 5.1 Face multimode (RQ-CLI/JSON/RPC/SUB)

`std/face@v1` keeps its `0..1` cardinality: `vivy/tui` remains the one face Provider for the `vivy-code` Generation, and its Runner dispatches on an extended `face.Options`:

```go
type Options struct {
    Prompt                          string   // -p / positional prompt
    ContinueNewest, DebugToolOutput bool
    Mode                            string   // "text" (default) | "json" | "rpc" | "print"
    Model, Provider, Thinking       string   // overrides for this launch
    APIKey                          string   // never logged; env fallback stays primary
    Tools, ExcludeTools             []string // nil = defaults
    SessionID, SessionDir           string
    Continue, Resume, Fork          string   // session selectors
    NoSession, Offline, Verbose     bool
    Name, Export                    string
    Out, Err                        io.Writer
}
```

- `text` → existing TUI loop.
- `print`/`json`/`rpc` → a headless runner inside the same face module (pattern proven by `faces/headless`: drive `Host.Call` + `Host.OnEvent`, no kernel import).
- `json` = event projector: Journal/run events → fixed JSONL record names.
- `rpc` = stdin command loop; each command translates to control-plane calls (§5.3 map).
- `mcp`/`config` subcommands → ActionHost control actions (`mcp.*`, `config.*`); no RPC socket needed when run in-process as a face mode (`--mode rpc` stays available for attached control).

`cmd/vivy-code/main.go` gains a real flag parser (stdlib `flag` is enough; unknown args exit 2 with usage — preserving the current contract).

### 5.2 Steering (RQ-STEER) — the kernel core

Pi semantics to match: a steer message is injected at the next *turn boundary* inside the active run (after the current model turn's tool calls settle); a follow-up waits for run settle.

VIVY design (minimal-mechanism choice, pending Eino check in B1):

```text
turn/steer ──► pending-steer set (Journal: turn.queued{track:"steer"})
                  │
run consume loop ─┤ at each turn-boundary safe point (tool batch settled):
                  │   if steer pending → journal turn.steered + user message row,
                  │   feed it into the loop's next history segment
turn/follow_up ──► pending-follow-up queue (Journal: turn.queued{track:"follow_up"})
                  │   delivered as turn/start after run terminal state
```

Two candidate mechanisms, chosen in B1 after the mandatory Eino capability check:
1. **In-loop injection**: if pinned Eino v0.9.13 ADK exposes a mid-run message injection (e.g. via `AgentEvent`/`ResumeParams`-style seams or state middleware), use it — the run object stays one Journal run.
2. **Early-settle + continue** (fallback, likely): a pending steer triggers a graceful pause at the next turn boundary — the run segment commits its terminal state, the steering message is journaled, and a continuation segment is admitted immediately under the same visible turn flow. Faces render `turn.steered` as continuous. This reuses existing cancel/checkpoint machinery and the `consumeChildMailboxSafePoint` precedent (`service.go` ~2910) — parent-reply injection already proves the boundary concept.

Queue durability: `turn.queued` Journal events make both tracks restart-safe; `clear_queue`/`Alt+Up` journal `turn.dequeued`. Queue modes (`all` drains everything into one delivery; `one-at-a-time` delivers head only) are session settings, not face-local state — today the TUI-local `queuedTurn` FIFO is replaced by kernel-owned queue truth so RPC/GUI/TUI agree.

### 5.3 RPC command map (RQ-RPC)

Each pi command maps to existing or new `vivy.rpc.v1` methods:

| pi command | VIVY path |
|---|---|
| prompt | `turn/start` |
| steer / follow_up / abort / clear_queue | `turn/steer` / `turn/follow_up` / `turn/cancel` / `queue/clear` (B1) |
| new_session / switch_session / set_session_name | `session/new` / `session/select` / `session/rename` (existing) |
| get_state / get_messages / get_session_stats / get_last_assistant_text | `session/context`/`session/status` + `history/read` + `session/stats` (new light RPC) |
| set_model / cycle_model / get_available_models | `settings/update`-style model set + `model/list`; `cycle_model` is face-side (uses ordered list) |
| set_thinking_level / cycle_thinking_level / get_available_thinking_levels | `model/thinking` RPC (F1) |
| set_steering_mode / set_follow_up_mode | `queue/mode` (B1) |
| compact / set_auto_compaction | `context/compact` + `settings/compaction` (existing surface extended, D1) |
| set_auto_retry / abort_retry | `run/retry` config + `run/abort_retry` (new, thin) |
| bash / abort_bash | governed shell through `shell/start`-style seam or rejected-by-policy (must not bypass ToolHost) — B-scope decision: expose only when sandbox mode allows |
| export_html / fork / clone / get_fork_messages / get_entries / get_tree | `session/export`, `session/fork` (exists), `session/clone`, `session/tree`, `history/read` (C1) |
| get_commands | static descriptor in the rpc mode shim |

### 5.4 Session tree and portability (RQ-SESS)

Storage already carries the pieces: `session_truncations` (rewind/edit/fork markers, tail-anchored, append-only) and `fork_session_id` links. New read model:

- `session/tree` → `{nodes:[{session_id,title,created_at,parent_fork_message}], edges:[{from,to,kind}]}` computed from fork links + truncation rows (audit read; never folds).
- `session/clone {session_id}` → new session containing the source's *visible view* (post-folding) — Journal events are copied into the new session id, provenance stamped `cloned_from`.
- `session/import {path|jsonl}` → translate pi-style JSONL lines (role/content/tool calls) into message rows in a new session; unknown entry kinds → skipped with counts in the result.
- `session/export {session_id, format:"html"}` → standalone HTML (styles inlined, no network refs); file written to workspace exports dir; face shows path.

### 5.5 Compaction (RQ-CMP)

- `CompactSession(ctx, sessionID, opts {Instructions string})` — instructions join the summarizer prompt as an emphasis section (existing `summarizer models` seam at `compaction_middleware.go`).
- `context/compact` gains `instructions?: string`.
- TUI `/compact <free text>`; GUI CompactionSettingsCard button moved/exposed at chat level with an input.
- Per-model overrides: `CompactionPolicy` gains `PerModel map[string]CompactionOverride{TriggerPercent, KeepRecent}` resolved at `EffectiveMaxTokens`/`TriggerTokens` time.
- File tracking: summary prose gains a `Files:` manifest (read/modified lists) — extends the existing `FoldRetainsFileContextSnapshots` mechanism to name paths.
- Overflow recovery: provider error class `context_overflow` or `stopReason=="length"` mid-run → auto `CompactSession` once + continue (one retry, journaled `context.compacted{mode:"overflow-recovery"}`); repeated overflow → fail the run honestly.

### 5.6 Tool plane (RQ-TOOL)

- Protected-tool upgrades are T1 edits (`internal/tools/bash.go`, `commandline`, `execute`): env injection `VIVY_SESSION_ID/VIVY_PROVIDER/VIVY_MODEL/VIVY_THINKING`, `shell_command_prefix` config, `!!` prefix → `execute` with `include_in_context:false`, oversized output → spill file under instance scratch + tail excerpt + path in result.
- Exposure: `domain.ToolSpec` gains `Exposure` (`direct|model-only|deferred|hidden`, default `direct`). Model-visible catalog filters non-`direct` at prompt build; `deferred` tools remain listed to `tool_search`; activation is a control action `tools/activate {ids}` that flips session-scoped visibility (Journal event `tools.exposure_changed`).
- `coding-tools` module (`plugins/coding/tools`): `tool_search` (BM25 over the ToolWorld catalog descriptions — tiny in-module index, no dep), plus `find`/`ls` parity aliases only if real gaps remain after review.
- TUI renderer registry: `sdk/tui/view` gains `RegisterToolRenderer(name, func)` with a default renderer; modules don't supply renderers in v1 (renderers are face-local; a renderer registry in the face keeps it simple — documented as face-internal, not a Port).

### 5.7 Model host (RQ-MDL)

- Thinking levels: provider-profile schema gains `thinking` block `{supported_levels, default, clamp, sampling_by_level}`; `internal/provider` maps to Anthropic `thinking.budget_tokens` / OpenAI `reasoning_effort` (+ sampling params). `/thinking <level>` extends the existing command; `auto|on|off` remain aliases (`on`→medium default).
- Cache warming: `internal/modelhost` scheduler. Trigger points: after each model stream settles (`streaming` mode) and on an idle timer (`idle` mode). Warm request = current prompt prefix + `cache_control` breakpoint (Anthropic adapter already emits `AutoCacheControl`); response discarded; usage folded into session stats (`cacheWrite`/`cacheRead` already on the wire). Gate: only for models declaring a cache lifetime in their profile and only when estimated avoided-miss cost exceeds the configured floor (pi's $0.05).
- Model UX: `Ctrl+P`-style cycle through a `scoped_models` list (config + `/scope-model` toggle), `/model` saves a per-project default.

### 5.8 TUI (RQ-TUI)

- Themes: `~/.vivy/themes/*.json` + `theme: auto|light|dark|<name>` in settings; `sdk/tui/view/styles.go` palette becomes a theme struct; terminal background detection for `auto`.
- Keybindings: `keybindings.yaml` mapping action names → key chords; all current hardcoded keys become named defaults.
- Search/jump/copy/editor/listing/OSC8: face-internal changes only.
- Inline images: kitty/iTerm2 escape-sequence writer; behind `images: true` terminal capability check. Risky item — own story, flag-gated.

### 5.9 Bundle (RQ-BND)

`plugins/coding/` hosts: `tools` (tool_search + parity extras), `exposure` is kernel-side (ToolSpec field, not a middleware — middleware sees calls at execution time, it cannot hide tools from the model; ToolHost owns model-visible filtering), `session-extras` (control actions for tree/clone/import/export if they can be expressed as actions; storage query stays kernel, actions are thin adapters — decide per action in story), `local-llm`, `ui` (GUI additions). `recipes/vivy-code.vivy.yml` selects the family; Inspect inventory proves the bundle.

### 5.10 Provider scope (VCP-O2/O3)

Provider work is confined to the Anthropic and OpenAI adapter paths already in `internal/provider`. OAuth is explicitly not built here. Local models arrive through OpenAI-compatible endpoints (§5.9 `coding-local-llm`).

## 6. Open design points owned by stories (not blockers for the package)

- B1: exact steering injection mechanism (in-loop vs early-settle) — decided by the mandatory Eino capability check.
- A4: whether `bash` RPC command may reach the governed shell or is refused — decided by sandbox contract review.
- C1: whether `session/tree|clone|export` land as core RPC or `coding-session-extras` control actions — ActionHost can invoke them if a Module can hold a Journal-read facade; kernel RPC is the fallback.

## 7. Acceptance definition

Per Epic: observable requirement rows above verified end-to-end at the TUI (headless RPC where noted) plus `just ci`. Whole-program acceptance: the §5.3 command map runs green against `vivy-code --mode rpc` driving a live session end-to-end; the gap-report v1 table is re-scored with ✅ on every in-scope row.
