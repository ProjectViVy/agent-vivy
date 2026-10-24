# W2 iteration 1 — Verification

All commands run on `agent-vivy@feat/wails-migration`, Go 1.26.4.

## Focused pack suite

`go test ./sdk/internal -run 'TestPackGoHost|TestInspectGoHost|TestGoHost|TestParsePack' -count=1` → **ok 61.9s**, all pass:

- `TestGoHostFixtureRequiresGeneratedReplaceClosure` — consumer build fails
  citing `dashimaki/laputa`, `dashimaki/garden`, `agent-vivy/bml`,
  `agent-vivy/plugins/` (Task 1 replace non-inheritance proof).
- `TestPackGoHostRejectsHostFlagsOnOtherTargets`,
  `TestPackGoHostRequiresEveryHostFlag`, `TestPackGoHostRejectsUnsafeHostPaths`,
  `TestPackGoHostRejectsMissingInputs`, `TestParsePackGoHostHappyPath` —
  flag parsing, traversal/absolute/missing-pin rejection (Task 2).
- `TestPackGoHostSealedExternalArtifact` — full pipeline: binary built,
  `hostBuild` provenance sealed (schema/module/package/live host commit),
  embedded manifest == `generation.json`, `frontend/` hash ==
  `assets.sha256`, `vivy.go-host-report/v1` report + `generationId`,
  `checksums.sha256` verified line-by-line, `InspectArtifact` round-trip.
- `TestPackGoHostResolvesUnpinnedHostIdentity` — lock without
  host.commit/treeSHA256 packs successfully; live identity sealed.
- `TestPackGoHostRejectsSourceHashMismatch`,
  `TestPackGoHostRejectsRecipeDrift`, `TestPackGoHostRejectsDirtyRelease`,
  `TestPackGoHostRejectsOutputReuse` — pack-time rejections.
- `TestInspectGoHostRejectsTamperedArtifact` — tampered frontend byte,
  sidecar manifest, embedded manifest each fail Inspect independently.
- `TestPackGoHostReproducibleGenerationID` — two unrelated staging paths →
  byte-identical `generation.json` + `zz_assembly.go`.

## Story gates

- `go test ./sdk/internal ./sdk/internal/assembly ./sdk/internal/conformance -count=1` →
  **ok 271.6s / 6.2s / 52.9s** — all green; executable/shared pack+inspect
  conformance unchanged.
- `just ci` → see below.

## Wrapper parity

- Python `source_tree_hash` vs Go `hashSourceTree` on a fixture repo with a
  symlink: identical digest `ab864722…` — the dev-mode lock the wrapper
  generates passes SDK verification.

## Pending (environment)

- Full `scripts/build-desktop.py --mode build|test` run: pending W3
  (`cmd/diva` + root `go.mod` do not exist in agent-diva yet). Wrapper arg
  parsing, lock generation and staging verified by inspection and `--help`.
- Native CGO/toolchain needs: `go1.26.4`, CGO for the platform webview deps
  W3 adds; no CGO in the current go-host path (fixture built `CGO_ENABLED`
  default). Wails CLI pins recorded in the lock (`v3.0.0-beta.27`) for W3.
