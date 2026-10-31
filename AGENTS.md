# Vivy

The official Rust Vivy project, using YAML-driven, build-time agent assembly.
Read `docs/DESIGN.md` and `docs/HANDOFF.md` before implementation. This checkout
contains the recipe builder, native Windows launcher and optional Python
workbench; consult the recorded verification before claiming platform support.

## Working rules

1. **Stay independent.** Clone only Pi and Rig as reference projects under
   `.workspace/`, following the handoff. Do not clone, import, vendor, modify, or
   use the archived Go Vivy checkout as a build/runtime dependency. Do not access
   its sessions, databases, credentials, or production data. Its repair work is
   outside this repository.
2. **Keep the assembly core.** One YAML recipe selects Rust components;
   generated Cargo dependencies and static wiring assemble one runnable agent.
   Change the recipe and rebuild to change compiled plugins. Runtime settings
   configure compiled capabilities. Do not silently substitute runtime-only
   enable flags for compilation selection.
3. **Apply More, Fast, Good, Frugal.** Build the smallest coherent implementation
   meeting the current acceptance case. Reuse Rig before writing equivalents;
   keep execution paths direct; preserve readable code, useful errors, and data
   integrity; minimize total maintenance, dependency, runtime, and user effort.
   Add a layer only for an observed requirement. Measure build/iteration costs
   before optimizing or claiming improvements.
4. **Use a gateway first.** Start with one configurable endpoint, model ID, and
   environment-variable credential reference through a suitable Rig adapter.
   Keep routing/auth/provider aggregation at the gateway where available. Send
   the model identifier required by that endpoint without invented rewriting.
   Validate streaming and tool calls on the actual gateway before claiming
   compatibility. Do not build a provider catalog or gateway service preemptively.
5. **Keep composition small and explicit.** Prefer existing Rig types and small
   typed constructors/adapters. Add shared interfaces only when actual plugin
   consumers need them. One composition root and one agent execution path are
   enough. Avoid a second hand-maintained capability list: derive displayed
   availability from the assembled runtime. Treat compiled Rust plugins as
   trusted process code, not a sandbox.
6. **Keep the agreed exclusions.** No species hash enforcement, provenance
   platform, standalone plugin SDK, universal Port catalog, conformance platform,
   Studio lifecycle, or Pi TypeScript extension runtime for this project.
   Cargo.lock and normal version pins remain welcome. Use the Pi baseline for
   later coverage without making every item a first-run gate. Any scope change
   must be explicit.
7. **Work directly and protect concurrent work.** Use `rg` and CLI/API tools;
   batch independent reads. Complete small tasks directly. Delegate only useful
   independent work with clear ownership. Concurrent write lanes use separate
   branches/worktrees; leave unrelated changes untouched. Reference checkouts
   remain reference material; own product changes in this repository.
8. **Verify the behavior changed.** For documentation, review the diff and run
   `git diff --check`. Once Rust code exists, run formatting and focused build/
   test checks plus a real-path smoke appropriate to the change. Recipe changes
   need parse/error cases and selected-dependency evidence; execution changes
   need a representative failure/cancellation check. Keep unit tests offline.
   State what was inspected, built, tested, or remains unverified; never report
   source inspection as a successful run.
9. **Protect secrets and preserve causes.** Store credential environment-variable
   names rather than values in checked-in configuration. Redact tokens from
   errors and logs; do not commit private sessions. Preserve error causes and
   clean up owned processes/resources on cancellation. Gateways and Rust plugins
   do not make shell or filesystem operations safe by themselves.
10. **Keep communication and records lean.** Reply in the user's language; write
    durable docs, code, commits, and PR descriptions in English. Keep research
    and design under `docs/`; update the existing relevant document rather than
    multiplying checklists. Use proportionate skills, not mandatory ceremonies.
    Commit explicit related paths as one focused deliverable using the configured
    responsible human identity; no AI authors/co-authors. Push only with user
    authorization, including authorization already given in the conversation.

These rules retain useful ideas from Vivy's instructions without importing its
Go/Eino implementation rules, `just ci`, SDK gates, hash checks, release process,
or mandatory per-iteration document bundles. Commands must belong to this project
before they become its validation requirements.
