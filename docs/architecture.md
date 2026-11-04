---
id: architecture
type: architecture-design
status: draft
title: Vivy architecture — one core, three species
parent: goal
---

## Shape

A recipe is the only assembly input. `recipe-builder` parses it, writes
`.generated/<name>/` as a Cargo package with its own workspace, and builds it with
Cargo; the result is the species binary, launched directly or through
`vivy run`. Nothing is selected at runtime: the generated manifest lists the
selected crates and the generated `main.rs` adds the selected plugin types through
`rig_harness::load`, so a component the recipe omits is a crate the agent does not
contain.

Three recipes are the three species of the same core
(`recipes/coding-agent.yaml`, `recipes/persistent-agent.yaml`,
`recipes/diva-agent.yaml`). They differ in selected components, not in execution
path: every species runs Rig's agent loop, terminal view and session machinery,
and every front (`terminal`, print, JSON, `diva`) is a front of that one runner.

## Boundaries

| Spec | Crate | Owns | Depends on |
| --- | --- | --- | --- |
| *recipe-builder* | `crates/vivy` | Recipe parsing and checking, generation, build, launch | *process* |
| *process* | `crates/vivy-process` | Owned child processes, credential filtering, session ownership | — |
| *basics* | `plugins/vivy-basics` | The coding surface: trust, context and prompt files, templates, tools, session commands, model controls, Plan, *subagents*, *tui* | *process* |
| *gateway* | `plugins/vivy-gateway` | Provider instances, model catalog, credentials, presets, setup UI | *basics*, *process* |
| *python* | `plugins/vivy-python` | The persistent CPython worker, its MCP connections, memory tools and Skill binding; *workbench-python* | *basics*, *process* |
| *laputa* | `plugins/vivy-laputa` | The six-file persona: storage, revisions, publication, injection, onboarding | — |
| *durable* | `plugins/vivy-durable` | The persistent task lifecycle, wakes, assistant context, Recap and AutoDream | *gateway*, *laputa*, *python*, *basics*, *process* (all optional) |
| *diva-front* | `plugins/vivy-diva-front` | The desktop wire contract, transport, projections and management adapters | *basics*, *gateway*, *python*, *durable*, *laputa* (feature `persistent`) |

Every crate additionally depends on one pinned Rig revision (`rig-harness`,
`bevy_ecs = 0.20`). `durable` and `diva-front` reach their optional siblings
through Cargo features, so the recipe decides whether those edges exist.

## Invariants

- **Compilation selection.** A plugin crate is built with `default-features =
  false` and receives only the features the recipe lists for it. No runtime flag
  stands in for a component the recipe left out.
- **One Rig revision per agent.** The recipe names `rig.git` and `rig.rev`;
  generated dependencies and each selected plugin point at it. Crossing a
  revision mismatch is unchecked (see Limitations).
- **Separate workspaces.** The root workspace excludes `plugins/`, `.generated/`
  and `.workspace/`; the generated agent and each plugin crate build in their own
  workspace, so an agent never compiles a crate the recipe did not select.
- **One execution path.** A front adds transport and projection; it never adds a
  second agent loop.
- **No secret leaves the agent.** Configuration stores credential *names*;
  `process` withholds `RIG_SECRET_*` and registered credential variables from
  every process the agent starts, and only the user's settings are read for
  providers — never a project's.
- **Compiled defaults, editable prompts.** `prompts/` ships the compiled-in
  default texts; a trusted project `.rig/` override wins over `RIG_HOME`, which
  wins over the compiled default. Editing a prompt changes how a compiled
  capability is instructed, never which capabilities exist.
- **Trust is not a sandbox.** Trust decides whether a project's own resources
  load; tools, Python and shell still act with the user's rights.

## Records and evidence

`docs/DESIGN.md`, `docs/HANDOFF.md` and `docs/PI-BASELINE.md` are dated decision
and verification records. They remain as evidence and history; the spec graph —
this document, the goal and the module specs — is the current statement of what
Vivy is and how it is shaped. Rationale for a decision belongs in the spec that
owns it, not in a code comment.

## Limitations

- **Recipe/revision coupling is unchecked.** Each crate independently pins the Rig
  revision; nothing verifies that a recipe's `rig.rev` matches the selected
  plugins, and a mismatch surfaces only as a compile or link error.
- **The real gateway is unverified.** Only a scripted OpenAI-compatible endpoint
  was exercised; the environment denied a real provider.
- **Platforms.** Windows has an owner-run acceptance; Linux and macOS rest on
  scripted checks, not on a run recorded here.
- **The persistent species needs an unpublished Rig fork.** `persistent-agent` and
  `diva-agent` require the local Rig checkpoint implementation via `--rig-path`;
  the upstream pin alone does not provide it.
