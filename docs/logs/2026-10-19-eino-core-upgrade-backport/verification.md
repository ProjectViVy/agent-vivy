# Verification

Date: 2026-10-19

Environment: Windows, Go 1.26.4 (matches `go.mod`), `GOPROXY=https://goproxy.cn,direct`,
`../laputa` sibling present. Commands run from the repository root of
`feat/issue-91-base` unless noted.

## Dependency and build

- `go get github.com/cloudwego/eino@v0.9.21` — PASS. Diff is the `eino` require
  line plus two `go.sum` hashes; the remaining change is `require` block
  reordering already present in the module graph. No transitive pin changed.
- `go build ./internal/runtime/` — PASS.
- `go build ./internal/provider/` — PASS with `third_party/eino-claude` and the
  `replace` line.
- `go build ./internal/...` — not a valid gate on a fresh checkout: it fails on
  `ui/embed.go: pattern all:dist: no matching files found` because `ui/dist` is
  built by `just ci` first. Not an Eino regression; unchanged behavior.

## Tests

- `go test -run 'EngineVersion|MissingCheckpoint' -count=1 ./internal/runtime/`
  — PASS (2.3s). Cases: `TestVersionedCheckpointStoreEngineVersionFailClosed`,
  `TestRecoverAcrossEngineVersionCancelsPendingApproval`,
  `TestRecoverAcrossEngineVersionCancelsPendingQuestion`,
  `TestRecoverWithMissingCheckpointCancelsPendingApproval`,
  `TestRecoverAcrossEngineVersionCancelsPendingPlanReview`.
- `go test -run 'Recover' -count=1 ./internal/runtime/` — PASS (21.0s), all
  pre-existing recovery tests unchanged.
- `go test -count=1 ./internal/provider/` — PASS with the vendored Claude
  module.

## Not run

- `go test -race` for the nudge/tool-node suite (the benefit evidence for
  `EINO-TOOLSNODE-ERR-RACE`); the upstream log records the command and result at
  v0.9.21.
- Full `just ci`, `just ui-ci`, `headless-compile`, Docker build, real Postgres,
  real-provider run. These remain the final-candidate and PR-stage gates; none
  is claimed as passed here.
- UI build: no UI source changed.

## Limitations

The provider patch is a verbatim upstream component plus one-line race fix; it
is not a fork of behavior, but it must be re-checked when the upstream module
releases. `gofmt` was checked only on changed Go files.
