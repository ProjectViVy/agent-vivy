# Acceptance — TUI-WORKSPACE-OWNERSHIP

## How a person can confirm it works

1. Start the backend and UI (`just dev`), open `http://127.0.0.1:3015`, and start a
   tool-enabled session. Let the run create a workspace, then open the file
   panel — file-list and preview behavior remain unchanged (regression surface).
2. Send an RPC with an unknown run id directly to the same backend (for example,
   call `workspace/list` from the browser console with
   `run_id: "run_i_do_not_exist"`): it should return `run not found` 404, and
   **no new directory should appear** under the workspace root in `data/`.
   (Before the fix: it returned a successful empty list and created
   `run_i_do_not_exist/` on disk.)
3. Call `workspace/list` for a run id that exists in the Journal but has never
   been started: it should return `run workspace not found`, with no directory
   creation either.
4. The TUI `vivy-code` `/files <run>` command shows an error result for an
   unknown run instead of silently succeeding.

## Regression risks

- After a run normally creates a workspace, file-panel behavior is unchanged
  (existing runtime/rpc tests all green).
- When a run has just been created and its workspace has not been generated,
  the panel changes from an "empty list" to showing the `run workspace not found`
  error text — more honest semantics, and an expected change.
