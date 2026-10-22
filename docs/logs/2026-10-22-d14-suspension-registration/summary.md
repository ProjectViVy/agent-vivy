# Preserve human responses across native suspension registration

At baseline `9214f200ec0e5df7b4ffd5231eb4cdd56b27de06`, a root Run could publish
its durable approval/question request before installing its private native
suspension entry. A response arriving in that gap could consume the durable
first-writer slot, return success, and then leave the subsequently registered
Run parked indefinitely without a continuation or terminal event.

Native tool approvals and questions now reuse the existing bounded
`waitPendingRegistration` before their irreversible decision/answer CAS. An
unreachable suspension fails closed while its row remains pending. Once the
registration exists, the existing native checkpoint, prepared tool operation,
engine, budget ledger, context and resume owner continue unchanged. Duplicate
responses still lose the existing durable first-writer CAS.

The new wait can cross the human deadline. Both public response paths recheck
expiry before their CAS. The existing timed smart-mode auto-approval deliberately
settles after the deadline; its private `settleApproval` call explicitly permits
that behavior. This permission is an internal call argument, not an actor-name
exception or a public option.

Shell readiness and native orchestration retain their existing resume owners;
the new root approval guard excludes them. Existing child approval readiness
and refusal semantics remain. Plan decisions use their separate Work mutation
path and are untouched. There is no new global barrier, persistence field,
runtime, synchronization service or polling helper.

## Design and Eino capability check

The smallest viable fix is to reuse readiness at its existing Service owner
before settlement. Registering earlier would require broader failure cleanup
and would make the existing synchronous cancel-on-publish sink attempt to
reacquire `projectionMu` while publication already holds it. The retained
two-second bounded poll already supports child decisions and cancellation;
extending its native response consumers closes the confirmed gap without
reordering Journal publication or introducing another source of truth.

Inspected the actual pinned Eino `v0.9.21` `adk/runner.go`:
`TypedRunnerConfig.CheckPointStore`, `NewTypedRunner`, `TypedRunner.Run` and
`TypedRunner.ResumeWithParams`. Vivy's `Engine.RunHistory` and `Engine.Resume`
already delegate to those native APIs and its existing versioned checkpoint
adapter. Eino owns suspension/continuation. The defect is entirely in Vivy's
Journal visibility followed by private pending registration; no missing Eino
capability or custom LLM mechanism is introduced.

SQLite and PostgreSQL approval/question settlement queries were inspected.
They condition on pending status but do not themselves enforce expiry, which
justifies the post-readiness deadline checks at the existing Service owner.
Storage schemas and CAS queries remain unchanged.

## Delivery boundary

This patch changes only `internal/runtime/service.go`, its focused native
regressions and this iteration record. It does not certify D14, migration or
release acceptance. Integrated candidate Linux gates, browser/native product
smoke and full-product CI remain with the parent acceptance lane. No production
Journal or tenant workspace was accessed.
