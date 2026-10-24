# Baseline A Re-anchor Audit: Issue #91 File References at `202aac38` (2026-10-07)

Date: 2026-10-19

Worktree: `C:\Users\com01\Desktop\agent-vivy-loop-deletion`, branch `feat/agent-loop-deletion`,
HEAD `e06c656cedba7c7a47c7070906ff3f0fd5898542`.

Implementation baseline A: `202aac3885cb1479998127f0fb9004bea54aca57` (2026-10-07 11:40:15 +0800,
merge of PR #35 `feat/vivy-code-parity`; 3,184 tracked paths).

Plan authoring tree: `e96092966cfbfa443648374948e7e7e16dd01683` (2026-10-18; here "10-18").

Status: planning-only re-anchor audit. No runtime code, generated artifact, database, CI run or
real-path behavior was executed or verified by this document. This file adds no product claim.

## 1. Purpose, baseline and method

### 1.1 Purpose

The issue #91 deletion package (`docs/plans/2026-10-19-complete-deletion-plan.md` §10–§11 and the
fifteen Story plans `docs/plans/issue-91/D00.md`–`D14.md`) was authored against the
10-18 tree and was replayed here as docs-only commits onto baseline A. Its file references therefore
describe a tree that is 168 commits ahead of the tree the work will start from. This document:

1. records which cited paths exist, are missing, or are only proposed at baseline A;
2. assigns every missing path a disposition: (a) rename to the 10-07 equivalent, (b) remove because
   the feature was added after 10-07, (c) merge into another Story/owner, (d) keep as
   proposed-create;
3. maps C1–C11 onto the baseline-A owners and fields, with status;
4. states the consequences for the three retained experimental packages (nudge, warming, steering)
   and the epics that will own them;
5. lists open questions and the limits of this audit.

It does not revise the architecture. Where the retained design conflicts with baseline-A behavior,
§4.5 records the decision the owner must make rather than silently narrowing the plan.

### 1.2 Baseline facts checked

| Fact | Value / evidence |
| --- | --- |
| Baseline A commit | `202aac38` = `2026-10-07 11:40:15 +0800`, "Merge pull request #35 from ProjectViVy/feat/vivy-code-parity" (`git show -s --format='%H %ci %s' 202aac38`) |
| Baseline A size | 3,184 tracked paths (`git ls-files \| Measure-Object -Line`) |
| Working-tree code equals baseline A | `git diff --stat 202aac38 HEAD` = 31 files, all under `docs/`, 3,382 insertions, 0 deletions; all are doc commits (plan, research, Story plans, logs) |
| 10-18 tree still inspectable | `git cat-file`/`git show`/`git ls-tree` on `e9609296` succeeded; used only to classify missing paths |
| Migration heads | Baseline A SQLite and PostgreSQL both end at `035_workflow_definitions.sql`; 10-18 ends at `043_toolkit_bindings.sql` |
| Audit start cleanliness | `git status --short` was empty before concurrent Lead edits appeared during the audit (see §5) |

### 1.3 Check method (exact)

1. **Enumeration.** Every inline-code span in `D00.md`–`D14.md` and in plan §10, §10.1a and §11 was
   extracted with a regular-expression pass; verification-command fences were additionally scanned
   for bare path tokens. Path-shaped and symbol-shaped spans were separated from prose fragments.
2. **Expansion.** Brace lists were expanded (`{a,b}.go` → each member) and directory references were
   tested as directories. Shorthand spans were resolved with the table in §1.4 before testing.
3. **Existence.** `Test-Path -LiteralPath <path>` executed from the worktree root; `git ls-files -- <path>`
   cross-checked tracked status. Because the worktree was clean at audit start, existence equals
   tracked-at-baseline.
4. **Why a path is missing.** For each missing cited path: `git ls-tree -r --name-only e9609296 -- <path>`.
   `IN-10-18` means the path is a 10-18 addition (candidate disposition (b)); `ABSENT-10-18` means the
   path never existed in either inspected tree (candidate (a)/(c)/(d)).
5. **Owner/field mapping (C1–C11).** Targeted `rg -n` over the plan's named owner files for the exact
   types, fields, constants and functions; file:line evidence is recorded in §3.
6. **Document self-check.** `git diff --check` on this file.

Representative commands (run from the worktree root):

```powershell
git show -s --format='%H %ci %s' 202aac38
git diff --stat 202aac38 HEAD
git ls-files 'internal/runtime/nudge*' 'internal/runtime/queue*' 'internal/*bundle*'
Test-Path -LiteralPath internal/runtime/nudge_window.go
git ls-tree -r --name-only e9609296 -- internal/runtime/nudge_window.go
rg -n "loopWindow|fnv|FNV" internal/runtime/loopguard.go internal/runtime/nudge_state.go
git diff --check -- docs/research/2026-10-19-baseline-a-reanchor.md
```

### 1.4 Shorthand resolution used

| Cited form | Resolved and checked as |
| --- | --- |
| `catalog_v1.go`, `descriptor_v1.go`, `source_verify_v1.go`, `stageui_v1.go` | `sdk/internal/<name>.go` |
| `approval_binding.go`, `checkpoint.go`, `checkpointadapter.go`, `prompt_state.go`, `prompt_middleware.go`, `request.go`, `bundle_admission.go`, `contextsource_admission.go` | `internal/runtime/<name>.go` |
| `blob.go`, `run_admission.go`, `fileversions.go`, `continuity.go`, `skill_revisions.go`, `masks.go` | both `internal/storage/sqlite/<name>.go` and `internal/storage/postgres/<name>.go` |
| `VIVY-ASSEMBLY.md`, `VIVY-MODULE-STANDARD.md`, `VIVY-PLUGIN-SPEC.md` | `docs/architecture/<name>` |
| `{identity,manifest,catalog,operations,snapshot}.go` | each of the five `internal/bundlecontract/<name>.go` |
| `{acquire,artifact,manager}.go` | each of `internal/bundles/<name>.go` |
| `{filesource,host}.go`, `{resourcesource,host,toolworld}.go`, `{catalog,service,actions}.go`, `{manifest,runner}.go` | each member of the named package directory |
| `runtime/{skills_backend,...}.go` and similar | `internal/runtime/<member>.go` |
| `ui/src/lib/{api,deliverable-download,store}.ts` | each member |
| `internal/storage/migrations/metadata/{sqlite,postgres}.sql` | each member |

### 1.5 Method limits

- **Presence is not shape.** A file or symbol existing at baseline A does not prove its 10-18
  contract, fields or behavior are identical; §3 records field-level checks only for the C-row owners
  the plan names.
- **Not checked:** runtime behavior, tests, `go`/`just`/`node`, CI, generated Assembly/UI output,
  database schemas applied to a live store, migrations executed, submodule content (`studio/`), the
  external Laputa/INOFY/Studio repositories, and production data. Those are explicitly outside this
  audit.
- **Shorthand may hide drift.** The resolution table in §1.4 is an assumption. A cited bare filename
  such as `catalog_v1.go` is not proof the plan meant `sdk/internal/catalog_v1.go`.
- **Proposed-create paths are absent by definition.** For (d) dispositions, absence is not a defect;
  the table records them so no later reader treats them as existing.
- **Prose-only path mentions were excluded.** Slash-separated phrases such as `runtime/domain/storage/RPC/Face`
  are prose, not file references, and were not tested.
- **Story sources:** D00–D03 and D06–D14 were audited at HEAD (`e06c656c`). D04, D05, `index.md`, the
  shared plan and `docs/TODO.md` were concurrently edited by the Lead during this audit; D04/D05 were
  re-read in their current baseline-A form and are audited as such. No other Story plan was modified
  at audit time.
- **`.workspace/` outputs** cited by D06/D13/D14 (`issue91-minimal-artifact`, `issue91-usable-minimal`)
  are gitignored working directories, not repository paths; `.workspace/` does not exist in this
  worktree.

### 1.6 Tally

- 22 cited, non-proposed paths are missing at baseline A: 6 need rename/merge (a)/(c), 16 belong to
  feature families added after 10-07 (b).
