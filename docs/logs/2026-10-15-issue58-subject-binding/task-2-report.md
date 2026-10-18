# S2 Task2 evidence and retention implementation report

Status: implementation and final non-race Task2 gates complete; independent controller review pending. Required race gate remains FAILED on the pinned upstream Eino converter. This report does not claim S3 readiness, PostgreSQL parity acceptance, or centralized S6 CI completion.

Base: `4e57f3ea0954ef20ad598b33245c06cc7fa1f119`. Branch: `feat/issue-58-implementation`. Worktree: `/workspace/scratch/243d1cbb859f/agent-vivy-issue58-dev`. Implementation commit: `2cbb9aefc5fe395a05664430e87071741017cd77`. The subsequent report-only commit is returned in the handoff. Existing human identity `mastwet <1136523089@qq.com>` is used without an AI co-author. No push, merge, deployment, sibling source change or tenant data access occurred.

## Scope and ownership

This increment completes Task1's explicit evidence gaps using the existing Service.Run/shell admission, Journal, deterministic message projector, operation coordinator, BlobStore, Work receipts, core migrations and C2 metadata. It adds no transcript authority, database, engine, scheduler, watermark, product/config flag, dependency change, UI or #66 producer. C2 stores original coordinates and digests; copy provenance stores coordinates only. Original bodies remain in their existing owners.

Root AGENTS and applicable local kernel/Eino guidance were applied. Task2 brief and producer handoff govern this increment. Parent controls plan index/ledger and independent review; those paths are excluded. Centralized just ci and unrelated root go test ./... remain S6 per explicit dispatch. The controller explicitly authorized preparing this stable commit before independent review, despite the original brief's review-first wording and outstanding upstream race.

## Decisions and concrete schema gaps

Released 044 requires a NOT NULL user-message FK on every subject admission. Honest direct shell has no user message. Controller ruled to append paired 045, preserve old stored admissions and accept NULL only for a genuine governed primary/root shell start with actual shell-origin policy/sandbox/approval metadata, no model/provider lifecycle and current trusted source. Ordinary primary OmitMessage remains forbidden and existing child-only safeguards remain. No fake user text was introduced.

SQLite 045 rebuilds both linked metadata tables in one existing migration transaction, copying populated run pins and interactions, then renaming the referenced replacement before the referencing table. Restrictive existing FKs remain. Populated044 upgrade, foreign_key_check, reopen/reapply, real DDL rollback and retained-row deletion protection run in tests. PostgreSQL has the paired DROP NOT NULL and metadata table; execution needs a legitimate PostgreSQL test DSN.

The old fork/clone seam had parent-session metadata but no precise copy-to-parent-message coordinates. Content-field matching cannot distinguish equal-content originals. Controller explicitly authorized the second metadata-only part of 045: `message_copy_sources(message_id PRIMARY KEY REFERENCES messages(id) ON DELETE CASCADE, parent_session_id, parent_message_id)`. Existing atomic CommitSessionFork writes/checks direct parent coordinates. Parent coordinates deliberately have no FK so ordinary unprotected parent deletion behavior is unchanged. New nested copies follow exact direct links; historical copies without links return explicit unavailable, with no inference or backfill.

No operation columns were added. An authorized retained operation uses a versioned descriptor in its existing Result field, atomically completed with the existing Journal receipt. This is sufficient for existing consumers once projected/resolved through the adapter. Ordinary unshared result text is never parsed as a descriptor.

## Final APIs and S3 contract

