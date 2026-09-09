# Remove the Love theme

## Changes

Following user feedback that the Love theme was unattractive, the entire theme
was removed. The theming feature keeps 4 themes: Vivy Blue (default), Minimal
Pink & White, Deep Blue Night, and Miku Teal.

- `ui/src/hooks/use-theme.ts`: removed the `love` entry from `THEME_IDS` and
  `THEMES`.
- `ui/src/styles.css`: removed the `[data-theme="love"]` token block.
- `ui/index.html`: removed love from the anti-flash inline style and the
  bootstrap script’s `APPEARANCES` table.
- `ui/src/hooks/use-theme.test.ts`: DOM application/persistence cases now use
  `pink`; the storage fallback case uses `'love'` as an invalid value, explicitly
  covering the path where a deleted theme’s leftover localStorage falls back to
  the default.

For browsers with `vivy.theme=love` already stored, the TS-side
`readStoredTheme` and the index.html bootstrap script’s ID allowlist both reject
the value and fall back to `default`; no migration is needed.

## Explicitly not done

- No values in the other 4 themes were changed.
- No new theme was added (revisit if a replacement is needed).

## Verification

See `verification.md` for verification and `acceptance.md` for the acceptance
path. The previous delivery is recorded in:
`docs/logs/2026-08-25-vivy-ui-themes/`.
