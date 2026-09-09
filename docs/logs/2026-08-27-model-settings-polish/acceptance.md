# Acceptance — model-settings page visual cleanup + themed generation-parameter card

## User view (acceptance steps)

1. Open `http://127.0.0.1:3015/settings` and enter the 「Model」 tab.
2. 「Vivy model configuration」 and 「Generation parameters」 have the same icon card
   headers as the other settings cards (CPU / slider icons + primary-background icon blocks).
3. In the left provider list, the selected item uses a full accent background (no 4px left
   strip); 「More providers」 and 「Add custom provider」 are clean light rows without dashed
   borders.
4. The right panel has three clear groups from top to bottom: provider information + edit
   pencil → API Key → 「XX models」 title + Add button + model list; the 「Sync from official
   catalog」 button no longer exists.
5. The 「Generation parameters」 card has a small 「Demo」 badge beside the title; temperature
   is a slider with a live value on the right (0.0–2.0), Max Tokens is a numeric input;
   clicking 「Save demo parameters」 shows a green checkmark + 「Saved locally」; after refresh,
   the parameters persist (`vivy.demo.*`).
6. After switching to any theme (the theme card in the General tab), the two Model-tab cards
   and control colors change with the theme.

## Actual-result criteria

- Any removed/changed visual element (left strip, dashed row, sync button, in-card amber
  banner, English Temperature label) reappearing = fail.
- The right-side value not moving after dragging the slider or adjusting temperature by
  keyboard = fail.
