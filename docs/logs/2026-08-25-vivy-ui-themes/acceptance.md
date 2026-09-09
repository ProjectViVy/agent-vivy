# Acceptance path — Vivy UI theming

How a user can confirm the feature is working:

1. Start the development environment (`just dev`, or the existing split pair)
   and open `http://127.0.0.1:3015`.
2. Click “Settings” in the left navigation and stay on the “General” tab.
3. Below the “Application Info” card, find the “Theme” card with 5 preview cards:
   Vivy Blue (selected by default), Love, Minimal Pink & White, Deep Blue Night,
   and Miku Teal. Each card shows a gradient preview, the theme name, and a description.
4. Click each theme in turn: the entire app (sidebar, top bar, cards, and button
   accent color) should switch immediately without a refresh.
   - Love / Minimal Pink & White: a light pink-toned interface.
   - Deep Blue Night / Miku Teal: dark interfaces (the latter uses a teal accent).
5. After selecting any theme, refresh the page (F5): the theme remains selected,
   with no blank screen or incorrect-color flash during loading.
6. Return to the “General” tab: the current theme card still shows a “Selected”
   check mark; the migration-preview area (Demo / Local Mock) no longer contains
   a fake theme-selection card.
7. Theme selection is stored only in the current browser (localStorage
   `vivy.theme`) and does not affect runtime configuration, other browsers, or
   backend state.
