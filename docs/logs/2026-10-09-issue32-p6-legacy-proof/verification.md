# P6.1 verification

## Red evidence

Before the guard, the focused legacy approval test observed `DecideApprovalAsActor`
return nil and the old pending row was consumed. A post-restart approval was
also consumed. This established the unsafe path at the first-writer boundary.

## Eino and decoder inventory

- Inspected pinned Eino `v0.9.13` `compose.NewWorkflow`, `Workflow.Compile`,
  checkpoint serialization and `schema.RegisterName` APIs.
- Inspected the existing VIVY `VersionedCheckpointStore`: its `Get` verifies
  the outer engine/checksum/prompt envelope and returns opaque Eino bytes; it
  does not decode graph types.
- Reviewed `docs/logs/2026-09-29-inofy-cutover/verification.md`: graph builder
  callers were already test-only; the production approval resume dispatch was
  the only live route into the proof.
- `go list -f '{{.GoFiles}}' ./internal/runtime` excludes
  `orchestration_proof_test.go`; `TestGoFiles` includes it. Production `rg`
  found only the legacy marker detector, with no proof graph, type or
  registration references.

## Green evidence

- `go test ./internal/runtime -run '^Test(LegacyOrchestration|NativeOrchestration|Orchestration|INOFY|Cognitive)' -count=1` — passed.
- Final `go test ./internal/runtime -count=1` after the SQLite close/reopen
  fixture and guard-order review — passed (`39.965s`).
- `go build ./...` — passed with a temporary `ui/dist/.keep`, required because
  the checkout has no embedded UI build output. The placeholder was removed
  after the build.
- `go test ./internal/runtime -run '^TestINOFYWorkflowCancelPropagates$' -count=3` — passed after making the expectation match both pinned-engine outcomes.
- `git diff --check` — passed.

The authored INOFY and trusted strategy workflow regressions complete through
their current admission paths. Historical approval rows remain pending after
actor and system attempts; journal history gains no decision, tool-operation or
terminal work event. Descriptor content and checkpoint bytes read back
unchanged.

## Pending

`just` is not installed, so `just ci` was not run. P7 retains the aggregate
product gate and final integration evidence.

Implementation commit: `fd1954f1` (`refactor(runtime): isolate orchestration
proof and reject legacy resume`).
