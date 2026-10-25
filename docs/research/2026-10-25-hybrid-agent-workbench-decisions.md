# Research and decision snapshot: Rig, Vivy, Prime Agent, and the Python workbench

Date: 2026-10-25

Status: Research checkpoint with an accepted hybrid architecture, Prime-anchored automatic evolution and Vivy persona extension; no implementation authorized by this report.

Scope: Preserve this conversation's findings and latest direction in both repositories.

This report is intentionally saved identically under `docs/research/` in
`vivy-test` and `agent-vivy` at the owner's explicit request. It is a dated
handoff snapshot, not a second runtime configuration or a new architecture
contract. Subsequent experiment implementation belongs in `vivy-test`.
Nothing here authorizes changing the original Vivy repair work or its contracts.

## 1. Latest owner direction

The latest clarification takes precedence over the earlier proposal to move
every capability into Python:

- Keep Pi-style, frequently used coding tools directly available to the model.
- Reuse capabilities already supplied by Rig as selected plugins; wrap only
  where actual product behavior or recipe selection requires it.
- Put advanced, external, and long-tail capabilities into the Python workbench.
  MCP is an example of this deferred surface, not a reason to reproduce Prime's
  entire service catalog, OAuth UI, skill installer, or management platform.
- Carry forward Vivy's useful deferred-tool idea: discover detailed capabilities
  when needed instead of injecting all tool schemas into every model request.
- The workbench is preparation for the owner's PEN system. PEN's meaning,
  contracts, and requirements have not yet been explained. Do not invent them.
- The owner asked to preserve the research and subsequently accepted a miniature
  architecture diagram. Section 16 records the documentation/review checkpoint;
  runtime implementation requires a subsequent implementation task.
- Latest evolution decision: use Prime's concrete continual-harness formats and
  refinement lifecycle as the anchor, with automatic evolution and a focused
  Vivy persona-evolution addition. Section 15 supersedes earlier suggestions of
  a bespoke evolution format or mandatory manual review for every change.

Earlier accepted experiment constraints remain:

- Independent Rust experiment, one YAML recipe and one build-time composition
  path; gateway-first model access.
- Pi coding-agent is the functional reference, with incremental delivery.
  Equivalent Rust/Vivy extensions are wanted, not execution of existing Pi
  TypeScript extensions.
- More, Fast, Good, Frugal: reuse existing components, run useful tasks early,
  keep owned interfaces and total maintenance small.
- No species hash enforcement, standalone plugin SDK, universal Port catalog,
  provenance/conformance platform, or Studio lifecycle in the experiment.
- Do not turn the original Vivy checkout into an experiment dependency.
  Original Vivy repair is a separate effort.

## 2. Inspected baselines and evidence limits

| Project | Inspected revision or source | Interpretation |
| --- | --- | --- |
| vivy-test | Local main `2b03b44`, 2026-10-25 | Existing experiment; latest commit fixes UTF-8 terminal startup. |
| Rig | `55cd481a2421fd5da468b5bd6afc6ac876883599` | Pin used by the experiment; do not substitute historical Rig API assumptions. |
| Pi | `f321e5bdd26b3624bfb2ab182a334d0a5edb698a`, coding-agent 1.4.0 | Prior source inventory; see vivy-test's existing PI-BASELINE.md. |
| Original Vivy local main | `525d50a67152d6e9f5cc2eb469c889c6f26c21ab` | Older local checkout, not the current remote product baseline. |
| Original Vivy remote main | `9cd2801f8090723a652d3e835f72d83715b82121`, 2026-10-20 | Inspected separately through Git objects and official sources; root was not updated. |
| Prime Agent | `5c898d1b27b109bd2d383c68d60a30536e39cc22`, 2026-10-24 | Source, runtime protocol, prompts, and historical changes inspected. |

Research used source/docs/Git history inspection. It did not run a new product
test, benchmark, or model comparison. Existing experiment verification belongs
to its earlier implementation records. The owner reported that the project
passed their test; this report does not infer details of that test.

Source inspection used reference material already available in agent-vivy's
ignored workspace. Successors working on vivy-test should follow its own
reference-setup instructions, not clone or vendor the original Vivy project.

## 3. What Rig supplies, and what belongs to Vivy

At the inspected revision, distinguish the crates:

- `rig-core`: model/provider, completion/streaming, tool, conversation and
  related foundation contracts.
- `rig-agent` and `rig-memory`: agent execution, hooks, tool dispatch,
  conversation/state handling, compaction and related primitives.
- `rig-harness`, `rig-ecs`, `rig-basics`, `rig-tui`: the broader coding-agent
  and plugin environment already reused by vivy-test.
- `rig-rmcp`: an available MCP integration option. The owner's latest choice
  favors the advanced Python surface; do not add both MCP stacks by default.

