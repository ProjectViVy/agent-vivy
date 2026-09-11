# PLG-P6 Task 7 implementation report — UI conformance and removal proof

Date: 2026-09-11
Branch: `feat/plugin-v1-p6`
Scope: PLG-P6 Task 7 only

## Outcome

Task 7 adds a hermetic conformance matrix for the full-code UI path and seals
its evidence into the P6 record. The matrix covers four explicit fixture
Generations — `default`, `extension`, `replacement-root`, and `minimal` — and
does not discover source directories, load remote entries, or use a network
runtime. Each Go fixture generates the typed Assembly, typechecks it in a
temporary source tree, seals the UI projection through `SealManifest`, and
round-trips it through `InspectManifest`.

The new browser suite exercises the real `composeUI`, `PresentationHost`, and
`createModuleActionClient` seams. It proves exclusive/missing/duplicate
Providers, contradictory extension order, runtime install rollback, reverse
and idempotent cleanup, provenance display, typed `module.action.invoke`
failure handling, and minimal Generation behavior. The minimal assertions
check that old `vivy.plugin/v0`, `vivy.generation/v0`, UI Grant/permission,
and legacy route markers do not return to the selected presentation.

The P6 evidence ledger now points the two UI Ports at this conformance suite
and its sealed Inspect matrix. The Control Action seven-artifact record keeps
the real authenticated RPC failure/conformance tests as its authority proof,
with the same sealed Inspect projection. No UI Grant, permission registry,
runtime discovery, or arbitrary Action route was added.

## Seven-artifact evidence

`TestTask7P6PortsHaveCompleteSevenArtifactProof` checks all seven required
evidence kinds for `std/ui-extension@v1`, `std/ui-root@v1`, and
`std/control-action@v1`, verifies each source path exists, and evaluates each
ledger entry as `SUPPORTED`. The entries are:

- Port definition: the public `sdk/port/catalog.go` catalog.
- SDK contract: `sdk/ui/src/module.ts` and `sdk/port/controlaction/action.go`.
- Host consumer: `PresentationHost` and `internal/actionhost/host.go`.
- Real provider: generated UI constructors and the full UI fixture Action
  provider.
- Failure model: runtime rollback and authenticated RPC rejection tests.
- Conformance suite: the new browser matrix (UI) and authenticated RPC
  suite (Action).
- Inspect projection: the four-generation `SealManifest`/`InspectManifest`
  matrix and the existing final-artifact inspection tests.

## TDD and focused verification

The new browser test was first run while its matrix assertion intentionally
failed on the default fixture's no-navigation behavior; the fixture-specific
assertion was corrected and the suite then passed. The Go matrix initially
exposed nil-versus-empty JSON normalization during Inspect comparison; the
assertion was made semantic and now passes.

Focused commands that passed in this worktree:

```text
cd ui && pnpm typecheck
cd ui && pnpm exec vitest run src/plugins/conformance.test.tsx
cd ui && pnpm exec vitest run src/plugins/presentation-host.test.tsx src/plugins/conformance.test.tsx
PATH=/workspace/scratch/82edc4b9648e/toolchains/go1.26.4/bin:$PATH GOFLAGS=-buildvcs=false go test ./sdk/internal/assembly -run 'Task7|GenerateNonEmpty' -count=1
PATH=/workspace/scratch/82edc4b9648e/toolchains/go1.26.4/bin:$PATH GOFLAGS=-buildvcs=false go test ./sdk/internal/assembly -count=1
```

## Deferred gates and boundary

The repository `just` executable is not installed in this Linux runner, so
`just ci` is recorded as deferred. The split development pair required by
the opt-in Playwright fixture was not running, so the Playwright smoke is also
deferred. These are release-conformance inputs owned by the separately
`UNSCHEDULED` PLG-P9 phase; they are not a P6 release claim. P6 capability
implementation is complete, with broad verification/review left to the
consolidated P6 lane.

No external release, push, or PR was started.
