# Verification

- Canonical-content observation: process regression RED (empty canonical observation) -> GREEN after reading canonical content; composition and both child phases pass.
- Legacy receipt: observer replay RED (event_conflict) -> GREEN by joining the existing trusted event receipt.
- Garden receipt authorization: RED (lookup absent) -> GREEN; alternate session/agent cannot read the receipt and unauthenticated caller is rejected. Actual Garden SQLite acceptance is read, not synthesized.
- Actual factory/Journal/Garden close/reopen regression verifies old acceptance rejoined without rewriting; direct changed payload still conflicts.
- Codeface linked-parent fixture: CI home-write failure -> focused suite GREEN using explicit temporary config.
- NotifyInput: observed foreground_busy race; test now waits for the automatic workflow and checks its completed actual input window. Ten repetitions are recorded separately.
- Final affected suite and required `just ci` are collected in the DIVA handoff; do not infer full CI from targeted green results.

Test-only helper initially caused an import cycle; moved to the owning module's tests before successful compilation. No runtime dependency around package boundaries was added.