Rig does not provide Vivy's exact Service.Run/Journal/Policy contract, typed
Port authority model, Grant semantics, or Generation evidence pipeline.
Its reusable coding-agent parts must not be described as full Vivy parity.

Earlier chat estimates were roughly 20-25/100 for rig-core alone, 55/100 for
direct reuse across the broad Vivy product, and 80-90/100 for the coding-terminal
slice using the full Rig ecosystem. These were subjective scope estimates,
not measured feature coverage, effort estimates, or acceptance results.
The later narrowed hybrid direction has a different denominator; do not use
these numbers to justify an implementation schedule.

Rig's upstream launcher documentation described Linux/macOS support. The
experiment's native Windows path is local adaptation with its own recorded
evidence; it must not be confused with an upstream platform guarantee.

## 4. Original Vivy and the remake donor branches

Vivy's useful composition concept is:

```text
Recipe -> selected Modules -> typed wiring -> compiled artifact
```

The original implementation also carries Port ownership/cardinality, trust,
Grants, source identity, lifecycle, conformance and Inspect obligations.
The experiment retains recipe-selected compilation without inheriting that
whole platform.

The local engine separates fixed-visible and deferred active tools and uses
Eino's `adk/middlewares/dynamictool/toolsearch`. This is evidence for the
owner's remembered deferred-tool mechanism, not a claim that the future
Python workbench already implements it.

The remake refs are not 16 accepted replacement implementations:

- 16 feature donor branches plus integration and archive refs were inspected.
- Feature areas include core fixes, providers, Notebook, reports, toolkits,
  BML, Recap, cognition, skill evolution, persona, workflow, ACP, tool
  parallelism, chat UI, miscellaneous changes, and warming.
- The integration plan says the published feature refs are raw donors and
  are not build/product-accepted. R10-R24 are blocked; R25 is a historical
  warming handoff.
- Feature branches share source base `202aac38`. The archive preserves the
  frozen history; integration is the planning/contract line.
- Code existing in the historical main tree does not prove the reorganized
  donor feature has been accepted against the repaired core.

Remote main's TODO also leaves material work open: toolkits acceptance,
unified memory/Recap integration, Eino child/DAG lifecycle gates, session
continuity, ACP integration/platform checks, and provider migration.
Do not present directory presence or donor branch counts as shipped capability.

## 5. Prime's actual evolution: correction to earlier discussion

An earlier answer treated retained bash/edit implementation files and stale
instruction text as evidence that those tools remain default model tools.
That was incorrect. Actual tool registration, the current core prompt, and
historical changes show the normal built-in model surface is `python_repl`.
SDK/custom tooling and old transcript rendering are separate concerns.

| Change | Evidence and meaning |
| --- | --- |
| 2026-05-08, PR #3, `4a7a55c9a` | Made ipython the only default-active tool; removed read/write/grep/find/ls; bash/edit remained explicitly selectable. The author's stated reason was matching the RLM-trained tool surface. |
| 2026-07-06, PR #323, `a20988132` | Removed bash/edit from the built-in tool registry. Shell work and editing continued through Python; some legacy implementations remained for custom use/replay. |
| 2026-10-01, PR #3211, `a3700282c` | Removed ineffective tool-selection flags. |
| Inspected current source | The core prompt says one tool, python_repl; session construction adds it to the configured tool set. |

Prime selected Python-first interaction early. The reviewed records do not
establish that a long-running hybrid design was benchmarked and found inferior.
Training-interface consistency is an explicit motive. Keeping large data
outside the main model context and enabling programmable composition are
documented RLM objectives. Reduced duplicate interfaces is an engineering
inference, not a published measured benefit.

"Python-only" here describes the normal model-facing action interface.
Prime's host, supervision, sessions and much of its management are Rust.

## 6. Prime's skills, MCP, and harness state are distinct

### Skills

Rust discovers Markdown skills and Python skill packages, parses metadata,
handles collisions, and advertises a compact inventory. Python skills have
SKILL.md, pyproject.toml, and an importable package. Host bootstrap uses uv
editable installs with incremental manifest checks and reports unavailable
packages. A Python skill does not need its own model tool schema.

Some core Python packages are only host-request wrappers: compact, goal,
refine, agent_message, agent_observe and rlm_heartbeat rely on Rust handlers.
Copying those Python packages alone does not transfer the capabilities.

### MCP

A generic Python MCP client offers catalog/connection discovery, live tool
search/description and calls. Adding a service is generally configuration plus
authentication, not a new per-service skill package.

Rust owns configuration, service/connection management and authentication.
Python resolves configuration through host requests, opens stdio or HTTP
connections, discovers schemas and performs calls. It reuses matching
connections and rebuilds changed ones, applies tool filters and timeouts, and
reports incomplete discovery as failure. Calls on the same connection are
serialized in the inspected implementation.

