# D06 shared core Generation identity closure

Removed `Generation.ArtifactSHA256` through the shared domain, frozen lifecycle
service, SQLite/PostgreSQL storage, RPC create/list/get results, Face SDK input
and result types, UI API and lifecycle card. Creating a Generation no longer
requires or writes a dummy artifact hash. Identity, source selection, Recipe,
phase, timestamps and existing human/evaluation/promotion gates remain owned by
the same services. The lifecycle list retains Generation ID and phase.

Paired append-only migration037 removes only `generations.artifact_sha256`.
Released migration files were not rewritten. The final runtime preflight checks
required Generation columns and refuses the retired field. Supported baseline
shapes are checked before returning upgrade_required; drift is refused before
writes. The explicit finite route remains baseline035 -> disposable copied
target -> ApplyManifest through036/037 -> verified reopen. Interim source036
remains an unsupported post-baseline format. No runtime in-place legacy upgrade
or second converter was introduced.

SQLite and real PostgreSQL fixtures preserve Generation IDs, parents, exact
source refs/Recipes/phases/timestamps, evaluation/promotion links, human actor
and event history, and prove failed column removal rolls back. The independent
Studio ledger and its release/install history remain with the previous D06
Studio fragment. No tenant data was accessed. D14 still owns publication of the
complete finite upgrade artifact and the final configuration switch.

No agent loop, model/tool orchestration or Eino implementation changed; existing
pinned Eino adapters remain authoritative. Existing headless eval smoke helpers
now build the supported headless candidate without requiring UI assets.
