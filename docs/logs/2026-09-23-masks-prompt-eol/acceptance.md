# Acceptance

On a Windows checkout, all built-in mask prompt files are checked out with LF line endings, and the masks package recognizes the embedded definitions as normalized. `go test ./internal/modules/masks -count=1` and the repository `just ci` gate pass.
