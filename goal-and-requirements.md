---
id: goal
type: goal-and-requirements
status: draft
title: Vivy — one recipe, one assembled agent
---

## Goal

Vivy is a Rust agent core for coding sessions and persistent assistants. One YAML
recipe selects components; one build assembles a runnable agent. It runs on Rig's
execution path, with a native terminal interface, configurable OpenAI-compatible
and Anthropic-compatible endpoints, and an optional Python workbench for tools,
memory and executable Skills.

Three recipes ship, all on the same core: `recipes/coding-agent.yaml` (the coding
agent), `recipes/persistent-agent.yaml` (the D4 assistant) and
`recipes/diva-agent.yaml` (that assistant behind the desktop front). *Unconfirmed
inference:* the audience is the person running an agent on their own machine — a
developer at a terminal, and the desktop user of the persistent assistant.

## Scope

- **Build-time assembly.** A recipe names plugin types per crate; the builder
  generates Cargo dependencies and static wiring. Changing the recipe and
  rebuilding changes what is compiled in. Runtime settings configure only
  capabilities already compiled. See *recipe-builder*.
- **Coding surface.** Trust, Pi's context and prompt files, coding/shell/search
  tools, sessions, model and reasoning controls, collaborative Plan mode, native
  subagents, prompt templates and the compact terminal view.
- **Providers.** OpenAI-compatible and Anthropic-compatible endpoints managed as
  instances with a model catalog, credentials, presets and desktop presets, with
  the older `[gateway]` table still supported.
- **Persistent assistant.** Durable tasks and timed wakes, the six-file Laputa
  persona, asynchronous Recap context and AutoDream.
- **Python workbench.** A persistent CPython worker with MCP connections, trusted
  Python Skills, optional native memory, and an offline `python_repl` tool.
- **Desktop front.** A JSON-Lines transport over the same runner, for Diva.

## Non-goals

- No species hash enforcement, provenance platform, standalone plugin SDK,
  universal Port catalog, conformance platform, Studio lifecycle, plugin
  marketplace or Pi TypeScript extension runtime. Ordinary Cargo locking and
  source pins remain dependency management.
- Pi's coding workflow is a coverage reference, never a second runtime; the wider
  Pi feature set is an incremental target, not a first-run gate.
- No sandbox. Compiled Rust plugins, Python, Skills and shell run with the user's
  permissions; trust, gateways and process ownership do not make them safe.
- No subscription login, Responses API, automatic model discovery, or installer;
  `vivy` builds and launches from a checkout.

## Principles

- **Selection is compilation.** A capability that is not selected is absent from
  the binary. A runtime-only enable flag is never presented as selection.
- **Reuse Rig before writing an equivalent**, and keep one composition root and
  one agent execution path; a front adds transport, not a second loop.
- **Credentials are names.** Configuration stores environment-variable names, and
  agent secrets are withheld from every process the agent starts.
- **Claim only what was run.** Recorded verification lives in `docs/logs/` and
  `docs/PI-BASELINE.md`; a platform or provider that was never exercised is not
  claimed.

## Limitations

- The real gateway is unverified: the environment denies it, and comparisons so
  far used a scripted Chat Completions endpoint.
- Windows has an owner-run acceptance; Linux and macOS are not verified in this
  checkout beyond the recorded scripted checks.
