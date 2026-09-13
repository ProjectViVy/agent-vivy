# PLG-P8 Gate A verification

Date: 2026-09-13

## Focused Gate A

- `go test ./sdk/internal/assembly ./sdk/internal -run 'TestP8SCXGateA' -count=1`
  — PASS.
- Sole-consumer hardening, including the dormant optional dependency case, was
  exercised first as a failing regression and then passed in
  `3848d39eb7e9ca156611608eba0d6bd3509f1edc`.
- `git diff --check` — PASS.

## Complete repository gate

The workspace has no `just` or PowerShell executable, so the Linux equivalents
of every `just ci` constituent were run with Go 1.26.4. `GOFLAGS=-buildvcs=false`
was used only because nested artifact builds cannot obtain VCS status from this
linked-worktree layout; the repository already records this workaround, and a
normal GitHub checkout does not need it.

- Go formatting check over tracked files — PASS.
- plugin-v1 fixture validator and eight-case corpus — PASS.
- `go vet ./...` — PASS.
- `go test -timeout 20m ./...` — PASS, including the SDK pack/Inspect suite.
- headless compile for `cmd/vivy`, `cmd/vivy-code`, and `ui` — PASS.
- per-module `go vet ./...` and `go test ./...` for every standalone Module in
  `plugins/` and `faces/` — PASS.
- UI frozen install, typecheck, 316 tests, Vite production build, catalog
  completeness, and eight cross-face i18n tests — PASS.

The previously failing documentation-baseline workflow run `34761436760` was
rerun before this work. Backend CI, UI CI, and aggregate `just ci` all passed,
showing its earlier approval-durability failure was non-reproducible and
unrelated to PLG-P8.

## Real pack and Inspect

Both commands packed executable artifacts and `inspect-artifact` returned the
same sealed identity as `pack`:

| Recipe | Generation ID | Expected SCX-facing state |
| --- | --- | --- |
| `recipes/default.vivy.yml` | `45bc9529d1b18d40c161e523c6afcac5dd95449fd08a19b3f294db73e4dead87` | Context/Skill Hosts and Sources selected; MCP compiled but unconfigured |
| `recipes/minimal.vivy.yml` | `3600600dd76e545ac37bec1add6d0154df68d362287beaff8bf99a6fff771acd` | Context/Skill/MCP omitted; channels and MCP `NOT_COMPILED` |

The temporary artifact directory is not release evidence and is not retained.
P9 remains responsible for release artifacts, deterministic rebuild, removal,
and exercised whole-Generation rollback.
