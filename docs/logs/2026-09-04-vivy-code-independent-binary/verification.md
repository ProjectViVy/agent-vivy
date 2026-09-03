# Verification

- `go test ./internal/codeface -count=1` — passed, including two unique runtime layouts and concurrent organism leases.
- `go test ./internal/app -run '^TestApplySettingsOverlayAtCanBeSharedOutsideRuntimeData$' -count=1` — passed; a private runtime consumed the shared provider settings document without moving its data root.
- `just vivy-code` and `vivy-code.exe --help` — passed; produced the independent headless-tagged binary.
- `just --dry-run tui` — passed; builds `cmd/vivy-code` and launches `vivy-code.exe`.
- Two concurrent smoke launches with the same temporary `VIVY_USER_HOME` created distinct `code-instances/<instance>/vivy.db` files and did not report a shared-Journal lease collision.
- `just ci` with `GOFLAGS=-p=2` — passed, including 197 UI tests, production UI build, root Go vet/tests, the `vivy_headless` compile of `cmd/vivy-code`, and all plugin/face modules.

No production Journal path under `data/` is used by verification.
