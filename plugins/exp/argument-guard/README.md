# Explicit argument guard experiment

`vivy/exp-argument-guard` is an opt-in T2 source Module pinned by its Descriptor.
It provides `std/middleware/pre-tool@v1` (`vivy.exp.argument_guard`) only;
ToolHost is the sole consumer and Policy/Recipe own authority. No Grants are
requested. The old field-name/substring algorithm returns Pass/Deny, never
executes a tool, mutates Run state, or replaces a protected tool identity.

Lifecycle is generation-scoped with no resources. Calls honor cancellation
and ToolHost deadlines; a denial returns a valid bounded Host decision. Local
tests preserve the old heuristic behavior. Verify with
`go run ./sdk verify plugins/exp/argument-guard` from the repository root.
No checked-in Recipe selects it. Source verification is not release conformance
attestation; packing it requires an explicitly authored experimental Recipe.
