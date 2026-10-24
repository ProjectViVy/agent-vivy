# Research and decision snapshot: Rig, Vivy, Prime Agent, and the Python workbench

Date: 2026-10-25

Status: Research checkpoint; implementation paused pending the owner's next idea.

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
- The owner explicitly asked to save today's research before continuing.
  Do not start implementation from this report while that next idea is pending.

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
