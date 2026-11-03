---
id: diva-front
type: module-design
status: draft
title: diva-front (`plugins/vivy-diva-front`)
parent: architecture
depends-on: [basics, gateway, python, durable, laputa]
---

## Responsibility

The desktop transport and presentation for the persistent assistant: a
newline-delimited JSON request/response protocol on the agent's standard streams,
the bounded pipe reader that enforces it, projections of agent state, readiness,
conversation and reasoning, management adapters for everything the desktop
configures, and a bounded background-job model.

## Boundary

- It registers a front (`diva`), not an agent: requests become the same operations
  the terminal would perform, on the same runner.
- Management adapters call the owning module's API — providers through `gateway`,
  persona through `laputa`, tasks, wakes and Dream through `durable`, workbench
  through `python` — instead of reimplementing any of it.
- `src/protocol.rs` is the wire contract's single source; `contract/protocol.ts`,
  `contract/examples.json` and `contract/version.json` are derived from it for the
  desktop side. `VERSION` and `MAX_FRAME` live there.
- `files.rs` reads provider, persona, prompt, Skill and task files for the
  management views; feature `persistent` is what pulls in `durable` and `laputa`.

## Invariants

- One request per line, bounded at 1 MiB: a frame over the cap, a missing newline
  terminator or a non-UTF-8 payload is an explicit error and ends the stream rather
  than truncating silently.
- Requests reject unknown fields (`deny_unknown_fields`), so a desktop build and
  this binary cannot drift apart silently.
- Responses carry an instance id and a monotonic sequence, so the desktop can
  detect a gap or a restart.
- Readiness is projected from the assembled runtime — what is compiled in is what
  can be reported ready — never from a second hand-maintained capability list.
- Projections are bounded; oversized content is reported as truncated with its
  byte count rather than dropped.

## Limitations

- `vivy-laputa` is used unconditionally in `files.rs` and `runtime_controls.rs`
  while the dependency is optional behind feature `persistent`, so a build without
  `persistent` does not compile. The shipped `diva-agent.yaml` selects
  `persistent`, which is why the supported path builds. *Found by inspection; not
  confirmed by a feature-off compile.*
- Closing to tray, data directories and desktop packaging live in the desktop
  application, not here.
