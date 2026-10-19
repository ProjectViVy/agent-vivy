# Vivy Complete Deletion Plan: Scope, Order, and Two Independent Audits

Date: 2026-10-19

Tracking: [GitHub issue #91](https://github.com/ProjectViVy/agent-vivy/issues/91). Plan branch: `feat/agent-loop-deletion` (baseline A); original plan branch: `codex/agent-loop-deletion`.

Executable planning package: [issue-91/index.md](../superpowers/plans/issue-91/index.md).
This document remains the shared scope/design authority. Sections 10–11 specify
the replacement contracts and detailed target architecture; the index owns Story status, dependencies,
decision gates and acceptance evidence. Planning is authorized; implementation
requires a separate implementation handoff. Scope is settled; concrete upstream
delivery and verification prerequisites remain in the index. The issue's embedded snapshot
is historical and does not override this linked package.

Owner scope update (2026-10-19): **retain nudge/coaching, proactive warming and same-Run steering as species-selected experimental packages**. Their whole-feature-retirement alternatives below are superseded; D04/D05 in the linked package preserve actual features while removing custom hashes/redundant coordination. Tool-visibility/durability choices remain separate. No runtime deletion has occurred.

Baseline A and delivery split (2026-10-19): the implementation baseline is `202aac38` (2026-10-07), the last stable pi-parity closeout that passed benchmark runs. The 168 commits and 30 first-parent merges added after it are unverified increments; they are not assumed present and are to be re-reviewed and re-introduced in gated batches. This plan and the linked package were authored at `e9609296` (2026-10-18) and replayed onto baseline A; 10-18-only file references are re-anchored in the [baseline-A audit](../research/2026-10-19-baseline-a-reanchor.md). Issue #91's base cut lands hash deletion, the core contraction and the inert loop-exp seam; the three retained packages are delivered as the independent [E-NUDGE](../superpowers/plans/epic-nudge/index.md), [E-WARMING](../superpowers/plans/epic-warming/index.md) and [E-STEERING](../superpowers/plans/epic-steering/index.md) epics. Until each epic lands, the corresponding behavior stays in one core implementation; C11 physical omission is claimed only by the epics, never by #91.

Status: Pre-deletion inventory and proposed execution plan. No runtime deletion, regeneration, database migration, or behavior test has been performed by this delivery.

## 1. Decision and completion standard

The owner's direction is **complete deletion of Vivy's custom hash system**, with only an optional informational digest of the final distributable remaining. Merely moving a check out of assembly or the foreground loop does not satisfy this direction. The same closure standard applies to every other mechanism explicitly selected for retirement.

Delete its implementation, construction, callers, state, persisted representation where applicable, configuration, public fields, generated outputs, feature-only tests, and current instructions requiring it. Do not leave a disabled implementation, a no-op compatibility wrapper, an ignored configuration field, or a replacement with another hash algorithm/name. Do not move rejected complexity to a plugin while retaining a core dependency on it. Rollback uses version control and a documented data strategy, not a second implementation in the shipped product.

This direction does not automatically retire every capability discussed in the research. Hash retirement is confirmed. The owner has now explicitly retained reminders/nudge coaching, proactive warming and same-Run steering. Tool-mount lifetimes and companion changes remain separately scoped decisions. Once selected, each must be deleted completely; an implementation cannot claim completion by making it optional.

One small final-artifact label is permitted: compute it after packaging, outside the bytes being hashed, and never read it as a prerequisite for installation, update, rollback, Inspect, startup, model/tool execution, continuation, approval, recovery, or rendering. Missing, malformed, or mismatching label metadata cannot change those behaviors. It identifies bytes; it is not evidence that functional tests passed.

## 2. Audit baseline and coverage

The inspected implementation is `origin/main@e96092966cfbfa443648374948e7e7e16dd01683`, containing 3,751 tracked paths. During the audit the checkout was based on older product code (`b4db00bd` plus documentation changes), so remote files were read directly. On 2026-10-19, branch `codex/agent-loop-deletion` was created and the documentation commits were rebased onto the freshly pulled `origin/main` at the same audited `e9609296` baseline. Recheck any subsequent upstream changes before implementation.

Five read-only subagent lanes reviewed runtime ownership; SDK/assembly/plugin generation; storage/recovery; UI/RPC/configuration; and CI/tests/current instructions. The lead reconciled their findings and checked cross-lane boundaries. No production Journal was accessed.

A repository-wide lexical sweep for SHA/MD5/FNV/HMAC, digest, checksum, fingerprint and content-address terminology found 616 matching tracked files, excluding dated `docs/logs/` and dependency lockfiles. These are search candidates, not 616 defects or 616 mechanisms. They include tests and documentation. The sweep exposed the separately built `bml/` module and event schemas beyond the first runtime inventory. It does not inspect submodule contents or prove that every semantic alias has been found.

| Area | Matching files in this sweep |
| --- | ---: |
| `internal/` | 310 |
| `docs/` outside logs | 105 |
| `plugins/` | 77 |
| `sdk/` | 57 |
| `ui/` | 29 |
| `bml/`, `schemas/` | 10 each |
| `.agents/`, `faces/` | 5 each |
| `.superpowers/`, `scripts/`, root `AGENTS.md` | 4, 3, 1 |

This is a cross-repository pre-audit, not an implementation-ready sign-off on every persisted field. The early inventory below identifies deletion families; §§10–11 resolve their technical contracts. The implementation lane must satisfy the recorded delivery prerequisites for its cut; the post-deletion audits remain pending.

The [core insight](../research/2026-10-19-agent-loop-core-deletion-insight.md) supplies detailed source links and distinguishes confirmed dependency edges from unproven incident hypotheses. This plan supplies deletion closure and execution order rather than duplicating that diagnosis.

## 3. Hash deletion families

Paths below are relative to the audited repository. A row describes a dependency closure, not permission to delete every file named in it. Shared files retain unrelated behavior.

| Family | Concrete implementation and propagation | Required final state |
| --- | --- | --- |
| Source/Recipe/plugin evidence | `internal/sourcehash/tree.go`; `sdk/internal/assembly/source.go` (`SourceCatalog`, `HashSourceTree`, `VerifyUnchanged`); Descriptor/Recipe source fields; `sdk/internal/catalog_v1.go`; plugin descriptors | Remove tree hashing, digest pins, rehash gates and metadata. Retain explicit selected source/version and typed dependency/Port composition. Update pack, verify and inspect together. |
| Assembly/Generation evidence | `sdk/generation/manifest.go`; `sdk/internal/assembly/manifest.go`, compiler/generators/conformance; `internal/generated/assembly/zz_default.go`; UI generated Assembly | Remove embedded digest chains and digest-dependent identity. Generate from simplified input contracts. Functional conformance remains executable behavior evidence, not a runtime digest gate. |
| Frontend asset/catalog evidence | SDK UI build/staging; `ui/src/plugins/presentation-host.tsx` (`validateCatalogProjection`, custom `sha256Hex`); generated UI catalog/asset metadata | Remove asset hash rewrite/rehash cycle and catalog digest rejection, including schemas and fixtures. Retain necessary syntax, catalog shape and explicit module selection. |
| Bundle/content-addressed packaging | `internal/bundlecontract/{digest,identity,manifest}.go`; `internal/bundles/{acquire,artifact,manager}.go`; bundle install/update/rollback and Inspect/RPC/UI consumers | Remove per-bundle digest identity/admission, not only calculation. Decide explicit artifact identity/reference semantics and update all lookup/storage consumers. Do not retain old digest paths as the new normal. |
| Event audit | `internal/runtime/audit.go`; default `AuditHook` in `internal/app/app.go`; audit tests and digest-only logs | Delete the whole digest audit facility. Retain canonical Journal and bounded, redacted operational errors. Preserve unrelated hooks only for actual retained consumers. |
| Model completion stream | Runtime mapper and `model.completed` payload/schema; `sdk/facerun/facerun.go`; `sdk/tui/stream`; headless app and Face consumers | Remove `content_sha256` production, accumulation, verification, DTO fields and hash-error states together. Keep call/event correlation, supported framing, final text, one terminal and replay behavior. |
| Admitted execution snapshots | Prompt/policy/toolset/mask/persona/context/skill/MCP hashes in runtime, Hosts, domain, storage, RPC and Face projections | Remove digest metadata and gates. Where identity matters, use the authoritative admitted value or existing semantic revision; delete duplicate snapshots/evidence with no independent purpose. |
| Tool operation and approval binding | `tool_operation.go`, `approval_binding.go`, `tooladapter.go`, domain operation types, SQLite/Postgres implementations and event schema | Remove digest fields and comparisons while retaining exact intended inputs, operation ownership and durable effect outcome. Raw request binding needs an explicit design; it is not proven redundant. |
| Files/references/deliverables | File preconditions/version storage, downloads, reference/report/deliverable services, RPC exports and associated schemas | Delete content-hash authority and fields. Retain a correct expected-content or authoritative version boundary where edits/approvals require it. Timestamp/size alone is not content equality. |
| Checkpoints and migrations | `internal/runtime/checkpoint.go`; SQLite/Postgres `blob.go`; storage migration manifest/runner/metadata | Delete both written-only and actually validated checksum layers through a defined data transition. Keep format/version, Run binding, ordered transactional migration and failure rollback. Record lost independent corruption/drift detection explicitly. |
| BML identities and evidence | `bml/{record,home,maintenance,migrate,store}.go` | Remove content-digest validation, digest-derived memory/workspace IDs and migration evidence. Resolve existing identity/data references before cutover. Inspect apparently unused journal DDL for whole-table removal rather than leaving an empty shell. |
| Custom signed cursor | `internal/runtime/history_service.go` HMAC encode/decode path | Resolve the client-input trust contract: remove the custom signed-cursor protocol or use a simpler authorized pagination contract. Do not merely skip the MAC check and retain trusted cursor claims. |
| Nudge/warming hashes | `nudge_window.go` signatures; `cachewarmer.go` prefix evidence; associated events/config/tests | Retain confirmed nudge/warming behavior; replace nudge FNV signatures with a bounded exact-value window and delete warming prefix hash-only diagnostics. Do not rename a digest or disable the feature. Details: D04/D05. |

Native dependency integrity and transport/authentication cryptography need a precise scope boundary. The recommended boundary is to remove Vivy's invented product evidence, not fork Go/pnpm/TLS to remove their cryptographic internals. This remains an explicit scope decision, not a hidden exemption. Any Vivy-owned security mechanism listed above still needs a concrete disposition.

### Source checks that change the deletion strategy

- The in-repository lifecycle owner `internal/studiocore/service.go:421` compares `ArtifactSHA256` before installation. The sole retained release label must not preserve this business gate; the external Studio submodule has not been audited by this inventory.
- Source catalog construction hashes sources before Recipe compilation. Kernel modules can share the entire `internal/` source root; the walker includes regular untracked source/test files except its specific exclusions. A small Recipe alone cannot guarantee a small validation dependency surface. Remove this mechanism, rather than add another source-hash cache.
- Frontend build/staging and runtime catalog validation form one evidence chain. Deleting only generated hashes allows the generator to restore them; deleting only the generator breaks consumers.
- `sdk/facerun.completeModelLocked` rejects a mismatching `content_sha256` and later reports a local failed terminal. Headless/TUI also verify completion. This is confirmed client-side failure authority, not proof that this branch cancels the backend Run or caused the reported incident.
- `sameRuntimeToolOperationBinding` compares stored argument bytes as well as digests. However, `RequestDigest` binds original input while the row stores middleware/effective input bytes. Removing both digests is not justified merely by seeing two byte comparisons; decide original-request identity first.
- Checkpoint storage writes a `checksum` column that its blob reader does not validate; the runtime checkpoint envelope separately validates `checksum_sha256`. Delete both producers/fields as appropriate, but do not confuse a dead column with the active envelope check.
### Other concrete consumers that cannot be omitted

- RPC/export/download/reference surfaces: `internal/rpc/{exports,deliverables,work_control,control}.go`; `ui/src/lib/{api,deliverable-download,store}.ts`; `HistoryPicker.tsx`; reference/detail components. Remove `ExpectedDigest`, `expected_digest`, `precondition_hash`, hash-derived Work IDs and MCP command-name hashing. Retain authorized snapshot selection, chunk consistency, retry identity and collision handling through a simpler explicit contract.
- Plugin feature surfaces: toolkits' candidate/release/declaration/effective digests; masks' release/cache digest; memory/evolution content/base hash CAS; notebook input/facts digests; workflow definition/catalog/program/result/resume digests. Their UI clients, DTOs, runtime owners, SQL keys and tests must change together. `ui/src/lib/demo-api.ts` implements mock hash CAS too; it is active demo behavior, not an exemption.
- Host-generated identities: `internal/contexthost/{filesource,host}.go`, `internal/mcphost/{resourcesource,host}.go`, `internal/skillhost/host.go`. Hashes currently represent candidate versions, de-duplication, schema invalidation, provenance and IDs. Choose actual identity/version/content semantics per retained feature. Memory provider IDs also hash a Run ID suffix; preserve uniqueness and idempotency without that computation.
- Persisted feature families: storage admission/snapshots, continuity, child sessions, Work, workflow/report/INOFY, notebook and bundle/toolkit state. Digests appear in uniqueness constraints and receipt identity, not just columns. A blanket column drop is insufficient.
- Event schemas: `run.started`, `model.request`, `model.completed`, `policy.evaluated`, `tool.operation`, `context.reference_attached` and workflow events. Remove old fields from `required`, examples and version unions as well as properties. Never substitute raw credential-bearing operation arguments into public events to replace a removed digest.
- Explicit dead structure: `bml/README.md` says the upstream governed-write seam was dead and was not ported, while `memory_apply_journal` remains for byte compatibility. The inspected references are DDL, table-existence tests and that explanation. Remove the obsolete table contract and tests, and migrate existing stores without deleting real memory records. This is direct evidence of compatibility residue, not merely a complexity heuristic.
- `plugins/exp/loop-detection/deferred.go` retains an FNV loop window despite deferred status. Include unused experimental implementations in the retirement inventory; a deferred label is not a reason to keep dead code.

## 4. Core-state deletion cuts

The highest-priority architectural goal remains: **one real tool outcome must reach Eino and allow continuation without agreement from multiple auxiliary state owners**. Hash deletion alone does not establish that goal.

| Cut | Delete together if selected | Behavior that must still work |
| --- | --- | --- |
| Proactive warming (retained) | Delete only PrefixSHA256 and hash-only serialization/diagnostics; retain warmer, configuration, actual maintenance call, event/UI and accounting. | Existing opt-in/eligibility/budget/timeout behavior works; admitted paid work is accounted and cannot recurse. D05. |
| Reminder/nudge coaching (retained) | Delete FNV and consumer-owned handoff only; retain exact bounded repetition window, advisory state/middleware, scheduling event and templates. | Eligible repeated failures/refusals still receive coaching; preserve schedule-before-injection, budgets, retries and approval/steering resume. Durable effects remain owned by the tool boundary. D04. |
| Synthetic tool lifecycle authority | Reconstructed `tool.started` and duplicate lifecycle ownership | Present actual execution facts, including refusal before claim; UI timing cannot pretend that result arrival was execution start. |
| Derived-work critical section | Old synchronous projection/output/hook path under `projectionMu`, including obsolete helpers and fallback path | Retain commit/terminal/delete ordering; derived views never acknowledge progress back to the runner. Do not replace it with another required coordination service. |
| Tool visibility lifetimes | Activation/mount/suppression mirrors, rebuild paths, associated session state/events/config | One clearly owned tool surface and server-side permissions. Decide fixed Run surface versus one supported search mechanism first. |
| Same-Run steering (retained) | Remove only custom hash binding and proven duplicate setup; keep steer queue/modes, native cancel/drain/checkpoint/history/resume, API/UI and tests. | User instructions change the active same Run; queue order, one-time consumption, approval/question recovery and no-blind-replay survive. D05. |
| Product-specific admission | Generic-core dependencies on companion/toolkit/Work capabilities | Optional selected features own their actual behavior. Delete old mandatory-core wiring, state and error paths; feature retention is distinct from retaining its old integration. |

Do not delete `operationFlights`, durable claims, permissions or call-ID correlation simply because they are maps/checks. Each has a concrete effect/replay responsibility. Conversely, do not keep an obsolete nudge class because a useful test helper or failure classification currently lives inside it: move only that still-needed behavior into the existing tool boundary, then delete the old owner and terminology.

## 5. Compatibility must have an end

The final supported runtime must have one active format and one execution path. A compatibility reader, fallback, ignored option or old protocol left indefinitely is an incomplete deletion.

Data changes require a finite decision before their implementation cut:

1. Name supported source versions and the new format. Inventory actual source columns/keys and whether the underlying content exists; a digest cannot be reversed into missing data.
2. Convert supported data at an explicit upgrade boundary. Pending approvals whose intended input cannot be recovered must expire and require new approval. Unknown side-effect outcomes must never become automatic retries.
3. Remove obsolete columns, indexes, constraints, handlers, old readers and fallback branches from the target runtime. If an upgrade helper is temporarily necessary, specify its separate delivery boundary and removal milestone. Do not call the final contraction complete while it remains an unspecified permanent dependency.
4. Verify fresh install, supported upgrades, reopen, failure rollback and SQLite/Postgres parity on disposable fixtures. Include BML as its own module and data format. No production-state reset is authorized.
5. If the owner requires even old SQL text to leave the current source tree, establish a new schema baseline and a finite supported upgrade route before retiring old migration files. Do not modify already released migration contents in place to conceal removed fields.

Opaque existing IDs that happen to look like hashes, historical Journal payload bytes and Git history are different from active hash computation/validation. Decide whether existing IDs remain opaque or are migrated with their references. Historical user data and Git history are not silently erased to obtain a clean search. Dated research/log records may describe what was removed, but must not remain current build/development requirements. Current contracts and entry points must have one authoritative new direction.

## 6. Execution order and ownership

This plan does not introduce a new deletion framework, validator service or permanent approval board. Use ordinary PRs, existing generators, existing test entry points and a short disposition table in each coherent change.

1. **Freeze scope and audit it.** Read-only subagents inspect disjoint domains; the lead resolves cross-domain identity, persistence and failure behavior. The current five-lane inventory is the first pass. Close each cut's open decisions and re-audit changes since the pinned baseline before implementation.
2. **Replace contradictory requirements in the same cut.** Update active architecture/skill/schema requirements with the removal, so another agent is not instructed to rebuild it. Keep instructions short; replace rules rather than append exceptions to contradictory rules.
3. **Delete isolated, closed families first.** Event audit and proven dead structures are small candidates. Warming is retained: remove its hash-only diagnostics while preserving its real optional call/accounting. Do not begin by renaming every shared `Digest` type.
4. **Delete the completion/hash protocol end to end.** Change producer, event schema, SDK/TUI/headless/Web consumers and fixtures as one cut. This removes a concrete extra completion authority and provides a narrow causal regression target.
5. **Simplify nudge coordination while retaining coaching.** Locate outcome classification at the actual tool boundary, replace hashed repetition counting with exact tuples, and remove only duplicate consumer handoff; preserve reminders/middleware/events. Preserve operation/recovery proofs. Reassess observer settlement after this removal rather than adding another synchronization workaround.
6. **Delete assembly provenance machinery end to end.** Simplify Descriptor/Recipe/manifest/contracts and their generators before regenerating official artifacts. No manual generated-output-only cleanup. Keep the agreed optional final label outside runtime identity.
7. **Complete persisted identity/approval and feature-specific cuts.** Apply the finite format decisions, including BML and migration metadata. Prioritize remaining active hash gates and whole-feature deletion where the feature itself is no longer wanted. Remove old public API/config states as part of each cut.
8. **Audit the resulting candidate twice, independently.** A behavior reviewer and a residue reviewer inspect the same final commit. Neither treats implementation claims as verification. The lead integrates corrections and reruns only affected evidence; every shipped cut passes its applicable gates, with an overall final pass for contraction completion.

Use one active write lane per checkout. Read-only auditors may run concurrently; independent implementation lanes need their own worktrees and must not edit shared root files. After substantial implementation, prefer fresh reviewer subagents that did not author the deletion. No post-deletion audit has happened yet.

### Existing Eino capability check

The audited pin is `github.com/cloudwego/eino v0.9.13`. `adk.NewChatModelAgent`, `adk.NewRunner`, `Runner.Run`, `ResumeWithParams`, `ChatModelAgentConfig.MaxIterations` and `compose.ToolsNodeConfig` already cover iteration, recovery and tool integration. `UnknownToolsHandler` is available for unknown tool calls. Keep the existing runtime/provider import boundary. No finding justifies a new runner, scheduler or generic plugin framework to replace removed coordination.

Native callbacks alone do not promise Vivy's synchronous durability semantics. Preserve the minimum necessary effect/accounting boundary, or explicitly change its contract; do not assume that substituting a callback proves equivalent recovery.

## 7. Independent audit A: behavior has not been broken

The acceptance anchor is a causal **model -> real tool -> tool result -> next model request -> final answer** through production construction. Counting events, a mock answer, or a green compilation is insufficient.

| Scenario | Required evidence |
| --- | --- |
| Minimal causal exchange | A real read tool reads a fresh nonce from an isolated file. Capture the second model request and verify the exact result/call ID; final answer follows, one terminal is reported and the Run becomes idle. |
| Real-provider human check | The tiny chat Face uses the same Service/RPC/tool path with configured real tokens; retain the call/result/continuation evidence without secrets. This remains pending, not replaced by stub success. |
| Error correction/refusal | Missing file, invalid arguments and denied effect reach the model appropriately; correction can proceed, denied mutation never occurs. |
| Effect crash/recovery | Preserve `TestToolOperationCrashMatrix`: admitted, claimed, effect performed and completed boundaries. Unknown effects are not blindly repeated; completed effects replay their outcome. |
| Approval and question resume | Correct call/arguments/Run binding, completed siblings not repeated, decided call continues, no old hash envelope dependency. |
| Streaming and Faces | Multi-step tools, Unicode, valid chunk boundaries and tool-only output reach consistent SDK/TUI/headless/Web final state without `content_sha256`. No false local protocol failure. |
| Cancellation and derived output | Cancel releases remaining waits; a slow/failed optional sink does not own core progress. Distinguish the already nonblocking Web Bus from synchronous sinks. |
| Retained editing/download/feature identity | Selected content stays consistent across chunks; edit conflicts, retry IDs and authorization remain as explicitly specified. Cover actual race/retry boundaries, not only happy paths. |
| Data transition | New and upgraded disposable stores reopen correctly; unsupported pending state is explicit; no lost records or accidental duplicate side effects. Include the independent BML module. |

Reuse meaningful existing test helpers and retain reminder/count/template/eligibility/steering assertions for the three confirmed features. Remove only hash-specific and superseded handoff assertions. Do not retain redundant coordination solely to keep old tests green, or delete business acceptance tests because their filenames mention a retained or retired mechanism.

Developer iteration may use the affected Recipe and shared-contract checks described in the [development proposal](../research/2026-10-19-agent-loop-slimming-review.md#11-recipe-based-development-and-pr-responsibility). The maintainer's final candidate still requires the current repository `just ci`, applicable builds and real-path smoke until the validation contract is explicitly changed. This document is not permission to suppress unrelated failing checks or declare a skipped gate passed.
## 8. Independent audit B: nothing retired is still active

This audit is separate from behavior verification. It follows consumers and generation, not just imports. The reviewer records every remaining match and rejects unclassified residue.

| Surface | What must be absent after its retirement cut |
| --- | --- |
| Implementation | Old algorithms/helpers/classes, constructors, hidden fallback, always-success verification wrapper, unreachable branch, disabled experimental implementation |
| Protocol/config | Old JSON/YAML/SQL fields, aliases, required-schema entries, event versions, error codes, CLI flags, ignored options, environment variables |
| Storage | Unused columns/indexes/tables, backfill jobs, digest-derived keys still produced, permanently supported old reader |
| Generation | Old descriptor fields/constants, templates, generator expressions, golden outputs, overlay fixtures, artifact fields restored by rebuild |
| Tests | Assertions demanding the old mechanism, obsolete fixture directories, feature-only mocks/helpers, tests that preserve an unused table solely for compatibility |
| User/developer surfaces | UI controls/status/translation keys, API DTOs, example recipes, source-hash CLI, mock/demo state and active tutorials |
| Instructions | Current AGENTS/skills/architecture/CI rules that require rebuilding the retired mechanism |

Use both exact symbols from the deletion diff and broad aliases. Initial search vocabulary:

```text
sha256 sha512 md5 hash/fnv crypto/hmac digest checksum fingerprint
content_hash base_hash precondition_hash RequestDigest ArgumentsDigest
SourceSHA256 SourceHash DependencyLockHash LockHash AssetHash FinalAssetHash
RecipeDigest GenerationID catalogProjectionDigest VerifyUnchanged
content_sha256 completionHash VerifyModelCompleted ModelCompletionTracker
AuditHook AuditRecord SlogAuditSink payload_sha256
newNudgeState restoreResumeNudgeBatch completedCall takeCompletion tool.nudge
runCacheWarmer CacheWarming cache_warming CacheWarmed cache.warmed
memory_apply_journal inofy-normal-v1:sha256:
```

These are discovery terms, not a global text replacement. Nudge/warming/steering feature names, events, settings and meaningful tests MUST remain; audit their retired hash fields and redundant handoff helpers rather than demanding their lexical absence. `GenerationID` still needs a non-content-addressed identity if Generations remain; ordinary URI fragments and natural-language summaries are not hash machinery. Existing opaque historical IDs, historical records and approved native dependency/security internals must be classified by exact purpose. An unexplained match fails closure; a broad wildcard exemption such as “all tests” or “all compatibility code” is not acceptable.

Regeneration is mandatory for affected outputs: `internal/generated/assembly/generate.go` invokes the default SDK generator; UI predev/prebuild/pretest invokes `ui/scripts/stage-ui-assembly.mjs`; DIVA/memory-loop scripts create generated overlays. Fix their source, regenerate, inspect the produced diff, and regenerate again to establish no further changes. Build and inspect affected official Recipe artifacts, including minimal and plugin examples; final contraction must account for all supported official Recipes and build variants. The current inventory has not proved all tag/submodule combinations.

Specific resurrection points include `sdk/internal/cmd/source-hash/`, `scripts/check-plugin-v1-fixtures.mjs`, Descriptor SHA constants, conformance evidence files, `sdk/internal/memory_loop_generation_test.go`, `scripts/memory-loop.mjs`, `scripts/diva-recall.mjs`, and UI demo CAS. Remove the obsolete producer or rule, not merely its generated answer.

Active requirements needing revision include `VIVY-ASSEMBLY.md`, `VIVY-PLUGIN-SPEC.md`, `VIVY-MODULE-STANDARD.md`, `.agents/skills/vivy-plugin/SKILL.md`, and AGENTS source-evidence/migration-checksum clauses. Preserve operative `NUDGE-DESIGN.md`, nudge plans and warming/steering feature requirements; revise only hash and redundant-coordination clauses with implementation. `VIVY-MEMORY-HOST-CONTRACTS.md` cites `AuditRecord`; replace the obsolete dependency while keeping its bounded-data requirement. Historical delivery logs can remain factual history; they cannot govern future implementation.

## 9. Decisions and release closure

Sections 10–11 now fix original-request binding, finite source formats and
recovery, release/consent identity, private content ownership, unsigned cursor
trust, Generation identity and the native boundary. Nudge/warming/steering remain
selected EXP capabilities; retained product features are not silently deleted.
Keep existing dynamic tool discovery semantics in this cut; delete only proven
duplicate authority, without introducing a fixed Run surface. Technical source
inspection is complete for the recorded pins; upstream contract delivery,
converter fixtures and implementation evidence remain release prerequisites.

A deletion deliverable is closed only when its implemented candidate, retained behaviors, data transition, active-rule changes and both independent audit results are recorded. Evidence must identify the actual commit/build tested. A failing check is not waived by a passing residue scan; a passing CI is not proof of deletion completeness.

Plan verification: assembly and storage auditors independently reviewed the integrated draft; their scope corrections are incorporated. Documentation diff checks passed. Product CI was not run because this delivery changes research/planning text only; this is not a waiver for implementation.

Current status: five read-only domain inventories integrated; broad tracked-tree search completed; detailed technical decisions are recorded in §11; implementation, regeneration, converter delivery, behavior verification, real-token E2E and post-deletion residue review are **pending**. No runtime fix, performance gain or incident root cause is claimed.

## 10. Executable-plan design amendment

Planning baseline: branch `9c5da97e3292591fa960b8cd14d921c17fea5812`;
freshly fetched main `e96092966cfbfa443648374948e7e7e16dd01683` on 2026-10-19.
There is no product-code delta from the original audit. Two read-only planning
reviews rechecked runtime/approval and assembly/persistence. They did not run
product tests or inspect production data.

Choose direct comparison of already-owned values, existing semantic revisions
and ordinary opaque identifiers. Retain the existing Eino runner and private
durable operation row. The alternative of moving hash checks into plugins,
adding token/state registries, or inventing a new receipt/evidence layer retains
or increases the rejected coordination cost. Raw private input can cost more
storage than a digest; that cost is justified only where the input is necessary
for approval, retry conflict detection or a retained snapshot. Do not replace
every diagnostic hash with another stored copy. No speedup is claimed.

### 10.1 Shared replacement contracts

| Contract | Concrete proposed target | Owner / implementation Story |
| --- | --- | --- |
| C1: stream completion | Emit `model.completed` payload v3 with exactly `{ "byte_len": N }`, where N is the UTF-8 byte count of the ordered text deltas for that model step. No full-text duplicate or digest. An emitted empty final completion has N=0. Preserve the current `tool.requested` phase boundary for tool-call responses: do not add a `model.completed` for a tool-only/preamble tool-call step. Keep live/replay budget charging and final-message persistence unchanged; a completion is not a Run terminal. New code accepts v3 on the live wire. Historical journal projection ignores digest metadata without validating it. At RPC `replayEvents`/`streamRun`, normalize supported historical v2 completion metadata to v3 before sending it to strict Face consumers, preserving sequence/count and never rewriting the Journal. G2 must enumerate any older historical shapes and their supported read path; no second live v2 protocol is admitted. | Runtime mapper, Face consumers; D02 |
| C2: tool operation | Add private `RequestArguments []byte` to `domain.ToolOperation`; retain `MiddlewareInputArguments`, `EffectiveArguments`, RunID/OperationID/ToolName, claim owner/state and durable outcome. `Lookup` compares original request bytes, not effective bytes. Persist first, then return a completed outcome to Eino. Remove RequestDigest/ArgumentsDigest and digest event fields. Different raw requests under the same operation ID are a conflict; do not introduce JSON normalization into this retry identity. Add durable OutcomeStatus/OutcomeReason/OutcomeEffects fields in this same row (success/recoverable/refused/fatal; existing reason vocabulary; not_executed/none/unknown). Result/Failure retain their text roles. Completion stores all fields atomically and replay uses those typed values without string-prefix guessing. A converted row lacking recoverable original input keeps RequestArguments=NULL and its original state and Result/Failure; new typed outcome fields remain NULL only on unbound legacy rows, and are required on new completed bound rows. Lookup/Execute returns explicit ErrToolOperationBindingUnavailable before claim or replay. New admissions require nonempty valid JSON within the existing 1 MiB argument bound; do not invent a recovery-required operation state. | Existing operation store; D03 |
| C3: approved invocation | Replace the private approval envelope with `vivy.tool-approval-proposal/v2`, carrying canonical intended arguments and provider data. Use the existing JSON canonicalization semantics, preserve numeric literals, bind existing Run/call/tool/target/checkpoint identifiers, and compare the resumed intended invocation exactly. Reuse the operation row where applicable; non-row orchestration/Work approvals retain their existing private owner. Canonical arguments never enter public event/Review DTOs merely to replace a hash. | Existing approval/operation owners; D03 |
| C4: source and Generation | Source selection retains Module ID, native ref, trust, explicit root and typed binding; remove all Vivy-computed source/lock hashes and `source-hash` CLI. Allocate one opaque Generation ID at `pack`; pass that same ID to Go/UI generation, manifest and Inspect. Authoring generation uses an explicit stable profile ID; pack always overrides it. Regeneration with identical explicit inputs is idempotent. No content-equality or authenticity claim attaches to the ID. | SDK catalog/compiler; D06. Baseline A: `GenerationID` is currently hash-derived, so the opaque pack ID is a design change rather than a rename. |
| C5: bundle identity and consent | Acquisition allocates an opaque ArtifactID; published identity is existing `(InstallID, ReleaseSequence)`. Keep existing catalog Epoch/ConfigRevision CAS. Resource paths and declarations belong to the immutable published release; remove content-addressed object names and deduplication. Bind exact selected declaration and configuration to release identity; a changed release/config requires new consent. Converted executable consents are unarmed; §11.7 defines scoped revocation and history retention. | Bundle manager; D07, decision G3. Baseline A: no bundle/toolkit subsystem exists (no `internal/bundles`, `bundlecontract`, `bundlesql` or toolkit packages); this row has no implementation to cut and D07 is withdrawable pending owner confirmation. |
| C6: admitted inputs and checkpoints | One immutable Run-owned prompt/tool/policy snapshot contains actual admitted values. Use existing RunPromptSnapshot RunID/SchemaVersion/ComposerVersion/GenerationID, not an invented admission revision. New checkpoint outer format v2 contains format version, Eino engine version, RunID, snapshot schema/composer/Generation identity and created time, followed by opaque payload. No checksum or prompt digest. Engine mismatch, foreign Run or missing admitted snapshot fails closed. Blob generation/pointer atomicity remains. | Admission and checkpoint stores; D08 |
| C7: history and selections | Cursor v2 is bounded, stateless and entirely untrusted: phase/after/cut positions plus normalized request/scope values, not authority claims. Every page derives permitted sessions from server context, compares normalized request values, checks positions against an authorized cut, and reapplies current visibility/truncation. Remove HMAC, random key/fallback, ScopeHash, FilterHash and SelectionDigest. Reference attachment binds selected source positions plus the bounded selected text shown in preview; re-read authorized values and compare exact selection before committing the destination copy. No token-to-state cache. Section 11.6 fixes cursor trust and restart/snapshot semantics. | Existing History/Reference services; D09 |
| C8: edits and downloads | File proposals/revisions bind exact expected bytes (including missing-versus-empty) through existing private payloads. Compare the reopened authorized file against those bytes before mutation; retain safe-open and race checks. Downloads use ItemID + existing TransferID and one immutable bounded scratch snapshot. Compare against the presentation-time captured bytes before opening a transfer; every chunk reads the same snapshot. Private presentation bytes are committed atomically with the presentation receipt under the existing ContinuityStore; retain them until the source session is deleted or its presentation is hidden by truncation, then prune in that existing lifecycle. Active transfer scratch retains the existing 60-second idle TTL; presentation bytes do not inherit that TTL. Missing/expired private presentation bytes yield explicit unavailable/re-present, never current-file acceptance. Reuse storage/transfer ownership; do not claim mtime/size prove content equality. Persisting bounded presentation bytes is the deliberate storage choice in §11.6. | File/Deliverable services; D09 |
| C9: feature CAS and retries | Staged Skill mutations use existing BeforePayload and exact current bytes; direct SetSkillEnabled stays a control-plane toggle and receives authorized expected SKILL.md bytes/existence instead of baseHash, with a read-before-submit UI flow and conflict refresh; managed memory/notebook/mask/toolkit values use existing owner revisions and exact admitted declarations. Workflow/Work/report operations retain request IDs and normalized intended inputs in their existing private receipts. Equal ID with unequal input is a conflict. Remove digest metadata, not idempotence or scope isolation. Remove Vivy calls that compute INOFY digests; external opaque values are never revalidated as Vivy evidence. If a pinned upstream API requires a digest authority that cannot be adapted, D11 is blocked for an upstream contract change. | Feature owners; D07/D10/D11. Baseline A: the report family and toolkit contracts are absent; `notebook` maps to the notes domain/tools/storage. Re-scope D11 accordingly. |
| C10: BML | Proposed schema v2/artifact `2.0.0`: strip digest fields from record JSON, preserve existing opaque record/workspace IDs and all links/scopes/revisions/tombstones; new IDs are ordinary opaque IDs, never content/path derived. Existing store revisions own CAS. Imports use migration ID + original source identity and exact raw source input plus normalized records for conflicts. Delete unused memory_apply_journal. Workspace association comes from persisted ownership, not path hashing; handle relocation/collision explicitly. | Independent BML module; D12 |

The exact values above are target contracts, not an assertion that their
producers exist. Section 11 resolves their technical design; the index records
accepted predecessor evidence before any dependent Story becomes Ready. Tests named in Story plans are proposed unless
explicitly identified as existing.

### 10.1a C11: Explicit species-selected experimental packages

This owner-approved amendment affects only nudge, warming and steering. The
other cuts and existing technical/data/security gates continue as planned.

| Proposed Module | Proposed source path | Selected behavior |
| --- | --- | --- |
| vivy/exp-nudge | `plugins/exp/nudge/` | Real threshold/window/template coaching and scheduling event |
| vivy/exp-warming | `plugins/exp/warming/` | Actual eligible, budgeted, accounted maintenance request |
| vivy/exp-steering | `plugins/exp/steering/` | Active instruction changes with the same RunID and durable queue semantics |

Use explicit Recipe source selection and generated Assembly. All unselected
Recipes, including minimal, omit package implementation, constructors, feature
state reads, registration, provider work, RPC/control actions and UI controls.
Shared historical event readers can still display old records without activating
these features. A request for an uncompiled capability reports NOT_COMPILED;
never silently accepts a live setting/action and does nothing. For selected
packages, existing config determines runtime behavior (e.g. warming off versus
streaming). No runtime directory scanning or unconditional engine imports.

The generic core retains tool feedback, authorization, exact approval binding,
durable effect outcomes, mandatory paid-call accounting, cancellation and native
Eino checkpoint/resume. Package-owned algorithms/policy/UI move out; only the
small selected Host/Eino adapter remains in runtime/provider. Experimental code
must not import quarantined Eino or access raw Journal/Policy/storage/credentials.
Custom hashes are deleted throughout, including these experimental packages.

Baseline A findings: the three features exist only as unconditional core code
(`engine.go` around line 376, `service.go` around lines 2948/4643, `queue.go`);
nudge has no config/UI, warming is config-only with off/streaming/idle (idle
uses a timer), steering has RPC/UI but no located custom hash binding. The
loop-detection hard stop fused with the nudge window in
`internal/runtime/loopguard.go` remains core continuation authority; the epics
extract coaching/behavior, not the run-fatal stop. C11 is the target contract
delivered by the epics, not by #91.

Verified seam gap: sdk/port/observer is bounded post-commit observation and
sdk/port/pretool only constrains pre-tool execution. Neither supplies these
three policy boundaries. Sections 11.1–11.5 resolve the gap with three closed
T1-only factory Ports, pure SDK contracts and selected runtime adapters. No
Observer privilege escalation, untyped registry or raw Service/Runner exposure
is permitted. D04/D05 consume that settled design and accepted predecessors. The package implementations are delivered by the independent E-NUDGE, E-WARMING and E-STEERING epics; #91 lands only the seam and the hash/handoff cuts.

Proposed explicit test Recipes live at recipes/exp-nudge.vivy.yml,
recipes/exp-warming.vivy.yml, recipes/exp-steering.vivy.yml and
recipes/exp-loop-features.vivy.yml. Verify none/each/all selection, real behavior
and physical omission with the simplified SDK generation from D06. D13 documents
runnable examples and D14 independently accepts that matrix. An existing paused
steer opened by a species lacking the package is explicitly unavailable/recovery-
required under G2; it must not instantiate a hidden legacy steering path.

### 10.2 Finite data transition

Section 11.7 fixes the finite support matrix and §11.9 the delivery boundary.
The conversion route is:

1. Quiesce one organism and back up its database and owned artifact directories;
   do not migrate a live Run or operate on tenant data during development.
2. A separately delivered, finite upgrade tool uses the **old release** to
   normalize currently supported legacy SQL shapes to baseline 035. Existing
   legacy PostgreSQL markers 13-23 and released-017 repair fixtures remain
   required in that tool; do not silently remove their support. The old release
   may validate its old formats, but the final runtime ships no old hash reader.
3. At that boundary convert exact baseline SQL/JSON/object formats on copies.
   The proposed source set is SQL 035 with baseline metadata, BML schema 1 /
   artifact 1.0.0, and the baseline checkpoint/approval envelope. Newly added
   migration IDs are allocated serially above 035 after rechecking the head
   (baseline A's migration head; the 10-18 tree reached 043, which is not
   present here).
4. Expire baseline pending/approved-unconsumed v1 approvals with explicit
   reapproval-required recovery; do not infer intent from old evidence. Preserve
   admitted/claimed/completed operation rows. An unresolved claimed effect
   remains unknown and is never automatically invoked again. Old paused Eino
   checkpoints are not promised convertible: recommendation is explicit
   recovery-required/reapproval with the durable effect fence intact, under
   the explicit terminal/recovery mapping in §11.7. Never claim this is seamless resume compatibility.
5. Stage copied bundle artifacts under new opaque IDs, prepare the complete
   legacy-reference map and commit DB reference changes last. Retain the backup
   until reopen/recovery/rollback checks pass. Handle copy failure, crash and
   disk-full without deleting the previous store.
6. Target runtime opens only the contracted final data formats; no dual
   checkpoint/approval/record parser or digest-directory fallback. Its migration
   metadata is version/name/applied_at, without checksum. Change metadata
   bootstrap and runner together: old `ensureMetadata` requires name/checksum
   as a pair before numbered migrations run. Merely dropping the column in a
   later migration fails on reopen.
7. Released SQL files are immutable historical transition definitions. They may
   contain old column names; final schemas and active Go readers may not.
   If the owner requires old SQL text absent too, approve a new schema baseline
   and archive the old migration delivery separately; never rewrite releases.
   Retire the conversion helper from the final candidate/release tree after
   delivering the finite upgrade artifact. Record both exact artifact commits.
8. Data rollback uses the matching backup and old executable while quiescent;
   an old binary must not open a converted store. Preserve post-cutover data
   separately rather than silently overwriting it with the pre-upgrade backup.

No installed-version census, converter test or downgrade proof exists in this
planning delivery. The source matrix is explicit; implementation evidence and
finite-artifact delivery are required before release, without resetting state.

### 10.3 Proposed core contraction and preserved boundaries

The owner explicitly retains nudge/reminder coaching, proactive warming and
same-Run steering as species-selected experimental packages (2026-10-19). D04 keeps the real reminder/window/templates/event
and removes custom signatures plus redundant consumer-owned result handoff.
D05 keeps real optional warming and same-Run steering, with their settings,
controls, queue modes, recovery and accounting. Replacing these with ordinary
error text, a never-called warmer, or cancel/new-Run/follow-up-only behavior is
not accepted. Nudge keeps its ten-step exact tuple window, thresholds 3/5,
1024-byte/context limits and schedule-before-injection durability; native
post-tools ordering replaces only the duplicate handoff. Warming keeps current
opt-in/eligibility/budget/timeout/paid-accounting behavior and loses hash-only
prefix diagnostics. Steering keeps the same RunID and native cancel/drain/
checkpoint/resume contract. Keep existing tool visibility/discovery semantics; remove proven duplicate
authority only. Section 11.4 fixes derived projection ownership; a fixed tool
surface is not introduced.
Companion, workflow, Work, memory and notebook capabilities also remain retained
unless separately authorized. Delivery split (2026-10-19): the package
implementation, parity, settings/UI/events/recovery, evidence and final deletion
of the core bodies are delivered by the independent epics E-NUDGE, E-WARMING and
E-STEERING; #91 keeps the single core implementation until each epic cuts over,
and no dual path is permitted at any point.

For D04, retain synchronous acknowledgment of admission, permission/approval,
effect claim/outcome, final message and terminal state. Move recoverable tool
failure classification to the existing tool result boundary. Remove only event-
consumer handoff acknowledgment as permission for the next model request. Keep
actual nudge computation and the existing scheduling-event append before
injection; changing that durable requirement is not authorized by retention.
Derived transcript/status views may lag committed execution and rebuild through
existing Journal replay; a failed optional sink must not own the runner.
Never make hard budgets, required audit/accounting facts or durable effect
completion best effort. Section 11.4 fixes this durability/visibility boundary without weakening mandatory commits.

Actual tool lifecycle originates at the existing invocation owner. A refused
call has no fictitious executed-start timestamp. Do not add an event scheduler,
unbounded fanout, or second transcript service to achieve this contraction.

### 10.4 Eino capability check and external boundary

Repository pin: `github.com/cloudwego/eino v0.9.13`. Fresh primary-source reads:
[adk/runner.go](https://github.com/cloudwego/eino/blob/v0.9.13/adk/runner.go),
[adk/chatmodel.go](https://github.com/cloudwego/eino/blob/v0.9.13/adk/chatmodel.go),
[compose/tool_node.go](https://github.com/cloudwego/eino/blob/v0.9.13/compose/tool_node.go).
`NewChatModelAgent`, `NewRunner`, `Runner.Run`, `ResumeWithParams`, native
checkpoint interfaces and `ToolsNodeConfig` already supply loop/tool/resume
composition. Existing `engine.go` sets `MaxIterations`. Their availability does
not prove Vivy durability; D03/D04 retain the existing effect fence. No new
runner or scheduler is proposed. Eino imports stay in runtime/provider only.

G5 proposes retaining native Go/pnpm dependency integrity, TLS/authentication
and bundler-managed resource naming. Vivy-owned source/evidence/asset business
checks, custom cursor MAC and explicit digest calls into INOFY are in scope for
deletion. A native Git commit ref may select source without being recomputed
as a Vivy product hash. The optional final-distributable informational hash is
omitted by default; adding one later is unnecessary to satisfy #91.

Host-side `internal/studiocore` hash gates are included. The external Studio
submodule and pinned Laputa/INOFY APIs were inspected for actual consumer obligations
(§11.10); this repository's plan cannot assert those repositories
have been changed or accepted. Escalate a required external contract change;
do not retain a local no-op verifier or silently fork the dependency.

## 11. Detailed target architecture

This section resolves the technical choices behind C1–C11; it is a design, not
implemented behavior. The owner has settled the deletion scope and the three
retained experiments. Use the existing Run, operation, revision and release
owners with direct values; add only three closed selectable policy factories
and their narrow runtime adapters. Keeping everything in core would fail
physical omission; giving arbitrary plugins a Runner/Service would duplicate
execution authority. Pure EXP policies plus generated, trusted adapters cost a
small typed boundary while preserving the existing execution owner. No generic
extension registry, new scheduler, second queue, snapshot service or receipt
framework is introduced. The index owns delivery state and dependencies.

### 11.1 Ownership and dependency direction

| Layer / proposed or existing path | Owns | Must not own |
| --- | --- | --- |
| Existing `internal/runtime/`, domain, policy and storage | Run admission, actual intended input, effect claim/completion, Journal, budgets, native Eino execution/cancel/resume and terminal state | EXP detection/eligibility/routing algorithms |
| Proposed `sdk/port/loopexp/{nudge,warming,steering}.go` | Eino-free policy DTOs and factory signatures below | Runtime imports, execution callbacks, registration or executable default policy |
| Proposed `plugins/exp/{nudge,warming,steering}/` | Their pure policies, descriptors and selected UI/settings declarations | `agent-vivy/internal/*`, Eino, raw Journal/storage/Policy/model/credentials |
| Proposed `internal/runtime/extensions.go` | Typed optional factory slots, bound Host contracts and opaque observed-input/prefix handles | Importing its adapter subpackages or EXP implementations |
| Proposed `internal/runtime/extensions/{nudge,warming,steering}/` | Translate selected policy decisions into current native middleware/model/queue calls through bound Hosts | Run ownership, arbitrary model execution, iterator consumption or terminal handling |
| Existing SDK catalog/compiler/generators and generated Assembly | Validate exact source/trust/owner/cardinality; emit selected factory imports and typed bindings | Runtime discovery, self-declared T1 trust or generated files edited by hand |
| Existing app and Face assembly | Install typed selected contributions and capability-derived controls | Feature constructors hidden in unconditional startup or frontend-only feature lists |

Dependency direction is `EXP -> sdk/port/loopexp`; selected adapters import that
SDK and root runtime; generated Assembly imports selected adapters and EXP.
Root runtime imports neither EXP nor its adapter children. Only
`internal/runtime/` and `internal/provider/` import Eino. This keeps both
`TestEinoImportsQuarantined` and `TestPluginWindowCannotImportInternal` valid.
The inert SDK types and optional factory slots may exist in minimal; algorithm,
feature state, adapter construction and feature registration must not.

### 11.2 Selection, trust and construction

Extend the existing **closed** core catalog in `internal/moduleport/ports.go`,
not the external `PublicCatalog`:

| Port | Exact sole owner | Cardinality | Pure SDK factory |
| --- | --- | --- | --- |
| `core/nudge-factory@v1` | `vivy/exp-nudge` | 0..1 | `loopexp.NudgeFactory` |
| `core/warming-factory@v1` | `vivy/exp-warming` | 0..1 | `loopexp.WarmingFactory` |
| `core/steering-factory@v1` | `vivy/exp-steering` | 0..1 | `loopexp.SteeringFactory` |

The build-owned SourceCatalog assigns T1 only to these exact canonical sources
and import/selectors. Recipe/descriptor text cannot promote a source to T1.
External T2 sources cannot provide the Ports, impersonate an owner or replace an
adapter. Recipe selection uses the simplified C4 source/module/ref/root identity;
no source-tree digest is retained to enforce this trust class. Do not trust a
module name supplied by an arbitrary root without catalog provenance.

Extend `sdk/internal/assembly/source.go` GoBinding and
`sdk/internal/assembly/runtime_generate.go` using their existing named-factory
pattern. Selected generated bindings have the form
`nudgert.Factory(expnudge.New)`, `warmingrt.Factory(expwarming.New)` and
`steeringrt.Factory(expsteering.New)` in typed root-runtime factory fields.
`Factory` in each adapter package accepts its corresponding SDK factory and
returns the root-runtime factory type below. Existing-style typed
`HasNudgeFactory`/`NudgeFactoryValue`, `HasWarmingFactory`/`WarmingFactoryValue`
and `HasSteeringFactory`/`SteeringFactoryValue` accessors expose false/nil when
absent. Proposed `internal/app/assembly_loop_extensions.go` performs typed
unwrapping only; it imports root runtime, never an EXP implementation.

Generate backend capability projection and selected settings/actions/UI from the
same admitted Module set. Unselected features contribute no config reads,
middleware, state allocation, RPC/action registration, provider work or UI
controls. A feature-specific request to an unselected species returns
`NOT_COMPILED`; generic cancel, approval/question recovery and follow-up remain.
Selected warming can be configured off; that is distinct from not compiled.
Unselected historical-event rendering is passive and cannot instantiate a
feature. Build the none/each/all-three Recipes listed in C11, inspect generated
imports and resulting artifacts, and test actual behavior in selected builds.

### 11.3 Exact policy and Host interfaces

These are **proposed signatures**, not existing APIs. SDK contracts contain no
runtime/domain/Eino import and no execution authority:

```go
// sdk/port/loopexp/nudge.go
 type NudgeFactory func() NudgeDetector
 type ToolSample struct {
     CallID, ToolName string
     Arguments, Result, Error string
     Status, Reason, Effects string
 }
 type NudgeNotice struct {
     CallID, ToolName, Status, Reason string
     Count int
     TemplateVersion, Text string
 }
 type NudgeDetector interface {
     Evaluate(orderedFresh []ToolSample) (NudgeNotice, bool)
 }

// sdk/port/loopexp/warming.go
 type WarmingFactory func() WarmingPolicy
 type WarmingInput struct {
     Mode string
     SupportsWarming bool
     CacheLifetimeSeconds, PrefixTokens int
     InputPerMTokens, CachedInputPerMTokens, MinSavingsUSD float64
 }
 type WarmingRequest struct {
     Prompt string
     MaxOutputTokens int
     TimeoutMillis int64
 }
 type WarmingDecision struct { Reason string; Request *WarmingRequest }
 type WarmingPolicy interface { Decide(WarmingInput) WarmingDecision }

// sdk/port/loopexp/steering.go
 type SteeringFactory func() SteeringPolicy
 type SteeringEligibility struct {
     Suspended, HasBoundaryCancel, PreferencesCompatible bool
     HasAttachments, HasFileContexts, HasContinuity bool
 }
 type QueueTrack string // "steer" | "follow_up"
 type QueueMode string  // "all" | "one-at-a-time"
 type SteeringPolicy interface {
     Route(SteeringEligibility) QueueTrack
     Take(QueueMode, int) int
     Render(selectedTexts []string) string
 }
```

ToolSample status/reason/effects are a lossless projection of C2's typed durable
outcome, with explicit enum-to-string conversion at the adapter. No consumer
parses Result/Error prefixes to infer authority. The existing bounded tool
argument/result representations feed the exact tuple; no extra unbounded raw
result copy or hash is introduced. `WarmingDecision.Request == nil` means skip.
Steering policy cannot select records, change Run identity or grant permission.

Root runtime contracts stay inside its import quarantine:

```go
// internal/runtime/extensions.go; runtime imports adk, domain and loopexp.
 type NudgeFactory func() NudgeExtension
 type NudgeExtension interface {
     Middleware() adk.ChatModelAgentMiddleware
     BindLeg(context.Context, NudgeLegHost) context.Context
 }
 type NudgeLegHost interface {
     Outcomes(callIDs []string) ([]loopexp.ToolSample, error)
     Schedule(context.Context, loopexp.NudgeNotice) error
 }
 type WarmingFactory func(MaintenanceHost) ModelSettlement
 type ModelSettlement interface {
     Settled(context.Context, ObservedModelInput) error
 }
 type MaintenanceHost interface {
     ReusablePrefix(ObservedModelInput) (ReusablePrefix, error)
     PolicyInput(ReusablePrefix) loopexp.WarmingInput
     GenerateMaintenance(context.Context, ReusablePrefix,
         loopexp.WarmingRequest) (MaintenanceResult, error)
     Diagnostic(context.Context, MaintenanceDiagnostic)
 }
 type SteeringFactory func(SteeringHost) SteeringController
 type SteeringController interface {
     Steer(context.Context, domain.QueuedTurn) (domain.QueuedTurn, error)
     SetMode(context.Context, loopexp.QueueMode) error
     OnBoundary(context.Context) (handled bool, err error)
 }
 type SteeringSnapshot struct {
     Pending []domain.QueuedTurn
     Mode loopexp.QueueMode
     Eligibility loopexp.SteeringEligibility
 }
 type HistoryModifier func(context.Context, []adk.Message) []adk.Message
 type SteeringHost interface {
     Snapshot(context.Context, domain.QueuedTurn) (SteeringSnapshot, error)
     Enqueue(context.Context, domain.QueuedTurn,
         loopexp.QueueTrack) (domain.QueuedTurn, error)
     SetMode(context.Context, loopexp.QueueMode) error
     RequestCancelAfterModel(context.Context, string) (domain.QueuedTurn, error)
     ContinueBoundary(context.Context, []string, HistoryModifier) error
 }
```

`ObservedModelInput` wraps the current `modelCallInput` in an unexported field;
`ReusablePrefix` privately contains its Run/call-bound leading-system/tool
projection and estimated token count. Only the Host creates these handles;
adapters cannot synthesize a different prefix or model request. The Host checks
ownership before invocation. `MaintenanceResult` contains only CallID, Status,
Reason and PromptTokens/CompletionTokens/CachedTokens/CacheWriteTokens;
`MaintenanceDiagnostic` adds EstimatedPrefixTokens to that result. They replace
the corresponding existing cache-warmed payload fields, with no prefix digest.
`PolicyInput` derives prices, capabilities and settings from existing admitted
owners rather than adding another configuration store.

All Hosts bind the current Run/leg; methods accept no caller-selected RunID,
checkpoint, model, credentials, storage handle or Journal operation. Snapshot
receives the candidate queued turn so compatibility/attachment eligibility is
computed by its owner, and returns copies. RequestCancelAfterModel accepts an
already-enqueued ID and owns arming/native cancel; refusal durably reclassifies
that same ID to follow_up and propagates Journal errors. It must not call the
new-ID enqueue path again. Root runtime validates policy return enums/counts, queue-prefix
membership and native state before effects. Maintenance admission/paid-accounting
errors remain mandatory. Factory presence is the only construction seam; it is
not a generic callback bus or observer privilege escalation.

### 11.4 Tool completion, continuation and nudge lifecycle

1. Native Eino invokes the existing tool boundary. Validate original JSON bytes
   (at most 1 MiB), permissions and exact approved invocation. Admit/claim in the
   existing `(RunID, OperationID)` row before an effect. Complete result/failure,
   typed status/reason/effects atomically before returning a completed result.
   This describes ordinary operation-backed tools: Work retains its existing
   receipt/commit owner, and questions interrupt before claim. Do not add a
   second durable claim around those paths.
2. The producer records a bounded per-leg outcome only when nudge is selected,
   after the mandatory durable commit and before returning to Eino. Refused
   preclaim calls have typed not-executed outcomes and no invented start time.
   Replayed completed siblings are marked non-fresh; unknown/claimed effects
   are never automatically invoked. The durable operation remains the owner;
   the per-leg map holds only references/projections needed for this batch.
3. Native ToolsNode joins actual tool results. The next WrapModel examines the
   trailing ToolMessages in native request order and synchronously obtains
   their producer-owned outcomes. Outcomes first validates coverage of every
   requested ID (including completed siblings initialized from accepted resume
   state), then returns only fresh samples in that native order. A replayed
   sibling may bypass tool invocation; it must still have coverage. The adapter
   caches the full ID batch separately. Missing expected outcomes are invariant
   errors, never a Register/Seal/Take wait or parked completion. No new
   AfterToolCalls hook or event-consumer acknowledgment is required.
4. The EXP detector advances a maximum-ten exact `(toolName, arguments,
   result, error)` tuple window. Success advances an empty tuple without a
   reminder candidate. Preserve thresholds 3/5, highest-count winner, earliest
   native-order tie, fixed corrective/refusal templates, maximum 1024-byte
   notice and current context-budget skip behavior. Delete FNV and signature
   serialization; compare bounded values directly.
5. The adapter caches the evaluated ordered call-ID batch and notice for model
   retries; it does not count the same batch twice. Schedule `tool.nudge` once
   **before** injecting advice. Preserve call_id/tool_name/reason/repeat_count/
   template_version. Schedule persistence failure aborts that attempted model
   call. Advice never changes permission, claim state or effect replay.
6. Allocate a fresh detector on initial execution, approval/question resume and
   each steering-resume leg, matching current reset semantics. Release leg
   projections on drain/termination. The next ordinary model request uses the
   original ToolMessage call IDs/results and the optional advice in the same
   Eino path. No selected package is necessary for normal continuation.

Admission, approvals, effect completion, final message, budget/usage and terminal
commits remain synchronous prerequisites. Optional derived transcript/status
sinks may lag and rebuild from Journal; they cannot block native continuation.
The new design removes only duplicate handoff ownership. One Run still has one
terminal transition and ordinary idle cleanup. Nudge schedule persistence keeps
its existing stricter semantics; it is not made best effort.

### 11.5 Warming and steering lifecycle

**Warming.** The selected adapter receives settlement of actual foreground/child
model calls. Host derives only leading system messages and the exactly bound
tools from that observed invocation; never user/history/tool-result content.
Policy preserves off/streaming, SupportsWarming, positive cache TTL and current
min-savings gate. The heuristic is one possible future input-price difference,
not promised net benefit. Eligible requests use `Reply with one word: ok.`,
maximum 16 output tokens and 60,000 ms timeout. One in-flight guard belongs to
the owning warmer; native drain/settlement finishes it before replacing a leg.
No timer, background service or cross-Run prefix cache is added.

`GenerateMaintenance` uses the same current provider, budget admission and
mandatory observation/accounting lifecycle, with source `maintenance`; its
observer has no warmer, so it cannot recurse. Only the existing budget-denial case skips optional model admission; other
Begin/persistence errors propagate. Provider failure/timeout records the current
diagnostic behavior. Failure to
persist usage/finish for already-admitted paid work propagates. Diagnostic event-budget denial may skip its marker. Once appended through
the existing persistAndPublish owner, an append failure keeps its current
terminal-failure behavior, even though the diagnostic helper returns no error;
this design does not silently weaken that boundary. Remove PrefixSHA256,
hash-only JSON encoding and `prefix_digest_failed`, not prefix construction,
real provider work or billed usage. Shutdown/cancel releases the in-flight guard
and waits under the current owner rather than leaving an orphan paid call.

**Steering.** Core still owns durable queued turns and generic follow-up/cancel.
Only selected steering calls policy Route/Take/Render and exposes steer/mode
controls. Construct one controller per active Run; preserve its armed state
across native handoff and rebind its current cancel/leg Host on resume.
OnBoundary returns false for unrelated approval/question interrupts. Nudge
resets per leg; that reset must not discard an accepted steering request. Preserve existing demotion eligibility for suspended Runs, missing
boundary cancel, incompatible preferences, attachments/file contexts/continuity;
ordinary supported steering must not be demoted wholesale. All/one-at-a-time
select a copied queue prefix; texts join verbatim with `\n\n`.

Host requests native cancel-after-model, drains the previous iterator, and uses
actual interrupt IDs with `ResumeWithParams`. Only agent-level interrupt targets
receive `ChatModelAgentResumeData`; pending tool targets remain untouched.
HistoryModifier inserts the steer before a trailing assistant with pending tool
calls, otherwise appends. Host verifies/commits exactly the selected prefix
under the existing lock and retains the existing ready gate: resumed history
cannot be consumed until durable queue consumption commits. Commit/resume
failure cancels/drains the tentative continuation and takes only the existing
specified fallback. It never consumes unrelated/newly queued turns. Replace
cancel handle and leg context on each successful resume. Same RunID, native
consumer, checkpoint owner, budget, effect fences and terminal owner persist.
Adapter owns no Runner/Engine/iterator or parallel consume loop.

Opening incompatible persisted state in a species lacking an EXP package is an
explicit unavailable/recovery result; it cannot auto-load hidden code or silently
pretend to apply a steer. Generic cancellation remains available to settle it.

### 11.6 Direct identity, equality and visibility contracts

| Boundary | Authoritative identity / comparison | Persistence / failure |
| --- | --- | --- |
| Model completion | Live v3 exactly byte_len of ordered UTF-8 deltas; no full text duplicate | tool.requested remains tool-response phase boundary; no extra terminal or usage charge |
| Original tool retry | Existing RunID/OperationID + exact original raw JSON bytes, distinct from middleware/effective bytes | SQL request_arguments BLOB/BYTEA; NULL is unavailable, never empty/coalesced |
| Approval | Private v2 canonical intended invocation using json.Number-preserving current canonicalizer + existing Run/call/tool/target/checkpoint | Changed intent rejects before effect; private credentials never replace digests in public DTOs |
| Admitted snapshot/checkpoint | Existing RunPromptSnapshot RunID/SchemaVersion/ComposerVersion/GenerationID + engine version | One admitted actual-value snapshot; no invented admission revision or duplicate snapshot service |
| Source/Generation | Explicit module/source/ref/root/trust binding; pack allocates ordinary opaque GenerationID once | Same ID propagates to Go/UI/Inspect; identical-ID regeneration is idempotent; authoring-<profile> stays stable until pack override |
| Bundle | Opaque acquired ArtifactID; existing InstallID/release sequence; epoch/config revision | Write-once owned artifact directory, exact release/config/declaration consent; no content dedup or independent disk-tamper proof |
| File/skill precondition | Exact existing before bytes + explicit existence; existing owner revision where available | Authorized safe-open/recheck before write; stale same-length content conflicts; missing differs from empty |
| Work/workflow/notebook/report | Existing request/operation identity + private normalized intended input; existing revision/CAS | Unequal reused input conflicts; preserve completed receipts and publication uniqueness |
| Memory/cognitive/mask | Existing owner revision + exact authorized mutation/strategy/admitted declaration | Human frozen-layer control, scopes and explicit review survive; upstream changes below are required |

Historical Journal bytes remain immutable. Supported v2 model.completed projects
its preceding deltas with byte_len validation while ignoring old digest values;
RPC replay/live-tail converts that metadata to v3 without adding/removing events
or changing seq. The older v1 content-bearing form is already unsupported by the
baseline projector: retain its historical persisted-message read path, skip its
unsupported reconstruction, and make v1 Journal replay/stream explicitly
unsupported rather than expose it to strict v3 consumers. Do not invent a second
live protocol or silently re-read v1 embedded content. New live v3 rejects extra
properties. D02 covers projector, trajectory, all Faces and live/replay billing.

**History/reference.** Cursor v2 carries normalized session/scope/filter,
phase/after and cut positions using the existing position types/limits. On every
page, rederive permitted sessions from server context, validate positions within
the authorized current/historical cut, reapply visibility/truncation and compare
normalized request values. Reject malformed/oversized/foreign scope positions;
unsigned tokens confer no authority and provide no issuance-authenticity claim.
Do not add a token-state cache. Attachment re-reads the authorized source
positions and compares exactly the bounded selected text returned by preview
before copying to the destination; stale selection conflicts.

**Presentations/downloads.** Proposed
`storage.DeliverablePresentation{SessionID domain.SessionID, ItemID string,
Content []byte}` joins existing `ContinuityMutation.Presentations` and commits in
the same transaction as its receipt.
`ContinuityStore.ReadDeliverablePresentation(context.Context, domain.SessionID,
string) ([]byte,error)` returns private immutable presentation bytes. Preserve
existing per-item/transfer byte limits. Prune in the existing session deletion or
presentation-truncation lifecycle. Begin transfer authorizes owner/item, compares
current authorized bytes against that captured content, then snapshots once;
ItemID + TransferID chunks all read the same scratch bytes. Scratch keeps the
current 60-second idle TTL; durable presentation content does not. Missing
capture requires re-present, never an mtime/size guess or current-file fallback.
Direct skill-toggle signature remains the distinct C9 control-plane API.

### 11.7 Final schemas and finite supported source formats

`T` denotes the final numbered application migration after all #91 owner
migrations above 035 are allocated serially (baseline A head). It is a build-time constant in the
delivered candidate, **not a runtime-configurable version**. D12 owns metadata;
D03/D07–D11 own their columns/JSON converters. Final metadata is exactly
version/name/applied_at without checksum. Final target reopen validates known
contiguous names/versions and actual required shape before any mutation.

| Source detected read-only | Finite upgrade route | Final runtime behavior |
| --- | --- | --- |
| Truly pristine DB: no metadata or application tables | Not an upgrade; initialize historical DDL + target migrations + final metadata in one owned transaction | Open only after complete commit; rollback on failure |
| Canonical SQLite/Postgres migration prefixes 001..N, 1 <= N <= 035, old complete metadata | Old-release normalizer to exact 035, then copy conversion to T | upgrade_required, before CREATE/backfill/write |
| SQLite legacy version/applied_at metadata | Existing old normalization/shape checks, then 035 -> T | upgrade_required |
| PostgreSQL compressed legacy markers max 13..23, no name/checksum | Existing legacyPostgresState recognition and accumulated bootstrap, including historical truncation repair at 020, then 035 -> T | upgrade_required |
| Released-017 drift fixtures at heads 016/017/023/035 | Frozen old-release repair path, then 035 -> T | upgrade_required |
| BML schema1 / artifact1.0.0 | Separate converter to schema2/artifact2.0.0 | upgrade_required |
| Complete target T (or a later known compatible target prefix) / BML2/artifact2.0.0 | Normal later target migrations only | Open/reopen; no old hash reader |
| Partial metadata, unexplained application tables without metadata, unknown/future versions, unsupported drift or incomplete conversion | unsupported_source_format; leave source untouched | Refuse before write |

The old normalizer validates only known original SQL bytes (including known
LF/CRLF and frozen 017 variants), not arbitrary drift. Legacy PG heads must
match actual recognized shape; a claimed marker alone is insufficient. There is
no installed-version census, and this support matrix is not a claim to cover all
deployments. Released SQL remains immutable transition history, even when it
mentions retired columns. Newly created final schemas, metadata and active
readers contain no such fields. Fresh initialization executes historical DDL
without recreating checksum calculation/validation.

Additional target ownership decisions:

- Tool operations retain `(run_id, operation_id)` primary key and
  admitted/claimed/completed states. Add nullable private request_arguments plus
  typed OutcomeStatus/Reason/Effects; retain middleware/effective bytes,
  result/failure, claim provenance and times. New rows require valid nonempty
  bounded original JSON. **Every baseline row defaults to NULL original input**:
  effective/checkpoint/preview bytes do not prove the original request. Baseline
  rows also lack typed outcomes: leave the three new typed columns NULL on
  those legacy unbound rows; preserve Result/Failure for history, never infer
  typed authority from text. New completed bound rows require all typed fields.
  NULL is
  readable history but cannot Lookup-reuse, claim or return a completed result
  to a newly offered invocation. Report source_binding_unavailable first.
- Old pending and approved-unconsumed approval v1 rows become stale/expired with
  upgrade_recovery_required; historical records remain. Do not infer exact
  intent from a digest. Active v2 private envelopes are only newly admitted.
- Old opaque paused checkpoint pointers are disabled/removed in the converted
  target, with no final legacy parser. New v2 has a four-byte header length,
  format=2, engine_version, RunID, existing snapshot schema/composer/GenerationID,
  created_at, then opaque native payload; retain current blob bounds/atomic
  pointer updates. No checksum or new admission-revision field.
- For old nonterminal Runs, active -> failed; accepted/queued -> cancelled,
  reason upgrade_recovery_required. Atomically settle old pending interactions
  and one terminal event. Existing terminal Runs stay untouched. Do not add a
  Run recovery_required enum; feature recovery_required projections retain their
  distinct current meaning. Claimed unknown effects remain fenced forever
  unless explicitly reconciled, never automatically restarted.
- Bundle target: bundle_installations.current_release_sequence;
  bundle_releases primary key `(install_id, release_sequence)` with artifact_id;
  bundle_artifact_refs primary key `(owner_kind, owner_id, artifact_id)`.
  ReleaseRef is InstallID/BundleID/ReleaseSequence; catalog identity remains
  Epoch/ConfigRevision/GenerationID. Allocate one opaque objects/<id> per old
  object; preserve all historical release sequences, including repeated content.
  Resolve old current digest to greatest matching release sequence (existing
  findRelease behavior); missing match is incomplete_conversion.
- Consent/revocation binds exact install/kind/local_id/release_sequence/
  config_revision/canonical declaration/consent revision. New release/config
  needs consent; revocation cannot accidentally revoke future component
  releases. **All converted executable consent is unarmed and requires fresh
  consent**, because historical composite evidence does not prove the exact
  declaration/config. Preserve publication/rollback history and unknown effects.
  Old operation receipts missing normalized input remain NULL/unreusable.
- Studio-owned lifecycle ledger is a separate SQLite store, not application SQL
  035. Its generations table currently requires artifact_sha256. D06 supplies a
  separate ledger format guard/conversion fragment: rebuild generations without
  that column, retain IDs/source refs/recipes/phases/parent links and all
  eval/release/install/event history. Existing file: SourceRef points to the
  actual SDK binary path, which may use an unrelated random output-directory
  ID or custom outDir. The converter records old-binary -> copied-target-binary
  mapping, verifies/copies each referenced artifact and rewrites file: SourceRef
  to that target path. Preserve non-file source-selection refs and Generation
  IDs/parent links; never assume directory ID equals manifest GenerationID.
  Missing or inaccessible custom output rejects conversion explicitly. Remove install hash gate, retain release/eval/
  actor gates. Old ledger shape requires finite upgrade before CREATE/writes;
  do not mistake a missing marker for pristine. Never run daily tenant migration
  merely because the Studio ledger is opened.

### 11.8 BML ownership and import/rollback

Preserve stored workspace/record IDs, scopes, revisions, tombstones and all
supersedes/reference links, including IDs once derived by an old hash. New IDs
are UUID-style opaque allocations; machine identity remains machine-memory-home.
OpenCanonical reads persisted workspace identity; fresh home allocates once.
Retain an explicit expected-workspace check; relocation does not hash the new
path or silently merge homes. Drop provenance.content_digest,
tombstone.reason_digest, evidence_refs[].hash and IntegrityReport digest fields.
Rebuild FTS from converted content and verify body bytes/counts/scopes/links.

Schema2/artifact2.0.0 imports bind migration ID + source kind/source ID and exact
raw source bytes (not necessarily JSON), plus normalized records in the existing
artifact owner. Persist source-to-opaque-record-ID allocation on first admission
and reuse it on retry. Equal source input retries return the same import;
changed input under that identity conflicts. Keep the three-file artifact
(records/rollback/manifest); replace evidence fields with owned actual inputs
and allocation map, not a new evidence sidecar. Old artifacts with only a digest
cannot reconstruct source bytes: retain history as complete/unverifiable and
refuse reusing that migration ID, while preserving imported records.

Delete memory_apply_journal only after checking the recognized source contains
no actual rows; unexpected nonempty data is unsupported_source_format, not
permission to discard it. Preserve rollback owned-path checks, before-state,
record revisions and restored graph validation. A forged rollback path cannot
delete another home. Conversion operates on copies; target runtime has one
active v2 parser, not open-ended v1/v2 negotiation.

### 11.9 Conversion transaction, recovery and delivery boundary

1. Read-only inspect source shapes; refuse unknown cases. Quiesce the organism,
   take a consistent backup (including SQLite WAL or a verified restored PG
   copy) and retain the matching old executable. No production data access is
   needed during implementation; fixtures cover supported shapes.
2. Create a separate target DB and owned artifact/BML roots. Stage copied bytes
   and a private stable old-to-new allocation map before rewriting references.
   On retry reuse that map; never allocate another set of opaque IDs. Restrict
   cleanup to paths created/owned by this conversion.
3. Convert target rows/JSON/references under the target DB transaction; preserve
   historical data and fences, apply explicit stale/pending policies above,
   rebuild projections/indexes and verify every cross-reference. Unknown
   effects prevent automatic reuse, not wholesale data reset.
4. Verify counts/content/ownership, no retired active fields, release selection,
   BML links and target reopen. DB plus filesystem is **not one atomic
   transaction**: all target components must be complete before switching the
   configured database/root pair. Before that switch, source stays complete
   and untouched. Partial target staging is never selected by final runtime.
5. Reopen the target executable on disposable copies and test injected failures
   at allocation/copy/SQL commit/verification/switch. If cutover fails, keep the
   old configuration/source. After cutover, rollback needs quiescence plus old
   executable/backup; preserve post-cutover data separately before reverting.
6. D12 delivers metadata/BML fragments; D03/D07–D11 and D06's Studio ledger
   deliver their own fragments. **D14** assembles, verifies and publishes the
   finite upgrade artifact only after all final formats exist. Record exact
   old-normalizer/upgrade-artifact/final-runtime commits. Remove converter code
   and old hash parsers from the final candidate tree; document how to obtain
   the finite artifact. An unavailable converter blocks final format release.

Stable public errors are upgrade_required, unsupported_source_format,
incomplete_conversion, source_binding_unavailable and upgrade_recovery_required.
Errors identify the owner/format and next recovery action without logging raw
private arguments, selected text, memory contents or credentials. Parse errors,
missing source bytes or partial conversion cannot be converted into success.

### 11.10 Pinned external contracts and required upstream delta

These are read-only source findings, not upstream changes or accepted upstream
releases. Pin changes require their own reviewed delivery and real integration
evidence before the corresponding Story can execute through that seam.

| Consumer inspected | Exact baseline pin | Findings / delivery owner |
| --- | --- | --- |
| INOFY | `71e2c9bbe47d5d095b27d756e38eaff565d93d36` | compiler.go programMeta computes definition/catalog/program digests; program.go Run/verifyEnvelope/resumeReplay require them; definitions publish/repository uses digest equality. D11 remains blocked on coordinated upstream API/version. |
| Laputa | `4b2bec2cc2ab3374b7ec1ab8d448e612c1e4db24` | CaptureRequest.ContentHash; AuthorizedMutation.PayloadDigest required by Validate; evolution RunBinding.StrategyDigest and effect PayloadDigest/replay; persona BaseHash/ContentHash CAS. D10 remains blocked on coordinated upstream API/version. |
| Studio submodule | `ac5532236ddb2e4312d569269fe9b64b5fbd79bb` | dsh-species-workbench/client.js displays ArtifactSHA256/UI sha256/policy_hash. D06/D07 need coordinated consumer update/pin; no shell mutation in this design delivery. |

**INOFY proposed contract.** CompileOptions receives an opaque owner-allocated
ProgramID; ProgramMeta exposes ProgramID, CompilerVersion and EinoBuild, not
content evidence. ExecutionRef binds ProgramID plus existing Run/node/Host
binding. Definition publication/revision compares exact normalized definition
and actually used catalog descriptors at its existing immutable revision owner;
compiler-version changes are explicit metadata. RunStore keeps exact admitted
input and resume answers in its existing Run/receipt record for equal-ID replay
conflicts. Checkpoint v2 binds Run/ProgramID/engine/serializer, without hashes.
Vivy's existing workflow revision and INOFY RunStore adapter supply that owner;
do not persist a second definition/snapshot merely to replace a digest. The
standalone upstream API must retain equivalent ownership for other consumers.
Remove required DefinitionDigest/ArtifactDigest/ProgramDigest/CatalogDigest/
InputDigest/Checksum and `inofy-normal-v1:sha256:` semantics, not replace them
with constant strings or unchecked aliases. Acceptance covers changed program,
input, answers, foreign Host/Run/node, publication CAS and crash/replay.

**Laputa proposed contract.** Capture uses existing event/source ID with exact
private content replay comparison. Authorized mutations/effects use operation
ID + exact canonical authorized intent (including scope/destination and
preconditions) in existing receipts. Persona uses BaseRevision plus explicit
expected existence/content where no revision exists. Strategy uses ordinary
strategy ID/version bound to the actual immutable admitted definition/config;
no invented digest-like token. Frozen layers, mission revision, memory scope,
claim/outcome and human review survive. Convert old pending receipts only when
actual intended values exist, otherwise explicit recovery/reapproval; completed
unknown effects never retry. Acceptance covers same-ID changed content,
concurrent persona edits, foreign scope and restart after claim/completion.

**Studio boundary.** Update workbench labels/DTO reads alongside host Generation
and install contracts; retain ordinary evaluation/release/actor rules. The
separate dsh-cost-meter session-log-repair custom sourceHash verifies standalone
DSH host log backup/replacement, not Vivy species/Generation/Run authority. It is
outside this #91 species-protocol cut; its native zstd checksum and host locks
also remain. This is a precise consumer disposition, not an external-library
hash exemption. Native Go/pnpm integrity, Git object refs, TLS/authentication
and bundler internals retain their existing G5 boundary. Every active custom
Vivy digest required by a dependency remains an unresolved **delivery blocker**,
never a deletion-complete exception.

### 11.11 Verification obligations and cost bounds

D04/D05 consume the exact signatures and lifecycle above; D06 produces source/
factory/UI bindings. D03 and every durable owner supplies its converter fragment;
D14 assembles the complete artifact and independently checks the same final
candidate. The existing 15-Story/26-edge/six-wave DAG stays intact; upstream
INOFY/Laputa/Studio are explicit external prerequisites, not fictional completed
Stories. D00's architecture choices are recorded; fixture/pin delivery and
implementation authorization remain separate from design closure.

Tests must prove: original-request conflicts despite equal effective arguments;
NULL legacy binding refusal before claim/replay; typed failure/refusal vs unknown
effect; missing producer outcome without a handoff wait; native parallel tool
order and provider retry; one scheduled nudge at thresholds/reset; real paid
warming plus mandatory accounting failure; same-Run steer with pending tool
siblings/approval/cancel/queue-commit failure; selected/unselected construction
and controls; all supported SQL/BML/Studio source conversions and crash points;
changed release/config requires new consent; same-length stale edits and exact
preview/download behavior; upstream changed-input/revision replay conflicts.
Use existing test owners/entry points in the Story plans, not a new permanent
scanner service. Product CI/real-provider/PG results are pending.

Costs are bounded by current owners: ten nudge tuple references plus current-leg
projections, no handoff wait; one optional synchronous maintenance round trip
with current 60-second cap; existing queue/checkpoint transitions; direct-byte
comparison at each retained conflict boundary; retained private before/input/
presentation bytes only where correctness requires them. No speedup, cache-hit
or net-savings claim has been measured. Opaque artifact acquisition intentionally
loses content deduplication and independent hash-based disk-tamper detection;
unsigned cursors intentionally lose issuance authenticity. Server permissions,
exact approved intent, durable outcomes and accounting remain acceptance
conditions. This design adds no runtime migration negotiation or extra worker.
