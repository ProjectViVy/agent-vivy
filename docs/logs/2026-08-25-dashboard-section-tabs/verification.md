# Verification

## Commands

| Check | Result |
|---|---|
| `just ci` | pass (2026-08-25, exit 0) |

## User-visible smoke

Open `http://127.0.0.1:3015/dashboard`:

1. Default tab Overview shows Runtime Status and Recent Activity, not Token totals.
2. Tab Token shows Total Tokens and period chips.
3. Tab Audit shows Audit Logs and the existing preview banner.
4. 390px: tabs remain usable, no horizontal page overflow.
