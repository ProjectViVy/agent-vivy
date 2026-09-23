# Nudge detailed architecture and execution package

2026-09-23. Issue #58; revision ND-D1; baseline a8d361b0244a1c40be513622bbdaebb5c9d40014.

Delivered one detailed engineering design and five Story plans with exact ownership, internal contracts, immediate dependencies, verification commands and failure/stop conditions. The dependency order is ND-0, ND-2, ND-1, ND-3, ND-4: observation types/state precede producers.

Source inspection identified that the current stream barrier runs after the provider request starts, MCP IsError becomes plain text at ToolWorld, strict tool.finished schemas need coordinated changes, and the hard stop currently omits the sixth result event. ND-D1 addresses each explicitly. It chooses transient Eino WrapModel injection and typed internal MCP errors without new SDK fields.

This is documentation only. No product code, schema, dependency, plugin, public API or existing runtime behavior changed. No automatic correction feature has shipped. The preliminary issue comment remains historical; NUDGE-DESIGN.md is the detailed contract and docs/plans/nudge/README.md owns readiness.
