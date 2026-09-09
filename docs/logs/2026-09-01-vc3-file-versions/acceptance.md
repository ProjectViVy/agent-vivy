# Acceptance: how to verify manually

This slice covers the kernel recording side and has no separate UI surface. The
observable behavior is in the file-tool path of an agent session:

1. **Version-chain recording**: have an agent write/patch/multiedit a file in a
   session, then verify that `data/vivy.db` has corresponding rows in
   `file_versions`—the first write to a path creates two rows (pre-change
   baseline + post-change content), later edits append one row each, repeated
   writes of identical content append nothing, and after more than 20 versions
   only the newest 20 remain. `file_reads` gains a `(session, path, read_at)` row
   after `read_file`.
2. **Stale-read guard**: after the agent reads a file in the session, change it
   externally (manual edit, bash, or download), then patch it again; the patch is
   rejected and the tool result contains "changed on disk after the last read;
   read it again before editing". After the agent reads it again, editing works.
3. **Plugin writes enter the chain too**: after the agent changes a file through
   a plugin write tool such as `lsp_rename`, `file_versions` contains the same
   chain rows as kernel write tools (baseline + post-change). If a plugin write is
   canceled or fails mid-write (without Close), the file retains its old content
   and no chain row is recorded.
4. **Session-delete cascade**: deleting a session that changed a file removes
   that session's rows from `file_versions` / `file_reads` /
   `session_compactions` (Postgres previously failed on a compactions foreign key;
   this is fixed).
5. **Invisible surface**: deployments without a workspace and calls without
   session context behave as before; failed historical writes produce only a Warn
   log and never block the file tool itself (best-effort guarantee).
