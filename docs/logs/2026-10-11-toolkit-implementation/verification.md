# Verification

## TK-01

- RED: `go test ./internal/bundlecontract -run 'Test(DecodeManifest|BundleIDs|ManifestPaths)' -count=1` failed for missing contract symbols.
- RED: digest/native Skill test command failed for missing digest/parser symbols.
- RED: `go test ./internal/bundlecontract -run TestView -count=1` failed for missing immutable View.
- GREEN: `go test ./internal/bundlecontract ./internal/bundles -count=1` passed.
- `go vet ./internal/bundlecontract ./internal/bundles` passed.
- `git diff --check` passed.
- Golden digest independently derived with Python hashlib/json/struct:
  `ac57d0d77f4eedfd11049d85a9f87be5c4b49d030cfd98c94a2755ea0f96a64b`.
- Full Windows CI pending; focused checks are not full-product acceptance.

## TK-02

- RED: `go test ./internal/bundles -run 'Test(Acquire|ObjectRecovery)' -count=1` failed for missing acquisition APIs.
- RED: `go test ./internal/bundles -run TestGitAcquisitionHasNoExecution -count=1` failed before Git extraction was implemented.
- GREEN: `go test ./internal/bundlecontract ./internal/bundles -count=1` passed.
- GREEN: `go vet ./internal/bundlecontract ./internal/bundles` passed.
- GREEN: `git diff --check` passed.
- GREEN: Windows cross compilation of the `bundles` and `bundlecontract` test binaries passed with `GOOS=windows GOARCH=amd64 go test -c`.
- Local and pinned Git fixtures verify declared-file-only copies, source immutability, no repository-hook execution, cancellation, owner availability, and stored-byte tamper detection.
- Full Windows CI pending.

## TK-03

- RED: migration-count and backend contract tests failed before schema/store implementation: SQLite backend did not expose `BundleStore`; migration count was 40.
- GREEN: `go test ./internal/storage ./internal/storage/migrations ./internal/storage/sqlite ./internal/storage/postgres -run 'Test(Bundle|EmbeddedManifestHasCanonicalPairs|ApplyFreshAndReapplyIsNoOp)' -count=1` passed. The PostgreSQL bundle conformance case is skipped because `VIVY_POSTGRES_TEST_DSN` is unset.
- GREEN: full `go test ./internal/storage ./internal/storage/migrations ./internal/storage/sqlite -count=1` passed.
- GREEN: `go vet ./internal/bundlecontract ./internal/bundles ./internal/storage ./internal/storage/bundlesql ./internal/storage/sqlite ./internal/storage/postgres` passed.
- GREEN: Windows cross compilation of storage and bundle test binaries passed.
- GREEN: `git diff --check` passed.
- PostgreSQL live transaction conformance remains blocked: no configured DSN, PostgreSQL executable or running Docker daemon is available in this environment.

## TK-04 / TK-05 — lifecycle and admission integration (in progress)

- GREEN: `go test -tags=http2legacy ./internal/bundles -run 'TestToolkit(PublishOrder|RestoreRun|Rollback|UpdateOnly|GarbageCollection|Recovery)' -count=1` passed during lifecycle integration.
- GREEN: `go test -tags=http2legacy ./internal/runtime ./internal/bundles -run 'TestToolkit|TestReportWorkflowAdmissionStoresToolkitSnapshot' -count=1 -timeout=90s` passed.
- GREEN: `go test -tags=http2legacy ./internal/runtime -run TestToolkitRevocationStopsModelAndToolBoundaries -count=1 -v -timeout=60s` passed.
- GREEN: SQLite and PostgreSQL package bundle conformance commands passed; PostgreSQL shared conformance skips live database cases because `VIVY_POSTGRES_TEST_DSN` is unset.
- Coverage currently proves ordinary/root and report admission markers, parent-to-child capture inheritance, workflow admission snapshots, durable blockers, queued-turn resumption, exact retained release restore after disable, and model/tool revocation fences. The full admission matrix, runtime restart restoration fixture, app composition evidence, settings/engine publication integration, race sweep and complete CI remain open.
- App package compile attempt is blocked before Toolkit integration can be checked: sibling `../laputa` does not expose `WithMissionRevision`, `CaptureActivity`, `LookupCapture` or `ArchiveCapturedSession`, and its `CaptureRequest` lacks `Activity`. No sibling source was changed.

## TK-06 — hosted Skills and declared resources (in progress)

