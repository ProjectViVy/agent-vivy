# Restoring DIVA → VIVY local Skill evolution: an engineering guide

Research date: 2026-10-11 UTC. Status: source-backed research and proposed future design. This delivery changes documentation only. It does not restore a runtime feature, mutate an installed Skill, migrate a store, certify runtime behavior, or authorize automatic adoption.

## 1. Decision and boundaries

Restore a narrow local learning path: DIVA turns authorized experience into a bounded Skill revision proposal; VIVY validates it, stores its provenance, presents it in the existing Review Center, and applies the exact reviewed revision through the existing Skill authority. Learning about the same capability should normally accumulate revisions of one stable Skill. EvoMap publication and generic strategy composition are independent, deferred concerns.

The minimal coherent choice is to extend the delivered DIVA typed candidate/effect contract with a local `skill_proposal`, and bind a VIVY adapter to the existing `SkillOperations` and `SkillRevisionStore`. Preserve the current DIVA execution/capture/watermark/receipt path and VIVY ToolHost/Service/Journal/policy/review path. A separate learning agent runtime, SkillHome database, approval service, or framework of strategy stages would increase maintenance and split authority without solving this particular gap. The tradeoff is a small cross-repository contract change and a few necessary hardening changes in existing owners.

Non-negotiable requirements for this proposal:

