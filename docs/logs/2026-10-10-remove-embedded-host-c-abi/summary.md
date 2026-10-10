# Remove Embedded Host and C ABI

Removed the deprecated native Embedded Host and its C ABI from VIVY.

- Deleted `cmd/vivy-shared`, `internal/embedded`, `sdk/host/v1`, the FFI smoke
  harness, and the SDK's Go host packer, tests, and fixtures.
- Removed `shared` and `go-host` pack/inspect paths and the `HostBuild`
  Generation Manifest field and validation.
- Removed `App.StartEmbeddedServices`; `App.Run` remains the single owner of
  background lifecycle services, and affected tests now exercise that path.
- Updated the active README, recipe, and Laputa bootstrap comment. Fixed
  `fmt-check` to check existing tracked and untracked Go files, so deletions
  and renamed tests pass the repository gate.

The ordinary executable packer and generic embedded Generation Manifest are
retained. Historical logs remain unchanged.
