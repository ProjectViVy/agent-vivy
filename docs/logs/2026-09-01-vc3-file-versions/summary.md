# VC-3 recording side: `file_versions` rows + filetracker stale-read + plugin-write chaining (kernel)

## What changed

Following the 2026-09-01 O1..O6 ruling (MVP aligns with Crush first; restore side
deferred under RB-L2-DEFER), implement the **recording-side** kernel portion of
the file-version chain:

1. **Storage layer (sqlite + Postgres backends)**
   - New `file_versions` table (session_id, run_id, path, version, content_hash,
     content, created_at; UNIQUE(session_id, path, version); retain 20 versions
     per (session,path) = O2; skip a single version >1MB without truncating) and
     `file_reads` (stale-read marker, PRIMARY KEY(session_id,path)).
   - sqlite migration019; Postgres base-schema addition + schemaV18Upgrade +
     schemaVersion=18.
   - New `storage.FileVersionStore` contract: `RecordFileMutation` (one
     transaction performs Crush-style chaining: first-seen baseline → if the
     chain latest differs from the old disk content, insert an external
     intermediate state → append new content, with hash deduplication; skip over
     the limit) + `TrackFileAccess` (upsert marker) + `LastFileAccess`. **No
     version-read API by design**—restore consumers (RPC/UI) are in RB-L2-DEFER,
     so there is no interface surface without a consumer.
   - The new tables have no foreign keys; session deletion uses explicit DELETE
     (avoiding deletion-order hazards exposed by the compactions foreign key).
   - The conformance suite adds CN-18 (contract: recording does not error,
     tracker round trip, session-delete cascade); chain semantics (baseline /
     intermediate state / deduplication / retention / over-limit behavior) are
     asserted by direct table tests in the sqlite and Postgres packages.

2. **Write-path chaining (`internal/runtime`)**
   - New optional `tools.FileVersionRecorder` seam (same injection pattern as
     WriteDiagnosticsSource) + `runtime.FileVersionRecorder` adapter (best effort,
     failures only log Warn).
   - `EinoFilesystemBackend` hooks three points: TrackAccess after successful
     `ReadFile`; RecordMutation + TrackAccess after successful `WriteFile`; and a
     stale-read guard before `WriteFile` (filetracker: disk mtime newer than the
     session's last-access marker → reject "changed on disk after the last read;
     read it again before editing"; **never-tracked paths are allowed**—the guard
     targets stale edits, not mandatory read-before-write).
   - patch/multiedit/native Eino write_file/edit_file all go through `WriteFile`,
     so one hook covers the full path.
   - App assembly:
     `fileBackend.SetFileVersionRecorder(runtime.NewFileVersionRecorder(backend, nil))`.

3. **Plugin-write chaining (`internal/pluginhost`, slice 2)**
   - `pluginhost.Adapt` adds a `tools.FileVersionRecorder` parameter (nil preserves
     old behavior). `hostedEnv.OpenWrite` changes from raw
     `os.OpenFile(O_TRUNC)` to a kernel-side capture wrapper: **opening does not
     truncate**; old content is snapshotted and remains on disk until Close;
     plugin writes buffer first (1MB cap), and Close is the destructive moment—
     truncate + flush + RecordMutation + TrackAccess.
   - Over-limit behavior: if new or old content cannot be fully snapshotted
     (>1MB / read failure), skip chain recording (never poison the chain with a
     truncated snapshot), but refresh the access marker normally; an abandoned
     writer that never closes produces no record.
   - Close is idempotent (at most one record); no session context and nil recorder
     silently pass through. The sdk/plugin surface and plugin source are unchanged
     (lsp_rename enters the chain automatically).

4. **Related fix (the same deletion-cascade surface)**
   - Add `session_compactions` to the `DeleteSession` (sqlite + Postgres) cascade
     list—Postgres had a latent bug: compactions rows declared
     `FOREIGN KEY REFERENCES sessions(id)`, but the deletion list omitted the
     table, so deleting a session with compaction failed with an FK error. It is
     fixed alongside the new tables as part of the "session deletion covers all
     child tables" surface.

## What was explicitly NOT done

- **All restore-side work is deferred** (files/versions RPC, files/restore,
  session-level rollback UI) → RB-L2-DEFER, to revisit after MVP validation.
- **download/bash writes do not enter the version chain**: both are out-of-band
  changes that bypass file tools (known O6 boundary); bash/download changes
  refresh disk mtime, and the stale-read guard correctly requires the model to
  read before editing.
- No new Journal events, RPC, or UI changes (outside the ruling's scope).
