# Summary

The minimal artifact packer now resolves the prepared module graph without pre-downloading every module archive. The selected Go build fetches and checksum-verifies its actual compile graph. The Minimal kernel workflow uses the official Go module proxy and verifies the checksum-pinned TUI module before running the gates.

The Actions failure involved `github.com/charmbracelet/glamour@v1.0.0`, which enters the headless executable through `sdk/tui/view`. The runner's direct VCS download repeatedly disagreed with the recorded hash, while the official proxy returned the recorded checksum in an isolated fresh-cache verification. The checksum remains unchanged.

## Scope

- Removed the eager, unscoped `go mod download` from SDK build dependency resolution.
- Kept module graph resolution against the prepared modfile.
- Set the Minimal kernel Actions job to the official Go module proxy and preflight the checksum-pinned TUI module.
- Left dependency versions and checksums unchanged.

No UI, runtime, storage, schema, or provider behavior changed.
