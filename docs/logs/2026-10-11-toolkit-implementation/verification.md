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
