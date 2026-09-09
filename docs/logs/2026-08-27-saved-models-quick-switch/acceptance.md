# Acceptance guide — 2026-08-27 「Selected models」 quick switch

Confirm that the feature works from the user's perspective (development environment:
`just dev`, open `http://127.0.0.1:3015`; or run directly in a Vivy Studio session):

1. **The settings page adds a 「Selected models」 area**: go to 「Settings → Model」. A
   「Selected models」 area appears at the top of the model card—initially empty, showing
   "Click a model in the provider list below to add it to quick switch".
2. **Clicking a model selects and adds it**: select a provider (such as DeepSeek) in the
   provider list and click any model on the right (such as deepseek-chat). The model becomes
   the runtime configuration immediately (the top bar becomes "DeepSeek | deepseek-chat"),
   a `DeepSeek · deepseek-chat` chip appears in the 「Selected models」 area above, and ✓
   appears at the model-row end (current runtime configuration).
3. **Repeated clicks do not add duplicates**: click the same model again; the chip count is
   unchanged (idempotent).
4. **Chip click = immediate switch**: click another chip in 「Selected models」 (for example,
   first add `OpenAI · gpt-4o`); the runtime configuration switches immediately, without
   clicking 「Save real settings」 again.
5. **Inline X = remove bookmark only**: click the small X in a chip; the model disappears
   from the quick list, while **the current runtime configuration is unchanged** (diva's
   "removal clears configuration" side effect was not ported).
6. **Bookmark marks selected models**: a selected but non-current model row shows a gray
   bookmark icon (hover notice "Added to quick list"); the current runtime row shows ✓.
7. **Top-bar quick switch**: click the top-bar model button; the dropdown has two sections,
   「Current configuration」 + 「Selected models」 (only added models, with the current item
   deduplicated). Click any row to switch immediately; X appears on hover at the row end,
   and clicking it only removes the bookmark, leaving the menu open and runtime configuration
   unchanged.
8. **Management entry goes directly there**: 「Manage model settings」 at the bottom of the
   top-bar dropdown enters the settings page and switches to the 「Model」 tab automatically
   (URL becomes `?tab=model`).
9. **Persists after refresh**: refresh the page; the 「Selected models」 list remains
   (`vivy.ui.savedModels` local storage).
10. **Manual form edits still require explicit submission**: after manually changing the
    three inputs to a custom combination outside the catalog, click 「Save real settings」 for
    it to take effect (the auto-save boundary is unchanged).
11. **Read-only deployment**: in a `read_only` deployment, clicking a model / chip / top-bar
    row is disabled (locked), but bookmark removal remains available.

As before, keys are managed only by the runtime environment; the quick list stores no keys.
