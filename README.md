# Vivy

Vivy is a Rust agent core for coding sessions and persistent assistants.
**One YAML recipe selects components; one build assembles a runnable agent.**
It uses Rig's execution path, a native terminal interface, configurable
OpenAI-compatible and Anthropic-compatible endpoints, and an optional Python
workbench for tools, memory and executable Skills.

This is the official Rust Vivy project at
[ProjectViVy/agent-vivy](https://github.com/ProjectViVy/agent-vivy). The previous
Go implementation is archived and is not a build or runtime dependency.
Pi's coding workflow remains a reference, not a second runtime.

Windows acceptance includes synthetic gateway, tool, persistence and evolution
checks. A narrow real StepFun foreground probe also passed; full real-provider
tool/evolution acceptance and economical multi-day operation remain unverified.

The coding species includes a recipe-selected Vivy wrapper over Rig's native
delegation. See [verification and limits](docs/logs/2026-10-25-subagents.md).

## Build and run on Windows

Prerequisites: Rust 1.97.1 and MSVC C++ build tools, Git for Windows, and Python
3.11+ when the recipe includes Python. Run these commands from this checkout:

```powershell
# Short paths avoid MSVC path/command-line limits in native dependencies.
$env:CARGO_TARGET_DIR = 'C:\vt'
$env:CARGO_PROFILE_DEV_DEBUG = '0'
cargo run -p vivy -- build recipes/coding-agent.yaml

& C:\vt\debug\vivy.exe                    # interactive terminal
& C:\vt\debug\vivy.exe -p 'Fix this bug'   # one task
& C:\vt\debug\vivy.exe --resume <id>       # resume a saved session
& C:\vt\debug\vivy.exe --json              # Rig's JSON protocol
```

`vivy` starts the already-built baseline species. Build explicitly after a
recipe or source change. This is a checkout-local development tool, with no
installer or automatic rebuild. Keep `CARGO_TARGET_DIR` consistent between build
and launch. Launching preserves the current working directory; resuming uses
the session's saved directory. Add the build directory to your PATH if desired.

### One-click Windows start

Double-click `start.cmd` to open the interactive coding terminal in this
checkout with the Windows console set to UTF-8. It reuses an existing build and
does not build automatically: it looks for `vivy.exe` and `coding-agent.exe` in
`%CARGO_TARGET_DIR%\debug`, `.generated\target\debug`, `target\debug` and
`C:\vt\debug`, preferring a directory that holds both. If `Desktop\keys.txt` or
`Desktop\key.txt` exists, it reads three or four non-empty lines (gateway URL,
model ID, API key, and optionally the Rig dialect such as `deepseek`). It
supplies a first-run hint to the host, which persists it in the ignored
`.workspace/launch-home/settings.toml`; existing managed provider settings take
priority. The key passes through `RIG_SECRET_GATEWAY` in process memory. Sessions
use that isolated `RIG_HOME`. Without a Desktop key file, the launcher uses
`RIG_HOME` (default `~/.rig`). An empty home can start and configure providers.

`vivy.exe` records this checkout's path at compile time, so a copied or moved
tree keeps pointing at the old location even when its `target` directory came
along. Rebuild after a move: `cargo clean -p vivy` and `cargo build -p vivy`.

### One-click Windows build

Double-click `build.cmd` after a recipe or source change; `start.cmd` only
launches what is already built. It builds into the `CARGO_TARGET_DIR` that
`start.cmd` looks at first, else `.generated\target`, keeps
`CARGO_PROFILE_DEV_DEBUG=0`, and prints the built binary. It takes an optional
recipe path, defaulting to `recipes/coding-agent.yaml`. It stops before
compiling when that build's `vivy.exe` or agent is still running, because
Windows cannot replace a running executable.

## Persistent assistant

`recipes/persistent-agent.yaml` selects the D4 assistant: durable tasks and timed
wakes, Laputa's six-file core persona, asynchronous Recap-only context, and
AutoDream validation and adoption. Coding sessions and persistent assistants are
separate recipes on the same core. The assistant does not yet delegate coding
sessions or subagents. The persona is global and editable under
`<LAPUTA_HOME>/personas/main/` (default `~/.laputa/personas/main/`), and for the
desktop under Settings → Core persona. On first launch the desktop tests a
provider, asks once who you are and what your mission is, extracts the six files
and writes them directly; an unclear description gets a random name and the
built-in persona. A final workspace step then offers to prepare the Python
workbench and native memory together, with memory pre-selected; either can be
skipped and completed later under Settings → Runtime.

This recipe still requires the local Rig checkpoint implementation recorded in
the [handoff](docs/HANDOFF.md#d4-autodream-ad-2-2026-11-01); the ordinary upstream
pin alone does not provide its persistence interfaces. The official repository
promotion does not publish that separate dependency fork. With a prepared checkout:

```powershell
cargo run -p vivy -- build recipes/persistent-agent.yaml --rig-path <rig-checkout>
cargo run -p vivy -- run recipes/persistent-agent.yaml --rig-path <rig-checkout>
```

Use D4 sessions with this build; preserve older executables and sessions. A live
terminal or JSON host delivers deadlines; print mode saves the wait and exits.
Background summary work yields to foreground input. Long-term memory uses explicit
tools, and idle reflection is off by default. Humans can edit the assistant,
coding, Recap and Dream instructions as [Markdown prompt files](prompts/README.md).

## License

The repository retains its [GNU GPL v3 license](LICENSE). Third-party notices,
including the [Prime Intellect license](plugins/vivy-python/PRIME-LICENSE), are
retained with the corresponding code.

## Providers and runtime settings

Start the terminal and run `/provider`. Add a built-in preset or a custom
OpenAI/Anthropic-compatible endpoint, configure its models and credential source,
and save. Only added instances enter the model catalog. The editor supports
provider/model creation, edits and deletion; changes stay in a draft until saved.
The instance ID stays fixed so sessions continue to identify the same account.

- `/login <instance>` opens hidden API-key entry and selects saved-key mode.
- `/logout <instance>` removes its saved key; environment variables are unchanged.
- `/model` searches all configured models; a provider's menu filters its models.
- `/model-default` explicitly saves the current model and thinking level for new
  sessions. Temporary switches do not change defaults; resumed sessions retain
  their recorded choices. `--model instance/model-id` overrides either.

OpenAI uses Chat Completions; Anthropic uses Messages. Subscription login and
Responses are not included. Presets reuse Rig's built-in metadata and option
mappings; they do not establish compatibility with an untested endpoint.

Edit `RIG_HOME/settings.toml` (default `~/.rig/settings.toml`):

```toml
[providers]
default_model = "company/org/model-1" # also saved by /model-default

[providers.instances.company]
protocol = "openai"                # or "anthropic"
base_url = "https://gateway.example/v1"
preset = "openrouter"              # optional known compatibility mapping
credential = { source = "env", name = "COMPANY_API_KEY" }
# Alternatives: { source = "stored" } or explicit { source = "none" }

[[providers.instances.company.models]]
id = "org/model-1"                 # sent to the endpoint unchanged
context_window = 131072
max_output_tokens = 8192
tools = true
image_input = true
reasoning = ["low", "medium", "high"]
input_price = 0.5                  # USD per million tokens, both or neither
output_price = 1.5

# Optional overrides; defaults discover Git for Windows and use `python`.
# [shell]
# executable = "C:/Program Files/Git/bin/bash.exe"
# [python]
# executable = "C:/path/to/python.exe"
```

```powershell
$env:COMPANY_API_KEY = '...'  # never store the key in this repository
& C:\vt\debug\vivy.exe
```

An explicit credential source never falls back to a different account. Saved
keys live under `RIG_HOME/auth/provider-keys/`, outside settings and sessions.
Configured credential variables and `RIG_SECRET_*` are withheld from owned
build/describe/tool processes. Adding credential variables resets existing
Python workers so subsequent calls use the updated filtering policy.

Provider/model lists reload file edits. An invalid file retains the last valid
catalog; external edits conflict with an open draft rather than being overwritten.
Active turns finish on their existing connection; idle agents and subagents
reconnect after a provider changes. A removed provider needs explicit repair.

Built-in models inherit Rig's catalog; `models` adds/overrides individual rows,
and `disabled_models` excludes removed preset models. Generic OpenAI endpoints
need a known compatibility preset before declaring reasoning levels. Models are
entered manually; automatic model discovery is not included.

Existing `[gateway]` settings remain supported as `gateway/<model>` with
`RIG_SECRET_GATEWAY`. A provider save migrates that table into the same instance
collection. An explicit `providers.instances.gateway` takes priority.

### A provider's own API, with no gateway in between

`[gateway]` is any OpenAI-compatible endpoint, so a provider's own API serves
as one: nothing is proxied, and the vendor's base URL replaces the gateway's.
Rig ships DeepSeek's request handling as the `deepseek` dialect — it drops a
forced tool choice while the model thinks, takes strict tools under `/beta`,
refuses `json_schema` response formats, and reads `reasoning_content` back —
so name it rather than leaving the generic `gateway` dialect:

```toml
[gateway]
base_url = "https://api.deepseek.com"
model = "deepseek-v4-flash"     # or deepseek-v4-pro, or the deepseek-flash alias
models = ["deepseek-v4-pro"]    # optional: more ids for /model and Alt+P
dialect = "deepseek"
```

The key stays the agent's `gateway` secret `RIG_SECRET_GATEWAY`, which the
launcher reads from the third line of the Desktop key file; the same
`dialect = "deepseek"` is its optional fourth line.

An explicit `--model` takes precedence; a resumed session keeps its model;
otherwise the configured gateway model is selected. Its raw model ID is sent
unchanged. Build and execution children do not inherit `RIG_SECRET_*`.

`/new`, `/resume <id>`, `/session`, and `/quit` use Rig's terminal and session
mechanisms. A session switch in print mode returns Rig's exit code 75 rather
than replaying the prompt. Rig's JSON interface is not Pi RPC compatibility.

Pi's session options work too: `--no-session` keeps no session, `--fork <id>`
continues a copy of one, `--session <id|path>` resumes one; `/clone` copies the
running session, `/import <file.jsonl>` loads an `/export` file into an empty
session, and `/tree` shows a session's branches. Project trust works as in Pi: the project's own skills, prompt templates and
`.rig/agents`, `.rig/SYSTEM.md`/`APPEND_SYSTEM.md` load only once it is trusted (`--approve`
for one run, `/trust` or the start question to remember; `[trust] default` in
`settings.toml` is `ask`, `always` or `never`).

In the terminal view, `/resume`
alone opens the session picker; `/thinking` and `--thinking <level>` are Pi's
names for `/effort`, Shift+Tab cycles the reasoning level and Alt+P the model
(configure models through `/provider`), and `--export <id>
[output]` exports a session from the command line; Alt+Up then
Enter rewinds to an earlier message, and the branch left behind is summarized
into the fork (Pi's branch summary); `/fork` and `/agents` manage a session's
agents. `python scripts/sessions_tui.py <vivy>` checks these on Linux/macOS.

## Terminal presentation

The recipe's `terminal` component selects `vivy_basics::tui::VivyTuiPlugin`:
a compact Codex-inspired screen built from Rig's native terminal widgets.
The welcome and window title identify **Vivy**. The footer remains visible
while typing, with activity, Plan, model, reasoning and available usage data;
narrow windows keep activity and Plan first. Tool calls show readable commands,
their full output and failure causes; file edits retain their complete diffs.

The editor, command/file completion, scrolling, questions and session controls
keep their existing keys. Enter sends, Shift+Enter or Ctrl+J adds a line, and
backslash then Enter is a multiline fallback. During work, Enter steers,
Tab queues and Esc stops. `NO_COLOR` disables colors. Removing `terminal` and
rebuilding excludes `rig-tui`; the default Vivy identity also works in
print/JSON mode. Configuration paths and `RIG_HOME` remain compatible.

`python scripts/branding_tui.py <vivy>` checks the compact screen at 120x40 and
50x20 using a scripted loopback gateway and a real pseudoterminal. It uses the
same test-only dependencies as `plan_tui.py`; set `VIVY_TUI_EVIDENCE` to capture
synthetic screens and terminal streams outside Git.

## Native subagents

The recipe's `subagents` component adds `task`, `message`, `wait`,
`task_parallel(tasks)` and `task_chain(steps)`. Ask the model to delegate, for
example: "Use two scout subagents to investigate the parser and the launcher,
then summarize both results." Tasks accept `description`, `prompt` and optional
`profile`, `model`, `effort`, `tools`, `cwd`, `environment`.

`/roles` lists scout/planner/reviewer/worker and Markdown roles from
`RIG_HOME/agents/*.md`, followed by trusted project `.rig/agents/*.md`. Optional
YAML frontmatter supports `name` (matching the filename), `description`, `model`,
`effort`, `tools` (array or comma-separated string); the body is the role prompt.
Files refresh on delegation; existing children retain their role snapshot.

Parallel calls accept 1–8 tasks; at most four children run per session, including
follow-ups. Chains pass the previous bounded answer through `{previous}` and
stop on failure. `/agents` opens child conversations and `/usage` includes their
model calls. In the terminal, Esc stops the turn and its active/queued children.
Resume retains completed child conversations; interrupted work is not replayed.

Default nesting depth is one (`[subagents] max_depth = 1`). Tool selection stays
within parent/role access. Scout and planner select read/search/list; reviewer
also has shell; worker inherits parent tools. Children share the process and
project context; `cwd` stays inside the project. These are not OS sandboxes or
separate Git worktrees. Give parallel writers distinct files. Remove the component
and rebuild to remove delegation; remove `terminal` for a headless agent.

The feature was verified separately with `CARGO_TARGET_DIR=C:\vt-subagents`.
After integration into local `main`, rebuild using `CARGO_TARGET_DIR=C:\vt`
as above; `start.cmd` uses that build and the Desktop gateway settings.

## Collaborative Plan mode

Use `/plan` to enter planning, `/plan <task>` to enter with a request, and
`/plan off` to leave. The terminal shows `PLAN`; a change during a running
turn takes effect at the next model request or when that turn ends.
Plan supplies model guidance and keeps the existing tools and permissions.
It is not a read-only execution mode.

Ordinary answers and clarification do not open a review. When ready, the
model calls `exit_plan_mode` with the complete Markdown plan. The terminal
shows `PLAN · REVIEW` and the full plan: Shift+Up/Down scrolls it, Enter
selects, and typed text supplies feedback. `Approve and execute` leaves
Plan on the next model request; `Keep planning` retains it. Esc stops the
turn and keeps Plan; `/plan off` also works when typed inside the review.
Submitted plans remain readable in session scrollback, without a separate
plan file. Saved mode resumes with the session; pending review is never
automatically approved. Without an interactive reviewer, submission stops
with an explicit message and leaves Plan enabled.

## Optional Python workbench

The recipe's `python` component adds `python_repl(code, timeout_secs?)` alongside
the native coding tools. Each live agent owns a persistent Python namespace and
asyncio loop, including top-level `await` and tasks that survive cell completion.
The default timeout is 60 seconds. Background output keeps its original call ID;
unattributable native/thread output is retained in the event journal.

Startup checks the environment in the background, then guides project trust,
provider configuration, Python and optional MemPalace in order. `/setup` reopens
the guide; `/setup provider`, `/setup python` and `/setup memory` open one item.
Skips and installation grants are remembered under `RIG_HOME/initialization`,
indexed by canonical project path. They do not travel with the repository.
MemPalace starts disabled. The desktop first-run workspace step pre-selects it;
that choice is stored in the same project consent files, not in `settings.toml`.
Consent to install does not grant project trust.

Approving Python installs hash-locked dependencies and trusted Python Skills,
validates them, then publishes `.vivy/workbench/current.json`. The same approved
scope can repair automatically; changed locks, Skill metadata, sources or enabling
memory need approval again. Failed automatic preparation is retained for manual
retry, including after restart. `/python prepare` remains an explicit one-shot
preparation command; it does not silently grant future automatic installation.
`/python close` cancels. Concurrent preparation reports busy; failure preserves
the old environment. Ordinary Python calls do not install packages.

Automatic detection also requires a locally recorded successful validation of
the published environment. After upgrading an older workbench or copying a
repository, run `/setup python` or `/python prepare` once to establish that record.
An unvalidated old environment is not executed by startup detection or reused
by preparation; preparation creates a fresh candidate instead.

On Windows x64, missing tools can be installed into `RIG_HOME/toolchains`:
SHA256-verified uv 0.11.7 and managed CPython 3.13.12, without modifying PATH,
registry or the system default Python. Suitable existing tools are reused. An
explicit broken `python.executable` produces an error; `/setup python` offers
managed Python as an explicit alternative. Other platforms still need existing
uv and Python; automatic bootstrap has only been validated on Windows x64.

Provider editing offers **Save only (unverified)** and **Save and test**. Testing
uses a fixed small streaming request and a harmless tool call when the model
declares tools, never conversation content. It has a cancel control and 45-second
deadline. Restart does not test the network automatically; failed tests retain
the saved configuration. Models without credentials can use `source="none"`.

`/python status` reports initialization and refreshes its bounded local check.
`/python reset` refreshes trusted Skill discovery and starts fresh on next use.
`/python close` releases this agent's worker. Neither command removes artifacts
or committed memories. A newly published environment is adopted at the next safe
cell boundary, with a fresh-variable notice; manual reset is not required.
Old and failed environment directories are retained; remove only unused ones
after all workers have closed. No live-resource migration or automatic replay.

The terminal's right footer alternates telemetry and Python/memory status every
five seconds. Preparation and infrastructure errors pin the workbench page;
recovery stays visible for five seconds. `Py prepared` means a published interpreter
exists, while `Py ready` means a live initialized worker. `Mem not init` is disabled
or unvalidated; `Mem ready` means preparation validated the native engine and
assets, independently of whether a worker is running. No periodic database health
query runs. `/python status` refreshes the bounded environment check.
Rotation never installs dependencies or starts Python. Activity, PLAN and stop
hints remain visible; a recipe without Python has no Python detection or rotation.
Provider missing/error/testing states share this right-hand slot.

Cancellation and timeout interrupt the running cell, allow bounded unwinding,
then terminate its owned process tree. Ordinary exceptions preserve variables.
Compaction retains the live worker; separate agents, forks, session switches,
process resumes and exit have separate/fresh namespaces. Python, trusted Skills
and shell run with the user's permissions; these are not sandboxes.

The trusted inventory is shared with Rig's Skill prompt. Markdown instructions
remain lazy: `workbench.list_skills()` and `workbench.instructions(name)`. A Skill
directory with `pyproject.toml` is installed during prepare and imported before
the first cell. Its module defaults to the project name with `-` changed to `_`.
`[tool.vivy.skill]` can declare `module`, `binding`, and `requires_host` (only
`mcp.config` and `mcp.list_connections` currently exist). A module's callable `run`
becomes a callable binding; otherwise the binding is the module. Broken imports,
missing host handlers, collisions and reserved names are reported as unavailable
without hiding other Skills. `workbench.readiness` distinguishes installed MCP
and prepared native memory from unavailable capabilities. Explicit library
imports are ordinary Python imports.

Configure MCP in user settings, using environment-variable references for secrets:

```toml
[python.connections.dataset]
type = "stdio"
command = "C:/tools/dataset/.venv/Scripts/python.exe"
args = ["C:/tools/dataset/server.py"]
env = { DATASET_TOKEN = { env = "MY_DATASET_TOKEN" } }
startupTimeoutMs = 20000
callTimeoutMs = 60000

[python.connections.remote]
type = "http" # MCP Streamable HTTP
url = "https://example.com/mcp"
headers = { Authorization = { env = "MY_MCP_AUTHORIZATION" } }
```

`await mcp.list_connections()` returns `connectionId` entries without secrets.
Use `list_tools(connection)`, `search_tools(query, connection_id=...)`,
`describe_tool(connection, tool)`, `call_tool(connection, tool, arguments)`,
`reload(connection)` and `close()`. Connections open lazily and are reused.
Reload closes the selected generation; close ends the registry until worker reset.
Calls return complete MCP envelopes, including `content`, `structuredContent`
and `isError`. `McpToolError.result` retains an error envelope. Calls are never
automatically retried. OAuth, marketplaces and service-specific wrappers are absent.

Complete text, oversized repr/errors, MCP envelopes and MIME displays are saved
under `.vivy/workbench/artifacts`, partitioned by session, agent, generation and
call. Tool views are bounded; descriptors report path, bytes, media type and
completeness. `workbench.artifact(value, media_type)` saves explicit output;
use `application/json` for structured objects. A 256 MiB per-artifact/event-journal
limit and disk failures are explicit errors, never successful complete descriptors.
Reset retains files, but a process crash can leave an incomplete journal.

Native memory is optional:

```toml
[python]
memory = true
# memory_root = "C:/my-workspace/native-memory" # default: .vivy/workbench/native-memory
```

Then `/python prepare` installs the full pinned upstream MemPalace and Chroma
engine and explicitly downloads/probes pinned multilingual embeddinggemma ONNX
assets. The desktop onboarding workspace step enables the same path by default
without editing this file. Normal memory calls stay offline. Configuration, state/locks, model cache,
temporary files and palace data live under `memory_root`; no existing user palace
is imported. Preparation probes use separate synthetic palaces under `probes/`.

```python
record = await memory.remember("Dataset sum: 42", source_id="dataset-a", event_id="run-1",
                               wing="coding", room="analysis")
hits = await memory.search("dataset total", wing="coding", room="analysis", limit=5)
complete = await memory.get(record["id"])  # or source_id=..., event_id=...
await memory.close()  # reset the worker to reopen
```

Same source/event/content/partition deduplicates; changed content or partition
conflicts. Identical content from different sources remains distinct. Inputs are
limited to 1 MiB per explicit event; complete reads validate ordered chunks and
their content hash. The facade uses the native drawers collection's vector search
with explicit source/wing/room filters, not MemPalace's broader ingestion or hybrid
search orchestration. `limit` caps matching chunks before logical-record deduplication,
so it can return fewer distinct memories. Each worker serializes synchronous engine work off the
asyncio loop; cross-process locking surrounds open, operation and close, including
reads. Lock contention fails before writing. Interrupted synchronous writes can
have committed: inspect the same source/event explicitly; never replay blindly.
No MemPalace MCP server or L0 persona is loaded. Durable and automatic ingestion
remain outside this delivery.

Remove the Python component and rebuild to exclude its crate and embedded worker.
Compilation selection remains in [the recipe](recipes/coding-agent.yaml).
The owned runtime adapts Prime's bounded REPL/MCP mechanisms; see
[NOTICE](plugins/vivy-python/NOTICE.md) for the donor pin and retained MIT license.

## Project notes

- [AGENTS.md](AGENTS.md): lean development rules.
- [DESIGN.md](docs/DESIGN.md): assembly direction, implementation and verification.
- [PI-BASELINE.md](docs/PI-BASELINE.md): wider functional coverage target.
- [HANDOFF.md](docs/HANDOFF.md): reference clones and source navigation.

Clone Pi and Rig into ignored `.workspace/` as described in the handoff. **Do
not clone, vendor, depend on, or modify the original Vivy repository.** It is
under separate recovery work. Its assembly ideas are summarized in this project.

Keep the experiment small: no species hashes, standalone plugin SDK,
attestation, plugin marketplace, Studio lifecycle, or Pi TypeScript extension
compatibility. Ordinary Cargo locking and source pins remain dependency
management. Pi's wider feature set is an incremental target, not a first-run gate.
