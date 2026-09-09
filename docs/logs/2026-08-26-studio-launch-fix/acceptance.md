# Acceptance — 2026-08-26 studio-launch-fix

User perspective: Vivy Studio starts and opens normally.

## Acceptance steps

1. Run `.\launch-vivy-studio.ps1` (or the existing launch entry); it no longer exits with an error.
2. The startup log contains `dsh web: http://127.0.0.1:3090`.
3. Open `http://127.0.0.1:3090` in a browser; the Studio main UI loads normally
   (not 502/blank).
4. The top-right/bottom-right debugger float (◈, dsh-vivy-debugger) works and
   opens to show Gateway status/logs.
5. The plugin marketplace (dsh-plugin / Plugin Hub) entry works, and
   `/dsh-plugin-hub/settings` returns normal data.

## Failure criteria

If startup still throws `ERR_MODULE_NOT_FOUND` / `Cannot find package 'dsh-plugin'`,
or `:3090` is unresponsive, the fix is considered unsuccessful.
