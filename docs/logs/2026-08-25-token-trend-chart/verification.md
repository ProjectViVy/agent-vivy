# Verification

## Commands

| Check | Result |
|---|---|
| `just ci` | pass (2026-08-25, exit 0) |

## User-visible smoke

Playwright against `http://127.0.0.1:3015/dashboard`:

- 1 day: twelve blue stacked columns fill the track
- 3 days: three columns, capped width, not full-width slabs
- 1 week: seven columns fill the track
- 390px: no horizontal overflow
