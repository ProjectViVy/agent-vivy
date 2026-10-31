# Vivy feature inventory and coverage of the Rust experiment

Date: 2026-10-27. This is an inventory and decision aid, not an implementation
plan, a new acceptance gate, or an authorization to remove features. The owner
requested the Go product, feature branches and remake donors to be checked
against the current vivy-test direction. The earlier migration assessment did
not list A2A and several other surfaces; this inventory corrects that omission.
Owner decisions in the 2026-10-27 refinement supersede the earlier tentative
dispositions; the rows below are updated, and the reference findings are in §8.

Owner update, 2026-10-30: DESIGN's locked
[PEN and NeuroLink decision](../DESIGN.md#pen-and-neurolink-decision-owner-2026-10-30)
now governs capability packages, external conversation/event interoperability,
Octos as the channel-package donor loaded through NeuroLink, and Minecraft as
the first game. Relevant scope rows below reflect that update;
other target statuses remain the dated inventory, not a fresh implementation
audit. Consult HANDOFF for subsequent workbench delivery evidence.

Owner update, 2026-10-31: the official inbox plugin and associated hook/durable-
agent handoff are deferred beyond the immediate stage; see DESIGN's
[inbox decision](../DESIGN.md#inbox-and-event-handoff-decision-owner-2026-10-31).
Octos reuse is now limited to official QQ, with NapCat QQ separately selected
for active browsing using its existing data interfaces. These are scope and
schedule decisions, not implementation evidence.

Owner update, 2026-10-31: DESIGN's
[Laputa protocol decision](../DESIGN.md#laputa-protocol-and-garden-governance-decision-owner-2026-10-31)
repositions Laputa as a persona/memory-governance specification with host-native
implementations. Recap belongs to the agent/harness; full MemPalace remains the
current long-term backend, with future replacement possible. Garden adapts its
closed-loop collection/update workflow to native host authority files and
evolution rather than establishing parallel stores. New adapter implementations
and a unified management console are not delivered by this inventory update.

## How to read the matrix

The **scope** column describes decisions actually made for the new Vivy:

- **Keep**: the owner has selected this capability or its explicitly stated
  functional direction. Exact parity, sequencing and UI are not implied.
- **Decide**: this capability has not been explicitly retained or dropped. An
  earlier suggestion to defer a product is not treated as an owner decision.
- **Exclude**: the owner has explicitly omitted this implementation or
  compatibility target from the current scope. A later phase may make a new
  choice; the user-visible problem can still have a replacement.

The **target** column describes source/recipe state in vivy-test, not a promise
of behavior parity: **Yes** = implemented and selected for the stated narrow
behavior; **Partial** = a meaningful piece exists; **No** = no selected
implementation found; **Reference** = upstream source exists but is not selected.
An acceptance-scope **Keep** plus target **No** is planned, not delivered.
Capabilities involving real providers, different platforms, durable recovery,
external services, or a UI need their own applicable evidence. See
`docs/PI-BASELINE.md` and `docs/HANDOFF.md` for the actual recorded tests.

The **Go evidence** column indicates where an observable implementation,
proposal or source example was found. A source path, RPC name, checked issue or
feature-branch commit is not by itself proof of installed-product acceptance.
Go main, stable and feature branches have different contents; do not combine
their status into an imaginary single release.

## Source and evidence map

| Key | Inspected evidence |
| --- | --- |
| G1 | Go [main](https://github.com/ProjectViVy/agent-vivy/tree/9cd2801f8090723a652d3e835f72d83715b82121): `internal/runtime`, `internal/tools`, `internal/storage`, `internal/app`, `internal/rpc/control.go`, `go.mod`, `recipes/`, `plugins/`, UI routes, TODO/COMPLETE/DEFER. Git objects only. |
| G2 | The 16 [remake donor lanes](https://github.com/ProjectViVy/agent-vivy/blob/0059b73138b3830fd3bfc1c041b9bc8353e93a33/docs/superpowers/plans/remake-20261019/index.md), integration `0059b731`, archive `495caf6e` and shared base `202aac38`. Donor features are not automatically accepted on the remake target. |
| G3 | [A2A branch](https://github.com/ProjectViVy/agent-vivy/tree/412fdff9c184ada428554dcfaad618b214d5c835), `plugins/a2a-server`, `recipes/a2a.vivy.yml`, TaskHost and [A2A-06 verification](https://github.com/ProjectViVy/agent-vivy/blob/412fdff9c184ada428554dcfaad618b214d5c835/docs/logs/2026-10-07-a2a-A2A-06/verification.md). |
| G4 | [ACP branch](https://github.com/ProjectViVy/agent-vivy/tree/21c5b8bb49edd1502a033b0ca557737e102b9551) and main's `plugins/acp`; [remote SSH planning branch](https://github.com/ProjectViVy/agent-vivy/blob/0cf252a1c883b829714577d916a6ce944a670f56/docs/superpowers/plans/code-ssh/index.md). |
| G5 | GitHub issues, including [#2 A2A](https://github.com/ProjectViVy/agent-vivy/issues/2), [#12 Plan/Goal](https://github.com/ProjectViVy/agent-vivy/issues/12), [#13 hooks](https://github.com/ProjectViVy/agent-vivy/issues/13), [#57 assistant/worker](https://github.com/ProjectViVy/agent-vivy/issues/57), [#58 unified memory](https://github.com/ProjectViVy/agent-vivy/issues/58), [#60 offload](https://github.com/ProjectViVy/agent-vivy/issues/60), [#62 context inspection](https://github.com/ProjectViVy/agent-vivy/issues/62), [#63 Obsidian](https://github.com/ProjectViVy/agent-vivy/issues/63), [#64 skill evolution](https://github.com/ProjectViVy/agent-vivy/issues/64), [#66 companion entry](https://github.com/ProjectViVy/agent-vivy/issues/66), [#72 evolution](https://github.com/ProjectViVy/agent-vivy/issues/72), and [#93 runtime experiments](https://github.com/ProjectViVy/agent-vivy/issues/93). Open/closed issue status alone is not proof of a shipped product. |
| G6 | Remote `stable` `0e795b0c600979f606ee90f45600fb01d2981a45`: read-only GitHub tree and 34 commit summaries since locally recorded `83029dc0`, plus the current source inventory under `internal`, `plugins` and `ui`. Those commits include child context/workspace, retry, failed tool continuation and steering corrections. It was not checked out or fully built. |
| N1 | vivy-test `recipes/coding-agent.yaml`, `plugins/vivy-basics`, `plugins/vivy-gateway`, `plugins/vivy-python`, launcher and README. |
| N2 | vivy-test DESIGN's 2026-10-27 two-harness and memory decisions; HANDOFF; PI-BASELINE. |
| N3 | Pinned Rig `55cd481a`: `corpus/base/src/session_goal.rs`, `corpus/sn/src/hook_scripts.rs`, `corpus/fh/src/lsp_tools.rs`, `corpus/base/src/checkpoint_rewind.rs`, `rig-basics` remote/checkpoint, harness and TUI. Corpus examples are reference source until selected and tested. |
| N4 | Pinned Pi `f321e5bd`: coding-agent and `packages/durable` as behavior references. |
| N5 | Prime `c4c238ca`: Python REPL protocol/event loop and MCP lifecycle reference; mysoul source and its pinned MemPalace `f41e8e75` as documented in DESIGN. No Prime/MemPalace/mysoul end-to-end integration is asserted. |

GitHub remote heads for main, remake, A2A, ACP and code-ssh were compared on
2026-10-27. Local `origin/stable` was stale; its newer remote tree and commit
summaries were inspected read-only through GitHub. The complete Studio submodule,
all external Laputa/INOFY sources, every historic pull request and private user
data were not audited. This is feature-family coverage across the named sources,
not a guarantee that every historical microfeature has been discovered.

The A2A branch's plan index contains stale planning prose alongside completed
story rows. Its source and A2A-06 logs show an opt-in server implementation;
the final [issue #2 comment](https://github.com/ProjectViVy/agent-vivy/issues/2#issuecomment-6036292999)
records owner acceptance on that branch and keeps merge as a separate gate. The
branch is not an ancestor of inspected `origin/main`, and no `a2a-server`
module appears in the inspected remote stable plugin tree. The branch evidence
does not establish a selected server in either Go main/stable or vivy-test.

## 1. Execution, tasks and composition

| Feature / user-visible behavior | Go evidence | Scope | Target | Boundary / gap |
| --- | --- | --- | --- | --- |
| One YAML-selected, statically assembled executable | G1 `sdk/internal/assembly`, G2 | Keep | Yes | vivy-test recipe selects compiled Rust plugins; it does not implement Go Generation compatibility. |
| Interactive coding agent: model/tool/model loop | G1 `runtime/engine.go` | Keep | Yes | Rig coding path exists; see PI-BASELINE for tested scenarios and remaining parity. |
| Token/reasoning streaming and final result | G1 engine/mapper, G6 | Keep | Partial | Basic scripted streaming exists; exact gateway/reasoning/event behavior requires selected-endpoint verification. |
| Tool-call error returned to model, continue or report | G1 `tool_failure.go`, G6 | Keep | Partial | Offline failure scenarios exist; long-running and provider-specific failure paths remain bounded checks. |
| Cancel turn and owned shell descendants | G1 service/shutdown, G6 | Keep | Yes | Current Windows shell checks exist; this does not establish durable-task abort. |
| In-flight steer, queued follow-up, send-now | G1 queue/steer RPC, G6 | Keep | Partial | Current TUI documents Enter steer and Tab queue; exact Go send-now, approval and restart semantics have not been established. |
| Chat session persistence, list and resume | G1 sessions | Keep | Yes | Rig session mechanism, not old Journal or opaque Eino checkpoint compatibility. |
| Rewind, fork, clone, branch summary | G1 session tree | Keep | Yes | Present in coding path; representations and exact Go/Pi behavior differ. |
| Session delete, rename, search and cross-session history | G1 session/history RPC | Decide | Partial | List/resume/tree exist; the whole Go management and content-search surface is not established. |
| Import/export/copy of conversation | G1 session/export | Keep | Partial | HTML/own JSONL and clipboard are present; old Go/Pi formats and UI sharing are not full parity. |
| Automatic and manual context compaction | G1 compaction | Keep | Partial | Rig compaction/branch summary exist; exact overflow/manual and durable rules need evidence. |
| Per-run usage, tokens, pricing | G1 stats, G6 | Keep | Partial | Configured model facts and TUI usage exist; gateway completeness and historical accounting remain unverified. |
| Durable admission, independent task identity | G1 run admission/queue | Keep | No | Separate Rust durable harness is planned; existing coding sessions do not establish it. |
| Durable checkpoint, restart and interrupted effects | G1 checkpoint/recovery | Keep | No | Decide replay eligibility; do not directly resume Eino's version-bound opaque checkpoint in Rig. |
| Durable sleep, external wake, work abort | G1 queue/recovery; N4 | Keep | No | A host or OS trigger must be available; cancelling an observer is not aborting the task. |
| Timeout, budgets, bounded results and loop guard | G1 budget/service | Keep | Partial | Existing component bounds apply; a durable multi-run/goal budget is not delivered. |
| Native subagents, roles, follow-up, message/wait | G1 child lifecycle, G6 | Keep | Yes | Rig-backed in-process subagents and roles exist; not the Go remote/multi-process contract. |
| Parallel child tasks and sequential chain | G1 orchestration; G2 tool-parallelism | Keep | Yes | Bounded Rig wrappers exist; they share a workspace and do not promise arbitrary DAG execution. |
| Child recovery, restart, parent/child context isolation | G1 child recovery; G6 | Keep | Partial | Completed child sessions persist; interrupted work is not replayed. Full durable semantics remain. |
| Agent-authored bounded DAG and dependency execution | G1 workflow/orchestration; G5 #8/#39 | Decide | No | Parallel/chain are not a general DAG; historical Go source status varies by candidate. |
| Reusable workflow editor, versioned publication and run inspection | G1 INOFY/workflow RPC; G5 #9/#21 | Decide | No | If selected, implement as a Python component; reuse of INOFY itself is unverified. Bounded subagent chains are a separate feature. |
| Arbitrary recursive agent hierarchy / Prime RLM | G1 orchestration, N5 | Decide | Partial | Rig recursion is configurable; full recursive-RLM/factory is not a first-loop prerequisite. |
| Collaborative Plan entry, submit, user review | G1 Plan; G5 #12 | Keep | Yes | `PlanPlugin` is selected; guidance does not change underlying tool permissions. |
| Goal create/edit/pause/resume/complete/block | G1 Goal RPC; G5 #12 | Keep | Reference | Rig's saved session-goal example is not selected; autonomous lifecycle is not wired. |
| Bounded automatic Goal progression across rounds | G1 `goal_driver.go` | Keep | No | Requires clear stop, blocker, cancellation and recovery policy; reuse selected task authority. |
| Persistent todo/task checklist | G1 `tools/todo.go`, todo RPC | Decide | Partial | Plan and task delegation are not equivalent to the Go CRUD checklist. |
| Cron create/edit/list/trigger/stop and recurring/one-shot schedules | G1 `cron_scheduler.go`, RPC, UI | Decide | No | Redesign rather than port the Go scheduler; scope and missed-run/overlap policy are open. Durable sleep/wake above is retained. |
| Inbound event/webhook triggers a task | G5 #13 | Decide | No | Redesign if selected. Lifecycle hooks, webhook ingress and scheduler remain distinct surfaces. |
| Background process/job output and kill | G1 `tools/jobs.go`, `background/*` | Decide | Partial | Redesign rather than port Go's architecture. Shell cleanup exists; durable jobs/attach are not established. |

## 2. Context, persona and memory

| Feature / user-visible behavior | Go evidence | Scope | Target | Boundary / gap |
| --- | --- | --- | --- | --- |
| Layered context / working-context assembly | G1 `context.go`, G5 #58 | Keep | Partial | Project instructions and coding compaction exist; planned durable layering is not implemented. |
| Workspace instruction files and system prompt additions | G1 `agentsmd.go`; N1 context | Keep | Yes | Current discovery covers AGENTS/CLAUDE and prompt overrides with trust; older Go choices differ. |
| Laputa persona/identity and context authority | G1 persona/cognitive, G2 persona | Keep | No | Specification/protocol with a preferred minimal native Rust persona implementation; update API, representation and injection boundary remain open. No mandatory Garden runtime or duplicate persona store. |
| ACTMEM / cross-session resident Recap for companion | G1 `recap_*`, G2 recap, G5 #58 | Keep | No | Agent/harness-owned working context; Laputa does not require its own Recap engine. Durable path; coding does not automatically inherit mysoul's recap-only policy. |
| Recap revision, source association and commit before unload | G1 `recap_compaction.go`, G5 #58 | Keep | No | Preserve the behavior/invariant; do not assume the old storage schema must be copied. |
| View, correct, edit and targeted forget of Recap | G1 memory actions/UI, G5 #58 | Keep | No | Memory is user-manageable in the retained direction; exact interface is undecided. |
| Full upstream MemPalace engine in Python | N5; owner decision | Keep | No | Dated inventory status; consult HANDOFF for later native delivery. Remains the current backend; future replaceability does not select a Mem0 adapter or migration. Do not inject L0 identity. |
| Shared long-term memory by palace/wing/room/drawer | N5; owner decision | Keep | No | Domain partitions span both harnesses; filtering is not an access-control system. |
| Recall on demand with bounded source-backed results | G5 #60; N5 mysoul | Keep | No | Scope and source identity must survive direct library integration. |
| Memory ingestion, automatic archive and consolidation policies | G1 cognitive/memory; G5 #58/#60 | Decide | No | Exact triggers, dedup, updates and failure handling need an actual end-to-end selection. Recap commit ordering above remains retained. |
| Memory delete, correction, indexing and provenance | G1 memory management | Keep | No | Preserve upstream engine behavior and distinguish a Recap edit from deletion in long-term memory. |
| MemPalace optional graph/diary/mesh/etc. product interfaces | N5 | Decide | No | Full package is intended; exposing every optional upstream interface is a separate choice. |
| Original BML store / provider | G1 `bml/`, G2 bml | Exclude | No | Retired in favor of MemPalace; old BML data migration is excluded. |
| Persona masks and session role selection | G1 `plugins/vivy-masks-ui`, G2 persona | Decide | Partial | Rig roles exist for subagents; they are not equivalent to Go persona-subordinate session Masks. |
| Context inspection / attribution dashboard | G5 #62 | Decide | No | An optional proposal; distinguish estimated tokens, actual model input and admitted memory. |
| MemPalace + Obsidian capability loop in Python workbench | G5 #63; owner clarification | Keep | No | Direction selected; read/write, sync, authority, UX and automation await detailed design. |
| Obsidian/Vault import, subscriptions and sync | G5 #63 | Decide | No | The new loop is selected, but its exact source ingestion and watcher semantics are not. |
| Old Notebook: durable notes, editor and source linking | G1 notebook modules, G2 notebook | Exclude | No | Do not migrate the old product; the new MemPalace/Obsidian direction is separate. |
| Old scheduled daily/weekly/monthly Reports product | G1 reports, G2 reports, G5 #5 | Exclude | No | Do not migrate the old product; future Python capabilities are not thereby specified. |
| Old AutoDream candidate reflection | G5 #6 | Exclude | No | Do not migrate it. The new Vivy AutoDream workspace/candidate/automatic-adoption decision is retained; reusable Garden governance workflows adapt to each host's native authority. |
| Old Skill evolution proposals, review and revision | G2 skill-evolution; G5 #64/#72 | Exclude | No | Do not migrate the old evolution path; ordinary Skill discovery/use remains selected. |
| Persona evolution / EvoMap / strategy composition | G1 cognitive, G5 #7/#72 | Exclude | No | Do not migrate the old evolution stack into Vivy; an independent new chain is a later design. |

## 3. Tools, Python, MCP and packages

| Feature / user-visible behavior | Go evidence | Scope | Target | Boundary / gap |
| --- | --- | --- | --- | --- |
| File read/write/edit, list and local search | G1 `internal/tools`; N1 | Keep | Yes | Coding recipe selects Rig tools plus search/list. Exact tool schemas and full limits differ. |
| Shell / terminal command execution | G1 bash/execute/jobs; N1 | Keep | Yes | Git Bash and process cleanup exist on Windows; this is not an OS sandbox. |
| Multiple edits, patch and shell output spill | G1 `multiedit`, patch, spill | Decide | Partial | Current editing exists; Go-specific multi-edit and recoverable spill behavior not fully checked. |
| LSP diagnostics, definition, references, hover and symbols | G1 `plugins/coding/lsp`, VC-3; N3 Rig corpus | Keep | Reference | Optional/non-core; Rig's example is not selected or Windows-validated in vivy-test. |
| LSP rename | G1 `plugins/coding/lsp`, VC-3 | Decide | No | Rig's inspected five-tool example does not include rename; choose separately if needed. |
| Task notes, structured scratch and sequential-thinking tool | G1 `notes.go`, `sequential_thinking.go` | Decide | No | Workspace files/Plan are available; these specific Go tools are not selected. |
| Ask-user questions and structured choices | G1 `askuser.go`, question RPC | Decide | Partial | Rig terminal questions and Plan review exist; old structured question/recovery semantics are not established. |
| Image/file attachments as model input | G1 attachments/read_image; G6 | Decide | Partial | Image input was exercised in scripted Pi comparison; full terminal/channel UX varies. |
| Network search, web page fetch and download | G1 `network_search`, `web_fetch`, `download` | Decide | No | Python capability is a possible delivery path; none is implied by installing the worker. |
| Arbitrary HTTP/API capability and integrations | G1 `http_request` | Keep | Partial | Python code can call libraries when installed; managed discovery/configuration is not complete. |
| Tool catalog / deferred discovery / activation | G1 toolselection/toolmount/tool_search | Keep | Partial | Direct tools and trusted skills exist; deferred Python-capability discovery is planned. |
| Persistent Python workspace and top-level await | N1 `vivy-python` | Keep | Partial | Namespace persists during a worker lifetime; async background tasks are cancelled per cell. |
| Python capability instructions, environment and dependency setup | N5 Prime | Keep | No | The package manager/environment choice and first real capability are not implemented. |
| Python↔Rust host callbacks and nonblocking cancellation | N5 Prime protocol | Keep | No | Prime synthetic protocol probe is not evidence of an integrated host. |
| Python result bounds, errors and worker cleanup | N1 worker and Rust wrapper | Keep | Partial | Basic limits/error handling exist; long-lived resource and durable recovery remain. |
| MCP stdio/HTTP connection and managed lifecycle | G1 MCP backend; N5 Prime | Keep | No | Python MCP client is planned; server crash/reconnect/reload need explicit handling. |
| MCP tool discovery, schema and invocation | G1 `mcpadapter`, MCPHost | Keep | No | Actual native tool discovery/call has not been integrated in vivy-test. |
| MCP resources, templates, prompts, elicitation and OAuth | G1 MCP resource RPC; G5 #13 | Decide | No | Do not infer all optional MCP features from a basic tool call. |
| PEN external capability packages, including tools, Skills and connectors | G1 bundles; G2 toolkits; G5 #20/#67; owner, 2026-10-30 | Keep | No | Import delivers a complete capability; MCP is one possible interface. Exact manifest/runners and import behavior remain open; this does not select every legacy toolkit entry. |
| Minecraft PEN: independent multiplayer first, client control retained | Owner, 2026-10-30 | Keep | No | Independent-player multiplayer is primary. Client operation remains a second design route; edition/version, connector and auxiliary-mod policy are undecided. |
| PEN package updates, rollback, revocation and catalog UI | G1 bundles/Toolkit UI | Decide | No | Package import/preparation is selected; update/removal mechanisms and a hub/UI remain implementation choices. |
| Skill discovery and project trust | G1 skillhost; N1 | Keep | Yes | Rig skills/trust are selected; no claim of every legacy marketplace feature. |
| Skill marketplace / package download and updates | G1 marketplace RPC | Decide | No | Installing a bundle, discovering a skill and an online marketplace are distinct. |
| Prompt templates and slash commands | N1 templates | Keep | Yes | Current coding path provides Pi-style Markdown templates. |
| Lifecycle hooks / external script hooks | G1 `hooks*.go`; G5 #13 | Keep | Reference | Rig corpus supplies a candidate, not a selected plugin. Need actual events and process handling. |
| Official persistent inbox and notification hooks | Owner, 2026-10-31 | Keep | No | Later phase, outside the immediate stage. Agent-controlled reminders; pending messages survive restart. Normal handoff follows one tool completion; explicit human interruption remains immediate. Swarm reuse is a possible consumer. |
| Capability-level hooks inside Python execution | N5 host callback | Decide | No | An outer `python_repl` hook cannot see each internal library operation. |
| Hook format compatibility and verified webhook dispatch | G5 #13 | Decide | No | Old issue delivered research; actual format/transport choice remains open. |
| Tool execution approval / editable review | G1 policy/approval/review | Decide | Partial | Plan review and project trust exist; full effectful-tool HITL parity is not established. |
| Workspace path rules, shell policies and secret filtering | G1 policy/sandbox; N1 | Keep | Partial | Credential filtering and project trust exist; Go policy parity/OS isolation is not claimed. |
| OS-level sandbox and automatic worktrees | G1 deferred SBX-OS; N1 | Decide | No | Existing subagents share the host process/workspace. |
| Tool-call concurrency ordering and unsafe side-effect handling | G2 tool-parallelism; G6 | Keep | Partial | Bounded child parallelism exists; arbitrary concurrent tool effects/replay need a defined policy. |

## 4. Interfaces, protocols and interoperability

| Feature / user-visible behavior | Go evidence | Scope | Target | Boundary / gap |
| --- | --- | --- | --- | --- |
| A2A inbound server, Agent Card and authentication | G3, G5 #2 | Decide | No | Distinct external-agent protocol; not provided by local subagents or MCP. |
| A2A task submission, status, streaming, cancel and continuation | G3 handler/TaskHost | Decide | No | Go branch has a text-first JSON-RPC/SSE subset and maps external work to native Runs. |
| A2A outbound client / delegation to remote agents | G3 design exclusions | Decide | No | The Go A2A branch is inbound server; do not infer an outbound client. |
| A2A exact event replay, push, rich attachments and remote approvals | G3 branch | Decide | No | Branch uses snapshot convergence; exact wire replay is unselected, push unsupported, remote approvals excluded. |
| ACP IDE/editor protocol over stdio | G1 `plugins/acp`, G4, G2 acp, G5 #1; N5 Prime | Decide | No | Do not continue Go's architecture. Prime's ACP code is a source reference; no ACP implementation was found in the pinned Rig source search. |
| ACP client-side execution, custom directories or MCP forwarding | G4 `plugins/acp/agent.go`; N5 Prime | Decide | No | The old pilot rejects some of these; select actual behavior separately. |
| SSH remote coding workspace / remote tool execution | G4 code-ssh, G5 #15; N3 Rig remote | Decide | Reference | Do not continue Go's architecture. Rig's RemoteHostsPlugin is available as an unselected reference. |
| Long-lived RPC control / embedding protocol | G1 JSON-RPC, codeclient | Decide | Partial | Print/JSON exists; it is not Go control RPC, Pi RPC or ACP parity. |
| Browser/Web GUI and control plane | G1 `ui/`, routes, RPC | Decide | No | Existing terminal is an entry; old web product is not automatically available. |
| Native desktop/Wails product | G5 #31; Go feature records | Decide | No | Original product/release work is separate; no assumed reuse of its app shell. |
| TUI interactive coding and task progress | G1 `sdk/tui`; N1 | Keep | Yes | Current compact Rig TUI exists; detailed old UI/theme/keybinding parity is partial. |
| Print/JSON/headless modes | G1 face; N1 | Keep | Yes | Existing Rig JSON events are not full legacy wire compatibility. |
| Channel packages ported from Octos and loaded through NeuroLink | Owner final ruling, 2026-10-30; refined 2026-10-31; pinned Octos `7486f3df` | Keep | No | Only official QQ is selected. Exact port/loading contract remains open; Octos runtime/OUP adoption is not selected. |
| Official QQ Bot channel | Owner, 2026-10-31; Octos `qq_bot_channel.rs` | Keep | No | Port the official QQ implementation from Octos and load through NeuroLink; other Octos channels are unselected. |
| NapCat QQ active browsing / agent phone | Owner, 2026-10-31 | Keep | No | New-message viewing, selected-group filtering/history and owner identity. Reuse NapCat/NTQQ data; MCP and coverage details need verification. Future inbox integration is later-phase work. |
| Telegram, DingTalk, Feishu, Discord channels | G1 `plugins/*`, channel host | Exclude | No | Outside the earlier first-phase delivery scope; the broader NeuroLink positioning does not itself select these platform implementations. |
| Channel image input/output and media handling | G1 COMPLETE CH-MEDIA | Decide | No | Old image/media plumbing does not establish voice/video support. |
| Channel approvals, notifications and interactive cards | G1 channel/HITL notes | Decide | No | Text notifications and platform-native approval cards have different availability. |
| NeuroLink external conversation and event protocol/standard | Owner, 2026-10-30 | Keep | No | Separate from PEN capability packaging; also the loading entry for adapted Octos channel packages. Wire format and A2A/ACP/AG-UI interoperability remain open. Actual lifecycles belong to the responsible host/module. |
| WeCom, Weixin and public webhooks | G1 DEFER/COMPLETE; G5 #2/#13 | Decide | No | Historical proposals/deferrals, not established current product capabilities. |
| Voice, video, embodied device and robotics integration | G1 media limits; prior owner direction | Decide | No | NeuroLink's intended range includes external embodiments; specific device and live-media implementations remain unselected. |
| Image generation/editing | G5 #3 | Decide | No | Old owner assigned image generation to external PENS; consumption by new Vivy is not selected here. |

## 5. Model providers and runtime configuration

| Feature / user-visible behavior | Go evidence | Scope | Target | Boundary / gap |
| --- | --- | --- | --- | --- |
| Configurable OpenAI-compatible endpoint | G1 provider; N1 | Keep | Yes | OpenAI Chat Completions adapter and configurable endpoint; real-gateway verification is separate. |
| Configurable Anthropic-compatible endpoint | G1 provider; N1 | Keep | Yes | Anthropic Messages adapter is selected; actual endpoint compatibility requires a real check. |
| Provider/model creation, selection and saved defaults | G1 provider registry; N1 | Keep | Yes | Current TUI editor and manually configured models; exact Go settings migration is separate. |
| Model windows, reasoning, pricing and usage settings | G1 provider/model; N1 | Keep | Partial | Facts can be configured; actual provider output and pricing completeness vary. |
| Credential env references and local key management | G1 settings; N1 | Keep | Yes | Existing provider credential sources/filtering; do not copy old private settings or tokens. |
| Provider retries, failover and transient errors | G1 provider/runtime; G6 | Keep | Partial | Some retry behavior is inherited; full cross-provider and child parity not established. |
| Auto model discovery and local-model process management | G1 `plugins/infra/llm` | Decide | No | Presets/manual models do not imply discovery, pull or process start/stop. |
| Provider catalog, routing aggregation and virtual models | G1 provider registry/DEFER | Decide | Partial | Existing presets are not an independent gateway/router; user originally chose gateway first. |
| Subscription OAuth / refresh and provider-specific native protocols | G1 DEFER; N1 | Exclude | No | Current decision selects OpenAI-compatible and Anthropic-compatible protocols, without subscription OAuth. |
| Provider cache warming / prefix accounting | G1 `cachewarmer`; G2 warming | Decide | No | Historical opt-in work and #93 are not proof of benefit on a new gateway. |
| Provider fallback / utility models for summaries or titles | G1 provider/compaction/titler | Decide | Partial | Model selection exists; separate utility-model policy and auto-title are not established. |

## 6. Product surfaces, state and distribution

| Feature / user-visible behavior | Go evidence | Scope | Target | Boundary / gap |
| --- | --- | --- | --- | --- |
| Persistent companion entry, avatar and ongoing subject | G5 #57/#66 | Keep | No | Durable companionship is selected; exact browser/channel/avatar interface remains undecided. |
| Shared workspace and per-task identity | G1 workspace; G6 child-sharing | Keep | Partial | Common filesystem is present; durable namespaces/checkpoint ownership and conflicting writes need handling. |
| Workspace file browser, attachments and deliverables/downloads | G1 workspace/deliverables RPC | Decide | Partial | Coding tools operate on files; a human-facing deliverables product is separate. |
| Conversation question, approval and review center | G1 question/review RPC and UI | Decide | Partial | Plan review and native prompts exist; full cross-session durable review is not established. |
| Session trajectory, tool cards, diff and run event history | G1 trajectory/RPC/TUI | Keep | Partial | Current terminal shows tool results/diffs; durable historical inspection parity is open. |
| State/config management and preferences | G1 settings UI/RPC; N1 | Keep | Partial | Provider settings exist; no complete old Settings/diagnostics interface. |
| Localization / Chinese and English UI | G1 i18n, UI catalogs | Decide | No | Full bilingual product surfaces are not established by the English terminal. |
| File version recording and restore | G1 fileversion/rewind; DEFER restore | Decide | No | Rewind conversation does not prove filesystem rollback. |
| User-facing usage/cost dashboard | G1 `stats/tokens`, UI | Decide | Partial | TUI usage exists; historical dashboard/ledger is not a delivered equivalent. |
| Logging, error diagnostics and troubleshooting bundle | G1 logging/diagnostics | Keep | Partial | Errors and some terminal diagnostics exist; the old diagnostics UI/export is not selected. |
| Reliable download / explicit result delivery | G1 `deliverable_*` | Decide | No | Files on disk and durable, tracked, recoverable delivery are different capabilities. |
| SQLite/PostgreSQL product stores and migrations | G1 `internal/storage` | Decide | Partial | Rig session persistence exists; durable store choice is open, PostgreSQL is not a requirement by default. |
| Import old sessions, memories, configs or checkpoints | G1 storage/checkpoint | Exclude | No | The owner explicitly excludes legacy-data compatibility; no user data was accessed or copied. |
| Windows launcher and executable packaging | G1/N1 | Keep | Yes | Native checkout launcher exists; installer/update and production platform coverage are separate. |
| Legacy multi-platform deployment compatibility | G1 Docker/CI, N4 | Exclude | Partial | Not a compatibility target. Earlier Linux evidence remains historical; a future platform requires its own selection and checks. |
| Docker / hosted service / multiple clients | G1 Docker, HTTP | Decide | No | A persistent host can be local or external; product deployment is a separate choice. |
| Self-update, installation and release UI | G1 SDK/Studio | Decide | No | The experiment builds explicitly; old Studio distribution lifecycle is excluded below. |
| Studio IDE, release/eval/promote/rollback platform | G1 Studio/submodule | Exclude | No | Explicit experiment exclusion; individual user-facing capabilities can be reconsidered independently. |
| Public SDK, universal Port catalog, generation hashes and conformance platform | G1 `sdk/` | Exclude | No | YAML build-time assembly remains; the full legacy distribution/governance platform is excluded. |
| Pi TypeScript extension runtime / source compatibility | G1 DEFER; N2 | Exclude | No | Native Rust capabilities and Python content can still provide selected equivalent behaviors. |

## 7. Remake donor coverage

Every named lane is represented above. This is a cross-index, not an instruction
to retain each whole lane:

| Donor | Feature rows above |
| --- | --- |
| core-fixes | loop/failure/cancellation/recovery, task identity and effects |
| provider | model protocols, settings, model facts, retry and credentials |
| notebook | notebook storage/editor/source links |
| reports | manual/scheduled reports and result delivery |
| toolkits | content bundle, hooks, skills, MCP, lifecycle/UI |
| bml | explicitly retired BML and MemPalace replacement |
| recap | ACTMEM, context admission, revision, source, correction and forgetting |
| cognition | persona, durable context, reflection, Goal/worker boundaries |
| skill-evolution | Skill proposal, review and revision |
| persona | Laputa/persona authority, masks and companion entry |
| workflow | native delegation, bounded DAG, reusable published workflows |
| acp | ACP server/Face and its distinct pilot limitations |
| tool-parallelism | tool concurrency, ordering and side effects |
| chat-ui | companion/TUI/Web entry, review and status |
| misc | cross-cutting fixes; A2A/SSH are also checked from their own branches |
| warming | cache warming and runtime experiments |

The separate **reusable workflow product** includes editor, definitions,
publication, versioned invocation, node status and cancellation (G1/G5 #9/#21).
Its scope is **Decide**, target **No**. If selected, it is a Python component;
the ability to reuse the existing INOFY implementation has not been checked.
The current `task_parallel` and `task_chain` do not implement that product.

## 8. Prime and Rig references for history, context and file rewind

Inspected Prime `c4c238ca025e382d96fb5f37e6328ca77fb82ef3` and Rig
`55cd481a2421fd5da468b5bd6afc6ac876883599`. These are source observations,
not tests run against the user's data or proof of integration in vivy-test.

### History search

Prime's `pa-tui/src/agents_view_search.rs` and `agents_view_state.rs` build a
picker over the displayed session title, durable session ID and working
directory. They merge live and saved session records; a title may come from the
first prompt, but the searched title is clipped to the displayed column. Queries
support fuzzy matching, quoted phrases and bounded `re:` regex. The implementation
explicitly excludes full transcript text and file paths from its search corpus.
The daemon supports JSONL session storage, active-branch windows, and paged
transcript display (`pa-core/src/session/window.rs`,
`pa-daemon/src/session_store/view.rs`). The inspected picker is not a cross-session
message full-text or semantic search. No separate full-history search was found
in the searched source; do not infer that none could exist in another surface.

vivy-test already selects Rig's session picker and resume/tree commands. A
separately selected search over complete messages would need its own source,
scope and index/query behavior; full MemPalace retrieval is not automatically
the authoritative searchable conversation transcript. No search implementation
or storage decision was made in this investigation.

### Context inspection

Prime's `/context` uses `get_context_tree` in
`pa-daemon/src/state_getters.rs`: root and child agents report model, status,
estimated context/window, own and accumulated usage, costs and optional model
breakdown. Persisted child-session artifacts supplement the live roster; a
single-flight cache refreshes the disk walk. `/context all` displays the full
agent tree; `/system-prompt` displays the prompt separately.

`pa-daemon/src/session_stats.rs` estimates current context from the last valid
assistant usage plus later message estimates; it reports unknown when post-
compaction usage is not yet reliable. These are useful usage views, not a full
categorized inspection of every actual request block, injected memory source,
tool schema and selection reason. Such attribution would need its own measured
capturing/projection seam if selected for the layered Vivy context. Existing
vivy-test usage/footer is a partial basis; neither an exact per-layer inspector
nor a new context store has been implemented.

### File rewind

Prime's `/tree` and `/fork` navigate/copy **conversation branches**; they do not
by themselves restore workspace files. Its `workspace_snapshot` captures a Git
worktree delta and optionally its HEAD-tree baseline into manifest/content blobs
for a future cloud-session artifact, with explicit exclusions, verification and
bounded sizes. At the inspected revision this is staging/verification plumbing,
not a user-facing file rollback; capture explicitly refuses non-Unix platforms.
Python-kernel snapshots, refinement-history rollback and binary update rollback
also have different purposes. Source search found no selected per-turn workspace
rollback command in the inspected Prime paths; that is not a claim about every
possible external extension.

Rig has a stronger direct reference: `corpus/base/src/checkpoint_rewind.rs`.
Before each root turn it snapshots the Git working tree, including nonignored
untracked files, under `refs/rig/checkpoints/<session>/<turn>` using a temporary
index. `/rewind` can restore files and conversation together or separately;
it asks before overwriting detected user edits. Its restore removes files absent
from the target tree and checks out changed files. It needs a Git worktree and
explicitly does not capture ignored files. Current vivy-test selects Rig's
conversation rewind but **not** this file-rewind corpus plugin. It is a source
candidate, not proof of Windows support or safe coordination with another
durable writer. `rig-basics::checkpoint::CheckpointPlugin` is a different
per-call change-observation facility and is also not selected by the recipe.

### LSP, SSH and ACP references

Rig's `corpus/fh/src/lsp_tools.rs` offers diagnostics, definition, references,
hover and workspace symbols, starts configured servers on demand, and manages
server lifecycle. It is in the non-published `corpus-fh` example crate, not a
selected vivy-test plugin; that crate carries additional dependencies. Its
source test uses a fake server, and this investigation did not run it or verify
native Windows behavior. No rename tool is present in that five-tool example.

Rig's `rig-basics/src/coding_tools/remote.rs` contains `RemoteHostsPlugin`:
configured `HOST:PATH` operations use transports such as SSH. It is not
selected in the current recipe, which also has a Vivy shell wrapper, so tool
integration and failure/cancellation semantics need checking before use.
Search of the pinned Rig source did not find an ACP implementation. Prime does
contain an ACP stdio/daemon implementation under `pa-daemon/src/acp`; it is a
source reference, not an already reusable small Rig plugin. Neither ACP nor SSH
was implemented by this research.

## 9. Remaining decision gaps

These questions are recorded for later owner choice; they are not prerequisites
to reading the inventory and do not silently change the agreed scope:

1. NeuroLink's wire format and relationship to A2A/ACP/AG-UI; inbound/outbound
   support and which harnesses accept external work. SSH is a separate choice.
2. Official QQ extraction/loading details and NapCat data/identity/API coverage;
   companion/embodiment implementations remain separate delivery choices.
3. Human task approvals, status/recovery visibility, history/search and explicit
   file delivery: which behavior is needed in the first usable product.
4. Memory management granularity: editable Recap, long-term delete/forget and
   optional full-engine MemPalace features. The Python MemPalace + Obsidian loop
   is selected as a direction; its detailed flows and sync are not yet decided.
5. PEN package manifest/runners, dependency composition, preparation, updates/
   removal and optional hub/UI; complete capability packaging is already selected.
   Minecraft still needs edition/version, connector and auxiliary-mod decisions.
6. Web search/fetch, LSP, Python-hosted reusable workflows and masks: select
   specific user cases. Old Notebook/Reports, Skill evolution and AutoDream
   are excluded; the new evolution chain is outside Vivy.
7. For new Cron/webhook/background mechanisms and optional ACP/SSH, decide the
   actual new behavior. Legacy data and multi-platform deployment compatibility
   are excluded from the current target.

No Go app or Vivy-test product was built or run for this source inventory. The
recorded A2A branch tests are historical branch evidence, not tests run here.
