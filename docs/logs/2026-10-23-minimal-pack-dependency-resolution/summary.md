# Summary

The minimal artifact packer now resolves the prepared module graph without pre-downloading every module archive. The selected Go build fetches and checksum-verifies its actual compile graph. The Minimal kernel workflow sets both `GOPROXY` and `VIVY_GOPROXY` to `direct`, then verifies and caches the checksum-pinned TUI module before running the gates.

The Actions failure involved `github.com/charmbracelet/glamour@v1.0.0`, which enters the headless executable through `sdk/tui/view`. The runner repeatedly received an archive hash that conflicts with the repository's recorded hash. A direct fetch from the pinned Git tag returned the recorded hash, so the checksum remains unchanged and the workflow explicitly uses the direct source.

## Scope

- Removed the eager, unscoped `go mod download` from SDK build dependency resolution.
- Kept module graph resolution against the prepared modfile.
- Set the Minimal kernel Actions job to direct Go module downloads and preflight the checksum-pinned TUI module.
- Left dependency versions and checksums unchanged.

No UI, runtime, storage, schema, or provider behavior changed.
