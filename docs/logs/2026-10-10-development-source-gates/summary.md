# Development source provenance

Issue #51 separates ordinary development from historical release source identity.
Static SDK verify no longer compares authored source hashes. The SDK Source
Catalog derives current T1 identities once per root during construction; the
internal default inventory declares composition without reading/hashing files.
T2 source locks, Recipe pins, import/capability firewalls, symlink confinement,
UI dependency/asset checks and executable artifact inspection remain strict.

Ordinary Provider conformance executes the existing Provider/Host commands and
all semantic assertions against current sources. Only comparison copies omit
source identity. No historical outcomes or committed source digests are updated.
`just release-conformance` explicitly checks the historical bundle against exact
sources. Pack attaches historical conformance only for matching selected hashes;
a changed first-party source receives no old passing record. Successful build or
Inspect is artifact integrity evidence, not species publication certification.

Species recipes, capabilities, interfaces and runtime behavior are preserved.
Full SDK publication design remains deferred; this adds no release platform,
dev-mode switches, Notebook migration changes or audit remediation.

## Integration ownership

Base: latest main `e67e6ffaf1b6d71b8acb18b7cf304c669e00bb60`, including merged
PR #48's Laputa, Windows, shutdown and complete memory-loop fixes.

PR #49 was open at `701cc10115fabb181923668a423aabfff7b4002b`; its M1–M6 repairs
remain separate. Shared files: `internal/modules/defaults/catalog.go`,
`sdk/internal/conformance/reproduction_test.go`, and `justfile`. Preserve its
metadata-only optional imports, runtime/QQ fixes and DIVA recall test selection
when integrating. Its authored conformance digest refreshes must not restore
development hash gates; the historical bundle itself remains untouched here.

PR #50 was open at `393c84e10ac7b2e6292b842dad2ca19ace33635b`; shared files are
`sdk/internal/frontend_v1.go`, `sdk/internal/frontend_v1_test.go`, and `justfile`.
This change leaves its removal work to that PR and does not recreate removed
host/ABI functionality. Neither other branch was modified or cherry-picked.
