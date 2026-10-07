# A2A-06 Verification

A2A-06.3 gate list (all green on this tree):

- `go run ./sdk verify plugins/a2a-server` → `ok projectvivy/a2a-server`
  (also `ok` for repinned discord/qq/tui/persona).
- `go run ./sdk pack --recipe recipes/a2a.vivy.yml --source plugins/a2a-server --output dist/a2a-acceptance` → packs clean.
- `go run ./sdk inspect-artifact dist/a2a-acceptance` → module
  `projectvivy/a2a-server`, provides `vivy.a2a`, grants
  `channel.a2a`+`secret.read`, wired channel edge; no token values.
- `go test ./sdk/internal -run 'TestA2ASelectedAndOmittedArtifacts|TestMinimalArtifactPhysicallyOmitsOptionalModules'` → PASS (25.8s/16.7s; binary `a2a-go` dep present only in the selected artifact).
- `go test ./sdk/internal/conformance -run TestCheckedInProviderConformanceMatchesExecutedSuites` → PASS 53.7s (after repinning executed digests).
- `go test ./sdk/internal/conformance -run 'TestGeneration(FailureMatrixEvidence|RollbackRestoresCatalogAndLocaleIdentity)'` → PASS.
- `go test ./sdk/internal -run TestGenerationFailureMatrixExecutesEveryCase` → PASS (all matrix cases).
- `go test ./sdk/internal/assembly -run 'Test(CompilePluginV1GraphFixtures|StartFailureRollsBackEveryConstructedOwner)'` → PASS.
- `go test ./internal/toolhost -run 'TestMiddleware(TimeoutFailsClosed|PanicAndInvalidDecisionFailClosed)'` → PASS.
- `go -C plugins/a2a-server test ./...` → PASS (module composition, handler/mapping, card, artifact smoke skipped without env).
- `go test ./internal/app -run 'A2A'` → PASS (native service path, local-only approval, listener-without-module).
- `TestA2APackedArtifactSmoke` → PASS 9.7s (real packed binary + scripted tool-requesting model + official client).
- `just ci` → green except `sdk/codeclient TestClientAgainstRealVivyCode`
  (pre-existing main regression; fix in open PR #37, unrelated to this
  branch).
