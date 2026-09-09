# Acceptance

1. Drag the window from ~1280px to ~360px: the page does not grow a horizontal scrollbar; the composer stays in view.
2. Below 768px, the menu opens a left drawer. Closing it or changing route dismisses it. Widening the window shows the left rail again without an extra click.
3. Mask and model remain reachable in the header on a phone-sized width.
4. Notebook / Memory / Skills / Approvals (page) / Cron: below `md`, tap a row to see only the detail plus Back to List; from `md` up, list and detail sit side by side.
5. Dialogs and right-hand sheets stay inside the viewport; titles and confirm/cancel stay visible while long content scrolls.

`just ci` remains the gate. User-visible layout also needs the Playwright path in `just ui-e2e` or a walk at `http://127.0.0.1:3015`.
