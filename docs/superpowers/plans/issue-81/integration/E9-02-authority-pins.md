# E9-02 Single Cognitive Policy and Mission Pin Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans after implementation authorization. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Recover #32 C3/C4 without duplicate policy authority or a non-atomic Mission effect check.

**Architecture:** Runtime admission supplies its durable policy snapshot to the existing bundle resolver. The persisted current Mission pin must be checked by the existing Garden owner atomically with effect application and synchronized with human Mission writes.

**Tech Stack:** Existing Go cognitive contract/runtime/App, selected Laputa Garden agentapi.

**Spec:** Old #32 P2.3, `35e559f6` VIVY lineage within `6045c0a...`; published Laputa Mission-gate source `30fa208e3cded4af43f8cb226d80b225b1910854`; architecture.md D-POLICY. Predecessor and precise dependency gate: package.md / dependencies.md.

## Global Constraints

No Laputa edits in this session/package; no blind old dependency repin. Preserve E5 peer repair and E6 Skill contract in the eventual combined source closure. No mutable policy synchronization checklist: remove the second owner. No new Policy store or generic lock framework.

## Review Focus

Live Mission changes after app construction; Mission changes between resolve and apply; policy write/admission race; restart with previously nonzero policy; missing atomic owner API. Every case must preserve the accepted immutable run pin or fail closed before an effect.

## Files and interfaces

Modify existing `internal/runtime/{cognitive_service,cognitive_binding}.go`, `internal/cognitivecontract/ports.go`, `internal/modules/diva-cognitive/{factory,dispatch}.go`, `internal/app/app.go`; update their fixture implementations/tests including `assembly_cognitive_test.go`. Read, never edit, the selected Laputa owner API and original Mission gate tests.

Consume E9-01 immutable intent and durable policy snapshot. Reuse old P2.3 signatures:

```go
// cognitivecontract.Bundle
ResolveBinding(ctx context.Context, policy laputaevolution.TriggerPolicy) (laputaevolution.RunBinding, error)
// runtime.CognitiveBinding field
Resolve func(context.Context, laputaevolution.TriggerPolicy) (laputaevolution.RunBinding, error)
```

Resolver hashes exactly its policy argument. `Policy()` is immutable construction seed only; delete mutable `setPolicy`/post-write mirror. Existing Bundle.BoundDomain supplies Mission-pinned owner domain. Scope identity stays composition-bound while mutable Mission revision is per admission.

## Tasks

### 1. Clear the exact dependency gate

- [ ] Read published `feat/issue32-mission-pinned-effect` at full `30fa208e3cded4af43f8cb226d80b225b1910854` and old VIVY lock. Verify the owner lifecycle read lock plus shared Mission read/write gate covers revision validation **through** Domain.Apply, and direct human initialization/save/review paths take the paired write gate. Existing `garden/agentapi/embedded_domain_test.go` contains `TestMissionGuardSerializesOwnerWriteWithEffectApply` and `TestMissionGuardRejectsStaleRevisionBeforeApply`; use them as API evidence; no application-only precheck substitute.
- [ ] Parent/dependency owner supplies one accepted published Laputa candidate combining required Mission gate with the selected #9/#10 contracts and any peer patch. Update all three versions/source lock only through the single pin owner. Until its exact combined SHA and compatibility proof exist, this Story's integration is blocked at D-LAPUTA-MISSION; no other Story is globally blocked.
- [ ] Required dependency command, to be run by its owner in the selected Garden module: `go test ./agentapi -run 'Mission' -count=1 -v`; require actual cases for concurrent human write vs effect. Author claims or branch existence alone do not pass this gate.

### 2. Pin current durable authority

- [ ] Recover old tests `TestCognitiveCurrentMissionBindingAdmitted`, `TestCognitiveMissionChangesAfterResolutionFailClosed`, `TestCognitivePolicyPinSurvivesReopen`, `TestCognitiveConcurrentPolicyUpdatePinsAdmissionSnapshot`, `TestResolveBindingPinsSuppliedPolicy`. Current Mission revision 2 after composition at 1 must admit with pin 2; mutation after resolve must reject before apply. Reopen must preserve the policy digest derived from admitted durable bytes.
- [ ] Run focused runtime/module tests before correction and record red triggers. Pass the exact loaded policy through Resolve; validate current Mission against the resolved binding, not `b.Binding`; preserve static subject/workspace/destination checks. Persist that binding in E9-01 input.
- [ ] Remove bundle mutable policy and dispatch `setPolicy`, update App/fakes and typed contract call sites. Reads and effects retain current Mission checks; adapters without atomic Mission application fail closed. Do not expose Eino through this seam.
- [ ] Run `go test ./internal/runtime ./internal/modules/diva-cognitive ./internal/app -count=1`, focused concurrency tests under race, selected dependency owner proof, then `just ci`. Expected: source, restart and owner-gate cases pass on the same exact dependency; no result inferred from a different lock.

### 3. Review and handoff

- [ ] Independent reviewer traces policy ownership and Mission locking through the actual selected dependency, not only mocked VIVY interfaces. Record one-home policy proof and exact dependency SHA in future iteration evidence. Commit explicit paths under human attribution.

**Acceptance:** Each run's policy digest derives from its durable admission snapshot; accepted current Mission changes admit; later changes cannot race effect application. No second policy owner remains.

**Stop conditions:** No accepted combined dependency SHA; gate not atomic across actual human writes; a required public compatibility change lacks approval. Preserve the exact blocker and other Ready Stories; do not implement an alternate VIVY lock around a separate owner.
