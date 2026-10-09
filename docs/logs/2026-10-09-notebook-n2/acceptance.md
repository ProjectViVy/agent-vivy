# N2 — acceptance

Every checkbox in `docs/superpowers/plans/notebook-reports/N2.md` is met:

- **Task 1**: selected/omitted/duplicate/mistyped/closed-handle compositions verified at generator and app level; `HasNotebookFactory()`/`NotebookFactoryValue()` present; `core/notebook-service@v1` registered in the canonical closed catalog.
- **Task 2**: forged origin/actor/scope rejected at the strict-schema boundary (wire carries no authority fields — provenance is bound from the server-resolved peer identity only); direct authenticated human edit admitted under the normal profile; explicit deny and explicit prompt rules both remain decisive; unauthenticated writes nothing; agent-bound invocations carry `Origin=agent` and keep frozen-run policy; all 18 §7 actions typed with strict input schemas, declared effects, and bounded results; same `operation_key` reconciles the receipt after response loss.
- **Task 3**: `list_notes`/`read_note`/`write_note` bound through the module's ToolProviders with a 4 KiB result bound; physical module omission removes the tools; notebook ToolOperations, projected messages, and folded compaction summaries carry `ContentOrigin=notebook` + `ExcludeAutomaticIngest`; automatic observer delivery excluded while the cursor still advances; paired `037` migration on both dialects; CN-45 asserts durability across `Reopen` on sqlite and Postgres.
- **Task 4**: recipes updated; `zz_default.go` regenerated via `go generate`; `vivy-sdk verify` green; pack+inspect on all three recipes proves physical inclusion/omission; headless RPC exercise covers CRUD/CAS/comments/revisions/export; `just ci` green.

## Hand-off notes

- **N3** receives the frozen action schemas (`internal/modules/notebook/actions.go`): envelope `{operation_key, request}` for mutations, `additionalProperties:false` everywhere, `outcome{status,data,error{code,message,retryable,current_version,current_revision_id}}`, and the 13 contract error codes from N1.
- **R0** receives the hooks: `ScopeID`/`WorkspaceScope`, `Origin`/`Actor`, `ExcludeAutomaticIngest`/`ContentOrigin` on ToolOperations + Messages, `HasExcludedToolOperations`, and the `PolicyEvaluation.Matched` rule-vs-default distinction.
