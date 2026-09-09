# RB-1 Rollback research: how rollback works and whether Vivy promises "code rollback"

- Date: 2026-08-31
- Initiation: `docs/TODO.md` §0.1 RB-1 (user decision on 2026-08-31, addition 3 to the VC-0 decision list)
- Research scope: checkpoint bridge (FR-8 session recovery) / file-version history (archiving before VC-3 edits) / git-semantic rollback—the boundaries and combination of the three; output = whether Vivy promises a "code rollback" capability and where it belongs (mainline/plugin/code face)
- Reference sources: `internal/` in this repository (rb1-rollback worktree snapshot); `.workspace/crush/` (Charm Crush, FSL-1.1-MIT, behavioral reference only; code must not be copied in)

---

## 1. One-sentence conclusion

**Vivy currently supports no code rollback, but that is "correct non-support" for the current product form—the file tools write only to per-run isolated scratch space and cannot touch the user's real code at all. When the VC code face connects the tool surface to a real workspace, file-level rollback becomes a necessary safety net; at that point, deliver it in the mainline kernel as an "archive-before-edit version chain + single-file/session-level restore" (not git-semantic rollback). For now, decide only the design direction; deliver the implementation along with the VC-1/VC-3 storage design rather than creating a separate early project.**

---

## 2. Semantic boundaries of the three kinds of "rollback"

| Semantics | What it is | What it solves | What it cannot solve | Vivy status |
|---|---|---|---|---|
| **Checkpoint bridge (FR-8)** | eino ADK runtime snapshot (opaque bytes + engine-version envelope + checksum), restoring a run interrupted by approval/question or interrupted by restart | Session/run-state recovery: pending approvals can be decided again and resume can continue | Contains no file semantics—the recovered run continues with the files currently on disk; changed files are not restored | Implemented (`internal/runtime/checkpoint.go` + restart recovery) |
| **File-version history** | Archive the old content into a version chain before every write/edit; rollback = restore the file to a version on the chain | The correct form of code rollback: single-file "undo this change" and session-level "undo every file changed by this session" | Ineffective for execute/bash side effects; cannot express repository-level semantics such as "restore to a git commit" | **No implementation** (no table, archive, or recovery RPC) |
| **Git-semantic rollback** | Repository-level operations such as commit / branch / reset / stage | Precise, mature repository-level history | Requires the workspace to be a git repository; uncommitted intermediate states have no useful granularity; treating "make the agent run git" as rollback builds the safety net on a tool | None (and the mainline deliberately does not install a shell/composite-command surface) |

**Decision: Vivy should promise "file-level rollback" (version chain), not git-semantic rollback**—git belongs to the user's git, and the agent is responsible only for files it changed. The checkpoint bridge and file-level rollback are complementary (one restores runtime state, the other file state); see §5 for their integration points.

---

## 3. Vivy current-state inventory (verified item by item)

### 3.1 Checkpoint bridge = pure runtime state, no file semantics

- `internal/runtime/checkpoint.go`: `VersionedCheckpointStore` wraps the eino runner's opaque checkpoint bytes in a `{engine_version, checksum_sha256, created_at}` envelope (4-byte length header + JSON header + payload), stores it in `storage.BlobStore`, and fail-closed validates the engine version + checksum on read. **The envelope contains no file path/content and cannot contain them.**
- `internal/runtime/checkpointadapter.go`: only adapts it to `adk.CheckPointStore/Deleter`.
- Purpose (`internal/runtime/service.go`): suspend a run on an approval/question interrupt; restart recovery rebuilds a "non-terminal run whose verifiable checkpoint can still be read" as pending, waiting for the next decision to resume. engine.go:224 `Resume` = `runner.ResumeWithParams`.
- **Conclusion: FR-8 "recovery" is runtime-state recovery and unrelated to code rollback. Rollback research does not require changing it.**

### 3.2 File-write path = per-run isolated scratch, with nowhere to archive old content

- The tool surface (write_file / patch) uniformly uses `EinoFilesystemBackend` (`internal/runtime/filesystem_backend.go`); path resolution **unconditionally** goes through `WorkspaceManager.Ensure(runID)` → private scratch at `<Runtime.WorkspaceRoot>/<runID>/` (`resolve()`, filesystem_backend.go:627); `safeWorkspacePath` then blocks escapes/symlinks/protected names. **Even under `danger_full_access`, the file tools cannot leave this scratch** (danger mode only skips sandbox validation; backend containment remains).
- Default configuration is `workspace_root: data/workspaces` (config.example.yaml:60)—scratch is in the data directory, not the user's project.
- `WriteFile` (:296): read old content → compare with `ProposalPreconditionHash` (reject if the target changed externally after approval, "proposal stale") → atomically replace with `atomicWrite` → produce `boundedDiff`. **Old content leaves only two non-recoverable traces**:
  1. `boundedDiff` (:768): a single `@@` hunk with the full `-old`/`+new` content embedded and truncated above 32KB—it is persisted in the tool-result payload with the Journal tool event, but the format is lossy and cannot be written back directly;
  2. Approval proposal (`PrepareWriteFile`/`PreparePatchFile`): the `reviews` table stores `precondition_hash` + `preview`—the hash can only tell whether it changed, and the preview is a lossy diff.
