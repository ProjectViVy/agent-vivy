# Acceptance perspective (2026-08-27, Settings → Language)

## How a user can tell it works

1. Open the app and enter 「Settings」 → 「Language」: 「Language」 on the top tab no
   longer carries a gray 「Preview」 badge.
2. The section contains two language-selection cards, "Simplified Chinese / English"; click either:
   - section copy switches immediately (for example, the "Language" title becomes
     "Language", and the literal "Select interface language…" becomes "Pick the interface language…");
   - the selected card shows a checkmark and "Current language".
3. Refresh the page: the last language selection remains (stored in browser localStorage
   `vivy.language`).
4. When the browser's language preference is Chinese, first open still defaults to Simplified Chinese
   (the default has not changed).
5. Other UI copy affected by the language switch (such as the "Model" section title and
   theme-selection card copy) switches as well.
