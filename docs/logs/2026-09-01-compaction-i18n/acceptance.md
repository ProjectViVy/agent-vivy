# Acceptance: How to verify from the user's perspective

- Open `http://127.0.0.1:3015` → Settings → General: all copy in the
  "Context compaction" card displays normally (title, description, enable switch, the
  three form labels and hints, and buttons), with no raw keys such as
  `settings.compaction.title`.
- Switch the language section on the right to English and refresh: the same card
  displays English (Context compaction / Max tokens / Compaction threshold (%) /
  Keep recent messages / Compact now / Saving… and so on), with no Chinese mixed
  in.
- Switch back to Simplified Chinese: the copy is character-for-character
  identical to before the fix (Chinese users see no change).
- Success and failure feedback for saving configuration, Compact now, and
  refreshing usage is localized for both languages as well (for example,
  `Compaction config saved…` and `Compaction complete: X → Y tokens.`).
- Regression line: `compaction-setting.spec.ts` in `just ui-e2e` asserts that all
  labels are present in both zh/en and that the full page contains no raw keys;
  a failed assertion is a regression.