- `PatchFile` (:346): delegates to `WriteFile` after an exact unique replacement and likewise has no archive.
- download (`internal/runtime/download.go`) also writes files directly into scratch.
- **Scratch lifecycle:** there is no cleanup/reclamation code (no RemoveAll for workspaces under runtime/app), and scratch is deterministically rebuilt/reused by run ID. Thus "run-level rollback" is equivalent to "discard/rebuild this run's scratch"—a trivial operation in the current form that needs no version chain.

### 3.3 Only `execute` (danger mode) can touch the real filesystem, and it has no rollback by nature

- `internal/runtime/command_backend.go:151`: allowed only under `danger_full_access` and when it matches the allowlist; composite commands (pipes/`&&`/git) cannot run at all today, so there is no rollback problem for "bash damaging the workspace." **VC-1 bashification will open this path—that is the point at which the conclusion of this research actually attaches (see §5.1).**

### 3.4 Storage layer = has version primitives, no file-version table

- The sqlite/postgres 26-table family (approvals/blob/compaction/crons/journal/lease/messages/notes/questions/reviews/runs/sessions/skill_revisions/snapshot/studio/todos/token_usage…): **no file/version table**.
- Existing version primitive: `storage.BlobStore` = append a generation + flip a pointer (D-030, sqlite `blob.go`: `checkpoint_generations(id, generation, blob)` + `checkpoints` pointer row). **The API exposes only latest reads** and has no query to list historical generations—reusing it for file versions would require extending the interface (List generations / read by generation).

### 3.5 Audit (Journal) records "what happened," not "what was restored"

The ~40 RunEvent event-sourcing types already record each write's diff trace and precondition_hash in the event stream; the increment for rollback capability is not "more logs," but **machine-readable, writable-back old-content archives + approval for the restore action itself**.

---

## 4. Crush reference (behavioral-alignment target; do not copy code)

