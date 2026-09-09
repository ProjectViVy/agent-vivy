# Verification

## Commands

| Check | Result |
|---|---|
| `just ci` | pass (2026-08-25, exit 0) |

`just ci` covers `fmt-check` + `vet` + `test` + `headless-compile` + `ui-ci`
(`pnpm typecheck`, `pnpm test` — 7 files / 19 tests passed — and
`pnpm build`).

## Static checks

- No remaining references to `AuditDrawer` or the header audit button
  (`grep AuditDrawer|aria-label="审计"` over `ui/` returns nothing).
- `DIVA_AUDIT_EVENTS` is consumed only by the new `AuditPanel`;
  `diva-preview-data.test.ts` still passes unchanged.

## User-visible smoke

Open `http://127.0.0.1:3015/dashboard` in the split pair: the "Audit" card is
visible below "Recent Activity"; tabs, date picker, and refresh feedback work. The
top-right header no longer shows the Audit button.
