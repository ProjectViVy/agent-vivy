---
id: process
type: module-design
status: draft
title: process (`crates/vivy-process`)
parent: architecture
---

## Responsibility

Process ownership and credential policy for everything the agent starts: the
build, the shell tool, the Python worker and its toolchain processes.

It answers two questions in one place: *what environment may a child see*, and
*what happens to a child when its owner goes away*.

## Boundary

- No agent logic and no dependency on the harness beyond the protocol crate, so
  every other module may use it without a cycle.
- The Windows and Unix implementations are the only platform-specific code here;
  callers stay platform-free.

## Invariants

- `command(program)` hands out a `Command` without `RIG_SECRET_*`, the
  agent-only `RIG_*` variables, or any registered credential variable.
- It records credential variable **names**, never values; names come from
  settings and from `.env`-style files, and the policy generation bumps when the
  set changes so live workers can be reset.
- An `OwnedChild` kills the process it owns when dropped: the tree on Windows via
  a job object, the group on Unix. Cancellation is not left to a child's good
  behaviour.
- `session` keeps one writer per session (`EXECUTION_LOCK`, `SessionGuard`) and
  clears launcher context, so two launchers do not drive the same session.
- **This is not a sandbox.** It withholds secrets from the environment it hands a
  child and stops the processes it owns; those processes still act with the
  user's rights on the user's files.
