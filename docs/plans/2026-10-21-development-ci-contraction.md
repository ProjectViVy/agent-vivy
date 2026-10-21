# Development rules and layered CI contraction

Owner-approved scope: simplify project instructions and skills, add one shared
path/Recipe/test mapping, and use small CI during development with full product
CI at Epic closeout. Preserve shipped Recipes, runtime behavior, data contracts,
existing full-product checks and the `just ci` aggregate name. No push or remote
settings change is authorized.

Implementation order:
1. Unify rules and retain environment-aware `vivy-kernel-ci` guidance.
2. Implement `just ci-small --base <ref> [--scope <id> ...] [--plan]` and explicit
   UI Recipe sessions, with deterministic selection/failure tests.
3. Unify workflow orchestration: automatic scoped PR/branch checks; `epic-final`
   PR label enables full lanes after scoped success, rerunning on updates.
4. Review, execute focused checks and attempt one final full-product CI run.

One `.github/ci-scopes.json` owns paths, Recipes and check associations. Explicit
scopes add to automatic selection. Unknown executable paths fail; zero selected
required tests never pass. Local and GitHub use the same selector. Preserve
history, mark active plans with applicability notices, and use proportional
delivery records. External skills remain unchanged outside this project.

Acceptance: selector union/rename/untracked/unknown-path cases; Recipe validation;
command and required-test failure propagation; label/update/removal and aggregate
semantics; one no-UI real combination and one selected-Recipe split browser
combination where the available host permits; final full CI results reported
without claiming missing lanes passed.
