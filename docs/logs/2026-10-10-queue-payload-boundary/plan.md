# Queue payload boundary plan

Scope: fix the acknowledged-steer payload boundary only. Enqueue currently reserves a full restore marker with steer track; fallback admission changes both encoded track fields to follow_up and adds eight bytes. Reserve the future follow_up marker before acknowledgement while keeping the actual queued track and persisted queued marker unchanged.

1. Establish isolated baseline from 245bbc15 and reproduce accepted steer whose future restoration marker exceeds the configured journal ceiling.
2. Add regressions for oversized rejection with unchanged live/durable queue and for a bounded steer admitted after missing-checkpoint fallback.
3. Change only enqueue's restoration-size reservation to a cloned follow_up DTO; retain all other queue behavior.
4. Run focused queue tests and race checks, record English verification/acceptance, and commit explicit paths. Root integration owns sourcehash regeneration and the complete required CI/package gates.

Eino capability check: the existing fallback uses native adk.WithCancel, ResumeWithParams and ChatModelAgentResumeData.HistoryModifier when checkpoints are available; missing checkpoints settle the existing run and admit through Service.RunWithOptions. This change affects only the Vivy journal's encoded DTO ceiling and needs no new Eino implementation, runtime driver, scheduler, schema, or storage namespace.
