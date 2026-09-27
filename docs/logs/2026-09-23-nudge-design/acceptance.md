# Planning acceptance

1. Open docs/architecture/NUDGE-DESIGN.md and find N1–N5, Eino capability evidence, typed failure allowlist, per-Run lifecycle, concurrency/durability, bounded reminder policy and compatibility.
2. Open docs/plans/nudge/README.md and follow its immediate dependencies in the declared order.
3. Each of ND-0 through ND-4 supplies file scope, consumed/produced contracts, ordered checkbox tasks, concrete behavioral cases, verification commands and escalation conditions.
4. The package does not mark source inspection or absent runtime checks as passing evidence. ND-0 is the first execution gate; downstream Stories stay blocked on accepted predecessor outputs.
5. Implementation requires the user's next instruction and a repository-supported toolchain. No human input is required to invent the architecture inside downstream Stories; failed integration assumptions return to this specification for revision.