- GREEN: `go test -tags=http2legacy ./internal/bundles ./internal/runtime ./internal/skillhost ./internal/tools -run 'TestBundleSkill|TestHostedBundleSkill|TestHostedSkillBackend' -count=1 -timeout=90s` passed.
- Tests cover captured release bodies under a shared stable ID, model/explicit lookup through SkillHost, duplicate IDs from two sources, required captured scope, unavailable host activation, read-only tool operations, UTF-8 resource reads, traversal/undeclared path denial, binary and oversized resource denial.
- The compiled source is registered from app composition and Runtime `skill_view` resolves through SkillHost before reading a declared artifact resource. Local Skill mutation remains delegated to the existing owner. Real app fixture and app package build remain blocked by the pre-existing sibling API mismatch above.

## TK-07 — Toolkit Masks and independent recommendations (in progress)

- GREEN: full tests for `internal/maskcontract`, `internal/modules/masks`, `internal/storage`, `internal/storage/sqlite`, `internal/storage/postgres`, `internal/actionhost` and `internal/bundles` passed with `-tags=http2legacy`.
- GREEN: focused Runtime Mask/Skill tests and `TestNativeOneShotChildApprovalRecoversThroughServiceAfterRestart` passed.
- GREEN: `go test -tags=http2legacy ./internal/storage/sqlite -run '^TestBackendConformance$' -count=1 -timeout=180s` passed, including shared copied-Mask metadata, selection/capture and reference conformance.
- GREEN: focused tests cover PM/Dev Mask catalog, builtin/custom merge, schema-2 bundle provenance, legacy schema-1 reads, exact epoch selection, stable selection across a same-ID release update, disabled selected Masks, removal blockers, copy provenance/idempotency, unavailable recommendations, SkillHost scope non-expansion, and read-only bundle definitions.
- GREEN: after the full Runtime suite exposed legacy one-shot child recovery without a Toolkit `run.started` event, `TestNativeOneShotChildApprovalRecoversThroughServiceAfterRestart` passes. Missing markers remain legacy only when no run-bundle snapshot row exists; a snapshot without its marker fails closed.
- PostgreSQL package tests compile and pass, but `TestBackendConformance` skips live database cases because `VIVY_POSTGRES_TEST_DSN` is unset.
- Runtime full-suite still has the pre-existing `TestCognitiveCompletedUnresolvedWindowDoesNotAdvance/rejected` fixture failure. `TestMCPStatusErrorPreservesPathsAndSyntheticEnvironmentValues` passes with `TMPDIR=/tmp`; the default temporary path contains a synthetic resolved environment value in its failure output.
- App composition cannot be built because sibling `../laputa` lacks `WithMissionRevision`, `CaptureActivity`, `LookupCapture` and `ArchiveCapturedSession` and has no `CaptureRequest.Activity`; app owner wiring is recorded but awaits that dependency fix.
- `just ci` is unavailable in this environment.

## TK-08 / TK-09 — Rules, Hooks and MCP (in progress)

- GREEN: `go test -tags=http2legacy ./internal/runtime -run 'TestToolkit|TestBundle(Hook|MCP|Skill)|TestReportWorkflowAdmissionStoresToolkitSnapshot' -count=1 -timeout=120s` passed.
- GREEN: focused cases cover Code-only sorted immutable Rule snapshots, Rule size and prompt-budget rejection, project agents.md delegation, exact Release/declaration consent, Hook revocation fences, typed MCP merge, duplicate/unsafe names, binding and consent activation gates, and exact captured MCP call authorization.
- GREEN: MCP runtime status is projected into Toolkit inspection without exposing error text or resolved environment values; `TestToolkitInspectionIncludesSecretFreeMCPRuntimeStatus` passed in the core Go package run below.
- The complete Hook protocol matrix, app settings publication races, local MCP transport smoke and app composition remain open. `../laputa` still prevents compiling `internal/app`; see TK-04/05 and TK-10/12.

## TK-10 — authenticated Toolkit actions (in progress)

