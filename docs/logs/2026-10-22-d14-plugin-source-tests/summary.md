# Independent module source-contract tests

Remove the obsolete `descriptor.Source.SHA256` assertion from each declaration
test in `plugins/coding/session-tree`, `plugins/vivy-evolution`,
`plugins/vivy-memory`, `plugins/vivy-notebook` and `plugins/vivy-persona`.
The field was already removed from the production Source contract. These five
stale test references prevented the independent module CI lane from compiling.

The implementation delta is exactly five test-line deletions. API version,
Module identity/version, Port/provider, source reference, catalog, lifecycle,
Grant and relation checks remain intact. No production source, descriptor,
declaration, generated output, dependency, schema or format changed. No new
tests or compatibility fields were added.

No production data was accessed or release produced. The parent owns the next
full CI run and final artifact/runtime-pin acceptance; this lane only verifies
the five affected independent modules.
