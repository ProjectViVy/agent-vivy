# UI overhaul Block 6 — acceptance

Human checks against the split dev pair (`just dev-vivy` or the isolated pair;
the screenshots in this directory show each state). Switch families from the
theme picker — every route below should look correct in all seven
(`default`, `dark`, `pink`, `miku`, `rice`, `fluor`, `indigo`).

1. **Every route renders in every family at 390 / 768 / 1280.** Visit chat,
   dashboard, approvals, mcp, settings, skills, cron tasks and lifecycle at
   each width and each family. No horizontal scrollbar appears on the page in
   any combination (`block6-report.json`: 168 combos, 0 document overflow).
2. **Page headings are consistent.** Cron and mcp use the shared `PageHeader`
   (icon well + `text-page` title + muted subtitle + right-aligned actions);
   approvals, dashboard, lifecycle and skills headings match the same scale.
   No page shows an oversized `text-2xl`/`text-xl` heading any more.
3. **Status chips share one palette.** Cron task status, channel health
   badges, trajectory ledger chips and diff/ok/danger markers all use the
   semantic `ok / warn / danger / info / brand / neutral` tones — the same
   status reads the same colour on every page and in every family.
4. **Cron empty state fits at 768.** With no cron tasks, at the 768 breakpoint
   the "Create task" action button sits fully inside the list column instead
   of clipping (`route-cron-*-768.png`); at ≥1024 the detail column widens
   back to 20 rem.
5. **Long skill names clamp.** In the skills list a long name (e.g.
   `vivy-advance-dev-env`) truncates with an ellipsis and shows the full name
   in a tooltip instead of overflowing its row.
6. **Run Inspector identifiers are reachable.** In `/settings` → Run Inspector,
   a long run/workflow/child id truncates but the full id appears on hover;
   the child-history list scrolls within a viewport-relative height rather
   than a fixed 10 rem box.
7. **Long plan / approval content scrolls, not clips.** A long plan in the
   plan-review card and a long prompt/command in the approval quick card scroll
   within a viewport-relative cap (`40vh` / `30vh`) instead of being cut off.
8. **Files and To-dos panels render at both widths.** From the chat route, open
   the Files panel and the To-dos panel at 1280 (inline) and at 390 (mobile
   sheet). Neither causes a horizontal scrollbar (`panel-files-*.png`,
   `panel-todos-*.png`).
9. **Token ratchet is empty for `ui/src`.** `pnpm vitest run
   src/theme/tokens.test.ts` reports src palette 0, textSize 0, hex 0 — no
   stock Tailwind colour, arbitrary `text-[Npx]` size or hex literal remains in
   the kernel UI source (plugins still 29/45/0, deferred to Block 7).
