# Verification record

## Commands and results (2026-08-27)

- `cd ui; pnpm typecheck` → ✅ passed (tsc --noEmit, no errors)
- `cd ui; pnpm test` → ✅ 105/105 tests passed (including the kiwi zh/en key-parity check;
  `editAddressAria` added to both sides)
- Root `just ci` → ✅ all green:
  - gofmt -l (all .go files under cmd/internal/sdk/ui) found no unformatted files
  - `go vet ./...` and `go test ./...` all passed
  - ui: pnpm install (frozen-lockfile) → typecheck → test (105/105) → build
    (3.94s, output dist/assets/index-*.js 974.12 kB; only the existing chunk > 500 kB
    warning, unrelated to this change)

## Browser smoke test

Ports 8787 / 3015 were occupied by the user's Vivy Studio debug session (vivy-backend pid
22900, Vite pid 21516), so no server was started separately—in accordance with the existing
convention, **the user's Studio debug session performed the verification**; see
acceptance.md. The user was reminded that the change exists only in split Vite
`http://127.0.0.1:3015` (hard refresh with Ctrl+Shift+R required); the embedded page at
`:8787` is the release build and does not contain this round's UI changes.
