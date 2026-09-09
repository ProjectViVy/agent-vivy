# Vivy Plugin Platform v1 Program Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> `superpowers:subagent-driven-development` (recommended) or
> `superpowers:executing-plans` to implement one scheduled phase task-by-task.
> Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the unreleased v0 plugin experiment with a modular,
compile-time v1 Module/Port/Generated-Assembly platform while preserving the
default Vivy product and keeping SCX on the single Runtime path.

**Architecture:** A sole Assembly Compiler resolves explicit Recipes and
pure-data Module Descriptors into typed wiring and a sealed Generation.
Internal and public Modules share composition mechanics but not authority;
runtime Module graphs are frozen and every execution reaches the existing
Service, Journal, Policy, and Eino adapter boundaries.

**Tech Stack:** Go, repository-pinned Eino v0.9.13 and EinoExt modules,
YAML/JSON Schema, generated Go wiring, React/TypeScript/Vite UI, `vivy-sdk`,
and `just ci`.

**Spec:** `docs/architecture/VIVY-MODULE-STANDARD.md`,
`docs/architecture/VIVY-PORT-CATALOG.md`,
`docs/architecture/VIVY-PLUGIN-SPEC.md`, and
`docs/architecture/VIVY-ASSEMBLY.md`.

## Global Constraints

- No v0 compatibility, Adapter, alias, deprecation track, or migration command.
- No functional work starts until its phase is manually scheduled.
- One `Service.Run`, Journal authority, Policy/approval path, ToolHost,
  ChannelHost, FaceHost, ActionHost, and Generation truth.
- Only `internal/runtime` and `internal/provider` may import Eino/EinoExt.
- For Provider/model/OAuth/orchestration/RAG/MCP adapter work: adapt a concrete
  pinned Eino/EinoExt API when present; otherwise mark the capability
  `DEFERRED-INDEFINITE` and add no custom substitute.
- All standard public Port Hosts and established first-party Providers are in
  the default Generation; unconfigured network instances remain inactive.
- External Modules are explicit Recipe entries; runtime directory discovery is
  forbidden.
- UI Modules have complete UI access by default and require no UI Grant or
  permission prompt; backend authority remains server-side.
- If execution uses subagents, use model `gpt-5.6-luna` with reasoning effort
  `max`, and isolate concurrent write lanes in separate git worktrees.
- P1+ and every functional/generated-code deliverable end in a focused commit,
  an iteration log, and `just ci`. P0 is a documentation-only freeze and records
  this delivery's explicit full-compile skip in its verification log.

---

## Program status

| Phase | Deliverable | State | Depends on | SCX relation |
|---|---|---|---|---|
| PLG-P0 | Normative contract and executable plan | `COMPLETE` | Approved decisions | Defines Gate A |
| PLG-P1 | v1 SDK and Assembly Compiler foundation | `UNSCHEDULED` | P0 | Gate A critical path |
| PLG-P2 | Default Generation zero-behavior parity | `UNSCHEDULED` | P1 | Gate B critical path |
| PLG-P3 | Unified Tool governance and protected Tools | `UNSCHEDULED` | P2 | Gate B critical path |
| PLG-P4 | Context, Skill, and MCP Hosts/Sources | `UNSCHEDULED` | P3 | Gate B critical path |
| PLG-P5 | Declarative Provider Profile and Eino adapters | `UNSCHEDULED` | P2 | Conditional Gate B input |
| PLG-P6 | Full-code UI Modules and Control Actions | `UNSCHEDULED` | P2 | Not on core SCX path |
| PLG-P7 | Closed internal Module composition | `UNSCHEDULED` | P2, P3, P4, P5 | Gate B hardening |
| PLG-P8 | SCX integration Gates A/B/C | `UNSCHEDULED` | P1–P7 as identified | Direct SCX integration |
| PLG-P9 | Release conformance, Inspect, removal, rollback | `UNSCHEDULED` | P1–P8 | Gate C critical path |

Only a human changes an `UNSCHEDULED` phase to scheduled. Dependency order is
an execution constraint, not a calendar commitment. P0 is complete as a
documentation-only contract freeze; it starts no implementation lane.

## Critical path

```text
P0 contract
  -> P1 compiler/SDK                      [SCX Gate A]
  -> P2 default parity
  -> P3 Tool governance
  -> P4 Context/Skill/MCP
  -> P7 internal composition
  -> P8 SCX integration                  [SCX Gate B]
  -> P9 release proof                    [SCX Gate C]
```

P5 joins Gate B only for an SCX slice that requires its Provider Profile.
P6 may execute in parallel after P2 in an isolated worktree and does not block
SCX core semantics.

## Phase documents

- `00-standing-orders.md` — authority, execution, evidence, and scheduling.
- `PLG-P0-contract-freeze.md` — this documentation-only delivery.
- `PLG-P1-v1-sdk-and-compiler.md` — clean-break SDK and compiler.
- `PLG-P2-default-generation-parity.md` — existing first-party default body.
- `PLG-P3-tool-governance.md` — ToolHost, protected Tools, Middleware, Observers.
- `PLG-P4-context-skill-mcp.md` — source Hosts and T3 MCP instances.
- `PLG-P5-provider-eino-adapters.md` — declarative profiles and verified adapters.
- `PLG-P6-full-ui-modules.md` — unrestricted UI and typed backend Actions.
- `PLG-P7-internal-moduleization.md` — required and optional internal Ports.
- `PLG-P8-scx-integration-gates.md` — SCX dependency and acceptance mapping.
- `PLG-P9-release-conformance.md` — whole-product proof and release closure.

## Definition of program completion

The program closes only when all phases are complete, every public Port claimed
as `SUPPORTED` has the seven required artifacts, the default Generation has
behavior parity, a minimal Generation proves real removal, all SCX gates pass,
Inspect reports sealed provenance, whole-Generation rollback is demonstrated,
the v0 API and path no longer exist, and `just ci` plus required real-path
smokes are green.