This division is evidence, not a requirement to copy Prime's entire
management surface into the experiment.

### Harness state

Harness entries store prompt notes, memories, skill callable references,
subagent descriptions and refinement history. A harness "skill" entry is not
installation of executable code. Python owns a file-backed harness-state
implementation; refinement scheduling and integration with the main run use
host facilities. Do not conflate this store with the session transcript.

## 7. RLM and long-context handling

RLM means Recursive Language Model. In this discussion it is a harness/execution
method around existing models, optionally supported by model training for that
environment. It is not a requirement for a new base-model architecture.

A characteristic flow is:

```text
Goal + reference to large data
  -> model writes Python to inspect/filter/split it
  -> intermediate data stays in variables/files
  -> selected evidence enters model context
  -> optional child-model analysis
  -> integration and further inspection
```

Assigning a large object to a Python variable does not make the model see its
contents. Printed output, displayed values, attachments and explicit messages
are the parts that can enter model context. Printing the entire object loses
the context-saving benefit.

Important boundaries:

- Prime's inspected ordinary text admission constructs a user message. Do not
  claim arbitrary pasted long text is automatically externalized.
- To avoid paying for a huge first model input, ingestion must store the material
  before that request and provide a reference plus necessary description.
- Modern conventional coding agents can also search files, retrieve excerpts,
  compact history and delegate. Python does not uniquely enable those behaviors.
- Exhaustive semantic analysis still requires reading the relevant material.
  Recursive calls can increase cost and latency; no benchmark here proves savings.
- A Python workbench is useful without recursive subagents. RLM is a possible
  later capability, not the current experiment's minimum requirement.

## 8. Proposed minimal workbench boundary

This section records assistant recommendations, not newly accepted APIs.

Keep the model-facing frequent coding tools and add the optional workbench:

```text
Model -> direct read/write/edit/shell tools -> selected Rig/Vivy plugins
      -> python_repl -> deferred capability discovery/calls + data composition
```

The useful workbench primitives are persistent execution, named intermediate
state, bounded inspection/output, external capability discovery and invocation,
managed long-running work, and file/artifact delivery.

- Reuse an existing tool implementation if Python also needs that capability.
  Do not maintain independent edit/shell semantics on both sides.
- Keep one host owner for the agent loop, gateway model calls, transcript,
  cancellation and execution lifecycle.
- Use ordinary Python functions and packages where sufficient. A universal
  Plugin interface or public SDK is not a prerequisite.
- Persistence between cells does not imply recovery of arbitrary Python objects
  after worker restart. Preserve authoritative data in files when necessary.
- A cell is not a transaction. Partial writes or remote effects can precede an
  exception; do not automatically replay failed cells.
- Keep structured protocol traffic separate from stdout/stderr. Capture output
  honestly, preserve call identity, and make cancellation/status observable.
- A trusted Python process with OS access is not a security sandbox.

The current experiment already has an optional stdlib-only Python worker,
top-level await, state across ordinary calls, bounded output and Windows process
ownership. Its DESIGN records namespace loss on timeout/cancellation/restart
and explicitly excludes automatic installs, background tasks, snapshots,
subagent APIs and programmatic tool bridges. Those exclusions must not be
reported as implemented merely because they were discussed today.

## 9. Recipe/build semantics and unresolved choices

The established Rust recipe continues to choose compiled components. Selecting
a Python workbench plugin at build time is compatible with that experiment.

A future recipe selecting Python capability packages would also need to package
those sources and dependencies. That can be one assembly/build delivery, but
it does not make interpreted Python packages native Rust code compiled into
one executable. The earlier suggestion to ship a fixed Python environment was
a proposal, not an accepted packaging decision.

Still unresolved, awaiting the owner:

- PEN's actual purpose, requirements and relationship to the workbench.
- Which external capabilities should enter the first deferred catalog.
- Whether advanced tool descriptions stay exclusively in Python or can be
  promoted into the direct model tool set.
- Minimum metadata/discovery interface and necessary host bridge operations.
- Packaging, dependencies, restart semantics and observability for advanced work.

No implementation order, migration, public API, or full-Python conversion is
authorized by this checkpoint. Preserve the working experiment while the owner
explains the next idea.

## 10. Source map

