# W2 iteration 1 — Release notes

## What ships

- `sdk pack --target go-host` plus `--host-dir/--host-package/--host-assets/--host-lock`:
  seals an external Go host executable with generated VIVY assembly and the
  framed Generation manifest overlaid at dependency-compile paths.
- Optional `hostBuild` (`vivy.go-host/v1`) in the Generation manifest:
  module/package, live-resolved host commit/tree digest, host-assets hash,
  tracked input-lock hash, canonical consumer modfile/sum hashes, toolchain
  pins. Participates in the Generation ID.
- `inspect-artifact` verifies go-host artifacts without executing them:
  embedded == sidecar manifest, `frontend/` hash, host binary identity.
  Executable/shared artifacts remain inspectable unchanged.
- Artifact gains `consumer.mod` + `consumer.sum` + `build-report.json`
  (`vivy.go-host-report/v1`) + `checksums.sha256`.
- agent-diva gains `scripts/build-desktop.py` (`build|test|repin`,
  `--development`) and just recipes on branch `feat/wails-go-host`.

## Compatibility

- Default/executable/shared pack behavior unchanged (empty target
  normalizes to executable inside `Pack()`).
- Manifests without `hostBuild` decode and inspect as before.
- `--host-*` flags are rejected on executable/shared targets.
- VIVY remains Wails-independent; the go-host path only builds with
  `vivy_headless` (no `ui/dist` requirement).

## Not shipped

- No mainline merge, product release or published artifact.
- `cmd/diva`, root `go.mod` and Wails glue belong to W3; the wrapper's
  end-to-end run is pending them.
- Wails packaging consuming the sealed executable is W6 scope.

## Reproducibility statement

Same normalized inputs → same Generation ID and byte-identical
`generation.json`/`zz_assembly.go` across unrelated staging paths (proven).
Raw executable bytes may differ by build paths/toolchain metadata; the
authoritative identity is `generation.json` + `build-report.json` +
`checksums.sha256`, not a promise of bit-identical signed binaries.
