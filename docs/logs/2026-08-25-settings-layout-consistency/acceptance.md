# Acceptance — 2026-08-25 Settings-page layout and description consistency

## How a user can confirm it

1. Open `http://127.0.0.1:3015` → the “Settings” item in the sidebar.
2. In the top tab bar, distinguish the real tabs (General / Models / Persona /
   Tools / Vivy Features) from the migration-preview tabs (Channels / Network /
   Language / Compression / Self-Evolution / Sandbox) by the small “Preview” badge
   on preview tabs.
3. On the “General” page, the top-to-bottom order is Application Info, Theme,
   and the Agent-Diva migration preview (including General & About chat display /
   Cache & Runtime Status / About Vivy); the misplaced “Demo / Local Mock” notice
   bar is gone.
4. On the “Network” preview page, the “Current Preview Summary” card has a
   description; on the “Compression” preview page, the “Compression
   Configuration” card has a description—matching the description style of the
   other cards on each page.
5. On the “Vivy Features” page, the Lifecycle and Run Inspector cards have the
   same header structure (icon above, title, description).

## Acceptance criteria

- Every card on the Settings page has a title + description, and cards of the
  same kind share a structure.
- Real configuration and migration previews are distinguishable from the tab bar.
- No misplaced or ambiguous notices.
- `just ci` passes.
