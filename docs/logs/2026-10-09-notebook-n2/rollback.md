# N2 — rollback

Revert the N2 commit (`feat(notebook): bind optional scoped notebook actions and tools`).

Then, on a checkout that must still run:

1. Remove `vivy/notebook-core` and `vivy/notebook-tools` from `recipes/default.vivy.yml` (and delete `recipes/notebook-backend.vivy.yml`, `recipes/no-notebook.vivy.yml` if desired).
2. `go generate ./internal/generated/assembly` to restore the assembly without the notebook seam.
3. Restore the legacy note-tool binding: `internal/app/assembly_tools.go` — drop the generated-ID suppression loop; `internal/tools/tools.go` — re-add `NewWriteNote/NewListNotes/NewReadNote` unconditionally if a `storage.NoteStore` is available.
4. Data written by N2 is unaffected: migration `037` only adds nullable columns (`content_origin`, `exclude_automatic_ingest`) to `tool_operations` and `messages`. Down-level code ignores them; no down-migration is needed and none is provided. Notebook content tables from N1 (`036`) are untouched and remain usable by `vivy notebook export`.
5. Policy: removing `withNotebookHumanOrigin` restores the pre-N2 behavior where notebook writes require full-auto/approval under the default profile; `PolicyEvaluation.Matched` may be dropped or left (readers tolerate the field).
6. Re-pin `internal` `sourceSha256` in `sdk/internal/assembly/conformance_results.json` with `go run ./sdk/internal/cmd/source-hash internal ""` if `internal/` content reverted.
