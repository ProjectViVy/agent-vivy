# Mask identity and lifecycle

The approved mask design is now implemented in the application. An empty mask selection has the explicit identity `我就是我` (English: `Just me`), visible in the toolbar, current-session card, library and details. The default is a presentation of the backend's empty selection ID, never a fabricated catalog definition.

The library restores the original demo's role cards and deliberate preview/use interaction with the real backend catalog. Built-in masks have read-only instructions and a duplicate action. Custom masks have a separate create/edit drawer with Cancel, Save and Save and use, protected drafts, revision conflict recovery and confirmed deletion. Delete remains prohibited while any session references the mask. Desktop shows library and details together; tablet stacks them; mobile uses bottom sheets for selection, details and editing.

The header and management page share the same committed session-selection projection. Switching updates both immediately after the backend response. An admitted reply retains its immutable original role snapshot; the session selection applies to subsequent messages. Loading, unknown identity and catalog failures stay distinguishable from a confirmed default or empty library.

A small Code/Life placeholder sits immediately left of the mask selector and defaults to Code. Its toggle is local presentation only, resets on reload, and does not change runtime Face routing. Model selection remains backend-authoritative. Toolbar controls shrink within the available width, keeping the Chinese default identity fully readable at 320 pixels.

Failure-path review also fixed late definition responses replacing new drafts, stale cached definitions after external edits, conflict recovery disappearing after typing, stale-cache reload after a failed read, edits during post-save refresh, and an editor being trapped behind a slow refresh. Ambiguous creates retain their exact operation ID and payload for safe retry; authoritative validation/authorization rejections allow corrections.

## Ownership and delivery

`vivy/masks-ui` is a first-party T1 source-pinned Module at `repo:plugins/vivy-masks-ui`, selected by the default Recipe. It consumes the supported `std/ui-extension@v1` PresentationHost seam and existing typed `vivy/masks` ActionHost operations. Backend mask storage remains the sole definition/selection authority. Existing authorization, optimistic revisions, immutable admitted-run snapshots and deletion reference checks remain authoritative. There is no additional runtime path, Port, Grant or schema. This maintains completed PLG-P6; no Eino orchestration capability changed.

The host UI kit supplies the generic navigation guard. Module code does not import shell state or another Module's internals. Generated assembly changes come only from SDK staging. The source pin and inspected packed generation are recorded in verification.md. Delivery is a mainline source change; the packed artifact is verification scratch, not a separate external release.

## Design reference and limits

The user approved the composed editable Figma views at https://www.figma.com/design/Ohx5shJMKnLuunRnIqkeW1?node-id=7-672 and then explicitly authorized code implementation. The original reference cards were inspected at commits `13f03f49` and `89f301c9`; demo local storage was not carried into production.

Figma's Starter MCP quota prevented completing secondary lifecycle frames and updating all reference labels. Five primary desktop/preview/active/tablet/mobile screens and reusable components were composed; other wrappers remain unfinished. The actual create/edit flows and latest identity name are implemented and verified in the application. Reference completion is recorded on the open board.

The earlier screenshot's runtime-config fetch failure was not reproduced on the verified split pair. Fetch failures now retain their diagnostic cause; the same-origin fallback still applies only to the documented missing-config/HTML responses. This change does not claim to diagnose the user's earlier startup environment.
