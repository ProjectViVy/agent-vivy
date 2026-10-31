# Python workbench integration

Status: implementation and whole-branch review fixes complete; local W1–W5 and
M1/M2 verification recorded in DESIGN.md. The owner deferred the actual-provider
M3 gate to a manual run after switching provider. Durable remains deferred.

Authority: DESIGN.md and research/2026-10-25-hybrid-agent-workbench-decisions.md
sections 19.9–19.11, plus the approved conversation plan. Durable is deferred.

## More, Fast, Good, Frugal

More means closing every accepted lifecycle, not counting imports as features.
Fast means one vertical path with focused checks before broader verification.
Good means preserving causes, ownership, full results and memory identity through
failure and cancellation. Frugal means adapting Prime's bounded runtime mechanisms,
reusing Rig and vivy-process, and integrating the full upstream MemPalace engine
through a thin facade. No second agent loop, SDK, database or capability catalog.
Measure preparation, install size, cold/warm calls, resident memory and cleanup;
do not claim an optimization without measurements.

## Tasks and interfaces

1. Package the owned Python runtime; retain Prime MIT notice and donor commit
   c4c238ca025e382d96fb5f37e6328ca77fb82ef3. Pin core MCP 1.27.0 separately from
   native memory (MemPalace f41e8e75ddaadf5588d5127715fa531d79eadb45 / 3.12.0,
   Chroma 1.5.8). CPython 3.13.12 and current Rust/Rig are the Windows baseline.
   Reference checkouts must not become runtime dependencies.
2. Port compile/eval, top-level await, persistent event loop, attributed output,
   host replies and targeted interrupts. Rust continuously drains protocol events;
   version, worker generation, agent and string request IDs identify work.
   Execute is sequential; host replies and interrupts bypass that queue. EOF,
   malformed frames and crashes fail pending work and allow a fresh next worker.
3. Keep process-tree ownership in vivy-process. Queued cancellation cannot kill
   another call. Running cancellation targets the cell, then closes the worker
   within five seconds. Ordinary errors preserve state and normal completion keeps
   async resources. Add /python status, reset and close. Never replay cells.
4. Add explicit cancellable /python prepare using uv, locked dependencies and
   atomic environment publication under a cross-process lock. Ordinary calls never
   install. Consume the trusted Skill inventory: lazy Markdown, preimport Python
   packages (run callable or module), ordinary explicit imports, reserved-name and
   failure isolation. Reset refreshes bindings; dependency changes require prepare.
5. Adapt Prime MCP registry, lifecycle and pagination. Expose list_connections,
   list_tools, search_tools, describe_tool, call_tool, reload and close. Exercise
   real stdio and Streamable HTTP, lazy reuse, concurrent opening and shutdown,
   credentials by environment-variable reference, filtered child environment,
   full result envelopes and isError. No auto retries, OAuth or arbitrary host API.
6. Persist complete text, MCP envelopes and MIME displays before abridging the
   model view. Report size, media type, completeness and path. Disk/cap failures
   are errors. Partition artifacts by session/agent/call and retain them on reset.
7. Preserve YAML omission, no-TUI, Coding/Plan/delegation/JSON and existing tools.
   Workers belong to live agents: compaction retains, fork starts fresh, switch,
   deletion and exit close. Coding acceptance discovers Skill/MCP, fetches synthetic
   data, processes across cells, writes and reads a full artifact, then exercises
   error, cancel, crash and reset. Pass W1–W5 before declaring memory accepted.
8. Integrate full MemPalace without importing mempalace.mcp_server. Async facade:
   remember/search/get/close. Logical IDs hash source_id + event_id; same identity
   and content deduplicate; changed content or partition conflicts; different
   sources remain distinct. Validate complete ordered chunks and content hashes.
   Use a single executor per worker and the upstream cross-process lock around
   open -> operation -> close, including reads. Bounded wait happens before write;
   interrupted writes are uncertain and require explicit lookup, never replay.
   Isolate palace/config/cache/locks. Explicitly prepare local multilingual
   embeddinggemma assets and identity; ordinary operations stay offline.
9. Run offline protocol, Rust and real-process fixtures, generated-recipe/no-Python/
   no-TUI regressions, Windows terminal and authorized real-gateway joint scenario.
   Run 20 lifecycle cycles and measure costs. Update English evidence and usage
   docs, obtain one whole-branch review, commit explicit related paths; do not push.

## Acceptance

W1: persistent runtime and protocol failures/cancellation. W2: preparation and
trusted loading. W3: both actual MCP transports and lifecycle. W4: Coding flow.
W5: ownership, repeated cleanup and cost measurements. M1: memory identity and
complete reads. M2: competing writers and interruption. M3: joint real-model flow.
Offline tests and source inspection cannot substitute for actual gateway evidence.

## Exclusions and review focus

No durable, snapshots/replay, Prime factory, persistent bash, evolution, marketplace,
OAuth, automatic ingestion, Obsidian synchronization, persona redesign or second DB.
Review especially late events, queued cancellation, shutdown/open races, atomic
environment publication, secret exposure, partial writes, chunk loss, same-source
conflicts, unexpected imports, and compilation selection.
