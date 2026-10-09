# PR #43 merge conflict resolution

Merged main `83e46efc6c6d56a6a07ba7dbf60fae9617e6268e` into PR #43,
whose previous head was `12cef8198bdda7fb7ced68dae0024d28c4565183`.

The sole conflicted file was `sdk/internal/assembly/conformance_results.json`.
Both branches had updated the five records bound to the whole `internal/`
source tree. Recomputed the digest with the repository's canonical source
hasher and set those records to
`6ec575e0a87d125980d51ee8296e2a4a60a1e0619520d55de6d89ec875dd606f`.

Preserved PR #43's RPC notification ordering implementation and tests, and
main's bash redirect classification implementation and tests, byte for byte.
No runtime behavior, interfaces, or dependencies were added by this resolution.
Release notes are omitted because this is a PR branch synchronization, not a
release. The PR is left open for review and merge.
