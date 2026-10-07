# VCP-A1 verification

- `go build ./cmd/vivy-code ./cmd/vivy ./sdk/port/face ./sdk/tui/face ./internal/codeface` — clean.
- `go test ./cmd/vivy-code ./internal/codeface ./sdk/tui/face ./sdk/port/face -count=1` — all ok.
- `go test ./sdk/...` — all ok except the pre-existing UI-toolchain failures fixed by installing node_modules + `stage:ui`; the digest gate
  `sdk/internal/conformance` re-verified green after re-pin
  (`TestCheckedInProviderConformanceMatchesExecutedSuites` ok, 60s).
- `go vet` on touched packages — clean.
- Manual:
  - `vivy-code --help` prints the full surface.
  - `vivy-code --version` prints `dev` (buildinfo).
  - `vivy-code --bogus` → `unknown argument` + usage, exit 2 (contract preserved).
  - `vivy-code --mode json -p hi` → `face mode "json" is not available`
    (typed `ModeUnavailableError`, exit 1 — `-p` does not clobber an explicit mode).
  - `echo hello | vivy-code` (piped stdin) → resolves to `print` mode, exit 1
    with the same typed error (pi `resolveAppMode` parity).

## Outstanding

- `just ci` full run deferred: requires 35m+ Go test sweep; focused gates above
  cover the touched surface. CI env lacks nothing else known.
