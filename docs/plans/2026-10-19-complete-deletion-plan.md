# Vivy Complete Deletion Plan: Scope, Order, and Two Independent Audits

Date: 2026-10-19

Tracking: [GitHub issue #91](https://github.com/ProjectViVy/agent-vivy/issues/91). Working branch: `codex/agent-loop-deletion`.

Executable planning package: [issue-91/index.md](../superpowers/plans/issue-91/index.md).
This document remains the shared scope/design authority. Section 10 specifies
the proposed replacement contracts; the index owns Story status, dependencies,
decision gates and acceptance evidence. Planning is authorized; implementation
and unresolved product/data changes await review. The issue's embedded snapshot
is historical and does not override this linked package.

Status: Pre-deletion inventory and proposed execution plan. No runtime deletion, regeneration, database migration, or behavior test has been performed by this delivery.

## 1. Decision and completion standard

The owner's direction is **complete deletion of Vivy's custom hash system**, with only an optional informational digest of the final distributable remaining. Merely moving a check out of assembly or the foreground loop does not satisfy this direction. The same closure standard applies to every other mechanism explicitly selected for retirement.

Delete its implementation, construction, callers, state, persisted representation where applicable, configuration, public fields, generated outputs, feature-only tests, and current instructions requiring it. Do not leave a disabled implementation, a no-op compatibility wrapper, an ignored configuration field, or a replacement with another hash algorithm/name. Do not move rejected complexity to a plugin while retaining a core dependency on it. Rollback uses version control and a documented data strategy, not a second implementation in the shipped product.

This direction does not automatically retire every capability discussed in the research. Hash retirement is confirmed. Full removal of reminders, proactive warming, tool-mount lifetimes, same-Run steering, or companion behavior remains a scoped product decision. Once selected, each must be deleted completely; an implementation cannot claim completion by making it optional.

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

This is a cross-repository pre-audit, not an implementation-ready sign-off on every persisted field. Tables below identify deletion families and unresolved data/security decisions. The implementation lane must close those decisions for its cut before editing; the post-deletion audits remain pending.

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
| Nudge/warming hashes | `nudge_window.go` signatures; `cachewarmer.go` prefix evidence; associated events/config/tests | Do not replace a signature with a renamed identity while retaining an unneeded subsystem. Decide whole-feature retirement alongside the core-state cuts below. |

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
| Proactive warming | `cachewarmer.go`, observer wiring, config/defaults/parser, `cache.warmed` event/schema, UI options, feature tests and active plan | No extra proactive paid call; foreground call usage and provider-native cache accounting remain accurate. |
| Reminder and nudge handoff | `nudge_window/state/middleware`, unconditional engine registration, adapter failure side channel, mapper parked completions, Service outstanding/batch restoration, steering integration, reminder event and normative design | Recoverable tool errors/refusals still reach the model. Durable effect completion remains before ordinary result return. No missing-call wait or replayed sibling effect. |
| Synthetic tool lifecycle authority | Reconstructed `tool.started` and duplicate lifecycle ownership | Present actual execution facts, including refusal before claim; UI timing cannot pretend that result arrival was execution start. |
| Derived-work critical section | Old synchronous projection/output/hook path under `projectionMu`, including obsolete helpers and fallback path | Retain commit/terminal/delete ordering; derived views never acknowledge progress back to the runner. Do not replace it with another required coordination service. |
| Tool visibility lifetimes | Activation/mount/suppression mirrors, rebuild paths, associated session state/events/config | One clearly owned tool surface and server-side permissions. Decide fixed Run surface versus one supported search mechanism first. |
| Same-Run steering | Steer queue branch, cancellation/history surgery, duplicate resume setup, API/UI controls and tests | Explicit cancel/new-Run behavior if chosen; keep necessary approval/question recovery and any retained next-turn queue. |
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
3. **Delete isolated, closed families first.** Event audit and proven dead structures are small candidates. If warming retirement is selected, remove the entire extra-call facility. Do not begin by renaming every shared `Digest` type.
4. **Delete the completion/hash protocol end to end.** Change producer, event schema, SDK/TUI/headless/Web consumers and fixtures as one cut. This removes a concrete extra completion authority and provides a narrow causal regression target.
5. **Delete the nudge coordination closure if selected.** First locate retained outcome classification at the actual tool boundary, then remove the whole reminder/handoff mechanism. Preserve operation/recovery proofs. Reassess observer settlement after this removal rather than adding another synchronization workaround.
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

Reuse meaningful existing test helpers while removing reminder/count/hash-only assertions. Do not retain the old architecture solely to keep old tests green. Do not delete business acceptance tests because their original filenames mention the retired mechanism.

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

These are discovery terms, not a global text replacement. `GenerationID` still needs a non-content-addressed identity if Generations remain; ordinary URI fragments and natural-language summaries are not hash machinery. Existing opaque historical IDs, historical records and approved native dependency/security internals must be classified by exact purpose. An unexplained match fails closure; a broad wildcard exemption such as “all tests” or “all compatibility code” is not acceptable.

Regeneration is mandatory for affected outputs: `internal/generated/assembly/generate.go` invokes the default SDK generator; UI predev/prebuild/pretest invokes `ui/scripts/stage-ui-assembly.mjs`; DIVA/memory-loop scripts create generated overlays. Fix their source, regenerate, inspect the produced diff, and regenerate again to establish no further changes. Build and inspect affected official Recipe artifacts, including minimal and plugin examples; final contraction must account for all supported official Recipes and build variants. The current inventory has not proved all tag/submodule combinations.

Specific resurrection points include `sdk/internal/cmd/source-hash/`, `scripts/check-plugin-v1-fixtures.mjs`, Descriptor SHA constants, conformance evidence files, `sdk/internal/memory_loop_generation_test.go`, `scripts/memory-loop.mjs`, `scripts/diva-recall.mjs`, and UI demo CAS. Remove the obsolete producer or rule, not merely its generated answer.

Active requirements needing revision include `VIVY-ASSEMBLY.md`, `VIVY-PLUGIN-SPEC.md`, `VIVY-MODULE-STANDARD.md`, `.agents/skills/vivy-plugin/SKILL.md`, and AGENTS source-evidence/migration-checksum clauses. If reminders/warming retire, also remove or clearly retire the operative `NUDGE-DESIGN.md`, nudge implementation plans and warming backlog. `VIVY-MEMORY-HOST-CONTRACTS.md` cites `AuditRecord`; replace the obsolete dependency while keeping its bounded-data requirement. Historical delivery logs can remain factual history; they cannot govern future implementation.

## 9. Open decisions and release closure

Before the corresponding implementation cut, record a concrete choice for:

- Supported old database/checkpoint/approval versions and the finite upgrade boundary; handling incomplete/unknown effects without replay.
- Original request binding after `RequestDigest`; snapshot/content ownership after removing file/bundle/approval hashes.
- Custom HMAC cursor retirement and native dependency/transport cryptography boundary; builder-owned resource names versus Vivy-owned validation.
- Final delivery unit for the sole optional artifact label; ordinary Generation identity without content addressing.
- Which non-hash features retire completely, especially nudge/reminders, warming, same-Run steering and tool visibility lifetimes.
- Retained product capability versus whole-feature removal for large workflow/Work/bundle/companion protocols. Do not silently delete a user feature to make a search result disappear.

A deletion deliverable is closed only when its implemented candidate, retained behaviors, data transition, active-rule changes and both independent audit results are recorded. Evidence must identify the actual commit/build tested. A failing check is not waived by a passing residue scan; a passing CI is not proof of deletion completeness.

Plan verification: assembly and storage auditors independently reviewed the integrated draft; their scope corrections are incorporated. Documentation diff checks passed. Product CI was not run because this delivery changes research/planning text only; this is not a waiver for implementation.

Current status: five read-only domain inventories integrated; broad tracked-tree search completed; exact per-cut decisions, implementation, regeneration, behavior verification, real-token E2E and post-deletion residue review are **pending**. No runtime fix, performance gain or incident root cause is claimed.

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

### 10.1 Shared contracts proposed for review

| Contract | Concrete proposed target | Owner / implementation Story |
| --- | --- | --- |
| C1: stream completion | Emit `model.completed` payload v3 with exactly `{ "byte_len": N }`, where N is the UTF-8 byte count of the ordered text deltas for that model step. No full-text duplicate or digest. An emitted empty final completion has N=0. Preserve the current `tool.requested` phase boundary for tool-call responses: do not add a `model.completed` for a tool-only/preamble tool-call step. Keep live/replay budget charging and final-message persistence unchanged; a completion is not a Run terminal. New code accepts v3 on the live wire. Historical journal projection ignores digest metadata without validating it. At RPC `replayEvents`/`streamRun`, normalize supported historical v2 completion metadata to v3 before sending it to strict Face consumers, preserving sequence/count and never rewriting the Journal. G2 must enumerate any older historical shapes and their supported read path; no second live v2 protocol is admitted. | Runtime mapper, Face consumers; D02 |
| C2: tool operation | Add private `RequestArguments []byte` to `domain.ToolOperation`; retain `MiddlewareInputArguments`, `EffectiveArguments`, RunID/OperationID/ToolName, claim owner/state and durable outcome. `Lookup` compares original request bytes, not effective bytes. Persist first, then return a completed outcome to Eino. Remove RequestDigest/ArgumentsDigest and digest event fields. Different raw requests under the same operation ID are a conflict; do not introduce JSON normalization into this retry identity. Add durable OutcomeStatus/OutcomeReason/OutcomeEffects fields in this same row (success/recoverable/refused/fatal; existing reason vocabulary; not_executed/none/unknown). Result/Failure retain their text roles. Completion stores all fields atomically and replay uses those typed values without string-prefix guessing. A converted row lacking recoverable original input keeps RequestArguments=NULL and its original state/outcome; Lookup/Execute returns explicit ErrToolOperationBindingUnavailable before claim or replay. New admissions require nonempty valid JSON within the existing 1 MiB argument bound; do not invent a recovery-required operation state. | Existing operation store; D03 |
| C3: approved invocation | Replace the private approval envelope with `vivy.tool-approval-proposal/v2`, carrying canonical intended arguments and provider data. Use the existing JSON canonicalization semantics, preserve numeric literals, bind existing Run/call/tool/target/checkpoint identifiers, and compare the resumed intended invocation exactly. Reuse the operation row where applicable; non-row orchestration/Work approvals retain their existing private owner. Canonical arguments never enter public event/Review DTOs merely to replace a hash. | Existing approval/operation owners; D03 |
| C4: source and Generation | Source selection retains Module ID, native ref, trust, explicit root and typed binding; remove all Vivy-computed source/lock hashes and `source-hash` CLI. Allocate one opaque Generation ID at `pack`; pass that same ID to Go/UI generation, manifest and Inspect. Authoring generation uses an explicit stable profile ID; pack always overrides it. Regeneration with identical explicit inputs is idempotent. No content-equality or authenticity claim attaches to the ID. | SDK catalog/compiler; D06 |
| C5: bundle identity and consent | Acquisition allocates an opaque ArtifactID; published identity is existing `(InstallID, ReleaseSequence)`. Keep existing catalog Epoch/ConfigRevision CAS. Resource paths and declarations belong to the immutable published release; remove content-addressed object names and deduplication. Recommended consent policy: bind exact selected declaration and configuration to release identity; a changed release/config requires new consent. Existing consent reuse/revocation behavior must be reviewed before adopting this policy. | Bundle manager; D07, decision G3 |
| C6: admitted inputs and checkpoints | One immutable Run-owned prompt/tool/policy snapshot contains actual admitted values. Use existing RunPromptSnapshot RunID/SchemaVersion/ComposerVersion/GenerationID, not an invented admission revision. New checkpoint outer format v2 contains format version, Eino engine version, RunID, snapshot schema/composer/Generation identity and created time, followed by opaque payload. No checksum or prompt digest. Engine mismatch, foreign Run or missing admitted snapshot fails closed. Blob generation/pointer atomicity remains. | Admission and checkpoint stores; D08 |
| C7: history and selections | Cursor v2 is bounded, stateless and entirely untrusted: phase/after/cut positions plus normalized request/scope values, not authority claims. Every page derives permitted sessions from server context, compares normalized request values, checks positions against an authorized cut, and reapplies current visibility/truncation. Remove HMAC, random key/fallback, ScopeHash, FilterHash and SelectionDigest. Reference attachment binds selected source positions plus the bounded selected text shown in preview; re-read authorized values and compare exact selection before committing the destination copy. No token-to-state cache. Review restart/snapshot semantics under G4 before implementation. | Existing History/Reference services; D09 |
| C8: edits and downloads | File proposals/revisions bind exact expected bytes (including missing-versus-empty) through existing private payloads. Compare the reopened authorized file against those bytes before mutation; retain safe-open and race checks. Downloads use ItemID + existing TransferID and one immutable bounded scratch snapshot. Compare against the presentation-time captured bytes before opening a transfer; every chunk reads the same snapshot. Private presentation bytes are committed atomically with the presentation receipt under the existing ContinuityStore; retain them until the source session is deleted or its presentation is hidden by truncation, then prune in that existing lifecycle. Active transfer scratch retains the existing 60-second idle TTL; presentation bytes do not inherit that TTL. Missing/expired private presentation bytes yield explicit unavailable/re-present, never current-file acceptance. Reuse storage/transfer ownership; do not claim mtime/size prove content equality. Persisting bounded presentation bytes is a deliberate storage change reviewed under G4. | File/Deliverable services; D09 |
| C9: feature CAS and retries | Staged Skill mutations use existing BeforePayload and exact current bytes; direct SetSkillEnabled stays a control-plane toggle and receives authorized expected SKILL.md bytes/existence instead of baseHash, with a read-before-submit UI flow and conflict refresh; managed memory/notebook/mask/toolkit values use existing owner revisions and exact admitted declarations. Workflow/Work/report operations retain request IDs and normalized intended inputs in their existing private receipts. Equal ID with unequal input is a conflict. Remove digest metadata, not idempotence or scope isolation. Remove Vivy calls that compute INOFY digests; external opaque values are never revalidated as Vivy evidence. If a pinned upstream API requires a digest authority that cannot be adapted, D11 is blocked for an upstream contract change. | Feature owners; D07/D10/D11 |
| C10: BML | Proposed schema v2/artifact `2.0.0`: strip digest fields from record JSON, preserve existing opaque record/workspace IDs and all links/scopes/revisions/tombstones; new IDs are ordinary opaque IDs, never content/path derived. Existing store revisions own CAS. Imports use migration ID + original source identity and exact normalized input for conflicts. Delete unused memory_apply_journal. Workspace association comes from persisted ownership, not path hashing; handle relocation/collision explicitly. | Independent BML module; D12 |

The exact values above are a proposed design, not an assertion that their
producers exist. D00 records approval or edits to these contracts before any
dependent Story becomes Ready. Tests named in Story plans are proposed unless
explicitly identified as existing.

### 10.2 Finite data transition

G2 must approve supported source formats and delivery boundaries before durable
cuts execute. The proposed least-regressive route is:

1. Quiesce one organism and back up its database and owned artifact directories;
   do not migrate a live Run or operate on tenant data during development.
2. A separately delivered, finite upgrade tool uses the **old release** to
   normalize currently supported legacy SQL shapes to baseline 043. Existing
   legacy PostgreSQL markers 13-23 and released-017 repair fixtures remain
   required in that tool; do not silently remove their support. The old release
   may validate its old formats, but the final runtime ships no old hash reader.
3. At that boundary convert exact baseline SQL/JSON/object formats on copies.
   The proposed source set is SQL 043 with baseline metadata, BML schema 1 /
   artifact 1.0.0, and the baseline checkpoint/approval envelope. Newly added
   migration IDs are allocated serially above 043 after rechecking the head.
4. Convert old pending approvals only when exact reviewed intent is recoverable
   without inference; otherwise mark expired/reapproval-required. Preserve
   admitted/claimed/completed operation rows. An unresolved claimed effect
   remains unknown and is never automatically invoked again. Old paused Eino
   checkpoints are not promised convertible: recommendation is explicit
   recovery-required/reapproval with the durable effect fence intact, subject
   to owner approval. Never claim this is seamless resume compatibility.
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

No installed-version census, downgrade test or old checkpoint conversion proof
exists in this planning delivery. These facts prevent a Ready claim for durable
cuts until G2 is settled; they do not justify resetting user state.

### 10.3 Proposed core contraction and preserved boundaries

G1 recommends complete retirement of nudge/reminder coaching, proactive warming,
same-Run steering, and mount/activation lifetime mirrors; use a fixed admitted
tool surface for basic Runs, retain next-turn follow-up and approval/question
resume, and evaluate existing native ToolSearch only as an explicit larger
Recipe variant. This is an explicit proposed product reduction, not authority
to delete it now. Companion, workflow, Work, memory and notebook capabilities
remain retained unless separately authorized.

For D04, retain synchronous acknowledgment of admission, permission/approval,
effect claim/outcome, final message and terminal state. Move recoverable tool
failure classification to the existing tool result boundary. Remove event-
consumer/nudge acknowledgment as permission for the next model request.
Derived transcript/status views may lag committed execution and rebuild through
existing Journal replay; a failed optional sink must not own the runner.
Never make hard budgets, required audit/accounting facts or durable effect
completion best effort. This durability/visibility revision needs G1 approval.

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
submodule and pinned Laputa/INOFY APIs must be inspected at D00/D11 for actual
consumer obligations; this repository's plan cannot assert those repositories
have been changed or accepted. Escalate a required external contract change;
do not retain a local no-op verifier or silently fork the dependency.
