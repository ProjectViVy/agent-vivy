# Verification

- Clean detached sibling pair at original Vivy 59a673ac and exact Laputa ff3936f: `go test -tags vivy_headless -run '^$' ./cmd/vivy ./sdk` fails on WithMissionRevision, CaptureActivity, CaptureRequest.Activity, LookupCapture and ArchiveCapturedSession. This establishes the original source state failure independently of modified root files.
- Metadata import regression: RED before extraction; `node --test scripts/dependency-closure.test.mjs` GREEN (2 tests).
- Bootstrap regressions: RED on revision mismatch, modified exact-pin source and Go mismatch before guards. `PWSH=<pwsh> node --test scripts/ensure-laputa.test.mjs` GREEN (9 tests), including clean source fixtures for invalid module/Go versions.
- `go mod download -json` for all three exact new pseudo-versions succeeded, reporting full upstream SHA and checksums recorded in go.sum.
- `go test -tags vivy_headless -run '^$' ./cmd/vivy ./cmd/vivy-code ./sdk ./internal/runtime`: PASS.
- `go test -run '^$' ./internal/app ./sdk/internal ./ui`: PASS with existing built UI assets.
- `go test ./internal/modules/defaults ./internal/modules/diva-cognitive ./internal/modules/memory ./internal/modules/masks`: PASS.
- Pinned Laputa `go test ./agentapi -run 'Test.*(Mission|Capture|Frozen)' -count=1`: PASS.
- `go list -deps ./sdk`: no concrete optional cognitive/memory/mask implementation or Laputa packages. Independent review verified defaults and sdk/internal too.
- `go mod verify` returns missing ziphash errors for the repository's local v0.0.0 face/plugin replacements. Remote Laputa downloads verified successfully; this broad local-replacement check is not claimed as passing.

Final clean after-checkout, minimal/default pack/inspect, source-bound conformance reproduction and integrated just ci results will be recorded in the final repair verification. Scratch toolchains and fixture trees are excluded locally and are not release inputs.