- Automatic proposal generation is allowed only within the configured evidence scope and budgets. It does not imply approval or application permission. New evolution revisions require an explicit human decision, including under permissive foreground presets or timeout settings.
- Reuse VIVY identity, revisions, Review Center, precondition hashes, run/operation receipts, refresh, and rollback seams. Keep Skill files in the existing trusted root.
- Keep local SOP learning available without EvoMap configuration, network access, or publishing. Preserve EvoMap-specific `capability_proposal` semantics rather than repurposing its payload as a local Skill write.
- Reuse an existing Skill for the same durable capability. A new incident, source window, or reflection run is not a new capability identity.
- Persist DIVA origin and evidence separately from editable Markdown. A frontmatter flag alone cannot be the provenance authority.
- Do not expand this delivery into VIVY [#7, Evolution strategy composition](https://github.com/ProjectViVy/agent-vivy/issues/7). The issue was read on the research date; it separates recommendation from permission, preserves current DIVA consumers, and leaves general typed-stage composition unscheduled.

Laputa's checked-in `AGENTS.md` and `laputa/AGENTS.md` still state that EvoMap owns all capability artifacts and that other modules do not create/install Skills. The user's explicit direction selects VIVY's local Skill authority for this design and takes precedence. Future implementation must deliberately reconcile that architectural statement for **local SOP proposals only**, while retaining EvoMap ownership of its own artifact/publication lifecycle. This guide is not a silent change to those normative files.

## 2. Revisions and evidence discipline

Remote refs were refreshed using Git on 2026-10-11. The ProjectViVy checkouts were clean before creating the research branch. Hermes was cloned from its remote default branch; the earlier supplied revision is still the inspected head. Historical Rust DIVA was fetched as a specific object and read with `git show`, without switching the active Go checkout.

| Evidence label | Repository / inspected revision | Role |
| --- | --- | --- |
| H | `NousResearch/hermes-agent` main `c0ef5af5205dbdc25e9e44acd66873b4bb3503c9` | Actual background learning, discovery, mutation, optional approval, ledger, usage, curator |
| V | `ProjectViVy/agent-vivy` main `ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666` | Current local Skill authority, review/runtime, storage, UI, DIVA host integration |
| D | `ProjectViVy/agent-diva` main `0a64d1ce152440f1f63d4224a155c4616d2aff5b` | Current Go composition and bound domain |
| L | `ProjectViVy/laputa` main `4b2bec2cc2ab3374b7ec1ab8d448e612c1e4db24` | Current DIVA strategy/schema, evidence, effect and receipt contracts |
| R | `ProjectViVy/agent-diva` historical `c565bb245cc920258d7f8c7fcd9544fbba545af7` | Concrete retired Rust SkillHome/AutoDream implementation; reference, not migration source |
| E | `cloudwego/eino` tag `v0.9.13`, commit `c5e6aef927cca02bea934541f8dff2ea711b2ca7` | Pinned VIVY framework API check |

The repository pins Eino v0.9.13 and EinoExt OpenAI v0.1.13 in [go.mod](https://github.com/ProjectViVy/agent-vivy/blob/ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666/go.mod#L15-L21). Research checked the Eino tag's source in a temporary read-only reference checkout, not by executing framework or repository code.

Evidence classes used below:

- **Source-confirmed:** a traced branch, data type, or call path at a pinned revision. This establishes implementation shape, not an observed production outcome.
- **Static risk:** a failure/concurrency consequence inferred from a traced path. No failure is reported as reproduced unless explicitly stated.
- **Test source:** an inspected existing test shows the author's intended contract. It was not executed in this session and does not independently certify runtime behavior.
- **Proposal:** future behavior, contract, state, UI or acceptance test. Nothing in these sections is delivered runtime code.

No application, imported repository module, repository script, model call, database, tenant journal, test suite, migration, or live Skill operation was executed. Focused runtime reproduction remains future work on temporary roots and isolated test databases. Git/source/document checks are the evidence for this research artifact only.

## 3. Current VIVY: existing authority and exact gaps

### 3.1 Existing data and tool surface

[SkillRevision](https://github.com/ProjectViVy/agent-vivy/blob/ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666/internal/domain/skill.go#L3-L31) already persists `ID`, `RunID`, `SkillName`, `Action`, `TargetPath`, opaque mutation `Payload`, `BeforePayload`, `BaseHash`, `ContentHash`, `Preview`, `WarningsJSON`, `Status`, `CreatedAt`, and `AppliedAt`. Statuses are `pending`, `applied`, `rejected`, and `failed`. Storage owns persistence; the Skill backend owns payload interpretation.

[SkillOperations and request/result types](https://github.com/ProjectViVy/agent-vivy/blob/ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666/internal/tools/skills.go#L51-L86) already provide list/view, prepare, apply, manage, and an enabled toggle. Actions exposed by `skill_manage` are create, edit, patch, delete, write_file, remove_file, and rollback. The model-facing description says it stages human-reviewed changes, but the underlying `ManageSkill` method calls prepare then apply directly. Governance belongs to the runtime wrapper and its policy, not to this helper by itself.

[PrepareSkillProposal](https://github.com/ProjectViVy/agent-vivy/blob/ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666/internal/runtime/skills_backend.go#L213-L268) validates name and ownership, reads the target, computes before/base/after/content hash, bounds preview and content, stores the pending revision, and returns a `ToolProposal` containing only the `revision_id` reference plus review display and precondition data. This is exactly the seam local evolution should reuse.

[SQLite revision store](https://github.com/ProjectViVy/agent-vivy/blob/ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666/internal/storage/sqlite/skill_revisions.go#L15-L84) and its [PostgreSQL peer](https://github.com/ProjectViVy/agent-vivy/blob/ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666/internal/storage/postgres/skill_revisions.go#L15-L84) persist the same logical record. Terminal status updates are conditional on `pending`; zero affected rows is a version conflict. The [store interface](https://github.com/ProjectViVy/agent-vivy/blob/ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666/internal/storage/contracts.go#L400-L408) offers get and pending-list, not a complete per-Skill history query or operation-key dedup index. Existing migrations already own the revision table; any future columns/indexes belong in paired Core Storage migrations, not DIVA initialization code.

### 3.2 Current call graph

```text
Model skill_manage invocation
  → runtime toolAdapter: final arguments, policy, hooks, middleware, operation admission
  → Eino tool interrupt when policy asks
  → Service pause path: checkpoint read → Engine.PrepareProposal
      → skillManageTool.PrepareProposal → EinoSkillBackend.PrepareSkillProposal
      → persist SkillRevision(pending); return revision_id
  → bind reviewed arguments hash → CreateApproval(ProposalData with revision_id)
  → durable tool.approval_required event → existing Review Center
  → approved decision → checkpoint resume with proposal data → verified approved dispatch
  → skillManageTool.InvokableRun → ApplySkillRevision(revision_id)
  (denied decision → refusal; no Skill backend invocation)
  → reload target → compare BaseHash → filesystem mutation → status applied
```

Pinned nodes: [Engine.PrepareProposal](https://github.com/ProjectViVy/agent-vivy/blob/ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666/internal/runtime/engine.go#L513-L526), [Service proposal/approval persistence](https://github.com/ProjectViVy/agent-vivy/blob/ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666/internal/runtime/service.go#L3886-L3969), [tool application reference](https://github.com/ProjectViVy/agent-vivy/blob/ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666/internal/tools/skills.go#L228-L254), and [ApplySkillRevision](https://github.com/ProjectViVy/agent-vivy/blob/ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666/internal/runtime/skills_backend.go#L271-L328).

This call graph describes the asking-policy path. [ToolAdapter](https://github.com/ProjectViVy/agent-vivy/blob/ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666/internal/runtime/tooladapter.go#L739-L802) can also auto-approve an invocation under existing policy. `forceHumanApproval` currently names create_goal, not skill_manage. [Timed approval](https://github.com/ProjectViVy/agent-vivy/blob/ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666/internal/runtime/service.go#L668-L677) recognizes workspace-write + ask, and [SweepExpired](https://github.com/ProjectViVy/agent-vivy/blob/ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666/internal/runtime/service.go#L1773-L1798) approves those rows when the settle window is enabled. A future evolution proposal must carry a trusted explicit-review requirement into this existing approval machinery. Neither setting a prompt nor calling `ManageSkill` from a strategy is an adequate human boundary.

### 3.3 Discovery, ownership and parsing

The [root scan](https://github.com/ProjectViVy/agent-vivy/blob/ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666/internal/runtime/skills_backend.go#L108-L132) puts closer-first project overlays ahead of the writable user root; a session's selected workspace can change those overlays. `loadSkills` deduplicates case-insensitive names by first visible root; `locateSkill` resolves the first matching directory. The loader requires frontmatter name to match directory identity. Project roots are read-only; user/marketplace installation root is writable. A revision's name/path/base bytes alone do not bind the exact authority root across workspace selection changes. Future submissions must retain the host-resolved target origin/root and revalidate it at apply.

Bounds already exist: 512 KiB file/content, 128 supporting files in listing, 512-byte path and 32 KiB preview. Supported directories are references, templates, scripts and assets. [Path/content helpers](https://github.com/ProjectViVy/agent-vivy/blob/ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666/internal/runtime/skills_backend.go#L629-L775) reject escape paths and invalid patch matches; [readTrustedFile](https://github.com/ProjectViVy/agent-vivy/blob/ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666/internal/runtime/skills_backend.go#L867-L878) rejects symlink/nonregular/oversized targets; [ViewSkill](https://github.com/ProjectViVy/agent-vivy/blob/ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666/internal/runtime/skills_backend.go#L173-L196) additionally rejects binary supporting-file views; loaded Skills are untrusted data. Validation is incomplete for the proposed learning contract: create parses Markdown, while edit/patch and a write_file targeting SKILL.md do not apply equivalent frontmatter/name checks. Mutation-side UTF-8/binary validation must also be explicit; a safe supporting-file view does not validate every proposed write. No general identical-content short circuit exists for edit/write_file. `patch` forbids identical old/new strings; that is not a complete no-op policy.

Do not treat the substring warning scanner as proof of safety or usefulness. It flags a few instruction/command/credential-like markers and returns warnings; review and policy still own authorization.

### 3.4 Provenance cannot live only in YAML

[SetSkillEnabled](https://github.com/ProjectViVy/agent-vivy/blob/ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666/internal/runtime/skills_backend.go#L535-L574) reads a hash, parses into the local typed frontmatter struct, changes enabled and re-renders. [The frontmatter struct/render helpers](https://github.com/ProjectViVy/agent-vivy/blob/ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666/internal/runtime/skills_backend.go#L806-L864) contain no evolution provenance and explicitly drop unknown keys. Parsing also trims the body, so the nearby comment promising byte-for-byte preservation is stronger than the actual helper behavior. This is source-confirmed transformation behavior; it was not executed here.

Therefore `evolution_managed: true` or a custom DIVA YAML object can disappear on enable/disable. Even a known key can be edited by the user and cannot independently attest the originating run. Prefer typed immutable revision provenance in the existing revision authority, and a derived UI projection. Preserve the full historical origin when later manual edits change the current text. An optional export manifest is a serialization of that authority, not another approval store.

The existing [`.vivy-skill.json` manifest](https://github.com/ProjectViVy/agent-vivy/blob/ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666/internal/runtime/marketplace.go#L185-L215) already records marketplace origin. Keep marketplace provenance intact. Do not overwrite it with DIVA metadata or pretend marketplace installation counts are improvement evidence.

### 3.5 Failure and concurrency findings to verify before implementation

| Finding | Pinned evidence / current behavior | Classification / required future check |
| --- | --- | --- |
| Pending revision can outlive denied/expired/cancelled approval | Revision is created before approval. Denial returns before Skill dispatch; global call search finds status updates only in apply paths. Approval cancellation/expiry update approval/run/journal rather than revision. | Static finalization gap; reproduce each terminal review path on temp DB/root and check pending rows. |
| Prepare succeeds, approval persistence fails | Revision and approval creation are separate calls; pending row exists before CreateApproval. | Static orphan window; fault-inject approval-store failure and restart reconciliation. |
| Hash compare is not an indivisible filesystem CAS | Apply reads hash then writes; backend has no target mutex. Enabled toggle/marketplace/manual writers can interleave. Status CAS occurs after filesystem effect. | Static concurrency risk; synchronize all host writers and reproduce competing proposals/toggle/import. |
| Filesystem effect succeeds, status write fails | Apply mutation precedes SetSkillRevisionStatus; status failure returns an error after bytes changed. Failed-status updates on file failure discard their own error. | Static cross-resource durability window; crash/fault/replay tests must distinguish committed, uncommitted and unknown. |
| Changed-base proposal remains pending | Apply returns on base mismatch before terminal revision update. | Source-confirmed branch, unreproduced runtime outcome; require stale/conflict finalization and explicit re-review. |
| Cross-run rollback blocked | rollbackMutationState rejects nonempty source.RunID different from nonempty caller runID. | Source-confirmed restriction; ordinary later-run rollback needs a scoped authority decision, not removal of all binding. |
| Rollback is not current-head equality with source revision | Non-delete rollback captures current bytes as its new base and restores source.BeforePayload; intervening later edits are not automatically excluded. | Static semantic concern; UI must show exactly what would be replaced and re-review against current head. |
| Whole-directory delete base covers SKILL.md | Supporting files are moved with directory while base hash checks the main file. | Static review coverage gap; either exclude destructive autonomous proposals or use package identity for these operations. |
| Create rollback removes only created SKILL.md | applyRollbackMutation uses os.Remove(target), leaving the directory/supporting files. | Source-confirmed behavior; define create rollback after later support additions before promising package rollback. |

[Rollback implementation](https://github.com/ProjectViVy/agent-vivy/blob/ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666/internal/runtime/skills_backend.go#L935-L998) contains the binding and restore branches. These findings are not reproduced incidents and should not be phrased as observed data loss. They are release-blocking hypotheses/contracts for a future evolution implementation because it increases background mutation frequency.

## 4. Framework capability check

VIVY already uses [Eino Skill Backend List/Get](https://github.com/cloudwego/eino/blob/c5e6aef927cca02bea934541f8dff2ea711b2ca7/adk/middlewares/skill/skill.go#L48-L69), [NewMiddleware](https://github.com/cloudwego/eino/blob/c5e6aef927cca02bea934541f8dff2ea711b2ca7/adk/middlewares/skill/skill.go#L240-L267), and [Runner/checkpoint resume](https://github.com/cloudwego/eino/blob/c5e6aef927cca02bea934541f8dff2ea711b2ca7/adk/runner.go#L55-L149). The backend interface supplies discovery/loading, not Skill revision persistence, human decisions, provenance or atomic application. Retain it for consumption; VIVY-owned persistence/policy is a concrete product gap outside that read interface.

[VIVY engine wiring](https://github.com/ProjectViVy/agent-vivy/blob/ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666/internal/runtime/engine.go#L309-L315) uses the pinned middleware; [agent/runner construction](https://github.com/ProjectViVy/agent-vivy/blob/ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666/internal/runtime/engine.go#L403-L428) already supports sequential tools and checkpointing. DIVA inference [uses governed one-shot children without tool access](https://github.com/ProjectViVy/agent-vivy/blob/ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666/internal/runtime/cognitive_binding.go#L268-L350), deterministic child identities, run budgets and cancellation. Do not replace that with an unconstrained Hermes-style secondary agent.

The smallest candidate-discovery adaptation is a host-prepared bounded Skill context, supplied to that existing one-shot inference. If a second read is needed, use an explicitly bounded host step through the same reader. It does not require a new ReAct loop. Eino `compose`/ADK remains available for actual execution needs; a generic strategy compositor would duplicate already-delivered INOFY/DIVA machinery and belongs to #7's separate decision. Imports remain quarantined to runtime/provider. If upstream later offers an applicable revision workflow, adapt behind the existing SkillOperations seam; do not let framework APIs redefine VIVY approval authority.

## 5. Recommended local pipeline (proposal)

```text
Existing eligible committed evidence window and DIVA admission
  → discover bounded candidate Skill metadata
  → host-read selected Skill and relevant support files, capturing exact hashes
  → bounded inference: no_change / update_existing / create_new / ambiguous
  → strict local skill_proposal + inherited source references
  → trusted VIVY adapter resolves target, validates snapshots, provenance and scope
  → unchanged-content and pending/repeat-event dedup
  → persist SkillRevision + existing operation receipt and durable Review link
  → explicit human review through existing Review Center
  → recheck scope/root/policy + indivisible target CAS → apply reviewed bytes
  → finalize receipt/revision; refresh Skill discovery; project history
  → observe real subsequent task outcomes; optionally propose reviewed rollback
```

This pipeline has two separate completion points: durable **submission** completes the DIVA effect/window, while later human **application** completes the Skill revision. Do not hold the cognitive watermark hostage to a pending review, and do not mark submission as installed or effective. A rejected revision remains historical evidence, not unprocessed input for automatic regeneration.

### 5.1 Evidence and scheduling

Use existing cognitive trigger eligibility: enabled policy, new committed source watermark, cooldown/minimum activity, foreground/active-run constraints, bounded retry and unresolved-effect fences. Add no independent cron scheduler. Skill learning consumes the same snapshotted window; it may emit no proposal. Replay must rejoin the original operation and revision without spending another model call.

Capture concrete learning signals: a user correction, repeated tool failure followed by a verified recovery, a successful non-obvious sequence with observable result, outdated Skill instruction, a newly discovered precondition, or a missing validation step. Ordinary conversation, a syntactic tool success, and a recap of an unverified claim are insufficient by themselves. Record successes and negative evidence without converting them automatically into improvement claims.

Use orientation summaries to choose original evidence to retrieve. Include the relevant goal, environment/version, Skill actually viewed/used, attempted procedure, failing result, correction, final result, and uncertainty. Preserve source lineage through recap/reflection; derived text is not an independent corroborating event. Exclude report/reflection/strategy output from fresh-experience selection by default. Respect source deletion/tombstone/provider scope; never recreate intentionally forgotten evidence through replay.

### 5.2 Existing-Skill update policy

The host must resolve identity and the proposal must explain its decision. The following order is a decision policy with deterministic guardrails, not a claim that semantic identity can be proven by a hash:

1. Read Skills actually used in the source run. If the lesson concerns one of their procedures or preconditions, prefer a bounded revision to that Skill.
2. Discover existing class-level Skills with matching purpose, trigger and output, then read their bodies before proposing changes. Reuse one when it covers the same capability.
3. Put situational detail, error signatures or a narrow example in a relevant supporting file when the main SOP already covers the class. A support-file revision must still show the parent Skill and exact target. If it introduces a new reference, the single-file MVP can first review/apply the supporting file, then separately review a main-SOP pointer against the refreshed head; preserve linkage and never install a dangling pointer. An all-or-nothing package change would need a justified extension to the existing revision payload, not a new batch engine.
4. Create a new stable slug only when no suitable existing Skill covers the task class, or when incompatible environments/procedures genuinely require separate selectable capabilities. Explain the separation in review.
5. Return `ambiguous` or insufficient-evidence with the candidate identities when more than one plausible owner exists. Do not pick the first basename or create a duplicate to escape ambiguity. Human clarification can select the target through existing UI.

A Skill should describe when it applies, prerequisites, a reproducible ordered procedure, observable checks, failures/recovery, and limits. The proposal should change the smallest relevant section. Keep useful existing examples, warnings and support links; do not replace an entire SOP with the latest incident narrative.

Update identity is the existing scoped authority + canonical Skill name/path, not the incident ID. Stable slug should survive repeated learning. Hash identifies the bytes reviewed, not semantic capability. Two differently named Skills with related descriptions remain semantic candidates, not automatic duplicates; consolidate only after explicit target/compatibility review and preserved histories.

### 5.3 Target ownership and concurrent changes

Project overlays are read-only. Propose information or an explicitly reviewed local adaptation when policy allows; never rewrite project files or silently shadow their canonical identity. Writable user-owned Skills are technically editable, but background evolution must present a reviewable change and preserve the user's authorship/provenance. Marketplace Skills keep their upstream manifest and an explicit local modification state; an upstream upgrade must not silently discard DIVA changes. Local adaptation vs in-place revision is an open product decision, not a reason to create arbitrary shadow copies.

Candidate identity must include the host-resolved authority scope/root. The model receives an opaque target reference; it cannot name an absolute path/root or request another subject's Skill. Resolve/read again before admission and application. If a closer project overlay appears, or a session changes workspace, refuse/rebind through a new review rather than apply to whichever root now wins name lookup.

Capture the expected base **at the read that informed generation**. Existing PrepareSkillProposal computes its base at staging; that protects stage-to-apply changes but not stale model-context-to-stage changes. Extend the request adapter to compare the supplied read hash with the current target before persisting. Never repair a stale model proposal by replacing its expected hash with the newest hash. Creation uses an explicit expected-absence condition; permission/read failures are not absence.

### 5.4 Minimal contract fields

The following is a **proposed wire shape**, not a currently accepted schema or new mandatory storage table. Persist only fields that existing authorities cannot derive. Field names and limits must be frozen at implementation kickoff.

```json
{
  "kind": "skill_proposal",
  "skill_proposal": {
    "decision": "update_existing",
    "target_ref": "host-issued-local-skill-reference",
    "name": "stable-existing-slug",
    "mutation": {"action": "patch", "path": "SKILL.md", "old_string": "...", "new_string": "..."},
    "expected_base_hash": "sha256-of-exact-read-bytes",
    "expected_absent": false,
    "read_set": [{"path": "SKILL.md", "hash": "..."}],
    "rationale": "The verified recovery belongs to this existing SOP.",
    "sources": [{"source_id": "existing-source-id", "record_id": "existing-record-id", "revision": 1, "scope": {"subject_id": "bound-subject", "kind": "workspace", "workspace_id": "bound-workspace"}}],
    "outcome": {"observation": "What was actually checked", "effectiveness": "not_evaluated"}
  }
}
```

The `SourceRef` example deliberately uses the existing source_id/record_id/revision/scope identity rather than inventing a new event schema. Host-origin run/window/event bindings supplement it when Journal events are the underlying evidence.

| Field / fact | Existing identity to reuse | Proposed treatment / why needed |
| --- | --- | --- |
| Skill identity | `SkillName`, existing root, session workspace and subject binding | Host-resolved opaque `target_ref` binds scope/root/name. Add only the missing root/scope commitment; no new universal capability UUID. |
| Mutation | `SkillManageRequest` actions/path/content/patch fields | Reuse normalized request payload; initial evolution only create/edit/patch/write_file. Deletion, remove_file and rollback are explicit separate reviewed actions, not ordinary learner choices. |
| Base | `SkillRevision.BaseHash` | Validate the inference read hash before staging; persist base into revision. Use expected absence for create, not a magic `0` on all read failures. |
| Optional head revision | Existing applied `revision_id` when known | Advisory lineage link for VIVY-managed head; externally edited files may lack it, so the byte hash remains authoritative. Avoid gratuitous numeric versions for manually placed Skills. |
| Supporting dependencies | Parent/body/support read hashes | A small `read_set` is justified when a support-file lesson depends on another file; validate all semantically relevant inputs, not an unrelated whole repository. |
| Submission idempotency | DIVA/INOFY effect `OperationID` + `PayloadDigest` | Reuse these as request mapping keys. Same key+digest returns original revision/receipt; same key+different digest conflicts. Do not add a second random request ID. |
| Review identity | Existing approval ID + `revision_id` | Link them durably; human choice applies this immutable revision. Revised content requires a new revision/review. |
| Origin | Existing strategy/run binding/digest and parent/child run IDs | Typed trusted `origin = diva` plus pinned strategy/version, source window and originating run; stamped by host, not authored by LLM. |
| Evidence | Existing SourceRef, admitted Journal event identities | Store bounded citations and read revisions/digests. Preserve original source IDs, deletion state and scope; a display summary is optional, never evidence authority. |
| Reason/result | Rationale, observed procedure/result, limits | Bounded immutable proposal metadata. Distinguish expected benefit from observed outcome. `effectiveness = not_evaluated` until comparable checks exist. |
| Dedup | Target/base/content hash + operation receipt | Logical pending key over scope/target/base/after content; separately track consumed source IDs/revisions and unchanged no-op. Hash is a key, not a semantic classifier. |
| Decision finalization | Existing Skill revision status + approval decision | Extend existing record with terminal reason/decision linkage if needed; do not invent a generic approval state machine. |

The narrow runtime-owned provenance document can be added to the opaque revision payload if backward-compatible strict validation and querying remain clear; typed fields/metadata columns may be preferable for indexed projection. Choose one authority. Do not persist the same mutable provenance in Markdown, a DIVA JSON queue and a UI cache.

### 5.5 State and replay contract

Keep two vocabularies separate. DIVA's effect receipt `submitted` means the local authority durably owns a reviewable change; it does not mean SkillRevision `applied`. Current Skill statuses are pending/applied/rejected/failed; new explanatory states below are proposals to extend or project with terminal reasons.

| Event | Revision / review result (proposal) | DIVA window/receipt behavior |
| --- | --- | --- |
| No useful lesson or identical after bytes | No pending review; durable `no_change` reason | Existing operation receives no_change and window can settle. |
| Valid first submission | pending revision linked to pending approval | Receipt submitted with target_ref = revision_id; process window once. |
| Replay same operation+digest | Return identical existing revision/approval linkage | Return same recorded receipt; no new inference/write/review. |
| Same operation, different digest | Conflict; preserve original | Fail closed as replay mismatch; no replacement under original approval. |
| Identical pending target/base/after | Reuse existing pending review; record additional independent citations through immutable linked evidence | Current window settles as submitted/coalesced; do not change reviewed bytes. |
| Different useful evidence while target pending | Preserve pending proposal; bounded follow-up references/queue in existing operation/authority records | Process source window once. After pending review terminal, read current head and optionally propose a successor. |
| Human approve | approved review → CAS apply → applied revision | Preserve submission receipt; reflect later application through linked revision projection. |
| Human reject | rejected revision with reason and approval link | Do not re-propose same source/content automatically. New evidence or explicit user retry may justify new revision. |
| Human expiry/cancellation | terminal non-applied reason, no lingering pending row | Existing submitted receipt remains historically true; no implicit retry from completed source window. |
| Base/scope/policy changed | stale/conflict terminal reason; no file mutation | Re-read and regenerate only as a successor requiring new review, retaining lineage. |
| Validation/read/permission failure | No apparent successful submission; actionable error | Retry only under existing bounded/fenced semantics; distinguish absence from failure. |
| Unknown commit outcome | Record/recover operation, block new competing operation | Do not manufacture new IDs or infer no effect from an error. Resolve durable receipts first. |
| Later rollback recommendation | New revision reviewed against current head; original history preserved | Original applied revision remains true; rollback is a new authorized effect. |

One pending proposal per capability is a useful first-delivery backpressure rule, inherited conceptually from old SkillHome. It is not enough on its own: incident replay suppression, same-content dedup, independent evidence attachment and stale finalization have distinct meanings. Do not mutate a pending diff while a human is reviewing it. A successor link can reuse existing revision IDs; no new “evolution session” identity is required.

### 5.6 Durable application and CAS

First close the existing revision/approval finalization seams, then bind learning to them. A successful submission must have a durable revision, review/operation linkage and recoverable receipt. Where these records share a Core Storage backend, use its transaction boundary; where records belong to different existing owners, use existing receipt/recovery patterns and explicit unknown outcomes. Never pretend that a database transaction alone makes a filesystem effect atomic.

All host-owned writes to one Skill target must share a small keyed synchronization/CAS seam: agent revisions, control-plane edits/toggles, marketplace upgrade and rollback. Under the same target lock, re-resolve owner/root, verify authorization and expected hashes, derive/validate exact after bytes and perform the atomic filesystem replacement. The shared atomicWrite helper also has a destination-removal fallback after a failed rename, and does not sync the containing directory; see [filesystem_backend.go L821–852](https://github.com/ProjectViVy/agent-vivy/blob/ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666/internal/runtime/filesystem_backend.go#L821-L852). Fault-check both rename failures before promising atomic replacement across supported hosts. A lock is not protection against an external editor or crash; retain hash checks, receipts and recovery classification.

Record enough before/after digest/operation information to reconcile a crash between atomic rename and status commit. Recovery compares committed record and actual bytes: before bytes permit retry under the original operation; matching after bytes can finalize the original application when the operation receipt proves ownership; unrelated bytes mean conflict/unknown, never overwrite. An equal hash alone does not prove which actor wrote it. Choose the smallest extension of existing operation coordination that can prove that fact; do not add a general distributed transaction system.

Refresh discovery after an applied revision through existing backend/model admission rules. Pending/rejected/stale candidates must not be exposed as active SOPs. Already-running cached or frozen contexts should not be silently replaced; define the next safe Skill read/new-run refresh boundary and test it.

## 6. Review and Evolution UI (proposal)

Source-confirmed UI baseline: [ApprovalsView](https://github.com/ProjectViVy/agent-vivy/blob/ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666/ui/src/components/approvals/ApprovalsView.tsx#L12-L55) and shared [ReviewCard](https://github.com/ProjectViVy/agent-vivy/blob/ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666/ui/src/components/approvals/ReviewCard.tsx#L38-L103) use live state; the [Evolution hook](https://github.com/ProjectViVy/agent-vivy/blob/ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666/plugins/vivy-evolution/ui/vivy-evolution/src/useEvolution.ts#L1-L30) imports its skill/request/history operations from demo-api. The following UI requirements are proposals.

The existing Review Center remains the only decision engine for agent-origin local Skill mutations. Add Skill-specific projection details to its existing card and RPC types, not a competing accept/reject backend hidden inside the Evolution module.

A reviewer needs: canonical Skill name and origin/root, mutation/path, full accessible before/after or diff with truncation indication, expected base/head, DIVA originating strategy/run/window, cited source events/revisions, actual observed result and uncertainty, rationale, warning findings, pending dedup/follow-up evidence, status and terminal reason. Display “proposed”, “applied; effectiveness not evaluated”, and “evaluated” separately. Show explicit approve/reject actions and an accessible link to the underlying revision. Expired/stale/cancelled rows stay discoverable in history and do not look installed.

For the Evolution page, reuse its present skill/request/run/history interaction layout as a presentation reference. Replace demo/localStorage calls with existing backend-authoritative list/view/revision/review RPC projections. Do not promote `demo-api`'s SkillRequest or AutoDream state into a second source of truth. “DIVA evolution” filter derives from accepted/applied revisions with trusted DIVA origin, not from an editable YAML flag or every accepted manual request. Display that a user-origin revision later edited a DIVA-derived Skill without erasing its ancestry.

Skill detail should show current authoritative text/hash, pending revision, applied revision lineage, originating evidence, subsequent manual/marketplace changes, and reviewed rollback. The existing `skills/revisions/list` RPC is pending-only and omits base/content hashes, full before/after, approval link and provenance. Review projection does not yet expose the Skill revision/apply outcome; approval accepted is not application completed. A revision browser needs a backend per-target history query; current pending-list alone is insufficient. Reuse Review IDs/SkillRevision IDs in links and store wire types in `ui/src/lib/api.ts`, with authoritative view state in the existing store/host interface. Edit owning Module sources, never generated UI assembly copies.

Metadata persistence acceptance:

| Operation | Required behavior (proposal) |
| --- | --- |
| Edit/patch main SOP or support file | New revision preserves prior origin, creates new actor/change record and exact before/after; cannot erase ancestry by editing YAML. |
| Enable/disable | Remains the existing control-plane toggle; preserves revision provenance and records an observable control action. Unknown YAML keys should either be preserved by a reviewed parser change or explicitly unsupported, but are never the provenance authority. |
| Restart | Read pending and history from existing durable storage; preserve approval linkage/idempotency/fences and resume original identity. |
| Export | Export current package plus a bounded versioned provenance/history manifest derived from revision authority when requested. Markdown-only export must disclose omitted history; it cannot claim durable full provenance. |
| Import/copy | Preserve export origin as imported evidence, not a trusted local DIVA attestation or automatic permission. Allocate only identities required by the existing installation contract. |
| Marketplace upgrade | Preserve upstream provenance and DIVA/local lineage; surface conflict when local edits would be replaced. |
| Delete/rollback | Preserve historical record and terminal reason; do not discard or rewrite evidence when head is restored. |

No new export runtime is implemented here. A portability choice must be made before claiming provenance survives standalone exported Markdown outside VIVY's authority.

## 7. Incremental implementation sequence (proposal)

Each cut is separately reviewable and tested through the owning repository's established gates. Dependency/library upgrades must be pinned only after compatible source consumers are ready. No phase below is executed by this research delivery.

| Cut | Owner / modules | Work and exit condition | Dependency |
| --- | --- | --- | --- |
| 0. Freeze local proposal contract | VIVY domain/runtime owners + Laputa `evolution` | Agree local Skill authority exception, explicit-review class, source lineage, mutation bounds, expected-base and submission/application semantics. Read current #7/#31 boundaries. | Owner schedules implementation; research is not scheduling. |
| 1. Harden existing authority | VIVY `internal/runtime/skills_backend.go`, Service review handling, `internal/storage` | Finalize denied/expired/cancelled/stale/orphan revisions; validate all SKILL.md mutations; identical-content no-op; operation mapping/dedup; shared target CAS and crash reconciliation. Add paired migrations only if necessary. | No DIVA schema dependency; useful standalone correction. |
| 2. Extend typed strategy output | Laputa `laputa/evolution/{effects,contracts}.go`, `diva`, `inofy` | Add bounded `skill_proposal` union + input Skill context through existing strategy; strict decode/validate/digest/dispatch. Keep capability_proposal EvoMap-only. Missing local Skill binding yields explicit unavailable/skipped behavior and no repeated model spending. | Frozen contract; preserve existing consumers and strategy pins. |
| 3. Bind VIVY adapter | VIVY `internal/runtime/cognitive_binding.go`, cognitive composition in `internal/app`, existing SkillOperations | Host discover/read snapshot; map DIVA candidate+operation to existing staged revision; persist provenance and review receipt without applying; preserve Service/Journal/policy/budget/cancellation. | Cuts 1–2; explicit Eino capability check above. |
| 4. Reconcile other consumer | Current Go DIVA composition + Garden/Laputa domain construction | Compile against changed library; bind only supported local consumer or surface absence. No old SkillHome migration and no forced EvoMap installation. | Compatible Laputa pin; no second store. |
| 5. Real UI projection | VIVY review card/API/store, `plugins/vivy-evolution` owning UI | Backend revision/history/provenance projection and deep links; replace selected demo paths; retain existing decision route. Browser smoke on split pair. | Durable backend contract; UI never fabricates active capability. |
| 6. Usefulness evaluation | VIVY temporary-root integration fixtures and controlled real tasks | Prove reuse of an existing SOP and materially better outcomes on independent tasks; preserve negative/inconclusive results, cost and human correction count. | Operational cuts and separately authorized bounded evaluation. |

Do not gate Cut 1 or the local MVP on arbitrary strategy composition, a visual editor, experiments for every lesson, EvoMap, or a mass skill consolidation job. Broader strategies stay with #7. Keep runtime imports/policy/Journal boundaries and plugin architecture rules intact. A local host adapter should remain first-party and narrow unless a real external Provider requires a cataloged Port; naming a hypothetical Port is not sufficient evidence to add one.

Open decisions requiring explicit design agreement before coding:

1. Store provenance in a versioned revision payload vs typed metadata column, and persist/index operation → revision → review linkage using existing storage ownership. Pick one authoritative representation.
2. Project terminal reason on existing rejected/failed statuses vs adding explicit stale/expired/cancelled states. Preserve truthful UI and distinguish denied review from apply failure.
3. One pending capability proposal: immutable evidence links vs deferred follow-up references. Define bounds and cleanup; never edit the reviewed bytes.
4. Support-file read-set and package hash coverage; initial mutations should remain single-file to avoid unjustified batch complexity.
5. User/marketplace local adaptation policy and upstream upgrade conflict behavior.
6. Explicit-review marker/class in existing approval rows and policy, including timeout/system actors and restart. Do not infer permission from global preset.
7. Cross-run rollback authority: retain original revision/run lineage while authorize a new same-subject reviewed rollback; protect against selecting another subject or unrelated Skill.
8. Export completeness and representation; no custom frontmatter key as sole durability mechanism.
9. Skill-context retrieval budgets and selection uncertainty thresholds; tune from observed tasks rather than inventing a universal numeric score.

## 8. Acceptance matrix for future delivery

All rows below describe required future checks, not tests run in this session. Use temporary Skill roots and isolated SQLite/PostgreSQL databases, deterministic model stubs for fault/replay behavior, and separately authorized real task fixtures for usefulness. Do not run untrusted downloaded scripts in order to “validate” an SOP.

| ID | Scenario | Required assertion |
| --- | --- | --- |
| A01 | Two incidents about the same capability | Same canonical Skill target, successive revisions; no per-incident skill names. |
| A02 | Loaded Skill has fixable outdated step | Read exact Skill, narrow patch, preserve unrelated sections, cite user/tool evidence. |
| A03 | Class SOP exists but was not loaded | Discover and read before proposal; update/support decision explains owner. |
| A04 | New capability genuinely absent | Expected absence and stable name; valid SOP, provenance and pending human review. |
| A05 | Two plausible semantic matches / same-name overlays | Ambiguous/read-only result; no first-match mutation or duplicate escape creation. |
| A06 | Identical bytes / duplicate pending content | Durable no_change or same revision/review identity; no extra history head or model loop. |
| A07 | Replayed source event, same revision; recap of same event | Original operation/receipt and lineage reused; derived evidence not counted twice. |
| A08 | Source event was corrected/deleted/tombstoned | Revalidate provider version/scope; do not resurrect or cite stale source as current. |
| A09 | Same operation ID with changed payload/destination | Conflict, original bytes/review preserved, no new effect. |
| A10 | User-owned / project / external marketplace target | User target reviewed; project write refused; marketplace provenance/conflict retained. |
| A11 | Stale inference read before staging | Admission conflict before pending review; fresh read required; no newest-hash substitution. |
| A12 | Base changes while review pending | Apply refuses and finalizes stale/conflict; next proposal gets new review. |
| A13 | Two concurrent revisions same base | At most one winning filesystem apply; other conflicted; status/file hash consistent. |
| A14 | Revision vs enable/disable/marketplace upgrade | Shared target serialization, no lost update, clear conflict and preserved metadata. |
| A15 | New evidence while review pending | Pending diff immutable; follow-up evidence bounded; new source consumed once and later reevaluated. |
| A16 | Reject, expiry, cancel, restart | Each linked revision terminal/non-applied; no orphan pending; rejection prevents automatic identical re-proposal. |
| A17 | Global full-auto or timed smart approval | DIVA-origin revision still needs explicit human approval; timeout never applies it. |
| A18 | Approval actor spoof / changed args / cross-subject or root | Runtime rejects binding violation; model origin fields cannot change permissions. |
| A19 | Validation: malformed YAML/name, path traversal, symlink, binary, oversized, invalid patch | No mutation/pending success; bounded actionable error; all main-file mutations equally validated. |
| A20 | Create read error vs true absence | Permission/I/O/parse error never converted into create permission. |
| A21 | Revision persisted; approval/event/receipt persistence fails | No apparent submitted success; recover/reconcile original identity or terminal orphan. |
| A22 | File rename succeeds; DB status/receipt write fails or process crashes | Original effect recovered as committed/conflict/unknown with evidence; no new operation or blind overwrite. |
| A23 | File write fails; failed-status persistence also fails | Both causes observable; recovery fence prevents falsely reporting applied or retrying as new incident. |
| A24 | Restart at every boundary: before infer, after infer, after stage, after review, during apply | Resume same pins/source window/operation; no duplicate spend/write; already committed submission remains historical. |
| A25 | Missing Skill binding / EvoMap disabled / offline | Visible unavailable or local-only learning success as appropriate; no EvoMap prerequisite or repeated expenditure. |
| A26 | Edit/toggle/restart/export/import | Provenance/history survive all claimed operations; imported manifest never grants trusted origin/approval. |
| A27 | Cross-run rollback, intervening head, removed support file, create/delete rollback | Correct same-subject authorization and current-base review; precise package/file semantics, history preserved. |
| A28 | Pending/rejected/stale Skill vs active discovery | Only installed head is active; refresh at documented safe boundary; no pending candidate injection. |
| A29 | SQLite/PostgreSQL schema parity and migration reopen | Paired ownership, fresh install/upgrade/reopen, rollback on migration failure; no feature-owned DDL. |
| A30 | Review Center + Evolution real browser path | Full diff/provenance/history links and statuses reflect backend; approve/reject via one route; no localStorage truth. |
| A31 | Skill usefulness on held-out task | Record baseline/candidate revision, model/tools/environment/budget; observable successful output, lower correction or recovery failure; independent of extraction examples. |
| A32 | Unrelated tasks and alternate environment | No regression introduced by over-generalized SOP; record failures and cost as separate dimensions. |
| A33 | Evidence inconclusive or procedure failed | Candidate may remain proposed/applied-unverified; no “learned improvement” claim; retain negative observations. |

For A31, a useful concrete example is a Skill that previously omitted a necessary repository-specific validation step: baseline and candidate solve fresh tasks in disposable checkouts; compare actual output validity, missed checks, retries and human corrections. A scripted model choosing a tool is appropriate for replay tests but cannot establish an SOP's real effectiveness. An LLM judging its own generated Skill is supporting evidence only. Passing schema, review approval, number of uses and time since installation are not performance proof.


## Appendix A. Hermes actual end-to-end implementation

All observations in this appendix are source-confirmed at H unless marked as static risk. They describe actual implementation, not what an upstream marketing page promises. The relevant files are distributed across the agent, tools, CLI and curator; treating background_review.py alone as the implementation misses the approval and persistence semantics.

### A.1 Trigger and execution call graph

```text
agent initialization: creation_nudge_interval (default 10)
  → turn_iteration_prep increments _iters_since_skill when interval and tool available
  → turn_finalizer: threshold + delivered final response + not interrupted/skipped
  → AIAgent._spawn_background_review
      → automatic enable/delegation gate → clone structural snapshot
      → immediate spawn or coalescing process-local idle queue
  → _spawn_background_review_now: unique in-flight token, daemon thread/context binding
  → spawn_background_review_thread: skill / memory / combined prompt; optional focus
  → _run_review_in_thread → _run_review_fork: detached AIAgent
      → parent route or auxiliary route; full transcript or digest
      → dispatch whitelist + fresh read set + background write-origin
  → run_conversation → skills_list / skill_view / read_file → skill_manage
      → optional approval stage OR mutation lock/validate/write
      → best-effort ledger, usage telemetry, cache invalidation
  → action summary and usage attribution to parent; release clients
```

The interval default is [agent_init.py L1378–1382](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/agent/agent_init.py#L1378-L1382); [loop preparation L165–167](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/agent/turn_iteration_prep.py#L165-L167) checks interval and skill_manage availability, not whether the upcoming response contains tool calls. Thus “every ten tool-calling iterations” is less precise than the actual counter condition. [Executing skill_manage L744–747](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/agent/tool_executor.py#L744-L747) resets the counter before tool execution: a failed or merely staged action also resets it. [Turn finalizer L762–791](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/agent/turn_finalizer.py#L762-L791) schedules after final response delivery and excludes interrupted/skipped cases.

[Spawn gates L752–822](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/run_agent.py#L752-L822) suppress automatic delegated-agent reviews when focus is absent. Explicit focused refinement bypasses those automatic enable/delegation checks and explicit execution avoids idle deferral. [background_review_enabled L228–242](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/agent/background_review.py#L228-L242) defaults enabled and returns enabled with a warning on config read error.

[In-flight tokens L75–99](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/agent/background_review.py#L75-L99) prevent an older thread completion from clearing a newer run's state. Context/profile propagation binds thread work to its owner. Cancellation includes startup fencing and bounded acknowledgement; if deadline expires, foreground continues. See [L38–61](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/agent/background_review.py#L38-L61) and [L129–167](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/agent/background_review.py#L129-L167).

For managed local llama servers, [review_idle_queue.py L26–107](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/agent/review_idle_queue.py#L26-L107) uses `defer: auto`, a 15-second quiet window and a 30-minute age-out. A process-local dictionary coalesces snapshots by session: the newest snapshot replaces older context while retaining initial queue age and copied ContextVars. [Dispatcher L126–180](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/agent/review_idle_queue.py#L126-L180) checks age/idle and enabled state again. This queue is not durable and has no source-event replay receipt. Adopt foreground priority conceptually; reuse VIVY's durable cognitive gate instead of porting this scheduler.

### A.2 Context, evidence and isolated reviewer

[Route resolution L245–303](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/agent/background_review.py#L245-L303) normally inherits parent route/model/credentials. A distinct configured auxiliary route uses another model; resolution failure warns and falls back. The same-model path preserves cached system prompt, full cloned transcript, parent tool schemas and session/cache scope. It keeps advertised schemas stable and constrains actual dispatch separately.

The other-model [digest L324–354](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/agent/background_review.py#L324-L354) retains the most recent 24 messages, extends to avoid a tool-result-only beginning, and summarizes older user/assistant text with 300/200-character limits and tool names. Older tool-result bodies are omitted. This reduces cost but can omit exactly the failure/recovery evidence needed to justify a procedure change. In VIVY, preserve referenced original outcomes rather than assuming a recap is sufficient.

The [fork L912–1036](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/agent/background_review.py#L912-L1036) disables SessionDB persistence, recursive nudges, MCP refresh, external memory participation and session finalization; client-only cleanup avoids killing parent processes. Review is bounded to 16 iterations. [Budget helpers L169–225](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/agent/background_review.py#L169-L225) default aggregate input to 75% of model window with a 600k ceiling and 120k fallback; configured nonpositive budget disables that aggregate cap. These are token/work controls, not proposal evidence correctness checks.

Hermes does not first persist a structured immutable learning envelope with event IDs, read revision, rationale and observed outcome. The transcript is the model's evidence; the later ledger records a much narrower mutation audit.

### A.3 Update-first policy and enforcement distinction

The [skill review prompt L440–508](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/agent/background_review.py#L440-L508) prioritizes consulted Skill, then existing class-level umbrella, then topical supporting content, then a new class-level Skill only if necessary. The [combined review prompt L511–560](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/agent/background_review.py#L511-L560) repeats the principle. It encourages reusable corrections, new techniques, missing steps and obsolete instructions; discourages incident-named Skills and session-specific support files.

| Rule | Actual implementation level |
| --- | --- |
| Prefer the Skill used in the conversation | Prompt; no deterministic used-Skill-to-target decision guard |
| Discover/read a fitting class Skill before creation | Prompt; create can proceed without a proven alternatives search |
| Avoid incident naming and semantic duplicates | Prompt plus syntactic name validation; no semantic uniqueness check |
| Read target before existing-file background mutation | Exact-path ContextVar read-set guard |
| Base remains the same since that read | Not enforced by read set; no read hash/revision captured |
| Narrow allowed tools | Dispatch whitelist; parent schemas may still advertise broader tools |
| Never delete in normal review | Prompt; background deletion is supported under ownership/absorbed_into guards |
| Improvement is generalizable/useful | Model instruction; not established by parser, use count or successful write |
| Human approval | Optional configuration; default direct writes |

The prompt pressures most sessions to produce an update and frames inaction negatively. Do not adopt that incentive. VIVY must let no useful lesson, insufficient evidence and unchanged content terminate normally.

### A.4 Constrained tools and fresh reads

[Whitelist construction L1079–1119](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/agent/background_review.py#L1079-L1119) allows Skill tools, optionally memory, read_file/search_files, and configured extra parent tools. Ordinary write_file/terminal/file patch are otherwise denied. [Enforcement L1138–1184](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/agent/background_review.py#L1138-L1184) and [pre-tool whitelist before hooks](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/hermes_cli/plugins.py#L1959-L1985) implement the restriction. A thread-scoped dangerous-command callback auto-denies; [cleanup L1210–1291](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/agent/background_review.py#L1210-L1291) clears gates and accounts usage.

[skill_manager_guards.py L35–73](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/tools/skill_manager_guards.py#L35-L73) shares a mutable thread-safe set of resolved paths across copied tool contexts. Main and support [skill_view reads mark exact paths](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/tools/skills_tool.py#L590-L647); read_file integration also marks inspected files. Existing targets require marks, new Skills/support files do not. Foreground edits do not use this review guard. [_is_background_review L20–26](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/tools/skill_manager_guards.py#L20-L26) returns false on origin import/lookup failure. Background [repeat-view dedup is disabled](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/tools/skills_tool.py#L697-L720) to return full fresh content and produce read marks.

Read-before-write guards block missing inspection, but a path mark cannot detect changed content between read and mutation. Write locks serialize mutation execution; they do not prove the generated change was based on the current version.

Candidate inspection is also not entirely side-effect-free upstream: [skill_view L605–633](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/tools/skills_tool.py#L605-L633) can preprocess inline shell when opted in and ensure declared package dependencies. DIVA should inspect raw canonical content through VIVY's bounded reader, without activating embedded commands or dependency installation.

### A.5 Discovery, aliases and actual dedup

[Shared read precedence](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/agent/skill_utils.py#L381-L409) is trusted-project > profile-local > create_dir > external_dirs; project auto-discovery requires configured trust. [Alias resolution](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/tools/skills_tool.py#L342-L382) recognizes declared name, directory basename, relative path and legacy flat Markdown; plugin-qualified names use plugin registry/fallback representation.

[Same-tier tie handling](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/agent/skill_utils.py#L412-L487) refuses different candidates and provides precise load names. Byte-identical copies in the same root can prefer real SKILL.md over legacy flat Markdown and a strictly shallower copy; equal ranking remains ambiguous. Equality covers the main document or resolved file, not every supporting asset. [skill_view L471–517](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/tools/skills_tool.py#L471-L517) follows this resolver.

Mutation uses another resolver: [_find_skill L240–293](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/tools/skill_manager_tool.py#L240-L293) excludes trusted project roots, returns the first matching directory basename, and only refuses multiple fallback frontmatter-name matches. Local relative-path matching is confined to profile root. Consequently, a project Skill read by bare name may be unavailable to the manager or resolve to a different local target, and same-tier basename ambiguity does not have equivalent read/write protection. This is a static identity mismatch, not a reproduced overwrite. Adopt readable ambiguity handling; avoid separate target resolution rules.

Do not conflate five different operations: alias collision resolution, byte-identical catalog copies, repeat-view token dedup, unchanged ledger delta suppression, and semantic capability consolidation. Hermes has no general semantic duplicate detector for Skill creation.

### A.6 Mutation operations, validation and ownership

[Manager actions](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/tools/skill_manager_tool.py#L707-L725) include create; patch by exact/fuzzy matching or full content; legacy edit; write_file/remove_file; and delete with optional absorbed_into. Shape/size/path checks enforce lowercase safe names up to 64 characters, one-level category, YAML mapping with name/description, closed frontmatter/nonempty body, description limits, content up to 100k characters and support UTF-8 bytes up to 1 MiB. Allowed support roots are references/templates/scripts/assets. Default main-document patches are revalidated; an explicit file_path="SKILL.md" skips that frontmatter validation branch. See [patch validation L520–546](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/tools/skill_manager_tool.py#L520-L546). See [L41–64](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/tools/skill_manager_tool.py#L41-L64), [L108–188](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/tools/skill_manager_tool.py#L108-L188), and [L355–414](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/tools/skill_manager_tool.py#L355-L414).

[Optional security scan/default L41–64](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/tools/skill_manager_tool.py#L41-L64) and [post-write restore L395–414](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/tools/skill_manager_tool.py#L395-L414) is off by default, runs after writing, restores previous content/unlinks new content on a blocking result, and logs/allows scanner exceptions. It is neither a pre-write approval mechanism nor proof a procedure works.

[Mutation lock L84–105 and L765–814](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/tools/skill_manager_tool.py#L765-L814) uses a basename-hash lock file under active profile `.locks`. Cross-process fcntl/msvcrt locking includes thread reentrancy; unsupported mechanisms yield no-op locking. Bare foo and category/foo share a lock; a differing frontmatter display-name alias need not hash to that same lock. A shared external target edited through two different profile roots may have separate lock locations; do not claim universal cross-profile target serialization.

[Ownership guards L143–208](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/tools/skill_manager_guards.py#L143-L208) protect deletion/archive of pinned, essential, bundled, hub-installed, external and unmanaged Skills. They intentionally permit content edits broadly. Curator ownership is `.usage.json` management opt-in (`created_by == agent` or legacy agent_created), not an authorship identity. [Background delete absorption guard L232–252](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/tools/skill_manager_guards.py#L232-L252) checks an existing different absorbed_into target; background deletes archive. It does not verify that every instruction was incorporated or that two skills are semantically compatible. VIVY should keep its read-only project policy and explicit user review instead of copying blanket edit permissions.

### A.7 Batch behavior and limits

[Batch module](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/tools/skill_manager_batch.py#L86-L308) bounds operations to 20, validates shapes/destructive ordering, supports create/patch/write/remove composition, and requires delete to stand alone. A batch can stage as one pending item. Locks are acquired in stable order, touched directories snapshotted and operations applied sequentially through the ordinary manager.

A returned `success:false` triggers snapshot restore; rollback failure retains snapshots. Unexpected exceptions are not visibly routed through the same rollback branch, and cleanup in finally is not an undo operation. Intermediate ledger/telemetry/cache effects are not undone by restoring files. Files are visible before batch completion and no persistent transaction journal makes the batch crash-atomic. These are static failure-path limitations. Adopt bounded validation/ordered locking ideas only when a real multi-file need arises; do not port this as a second VIVY transaction engine. Single-file revision proposals are enough for the first local learning cut.

### A.8 Optional approval lifecycle and failure behavior

[write_approval.py L1–9 and L201–210](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/tools/write_approval.py#L201-L210) defines `skills.write_approval` as opt-in, default false for both foreground and background. When enabled, writes stage rather than apply inline. Pending JSON under `<home>/pending/skills/<id>.json` contains an 8-hex UUID fragment, subsystem, action, summary, origin, timestamp and exact operation kwargs/batch payload. It contains no expected base hash/revision or immutable scoped target/evidence/outcome contract.

```text
skill_manage approval gate
  → stage_write pending JSON (single-op shape/content validation follows staging)
  → /skills pending or diff
  → approve/apply: replay payload with gate bypass → revalidate/mutate now
      success → remove pending JSON
      failure → keep pending JSON
  → reject/deny/drop: remove pending JSON
```

Delete preflight precedes the gate; batch shape/category/clobber checks also precede batch staging. This ordering differs from single-operation content validation.

[Config gate L44–60](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/tools/write_approval.py#L44-L60) turns off on invalid/unset config or config load/import failure. [Manager gate L654–695](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/tools/skill_manager_tool.py#L654-L695) proceeds directly if approval import fails. [stage_write L65–89](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/tools/write_approval.py#L65-L89) logs persistence failure but returns the constructed pending object; its caller can return successful staged status although durable staging failed. This does not apply the Skill bytes, but misreports durable proposal ownership. Fail closed in VIVY instead.

[CLI commands L17–157](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/hermes_cli/write_approval_commands.py#L17-L157) support pending/diff/approve/reject, all-items forms and gate on/off. Background records display an auto-origin badge. Reject and successful approve remove JSON rather than retaining terminal history. [Diff L289–365](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/tools/write_approval.py#L289-L365) reads current disk and folds batch prefixes using the apply matcher, so it is not a staging-base snapshot. No pending dedup, persisted application claim or expected-base CAS is present in this queue.

Original origin is stored in pending JSON but replay receives only payload plus gate bypass, not original origin/session. [CLI replay L122–134](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/hermes_cli/write_approval_commands.py#L122-L134), [manager replay/creation attribution L698–758](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/tools/skill_manager_tool.py#L698-L758), and [usage creation L522–534](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/tools/skill_usage.py#L522-L534) mean foreground approval of a background creation is classified as `learn` rather than background `agent` creation. Ledger actor similarly follows execution context. Preserve proposed DIVA origin independently of the approving/applying actor in VIVY.

[Background action summary L658–663](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/agent/background_review.py#L658-L663) skips ordinary staged results unless a different proposal_staged flag is set. A pending badge is useful UX, but the summary is not a reliable complete evolution audit. Reuse VIVY revision/review projections, not this pending-file lifecycle.

### A.9 Ledger, history and rollback

[skill_ledger.py L35–173](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/tools/skill_ledger.py#L35-L173) defaults ledger on, uses `skills/.curator_ledger.jsonl` and content-addressed SHA-256 blobs under `.curator_backups/blobs`. Rows contain 12-hex ID, timestamp, actor, action, name, evidence and before/after path/hash manifests. [Manager ledger evidence L728–741](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/tools/skill_manager_tool.py#L728-L741) generally supplies session/file path and deletion absorption/archive data, not candidate-specific original events, rationale or performance outcomes.

Capture/append is best effort; failure does not block a Skill mutation. [Delta generation L286–323](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/tools/skill_ledger.py#L286-L323) omits unchanged path hashes, but an identical rewrite may still return success, append an empty delta and increase patch telemetry. This is not a content no-op guard. [Retention L343–415](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/tools/skill_ledger.py#L343-L415) defaults to 5 MiB and compacts/trims oldest history; it is not unbounded permanent provenance.

[Rollback L557–633](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/tools/skill_ledger.py#L557-L633) prechecks containment under Hermes home, blobs and safety snapshot persistence, saves current touched paths, restores before blobs/removes after-only files and logs rollback. External edited files outside home are refused. There is no visible current-head expectation check or mutation lock in this rollback path; sequential file writes are not a crash-atomic restore. Do not copy the comment-level claim that every autonomous write is reversible: ledger can be off/fail/trim history and external rollback can be unavailable.

### A.10 Usage metrics and curator consolidation

[skill_usage.py](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/tools/skill_usage.py#L347-L409) records uses/views/last times/patch counts, patch generation/reuse generation, lifecycle state, management opt-in and pinning. `skill_view` increments both use and view, including successful background reads. Reuse-after-patch is a later load marker, not a verified task benefit. Curator outcome success means no recorded LLM error, not performance improvement. Usage and patch volume should remain separate telemetry in VIVY.

[Curator configuration L106–179](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/agent/curator.py#L106-L179) enables periodic maintenance by default, normally weekly with two-hour idle minimum, stale at 14 days/archive at 30. First observation seeds scheduling. **LLM consolidation is disabled by default**. [Deterministic transitions L209–275](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/agent/curator.py#L209-L275) consider pinning/cron refs/management/age/first-seen grace; dry-run skips those writes.

[Optional consolidation L832–924](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/agent/curator.py#L832-L924) excludes bundled/disabled candidates, and [LLM setup L1085–1157](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/agent/curator.py#L1085-L1157) is skills-only/background origin/fresh-read guarded, but max iterations is 9999. Its dry-run LLM prohibition is prompted rather than a demonstrated immutable no-write dispatch surface. This path is distinct from conversation learning. Do not recommend mass merging or automatic age-based retirement for VIVY merely because the upstream curator supports it.

### A.11 Adopt, adapt, avoid

| Decision | Hermes idea | VIVY application |
| --- | --- | --- |
| Adopt | Update used Skill/class SOP before creating; additive topical support | Explicit decision policy with target/read evidence and review |
| Adopt | Foreground-priority, bounded background opportunity | Existing cognitive admission, budgets, cancellation and durable execution |
| Adopt | Diffs, before/after snapshots, mutation telemetry | Existing revision/review/history projection; usefulness separately measured |
| Adapt | Exact-path fresh read | Canonical scoped Skill target plus captured base/read-set hashes |
| Adapt | Alias ambiguity/byte-identical copy handling | One read/write resolver; distinguish main-file equality from package identity |
| Adapt | Optional pending approval | Mandatory explicit human review for this origin within existing Review Center |
| Adapt | Ledger/provenance/rollback | Durable revision provenance, original origin + applying actor, current-head reviewed rollback |
| Adapt | Locks/batches | Shared target CAS and operation recovery in existing authority; single-file MVP |
| Avoid | Default direct writes, fail-open approval/config | Failure means unavailable/error; never implicitly granted write permission |
| Avoid | Successful stage response after persistence failure | Durable submission receipt only after authority ownership established |
| Avoid | Post-stage/current-disk diff without pinned base | Review exact captured before/after, reject stale hash |
| Avoid | Background origin lost on human replay | Immutable trusted origin retained across approval/restart/export |
| Avoid | Blanket edits to external/bundled/user content | Preserve VIVY read-only project and explicit user-owned review policies |
| Avoid | Pressure to modify every session | First-class no-change/insufficient/ambiguous outcomes |
| Avoid | Use/view/patch counts as improvement proof | Independent held-out task outcomes, cost and correction measures |
| Avoid | Mass LLM umbrella consolidation and filesystem queue clone | No second Skill store, scheduler or approval engine |

## Appendix B. Historical DIVA and current capability gap

### B.1 Historical Rust SkillHome data and lifecycle

The historical implementation at R is concrete code, not merely a planned AutoDream feature. [skill_home.rs L39–166](https://github.com/ProjectViVy/agent-diva/blob/c565bb245cc920258d7f8c7fcd9544fbba545af7/agent-diva-core/src/evolution/skill_home.rs#L39-L166) defines stable slug, Skill document/history, source (`autodream`, `distill`, `user_request`), evidence and proposal. Proposal fields include request UUID, slug/title/full proposed Markdown, evidence/attestation, base_hash/source/reason/status and timestamps.

[SkillHome construction L245–255](https://github.com/ProjectViVy/agent-diva/blob/c565bb245cc920258d7f8c7fcd9544fbba545af7/agent-diva-core/src/evolution/skill_home.rs#L245-L255) uses machine-wide config_dir/skills. Home overrides same-slug Builtin. Evidence may cite session, ACTMEM pointer, AutoDream run, tool and artifact. [Validation L856–885](https://github.com/ProjectViVy/agent-diva/blob/c565bb245cc920258d7f8c7fcd9544fbba545af7/agent-diva-core/src/evolution/skill_home.rs#L856-L885) requires title/reason, evidence for automatic sources or manual attestation, and a 64-hex hash or `"0"` base. It establishes shape, not source authenticity or effectiveness.

```text
AutoDream reflection or user request
  → SkillHome.create_request: validate + same-slug pending suppression + UUID JSON
  → pending
      reject_request → rejected
      accept_request → revalidate + current hash CAS
          mismatch → stale persisted
          match → same-slug Home SKILL.md + history snapshot
              → accepted request persisted → epoch bump
```

[Create/reject/accept L496–587](https://github.com/ProjectViVy/agent-diva/blob/c565bb245cc920258d7f8c7fcd9544fbba545af7/agent-diva-core/src/evolution/skill_home.rs#L496-L587) uses a root-keyed process-local write mutex and one pending request per slug. It does not deduplicate repeated source events or identical proposal payloads. Acceptance obtains absence only from NotFound and propagates other errors; writes retain same slug, with [history L657–692](https://github.com/ProjectViVy/agent-diva/blob/c565bb245cc920258d7f8c7fcd9544fbba545af7/agent-diva-core/src/evolution/skill_home.rs#L657-L692) under `<slug>/history/<revision>.md` and requests under `requests/<uuid>.json`.

[Direct update L314–341](https://github.com/ProjectViVy/agent-diva/blob/c565bb245cc920258d7f8c7fcd9544fbba545af7/agent-diva-core/src/evolution/skill_home.rs#L314-L341) compares normalized content and returns unchanged without new history. Do not broaden this into a complete proposal no-op claim: acceptance always writes/marks accepted/bumps epoch; only duplicate latest history content suppresses another snapshot. Other direct mutation paths mark pending requests stale. [Locks L734–743](https://github.com/ProjectViVy/agent-diva/blob/c565bb245cc920258d7f8c7fcd9544fbba545af7/agent-diva-core/src/evolution/skill_home.rs#L734-L743) are process-local; [atomic write L944–981](https://github.com/ProjectViVy/agent-diva/blob/c565bb245cc920258d7f8c7fcd9544fbba545af7/agent-diva-core/src/evolution/skill_home.rs#L944-L981) syncs temp file/rename and best-effort directory sync. Head/history/request remain separate writes.

[Recovery L591–618](https://github.com/ProjectViVy/agent-diva/blob/c565bb245cc920258d7f8c7fcd9544fbba545af7/agent-diva-core/src/evolution/skill_home.rs#L591-L618), invoked by [manager startup L256](https://github.com/ProjectViVy/agent-diva/blob/c565bb245cc920258d7f8c7fcd9544fbba545af7/agent-diva-manager/src/state.rs#L256), can classify a pending proposal as accepted when normalized current head equals proposed content and repair history. That is content-based reconciliation, not proof a human approved it. Avoid importing that inference into VIVY permission recovery.

`evolution_managed` was not a special YAML key. [Read L297–298 and accepted_slugs L713–719](https://github.com/ProjectViVy/agent-diva/blob/c565bb245cc920258d7f8c7fcd9544fbba545af7/agent-diva-core/src/evolution/skill_home.rs#L713-L719) derive it from Home ownership plus **any accepted request**, including manually accepted user_request. For a DIVA provenance view, keep an explicit trusted source filter; a broad accepted flag is not evidence of automatic evolution. Historical YAML renderer preserved a full mapping, unlike current VIVY's typed unknown-key-dropping renderer.

The [historical handlers L216–265](https://github.com/ProjectViVy/agent-diva/blob/c565bb245cc920258d7f8c7fcd9544fbba545af7/agent-diva-manager/src/handlers/skills.rs#L216-L265) expose create/accept/reject through SkillHome. Existing tests of no-op/CAS/stale, request evidence and orphan repair, and evolution_managed were inspected as intent, not executed proof. Restore the semantics through VIVY owners; do not migrate/copy this old store.

### B.2 Historical AutoDream path and shortcomings

[Worker](https://github.com/ProjectViVy/agent-diva/blob/c565bb245cc920258d7f8c7fcd9544fbba545af7/agent-diva-autodream/src/worker.rs#L199-L505) collects evidence, organizes ACTMEM Work, requests reflection, validates candidates and creates `autodream` Skill requests. Restricted-profile checks precede reflection. Input bounds include Work 4,000 chars, Pulse/Recap 1,000 each, first 16 evidence summaries at 500 chars, full MEMRULES and maximum eight candidates.

The [Skill index/input DTO](https://github.com/ProjectViVy/agent-diva/blob/c565bb245cc920258d7f8c7fcd9544fbba545af7/agent-diva-autodream/src/reflection.rs#L24-L62) provides only slug and hash, not descriptions/body. The generated candidate has slug/title/description/full Markdown/reason and no read base. [LLM runtime L93–209](https://github.com/ProjectViVy/agent-diva/blob/c565bb245cc920258d7f8c7fcd9544fbba545af7/agent-diva-manager/src/runtime.rs#L93-L209) disables tools, uses temperature 0.1 and 1,024-token limit, up to three schema attempts, and a 90-second stream-consumption timeout after chat_stream returns. Do not claim that timeout bounds the complete initial request await. The prompt allows updates to existing slugs but lacks Hermes' explicit update-first hierarchy.

Crucially, [worker L452–473](https://github.com/ProjectViVy/agent-diva/blob/c565bb245cc920258d7f8c7fcd9544fbba545af7/agent-diva-autodream/src/worker.rs#L452-L473) captures the current hash **after** generation and turns every read error into `"0"`. It therefore cannot bind a candidate to a body the model actually read and can misclassify authority failure as absence. Worker-generated evidence cites ACTMEM Work/run/artifact, not a candidate-selected validated set of original events. Fix both in the proposed bridge.

Pending same-slug collisions are recorded as skipped; malformed candidates as diagnostics. [Worker L343–362](https://github.com/ProjectViVy/agent-diva/blob/c565bb245cc920258d7f8c7fcd9544fbba545af7/agent-diva-autodream/src/worker.rs#L343-L362) degrades reflection failure after successful ACTMEM organization and can mark proposal stage successful with zero IDs. This can be legitimate partial maintenance, but is not a successful Skill improvement. Report stage/result distinctions in VIVY.

### B.3 Current DIVA → Garden flow

At current pins the actual strategy lives in Laputa, while current Go DIVA composes VIVY and Laputa capabilities. [Closed effect kinds](https://github.com/ProjectViVy/laputa/blob/4b2bec2cc2ab3374b7ec1ab8d448e612c1e4db24/laputa/evolution/effects.go#L12-L17) are work_patch, memory_mutation, persona_request, capability_proposal and reflection_note. There is no local skill_proposal. [Capability payload L112–126](https://github.com/ProjectViVy/laputa/blob/4b2bec2cc2ab3374b7ec1ab8d448e612c1e4db24/laputa/evolution/effects.go#L112-L126) is EvoMap-only and lacks local target, mutation, base/read identity and Skill revision.

[Reflect prompt](https://github.com/ProjectViVy/laputa/blob/4b2bec2cc2ab3374b7ec1ab8d448e612c1e4db24/laputa/evolution/diva/prompts.go#L21-L44) and strict candidate decoder agree with that vocabulary. [Strategy reflect/effects L229–375](https://github.com/ProjectViVy/laputa/blob/4b2bec2cc2ab3374b7ec1ab8d448e612c1e4db24/laputa/evolution/diva/strategy.go#L229-L375) assigns trusted operation keys with candidate index, digests typed payload/scope/destination, looks up original receipts and applies only on not-found. Changed replay digest conflicts; unknown outcomes require recovery. Submitted output is distinct from applied.

```text
VIVY diva-cognitive factory.BoundDomain
  → Garden embedded owner.BindEvolution
  → evolution.NewDomain(ACTMEM/Persona/Memory/Activity/Dir; no Proposals)
  → DIVA reflect: capability_proposal
  → Domain.applyCapabilityProposal
  → missing EvoMap proposer → rejected receipt evomap_unavailable
```

Sources: [VIVY factory L211–231](https://github.com/ProjectViVy/agent-vivy/blob/ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666/internal/modules/diva-cognitive/factory.go#L211-L231), [Garden binding L333–361](https://github.com/ProjectViVy/laputa/blob/4b2bec2cc2ab3374b7ec1ab8d448e612c1e4db24/garden/agentapi/embedded_domain.go#L333-L361), [capability apply L352–375](https://github.com/ProjectViVy/laputa/blob/4b2bec2cc2ab3374b7ec1ab8d448e612c1e4db24/garden/evolution/domain.go#L352-L375). This is a source-traced unbound consumer, not an observed live rejection in this session. Binding EvoMap would still not create the required local Skill review/CAS path.

### B.4 Reuse source identities, enforce provenance membership

[Window/SourceRef/Entry/EvidenceBatch](https://github.com/ProjectViVy/laputa/blob/4b2bec2cc2ab3374b7ec1ab8d448e612c1e4db24/laputa/evolution/contracts.go#L360-L469) plus [RunBinding L203–213](https://github.com/ProjectViVy/laputa/blob/4b2bec2cc2ab3374b7ec1ab8d448e612c1e4db24/laputa/evolution/contracts.go#L203-L213) already supply trusted subject/workspace/destination/policy/strategy pins, `(after, through]` source window, source_id/record_id/revision/scope, original session/event/time and entry body. [Ingestion](https://github.com/ProjectViVy/laputa/blob/4b2bec2cc2ab3374b7ec1ab8d448e612c1e4db24/garden/internal/ingest/service.go#L193-L294) preserves accepted sequence/event/hash, rejects changed same-event content, deduplicates repeated acceptance, and fences unresolved projection behind the source watermark. Use these rather than inventing a separate evolution-event UUID.

The current contract validates SourceRef shape/scope; the inspected strategy/domain does not independently enforce that every candidate citation belongs to the admitted evidence batch. The prompt asks for batch sources. Add deterministic membership/revision/scope checks for local Skill proposals; don't treat a valid JSON citation as authenticated evidence. If additional retrieval is allowed, host-register that authorized evidence in the read set before it is cited.

Garden's effect ledger and notes are separate from VIVY Skill revision history. Existing Lookup fallback recovers memory mutation receipts, not future local Skill submissions. A local bridge must support operation → durable revision/review lookup to recover a crash after staging but before effect-receipt persistence. A submitted receipt should point to the existing VIVY revision; Garden must not become Skill authority.

### B.5 Capability and gap table

| Concern | Historical Rust DIVA | Hermes H | Current VIVY/Laputa | Minimal proposed restoration |
| --- | --- | --- | --- | --- |
| Same capability identity | Stable slug + head/history | Names/aliases; update-first prompt | Canonical Skill directory/name + revision records | Reuse target, accumulate revisions |
| Existing-Skill context provided to reflection | Slug/hash index only | Reviewer can read full Skill | Reader exists; DIVA receives no local candidate context | Host-bounded discovery/read snapshot |
| Proposal type | SkillProposal with evidence/base/source | Operation kwargs; optional pending JSON | Typed effects omit local Skill; existing SkillRevision separate | Narrow local skill_proposal bridge |
| Evidence | ACTMEM/run/session/artifact, coarse validation | Transcript; narrow ledger session/file | Admitted event/source/sequence identities exist | Validate membership and retain immutable citations/outcomes |
| Read-base binding | Post-generation hash in worker | Path-only read mark | Stage-to-apply BaseHash check | Read-to-stage expected hash + apply CAS |
| Human review | Pending/accept/reject HTTP flow | Optional; default direct | Review Center live, global auto/timeout paths | Trusted explicit-review class in existing engine |
| Pending suppression | One per slug | No general pending/event dedup | Random revision ID; pending-list | Operation digest replay + content/target/base dedup |
| No-op | Direct normalized update; history dedup in accept | Ledger delta only | Patch same-string rejection; no broad edit no-op | Before staging compare exact canonical after bytes |
| Provenance classification | Any accepted request marks evolution managed | Background origin can be lost on pending replay | No typed DIVA Skill origin; YAML keys drop | Revision-owned origin + separate approving actor |
| Terminal finalization | Accepted/rejected/stale | Pending removed on approve/reject | Rejected enum exists; several terminal links missing | Finalize each review/apply outcome idempotently |
| Concurrency | Process-local root mutex | Profile lockfiles, no base CAS | Read/check/write/status is separable | Shared target owner/CAS and operation recovery |
| History/rollback | Per-slug snapshots; content-based orphan recovery | Optional/best-effort/bounded ledger rollback | BeforePayload/trash and run-bound rollback | Durable lineage and reviewed same-subject cross-run rollback |
| UI | Real historical request/history handlers | Pending CLI/diff + ledger CLI | Real Skills/Review; Evolution demo | Existing UI backed by revision/history projection |
| Usefulness | Not proven by request acceptance | Use/view/patch and LLM completion metrics | No restored local learning evaluation | Observable held-out task outcomes; no compulsory mass benchmark |

## Appendix C. Versioned evidence map and reading order

This index complements the call graphs above. Every link uses an immutable commit; line anchors are checked against the corresponding fetched object, not against mutable main. A developer can begin with the exact seam needed without treating all linked code as a mandatory porting scope.

| Question / symbols | Pinned source |
| --- | --- |
| Hermes trigger gates and background fork | [run_agent.py L752–822](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/run_agent.py#L752-L822); [background_review.py L912–1184](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/agent/background_review.py#L912-L1184) |
| Hermes discovery/ambiguity and write resolver | [skill_utils.py L381–487](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/agent/skill_utils.py#L381-L487); [_find_skill](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/tools/skill_manager_tool.py#L240-L293) |
| Hermes mutation guard, batch, approval and ledger | [guards](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/tools/skill_manager_guards.py#L35-L73); [batch](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/tools/skill_manager_batch.py#L219-L308); [approval](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/tools/write_approval.py#L44-L89); [ledger](https://github.com/NousResearch/hermes-agent/blob/c0ef5af5205dbdc25e9e44acd66873b4bb3503c9/tools/skill_ledger.py#L298-L323) |
| Historical stable head/history/review | [SkillHome create/accept](https://github.com/ProjectViVy/agent-diva/blob/c565bb245cc920258d7f8c7fcd9544fbba545af7/agent-diva-core/src/evolution/skill_home.rs#L496-L618); [head/history](https://github.com/ProjectViVy/agent-diva/blob/c565bb245cc920258d7f8c7fcd9544fbba545af7/agent-diva-core/src/evolution/skill_home.rs#L657-L719) |
| Historical AutoDream reflection/base weakness | [worker.rs L393–505](https://github.com/ProjectViVy/agent-diva/blob/c565bb245cc920258d7f8c7fcd9544fbba545af7/agent-diva-autodream/src/worker.rs#L393-L505) |
| Current DIVA composition pin | [current Go DIVA go.mod](https://github.com/ProjectViVy/agent-diva/blob/0a64d1ce152440f1f63d4224a155c4616d2aff5b/go.mod#L1-L25) |
| Current strategy reflect/effects/replay | [Laputa strategy.go L229–375](https://github.com/ProjectViVy/laputa/blob/4b2bec2cc2ab3374b7ec1ab8d448e612c1e4db24/laputa/evolution/diva/strategy.go#L229-L375) |
| Current VIVY persisted trigger/window fences | [cognitive_service.go L35–71](https://github.com/ProjectViVy/agent-vivy/blob/ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666/internal/runtime/cognitive_service.go#L35-L71) |
| Current VIVY trusted DIVA binding, one-shot inference | [cognitive_binding.go L197–235](https://github.com/ProjectViVy/agent-vivy/blob/ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666/internal/runtime/cognitive_binding.go#L197-L235); [L268–350](https://github.com/ProjectViVy/agent-vivy/blob/ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666/internal/runtime/cognitive_binding.go#L268-L350) |
| Revision staging/apply/CAS | [skills_backend.go L213–328](https://github.com/ProjectViVy/agent-vivy/blob/ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666/internal/runtime/skills_backend.go#L213-L328) |
| Approval binding/resume | [approval_binding.go L8–38](https://github.com/ProjectViVy/agent-vivy/blob/ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666/internal/runtime/approval_binding.go#L8-L38); [service.go L4751–4782](https://github.com/ProjectViVy/agent-vivy/blob/ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666/internal/runtime/service.go#L4751-L4782) |
| Denial/expiry/cancellation finalization audit | [denied dispatch L225–234](https://github.com/ProjectViVy/agent-vivy/blob/ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666/internal/runtime/tooladapter.go#L225-L234); [cancel L4397–4412](https://github.com/ProjectViVy/agent-vivy/blob/ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666/internal/runtime/service.go#L4397-L4412); [expire L4462–4503](https://github.com/ProjectViVy/agent-vivy/blob/ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666/internal/runtime/service.go#L4462-L4503) |
| Actual Review Center/Card | [ApprovalsView L12–55](https://github.com/ProjectViVy/agent-vivy/blob/ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666/ui/src/components/approvals/ApprovalsView.tsx#L12-L55); [ReviewCard L38–103](https://github.com/ProjectViVy/agent-vivy/blob/ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666/ui/src/components/approvals/ReviewCard.tsx#L38-L103) |
| Review decision/projection and pending-only revision RPC | [control.go L4181–4243](https://github.com/ProjectViVy/agent-vivy/blob/ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666/internal/rpc/control.go#L4181-L4243); [L2969–2993](https://github.com/ProjectViVy/agent-vivy/blob/ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666/internal/rpc/control.go#L2969-L2993) |
| Actual Skills UI versus Evolution mock | [SkillsView L1–67](https://github.com/ProjectViVy/agent-vivy/blob/ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666/ui/src/components/skills/SkillsView.tsx#L1-L67); [Evolution demo registration](https://github.com/ProjectViVy/agent-vivy/blob/ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666/plugins/vivy-evolution/ui/vivy-evolution/src/index.tsx#L17-L23); [mock imports](https://github.com/ProjectViVy/agent-vivy/blob/ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666/plugins/vivy-evolution/ui/vivy-evolution/src/useEvolution.ts#L1-L30) |
| Framework read backend and checkpoint APIs | [Eino Skill Backend](https://github.com/cloudwego/eino/blob/c5e6aef927cca02bea934541f8dff2ea711b2ca7/adk/middlewares/skill/skill.go#L48-L69); [Runner](https://github.com/cloudwego/eino/blob/c5e6aef927cca02bea934541f8dff2ea711b2ca7/adk/runner.go#L55-L149) |

## 9. Delivery and verification record

Delivered artifacts are this guide, one focused research backlog entry, and the corresponding research iteration logs. The local branch is `research/hermes-skill-evolution-20261011`, based on VIVY `ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666`. Only local documentation is committed. A private Library copy of the full guide is provided solely as an accessible deliverable. Repository publication/PR/push/merge/deployment is left to the parent session under its authorization.

Checks: refreshed/pinned Git revisions; read repository instructions and relevant `.agents/skills/vivy-eino`, `.agents/skills/vivy-plugin`, and CI scope instructions; traced source and inspected existing tests without executing them; checked cited pinned paths and line ranges; reviewed documentation diff and whitespace. Full `just ci`, browser smoke and runtime fault tests are intentionally not run: no runtime/product-contract implementation was changed, and repository scripts are outside this research-only execution scope.

The actionable blockers are future design/implementation work identified in Sections 3 and 7, not blockers to completing this research artifact. Claims of restored runtime evolution, successful migration, approved installation, runtime fault reproduction, or demonstrated improved Skill performance would require new authorized work and the acceptance evidence above.