- **Version chain** (`internal/history/file.go`): per-session file-version table (session_id, path, content, incrementing version, full text rather than deltas). Each write tool (edit/write/multiedit/lsp_replace_symbol/lsp_rename) attaches to the chain through `commitFileChange` (edit.go:246): the first time a file is seen, `Create(path, oldContent)` stores the old content; if the user edited it manually (chain latest ≠ disk content), insert an intermediate version first; then `CreateVersion(newContent)`. Recording failures are logged only and do not block the write (best effort).
- **Stale-read protection** (`internal/filetracker`): record (session, path, last_read_at); before editing, reject immediately if the disk mtime is later than the last read ("read before edit"). It aligns with the version chain in being per-session semantics.
- **Consumer = display only** (a key finding of this research): the TUI's `loadSessionFiles` (ui/model/session.go:112) aggregates the version chain into a "which files this session changed" panel (first→latest diff statistics + real-time pubsub updates). **A full-repository search for restore/revert/rollback found that Crush has no restore/undo consumer.** The version chain is infrastructure that "plants data first and waits for consumers"—comparison conclusion: Crush itself has not implemented code rollback either; if Vivy does so, it will lead directly.
- Migration implication: Crush stores full TEXT with no size limit or retention policy; Vivy should design in a 1MB-scale per-version limit + retain N versions per (session, path) + a session-cleanup hook (aligned with Vivy's existing 1MB maxFileBytes tool-surface budget).

---

## 5. Recommendation: promise scope, ownership, and attachment point

### 5.1 Whether to promise "code rollback" → promise file-level rollback; decide the direction now and deliver it along with VC

1. **Current mainline product (today):** file tools write only to scratch, so user code is unaffected; rollback is not a current gap. Do not start it early as a separate project.
2. **VC-1 (bashification + tool-surface alignment):** once bash/composite commands open a real execution surface, "damaging a file" changes from impossible to possible. At that point **file-level rollback (L1) becomes a safety-net prerequisite**: archive files affected by write/patch/bash before changing them.
3. **VC-3 (deeper LSP/editing + the stale-read filetracker merged from VC-1):** the version chain and filetracker already require one combined storage design (recorded in research §8.4); recovery RPC and UI are the read side of the same table and can be delivered along the way (L2).

### 5.2 Ownership: mainline kernel, not a plugin or code-face exclusive

- The version chain spans the storage layer + file-tool execution path + approval flow (the restore action itself must pass HITL)—all three are mainline core, so **pluginization has no meaning**.
- The code face is only the first heavy consumer; the mainline web face's write/patch uses the same table and chain (the chain's value is low in scratch mode, so full archiving can be enabled in the code face's real-workspace mode while the mainline remains lightweight or uses a per-file switch—see O4).
- Governance alignment: add `file_version.archived` / `file.restored`-type events to Journal (persist before push through the existing RunEvent pipeline); a restore action is a proposal with write_file semantics and follows the approval policy (even in auto mode, precondition hash is required to prevent overwriting concurrent writes).

### 5.3 Layered design (minimum promise to write into VC-1/VC-3 acceptance)

| Layer | Contents | Status |
|---|---|---|
| **L1 archive + single-file restore** | Before write/patch, write old content (sha256 deduplicated) to the version chain; read-only `files/versions` RPC; `files/restore(version)` goes through an approval proposal (precondition hash prevents concurrent overwrite) | Lands in VC-1 together with the filetracker storage design |
| **L2 session-level rollback** | Aggregate the version chain by session and restore all changed files in reverse order (list stale conflicts separately as rejected and continue); one-click "undo this session's changes" in the UI | VC-3 (alongside deeper LSP editing) |
| **L3 git semantics** | Do not implement. When the code face detects a git repository, tell the user to use git; the agent does not promise reset/commit-level rollback | Explicitly not implemented (product position) |

### 5.4 Storage-shape tradeoff (design decision; finalize before implementation)

- **Option A (recommended): new `file_versions` table** (sqlite + postgres dual backend): `(id, session_id, run_id, path, version, content_hash, content BLOB, created_at)` + unique `(session_id, path, version)`. Rationale: the BlobStore API has only latest semantics, and checkpoint envelope bytes are eino-specific; file versions need aggregation queries by path/session and a retention policy, which are more straightforward in a separate table. The archive can live in the BLOB column (Vivy already has a SQLite/PG dual-implementation precedent).
- Option B: extend BlobStore with ListGenerations/GetGeneration. Saves a table, but puts file semantics into the checkpoint namespace, makes queries awkward, and requires extensions in both PG/SQLite.
- Combined storage with filetracker (established in research §8.4): design `file_reads(session_id, path, read_at)` in the same batch, record it in the read tool, and validate before writes—complete both CRUDs in one migration.

---

## 6. Decision points (attach as Open when writing back to TODO; user decides)

| # | Decision | Recommendation |
|---|---|---|
| O1 | Promise scope: L1 only / L1+L2 | L1 with VC-1, L2 with VC-3 (two steps; do not do it all at once) |
| O2 | Retention policy: retain N versions per (session,path) + per-version limit | N=20, 1MB per version (aligned with tool-surface maxFileBytes); cascade cleanup when a session is deleted |
| O3 | Storage: new table (A) vs. BlobStore extension (B) | A |
| O4 | Archive scope: full for the code face's real workspace, lightweight for mainline scratch (hash + diff trace only) vs. one chain for both surfaces | One chain for both surfaces, one RPC set (simple implementation, consistent auditing; the cost of chaining scratch is only a few KB/version) |
| O5 | Governance of restore actions: always approve vs. follow approval policy | Follow policy + enforce precondition hash; retain hash validation under danger_full_access as well (prevent concurrent overwrite) |
| O6 | Include files damaged by bash (outside the write/patch path) in L1? | Do not capture at file level in the first version (bash impact is constrained by sandbox + allowlist); record as a known boundary |

### 6.1 Decision (2026-09-01)

User decision: **the MVP first aligns with Crush; additional functionality is deferred** (the prototype MVP is not out yet, so do not preemptively pursue differentiation).

- O1 → **recording-side parity enters implementation** (file_versions table + write-tool chaining + filetracker stale-read, carried by the VC-3 tail payment); the restore side (L2 session-level rollback + restore RPC/UI) is **removed from the MVP** and attached to the deferred row `RB-L2-DEFER` in `docs/TODO.md` §0.1.
- O2 → follow the recommendation: retain 20 versions per (session,path), 1MB per version (effective immediately when the recording side writes).
- O3 → follow the recommendation: new `file_versions` table (Option A).
- O4/O5 → suspended with the restore side; decide again when RB-L2-DEFER starts.
- O6 → retain the recommendation: do not capture bash impact in the first version; record it as a known boundary.

---

## 7. Eino native-support verification (added 2026-08-31 in response to the user asking "does eino support these natively?")

Baseline: `github.com/cloudwego/eino v0.9.13` (locked in go.mod; source checked against the module cache). Decision: **of the three rollback components, eino natively supports only checkpoint/interrupt recovery (already used by Vivy); file versions/rollback and git semantics have no native support, so L1/L2 must land on the Vivy side—but the attachment seam already exists (Vivy has implemented `filesystem.Backend`).**

### 7.1 Rollback-related capabilities: item-by-item decision

| Rollback component | Native in eino | Evidence |
|---|---|---|
| Checkpoint/interrupt recovery | **Yes, and Vivy already consumes it** | `adk.CheckPointStore/Deleter` (runner.go:64); checkpoint payload = gob-encoded `serialization{RunCtx{RootInput,RunPath,Session}, InterruptInfo, EnableStreaming, InterruptID2Address/State}` (interrupt.go:210/283)—pure runtime state (conversation, agent steps, interrupt state), **zero file semantics**. resume/load also restores only these (interrupt.go:219). eino makes no compatibility promise for checkpoint format (v0.8.x once required byte rewriting to repair gob incompatibility, interrupt.go:244); Vivy's engine-version envelope fail-closed behavior is the correct defense |
| File-version history / rollback | **No** | All `adk/filesystem.Backend` operations = LsInfo/Read/GrepRaw/GlobInfo/Write/Edit (backend.go:243), with no versions, diff return, delete/move/rename; the official `InMemoryBackend` = `map[string]fileEntry{content, modifiedAt}` (backend_inmemory.go:31), with no version chain. Version chain + recovery RPC must be built by Vivy (storage layer §5.4 + hook in the Backend implementation) |
| Git-semantic rollback | **No** | The `filesystem.Shell` protocol has only single commands `Execute`/`ExecuteStreaming` (backend.go:298), with no concept of composite commands/repository operations |

### 7.2 Seam conclusion (no effect on the correction to §5.4)

- Vivy's `EinoFilesystemBackend` already implements both `einofs.Backend` and `tools.FileOperations` (filesystem_backend.go:54)—**the archive hook can live in Vivy's own Backend implementation; nothing in eino needs to change**; the `Write/Edit` call sites are natural chaining points.
- eino's `Write/Edit` request structures have no precondition field—Vivy's `ProposalPreconditionHash` context injection (prevent stale state after approval) is an owned protocol and remains.

### 7.3 Related finding: native eino middleware mapped to the VC track (verified present in v0.9.13)

| Native eino | Corresponding Vivy item | Impact |
|---|---|---|
| `adk/middlewares/filesystem`: native registration of seven tools—`ls`/`read_file`/`write_file`/`edit_file`/`glob`/`grep`/`execute` (with Chinese and English descriptions, filesystem.go:41)—+ large tool-result handling | VC-1 tool surface | grep/glob/edit_file tool shapes and names already exist natively in eino; the VC-1 implementation can write less tool-definition code by aligning the "Backend already aligned, connect middleware registration layer" (map the naming difference edit_file vs. Vivy patch) |
| `filesystem.Shell`/`StreamingShell` + `ExecuteRequest.RunInBackendGround` (backend.go:288) | VC-1 bash + background jobs | The execute-tool protocol natively includes a background flag; but **job-management tools such as job_output/job_kill are absent natively**—the retrieval/termination layer for background jobs still must be built |
| `adk/middlewares/agentsmd`: AGENTS.md injection (recursive @import depth 5, total-byte limit, transient model-call injection not added to session state or summaries) | D6 context-file injection | **D6 is a free pass-through**: more disciplined than in-house preamble injection (transient injection naturally avoids compaction); `vivy init` still needs to generate AGENTS.md itself |
| `adk/middlewares/patchtoolcalls`: repair historical orphaned tool calls | resume/compaction-boundary hygiene | Compare and evaluate; may replace the in-house repair logic |
| `adk/middlewares/plantask`: task_create/get/list/update | Vivy task_* five-piece set | Vivy already built its own (including persistence + dependencies); compare only, no migration |
| `adk/middlewares/summarization`/`reduction`/`skill`/`dynamictool(toolsearch)` | Compaction/skills/tool discovery | Vivy already uses these or has equivalents |
| `adk/filesystem.MultiModalReader` (image/PDF parts) | VC-3 `read_file` image support | Native protocol slot is ready; only the backend side needs implementation |

---

## 8. Write-back actions

- `docs/TODO.md` RB-1 row: write back the conclusion (this file + §5 recommendation; after user confirmation of O1..O6, change to DONE or rewrite according to the decision).
- Add to the VC-1 row notes: combined archive/filetracker storage design + L1 as the bashification safety-net prerequisite.
- Add to the VC-3 row notes: L2 session-level rollback and recovery RPC/UI attachment.

All three actions were executed (2026-09-01, per §6.1 decision): RB-1 changed to DONE; the VC-1/VC-3 rows were rewritten per the decision so the recording side follows the VC-3 tail payment and the restore side is attached to RB-L2-DEFER.