- GREEN: `go test -tags=http2legacy ./internal/modules/toolkits -count=1` and `go test -tags=http2legacy ./internal/actionhost -count=1` passed.
- GREEN: action inventory/schema limits, duplicate/unknown/null input rejection, local/Git source-kind pinning and human-only consent identity checks pass. The first focused run found the action test fixture no longer satisfied the private `ActionHost` interface after `SetBindings` was added; implementing that fixture method fixed the test.
- GREEN: SDK default Assembly generation and `go run -tags=http2legacy ./sdk verify plugins/vivy-toolkits-ui` / `... vivy-masks-ui` passed. Current UI source pins are Toolkits `00a277212d96789aee4fd8ba3907e9c4189a7516e648a4ef6221bae180c2e29f` and Masks `9fc9e4e8257900331dc090664687323b74de9fbb799b0bbfc9c1149735f2b9da`.
- GREEN: `go test -tags=http2legacy ./sdk/internal -run 'Test(RepositoryModulePathsUseCanonicalNamespace|VerifyEverySelectedPublicModule|StageUIDerivesFirstPartyEditsBeforeSelection)' -count=1` passed, covering first-party Module path/source registration and staging selection.
- The actual app package build and action invocation over authenticated RPC are not verified because of the sibling `../laputa` API mismatch. No sibling source was edited.

## TK-11 / TK-12 — dedicated UI and Toolkit management (in progress)

- GREEN: the targeted Vitest run for the Toolkit page/client/update UI and Masks page/client/session completed with 6 files and 18 tests passing. This includes stale catalog response protection, all five owner sections, update digest/consent review, retained-release rollback, binding conflict form retention, secret-reference-only submission, bundle Mask copy without session selection changes, and recovery after a lost publish response.
- GREEN: `pnpm --dir ui exec tsc --noEmit --pretty false` passed. A temporary focused TypeScript project covering the production Toolkit and Masks Module sources also passed and was removed afterward.
- GREEN: `go run -tags=http2legacy ./sdk/internal/cmd/generate-default --repo . --output internal/generated/assembly/zz_default.go` and `GOFLAGS=-tags=http2legacy pnpm --dir ui run stage:ui` passed. The plain `stage:ui` invocation first failed because its nested `go run ./sdk stage-ui` omitted the repo-required `http2legacy` build tag; setting `GOFLAGS` resolved the environment/build invocation. Generated files were produced only by SDK commands.
- GREEN: `go run -tags=http2legacy ./sdk verify plugins/vivy-toolkits-ui` and `... vivy-masks-ui` passed; both local plugin Go modules pass `go test ./...` when run from their respective module roots.
- One initial UI assertion still expected an obsolete mixed-language label (“Masks / 面具”); it was corrected to the current localized “面具” label, after which all 18 targeted tests passed.
- Real split-server/browser smoke, keyboard/responsive acceptance, app-backed operation recovery and artifact omission/inspection remain open under TK-13.

## Current focused regression pass

- GREEN: `go test -tags=http2legacy ./internal/bundles ./internal/modules/masks ./internal/maskcontract ./internal/actionhost ./internal/storage ./internal/storage/bundlesql ./internal/storage/migrations ./internal/storage/sqlite ./internal/storage/postgres` passed. PostgreSQL package tests compile, but live database conformance remains skipped because `VIVY_POSTGRES_TEST_DSN` is unset.
- GREEN: `go test -tags=http2legacy ./internal/runtime -run 'TestToolkit|TestBundle(Hook|MCP|Skill)|TestReportWorkflowAdmissionStoresToolkitSnapshot' -count=1 -timeout=120s` passed.
- GREEN: after the action fixture correction and gofmt, `go test -tags=http2legacy ./internal/bundlecontract ./internal/maskcontract ./internal/modules/toolkits ./internal/modules/masks ./internal/actionhost` passed.
- GREEN: root UI TypeScript check, both Module SDK verifications and both plugin-module Go tests passed.
- BLOCKED: `go test -tags=http2legacy ./internal/app -run '^$'` fails before reaching Toolkit integration because `../laputa` lacks `WithMissionRevision`, `CaptureActivity`, `CaptureRequest.Activity`, `LookupCapture` and `ArchiveCapturedSession` APIs used by existing cognitive module code.
- Full repository CI, live PostgreSQL, full Runtime suite and TK-13 app/browser/artifact acceptance remain open. No commit or push was made.

## TK-13 — integrated acceptance (planned; prerequisite gates open)

- BLOCKED: `GOFLAGS=-tags=http2legacy go run ./sdk pack --recipe recipes/default.vivy.yml --output <new temporary directory>` reaches SDK artifact build generation but cannot compile `internal/app` because the sibling `../laputa` APIs listed above are missing. No valid artifact was produced, so `inspect-artifact` was not run against a generated artifact.
- BLOCKED: real app lifecycle/browser smoke cannot start until the app package compiles. Disposable live PostgreSQL conformance also awaits `VIVY_POSTGRES_TEST_DSN`; no connection string or database fixture was available.
- The focused module verification is not a substitute for packaging or omission acceptance. `TK-13` remains Planned until TK-12 and these app/database/browser gates can be exercised end to end.
