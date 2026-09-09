# Verification record

## Commands and results (2026-08-27)

- `cd ui; pnpm typecheck` → ✅ passed (tsc --noEmit, no errors)
- `cd ui; pnpm test` → ✅ 105/105 tests passed (including the kiwi zh/en key-parity check;
  `customDialogTitleManage` / `customDialogHintManage` synchronized on both sides)
- Root `just ci` → ✅ all green:
  - gofmt -l found no unformatted files; `go vet ./...` and `go test ./...` all passed
  - ui: pnpm install (frozen-lockfile) → typecheck → test (105/105) → build
    (3.67s; only the existing chunk > 500 kB warning, unrelated to this change)

## Browser smoke test

Ports 8787 / 3015 were occupied by the user's Vivy Studio debug session, so no server was
started separately—in accordance with the existing convention, **the user's Studio debug
session performed the verification**; see acceptance.md. Reminder: the change exists only in
split Vite `http://127.0.0.1:3015` (hard refresh Ctrl+Shift+R); the `:8787` embedded page is
the release package.
