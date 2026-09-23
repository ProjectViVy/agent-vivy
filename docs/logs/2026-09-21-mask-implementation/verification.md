# Verification evidence

Commits reviewed in this implementation slice include the Markdown composer,
native prompt middleware, atomic admission, runtime resume integration, the
Resolver/frame dependency seam, custom-capture error classification, the sealed
App admission gate, the symmetric edit-marker retry migration, the typed header
slot, independent code mode and the removable UI Module.

- `git diff --check`: passed.
- `python3 -m json.tool schemas/events/payloads/run.started.json`: passed.
- Python validation parsed all 39 checked-in JSON files, including the mask UI
  catalog: passed.
- The removable mask UI source-only TypeScript check with repository React and
  SDK aliases: passed; the temporary config lives outside the repository.
- The 19-file `vivy/masks-ui` source digest matches both `module.go` and
  `vivy-module.yaml`: passed.
- Static source checks for prompt marker fields, literal-brace handling and
  duplicate authoritative instruction guards: passed.
- SDK UI Vitest: 28 tests passed (`sdk/ui/src`).
- UI targeted Vitest: 6 tests passed for RPC and independent code mode.
- Removable mask UI Vitest with an isolated alias config: 11 tests passed.
- UI typecheck reached the compiler; remaining diagnostics are existing generated
  UI imports and compatibility fixtures, not the new code-mode files.
- Workspace-local Go 1.26.4 was installed and verified against the official
  toolchain checksum. The focused mask, runtime, app, storage, RPC, assembly and
  provider tests passed.
- `GOFLAGS=-buildvcs=false go test ./... -count=1`: passed for every Go package,
  including SDK artifact packing/inspection, generation failure matrices,
  storage, conformance and eval suites. The flag is required in this checkout
  because Go 1.26 otherwise discovers the outer `/workspace/.git` before the
  linked worktree and fails VCS stamping; it does not disable any product test.
- `gofmt` on all changed Go files and `git diff --check`: passed. The checked-in
  internal-source conformance digest was refreshed and its independent producer
  gate passed.
- `just ci`, browser smoke, and live-model evaluation remain unrun: `just` and
  the browser/PowerShell release tooling are not installed, while live-model
  evaluation requires credentials. These are release gates, not masked as
  passing by the local Go verification.
