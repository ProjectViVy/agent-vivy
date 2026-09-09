# Verification record — Vivy UI theming

## just ci (repository root)

Command: `just ci` (= fmt-check + vet + go test + headless-compile + ui-ci)

Result: **passed (exit 0)**, 2026-08-25.

- go: fmt-check / vet / `go test ./...` / `go test -run '^$' -tags vivy_headless`
  all green.
- ui-ci: `pnpm install --frozen-lockfile` + `pnpm typecheck` (tsc clean)
  + `pnpm test` (all 30 cases in 8 test files passed, including 8 cases in the new
  `src/hooks/use-theme.test.ts`) + `pnpm build` (✓ built in 3.11s).

## Browser run (smoke-for-user-visible-change)

Environment: reused the running split pair (Vite `http://127.0.0.1:3015` +
control plane 8787; the browser was the built-in ZCode browser, and Vite HMR
loaded these changes).

Path and results:

1. Open `http://127.0.0.1:3015/` and go to the “Settings” → “General” tab:
   - The new “Theme” card renders after “Application Info”; all 5 theme buttons
     are present, with “Vivy Blue” initially `[pressed]` and a “Selected” icon.
   - The old fake theme card is gone from the migration preview (remaining:
     Chat Display / Cache / About).
   - `DemoNote` covers only the migration-preview area.
2. Click “Love”:
   - `<html data-theme="love">`, `html.dark` count 0.
   - Screenshot confirmed: light-pink background/sidebar, white cards, pink
     selected state, and no rendering errors (screenshot:
     `vivy-theme-love.png`, temporary directory).
3. Click “Miku Teal”:
   - `<html data-theme="miku">`, `html.dark` count 1 (`dark:` variant active).
   - Screenshot confirmed: dark background/sidebar/cards + teal accent, with
     normal text contrast (screenshot: `vivy-theme-miku.png`).
4. Refresh the page (persistence + anti-flash bootstrap):
   - After refresh, `<html data-theme="miku">` and `.dark` remain; the page is
     still dark Miku, confirming that `vivy.theme` localStorage and the index.html
     bootstrap script work (screenshot: `vivy-theme-miku-reload.png`).
5. Switch back to “Vivy Blue”: `data-theme="default"`, `.dark` removed, default restored.

Known note: `just run` started by this session exited because `OPENAI_API_KEY`
was missing, and `pnpm dev` exited because port 3015 was already occupied.
Existing services were already running on both ports, so the smoke test used the
existing pair and the conclusion is unaffected.

## Unverified items

- Conversation flow with a real Provider (this change does not touch the RPC/message path).
- The embedded UI (:8787) was not run separately: it packages the same
  `index.html` + `styles.css`, the theme code path is identical to Vite, and
  `just ci`’s `pnpm build` covers the build.

## Addendum (2026-08-25, pre-commit review)

The “Love” theme (`love`) described among the 5 themes in this document was
subsequently removed, narrowing the theming feature to 4 themes;
`use-theme.test.ts` was updated to use `pink` as well. See
`docs/logs/2026-08-25-remove-love-theme/` for that iteration and its verification
(`just ci` at the repository root was all green after removal, exit 0).
