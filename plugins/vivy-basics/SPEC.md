---
id: basics
type: module-design
status: draft
title: basics (`plugins/vivy-basics`)
parent: architecture
depends-on: [process]
---

## Responsibility

The coding surface, as separately selectable plugin types, one recipe component
per concern: the default prompt, project trust, context files, prompt templates,
search tools, the shell tool, session commands, model and reasoning controls,
collaborative Plan mode, branch summaries, `subagents`
and the `tui`.

Each wraps Rig. The agent loop, sessions, tools, terminal widgets and compaction
are Rig's; this crate selects, configures and presents them, and adds only what Rig
does not carry.

## Boundary

- Depends on `rig-harness`, `rig-basics` and (feature `tui`) `rig-tui` at the
  recipe's revision, plus `process`.
- Never reimplements a Rig primitive: a Pi-shaped behaviour that Rig already has
  is a plugin over it, not a parallel implementation.
- Consumed by `gateway` and `python` (shared status types) and optionally by
  `durable`; it does not depend on any of them.

## Invariants

- **Trust is decided once.** `trust::allowed` is the single check other plugins
  call before loading a project's own resources; protected are `.rig/prompts`,
  `.rig/skills`, `.rig/agents`, `.agents/skills`, `.rig/SYSTEM.md` and
  `.rig/APPEND_SYSTEM.md`. `AGENTS.md`/`CLAUDE.md` load regardless, as in Pi. The
  decision order is `--approve`/`--no-approve`, then the nearest saved decision in
  `RIG_HOME/trust.json`, then `[trust] default`.
- Empty discovery is not permission: a project with no protected resources found
  at startup is not thereby trusted for resources created later.
- A project's prompt template wins over the same name in `RIG_HOME/prompts`, and a
  name another command already has is skipped with a warning. The template plugin
  registers in `finish`, so every other plugin's command names win.
- Trust is not a sandbox: tools still act with the user's rights.

## Corrections

- `templates.rs`'s module comment says a project's templates load without a trust
  step. The code filters both roots through `trust::allowed`, and `trust.rs` and
  the README agree that they are trust-gated; the comment is stale and was
  corrected in place.
