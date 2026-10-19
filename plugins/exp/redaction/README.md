# Explicit redaction experiment

`vivy/exp-redaction` is an opt-in T2 source Module pinned by its Descriptor.
It provides only `std/tool@v1` (`vivy.exp.redact_text`); ToolHost is the sole
consumer and Policy/Recipe own authority. It requests no filesystem, network,
process, or credential Grants. The tool accepts explicit text and returns the
old pattern transformation. It never installs a logger handler or intercepts
other tools, model requests, context, persistence, or errors.

Lifecycle is generation-scoped with no resources to start/close. Calls honor
caller cancellation and ToolHost deadlines/budgets; invalid arguments return
errors. Definition, Provider, lifecycle, and algorithm tests are local. Verify
with `go run ./sdk verify plugins/exp/redaction` from the repository root.
No checked-in Recipe selects it. Source verification is not release conformance
attestation; packing it requires an explicitly authored experimental Recipe.
