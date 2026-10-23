# Verification

## GitHub Actions evidence

- Stable run `39821191793` failed twice in `TestMinimalKernelArtifactBootAndInspect`; both attempts downloaded `github.com/charmbracelet/glamour@v1.0.0` with hash `h1:Dwe8CrZi+HX+1l3c17hFvofKyQL4RwSgXvSvkoqQK6E=` instead of the recorded `h1:AWMLOVFHTsysl4WV8T8QgkQ0s/ZNZo7CiE4WKhk8l08=`.
- After the packer change, stable run `39886111476` failed at the final `go build` with the same mismatch. This established that glamour belongs to the headless binary's real compile graph through `sdk/tui/view`.
- Stable run `39887928685` still failed with the same mismatch after setting `VIVY_GOPROXY=direct`.
- Stable run `39889505999` confirmed the mismatch in a preflight with job-wide `GOPROXY=direct`: it downloaded `Dwe8...` instead of the recorded `AWMLO...` hash.
- With the exact workflow setting `GOPROXY=https://proxy.golang.org,direct`, a fresh isolated Go cache downloaded `github.com/charmbracelet/glamour@v1.0.0` and verified hash `h1:AWMLOVFHTsysl4WV8T8QgkQ0s/ZNZo7CiE4WKhk8l08=` against `go.sum`; Go reported upstream tag commit `69661fd5423129850dbd3b3a6995cd32976f867b`. No checksum was changed.
- The follow-up workflow uses `https://proxy.golang.org,direct` and runs `go mod download -json github.com/charmbracelet/glamour@v1.0.0` before the gates. Its hosted run is still required.

## Linux checkout

Ran from the independent WSL filesystem checkout at source commit `8b00475b1ea84a9013f77eadd9682328f3415976`, with the patch applied and pinned Laputa `9acc999301d5d575da2cea902a1c6d213763f381`:

- `just wiring-ci` — passed, 127 tests, 0 skipped.
- `just kernel-ci` — passed, 1,970 kernel tests; one existing optional `TestBackendGrepRipgrepHonorsGitignore` was skipped because `rg` is unavailable. The required `TestMinimalKernelArtifactBootAndInspect` passed.
- `just d02-ci` — passed, 281 tests, 0 skipped.
- `git diff --check` — passed.
- An optional targeted run with a brand-new module cache was interrupted after 12 minutes while Go cloned the outer `sdk/internal` test package's dependency graph. It produced no test result and is not counted as a pass; the repository kernel gate above completed the required pack/inspect/boot case.

The latest hosted workflow failure is not treated as fixed. The next `stable` run must pass the proxy-backed checksum preflight and wiring, kernel, and D02 steps.
