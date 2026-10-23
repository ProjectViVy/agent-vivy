# Summary

The minimal artifact packer no longer downloads source archives for every module in the repository's full Go module graph. It resolves the graph, then lets the selected Go build fetch and checksum-verify the packages it actually compiles. This keeps UI-only dependencies out of the minimal kernel path while preserving Go's `go.sum` checks.

The change addresses the stable Actions failure where the minimal artifact test downloaded `github.com/charmbracelet/glamour@v1.0.0` despite the minimal Recipe containing no UI. Actions received an archive whose hash differed from the repository's recorded hash; the repository sum was retained.

## Scope

- Removed the eager, unscoped `go mod download` from SDK build dependency resolution.
- Kept module graph resolution against the prepared modfile.
- Left dependency versions and checksums unchanged.

No UI, runtime, storage, schema, or provider behavior changed.
