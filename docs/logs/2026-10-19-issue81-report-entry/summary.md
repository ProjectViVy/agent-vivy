# R1: natural first-report entry

## What changed

- Mounted the existing `NotebookReports` panel in the unselected/empty detail, so an empty section exposes report settings and generation without seeding a report through RPC first.
- Kept one panel per visible state: the empty detail and the selected-entry header never render together.
- Completion now resolves `generation.entry_id` through `NotebookClient.getEntry` before selecting anything, so a report published into a section the picker has not loaded is still selected in its authoritative section. The completion callback is refreshed through a latest-closure ref so the panel's poll cannot act on stale page state.
- A dirty editor (or a pending discard decision) keeps its buffer and selection; completion only refreshes the current list.
- Empty placeholder copy is `emptySection` when the section really has no entries, and `selectEntry` otherwise.
- Rewrote the first browser case to use the natural empty-section flow (open settings, save a disabled schedule, generate, then edit/comment/regenerate). Removed the RPC seeding helper.
- Added isolated port overrides to the notebook Playwright config (`VIVY_NB_E2E_BACKEND_PORT` / `VIVY_NB_E2E_UI_PORT`, defaults 8797/3015) through the shared gate-port resolver.
- Updated the generic notebook browser case: report generation is now expected in the unselected detail, while the omitted-generation case still proves the controls vanish.

Not done: report runtime/persistence, provider narrative quality, W1/W2, T1–T5, BML/E1, ACP/E8, issue mutation or a PR.
