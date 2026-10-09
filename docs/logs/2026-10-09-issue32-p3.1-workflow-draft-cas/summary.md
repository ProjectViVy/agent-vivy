# P3.1: Workflow draft CAS and opaque ETags

This work closes the same-author draft overwrite path and removes timestamp-derived ETags. A create request must identify create intent; an edit must carry the current ETag. SQLite and PostgreSQL keep author scoping and return conflicts for stale or duplicate writes.

## UI Module development record

- Module: `vivy/workflow-ui`, T2, source `repo:plugins/vivy-workflow`, declared SHA-256 `a0fcd1ea3fc3c57e68b4004e098ef8d9a40b92107b684c1841e9b7254547999d` before edits.
- Port: cataloged `std/ui-extension@v1`; Provider `vivy.workflow-ui.sidebar`; sole Host Consumer `PresentationHost`.
- Authority: the Module renders the workflow editor and calls the authenticated `inofy.*` action surface. Session, draft ownership, revision and admission checks remain backend-owned.
- Grants/Recipe: no Grants; `recipes/default.vivy.yml` selects the Module in the default Generation.
- Lifecycle/failure: generation scope; no backend resource owner or cleanup is introduced. UI save failures remain visible; server-side CAS and author checks are authoritative.
- Existing artifacts: Definition/Port in `VIVY-PORT-CATALOG.md`, SDK in `@vivy/ui-sdk`, Provider in `plugins/vivy-workflow/module.go`, Consumer in the generated PresentationHost composition, conformance in the UI action/Module suites, and provenance in Module source hash plus Inspect. P3.1 changes only the Module's editor behavior and source hash.
- Scheduled work: `docs/superpowers/plans/issue32-remediation/P3-workflow.md`, Task P3.1. The storage, runtime and RPC regression tests were observed failing before implementation.

## UI continuation

The `oil-frontend` skill remains absent from this checkout and the available skill catalogs. The user explicitly directed continuing until all work items are complete, so the recorded ruling applies: use current Module conventions and `testing-vivy-ui` for these behavioral changes, with no visual redesign.

The browser implementation now serializes `null` as explicit create intent, rejects an empty edit ETag locally, and opens published revision content with the current own-draft ETag (or explicit create if that draft is missing). The two-editor race regression proves that the second creator receives a conflict instead of replacing the first saved artifact. Module source hash is `2247d763d31ebd397ea322df97494a7b2a006fcb28802c38e263dd592a1d3656`.

UI tests, source hash, generated assembly staging/typecheck and the P3.1 Go package suite are recorded in `verification.md`. Backend implementation is in `64a3196c`; this UI continuation is committed independently. Candidate browser acceptance remains part of P7.
