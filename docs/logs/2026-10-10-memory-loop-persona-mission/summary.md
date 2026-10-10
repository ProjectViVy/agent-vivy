# Per-run Mission pin and persona session proof

Workflow nodes now check zero as a real Mission pin, bind the lazy domain to each run's persisted binding, and recheck Mission before each effect/recovery Lookup. Admission checks the newly resolved binding rather than stale App construction pins. The selected native owner provides the atomic check/commit gate; no second engine or authority is created.

Actual request-derived identity proposals remain pending until human reject/accept. Both decisions are verified across a distinct OS process: rejected authority stays unchanged; accepted new sessions use the updated identity while reopened old Frozen Core remains identical. Actual provider system messages use each session's own snapshot.

Actual Mission edits at the held reflect response boundary cover unassigned→assigned and assigned→new revision, fencing later effects at watermark0 with original run identity. No user data/profile is used. ACTMEM Pulse/Recap, ordinary recall and whole-phase source sealing remain pending.
