# MASK-1 Task 2 report

## Status

DONE_WITH_CONCERNS

## Commit

- d45f84c feat: add closed mask assembly foundation

## Changed files

- Added internal/moduleport/masks.go with MaskDependencies and typed
  MaskFactory.
- Added internal/modules/masks/module.go and lifecycle tests for the
  backend-only vivy/masks descriptor.
- Added the closed core/mask-service@v1 owner/cardinality entry and default
  Source Catalog binding. The default Recipe remains unchanged and does not
  select vivy/masks.
- Added MaskFactory metadata propagation through defaults, SDK Source Catalog
  conversion, and default-generation conversion.
- Extended runtime Assembly generation with an optional typed factory field,
  selected-provider validation, selected factory emission, and omitted-provider
  import tests. Generated Assemblies expose HasMaskFactory for app-side sealed
  binding validation.
- Added compiler tests for public ownership rejection, duplicate cardinality,
  and backend-only operation without a UI Port.
- Recorded core/mask-service@v1 as SPECIFIED only in
  docs/architecture/VIVY-PORT-CATALOG.md.

## Verification

- git diff --cached --check — passed before commit.
- git show --check --oneline d45f84c — passed.
- go test ./internal/moduleport ./sdk/internal/assembly ./internal/modules/masks -run Mask -count=1
  — unavailable: /bin/bash: go: command not found.
- just ci — unavailable: /bin/bash: just: command not found.

## Concerns

- Go compilation, formatting, focused tests, and just ci remain unverified
  because this environment has no Go or Just executable.
- MaskFactory generation intentionally points to the staged masks.Open
  constructor metadata, while the real storage-backed Open implementation
  belongs to MASK-2. No placeholder success Provider or fake Ready path was
  added in this Story. A selected mask Assembly must therefore be regenerated
  after MASK-2 supplies masks.Open.
- The checked-in generated default Assembly was not edited manually, per the
  Task 2 constraint; the default Recipe does not select the optional provider.

## Review follow-up

- Conditionalized the generated internal/moduleport import, MaskFactory field,
  and HasMaskFactory accessor so omitted Assemblies contain no mask
  implementation or internal module-port import. This preserves compilation
  of external generatedtest modules under Go internal import rules.
- Updated the omitted-provider generator test to assert the complete absence
  of masks, moduleport, MaskFactory, and HasMaskFactory strings.
- Follow-up static check: git diff --check passed. Go and just remain
  unavailable.

## Review follow-up 3

- Added the real-catalog production selection gate in Compiler.Compile. A
  selected vivy/masks entry is rejected with the SPECIFIED diagnostic whenever
  the catalog carries the existing vivy/storage production marker; isolated
  test catalogs remain available for foundation tests.
- Wrapped invalid public, duplicate, and production compile paths with a
  start-stage counter so all rejection tests prove no lifecycle start stage is
  entered.
- Added reverse Stop/Close exactly-once mask-labeled lifecycle evidence using
  the existing Generation rollback fixture.
- Follow-up static check: git diff --check passed. Go and just remain
  unavailable.

## Review follow-up 2

- Added the production Source Catalog gate: selecting vivy/masks in a
  catalog carrying the real vivy/storage marker now fails with
  core/mask-service@v1 is SPECIFIED and cannot be selected, before generation
  or lifecycle start. Isolated compiler fixtures remain usable for backend
  ownership and UI-independence tests.
- Added explicit zero-start counters to invalid public and duplicate mask
  compiler tests, plus a production-selection rejection test.
- Added mask-labeled Generation rollback evidence covering reverse Stop/Close
  order and exactly-once counters for the failing and constructed owners.
- Renamed the omitted-provider test to TestMaskOmittedHasNoProviderImport.
- Follow-up static check: git diff --check passed. Go and just remain
  unavailable.

## Review follow-up 4

- Reworked the rollback evidence to construct the real
  `internal/modules/masks.NewModule()` descriptor and owner, then place it in
  the selected lifecycle sequence with an explicit failing owner. The test
  asserts reverse Stop/Close order and exactly one invocation for every
  constructed owner.
- Removed the `compileWithoutStarting` callback helper. Invalid public,
  duplicate, and production selections now call `Compiler.Compile` directly
  and compare a typed `module.Instance` start guard before and after compile;
  the compiler receives descriptors/source metadata only and must leave the
  guard untouched.
- Added a test-only typed `maskFactoryFixture`, the explicit
  `var _ moduleport.MaskFactory = maskFactoryFixture` assertion, and a
  `go/parser` AST check that the selected generated `MaskFactory` literal
  targets that fixture selector. This keeps production `masks.Open` out of
  the foundation test while checking the generated typed binding.
- Follow-up static check: git diff --check passed. Go and just remain
  unavailable.

## Review follow-up 5

- Replaced the local selector-only mask fixture with the importable
  `sdk/internal/assembly/testdata/maskfixture` package. It exports the typed
  `Factory`, `NewModule`, and a lifecycle instance, and asserts
  `var _ moduleport.MaskFactory = Factory`.
- The selected generator test now emits `maskfixture.Factory` and verifies the
  selector with an AST check. Added `TestGeneratedSelectedMaskAssemblyCompiles`,
  which copies the fixture and generated Assembly into a temporary package
  under this module's `testdata`, then invokes `runtime.GOROOT()/bin/go test`.
  This keeps the internal/moduleport import boundary valid and type-checks the
  actual exported selector; the test skips when the Go toolchain is absent.
- Added a real `compileThenStart` test pipeline. Rejected public, duplicate,
  and production SPECIFIED selections never call `module.StartGeneration` and
  leave the guard at zero, while the valid backend-only mask fixture starts a
  generation and records exactly one Start before Close.
- Follow-up static check: git diff --check passed. Go and just remain
  unavailable.
