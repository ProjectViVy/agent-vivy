# Plugin platform v1 plan summary

## Scope

This delivery freezes the Vivy plugin platform v1 architecture and turns the
approved decisions into manually schedulable implementation plans. It is a
documentation-only P0. No Go, TypeScript, generated code, runtime behavior, or
test fixture was changed.

## Changed

- Added normative architecture contracts for Modules, Ports, plugin v1, and
  generated Assembly.
- Marked older plugin, Face, Channel, Gateway, Studio, remote-control, and
  research material as superseded where it conflicts with the v1 contracts.
- Added the detailed P0-P9 plugin-platform execution plan under
  `docs/plans/plugin-platform/`.
- Updated `AGENTS.md` so plugin work must route through the v1 Module/Port
  standard.
- Updated `docs/TODO.md` with the PLG-1 ruling, SCX gates, and P0 status.
- Created `.agents/skills/vivy-plugin/SKILL.md` and converted
  `vivy-plugin-five` into a legacy redirect that rejects v0 work.

## Not done

- No compiler, SDK, Host, UI, runtime, SCX, or conformance implementation was
  started.
- P1-P9 remain `UNSCHEDULED` and require human scheduling before code work.
- Full `just ci` was not run to completion for this docs-only closeout after
  the human questioned the compile step; the skip is recorded in
  `verification.md`.
