# Verification: UI audit cleanup

Commands run from the repository root on 2026-09-01:

1. `go build ./...` and `go test ./internal/tools/` — pass after the
   `ScanPrompt` removal.
2. `pnpm exec tsc -b --force` in `ui/` — pass.
3. `just ci` (fmt-check / vet / test / headless-compile / ui-ci) — pass.
   Go packages all green; UI vitest 21 files / 175 tests passed; `vite build`
   succeeded.
4. Browser smoke against the running split dev pair
   (`http://127.0.0.1:3015`, Vite serving this checkout with HMR):
   - `/dashboard`: `getByRole("tab")` returns exactly `["Token", "Trajectory"]`;
     "Active runs"/"Recent activity" text count is 0.
   - `/skills`: tabs are `["Installed skills (0)", "Market"]`; "Change requests" text count
     is 0.
   - `/` chat: `getByRole("button", { name: "New session" })` count is 1 (the
     bottom-row Plus); `getByRole("button", { name: "More" })` count is 0.
     Clicking the bottom Plus created a new session (empty state
     "Start a new conversation" shown).
