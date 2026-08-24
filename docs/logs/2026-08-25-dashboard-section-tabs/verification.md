# Verification

## Commands

| Check | Result |
|---|---|
| `just ci` | pass (2026-08-25, exit 0) |

## User-visible smoke

Open `http://127.0.0.1:3015/dashboard`:

1. Default tab 概览 shows 运行状态 and 近期活动, not Token totals.
2. Tab Token shows 总 Token and period chips.
3. Tab 审计 shows 审计日志 and the existing preview banner.
4. 390px: tabs remain usable, no horizontal page overflow.