| API/type | Contract |
|---|---|
| `storage.SubjectSourceStore.ReadSubjectRunEvidence(ctx, binding, runID)` | Validate exact canonical terminal evidence from existing owners; return original message rows and prospective interaction, no new transcript storage. |
| `Service.RepairSubjectHistory(ctx, sessionID)` | Under existing projection lock, current-authorized repair of terminal retained runs and idempotent seal retry. Missing originals fail explicitly and remain protected/unsealed. |
| `Service.SubjectRecent(ctx, reader, after, through, limit)` | Recheck resident whole-domain read grant, read C2 sequence order, independently recheck every returned source. Metadata only; through=0 remains an empty captured prefix. |
| `Service.ReadSubjectOriginal(ctx, reader, interactionID)` | Recheck reader/source grants; exact immutable originals, source digest, actual terminal outcome, plus separate current disposition and existing supersession markers. |
| `runtime.SubjectOriginalView` | InteractionID, RunID, SessionID, SourceDigest, Outcome, Messages, Disposition map (`included`/`superseded`), Supersession []SessionTruncation. Disposition is derived, never included in the original digest or a new authority. |
| `Service.ArchiveSubjectSource(ctx, sessionID)` | Current-authorized internal ArchiveRetainedSession, keeps pinned singleton and protected original bytes addressable. No new user lifecycle flow. |
| `storage.MessageCopySourceStore.GetMessageCopySource(ctx, messageID)` | Read exact direct parent coordinates written atomically with new copies. |
| `domain.Message.CopySource` / `MessageCopySource` | Optional copy input metadata for existing fork transaction; original canonical evidence does not acquire rewritten identity. |
| `domain.HistoryItem.OriginStatus` | Optional ok/unavailable/forbidden provenance result, included in existing history snapshot digest. Parent read scope is rechecked across every exact link. |
| `domain.Message.ToolError` | Exact expanded authorized tool failure; omitempty preserves old empty text-source digest behavior. |
| `ServiceDeps.ToolEvidence storage.BlobStore` | Optional existing seam for fault injection/composition; normally existing Journal/Sessions owner's Blobs store. |
| `storage.ToolEvidenceSchema` | `vivy.tool-evidence/v1`; descriptor binds artifact ID/SHA256/bytes, run/operation, raw result/error hashes/lengths and stable model projection. |
| `ToolEvidenceArgumentsID`, `ToolEvidenceResultID`, `NewToolEvidenceReference`, `DecodeToolEvidenceReference`, `VerifyToolEvidence`, `VerifyToolEvidenceArguments`, `ToolArgumentsReference` helpers | Deterministic owner-bound references; every retained exact expansion/replay validates ownership, hash and byte identity. Bodies remain BlobStore-owned. |

C2 identity is `SubjectInteractionID(runID, terminalSeq)` and digest is the existing SubjectEvidenceDigest over canonical originals. New fork copies reuse original IDs through provenance and do not reseal inherited history; only new branch interactions can allocate new C2 entries. Seal allocation uses the existing domain-head transaction/serialization. Complete unsealed sources retain durable sealing order, not wall-clock order. Before any new allocation, an already-terminal eligible incomplete source in the same authorized domain blocks head advancement. Active sources and NoContext/private/unpinned activity do not block. Existing sealed retry/read remains valid. Repair closes the gap without duplicate or missing sequence numbers; no second cursor was added.

Rewind uses existing atomic session.truncated Journal plus view-truncation markers. Canonical original ID/digest/seal are stable before and after rewind. Original readers return explicit supersession separately so downstream S3 can avoid treating rewound facts as current. Archive/list removal does not remove original evidence or protected source navigation. Runtime NoContext-first exclusion is tested; a future HTTP/S3 integration must preserve it at its actual boundary.

## Evidence boundary and actual media behavior

The existing tool adapter now captures the authorized raw result+error pair before non-nudge error return or soft diagnostic conversion, and before post-invoke interrupt conversion. Result and failure must be valid UTF8; invalid text fails before any successful receipt/seal/continuation rather than claiming JSON's replacement text as exact bytes. Normal nudge/shell behavior remains governed by the existing adapter.

The original model/user request argument bytes are stored because the old operation receipt retained only RequestDigest and the normal Journal projection can shorten arguments; neither can reconstruct the exact request. This artifact is restricted to original public requested arguments. Private middleware/effective arguments and credentials never enter it. Pending shell records its argument reference before tool.requested so cancelled/approval paths retain the actual known request.

Only an authorized pinned source receives retained descriptor semantics. Raw result/error and stable compact model outcome are durably saved before operation completion; operation save failure leaves explicit uncertain/error, no successful continuation or seal. Replay verifies original arguments as well as result artifact; missing/corrupt bytes after completion fail without reexecuting the tool or accepting the projection as original. Descriptor-looking ordinary raw strings remain unchanged.

Enhanced native multimodal conversion rebuilds image content from verified original artifact bytes and preserves image base64. A separate exact-reference text part survives normal text normalization/compaction. Existing policy/model media support restrictions remain in force; retention neither promises unsupported media execution nor bypasses policy. Authorized ordinary Coding and LIFE both keep selected tools and their existing request/context strategies; default ordinary and workers do not gain implicit contribution or history authority.

Work remains the only Work mutation/dedup authority. When submit_plan yields a known candidate and then the existing post-invoke interrupt suspends, the original candidate receipt can complete durably before returning that original typed interrupt. Restart/resume approval or revise retains the exact candidate and real human decision, makes no duplicate mutation, strands no claimed operation, and preserves visible replay semantics. No shared ToolOperation state was widened or native approval resume bypassed.

Known intermediate/final assistant, corrections, attachments and file snapshots use the existing Journal/projector IDs and completion checks. Failed/cancelled terminal known assistant deltas use explicit journal_partial projection; a completed receipt missing tool.finished repairs a labeled partial tool row. Unknown claimed operations remain incomplete and cannot seal. Eligibility lookup precedes Journal Replay to avoid holding its sole database connection across a second storage query.

