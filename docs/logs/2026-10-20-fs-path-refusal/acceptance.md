# Acceptance: filesystem path mistakes do not fail the run (2026-10-20)

The fix is observable in a normal chat turn, without reading journals.

1. Rebuild and restart the dev split pair after landing this change
   (`just dev`, or `just run` plus `cd ui; pnpm dev`). The backend binary must
   be rebuilt so it contains the new kernel code.
2. In a session whose workspace is empty, ask:
   “看看工作区有啥”
   - The model may call `list_dir` with `"./"` or `"."`.
   - The build step should finish with a model answer instead of the generic
     failure “The model run could not be completed. Please try again.”
   - An empty workspace is reported as an empty list (`entries: []`), not
     `null`; the model can say the workspace is empty.
3. In the same or another session, make the model use an invalid path, for
   example by asking it to list `../outside` or a protected file such as
   `keys.txt`.
   - The run should continue; the model receives a tool result that starts
     with `tool argument "path": ...`.
   - No file outside the workspace is read, and no protected file is exposed.
4. Negative check: a missing path on a readonly tool still produces a
   model-visible “not found” result, and a missing path on a write tool keeps
   its existing fatal behavior.

The executable regression proof is
`internal/runtime/filesystem_path_test.go`; it drives the real
`EinoFilesystemBackend` and the existing `classifyToolFailure` allowlist. The
live split pair was intentionally not restarted by the implementation lane, so
step 1 is the human action that makes the change visible in an existing
instance.
