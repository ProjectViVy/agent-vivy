# UI overhaul Block 5 — acceptance

Human checks against the split dev pair (`just dev-vivy` or the isolated
pair; the screenshots in this directory show each state).

1. **Sidebar collapse/expand (desktop).** At ≥768 px the header menu button
   collapses the rail to zero width and back (`nav-open-1280.png` vs
   `nav-toggled-1280.png`). The transition animates width; no horizontal
   scrollbar appears in either state and the transcript keeps its
   `max-w-content` column.
2. **Mobile nav sheet.** At 390 px the same button opens the navigation as a
   left sheet 288 px wide over a scrim (`nav-toggled-390.png`); Escape or a
   scrim tap closes it. Session titles inside wrap to two lines.
3. **Settings tabs at narrow width.** `/settings` at 390 px shows the tab
   chips wrapped on three rows, every tab reachable without horizontal
   scrolling (`settings-390.png`); at 1280 px they sit on one row.
4. **Wizard usable at 390.** On a fresh profile the wizard opens centered,
   fits the viewport (`wizard-intro-390.png`), and `Next` reaches the model
   step with the backend catalog preselected and the footer buttons visible
   (`wizard-model-390.png`); the content column scrolls internally.
5. **Long names clamp.** A session whose first message is very long shows a
   two-line clamped title in the sidebar and history picker with the full
   text in the tooltip; the rail width never changes (`long-title-*.png`,
   live probe `clientH 39 < scrollH 59`).
6. **Model switcher.** The header trigger stacks provider over model, each
   clamped to one line with a tooltip; the menu rows keep icon wells and a
   brand-colored check on the active entry (`nav-open-1280.png`).
7. **History picker.** At 390 px it opens as a bottom sheet with rounded top
   corners; at 1280 px as a large centered dialog that fills neither the
   full width nor the full height.
