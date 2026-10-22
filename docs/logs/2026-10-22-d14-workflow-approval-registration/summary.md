# Include native workflow approvals in suspension readiness

This correction starts at `86215cf20b65d8f45c3481c2bc965b091ea6590b` and
supersedes its unsupported native-orchestration exclusion described in
`../2026-10-22-d14-suspension-registration/summary.md`.

Independent review traced the actual native workflow route:
`executeNativeOrchestrationWorkflowWith` ->
`persistNativeOrchestrationApproval` -> `handleInterrupt`. It uses the same
durable request publication followed by `Service.pending` installation as
ordinary native tool approvals. There is no separate lock or readiness owner
that closes the gap. The prior exclusion allowed a published workflow approval
to settle successfully before its continuation was registered.

The existing pre-CAS readiness check now includes native workflow approvals.
Direct shell approvals retain their separate readiness owner. Plan decisions
retain their separate Work mutation path. Deadline checks, child behavior,
duplicate decisions, restart behavior and intentional system auto-approval
remain unchanged from the preceding fix.

`handleInterrupt` now returns an explicit `approvalRegistered` result for its
existing approval handoff. All failure/cancellation paths return false; the
successful approval registration returns true. Question and Plan routes return
false because they have separate interaction owners; the ordinary drive ignores
the result. The workflow caller uses this direct outcome instead of rereading
the mutable pending map after the call. A responder can legitimately consume
that map entry immediately after registration. That consumption-sensitive read
was identified in source review, not independently reproduced as another P1;
the change reports the existing handoff without new state or synchronization.

## Eino capability check and design

The actual pinned Eino `v0.9.21` already supplies `compose.NewWorkflow`,
`compose.WithCheckPointStore`, native `Runnable.Invoke` checkpoint suspension,
and `compose.ResumeWithData`. Their implementations were inspected in
`compose/workflow.go`, `compose/checkpoint.go`, and `compose/resume.go`.
Vivy's existing native workflow path already uses these APIs. The defect is
Service publication/registration ordering, not a missing framework capability.
The correction reuses the existing pending-registration wait and native
checkpoint/resume path; no runtime, barrier, registry, recovery mechanism or
production test hook is added.

Returning the current handoff result is the smallest owner-local choice.
Inferring success from a later map lookup confuses registration with subsequent
consumption; registering before publication would recreate the known
cancel-on-publish lock hazard. The actual workflow regressions exercise approval,
denial, cancellation and failed handoff with real Eino checkpoint bytes.

This is a scoped correction, not D14 or release acceptance. No tenant Journal,
schema, dependency pin, external Module, Studio code or production data changed.
