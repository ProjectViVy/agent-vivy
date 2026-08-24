# Verification

## Commands

| Check | Result |
|---|---|
| `just ci` | pass (2026-08-25, exit 0) |

## User-visible smoke

Playwright against `http://127.0.0.1:3015/dashboard`:

- 1天: twelve blue stacked columns fill the track
- 3天: three columns, capped width, not full-width slabs
- 1周: seven columns fill the track
- 390px: no horizontal overflow
