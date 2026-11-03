---
id: workbench-python
type: submodule-design
status: draft
title: vivy_workbench (`plugins/vivy-python/python/vivy_workbench`)
parent: python
---

## Responsibility

The Python half of the workbench: the cell loop that gives an agent one persistent
namespace and asyncio loop, the capability and readiness surface, the MCP
connection registry, the native-memory facade, artifact spill, environment
preparation and validation, and the error types the host turns into tool results.

## Boundary

- It runs as the agent's child process and speaks the worker protocol to
  `worker.py`, owned by `python`; it holds no policy about which capabilities
  exist, because the recipe already decided that.
- Dependencies come only from the hash-locked requirement sets; a cell imports the
  project's libraries itself.
- Markdown instructions stay lazy (`workbench.list_skills()`,
  `workbench.instructions(name)`); a Skill with a `pyproject.toml` is installed
  during preparation and imported before the first cell.

## Invariants

- One namespace per live agent, with top-level `await` and tasks that survive a
  cell; separate agents, forks, session switches and resumes get separate ones. The
  default cell timeout is 60 seconds.
- Text, oversized representations, MCP envelopes and MIME displays spill to
  `.vivy/workbench/artifacts`, partitioned by session, agent, generation and call,
  with path, byte count, media type and completeness. Hitting the 256 MiB
  per-artifact/journal limit, or a disk failure, is an explicit error — never a
  successful "complete" descriptor.
- Memory uses the native drawers collection's vector search with explicit
  source/wing/room filters, not MemPalace's broader ingestion or hybrid search.
  Same source, event, content and partition deduplicate; a changed content or
  partition conflicts and an error envelope is retained rather than retried.
  Explicit events are limited to 1 MiB, and complete reads validate ordered chunks
  against their content hash.
- Cross-process locking surrounds open, operation and close, reads included; lock
  contention fails before anything is written.
- `readiness` distinguishes installed MCP, prepared native memory and unavailable
  capabilities, so an absent capability is reported as absent.

## Limitations

- No MemPalace MCP server and no L0 persona are loaded; durable and automatic
  ingestion are outside this delivery.
- `limit` caps matching chunks before logical-record deduplication, so a search can
  return fewer distinct memories than asked for.
- An interrupted synchronous write may already have committed, so a retry must
  inspect the same source and event rather than replay blindly.
- A process crash can leave an incomplete artifact journal; reset retains files.