- [Rig pinned overview](https://github.com/0xPlaygrounds/rig/blob/55cd481a2421fd5da468b5bd6afc6ac876883599/README.md)
- [Vivy Assembly contract at inspected remote main](https://github.com/ProjectViVy/agent-vivy/blob/9cd2801f8090723a652d3e835f72d83715b82121/docs/architecture/VIVY-ASSEMBLY.md)
- [Remake integration plan at inspected ref](https://github.com/ProjectViVy/agent-vivy/blob/0059b73138b3830fd3bfc1c041b9bc8353e93a33/docs/superpowers/plans/remake-20261019/index.md)
- [Vivy remote-main open board](https://github.com/ProjectViVy/agent-vivy/blob/9cd2801f8090723a652d3e835f72d83715b82121/docs/TODO.md)
- [Prime PR #3: explicit RLM training-interface rationale](https://github.com/PrimeIntellect-ai/prime-agent/pull/3)
- [Prime PR #323: removal of bash/edit built-in entries](https://github.com/PrimeIntellect-ai/prime-agent/pull/323)
- [Prime PR #3211: tool-selection flag removal](https://github.com/PrimeIntellect-ai/prime-agent/pull/3211)
- [Prime's RLM explanation](https://www.primeintellect.ai/blog/rlm)
- [Prime core model-facing interface](https://github.com/PrimeIntellect-ai/prime-agent/blob/5c898d1b27b109bd2d383c68d60a30536e39cc22/crates/pa-core/src/prompts/layers/core.md)
- [Prime workbench usage rules](https://github.com/PrimeIntellect-ai/prime-agent/blob/5c898d1b27b109bd2d383c68d60a30536e39cc22/crates/pa-core/src/prompts/layers/usage.md)
- [Prime REPL protocol](https://github.com/PrimeIntellect-ai/prime-agent/blob/5c898d1b27b109bd2d383c68d60a30536e39cc22/prime-agent-runtime/src/rlm/repl.md)
- [Prime generic MCP client](https://github.com/PrimeIntellect-ai/prime-agent/blob/5c898d1b27b109bd2d383c68d60a30536e39cc22/prime-agent-runtime/src/rlm/mcp.py)
- [Prime skill discovery](https://github.com/PrimeIntellect-ai/prime-agent/blob/5c898d1b27b109bd2d383c68d60a30536e39cc22/crates/pa-core/src/skills/discovery.rs)
- [Prime Python skill bootstrap](https://github.com/PrimeIntellect-ai/prime-agent/blob/5c898d1b27b109bd2d383c68d60a30536e39cc22/crates/pa-core/src/kernel/bootstrap/venv.rs)
- [Prime ordinary prompt admission](https://github.com/PrimeIntellect-ai/prime-agent/blob/5c898d1b27b109bd2d383c68d60a30536e39cc22/crates/pa-core/src/session_engine/admission.rs)

This delivery changes research documentation only. Validation is content review,
identical-copy comparison and Git whitespace checks. It does not establish new
runtime behavior or supersede either repository's implementation evidence.

## 11. Later discussion: embodied-agent workflows

The owner clarified that the Gemini example concerned a model operating a
physical robot, constructing a simulation environment, and learning execution.
The following are vendor-reported demonstrations, not independently reproduced
results. Keep the two experiments separate:

- Google's October 13, 2026 Argon announcement describes a human cornhole video
  being turned into environment coordinates, whole-body joint motions, and
  simulation code, then transferred to Apptronik's Apollo. Google reports seven
  consecutive successful shots. The announcement alone does not establish that
  this example trained a new neural controller, nor expose complete trial counts
  or controller implementation. Generating a simulation scene/program does not
  mean implementing a new physics engine.
- Google's robotics article describes a separate tabletop experiment: an RGB
  photo becomes a MuJoCo twin, tasks and a closed-loop trajectory generator;
  4,500+ randomized simulation episodes are exported for VLA fine-tuning. Google
  reports real-robot success increasing from 56% to 75%, and completion time
  decreasing from 35 to 24 seconds. The article does not fully specify the model,
  fine-tuning time, or who launched every training/deployment step.

Neither account establishes that Argon updated its own weights during the task.
Do not combine the VLA metrics with the Apollo demonstration, or claim a fully
unattended, reproducible training system from these announcements.

Architectural inference: a workbench can produce reusable executable capabilities
(scripts, optimized parameters, datasets, or model artifacts) through experiment
and feedback. This supports exploring the hybrid workbench; it does not establish
that every tool must move to Python, that RLM is required, or that a new plugin
platform is necessary. Rust/Rig can retain runtime ownership while Python runs
the relevant data-processing, simulation, optimization, or training programs.

Sources inspected on 2026-10-25:

- [Google announcement: Speeding up robotics development](https://blog.google/innovation-and-ai/models-and-research/gemini-models/introducing-gemini-4-argon/)
- [Google robotics article: Action improvement via simulation](https://developers.googleblog.com/gemini-4-argon-for-robotics/)

## 12. MCP SDK selection

The official SDK list inspected today places both Python and Rust in Tier 1.
This reflects protocol support and maintenance commitments, not a measured
comparison of production failure rates. Do not justify Python by claiming the
official Rust SDK is inherently immature or unreliable.

Python's current stable line is v2, with a breaking migration from v1; v1 is in
maintenance. Rust's rmcp provides client support, child-process stdio and
Streamable HTTP. Pin the chosen dependency and verify the actual target servers;
old tutorials and examples are not sufficient compatibility evidence.

Recommendation for this experiment: put MCP client connection ownership in the
Python workbench and avoid a duplicate Rust MCP connection manager. This fits
the deferred-tool boundary and lets large results remain in Python for local
processing. A Python client can call servers implemented in other languages.
This recommendation is not an implemented MCP bridge or a finalized public API.

Acceptance should exercise connection, tool discovery/calls, timeout,
cancellation and child-process cleanup, and reconnection after worker restart.
No comparative SDK reliability benchmark or new MCP integration test was run.

- [Official SDK tiers](https://modelcontextprotocol.io/docs/sdk)
- [Python v2 changes](https://github.com/modelcontextprotocol/python-sdk/blob/main/docs/whats-new.md)
- [Rust SDK](https://github.com/modelcontextprotocol/rust-sdk)

## 13. Data collection and event-recording direction

The owner identified poor Journal modeling in original Vivy as a concern and
stated that future Vivy requires excellent event records. This discussion did
not audit the original Journal, approve its replacement, or change its canonical
contracts. The owner accepted preserving this discussion; the design below is
a proposed experiment direction, not a shipped feature or frozen schema.

### 13.1 What Prime actually records

At the inspected Prime revision:

- The session is append-only JSONL with typed entries, IDs, parent IDs and
  timestamps. It includes conversation messages, tool-start/progress entries,
  model changes, compaction and other session facts.
- Python results carry status, duration, stdout/stderr, exceptions, kernel
  restart information and selected shell/agent-message details. These are richer
  than a single combined output string, but are not an audit of every operation
  performed by arbitrary Python code.
- Worker recovery and orphan-process journals serve lifecycle recovery separately
  from the conversation record.
- Session telemetry explicitly emits counts, durations and categories, excluding
  prompt text, model output, tool arguments and results. It is not a training
  trajectory store.
- The inspected Python MCP call paths return results to Python without an
  explicit per-call persistent event write at those boundaries. This is a scoped
  source finding, not proof that no other instrumentation exists anywhere.

Primary code references at `5c898d1b27b109bd2d383c68d60a30536e39cc22`:

- [Session entries](https://github.com/PrimeIntellect-ai/prime-agent/blob/5c898d1b27b109bd2d383c68d60a30536e39cc22/crates/pa-types/src/session.rs)
- [Python tool results](https://github.com/PrimeIntellect-ai/prime-agent/blob/5c898d1b27b109bd2d383c68d60a30536e39cc22/crates/pa-core/src/tools/ipython.rs)
- [Recovery journals](https://github.com/PrimeIntellect-ai/prime-agent/blob/5c898d1b27b109bd2d383c68d60a30536e39cc22/crates/pa-daemon/src/journal.rs)
- [Telemetry contract](https://github.com/PrimeIntellect-ai/prime-agent/blob/5c898d1b27b109bd2d383c68d60a30536e39cc22/crates/pa-core/src/session_engine/telemetry.rs)
- [MCP dispatch](https://github.com/PrimeIntellect-ai/prime-agent/blob/5c898d1b27b109bd2d383c68d60a30536e39cc22/prime-agent-runtime/src/rlm/mcp.py)
- [MCP base client](https://github.com/PrimeIntellect-ai/prime-agent/blob/5c898d1b27b109bd2d383c68d60a30536e39cc22/prime-agent-runtime/src/rlm/mcp_base.py)

### 13.2 Minimum useful recording model

Keep the model-visible tool set small without making execution records coarse.
Preserve the chain of observation, action, environment response and evaluation:

```text
User goal
  -> model request (actual visible context and configuration)
  -> model response / native tool or Python cell
       -> nested MCP call(s), results, errors
       -> cell output and produced artifacts
  -> subsequent model observations
  -> test result or human feedback, with its source
```

Distinguish data available inside Python from data actually shown to the model.
A large MCP response reduced to a short summary must not later be treated as if
the model saw the full response. Record effective prompts, tool definitions and
settings through durable references or snapshots sufficient to reconstruct the
model-visible request, subject to declared retention and redaction.

Reuse Rig's existing session persistence for conversations. Supplement it with
internal workbench events linked to existing message/tool-call IDs; do not make
a second authority for conversation state. Let Python emit structured events
over the existing bridge and let the Rust host own correlation and persistence.
Choose the concrete storage hook only after inspecting the pinned implementation.

Proposed automatic collection boundaries:

| Boundary | Minimum facts |
| --- | --- |
| Model request/response | Actual input references, model/settings, output, usage, error |
| Native tool / Python cell | Call identity, arguments/code, start/end, status, duration |
| MCP call inside Python | Parent cell, server/tool identity, arguments, result, error |
| Workbench lifecycle | Start, exit, reset, new kernel-instance identity |
| Evaluation | Test or human feedback, source, associated task/run |

A previously suggested candidate execution-event envelope is `version`, `event_id`, `session_id`, `run_id`,
`parent_id`, `operation_id`, `seq`, `timestamp`, `kind`, and `payload`. This is
illustrative, not a required SDK or an accepted replacement for Prime's harness
and refinement formats. Section 15 governs the evolution-format decision.
Reuse existing identifiers where possible.
The host sequence records receipt order; parent/operation links express causality
and pair start/end events. Timestamps alone do not order concurrent causes.

Collection belongs in runtime/adapters, not in model-authored voluntary logging.
Do not instrument every Python function or variable assignment. Arbitrary direct
HTTP/filesystem calls that bypass managed wrappers are outside detailed tracing;
cell code and captured output do not prove a complete side-effect audit.

Keep events small. Store large results, images, patches, datasets and model files
as referenced artifacts. Separate model-context truncation from storage retention;
mark truncation, omitted/redacted content and missing artifacts explicitly. Do not
claim full reproducibility when relevant data was not retained. Redact credentials
at collection boundaries and keep collected data local by default.

### 13.3 Failure semantics and training-data boundaries

- Persist an operation's start before a managed side effect. A start without a
  terminal record means its outcome is unknown, not necessarily failed. Surface
  persistence failures; do not claim durable recording when a write failed.
- Never automatically repeat a potentially effectful operation just because its
  completion record is absent. Local logging cannot guarantee exactly-once remote
  execution across a crash.
- Restoring a transcript does not restore Python memory. Record kernel resets,
  preserve necessary data as explicit artifacts, and do not add arbitrary object
  snapshots for this first experiment.
- Keep model declarations, process exit status, test outcomes and human acceptance
  distinct. They are different evidence, not one undifferentiated success flag.
- Derive training/evaluation datasets later from retained trajectories and sourced
  labels. Do not make a training-data schema the runtime's primary record format.
- No event bus, standalone collection service, general event-sourcing framework,
  or new Journal platform is required by this proposal.

### 13.4 Current implementation and next evidence

In vivy-test, `plugins/vivy-python/src/worker.py` already emits separate stdout,
stderr, result, error and done frames. The Rust receiver in
`plugins/vivy-python/src/lib.rs` largely combines text into tool output. The
existing `/export` JSONL in `plugins/vivy-basics/src/session.rs` exports conversation
entries; it does not establish detailed internal workbench tracing.

The inspected Rig harness has session persistence (`rig-ecs/src/session_log.rs`
and `rig-harness/src/host/session.rs`). Before implementing, verify the exact pin,
extension seam, durability and failure behavior instead of duplicating that path.

Future acceptance should show one run containing a native call, a Python cell,
nested MCP success/failure, a large artifact and a worker reset, all correctly
correlated; it should also show an interrupted operation remaining unknown and
the precise subset of data exposed to the model. These checks have not been run.

This update records research only. The original Vivy repair, PEN requirements,
MCP integration, event APIs and implementation scheduling remain separate work.

## 14. Correction: original Vivy already has a DIVA evolution path

The subsequent inspection corrected an incomplete comparison with old Agent
Diva: original Vivy's remote main `9cd2801f8090723a652d3e835f72d83715b82121`
already contains a real local Skill proposal bridge. Its locked Laputa source
is `0e0027c5f60ad67614d3613c2d5e21d567c4b412`. The legacy Evolution page using
demo data does not mean that all backend evolution is a demo.

The inspected path is:

```text
Committed primary-run terminal activity
  -> ObserverHost capture with stable IDs and receipts
  -> input watermark / trigger policy / busy-state checks
  -> compiled diva/v1 strategy
  -> collect -> prepare -> reconcile -> reflect -> effects -> finish
  -> typed memory/persona/Skill/etc. effects, each with its own authority
  -> local Skill revision + Review Center
  -> human approval -> apply -> later reads use the changed Skill
```

Capture excludes workflow/inference output and the cognitive supervisor's own
bookkeeping runs, avoiding their reuse as fresh conversation evidence. DIVA
selects an enabled user-owned Skill before proposing an update to the same
identity. The host retains source evidence, rationale, operation and approval
identity; rejection, expiry, cancellation, stale revisions and application
failures have durable outcomes. Restart reconciliation handles pending reviews
and approved-but-unapplied revisions. Approval timeout never grants consent.

These are source findings, not a new end-to-end acceptance result. The remake
R18 skill-evolution lane remains blocked for re-intake against the repaired core.
Its plan calls for semantic revision checks under the no-hash contract, local
Skill evolution by default, and EvoMap experimental/default-off. Generic strategy
composition is separate. Historical implementation does not prove current repair
acceptance, and a reviewed modification does not prove improved task performance.

Sources:

- [Vivy cognitive capture](https://github.com/ProjectViVy/agent-vivy/blob/9cd2801f8090723a652d3e835f72d83715b82121/internal/runtime/cognitive_service.go)
- [Vivy strategy binding](https://github.com/ProjectViVy/agent-vivy/blob/9cd2801f8090723a652d3e835f72d83715b82121/internal/runtime/cognitive_binding.go)
- [Vivy Skill proposal bridge](https://github.com/ProjectViVy/agent-vivy/blob/9cd2801f8090723a652d3e835f72d83715b82121/internal/runtime/cognitive_skill_proposal.go)
- [Pinned DIVA strategy](https://github.com/ProjectViVy/laputa/blob/0e0027c5f60ad67614d3613c2d5e21d567c4b412/laputa/evolution/diva/strategy.go)
- [Remake R18](https://github.com/ProjectViVy/agent-vivy/blob/979a10c1/docs/superpowers/plans/remake-20261019/lanes/R18-skill-evolution.md)

## 15. Accepted direction: Prime formats first, automatic evolution, Vivy persona

The owner explicitly clarified the anchor: use Prime's format, then add Vivy's
distinctive persona evolution. This is more specific than merely borrowing a
Prime-like philosophy. Do not design a new evolution format and label it similar
to Prime, or port DIVA's full workflow and approval platform into the experiment.

### 15.1 Concrete upstream baseline

Start from Prime `5c898d1b27b109bd2d383c68d60a30536e39cc22`:

- `harness/harness_state.json`: `schema`, `entries` keyed by kind then ID, and
  `refinements`. Existing kinds are `prompt`, `memory`, `skill`, `subagent`, and
  `factory`. Their presence in the format does not require implementing every
  runtime capability in the first experiment.
- `HarnessEntry`: preserve the upstream vocabulary, including `id`, `kind`,
  `title`, `content`, `path`, `scope`, `reference`, `arguments`, `metadata`,
  `source`, `created_at`, `updated_at`, and `version`.
- Refinement proposals: `summary`, `rationale`, `expectedOutcome`, and `edits`;
  edits use `create`, `update`, or `delete` over the selected entry kinds.
- Refinement history: `refinement_history.jsonl`, applied before/after records,
  scope and rollback linkage. Keep expected benefit distinct from measured
  evaluation; Prime's event `outcome` can contain the proposed expected outcome.
- Upstream scopes are `local` and `global`. Earlier suggestions of additional
  project/persona scopes are not a frozen replacement enum. Resolve ownership
  and directory mapping without casually changing the baseline format.

Inspect actual readers/writers before copying a schema. Preserve supported
semantics and explicitly document any extension or unsupported behavior. This
decision does not promise untested import/export compatibility with upstream,
nor require replacing Rig's entire session store with Prime's implementation.
It anchors evolution state/proposals/history; execution events must link to that
state without creating a competing source of truth.

### 15.2 Automatic lifecycle and proportionate checks

Use one small automatic lifecycle:

```text
New task evidence -> decide whether refinement is useful -> small typed edits
  -> checks appropriate to the changed content -> apply at a task boundary
  -> retain history -> observe later outcomes and support rollback
```

Automatic evolution is the desired normal behavior, not a mandatory approval
queue. Budget/cooldown and a legitimate no-change result keep it bounded. The
exact trigger thresholds remain implementation choices. Do not promote transient
task state into global memory, and keep scope assignment under host ownership.

Checks match the modification:

- A sourced preference or declarative lesson needs evidence, scope and conflict
  checks, not a full benchmark.
- An executable Python helper needs relevant isolated functional checks before
  automatic adoption. Reusing Prime's skill-reference format does not itself
  install or validate Python code; artifact creation is a separate operation.
- A claimed performance improvement needs actual evaluation evidence. Do not
  claim measured gains merely because a change applied successfully.

The earlier suggestion to begin with manual-only refinement and require full
baseline/candidate evaluation for every edit is superseded by this direction.
This is not authorization to bypass existing original-Vivy approval contracts;
the new policy belongs to the independent experiment.

### 15.3 Focused persona addition

Add persona growth to the same refinement lifecycle rather than building another
evolution engine. Evolving observations about the user, expression preferences,
relationship experience and self-understanding should remain evidence-linked,
correctable and versioned. Explicit owner-defined identity constraints and
mission remain fixed inputs unless the owner changes them.

Prime has no native `persona` kind at the inspected revision. The representation
is not yet frozen: inspect whether existing entry metadata/reference fields are
sufficient; if a distinct kind is needed, make it an explicit versioned Vivy
extension and update validation/readers accordingly. Do not assume upstream
readers preserve unknown kinds. Keep the new surface limited to persona needs.

Rust plugin and recipe changes still require rebuilding and testing the selected
artifact. Automatic behavioral evolution does not silently replace build-time
species composition. Model-weight training remains outside the initial scope.

### 15.4 Delivery boundary and references

Implement later in vivy-test, preserving the working coding core and Python
workbench. Do not alter original Vivy's repair lane, introduce a standalone SDK,
general evolution platform, new approval center, or species hash enforcement.
The owner requested a durable decision record, not implementation in this turn.

- [Prime harness formats and storage](https://github.com/PrimeIntellect-ai/prime-agent/blob/5c898d1b27b109bd2d383c68d60a30536e39cc22/crates/pa-core/src/refinement/mod.rs)
- [Prime proposal schema and application](https://github.com/PrimeIntellect-ai/prime-agent/blob/5c898d1b27b109bd2d383c68d60a30536e39cc22/crates/pa-core/src/refinement/planner.rs)
- [Prime refinement execution](https://github.com/PrimeIntellect-ai/prime-agent/blob/5c898d1b27b109bd2d383c68d60a30536e39cc22/crates/pa-core/src/session_engine/refine.rs)
- [Existing DIVA persona-effect boundary](https://github.com/ProjectViVy/laputa/blob/0e0027c5f60ad67614d3613c2d5e21d567c4b412/laputa/evolution/effects.go)

Verification for this documentation update: inspect the source-backed claims,
review the diff, compare both saved copies, and run Git whitespace checks. No
runtime behavior or new automatic-evolution capability is delivered here.

## 16. Accepted diagram and architecture review checkpoint

The owner approved documenting the miniature architecture and requested a final
review focused on hidden architectural risk and low-value code growth. The
implementation authority is `vivy-test/docs/DESIGN.md`, section "Hybrid architecture
and first-stage boundary". The original sketch is preserved in vivy-test at
`docs/research/assets/2026-10-25-owner-architecture.png`. This mirrored research
report records the decision; it is not a second specification to keep synchronized.

The accepted shape is one recipe-built Rig host with direct coding tools and the
existing optional Python worker. Rust retains the main model/gateway path, sessions,
terminal, context and refinement commits. Python supplies deferred MCP/data work
and future consumers when specified. Rig skills remain file-backed; Prime-shaped
harness state owns learned entries. ACTMEM is a view of local working state, not
another memory service. Python variables are ephemeral. External memory and PEN
remain unspecified and are not prerequisites for this experiment.

Three useful slices set the boundary: preserve/verify the Pi-style coding core;
deliver one deferred MCP task with lifecycle and correlated evidence; then add
bounded automatic Prime-shaped refinement and persona growth. The full Pi inventory
remains in `vivy-test/docs/PI-BASELINE.md`. A postponed capability is a gap, not
parity. There is no new SDK, Port catalog, custom TUI, daemon, generic RPC platform,
automatic package installer, model training or complete Prime port in these slices.

The review inspected the current workbench seam: `worker.py` cancels remaining
asyncio tasks after each cell. Persistent MCP clients therefore need deliberate
lifecycle ownership. The design also requires branch/reset handling, host callback
dispatch while awaiting Python, bounded context and artifacts, and a single writer
with crash reconciliation for harness state/history. Arbitrary Python I/O is not
a sandboxed or completely traced managed effect. These are design constraints,
not runtime fixes delivered by this document update.

Fresh independent review found two gaps, resolved in the design: local learned
state must follow the conversation branch (compatible revision or explicit reset),
and refinement bookkeeping must remember considered evidence, including no-change
outcomes, to prevent duplicate consumption after resume. Shared profile learning
persists across rewind and uses explicit rollback. The pinned Rig log reports write
failure without stopping effects; the new Python/MCP bridge needs an acknowledged
record-before-dispatch seam. This is not a claim of global fail-closed native tools,
nor a mandate to replace Rig's persistence with a second Journal.

Every new owned layer must deliver a concrete behavior that direct Rig/SDK reuse
cannot supply. Review owned-code/dependency changes and remove superseded owners;
do not set a line-count quota or reward additional infrastructure. Known remaining
implementation choices include the Rig append hook, minimal bridge frames and the
state/history commit procedure. Verify those with actual API and failure-path
evidence before claiming the corresponding slice complete.

Verification for this checkpoint: fresh read-only architecture review, document
consistency and local-link checks, original sketch byte comparison, identical
research copies and `git diff --check`. No runtime code, original-Vivy product
contract or repair-lane behavior changes. No runtime CI is needed for this
documentation-only checkpoint; implementation claims still require real-path tests.
