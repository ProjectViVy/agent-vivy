# Verification

## GitHub Actions evidence

- Stable run `39821191793` failed in `TestMinimalKernelArtifactBootAndInspect` while `go mod download` fetched `github.com/charmbracelet/glamour@v1.0.0`.
- The downloaded archive hash was `h1:Dwe8CrZi+HX+1l3c17hFvofKyQL4RwSgXvSvkoqQK6E=`; `go.sum` records `h1:AWMLOVFHTsysl4WV8T8QgkQ0s/ZNZo7CiE4WKhk8l08=`. GitHub's retry produced the same mismatch.
- A fresh isolated Go cache fetched the pinned Git tag directly with checksum verification and returned the recorded `AWMLO...` hash. A fresh local request through `proxy.golang.org` also returned that recorded hash. No checksum was changed.
- The new source candidate has not yet run in hosted Actions.

## Linux checkout

Ran from the independent WSL filesystem checkout at source commit `8b00475b1ea84a9013f77eadd9682328f3415976`, with the patch applied and pinned Laputa `9acc999301d5d575da2cea902a1c6d213763f381`:

- `just wiring-ci` — passed, 127 tests, 0 skipped.
- `just kernel-ci` — passed, 1,970 kernel tests; one existing optional `TestBackendGrepRipgrepHonorsGitignore` was skipped because `rg` is unavailable. The required `TestMinimalKernelArtifactBootAndInspect` passed.
- `just d02-ci` — passed, 281 tests, 0 skipped.
- `git diff --check` — passed.
- An optional targeted run with a brand-new module cache was interrupted after 12 minutes while Go cloned the outer `sdk/internal` test package's dependency graph. It produced no test result and is not counted as a pass; the repository kernel gate above completed the required pack/inspect/boot case.

The hosted workflow still needs to run against the pushed candidate.
