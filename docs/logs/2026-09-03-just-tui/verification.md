# Verification

- `just --list` — passed; lists `tui` as the real VIVY CODE terminal face.
- `just --dry-run tui` — passed; resolves to `go run ./cmd/vivy tui`.
- `just ci` with `GOFLAGS=-p=2` — passed, including 197 UI tests, production UI build, root Go vet/tests, headless compile checks, and all plugin/face modules.
