# PG-2 Task 1: Soft Plan and legacy split

Soft Plan now draws its advisory text from the embedded `internal/runtime/prompts/plan.md` asset, shared by per-run composition and the existing Eino work-plan middleware. Historical shell approval recovery now reapplies the `RunModePlan` hard restriction even if a legacy payload contains a contradictory policy profile. Tests cover preserved writable and read-only policies, legacy tool and shell recovery, and guidance composition. The internal-source conformance digest was refreshed for the changed runtime tree.

Scope was limited to Plan guidance, shell recovery policy interpretation, scenario coverage, and its derived conformance identity. No dependency, schema, Port, policy source, or execution loop was added. PG-0/PG-1 live PostgreSQL acceptance and downstream product evidence remain outside this task.
