# Deferred turn-limit experiment

The former core `runtime.max_tool_turns` policy is preserved as independent
source: the default is eight model-generation cycles, explicit non-negative
values are retained, and child engines tighten zero or larger values to eight.
Run `go test ./...` in this directory to verify the policy.

Status: **DEFERRED-INDEFINITE**. This is not a runnable Vivy Module. It has no
Descriptor, Provider, Port binding, registration, Grants, lifecycle, or runtime
import. No Recipe or Generation selects it. The existing public observer and
pre-tool Ports cannot apply a Run-terminal model-iteration limit; a future
intervention contract requires a separate design and its full conformance gates.

The old enforcement mechanism was the pinned Eino v0.9.13
`adk.ChatModelAgentConfig.MaxIterations`, not a custom agent loop or counter.
The pure policy returns that configuration value; it cannot enforce a limit by
itself. Zero retains Eino's default of twenty, not unlimited iterations. Eino
types and internal runtime authority are deliberately absent from this source.

Production no longer exposes this policy or its terminal-error special cases.
Its thin Eino adapter uses the platform integer maximum because this pinned
version interprets zero and negative limits as twenty. Shared Run/descendant
resource accounting, cancellation, context/output bounds, Policy and Journal
durability remain authoritative. This does not promise unlimited task work.
