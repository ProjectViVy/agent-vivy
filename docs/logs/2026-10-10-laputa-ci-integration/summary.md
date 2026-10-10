# Laputa dependency integration repair

Advance the supported source lock from ff3936f44ff8cf08c12af2cf698c194cfe474fd3 to merged Laputa PR #5, commit 4b2bec2cc2ab3374b7ec1ab8d448e612c1e4db24. Main's DIVA cognitive consumer already requires its mission revision, capture activity, receipt lookup, and archive APIs. The old bootstrap pin caused the same compile failure in backend, UI generation, and browser CI.

The existing bootstrap remains the source checkout authority. No runtime implementation, module version declaration, workflow, test assertion, notebook branch, or architecture contract changes. No merge, deployment, or release is included.