## Eino capability check

Pinned Eino is unchanged at v0.9.13. Inspected and reused actual `components/tool.InvokableTool.InvokableRun`, EnhancedInvokableTool/native compose.ToolsNode streaming conversion, `compose.IsInterruptRerunError`, `tool.StatefulInterrupt`, and existing ADK runner/checkpoint handling. Eino imports stay within runtime/provider. No new agent loop, checkpoint engine, tool scheduler or substitute native converter was built.

The upstream interfaces do not own Vivy's current subject grants, existing transactional Journal/operation receipt or its BlobStore, and cannot provide original model-request byte reconstruction from the old digest-only receipt. Thin adapters at existing ownership seams are required. If upstream later exposes a compatible pre-conversion outcome callback, the per-invocation capture can move to that callback while keeping Vivy's authorization/receipt owner; it cannot replace those owners.

Inspected source SHA256:
- compose/tool_node.go: `44afbd5217f2e566ff4f312f4408657c1244a080f16b12e3d7c4a34309ac2e3d`.
- components/tool/interface.go: `d10e710bb80923427b68fb58b90737299e7aa98b75819f4d1c46df1bde1b712e`.

The required race gate exposed captured outer err writes by two enhanced stream converter goroutines at compose/tool_node.go:1253. Existing unretained Plan baseline reproduces it. Controller separately verified official first stable native fix v0.9.20 (#1303), and owns the next scoped pin/capability remediation. This increment does not modify dependency pins, vendor/fork upstream, suppress race detection, serialize converters, remove tools, or replace the required gate with the diagnostic subset.

## TDD, failure evidence and final checks

Meaningful initial RED command: `go test ./internal/runtime ./internal/storage/... -run '^TestIssue58' -count=1`, exit1. `/tmp/issue58-task2-red.log` records the original result truncated before receipt and both shell cases rejected at the temporary boundary. Minimum fixes then returned exit0 in `/tmp/issue58-task2-green1.log`; later focused cases extended coverage before final gates below.

Further focused behavioral REDs were observed for discarded partial result+error, invalidUTF8 substitution, missing known completed outcome without tool.finished, enhanced media/reference loss, a post-invoke Plan candidate left claimed, NoContext-only first activity disabling the trusted singleton, missing argument artifact replay, a later seal crossing an incomplete predecessor, accepted NoContext source admission and absent exact copy producer. Matching named final tests cover all these boundaries. Exact-copy RED ran `go test ./internal/runtime -run '^TestIssue58ExactCopyProvenanceAcrossRestartAndArchive$' -count=1` with the new schema/read seam present but atomic producer intentionally absent; `/tmp/issue58-task2-copy-red2.log` reports missing exact parent identity; restoring the actual transaction writer made it green (`copy-green2.log`). No content inference was accepted as a substitute.

Encountered setup/expectation failures are not counted as meaningful TDD: nil test event bus in early restart fixture (red2/green2), stale manifest expected44 after new045, equal-time cutoff/count assumptions, and test API typos. The new authorized-tools fixture was corrected to await completed sealing rather than race terminal status. A pending-shell test found the one-connection Replay deadlock; corrected lookup ordering resolves it. An ordinary concurrent-cancel failure exposed newly added seal I/O between terminal status and active cleanup; sealing moved after existing cleanup (under projection lock) and unshared no-resolver runs skip it. Final cancellation/race diagnosis passes. Full1 had those setup/cleanup failures and was not reported as a pass.

All final non-race gates and diagnostic race below ran on final source after exact copy/prefix guard/cleanup changes. Required/baseline race failure outputs were captured before those last unrelated changes; controller explicitly directed no repeated probing of the known upstream race and requested final non-race gates instead. Counts below are terminal Go JSON test events including groups/subtests, not distinct top-level tests. Go is go1.26.4 linux/amd64 with `/workspace/scratch/243d1cbb859f/toolchains/bin` on PATH and normal checksums. Laputa remains published pin `4c7c6b20859375d0d764d3ba2a0188926cf73c9e`.

### focused

```sh
go test ./internal/runtime ./internal/storage/... -run '^TestIssue58' -count=1 -json
```

Actual exit 0. agent-vivy/internal/storage: 1 pass; agent-vivy/internal/storage/postgres: 8 skip; agent-vivy/internal/storage/migrations: 2 pass; agent-vivy/internal/storage/sqlite: 24 pass; agent-vivy/internal/runtime: 39 pass.
Raw output SHA256: `9dae91d46933032cfcd5aa3432fea70c3a7a82c5a616946c8b35aead64022f38`.

### full

```sh
go test ./internal/runtime ./internal/storage/... -count=1 -json
```

Actual exit 0. agent-vivy/internal/storage/migrations: 40 pass, 5 skip; agent-vivy/internal/storage: 21 pass; agent-vivy/internal/storage/postgres: 36 skip, 1 pass; agent-vivy/internal/storage/sqlite: 209 pass; agent-vivy/internal/runtime: 1130 pass, 1 skip.
Raw output SHA256: `6434cb75037cd9025361ef484440d6087a685fad514dba80377e0677e656b05d`.

### app

```sh
go test ./internal/app -run '^TestIssue58' -count=1 -json
```

Actual exit 0. agent-vivy/internal/app: 14 pass.
Raw output SHA256: `0294c262bc667eb188e71db243ea5ef11f67db2e4cc24def00e92317aaa56b0b`.

### required_race

```sh
go test -race ./internal/runtime ./internal/storage/sqlite -run '^(TestIssue58|TestMessageProjector|TestToolOperationCrashMatrix|TestApprovalResumeReusesPreparedToolOperation|TestServiceCancelConcurrentIdempotent)' -count=1 -json
```

Actual exit 1. agent-vivy/internal/storage/sqlite: 21 pass; agent-vivy/internal/runtime: 47 pass, 2 fail.
Failures: `TestIssue58RetainedPlanReviewRestartAndDecisionReplay/revise`, `TestIssue58RetainedPlanReviewRestartAndDecisionReplay`.
Raw output SHA256: `200ccb3036f50c6687b323375a28693583f58c97a6810a6824932ddf36d23ed6`.

### baseline_race

```sh
go test -race ./internal/runtime -run '^TestPlanGoalProbeSubmissionFencesLaterToolInSameBatch$' -count=1 -json
```

Actual exit 1. agent-vivy/internal/runtime: 1 fail.
Failures: `TestPlanGoalProbeSubmissionFencesLaterToolInSameBatch`.
Raw output SHA256: `4d586171447282f7af70487d265a832fefad87e1de5c8837b8e8e391fe576dff`.

### diagnostic_race

```sh
go test -race ./internal/runtime ./internal/storage/sqlite -run '^(TestIssue58(Original|Shell|Evidence|Archive|Fork|Rewind|Terminal|Completed|Default|Invalid|Authorized|Failed|PostInvoke|RetainedMultimodal|NoContext|Pending|Interaction|Subject|Admission|Continuity|Formatted|Sealed|Exact|Snapshot|Contention|Missing|Historical|Copy)|TestMessageProjector|TestToolOperationCrashMatrix|TestApprovalResumeReusesPreparedToolOperation|TestServiceCancelConcurrentIdempotent)' -count=1 -json
```

Actual exit 0. agent-vivy/internal/storage/sqlite: 24 pass; agent-vivy/internal/runtime: 52 pass.
Raw output SHA256: `b9c39053148c2f8f99b9f3dad8fc508988edd5d55fd02df5890727d73f504543`.

`git diff --check` passes. PostgreSQL tests are registered and compile, but no real VIVY_POSTGRES_TEST_DSN is available: 36 server checks plus 5 PostgreSQL migration checks skip in the full gate; the pure PostgreSQL unit check passes. Runtime TestReportFeedbackSnapshot separately skips its existing external GitHub prerequisite. Every skip name is in task-2-verification.json. This is not PostgreSQL parity acceptance.

SQLite conformance includes actual retained BlobStore expansion/archive/reopen, same-domain head contention and prefix repair, authorizations and changed bindings, populated044 migration, copy metadata atomic rollback, exact duplicate provenance, nested fork/restart/archive and unchanged ordinary deletion. Fault injection verifies artifact save and operation receipt save failure deny continuation/sealing; replay corruption/missing result or original-arguments denies repeat invocation. Tests use temporary isolated stores, never tenant journals.

## Actual source manifests

These are actual fixture logs from the final focused run, not invented expected values. Random runtime-created IDs/time-dependent source hashes can differ between runs; the asserted equality is before/after within the same execution. The conformance retention-run fixture is deterministic. Full changed-file SHA256 and exact command/output manifests are committed in `docs/logs/2026-10-15-issue58-subject-binding/task-2-verification.json`.

- `plan_goal_probe_test.go:398: manifest retained Plan run_7d905312c2b538cc:19 51cd75449567fe9482c8ce92c15e3f76cca4bb002cb1989487440219d2f38dde candidate ac42d5b4f20ae5b58af788acd10b7d321370ed9d0b876ac540f53769ce7d181e`
- `plan_goal_probe_test.go:398: manifest retained Plan run_5809fd0aefc24388:19 8285f95a444d851acdbea9e4f142eb5982f0bd68c8bbbbb0a7fea11add1548ea candidate 744b3f350a0e8e1491ad041006b41d73c49259e52c76de2b182547a2823c06bb`
- `subject_history_test.go:28: manifest before/after archive retention-run:7 fe84c42bafadf6b87c0c20efb11c72bd230014462120c66e143169911154fd95 artifact subject-tool/2b602ac7cb2ba6e9b603e7c92a948fbd5494f12031f2360edc7075e9b4b8259b/result/1bc8bdb4c85de701af2d0e6eed1245b4924b8d52edbc44dc83331721c728dbc6 1bc8bdb4c85de701af2d0e6eed1245b4924b8d52edbc44dc83331721c728dbc6 190134`
- `subject_evidence_test.go:411: manifest resident=false run_ae220d744e23068a:16 e5b49cb368e9c86b3837f66c3e05d01c843b33c0e5bfc932a2dcac059eef11e2`
- `subject_evidence_test.go:411: manifest resident=true run_be2790d299865ec9:16 1c46f0e9d223b4a14537768a4d3fb59e4e28ac3cda5aee001821e3a85f6d9043`
- `subject_history_test.go:90: manifest before restart restart-run:11 2a5be1ad9a837ad2740c0b8fb3d2944e2f6ecfa32812fd42f9b9603588bede13`
- `subject_history_test.go:107: manifest after restart restart-run:11 2a5be1ad9a837ad2740c0b8fb3d2944e2f6ecfa32812fd42f9b9603588bede13`
- `subject_history_test.go:207: manifest before/after archive run_e800bdbd74ddd1be:6 7d3fa34549417b6aca99921b514c91af6a56a44647fb0149f8dd0d51c56374ef`
- `subject_history_test.go:254: manifest partial receipt receipt-repair-run:6 af4fb5092d33b6ddc21b708368ce41352440ff87e57d9567f3e76af1309e0c28`
- `subject_history_test.go:349: manifest before/after fork and rewind run_93db56e0a8bc7ee9:6 5f59890faf5a7957d1a3bb6eab19583a765b6f4239493dbdd23b4bfcb3a042f1`
- `subject_history_test.go:544: manifest exact copy before/after nested fork/restart/archive identical-run:3 3673a699d36e3425f3d728925b0f4268ed33b421c07b9cabbaf4242a8691483c`

## Exact implementation file inventory

The following 40 source/test/migration files are the exact implementation inventory. Their final SHA256 values are in the committed manifest; no generated assembly, dependency or unrelated product files changed.

- `internal/app/subject_admission_test.go`
- `internal/domain/history.go`
- `internal/domain/session.go`
- `internal/runtime/enhanced_tooladapter.go`
- `internal/runtime/history_service.go`
- `internal/runtime/mapper.go`
- `internal/runtime/message_projector.go`
- `internal/runtime/payloads.go`
- `internal/runtime/plan_goal_probe_test.go`
- `internal/runtime/rewind_service.go`
- `internal/runtime/service.go`
- `internal/runtime/shell.go`
- `internal/runtime/subject_binding.go`
- `internal/runtime/subject_evidence_test.go`
- `internal/runtime/subject_history.go`
- `internal/runtime/subject_history_test.go`
- `internal/runtime/subject_tool_evidence.go`
- `internal/runtime/tool_operation.go`
- `internal/runtime/tooladapter.go`
- `internal/storage/admission_validation.go`
- `internal/storage/conformance/subject_history.go`
- `internal/storage/conformance/subject_retention.go`
- `internal/storage/history.go`
- `internal/storage/migrations/manifest_test.go`
- `internal/storage/migrations/postgres/045_subject_shell_evidence.sql`
- `internal/storage/migrations/runner_test.go`
- `internal/storage/migrations/sqlite/045_subject_shell_evidence.sql`
- `internal/storage/migrations/subject_history_test.go`
- `internal/storage/postgres/history_mutations.go`
- `internal/storage/postgres/run_admission.go`
- `internal/storage/postgres/subject_history.go`
- `internal/storage/postgres/subject_history_test.go`
- `internal/storage/sqlite/history_mutations.go`
- `internal/storage/sqlite/run_admission.go`
- `internal/storage/sqlite/subject_history.go`
- `internal/storage/sqlite/subject_history_test.go`
- `internal/storage/subject_history.go`
- `internal/storage/subjectsql/evidence.go`
- `internal/storage/subjectsql/subjectsql.go`
- `internal/storage/tool_evidence.go`

Durable related evidence: docs/TODO.md open upstream race row; appended summary.md, verification.md and acceptance.md in docs/logs/2026-10-15-issue58-subject-binding; new task-2-verification.json plus complete task-2-required-race.jsonl and task-2-upstream-baseline-race.jsonl. This report is the only force-added ignored task file. Existing handoff/brief and parent plan/ledger are not committed.

## Self-review, limitations and rollback

Implementation review checked that no released044 migration changed; retained bodies are in existing BlobStore/message/Journal/Work owners; source protection survives archive and hard-delete denial; exact copy coordinates cannot be fabricated by equal content; read grants are rechecked rather than inferred from stored membership; private effective arguments are absent; old ordinary tool results stay raw; partial results and typed Plan interrupts retain their native semantics; and no new domain head allocation crosses an already-terminal evidence gap. Existing sealed originals remain stable across repair, fork, rewind and archive.

Material outstanding concern: required Eino race gate fails and must turn green after the separately scoped official native-fix pin before S3 readiness. Diagnostic race pass is only additional localization. Independent code/security/data-integrity review remains the controller's next step. PostgreSQL server parity and centralized S6 CI remain unverified; no unsupported pass is claimed. Historical copies lacking exact coordinates deliberately report unavailable, and corrupt/missing originals require authorized repair rather than reexecution or invented reconstruction.

No release package was made in this task, so release.md is omitted. Rollback stops new trusted source admission and sealing, retains045 schema, original artifacts and protected source rows, and never physically deletes evidence or downgrades released schema. S3 compaction/recap, producer #66, UI and final LIFE HTTP smoke are not implemented by this increment.


# S2 Task2 fix round1 — accepted independent findings I1/I2

This section supersedes the original report's outstanding native Eino race result for this final source, while preserving that historical evidence. Source corrections I1 and I2 are implemented and all prescribed fix-round1 checks pass. Independent scoped re-review, real PostgreSQL and centralized S6/product acceptance remain pending.

Exact fix BASE: `15e79419ccfa302ad3f8330fe72448c88796a5fb`. Implementation/evidence commit: `333a0bf0cd5def56d9f6f90e31d2d220058ab05b`. Report-only follow-up commit is supplied in the handoff. Human identity remains mastwet <1136523089@qq.com>. This round changes only two production files and one regression file, plus the existing iteration records and exact evidence below. Parent-owned docs/superpowers/plans/issue-58/index.md remains dirty and excluded. No subagents, push, merge, deployment or tenant data access.

## Finding I1: native PostgreSQL transaction syntax

Verified CommitSessionFork opens `b.db.SQL.BeginTx`, a raw *sql.Tx, so the two newly added copy metadata statements bypass the wrapper that translates question-mark placeholders. Corrected parent-message verification to `$1/$2` and mapping insertion to `$1/$2/$3` in internal/storage/postgres/history_mutations.go. Existing transaction, rollback, exact parent validation, copy metadata/FK behavior and shared backend conformance remain unchanged. No string-mirroring test, SQL wrapper, driver or dependency was introduced.

The actual PostgreSQL server regression cannot run here because VIVY_POSTGRES_TEST_DSN is absent. Existing TestIssue58CopySources remains registered for PostgreSQL and runs against SQLite; its exact copy identity and transaction rollback assertions are retained. PostgreSQL package compilation and its one pure unit pass do not establish real backend parity. Legitimate PostgreSQL conformance remains required at S6.

## Finding I2: native Question answer retention

Verified the native ToolInteractionQuestion success branch returned GetResumeContext[string]'s answer before operation admission, while the original evidence validator correctly requires a completed receipt for successful tool.finished. The new real restart regression reproduced the exact missing receipt and unsealed terminal source before any source fix. The validator was not weakened.

The fix stays at `toolAdapter.dispatchUngated`'s existing native Question resume boundary. After real targeted resume data exists and current authorization confirms retained evidence, it uses existing coordinator.Admit with the original public requested argument bytes and existing effective/middleware bindings, then existing coordinator.Execute to retain the actual already-committed Question answer. The per-invocation originalToolOutcome capture explicitly identifies the raw answer so even a legitimate answer beginning with the untrusted-output header retains every byte. No tool implementation is invoked. Question store/lifecycle and Journal still own the human decision; this receipt is evidence of that native outcome, not a second decision authority.

Retention completes before enhanced result normalization or model continuation. Existing BlobStore ownership, descriptor validation, uncertain/save-failure behavior and exact reference projection remain authoritative. No new operation state, fake tool invocation/outcome, alternate Question engine, orchestration loop or request authorization is added. Pending native Question still interrupts without an answer receipt; ordinary unshared resume still directly returns the real answer without retention. Policy denial, failed/unknown outcomes, cancellation and Plan review keep their existing paths. There are no new exported APIs or C2 signature changes.

The concrete regression `TestIssue58RetainedQuestionRestartKeepsExactAnswer` uses the real built-in ask_user (whose implementation explicitly errors if invoked directly), real Service.Run, native interruption, a durable checkpoint, closing/reopening SQLite, Service.Recover and AnswerQuestion. It checks that pending Question has no invented answer receipt or seal. The resumed answer is 38,043 bytes, begins with the literal untrusted-output header and exceeds MaxToolResultBytes=128. Exact original request, answer, hashes and length resolve from the receipt/artifact and original reader; the model sees a shortened answer with its exact reference. A second answer is rejected by existing Question authority. Terminal events remain singular; repair/idempotent retry leaves head1; a subsequent real same-domain interaction advances head2. The ordinary-unshared control keeps its actual Question answer and shortened native result but has no retained operation or C2 head. These assertions address the prefix-blocking consequence rather than merely observing successful run status.

## Native Eino capability check and unchanged ownership

This fix runs against the already committed official Eino v0.9.20 from normal module cache, not the old sibling clone. Inspected actual components/tool.GetResumeContext[string] target/data semantics and components/tool.Interrupt; existing native ADK checkpoint/resume and enhanced ToolsNode conversion remain in use. GetResumeContext already supplies the authoritative native resume data; adapting its success boundary to Vivy's existing durable authorized receipt is sufficient. Eino does not own Vivy's subject grants/Question transaction/BlobStore, so no native engine can replace those owners. No dependency pin, current constants, Persona real-snapshot fixture or native converter changed in this round. Laputa stays published pin 4c7c6b20859375d0d764d3ba2a0188926cf73c9e.

Inspected native source SHA256:
- `/root/go/pkg/mod/github.com/cloudwego/eino@v0.9.20/components/tool/interrupt.go`: `5939a961e46f474230dcb4cd06591fcbda450704d938c4cecb2098cb6e2dbba4`.
- `/root/go/pkg/mod/github.com/cloudwego/eino@v0.9.20/compose/tool_node.go`: `9ad538059f4ecc81137dbb045829d95792dfdf3fa3905b322f4152d4b6c131bd`.

## Actual RED → GREEN and final required gates

The first fixture attempt omitted the pre-existing five-minute ApprovalExpiration and the recovered Question was already expired. That setup failure is retained locally at /tmp/issue58-task2-fix1-setup-expiry.jsonl; it is not counted as meaningful RED and led to no production change. After correcting only the fixture expiry, the real behavior RED below reports `answered native question has no completed exact receipt: state= err=storage: not found` and the existing seal logs subject evidence incomplete; the ordinary control passes. The same test then passes after the narrow source fix.

All final gates below ran on the final source at native Eino v0.9.20. Counts include subtest/group terminal JSON entries. PATH includes /workspace/scratch/243d1cbb859f/toolchains/bin; Go1.26.4 linux/amd64; normal checksum behavior. No optional broad repetition, root ./..., just ci, provider/network smoke or dependency workaround was run. Exact commands and actual output hashes/counts/all skip names are committed in task-2-fix-1-verification.json. Complete red/green and original required-race outputs are committed adjacent to it.

### red

```sh
go test ./internal/runtime -run '^TestIssue58RetainedQuestionRestartKeepsExactAnswer$' -count=1 -json
```

Actual exit 1. agent-vivy/internal/runtime: 2 fail, 1 pass.
Failing test events: `TestIssue58RetainedQuestionRestartKeepsExactAnswer/retained`, `TestIssue58RetainedQuestionRestartKeepsExactAnswer`.
Raw output SHA256: `539d9063f245c050adda5b822e92f20006433b094c36a71f2394ebecf2263762`.

### green

```sh
go test ./internal/runtime -run '^TestIssue58RetainedQuestionRestartKeepsExactAnswer$' -count=1 -json
```

Actual exit 0. agent-vivy/internal/runtime: 3 pass.
Raw output SHA256: `8b4be5a86fa7d3e0019020d712df388afac3506fee7c83e73d6649dcd18188cd`.

### focused

```sh
go test ./internal/runtime ./internal/storage/... ./internal/app ./internal/rpc -run '^TestIssue58' -count=1 -json
```

Actual exit 0. agent-vivy/internal/storage: 1 pass; agent-vivy/internal/storage/postgres: 8 skip; agent-vivy/internal/storage/migrations: 2 pass; agent-vivy/internal/storage/sqlite: 24 pass; agent-vivy/internal/runtime: 42 pass; agent-vivy/internal/rpc: 10 pass; agent-vivy/internal/app: 14 pass.
Raw output SHA256: `53f6a0e29c177415311d93e55ef3bc388fcac1e049b839abc5a5f075e011e20e`.

### required-race

```sh
go test -race ./internal/runtime ./internal/storage/sqlite -run '^(TestIssue58|TestMessageProjector|TestToolOperationCrashMatrix|TestApprovalResumeReusesPreparedToolOperation|TestServiceCancelConcurrentIdempotent)' -count=1 -json
```

Actual exit 0. agent-vivy/internal/storage/sqlite: 24 pass; agent-vivy/internal/runtime: 58 pass.
Raw output SHA256: `7733751f15794561b70e13941a957a170a89641aceea0547a8a30df882a30ebe`.

### full

```sh
go test ./internal/runtime ./internal/storage/... -count=1 -json
```

Actual exit 0. agent-vivy/internal/storage: 21 pass; agent-vivy/internal/storage/migrations: 40 pass, 5 skip; agent-vivy/internal/storage/postgres: 36 skip, 1 pass; agent-vivy/internal/storage/sqlite: 209 pass; agent-vivy/internal/runtime: 1133 pass, 1 skip.
Raw output SHA256: `3e654b64c551029dda22d79a207290c714c7739bd117b93c5cb1061af2667313`.

### native-regressions

```sh
go test -race ./internal/runtime -run '^(TestServiceQuestion|TestGoalRecovery.*Question|TestPendingGoalQuestion|TestCancelResumedGoalQuestion|TestServiceRecoverRestoresSkillMountedTools|TestPlanGoalProbeSubmissionFencesLaterToolInSameBatch|TestMapperInterruptDetails|TestRealParkedCancellation|TestParkedCancellation|TestServiceCancelPendingRun)' -count=1 -json
```

Actual exit 0. agent-vivy/internal/runtime: 19 pass.
Raw output SHA256: `39e8500d5eda7848ec68225182daf39c474d36a07489daddcc118600c0b432b4`.

The original required-race expression is unchanged and includes every Issue58 test (new Question included), retained Plan, projector, operation crash matrix, prepared approval replay and concurrent cancellation. It passes82 entries with no skip/failure. The targeted native regression race command passes19 entries and includes the formerly failing unretained Plan baseline. The final full runtime/storage gate passes1404 entries and skips42:36 real PostgreSQL server tests,5 PostgreSQL migration checks,1 pre-existing external GitHub TestReportFeedbackSnapshot. Focused runtime/storage/App/RPC passes93 entries with8 PostgreSQL skips. No skip is counted as backend parity acceptance. `git diff --check` passes.

## Exact source and output manifests

- `internal/runtime/tooladapter.go`: `ff843369763de0df4cc9badc9cbab542c8444f7dcf5553f7f2ac870047269a8e`.
- `internal/runtime/subject_question_test.go`: `e1dbdcd991d258f9b2ba43650d30cfdbfdd5effc0869a9c4e410d033df28696e`.
- `internal/storage/postgres/history_mutations.go`: `5923b13c601e3204e2b5ae4aebbaa87f309429ff6b0376e556c6f6eb169108f1`.

Actual GREEN fixture identity/digest/answer manifest (random run ID is execution-specific; exact answer hash is stable):

- `subject_question_test.go:150: manifest resumed question run_7cb79a90ce7ea547:18 bbf95078951186b04bad400543e6d0964f2cb581caa0526cb54798c6bf3868f0 answer 0047201c5c5c653f027ff4641940a4fc750ea8f2014e056952a27cfb93e5a1ac bytes 38043`

New durable evidence files: task-2-fix-1-verification.json, task-2-fix-1-red.jsonl, task-2-fix-1-green.jsonl and task-2-fix-1-required-race.jsonl in docs/logs/2026-10-15-issue58-subject-binding. Existing summary/verification/acceptance files have concise fix-round appendices. Full report is appended both here and the requested ignored scratch task-2-report.md; scratch is not force-added or committed. The report-only follow-up commit changes this durable report only.

## Self-review and remaining acceptance

Self-review confirms the code delta is limited to actual raw PostgreSQL parameter syntax and authorized native Question success retention. Ordinary unshared flow is behaviorally tested, pending receipt absence is explicit, the real Question authority rejects duplicate answers, the native tool implementation is not invoked, and the original validator/operation state machine/artifact integrity and grant rechecks remain intact. Existing native Question, interrupted/recovered/approved/parked cancellation and Plan semantics pass their targeted tests; the original full required race now passes at the separate official dependency fix.

Still pending: independent scoped re-review of I1/I2; legitimate real PostgreSQL parity; native Windows; real provider/product and final HTTP/UI acceptance; #66 producer integration; centralized final S6 just ci/artifact gate. Domain pending-source discovery/repair and covered-source supersession integration remain S3/S5 architecture dependencies and were not expanded into this fix. No readiness claim closes those gates. Existing rollback retains schema/protected originals and stops new admission/sealing; no downgrade/deletion is added.
