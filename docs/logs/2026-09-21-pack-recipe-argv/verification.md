# Verification — pack walks the v1 recipe contract

## Commands run

| Command | Result |
| --- | --- |
| `go build ./...` | ok (whole repository compiles with the new `Pack` signature; no other Go consumer) |
| `go vet ./internal/studiocore ./cmd/vivy-studio` | ok |
| `go test ./internal/studiocore ./cmd/vivy-studio -count=1` | ok, 13.7s — includes `TestPackRequiresRecipe` and `TestPackRecordsV1Artifact` (fake vivy-sdk emits the real `Artifact` stdout shape; asserts ledger id = `manifest.generationId`, sha256 of the packed binary, `file:` source ref, empty recipe bill, and `--recipe/--source/--output` argv pass-through) |

## Why these checks

The change is confined to the Studio lifecycle producer. The sdk-side
contract (`parsePackArgs`, stdout `Artifact` JSON, output-must-not-exist)
is pinned by the existing sdk suites and was read line by line, not
re-tested here. `TestPackRecordsV1Artifact` fakes exactly the wire shape
documented in `sdk/internal/frontend_v1.go:34-38,1720`.

## Deferred to the follow-up real-path smoke

`just ci` runs once at the end of the species-workbench delivery (its
result will be recorded in `docs/logs/2026-09-21-species-workbench/verification.md`).
The manual proof that a real `vivy-studio pack --recipe
recipes/minimal.vivy.yml` produces a bootable candidate belongs to the
workbench acceptance pass, since it is minutes-long and shares the same
verification session.