- 1 cited symbol (`VerifyUnchanged`) is a 10-18 addition; only `HashSourceTree` exists here.
- Approximately 30 cited paths are proposed-create (d) in D04–D06, D13, D14 and §10.1a/§11; they are
  absent as expected, including all three loop-exp Port/factory/adapter/plugin/Recipe paths.
- No cited path in D00, D02, D09, D12 or D14 exists as a *modified source* and is missing; those
  Stories' only missing entries are proposed tests/outputs.

## 2. Per-Story disposition (D00–D14)

Legend for the state column: **E** = exists at baseline A; **M** = missing; **P** = proposed-create
(absent by design). Disposition codes: **(a)** rename to the 10-07 equivalent, **(b)** remove because
added after 10-07, **(c)** merge into another Story/owner, **(d)** keep as proposed-create.

### 2.1 D00 — Freeze scope and finite upgrade contracts

| Cited path (as written) | State | Disposition / note |
| --- | --- | --- |
| `docs/plans/2026-10-19-complete-deletion-plan.md` | E | Modify in place. |
| `docs/plans/issue-91/index.md` | E | Modify in place. |
| `AGENTS.md`, `.agents/skills/vivy-plugin/SKILL.md`, `.agents/skills/vivy-eino/SKILL.md` | E | Read. |
| `internal/storage/migrations/runner.go` | E | Read; head is 035, not 043 (see C/G2 row). |
| `internal/runtime/checkpoint.go`, `internal/runtime/approval_binding.go` | E | Read; both carry hashes/digests at baseline (C3/C6). |
| `bml/record.go`, `bml/migrate.go` | E | Read; BML v1 digest format exists (C10). |
| `laputa-source.lock.json`, `.gitmodules` | E | Read; `studio/` is a submodule and was not inspected. |

D00 has no missing file references. Its **non-path references are wrong for this baseline** and must
be re-anchored: "baseline SQL 043" is `035`; the G2 finite matrix "1..043" is "1..035"; released-017
repair fixtures do exist here (`internal/storage/sqlite/sqlite_test.go:334-342` checks the migration
017 marker), so that part of G2 is valid. Allocation language "above 043" becomes "above 035".

### 2.2 D01 — Delete digest audit and unused experimental hash code

| Cited path | State | Disposition / note |
| --- | --- | --- |
| `internal/runtime/audit.go`, `internal/runtime/audit_test.go` | E | Remove. AuditRecord/AuditHook/SlogAuditSink and `payload_sha256` exist (`internal/runtime/audit.go:15-59`). |
| `internal/app/app.go` | E | Modify the default AuditHook construction (`internal/app/app.go:739`). |
| `docs/architecture/VIVY-MEMORY-HOST-CONTRACTS.md` | E | Modify AuditRecord dependency. |
| `plugins/exp/loop-detection/deferred.go` | M | **(b) remove.** Path is IN-10-18; no `plugins/exp/` directory exists at baseline A. |
| Proposed `internal/app/assembly_audit_test.go` | P | (d) create. Absent at both trees. |

### 2.3 D02 — Remove model completion and request hash protocols across Faces

| Cited path | State | Disposition / note |
| --- | --- | --- |
| `internal/runtime/mapper.go`, `payloads.go`, `request.go`, `service.go`, `trajectory.go`, `budget.go` | E | Modify. `payloadModelCompleted` v2 requires `content_sha256`/`byte_len` (`internal/runtime/payloads.go:132-133`); `digestModelRequest` writes `preamble_sha256`/message `content_sha256` (`internal/runtime/request.go:7-41`). |
| `internal/runtime/message_projector.go:completedProjectionContent` | E | Symbol exists (`internal/runtime/message_projector.go:163-196`) and validates the digest; v1 is already rejected (`errUnsupportedCompletedVersion`). |
| `internal/rpc/control.go` (`replayEvents`/`streamRun`) | E | Symbols exist (`internal/rpc/control.go:3920`, `:4237`). |
| `sdk/facerun/facerun.go`, `sdk/tui/stream/events.go`, `sdk/tui/stream/reducer.go`, `internal/app/headless.go`, `ui/src/lib/api.ts`, `ui/src/lib/store.ts` | E | Modify; facerun `completeModelLocked` verifies `content_sha256` (`sdk/facerun/facerun.go:336-356`); TUI `ModelCompletionMetadata` validates digest (`sdk/tui/stream/events.go:63-106`). |
| `schemas/events/payloads/model.completed.json`, `model.request.json` | E | Modify; both encode digest fields. |
| Proposed `internal/runtime/completion_protocol_test.go`, `sdk/facerun/completion_protocol_test.go` | P | (d) create; absent at both trees. |

All modification targets exist. C1's plan statement that "older v1 is already unsupported by the
baseline projector" is **verified at baseline A**.

### 2.4 D03 — Bind tool operations and approval to exact private inputs

| Cited path | State | Disposition / note |
| --- | --- | --- |
| `internal/domain/tool_operation.go`, `tool.go`, `review.go` | E | Modify; `ToolOperation` carries `RequestDigest`, `MiddlewareInputArguments`, `ArgumentsDigest`, `EffectiveArguments` (`internal/domain/tool_operation.go:24-40`). |
| `internal/storage/tool_operations.go`, `contracts.go`, `sqlite/tool_operations.go`, `postgres/tool_operations.go` | E | Modify; SQLite/PostgreSQL columns exist in migration `031_tool_operations.sql`. |
| `internal/runtime/tool_operation.go`, `tooladapter.go`, `approval_binding.go`, `policy_context.go`, `service.go`, `orchestration.go` | E | Modify; approval envelope v1 stores `arguments_sha256` (`internal/runtime/approval_binding.go:10-40`). |
| `internal/runtime/approval_binding_test.go` | E | Modify/retain focused tests. |
| `schemas/events/payloads/tool.operation.json` | E | Modify; payload v1 exposes `request_digest`, `middleware_input_digest`, `arguments_digest`. |
| "paired append-only SQL migration above 043" (task text) | non-path | Re-anchor to **above 035**; next free ID is 036. Never edit released 031 or approval migrations. |

All cited paths exist; the migration-number instruction is the only re-anchor.

### 2.5 D04 — Settle the loop-exp seam and remove nudge hashing/handoff

D04 was rewritten by the Lead during this audit and now contains its own "Baseline A mapping". This
audit confirms most of it and corrects two points.

| Cited path | State | Disposition / note |
| --- | --- | --- |
| `internal/runtime/nudge_window.go` | M | **(a) rename/redirect to `internal/runtime/loopguard.go`.** IN-10-18. Baseline A keeps the same algorithm in `loopguard.go`: `loopWindow` (`[]uint64` FNV signatures), `loopWindowSteps = 10`, `loopRepeatLimit = 5`, `errLoopDetected`. `nudge_state.go` only *declares* `window loopWindow` (`internal/runtime/nudge_state.go:69`) and calls `record(...)` from `sealLocked`. **Modifying only `nudge_state.go` leaves the FNV code intact.** |
| `internal/runtime/nudge_state.go`, `nudge_middleware.go` | E | Modify. Middleware exists (`adk.ChatModelAgentMiddleware`, `nudgeReminderMaxBytes = 1024`, `schedule-before-injection`); thresholds 3/5 are in `sealLocked`. |
| `internal/runtime/prompt.go` | E (not yet cited) | **Add to D04 extraction list:** the reminder templates live here (`renderNudge`, `internal/runtime/prompt.go:110`); they are not in `nudge_state.go`. |
| `internal/runtime/engine.go`, `tooladapter.go`, `tool_failure.go`, `mapper.go`, `service.go` | E | Modify; `engine.go:376` installs nudge middleware unconditionally. |
| `schemas/events/payloads/tool.nudge.json` | E | Retain; v1 requires `tool_call_id`, `tool_name`, `reason`, `repeat_count` (enum 3/5), `template_version` const `nudge-v1`. |
| `docs/architecture/NUDGE-DESIGN.md`, `docs/archive/plans/nudge/ND-0..4.md` | E | Retain/update; both exist at baseline A. |
| `sdk/port/loopexp/nudge.go`, `internal/runtime/extensions.go`, `internal/runtime/extensions/nudge/adapter.go`, `internal/app/assembly_loop_extensions.go` | P | (d) create. Absent at baseline A **and at 10-18**; the plan's "adapter" file is proposed design, not a 10-18 artifact. |
| `plugins/exp/nudge/`, `recipes/exp-nudge.vivy.yml` | P | (d) owned by E-NUDGE; D04 explicitly does not create them. |
| `internal/runtime/tool_continuation_test.go` | P | (d) proposed test; absent at both trees. |
| Existing tests `nudge_state_test.go`, `nudge_middleware_test.go`, `nudge_acceptance_test.go`, `nudge_contract_test.go` | E | Retain/fork into the produced tuple-window tests. |
| "Retain: nudge UI/event consumers" (Files section) | — | **Correction:** no nudge UI consumer exists at baseline A (no `ui/` reference to nudge; only `tool.nudge` schema, domain event and ND docs). Drop "UI" or reclassify as future E-NUDGE scope. |
| `internal/app/importlint_test.go` tests `TestEinoImportsQuarantined`, `TestPluginWindowCannotImportInternal` | E | Retain guard tests (verified in `internal/app/importlint_test.go:86,126`). |

