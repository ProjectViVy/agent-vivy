---
name: vivy-plugin
description: Use when planning, implementing, reviewing, verifying, packing, or inspecting a Vivy v1 Module, Port, plugin, Recipe, or Generation.
---

# Vivy plugin v1 development

## Core rule

Build one typed contribution to one Vivy body. Preserve the single generated
Assembly, `Service.Run`, Journal, Policy, and Host path.

Read `docs/architecture/VIVY-MODULE-STANDARD.md`,
`VIVY-PORT-CATALOG.md`, `VIVY-PLUGIN-SPEC.md`, `VIVY-ASSEMBLY.md`, and the
scheduled phase under `docs/plans/plugin-platform/` before editing.

## Route the work

| Change | Route |
|---|---|
| Module/Port/Compiler/Host or product contract | **REQUIRED SUB-SKILL:** use `vivy-kernel-ci` |
| Eino-scoped adapter | **REQUIRED SUB-SKILL:** use `vivy-eino` |
| Web UI Module implementation | **REQUIRED SUB-SKILL:** use `oil-frontend` |
| Public Module using a supported Port | Stay in its source boundary; verify, pack the explicit Recipe, and inspect |
| Port is only `SPECIFIED` or phase is `UNSCHEDULED` | Stop implementation and report the unmet Gate; never fall back to v0 |

## Required development record

Before code, name:

1. Module ID, T1/T2/T3 classification, and source pin;
2. existing cataloged Port and exact version;
3. Provider, sole Host Consumer, authority owner, and cardinality;
4. requested Grants and Recipe constraints;
5. lifecycle, timeout, cleanup, and failure result;
6. Definition, SDK, Provider, Consumer, Failure Model, Conformance, and Inspect
   artifacts;
7. the scheduled plan task and its failing test.

For Provider/model/OAuth/orchestration/RAG/MCP adapter work, cite the concrete
pinned Eino/EinoExt package/API. Adapt it when present; otherwise record
`DEFERRED-INDEFINITE` and add no custom substitute.

## Non-negotiable boundaries

- No `vivy.plugin/v0`, `Seam`, God `Plugin` interface, compatibility Adapter,
  migration command, runtime discovery, hand-edited generated wiring, or
  `engine.go` plugin import.
- Public Modules provide only cataloged `std/*` Ports; they never provide
  `core/*` or access raw Journal, Policy, storage, credentials, RPC server, or
  Eino types.
- Protected Tool IDs cannot be shadowed, aliased, replaced, or overridden.
- All Tool sources use ToolHost. Channels, Faces, Context, Skills, MCP,
  Middleware, Observers, Status, and Actions use their named Host.
- All first-party features remain in the default Generation; unconfigured
  network instances stay inactive.
- Selected UI Modules have complete UI control by default. Do not add a UI
  Grant or permission prompt. The backend still distrusts every browser claim.
- Same-process T2 code is trusted, not sandboxed. Untrusted execution is T3.

## Example decision

Request: “Publish another `read_file` plugin.”

Result: reject the reserved identity. If the intent is a distinct analyzer,
define a namespaced `std/tool@v1` Provider, request scoped `fs.read`, pass the
ToolHost conformance suite, add it to an explicit Recipe, rebuild, and verify
its source/hash/Grant in Inspect.

## Completion

Use test-first implementation. Run focused tests, the phase gate, `just ci`,
and the real plugin path once v1 commands are supported. Record the iteration
log and commit one concern. A browser refresh never installs a Module.
