# Pi coding-agent functional baseline

Source baseline: [earendil-works/pi][pi], revision
`f321e5bdd26b3624bfb2ab182a334d0a5edb698a`, coding-agent package `1.4.0`, inspected
2026-10-24. Rig comparison revision:
`e1cd47d57f0bccb50ddffd6f45de2ac3365b271c` (`0.46.0-rc.1`).

On 2026-10-31 the local Pi reference was refreshed to
`a14cdf90f0a76532f3e20bc3c01872837cef24ba` for durable design preparation.
The coding comparison results below retain their original pins; they were not
rerun against that refresh. See [the handoff](HANDOFF.md#durable-design-preparation-and-pi-refresh-2026-10-31).

The user requested the complete coding-agent capability range, then prioritized
a minimal working experiment with gateway access and no standalone SDK or species
hash system. Keep the wider range visible without making it a first-run gate.
Rust functional equivalents are accepted; existing TypeScript extensions and
Pi's exact wire/source API compatibility are not requirements.

**Owner acceptance, 2026-10-27:** the agent fixed one of its own bugs in a real
coding task. The owner confirmed the chain was stable and explicitly accepted
the initial coding path. That gate is closed. The dated comparison below
retains its original automated-test scope and provider-specific limitations;
it does not reopen the accepted real-task gate or establish full Pi parity.

**No row has reached Pi parity.** The Result column records what was measured.
Core rows (the Rust-owned coding core, per DESIGN's core boundary) were compared
on 2026-10-25 by `scripts/compare_pi.py`: Pi at the pinned revision, run from
source, and vivy, each against the same scripted Chat Completions gateway,
capturing every request (`.generated/compare/`). Scenarios are named in
brackets. A scripted gateway shows what each agent sends and does; it is not a
real-gateway result, and the real gateway (OpenRouter) is still unverified
because this environment's network policy denies it. Pi's dependencies came
from `npm ci --ignore-scripts`; its generated model data, which the same policy
kept from being fetched, came from the published `@earendil-works/pi-ai@1.4.0`
package. Pi document paths are relative to `.workspace/pi/packages/coding-agent/`
unless stated otherwise.

The Windows delivery provides the native `vivy` launcher, Git Bash process-tree
cleanup, TUI session switching and the optional Python workbench; its commands
and verification are in DESIGN's "Windows coding species and Python workbench"
section. The owner reported a successful manual Windows run; the comparison
above ran on Linux. Python is an auxiliary tool, not Pi JavaScript Code Mode
parity.

| Area | Pi behavior to retain in the coverage map | Rig direction / important gap | Pi source entry | Result (2026-10-25) |
| --- | --- | --- | --- | --- |
| Agent execution | Streaming text/thinking, tool calls, cancellation, retries, usage, sequential/parallel tools, queued steering and follow-ups | Reuse an existing Rig runtime; verify event ordering, queue behavior and failure handling. | `../agent/src/agent-loop.ts`, `docs/how-pi-works.md` | [text, tools, failure, cancel] Same streamed loop: tool call, result, next request; a 401 exits 1 with the gateway's message on both. Ctrl-C to the process group during `sleep 300`: vivy exits 130 and kills the tool; Pi `-p` dies on the signal and leaves the `bash` child running. Pi sends `max_completion_tokens` (the model maximum) and `stream_options.include_usage` on every request; vivy sends neither until near the window (then `max_tokens`), so usage reporting on the real gateway needs checking. Signalling only the `vivy` launcher's PID does not stop the agent. [`sessions_tui.py`, vivy] Enter during a running turn steers it (the message reaches the model in that turn); a 503 with `Retry-After` is retried automatically and the turn then answers. Not exercised: parallel tools, Pi's follow-up queue. |
| Coding tools | Read including images; edit/write; shell; optional grep/find/ls and native Windows PowerShell; truncation and full-output spill | `rig-tools` and basics coding plugins; exact defaults, limits and PowerShell are gaps to check. | `src/core/tools/`, `docs/cli.md` | [tools] The same five calls succeed on both; the file ends as `beta`. `read` and `write` schemas match; `edit` matches except `old_text`/`new_text` for Pi's `oldText`/`newText`; vivy's `shell` takes `command, cwd, timeout_secs`, Pi's `bash` `command, timeout`. vivy also offers `search`, `list` and `python_repl` (Pi's grep/find/ls are off by default). Result texts differ: `Created notes.txt.` / `Successfully wrote to notes.txt`; vivy's edit returns the changed lines, Pi's a count; vivy's shell appends `[exit code 0]`; a missing file is `…: no such file` / Pi's raw `ENOENT …`. Not exercised: image reads, truncation and spill, PowerShell. |
| Terminal interaction | Streaming transcript, tool/diff views, multiline editor, history, completion, image input, model/reasoning picker, status and usage | Evaluate `rig-tui` before writing a terminal; compare actual interactions. | `docs/usage.md`, `docs/tui.md` | [tui] On a pseudo-terminal both views took a typed prompt, sent it and drew the reply. Not exercised: editor, history, completion, pickers, tool/diff views, image paste (needs a hands-on comparison). |
| Terminal customization | Themes, keybindings, custom rendering, widgets, dialogs, overlays, header/footer/editor hooks | Native Rust plugin equivalents; upstream UI extension points need mapping. | `docs/extensions.md`, `docs/themes.md`, `docs/keybindings.md` | Not core: extension surface, later. |
| Sessions | Persistent JSONL tree, resume/list/name/delete, ephemeral sessions, branch switching, fork and clone | Rig memory/session primitives exist; branch semantics and persistence are not automatically Pi-equivalent. | `docs/sessions.md`, `docs/session-format.md` | [continue] A second process continued the session on both (Pi `-c`, vivy `--resume <id>`); the history matches except the first message's role. [`scripts/sessions_tui.py`, vivy] In the terminal view: rewind (Alt+Up, Enter), `/fork`, `/agents`, `/tree` (opens the agents: Rig's branches are agents of one session, not Pi's tree file), `/clone` (a copy of the session, switched to), `/new`, the `/resume` picker, `/export` then `/import` into an empty session, the imported history reaching the model. In print mode: `--no-session` answers and keeps no session, `--fork <id>` continues a copy and leaves the original, `--session <id>` resumes. Not matched: Pi's tree navigation inside one session file, `/import` of Pi's own JSONL format (vivy imports its `/export` JSONL), `--session-dir`, session delete. |
| Context management | Manual/automatic/overflow compaction, branch summaries, context edits, preserved tool-call/result pairing | Reuse Rig compaction where it fits; verify retained state and branch projection. | `docs/compaction.md`, `docs/session-format.md` | [compaction] 64k window, ~10k-token replies: both compact at the same turn (~44k context). Pi sends a separate summarization request (its own system prompt, no tools, the conversation as text), then a second checkpoint request for the split turn, and keeps 8 messages. vivy sends Rig's in-place request (same system prompt and tools, history to the cut plus "Stop here … write a structured checkpoint", `max_tokens` 8000) and keeps 6. [step 3] Branch summary: on a rewind, vivy summarizes the messages left behind with a request of its own and adds Pi's "summary of a branch" user message to the fork; a refused summary adds nothing. vivy always summarizes; Pi asks first. Not exercised: overflow recovery, `/compact`. |
| Execution modes | Interactive, print, JSON event output, long-lived RPC control, embedding | Rig has terminal/print/JSON surfaces; its protocol is smaller than Pi's. Embedding can use normal Rust APIs. | `docs/cli.md`, `docs/json.md`, `docs/rpc.md`, `docs/sdk.md` | [all] Print (`-p`) and JSON events (Pi `--mode json`, vivy `--json`) on both; the terminal view on a pty [tui]. Not compared: Pi's RPC mode (vivy's `--json` is Rig's smaller line protocol), SDK embedding. |
| Configuration and trust | User/project settings, precedence, trusted project resources, CLI overrides | Reuse settings infrastructure; project trust is not a shell sandbox. | `docs/configuration.md`, `docs/security.md` | [resources, context; `sessions_tui.py` trust] Project trust now matches Pi: project skills (`.rig/skills`, `.agents/skills` in the working directory or above), prompt templates (`.rig/prompts`) and `.rig/SYSTEM.md`/`APPEND_SYSTEM.md` load only when trusted; AGENTS.md and CLAUDE.md load regardless. Decision order as Pi: `--approve`/`--no-approve`, the nearest saved decision (`RIG_HOME/trust.json`, `/trust [yes|no|forget]`), then `[trust] default` (`ask`, `always`, `never`). Print mode with "ask" skips them on both agents and `--approve` loads them; the terminal view asks at start, and trusting restarts into the session with them loaded, remembered for the next start; declining leaves them out. Difference: an unknown slash command (an untrusted template) is refused by vivy, sent to the model as text by Pi. Not compared: settings precedence beyond `[trust]`. |
| Project context | AGENTS/override/context files, system prompt overrides and additions | Rig's AGENTS plugin is a starting point; compare discovery and precedence. | `docs/configuration.md`, `src/core/resource-loader.ts` | [text, context] Both read the same context files: the agent directory's, then every directory from the outermost down to the working one, the first of `AGENTS.override.md`, `AGENTS.md`, `CLAUDE.md` in each (the override wins only in its own directory). `SYSTEM.md` replaces the default prompt and `APPEND_SYSTEM.md` adds to it (Pi `.pi/`, vivy `.rig/`, the project over the agent directory); `--system-prompt` and `--append-system-prompt` stand in for the files. Same order on both: prompt, appended text, context files; Pi wraps the addition in `<addendum>`, vivy in `<append_system>`. Both need project trust for the project's `SYSTEM.md`/`APPEND_SYSTEM.md`. Default prompts differ: Pi 2,948 characters as `developer`, vivy 1,250 as `system`. |
| Skills | Metadata discovery, lazy body loading, supporting files, explicit skill invocation | Rig skills plugin exists; verify discovery and model exposure. Pi's documented experimental `allowed-tools` is not an enforced sandbox. | `docs/skills.md` | [resources] A project `.agents/skills/demo-skill` is listed in the system prompt on both (both after trust). Not compared: lazy body loading on use, explicit skill invocation. |
| Prompt templates | File templates, metadata, arguments, expansion and invocation | Rig corpus examples are reference code, not a shipped default product feature. | `docs/prompt-templates.md` | [resources] `/review the API` expands identically on Pi and vivy, both after trust: `$1` is `the`, by Pi's argument splitting. Not compared: argument-hint display, nested and package templates. |
| Extension lifecycle | Input/prompt/context hooks; model/tool events; session, compaction and trust hooks; reload behavior | Preserve useful behavior through Rust hooks. Native code changes rebuild/restart; no TS loader. | `docs/extensions.md` | Not core: later, after delivery step 4. |
| Extension contributions | Tools, commands, shortcuts, flags, custom messages, events, dynamic tool exposure, branch-aware state | Avoid a giant plugin API; implement typed contributions as real consumers appear. | `docs/extensions.md` | Not core: later, after delivery step 4. |
| Extension distribution | Install/update/remove/list reusable packages and bundled resources | Later Rust/local/git/registry equivalents; package manager and marketplace are not needed for the first recipe. | `docs/packages.md` | Not core: later. |
| Provider/model access | API keys, provider protocols, subscription OAuth, custom providers, model catalogs and capabilities | Gateway first; broad direct-provider/OAuth parity explicitly deferred. | `docs/providers.md`, `docs/models.md`, `docs/custom-provider.md` | [all] Both reach a custom Chat Completions endpoint with a bearer key and the model ID unchanged (`org/model-1`). The real gateway is not yet called: the environment denies `openrouter.ai`. |
| Model behavior | Reasoning settings, usage/cost, prompt caching, model switching, virtual model routing | Establish the gateway's actual support. Reuse routing/capabilities instead of duplicating a catalog. | `docs/models.md`, `docs/virtual-models.md` | [thinking, image] Pi `--thinking high` sends `reasoning_effort: "high"`; vivy `/effort high` with `dialect = "openrouter"` sends OpenRouter's `reasoning: {"effort": "high"}`. Pi sends `reasoning_effort: "medium"` by default on a reasoning model, summary requests included; vivy sends none until a level is chosen. Both send an `image_url` part (Pi after the text, vivy before). vivy's costs come from `[gateway]` prices (step 1); Pi's not compared.  [`sessions_tui.py`, vivy] Pi's names work: `/thinking high` and `--thinking high` set the level sent; Shift+Tab cycles the level (as in Pi); model cycling is Alt+P (Pi: Ctrl+P, which Rig's view keeps for prompt history; Pi's own Windows key is Alt+P) across the gateway's `models`, and the next turn goes to that model. Not matched: Pi's `/scoped-models` selection and `--models`/`--list-models`. |
| Specialized model services | Classification and image generation/editing through appropriate services | Later capability, gateway/provider dependent; not implied by chat completion support. | `docs/codemode.md`, `docs/custom-provider.md` | Not core: later. |
| Local model workflow | llama.cpp discovery and model management | Later; not a requirement to create a second direct-provider stack initially. | `docs/llama-cpp.md` | Not core: later. |
| MCP connections | stdio and Streamable HTTP, configuration, connection lifecycle, OAuth, tool changes/progress | Planned: official Python MCP SDK in the optional workbench; managed client lifetime, reset and correlated calls. No parallel Rust MCP manager. Broad OAuth/configuration UX remains a gap. | `docs/mcp.md` | Not core: Python workbench (delivery step 4, only for servers that are not Python libraries). |
| MCP tool/resource use | Namespaces, direct/deferred/Code Mode/hidden tool exposure, resource lists/templates/read, search | Planned: bounded discovery/call from Python first. Resource UX and all exposure modes remain gaps; a compiled workbench does not imply compiled Python dependencies or complete MCP parity. | `docs/mcp.md`, `docs/codemode.md` | Not core: Python workbench. |
| JavaScript Code Mode | Model-facing JS tool orchestration, tool discovery, bounded output, cancellation, branch-aware store/load | Accepted Python functional counterpart, no JS/TS runtime compatibility. Current persistent worker is partial; host tool bridge, managed discovery and branch-aware durable store/load remain unimplemented. | `docs/codemode.md` | Not core: Python workbench; no JS runtime. |
| Session utilities | Export/import, HTML output, copy/share, diagnostic/bug-report workflows | Add useful equivalents later; keep sensitive transcript sharing explicit. | `docs/cli.md`, `docs/slash-commands.md` | Not core. Implemented in vivy: `/export` (HTML, JSONL), `/copy`, `/session`; `--export <id> [output]` exports a session without opening it, as Pi's; `/import` loads vivy's own `/export` JSONL; no share or bug report. |
| Platforms | Linux/macOS/Windows behavior, shell handling and terminal integration; documented Termux usage | Current host is Windows; inspected Rig launcher supports Linux/macOS. Do not claim portability without a run. | `docs/windows.md`, `docs/termux.md`, `docs/terminal-setup.md` | Linux: everything above. Windows: the owner's manual run only; not compared. |

## Native delegation addition (2026-10-25)

The `feat/subagents` delivery uses Pi's `examples/extensions/subagent/` as a
functional reference, not a new default-core parity claim. Vivy's selected Rig
wrapper implements single tasks/follow-up, named roles, parallel batches and
chains, per-child cwd, bounded inline results, cancellation, native persistence,
usage and terminal views. Unlike Pi's example it uses the same host process;
cwd stays within one project and shares project context. No TypeScript extension
runtime, separate worker process or OS sandbox is claimed. Offline tests,
actual species and Windows terminal checks are recorded in
[delivery evidence](logs/2026-10-25-subagents.md). Earlier Linux comparisons above
remain dated evidence and were not rerun against Pi for this addition.

## Findings that prevent misleading scope claims

- Vivy's selected collaborative Plan plugin is an owner-approved addition beyond
  this Pi core baseline. It supplies guidance and a terminal-reviewed exit,
  without changing tool permissions. See DESIGN's "Collaborative Plan mode";
  it does not establish general extension or autonomous Goal parity.

- This Pi revision includes MCP and JavaScript Code Mode. Older descriptions of
  Pi as having no built-in MCP no longer describe this source baseline.
- Pi's branch tree affects extension state, compaction, Code Mode storage and
  resumed context; implementing a flat message log alone does not match it.
- Pi's CLI and its bare embedding factory do not automatically initialize the
  same built-in extensions. Test the surface being claimed.
- Subagents, plan mode, permission systems, OS sandboxes and git checkpoints can
  be extensions/examples. Pi's README does not claim subagents or plan mode as
  built-in defaults. The entire community extension ecosystem is not a finite
  parity requirement.
- `packages/durable` is a separate experimental Pi library. Do not quietly add
  its durable scheduler/replay contracts to ordinary coding-agent parity or
  assume the coding-agent already uses them.
- Rig's `corpus/` contains useful examples. An example's existence does not mean
  the default harness provides that feature. Classic `rig-agent` memory saves
  successful completed runs; failure/cancellation durability needs deliberate
  handling if required by the selected runtime.

Update this same table with implementation evidence and explicit decisions as
work progresses. A postponed feature is not a completed one. Do not create a
second feature inventory or estimate the expanded scope from earlier small-spike
estimates.

[pi]: https://github.com/earendil-works/pi/tree/f321e5bdd26b3624bfb2ab182a334d0a5edb698a/packages/coding-agent