**Additional hard input (layout/coupling):** at baseline A the FNV window is also the *loop-detection
hard stop*: `record` returns `errLoopDetected` when a signature repeats more than 5 times in 10 calls,
and the runtime turns that into a classified `run.failed`. Nudge reminders are a second consumer of
the same window. D04/E-NUDGE must decide whether that hard stop is core safety (stays in core with an
exact-tuple implementation) or part of nudge (moves/deletes with the package). The current D04 text
removes FNV but does not name this ownership decision; leaving it implicit would silently remove a
core safety behavior for species that do not select nudge. E-NUDGE must also assume there is **no
nudge config, settings surface or UI** at baseline A; the only retained observable is the
`tool.nudge` Journal event plus reminder injection.

### 2.6 D05 — Settle the warming/steering seam and remove hash-only coupling

D05 was also rewritten by the Lead during this audit. Its seam analysis is correct; several removal
targets are 10-18-only.

| Cited path | State | Disposition / note |
| --- | --- | --- |
| `internal/runtime/cachewarmer.go`, `cachewarmer_test.go` | E | Modify only where real; see `PrefixSHA256` correction below. |
| `internal/config/config.go` | E (now cited) | Retain `CacheWarming` config/default/parser (`internal/config/config.go:316-325,902-908`). |
| `internal/domain/event.go`, `internal/runtime/cachewarmer.go` (`cache.warmed`) | E | Producer is `cachewarmer.go:217-228` (`persistAndPublish(domain.EventCacheWarmed, ...)`); event type is `internal/domain/event.go:82-85`. |
| `internal/runtime/model_stream_observer.go` | E | **Correction:** this file does not produce or consume `cache.warmed`; it only comments that `cache.warmed` is outside its observed set (`model_stream_observer.go:91`). The plan's claim that it "produce/consume[s] `cache.warmed`" is wrong. |
| `internal/runtime/queue.go`, `internal/runtime/service.go` | E | Modify; dual-track queue and steer lane are in `queue.go` (`steer` lane, `QueueModeAll`/`QueueModeOneAtATime`, `Steer`, `SetQueueMode`, demotion, boundary cancel), scheduling in `service.go` (`steerCancels`/`steerArmed`). |
| `internal/runtime/queue_steer.go` | M | **(a) redirect to `internal/runtime/queue.go`.** The path never existed at 10-18 either; D05 already says so. Remove the file reference from any prose that implies it exists. |
| `internal/runtime/toolmount_middleware.go`, `schemas/events/payloads/tool.mounted.json` | E | Inspect-only item; both exist. |
| `sdk/port/loopexp/warming.go`, `sdk/port/loopexp/steering.go`, `internal/runtime/extensions/warming/adapter.go`, `internal/runtime/extensions/steering/adapter.go` | P | (d) create; absent at both trees. |
| `plugins/exp/warming/`, `plugins/exp/steering/`, `recipes/exp-warming.vivy.yml`, `recipes/exp-steering.vivy.yml`, `recipes/exp-loop-features.vivy.yml` | P | (d) owned by E-WARMING/E-STEERING; D05 explicitly does not create them. |
| `ui/src/components/chat/ChatInput.restore.test.tsx` (cited as an existing test) | M | **Replace with `ui/src/components/chat/ChatInput.queue.test.tsx`**, which exists at baseline A. The restore test is IN-10-18. |
| Existing tests `queue_test.go`, `cachewarmer_test.go` | E | Retain/fork. |

**Hard-input corrections for D05:**

