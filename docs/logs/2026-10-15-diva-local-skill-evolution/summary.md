# DIVA local Skill proposal bridge

## Delivered

- Added a typed `skill_proposal` candidate to Laputa's DIVA reflection contract, separate from EvoMap `capability_proposal`.
- DIVA selects an existing local Skill before proposing an update, reads only that Skill's `SKILL.md`, and carries the selected content hash into an update proposal. It proposes creation only when no matching Skill is selected and requires evidence references from the admitted reflection batch.
- VIVY stores the proposal through the existing Skill revision authority and creates a durable `skill_revision` Review Center item. The revision payload retains DIVA origin, operation ID, rationale, approval ID, and source evidence.
- Human approval calls the existing revision apply path. Denial, expiration, cancellation, staleness, and apply failure are represented in the durable revision lifecycle. Approval timeout is explicitly never auto-approved for this review kind.
- Restart reconciliation resumes approved-but-unapplied DIVA revisions and leaves pending reviews in the durable Review Center.

## Scope

This is the first local Skill bridge only. EvoMap is unchanged; no general-purpose evolution framework, new Skill store, or parallel approval center was added.
