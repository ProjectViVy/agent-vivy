---
id: python
type: module-design
status: draft
title: python (`plugins/vivy-python`)
parent: architecture
depends-on: [basics, process]
---

## Responsibility

The host half of the workbench: one persistent CPython worker per live agent, the
`python_repl(code, timeout_secs?)` tool, the MCP connection registry, optional
native-memory tools, the binding of trusted Python Skills, and the preparation,
validation and publication of the environment the worker runs in.

## Boundary

- The Python half is `vivy_workbench`;
  this crate owns the process, the tool surface, the settings and the permission
  policy.
- It holds the child through `process` like every
  other tool process, and shares status types with
  `basics`.
- Feature-selected: `memory-tools` adds the memory tools, `managed-skills` the
  trusted-Skill binding, `protocol-types` the desktop contract. A recipe that
  omits the component excludes the crate and its embedded worker.

## Invariants

- **Installation is approval-scoped.** Approving preparation installs the
  hash-locked dependencies and trusted Skills, validates them and publishes
  `.vivy/workbench/current.json`; changed locks, Skill metadata, sources or the
  memory switch need approval again. Ordinary cells never install packages.
- Skills and their dependencies load only from the trusted inventory; broken
  imports, missing host handlers, collisions and reserved names are reported as
  unavailable without hiding the rest.
- The environment is adopted at the next safe cell boundary, so a running cell is
  never swapped underneath.
- Cancellation and timeout interrupt the cell, allow bounded unwinding, and then
  terminate the owned process tree.
- Changing the credential-variable set resets existing workers, so a later call
  cannot keep a stale environment.
- A live agent owns one namespace; separate agents, forks, session switches and
  resumes get separate or fresh ones. Reset retains artifacts and committed
  memories.

## Limitations

- Automatic toolchain bootstrap is validated on Windows x64 only; other platforms
  need an existing `uv` and Python.
- Automatic preparation also requires a locally recorded successful validation, so
  an upgraded or copied workbench needs one explicit `/setup python` or
  `/python prepare` first.
- No live-resource migration and no automatic replay of interrupted work.
- The workbench is not a sandbox.