1. **`PrefixSHA256` and `prefix_digest_failed` do not exist at baseline A.** Verified: baseline
   `payloadCacheWarmed` has `call_id/provider/model/mode/status/reason/prompt_tokens/cache_write_tokens`
   only (`internal/runtime/payloads.go:194-203`), `cachewarmer.warm` takes a mode string, and no
   `PrefixSHA256`/`prefix_digest_failed` symbol exists. Both symbols are IN-10-18
   (`git show e9609296:internal/runtime/cachewarmer.go`). D05 Task 2 ("delete `PrefixSHA256` and
   hash-only serialization/diagnostics") is therefore **not applicable at baseline A**; either drop
   it or restate it as "do not re-introduce the 10-18 prefix digest".
2. **Warming has three modes here, not two.** Config accepts `off`, `streaming` and `idle`
   (`internal/config/config.go:902-906`); `idle` arms `time.AfterFunc` before cache expiry
   (`cachewarmer.go:143-152`). The shared plan §11.5 and C11 say "off/streaming" and "no timer".
   Preserving current behavior and the §11.5 target conflict; the owner must decide (see §4.5).
3. **"Steering's custom hash binding" is not evidenced at baseline A.** No hash/digest exists in
   `internal/runtime/queue.go`, `internal/domain/turn_queue.go` or the steering paths of
   `internal/runtime/service.go`; the same is true at 10-18 for `queue.go`. If "custom hash binding"
   refers to a different mechanism, it must be located before implementation; otherwise remove that
   Task 3 clause.
4. Warming has **no UI** at baseline A and no `cache.warmed` JSON schema file (event type and payload
   struct exist only). Steering *does* have UI surfaces (`ui/src/lib/api.ts` `steerTurn`/queue state,
   `ChatInput.queue.test.tsx`, i18n queue-lane strings) and RPC handlers (`internal/rpc/control.go:1412`,
   `:3683-3780`). D05's "warming/steering settings and UI consumers" must be split accordingly: warming
   is config/event only; steering has RPC/UI but no config object.

### 2.7 D06 — Delete source, Assembly and UI evidence generation end to end

| Cited path | State | Disposition / note |
| --- | --- | --- |
| `internal/sourcehash/tree.go` | E | Remove package. |
| `sdk/internal/cmd/source-hash/main.go` | E | Remove command. |
| `sdk/module/descriptor.go` | E | Modify; `Source.SHA256` exists (`sdk/module/descriptor.go:41-45`). |
| `sdk/internal/descriptor_v1.go`, `catalog_v1.go`, `source_verify_v1.go`, `stageui_v1.go` | E | Modify. |
| `sdk/internal/assembly/source.go`, `manifest.go`, `ui.go`, `compiler.go`, `evidence.go` | E | Modify; `SourceCatalog`/`HashSourceTree` (`source.go:62-102`), manifest `GenerationID` is a hex identity (`manifest.go:348`). |
| `sdk/internal/conformance` | E | Modify consumers. |
| `sdk/generation/manifest.go` | E | Modify `GenerationID` accessor. |
| `internal/moduleport/ports.go` | E | Extend closed catalog; current entries include `core/cognitive-factory@v1` and no loop-exp factory. |
| `sdk/internal/assembly/runtime_generate.go` | E | Extend existing named-factory emission (`HasMaskFactory`/`MaskServiceFactoryValue`, `HasCognitiveFactory`/`CognitiveFactoryValue`). |
| `ui/src/plugins/presentation-host.tsx` | E | Modify; validates catalog digest with `sha256Hex` (`presentation-host.tsx:1202,1292`). |
| `ui/scripts/stage-ui-assembly.mjs` | E | Modify. |
| `scripts/check-plugin-v1-fixtures.mjs` | E | Modify. |
| `scripts/release-conformance.mjs` | M | **(b) remove.** IN-10-18. |
| `scripts/memory-loop.mjs` | M | **(b) remove.** IN-10-18. |
| `scripts/diva-recall.mjs` | M | **(b) remove.** IN-10-18. |
| `internal/app/assembly_loop_extensions.go` | M | **(d) proposed-create.** Absent at both trees; D06 already labels it proposed. |
| `internal/studiocore/ledger.go`, `internal/studiocore/service.go` | E | Modify Studio ledger fragment; `artifact_sha256` install/ledger gate exists (`internal/studiocore/service.go:421`). |
| `internal/generated/assembly/zz_default.go`, `ui/src/generated/assembly.ts` | E | Regenerate, not hand-edit. |
| `docs/architecture/VIVY-ASSEMBLY.md`, `VIVY-PLUGIN-SPEC.md`, `VIVY-MODULE-STANDARD.md` | E | Update affected clauses. |
| `schemas/events/payloads/tool.mounted.json` | E | Retain selection semantics. |
| `.workspace/issue91-minimal-artifact` (verification output) | P | (d) working directory; `.workspace/` is absent. |

Baseline A's `scripts/` is much smaller than 10-18's (no `build-split.mjs`, `docker-build.mjs`,
`doctor.mjs`, `fmt-check.mjs`, `dev.sh`, `lane-new.mjs`). Any D06 task that assumes those exist must
be re-anchored to the baseline command set (`just ci`, `just build-split` target, `ui` package
scripts).

### 2.8 D07 — Replace bundle and toolkit content addressing with owned releases

| Cited path | State | Disposition / note |
| --- | --- | --- |
| `internal/bundlecontract/{identity,manifest,catalog,operations,snapshot}.go`, `internal/bundlecontract/digest.go` | M | **(b) remove the whole story scope.** IN-10-18; no `internal/bundlecontract/` at baseline A. |
| `internal/bundles/{acquire,artifact,manager}.go` | M | **(b) remove.** IN-10-18. |
| `internal/storage/bundles.go`, `internal/storage/bundlesql/bundlesql.go` | M | **(b) remove.** IN-10-18. |
| `internal/toolkitcontract/actions.go`, `internal/modules/toolkits/actions.go`, `internal/actionhost/toolkits.go` | M | **(b) remove.** IN-10-18; the word "toolkit" does not occur in any baseline-A Go file. |
| `internal/modules/masks/bundles.go` | M | **(b) remove.** IN-10-18; no mask-bundle metadata at baseline A. |
| `internal/studiocore/service.go` `ArtifactSHA256` lifecycle gate | E | Retain, but it is already inside D06's file list; do not duplicate ownership. |
| "Create paired incremental SQL for release/artifact references" | non-path | Not applicable: no bundle/release tables exist in baseline-A migrations; nothing to re-key. |

**C5 becomes not applicable at baseline A** (see §3). D07 should be reduced to: (1) D06 owns the
Studio ledger `artifact_sha256` removal; (2) the bundle/toolkit acquisition, consent, catalog and
resource-release contract is a 10-18 feature that baseline A does not have, so its deletion is not a
valid Story here. Re-introducing that subsystem later would be a separate gated track.

### 2.9 D08 — Delete Host and admitted-snapshot hashes, bind checkpoints directly

| Cited path | State | Disposition / note |
| --- | --- | --- |
| `internal/contexthost/{filesource,host}.go` | E | Modify. |
| `internal/skillhost/host.go` | E | Modify. |
| `internal/mcphost/{resourcesource,host,toolworld}.go` | E | Modify. |
| `sdk/port/skillsource/skillsource.go` | E | Modify. |
| `internal/runtime/prompt_state.go`, `prompt_middleware.go`, `request.go`, `service.go`, `checkpoint.go`, `checkpointadapter.go` | E | Modify; `RunPromptSnapshot` has `PayloadSHA256` (`internal/storage/masks.go:50-57`); checkpoint envelope has `checksum_sha256` and prompt identity `payload_sha256` (`internal/runtime/checkpoint.go:41-52`). |
| `internal/runtime/bundle_admission.go` | M | **(c) merge into D08's `internal/runtime/prompt_state.go` + `service.go`.** IN-10-18 split; baseline A has no separate bundle-admission file (the word "bundle" in baseline runtime code refers to the cognitive bundle / permission preset, not an artifact bundle). |
| `internal/runtime/contextsource_admission.go` | M | **(c) merge into D08's `internal/runtime/contextadapter.go` + `service.go`.** IN-10-18 split. |
| `internal/domain/policy.go` | E | Modify prompt/toolworld snapshot DTO consumers. |
| `internal/storage/admission_validation.go` | E | Modify. |
| `internal/storage/sqlite/run_admission.go`, `internal/storage/postgres/run_admission.go`, `blob.go` | E | Modify; checkpoint blob row stores `checksum` (`internal/storage/sqlite/blob.go:63`). |
| `schemas/events/payloads/model.request.json`, `policy.evaluated.json`, `run.started.json`, `context.reference_attached.json` | E | Modify affected run/model/policy/context payloads. |

All other cited paths exist; C6 is applicable.

### 2.10 D09 — Remove file, history, reference and deliverable hash authority

| Cited path | State | Disposition / note |
| --- | --- | --- |
| `internal/runtime/{filesystem_backend,fileversion_recorder,history_service,history_authority,reference_service,deliverable_service,deliverable_transfer,download}.go` | E | Modify; HMAC cursor + random-key SHA fallback (`history_service.go:84-90,669-692`), `PresentFileBytes`/`PresentSetBytes` (`deliverable_service.go:219-224`). |
| `internal/domain/{tool,review,history,context_reference,deliverable}.go` | E | Modify; `ScopeHash`, `FilterHash`, `ExpectedDigest`, `SelectionDigest` (`internal/domain/history.go:157,184,207,326,433,630`). |
| `internal/storage/continuity.go` | E | Modify; `ContinuityStore` exists (`internal/storage/continuity.go:139`). |
| `internal/storage/sqlite/fileversions.go`, `sqlite/continuity.go`, `postgres/fileversions.go`, `postgres/continuity.go` | E | Modify storage contracts/adapters. |
| `internal/tools/{references,download}.go` | E | Modify; `ExpectedDigest` (`internal/tools/references.go:40`). |
| `internal/rpc/{exports,deliverables,references,control}.go` | E | Modify; `expected_digest` protocol fields exist (`internal/rpc/exports.go:20-24,62`; `internal/rpc/deliverables.go:32`). |
| `ui/src/lib/{api,deliverable-download,store}.ts`, `ui/src/components/chat/HistoryPicker.tsx`, `HistoryPicker.test.tsx`, `deliverable-download.test.ts` | E | Modify/retain tests. |
| Context/reference schemas and SDK clients | E | `context.reference_attached.json` exists. |

No missing path. C7/C8 applicable.

### 2.11 D10 — Preserve skill, memory and cognitive revision CAS without hashes

| Cited path | State | Disposition / note |
| --- | --- | --- |
| `internal/tools/skills.go`, `internal/rpc/control.go` | E | Modify. |
| `internal/domain/skill.go` | E | Modify; `BeforePayload`/`BaseHash`/`ContentHash` exist (storage scans them at `internal/storage/sqlite/skill_revisions.go:20-59`). |
| `internal/runtime/{skills_backend,skilladapter,always_skills,cognitive_binding,cognitive_service}.go` | E | Modify. |
| `internal/runtime/cognitive_capture_source.go` | M | **(c) merge into `internal/runtime/cognitive_service.go` + `cognitive_binding.go`.** IN-10-18 split; no equivalent separate file at baseline A. |
| `internal/cognitivecontract/ports.go` | E | Modify. |
| `internal/maskcontract/masks.go` | E | Modify; `Digest` on Definition/Snapshot/Metadata and `CreateRequestDigest`/`DefinitionDigest` exist (`internal/maskcontract/masks.go:154,164,191,374-411`). |
| `internal/modules/masks/{catalog,service,actions}.go` | E | Modify. |
| `internal/modules/masks/bundles.go` | M | **(b) remove.** IN-10-18 (mask bundle metadata). |
| `internal/modules/{memory,diva-cognitive}` | E | Modify; memory CAS is a content digest (`internal/modules/memory/service.go:192-216`); DIVA strategy digest is INOFY-derived (`internal/modules/diva-cognitive/factory.go:82-91`). |
| SQLite/PostgreSQL `skill_revisions.go`, `masks.go` | E | Modify adapters. |
| `plugins/vivy-memory`, `plugins/vivy-evolution`, `plugins/vivy-masks-ui` | E | Modify clients. |
| `ui/src/lib/demo-api.ts` | E | Modify demo CAS; demo `content_hash`/`base_hash` fixtures exist (`demo-api.ts:148-395`). |
| `laputa-source.lock.json` / Laputa pin | E (lock file) | External repository and pin content not verified; D10 remains upstream-blocked as the plan states. |

C9 is applicable for skill/mask/memory/DIVA. Notebook/toolkit clauses move to D11/(b).

### 2.12 D11 — Remove Work, workflow, notebook and report business digests

| Cited path | State | Disposition / note |
| --- | --- | --- |
| `internal/domain/{workflow,work_control,child_session}.go` | E | Modify; workflow digests (`DescriptorDigest`/`AuthorityDigest`/`ProgramDigest`/`CatalogDigest`/`InputDigest`, `internal/domain/workflow.go:12-25`), child `RequestDigest`/`AuthorityCeilingDigest` (`internal/domain/child_session.go:60-64`). |
| `internal/runtime/{workflow_service,workflow_product,inofy_admission,work_control,work_control_tools,child_sessions}.go` | E | Modify; INOFY digests and `inofy-normal-v1` input normalization exist (`internal/runtime/workflow_service.go:195`); `model-work-` IDs are SHA-derived (`internal/runtime/work_control_tools.go:106-108`). |
| `internal/runtime/report_admission.go`, `report_executor.go`, `report_effects.go` | M | **(b) remove.** IN-10-18; no report feature at baseline A (no report Go file, contract or plugin). |
| `internal/notebookcontract/types.go` | M | **(a) rename to the existing note owners:** `internal/domain/note.go`, `internal/tools/notes.go` (plus `writenote.go`), `internal/storage/sqlite/notes.go`, `internal/storage/postgres/notes.go`, `plugins/vivy-notebook`. Baseline A calls the feature "notes", not "notebook". |
| `internal/reportcontract/types.go` | M | **(b) remove.** IN-10-18. |
| `internal/storage/*` `work_control`, `child_sessions`, `workflow_*`, `notes` adapters | E | Modify; all exist (report adapters do not). |
| `internal/rpc/work_control.go` | E | Modify. |
| `plugins/vivy-workflow`, `plugins/vivy-notebook`, `ui/src/lib/workflow-api.ts` | E | Modify; all exist. |
| `ui/src/lib/workflow-api.test.ts` | E | Retain/update. |
| "report admission/generations/publication" storage | M | **(b) remove** with the report family. |

C9 narrows: workflow/Work/child-session/notes digests are real at baseline A; report and toolkit are
not; "notebook" must be renamed to notes.

### 2.13 D12 — Remove migration checksums and convert the independent BML format

| Cited path | State | Disposition / note |
| --- | --- | --- |
| `internal/storage/migrations/{manifest,runner}.go`, `manifest_test.go`, `runner_test.go` | E | Modify; checksum metadata exists. |
| `internal/storage/migrations/metadata/{sqlite,postgres}.sql` | E | Modify. |
| `bml/{record,home,maintenance,migrate,store}.go`, `bml/README.md` | E | Modify; BML artifact schema version and record digests exist (`bml/store.go:181-315`; `bml/migrate.go:468-539`), as does `memory_apply_journal` (`bml/store.go:281`). |
| `bml/go.mod` (independent module) | E | Present (verified); `(cd bml && go test ./...)` is a valid command shape. |
| "Paired target schema migrations … do not reserve conflicting 044" | non-path | Re-anchor: baseline head is 035, last "reserved" language should move to 036+. |
| Released-017 / legacy PostgreSQL suites | E | Fixtures exist in baseline tests; only the numeric head changes. |

C10 applicable. No missing path.

### 2.14 D13 — Ship usable minimal, a tiny real-token Face and runnable examples

| Cited path | State | Disposition / note |
| --- | --- | --- |
| `recipes/minimal.vivy.yml` | E | Modify. |
| `recipes/lite.vivy.yml` | E | Compare without changing `recipes/default.vivy.yml` (exists). |
| `recipes/minimal-chat.vivy.yml` | P | (d) create. |
| `faces/minimal-chat/` | P | (d) create; baseline `faces/` contains `headless` and `tui`. The reuse option is real: `faces/headless` exists. |
| `sdk/facerun`, `sdk/port/face` | E | Reuse. |
| `examples/README.md`, `examples/minimal-chat/README.md`, `examples/minimal-plus-hello-fs/README.md` | P | (d) create; `examples/` does not exist at baseline A or at 10-18. |
| `.workspace/issue91-usable-minimal` (verification output) | P | (d) working directory; `.workspace/` absent. |

No 10-18-only modification target. D13 remains valid after the C4/D06 and C11 dependencies land.

### 2.15 D14 — Accept one final candidate through independent behavior and residue audits

| Cited path | State | Disposition / note |
| --- | --- | --- |
| Read-only audits of all changed surfaces | — | No concrete path to verify. |
| `docs/logs/<delivery-date>-issue-91-acceptance/{summary,verification,acceptance}.md` | P | (d) create at delivery time (new iteration log directory per AGENTS.md). |
| Verification commands `just ci`, `just test-postgres`, `git diff --check`, `(cd bml && go test ./...)` | E (targets/module) | Present at baseline A (29 justfile targets include `ci`, `test-postgres`, `headless-compile`; `bml/go.mod` exists). |
| Verification command `node scripts/memory-loop.mjs` / `just diva-recall-test` (from the 10-18 tree) | M | **(b) remove/replace.** No such script or just target at baseline A; no `diva-recall`/`memory-loop` reference exists outside docs. |
| `ui` test commands (`pnpm --dir ui test -- …`) | E | Listed UI test files exist (`store.test.ts`, `api.test.ts`, `deliverable-download.test.ts`, `HistoryPicker.test.tsx`, `demo-api.test.ts`, `workflow-api.test.ts`); `ui/package.json` has `stage:ui`, `typecheck`, `test`. `ChatInput.restore.test.tsx` is 10-18-only. |

D14 has no missing source path; only its 10-18-era command list needs pruning.

### 2.16 Master table: every missing cited path and its disposition

| # | Missing path at baseline A | 10-18? | Disposition |
| --- | --- | --- | --- |
| 1 | `internal/runtime/nudge_window.go` | IN | (a) → `internal/runtime/loopguard.go` (+ `nudge_state.go` field); FNV/hard stop live there |
| 2 | `internal/runtime/queue_steer.go` | ABSENT | (a) → `internal/runtime/queue.go`; path never existed |
| 3 | `internal/runtime/bundle_admission.go` | IN | (c) → D08 `internal/runtime/prompt_state.go` + `service.go` |
| 4 | `internal/runtime/contextsource_admission.go` | IN | (c) → D08 `internal/runtime/contextadapter.go` + `service.go` |
| 5 | `internal/runtime/cognitive_capture_source.go` | IN | (c) → D10 `cognitive_service.go` + `cognitive_binding.go` |
| 6 | `internal/notebookcontract/types.go` | IN | (a) → `internal/domain/note.go`, `internal/tools/notes.go`, storage `notes.go` |
| 7 | `internal/toolkitcontract/actions.go` | IN | (b) remove (toolkit subsystem absent) |
| 8 | `internal/modules/toolkits/actions.go` | IN | (b) remove |
| 9 | `internal/actionhost/toolkits.go` | IN | (b) remove |
| 10 | `internal/storage/bundles.go` | IN | (b) remove |
| 11 | `internal/storage/bundlesql/bundlesql.go` | IN | (b) remove |
| 12 | `internal/bundles/{acquire,artifact,manager}.go` | IN | (b) remove |
| 13 | `internal/bundlecontract/{digest,identity,manifest,catalog,operations,snapshot}.go` | IN | (b) remove |
| 14 | `internal/modules/masks/bundles.go` | IN | (b) remove |
| 15 | `internal/reportcontract/types.go` | IN | (b) remove |
| 16 | `internal/runtime/report_admission.go` | IN | (b) remove |
| 17 | `internal/runtime/report_executor.go` | IN | (b) remove |
| 18 | `internal/runtime/report_effects.go` | IN | (b) remove |
| 19 | `plugins/exp/loop-detection/deferred.go` | IN | (b) remove (whole loop-detection experiment package absent) |
| 20 | `scripts/release-conformance.mjs` | IN | (b) remove |
| 21 | `scripts/diva-recall.mjs` | IN | (b) remove |
| 22 | `scripts/memory-loop.mjs` | IN | (b) remove |
| — | symbol `VerifyUnchanged` (C4 remove list) | IN | (b) only `HashSourceTree` exists here |
| — | `prefix_digest_failed` / `PrefixSHA256` (D05 task) | IN | (b) not present here |
| — | `ui/src/components/chat/ChatInput.restore.test.tsx` (D05 test) | IN | (a) → `ChatInput.queue.test.tsx` |
| — | `internal/runtime/tool_continuation_test.go` (D04 test) | ABSENT | (d) proposed test |

Proposed-create (d), absent at both trees, to be created by the named Story/epic:
`internal/runtime/extensions.go`; `internal/runtime/extensions/{nudge,warming,steering}/adapter.go`;
`sdk/port/loopexp/{nudge,warming,steering}.go`; `internal/app/assembly_loop_extensions.go`;
`plugins/exp/{nudge,warming,steering}/`; `recipes/exp-{nudge,warming,steering,loop-features}.vivy.yml`;
`recipes/minimal-chat.vivy.yml`; `faces/minimal-chat/`; `examples/` and its three READMEs;
`internal/runtime/completion_protocol_test.go`; `sdk/facerun/completion_protocol_test.go`;
`internal/app/assembly_audit_test.go`; `internal/runtime/tool_continuation_test.go`;
`docs/logs/<delivery-date>-issue-91-acceptance/`; `.workspace/issue91-*` outputs.

## 3. C1–C11 mapping at baseline A

Status vocabulary: **still applicable** (owner/field exists and the deletion target is real here),
**merge** (responsibility exists only inside a larger owner here), **not applicable at 10-07** (the
mechanism was added after baseline A), **needs re-scope** (part of the row exists, part does not, or
the baseline behavior differs from the assumed contract).

| Row | Plan's referenced owners / fields | Exists at baseline A? | Status | Concrete evidence |
| --- | --- | --- | --- | --- |
| C1 | model completion v3 `{byte_len}`; `completeModelLocked`; `digestModelRequest`; `completedProjectionContent`; Facers; historical v2 normalization | Yes | **Still applicable** | v2 payload requires `content_sha256`+`byte_len` (`internal/runtime/payloads.go:132-133`); projector rejects non-v2 and validates digest (`internal/runtime/message_projector.go:163-196`); request digests (`internal/runtime/request.go:7-41`); Facers/RPC consumers exist (`sdk/facerun/facerun.go:336-356`, `sdk/tui/stream/events.go:63-106`, `internal/rpc/control.go:3920,4237`). v1 lives only in the schema's legacy branch and is already rejected by the projector. |
| C2 | `domain.ToolOperation` `RequestDigest`/`ArgumentsDigest`/`MiddlewareInputArguments`/`EffectiveArguments`; `CompleteToolOperation` call shape; private `(RunID, OperationID)` CAS in SQLite/PostgreSQL | Yes | **Still applicable** | `internal/domain/tool_operation.go:24-40`; SQLite migration `031_tool_operations.sql` has all four columns; `sameToolOperationBinding` compares the digest (`internal/storage/sqlite/tool_operations.go:35-60`; runtime use `internal/runtime/tool_operation.go:89-108`). |
| C3 | private approval envelope binding exact intended arguments; redacted preview; effectful execution fails closed on stale binding | Yes (v1, hash-based) | **Still applicable** | `toolApprovalProposalEnvelope` kind v1 with `arguments_sha256` and `bindToolApprovalProposal` (`internal/runtime/approval_binding.go:10-40`); approvals storage carries `precondition_hash`/`proposal_data` (SQLite/PostgreSQL approval DDL); `unbindToolApprovalProposal` fails closed on unbound/legacy data. |
| C4 | `SourceCatalog`, `HashSourceTree`, `source-hash` CLI, Descriptor SHA, `SourceRecord` projection digests, `GenerationID` | Yes, except `VerifyUnchanged` | **Still applicable, needs re-scope of GenerationID semantics** | `SourceCatalog`/`HashSourceTree` (`sdk/internal/assembly/source.go:62-102`); `Descriptor.Source.SHA256` (`sdk/module/descriptor.go:41-45`); `sdk/internal/cmd/source-hash/main.go`; `headless generation` manifest sets `GenerationID = hex(identity[:])` (`sdk/internal/assembly/manifest.go:348`). Baseline A derives GenerationID from content, so C4's "allocate one opaque ID at pack" is a semantic change plus deletion, not a pure rename. `VerifyUnchanged` is absent (10-18-only). |
| C5 | `internal/bundlecontract/*`, `internal/bundles/*`, `internal/storage/bundles[ql]`, bundle/toolkit catalog Epoch/ConfigRevision CAS, `(InstallID, ReleaseSequence)` identity, artifact consent/revocation | **No subsystem** | **Not applicable at 10-07** | No `internal/bundlecontract`, `internal/bundles`, `internal/storage/bundles*`, no bundle table in baseline migrations, and "toolkit" appears in no Go file. The only surviving hash-bearing release artifact is the Studio ledger `artifact_sha256` (`internal/domain/generation.go:47`, `internal/studiocore/service.go:421`), already owned by D06. |
| C6 | `RunPromptSnapshot` RunID/SchemaVersion/ComposerVersion/GenerationID; checkpoint outer format v2; engine-version/foreign-Run fail-closed; blob generation/pointer | Yes | **Still applicable** | `RunPromptSnapshot` (`internal/storage/masks.go:50-57`) with `PayloadSHA256`; checkpoint envelope `checksum_sha256` + prompt identity `payload_sha256` and engine-version checks (`internal/runtime/checkpoint.go:41-107`); checkpoint blob checksum column (`internal/storage/sqlite/blob.go:63`). |
| C7 | history cursor v2 phase/after/cut; server-scoped permissions; remove HMAC/random key/fallback, ScopeHash/FilterHash/SelectionDigest; reference attachment binds selected text | Yes | **Still applicable** | HMAC cursor plus random-key/SHA fallback (`internal/runtime/history_service.go:84-90,669-692`); `ScopeHash`/`FilterHash`/`SelectionDigest`/`ExpectedDigest` (`internal/domain/history.go:157,184,207,326,433`); `historyCursorState` carries ScopeHash/FilterHash (`internal/runtime/history_service.go:485`); reference attach `ExpectedDigest` (`internal/tools/references.go:40`). |
| C8 | file proposals/revisions bind exact bytes; downloads ItemID+TransferID; one bounded presentation snapshot; presentation bytes committed with receipt | Yes, target contract partially absent | **Still applicable** | `PreconditionHash` (approval/review payloads, `internal/runtime/payloads.go:307`; `internal/domain/review.go`); `ExpectedDigest` for deliveries (`internal/rpc/deliverables.go:32`, `internal/rpc/exports.go:20-24`); `PresentFileBytes`/`PresentSetBytes` (`internal/runtime/deliverable_service.go:219-224`); `ContinuityStore` exists (`internal/storage/continuity.go:139`), but the target presentation-byte storage is new. |
| C9 | skill `BeforePayload`/`BaseHash`; memory/notebook/mask/toolkit owner revisions; Work/workflow/report receipts; INOFY digests | Partially | **Needs re-scope** | Skill CAS real (`internal/domain/skill.go`; storage scans); mask digest real (`internal/maskcontract/masks.go:374-411`); memory digest CAS real (`internal/modules/memory/service.go:192-216`); workflow/child/Work digests real (`internal/domain/workflow.go:12-25`, `internal/domain/child_session.go:60-64`, `internal/runtime/work_control_tools.go:106-108`); INOFY input normalization real (`internal/runtime/workflow_service.go:195`). **Not present:** report family (all report files IN-10-18), toolkit contracts, `notebookcontract` (feature is "notes" here). Drop report/toolkit; rename notebook→notes. |
| C10 | BML schema v2 / artifact `2.0.0`; record/workspace IDs; drop `provenance.content_digest`, `tombstone.reason_digest`, `evidence_refs[].hash`, IntegrityReport digests; delete `memory_apply_journal` | Yes (v1) | **Still applicable** | `bml/store.go:181-315` has schema/record metadata and `memory_apply_journal`; `bml/migrate.go:468-539` has `records_digest`/`InputDigest` artifact fields; `bml/go.mod` present; released-017 fixtures exist. |
| C11 | nudge/warming/steering as species-selected EXP packages; `sdk/port/loopexp/*`, `internal/runtime/extensions.go`, `internal/runtime/extensions/{nudge,warming,steering}/`, `plugins/exp/*`, `core/*-factory@v1`, generated Assembly bindings, Recipe selection | Feature bodies yes; seam no | **Needs re-scope** | All three behaviors exist only as unconditional core: `engine.go:376` installs nudge middleware; `service.go:2948,4643` create the run warmer; `queue.go` always owns the steer lane. All seam paths are absent at baseline A **and at 10-18** (checked with `git ls-tree e9609296`); `internal/moduleport/ports.go` has only `core/cognitive-factory@v1` as a factory precedent; `runtime_generate.go` has the named-factory pattern (`HasMaskFactory`/`HasCognitiveFactory`). §11 contract differs from baseline behavior in warming modes, nudge loop stop, and settings/UI presence (§4.5). |

## 4. Consequences for the retained experimental packages and their epics

### 4.1 What exists at baseline A (the epics' extraction source)

| Package | Implementation | Wiring | Configuration / settings | Events / UI | Persistence |
| --- | --- | --- | --- | --- | --- |
| Nudge | `internal/runtime/loopguard.go` (FNV `loopWindow`, 10-step window, repeat limit 5, `errLoopDetected`), `nudge_state.go` (batch/marks/window/notice, thresholds 3/5), `nudge_middleware.go` (`WrapModel`, reminder injection, 1024-byte and context-budget limits), `prompt.go` (`renderNudge` templates) | Unconditional: `engine.go:376`; `service.go:2953,4648` create per-leg state | **None** (no config file entry, no settings, no UI) | `tool.nudge` schema v1 + `domain.EventNudge`; no UI consumer | Journal event only |
| Warming | `internal/runtime/cachewarmer.go` (`runCacheWarmer`, eligibility gate, savings floor, in-flight guard, idle `time.AfterFunc`, accounting), `payloadCacheWarmed` | Unconditional construction in `service.go:2948,4643` (nil when config off / model not warmable) | `internal/config/config.go` `CacheWarming` (`off`/`streaming`/`idle`) + `CacheWarmingMinSavingsUSD`; **no UI** | `cache.warmed` event type (`domain/event.go:82-85`) produced at `cachewarmer.go:217-228`; no JSON schema file; no UI | Journal event only; no prefix hash at baseline |
| Steering | `internal/runtime/queue.go` dual-track queue (`steer`/`followUp` lanes, modes `all`/`one-at-a-time`, `Steer`, `SetQueueMode`, demotion, boundary-safe cancel), `service.go` `steerCancels`/`steerArmed` | Always active (single core path) | No config object; mode is per-session RPC state | `turn.queued`/`turn.dequeued`/`turn.steered` events; RPC `turn/steer`/queue state (`internal/rpc/control.go:1412,3683-3780`); UI `api.ts` `steerTurn`, `ChatInput.queue.test.tsx`, i18n strings | Journal markers only; no SQL table; no `queue_steer.go` |

### 4.2 §11 references that remain proposed

At baseline A, **every** file the plan uses to build the loop-exp seam is absent. None is a 10-18
artifact to modify; each must be created:

| §11 reference | State at baseline A | State at 10-18 (checked) | Owner |
| --- | --- | --- | --- |
| `sdk/port/loopexp/{nudge,warming,steering}.go` | Absent | Absent | D04 creates nudge; D05 warming/steering |
| `internal/runtime/extensions.go` | Absent | Absent | D04 |
| `internal/runtime/extensions/{nudge,warming,steering}/adapter.go` | Absent | Absent | D04/D05 create the slots; adapters land with each epic/Seam |
| `internal/app/assembly_loop_extensions.go` | Absent | Absent | D04/D05 |
| `core/nudge-factory@v1`, `core/warming-factory@v1`, `core/steering-factory@v1` catalog entries | Absent (only `core/cognitive-factory@v1`) | Absent | D04/D05 |
| Generated bindings for the three factories | Absent (only Mask/Cognitive factory precedent) | Absent | D04/D05 extend `runtime_generate.go` |
| `plugins/exp/{nudge,warming,steering}/` | Absent | Absent | E-NUDGE / E-WARMING / E-STEERING |
| `recipes/exp-{nudge,warming,steering,loop-features}.vivy.yml` | Absent | Absent | Epics |
| `faces/minimal-chat/`, `recipes/minimal-chat.vivy.yml`, `examples/` | Absent | Absent | D13 |

The only 10-18 file in this area is `internal/runtime/nudge_window.go`, and it is a **core
implementation split** of baseline A's `internal/runtime/loopguard.go`, not a Port/adapter. There are
**no “10-18 adapter files”** to port: the seam was designed but never materialized in either tree.

### 4.3 What the epic plans must assume

1. **They create the seam, they do not extend one.** E-NUDGE/E-WARMING/E-STEERING must assume
   `sdk/port/loopexp`, `internal/runtime/extensions.go`, the three factory catalog entries, the
   generated bindings and `internal/app/assembly_loop_extensions.go` arrive from D04/D05 (or are
   created by the epic if it is reordered). Nothing can be "modified" in place.
2. **Dependencies are real and large.** D04 depends on D02/D03/D06; D05 on D04; each epic is blocked
   until the seam and the shared C4/D06 selection machinery land. At baseline A, C4 selection is
   still the hash/evidence-based Source Catalog with `GenerationID = hex(identity)`; physical omission
   cannot be proven until D06 changes it.
3. **Nudge extraction source is four files, not two.** `loopguard.go` (FNV window **and** loop-detection
   hard stop), `nudge_state.go` (batch/seal/notice), `nudge_middleware.go` (injection/limits),
   `prompt.go` (`renderNudge`). The epic must decide and record whether the hard stop stays in core.
4. **Nudge has no settings or UI to preserve.** At baseline A the only user-visible artifacts are the
   reminder injection and the `tool.nudge` Journal event. Any "settings/UI parity" in the epic is new
   work, not preservation.
5. **Warming has three configured modes.** `off`/`streaming`/`idle`; `idle` is timer-driven. The
   shared plan's §11.5 "no timer / off vs streaming" target is narrower than baseline behavior. The
   epic must either preserve `idle` or obtain an explicit owner contract change; it cannot be treated
   as a pure refactor. Warming has no prefix digest at baseline, so hash removal is not part of its
   extraction.
6. **Steering is `queue.go` plus `service.go`,** with Journal-only state, per-session modes and UI/RPC
   consumers. No `queue_steer.go` exists and no SQL migration is implied by extraction. No "custom
   hash binding" was found in the cited steering files at either tree.
7. **Factory/selection pattern precedent is narrow.** The only working named-factory path is
   `core/cognitive-factory@v1` → `HasCognitiveFactory`/`CognitiveFactoryValue` in generated assembly.
   The epics must follow that exact pattern and must keep the Eino import quarantine intact
   (`TestEinoImportsQuarantined`, `TestPluginWindowCannotImportInternal` exist at
   `internal/app/importlint_test.go:86,126`).
8. **Migration IDs.** Nothing in the three packages requires a schema change at baseline A, but if an
   epic adds one, the ID is above **035**, not 043.
9. **C11 physical omission belongs to the epics only.** Until each epic lands, #91 keeps one core
   implementation; D04/D05 must report `Has*Factory == false`/nil and must not add placeholder
   providers or no-op middleware.

### 4.4 Epic-blocking mismatches to resolve before implementation

| # | Mismatch | Evidence | Consequence |
| --- | --- | --- | --- |
| 1 | Nudge hard-stop ownership (loop detection fused with nudge window) | `internal/runtime/loopguard.go:22-51`; `nudge_state.go:69,267-290` | Deleting/moving the window could remove a core safety stop or make it package-dependent; §11.4 and C11 do not name it |
| 2 | Warming `idle` mode vs §11.5 "no timer" | `internal/config/config.go:902-906`; `cachewarmer.go:143-152` | Either the plan is wrong or the product loses an existing mode; needs owner decision |
| 3 | D05 hash-removal tasks (`PrefixSHA256`, steering "custom hash binding") target 10-18-only or unlocated code | `git show e9609296:internal/runtime/cachewarmer.go`; no hash in baseline/10-18 `queue.go` | Task is a no-op or mis-scoped; must be deleted or reworded |
| 4 | D05 claims `model_stream_observer.go` produces/consumes `cache.warmed` | `model_stream_observer.go:91` vs `cachewarmer.go:217-228` | Correct the owner reference; no functional change |
| 5 | D04 claims the window lives in `nudge_state.go` and "retains nudge UI consumers" | `loopguard.go`; no nudge UI in `ui/` | If uncorrected, FNV survives the edit and a non-existent UI consumer is "retained" |
| 6 | D07 assumes a bundle/toolkit subsystem | all bundle/toolkit paths IN-10-18; no bundle tables | C5/D07 not applicable; scope must shrink or be withdrawn |
| 7 | Migration head 043 in plan/D00/D03/D07–D12/§11.7 | baseline heads 035; 10-18 heads 043 | Finite matrix, upgrade fixtures and "above 043" allocation are wrong for this tree |
| 8 | D10/D14 verification commands use 10-18 scripts/targets | `scripts/memory-loop.mjs`, `scripts/diva-recall.mjs`, `just diva-recall-test` absent | Commands must be replaced with baseline gates (`just ci`, `just test-postgres`, focused `go test`, `bml` module tests) |
| 9 | D06 "Modify generation scripts" lists 10-18-only scripts | `scripts/release-conformance.mjs` etc. absent | Remove those rows; baseline scripts are `check-plugin-v1-fixtures.mjs` + `ui/scripts/stage-ui-assembly.mjs` |
| 10 | C4 Generation ID semantics | `manifest.go:348` hash-derived identity | C4 is a design change, not a deletion-only row; D06 must state the new allocation contract explicitly |

## 5. Open questions and limits

### 5.1 Open questions

1. **Loop-detection ownership (D04/E-NUDGE).** Is the `errLoopDetected` hard stop a retained core
   safety behavior with an exact-tuple implementation in `loopguard.go`, or does it move to the nudge
   package? The plan currently implies the latter by "replacing the FNV window" without saying where
   the hard stop goes. This determines whether unselected species keep loop detection.
2. **Warming `idle` mode (D05/E-WARMING).** Does the retained contract include the timer-based `idle`
   mode? If §11.5's "no timer" is authoritative, the owner is approving a behavior reduction that the
   current owner-scope note ("prospective warming") does not mention.
3. **Steering "custom hash binding" (D05).** It was not found in `queue.go`, `turn_queue.go` or the
   service steering paths at either tree. What exact field/helper is meant? If none, the D05 Task 3
   clause should be removed.
4. **C5/D07 disposition.** Should D07 be marked "not applicable at baseline A" in the index and its
   content retained only as a future gate, or folded into D06's Studio ledger work? The index still
   lists D07 as an active Story with D03/D06 predecessors.
5. **`notebook` vs `notes`.** D11's notebook-contract scope must be renamed to the baseline note
   owners; it is not clear whether the 10-18 notebook contract also covered behavior baseline A lacks
   (the plan assumes notebook revisions; baseline has note storage and `vivy-notebook`).
6. **Release/date of re-introduced 10-08..10-18 work.** The bundle/toolkit/report subsystems, the
   prefix digest and the restore test are not present here. If those features are re-introduced later
   as gated work, their deletion targets return to scope; this audit treats them as out of scope now.
7. **Laputa/INOFY pins.** D10/D11 remain upstream-blocked. `laputa-source.lock.json` exists, but the
   external repositories were not inspected here, so the claim "current pin requires digests" is
   unverified at this baseline.
8. **Concurrent edits.** D04/D05/index/plan/TODO were modified by the Lead during this audit. The
   corrections in §2.5/§2.6/§4.4 apply to the versions read at audit time; re-check them after the
   Lead's final edit.

### 5.2 Limits (explicit)

- **No runtime code was executed or verified.** No `go build`, `go test`, `just` target, `pnpm`
  script, `git diff --check` on the plan package, or CI run supports any behavior claim here.
- **No real-path behavior was verified.** The 10-07 baseline is described as stable and benchmarked,
  but this audit did not run any model/tool loop, provider call, RPC, UI path or database migration.
- **No data verification.** SQLite/PostgreSQL schemas, released-migration markers, BML artifacts and
  conversion behavior were inspected only as text; no store was opened.
- **No generated artifacts.** Assembly/UI generation output, `internal/generated/assembly/zz_default.go`
  correctness and packed artifacts were not inspected beyond presence.
- **No external repositories.** `studio/` submodule content, Laputa and INOFY sources were not read.
- **Symbol-level scope.** C-row evidence proves the named type/field/function exists; it does not prove
  every consumer or failure path is unchanged between 10-07 and 10-18.
- **Presence classification only.** Dispositions (a)/(b)/(c) are based on existence plus the audited
  symbols; the plan owner must still decide Story-level scope (see §5.1) before implementation.

## 6. Verification record for this document

Commands actually run for this audit:

```powershell
# baseline identity and tree equality
git show -s --format='%H %ci %s' 202aac38
git diff --stat 202aac38 HEAD
git merge-base --is-ancestor 202aac38 HEAD
git ls-files | Measure-Object -Line

# header/migration heads
Get-ChildItem internal\storage\migrations\sqlite -Name | Sort-Object | Select-Object -Last 1
Get-ChildItem internal\storage\migrations\postgres -Name | Sort-Object | Select-Object -Last 1

# cited-path existence (all cited paths, expanded; representative)
Test-Path -LiteralPath internal/runtime/nudge_window.go
Test-Path -LiteralPath internal/bundlecontract
Test-Path -LiteralPath examples

# 10-18 classification
git ls-tree -r --name-only e9609296 -- internal/runtime/nudge_window.go
git ls-tree -r --name-only e9609296 -- internal/bundlecontract/digest.go
git ls-tree -r --name-only e9609296 -- scripts/memory-loop.mjs

# C-row owners (representative)
rg -n "loopWindow|fnv" internal/runtime/loopguard.go
rg -n "RequestDigest|ArgumentsDigest" internal/domain/tool_operation.go internal/storage/sqlite/tool_operations.go
rg -n "ScopeHash|FilterHash|SelectionDigest|ExpectedDigest" internal/domain/history.go internal/runtime/history_service.go
rg -n "RunPromptSnapshot|PayloadSHA256" internal/storage/masks.go internal/runtime/checkpoint.go
rg -n "CacheWarming|PrefixSHA256" internal/config/config.go internal/runtime/cachewarmer.go internal/runtime/payloads.go
rg -n "core/cognitive-factory" internal/moduleport/ports.go
rg -n "TestEinoImportsQuarantined|TestPluginWindowCannotImportInternal" internal/app/importlint_test.go

# this file
git diff --check -- docs/research/2026-10-19-baseline-a-reanchor.md
```

Outcome: document written; `git diff --check` run and clean for this file. No product gate was run,
and none is claimed. The worktree also contains concurrent Lead edits to `docs/TODO.md`,
`docs/plans/2026-10-19-complete-deletion-plan.md`, `D04.md`, `D05.md`, `index.md` and a new
`docs/logs/2026-10-19-baseline-a-epic-split/` directory; this audit did not modify them.
