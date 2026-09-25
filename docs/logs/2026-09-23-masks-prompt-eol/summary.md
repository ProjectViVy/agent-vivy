# Keep Built-in Mask Prompts LF-Normalized

## Scope

Added a targeted Git attribute for `internal/modules/masks/prompts/*.md` so the embedded prompt assets are checked out with LF line endings on Windows. This keeps the bytes consumed by `go:embed` consistent with the mask catalog's normalization contract.

No prompt content, runtime behavior, or repository-wide Markdown line-ending policy changed.

## Explicitly Not Done

- No broad line-ending conversion was applied to Markdown files.
- No live PostgreSQL service or model credentials were used.
