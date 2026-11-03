---
id: basics-subagents
type: submodule-design
status: draft
title: subagents (`plugins/vivy-basics/src/subagents`)
parent: basics
---

## Responsibility

Native delegation on Rig's execution path: the `task`, `message`, `wait`,
`task_parallel` and `task_chain` tools, named roles (scout, planner, reviewer,
worker plus Markdown roles from `RIG_HOME/agents/*.md` and a trusted project's
`.rig/agents/*.md`), child conversations, bounded results and cancellation.

## Boundary

- Children run in the same process and share the parent's project context. This is
  delegation, not isolation: no OS sandbox, no separate worktree.
- Tool access stays within the parent's and the role's own selection; `cwd` stays
  inside the project.
- It presents through the terminal like any other tool call, and it is its own
  recipe component: removing the component removes delegation.

## Invariants

- At most four children run per session, follow-ups included; parallel calls take
  1–8 tasks.
- A chain passes the previous bounded answer through `{previous}` and stops on
  failure.
- Role files are re-read on each delegation; a child already running keeps the role
  snapshot it started with.
- Esc stops the turn and its active and queued children; on resume, completed child
  conversations are retained and interrupted work is not replayed.
- Default nesting depth is one.

## Limitations

- Not a sandbox: children share the process and the user's rights.
- Parallel writers need distinct files, because nothing serializes them.
