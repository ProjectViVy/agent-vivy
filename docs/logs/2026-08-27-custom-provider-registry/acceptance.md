# Acceptance guide — 2026-08-27 custom provider registry

Confirm that the feature works from the user's perspective (development environment:
`just dev`, open `http://127.0.0.1:3015`; or run directly in a Vivy Studio session):

1. **Add custom provider**: go to 「Settings → Model」. At the bottom of the left provider
   list is the dashed row 「＋ Add custom provider」; click it to open the dialog. Fill in:
   display name (for example the literal "My home gateway"), runtime bundle
   (OpenAI-compatible / native Anthropic), Base URL (for example
   `http://localhost:11435/v1`), optional default model, and model list (one per line or
   comma-separated). After saving, a new row appears with a gray 「Custom」 marker at the end.
2. **Custom rows can be edited/deleted**: hover over a custom row to reveal the Pencil
   (Edit) and X (Delete) actions; the edit dialog is prefilled, and saving a changed
   display name updates the row; deleting removes the row (without affecting saved
   bookmarks or the current runtime configuration).
3. **Validation and conflicts**: an empty display name/Base URL is blocked; a Base URL
   that is not http(s) is blocked; entering the same (runtime bundle + Base URL) as the
   catalog or an existing custom provider produces the literal in-dialog error
   "That Base URL already exists".
4. **Clicking a custom model selects it**: click the custom row and its model list appears
   on the right (an empty list shows the literal "No static models" notice). Click a model: it
   immediately becomes the runtime configuration (the top bar becomes the literal
   "Display name | Model"), a `Display name · Model` chip appears in the "Selected models" area above,
   and the bookmark takes effect.
5. **Quick-switch custom models from the top bar**: click the top-bar model button; the
   「Selected models」 section lists the newly added custom entry (title = model id,
   subtitle = display name), and clicking it switches immediately. Hovering reveals X at
   the row end; clicking only removes the bookmark and does not affect runtime configuration.
6. **Rename takes effect globally**: edit the custom provider's display name → every saved
   bookmark chip and top-bar subtitle updates to the new name (single source of truth, with
   no need to edit bookmarks individually).
7. **Label fallback after provider deletion**: delete a custom provider and its bookmark
   remains; the provider name for that entry in chips / the top bar falls back to the Base
   URL hostname (such as `localhost:11435`), without affecting runtime configuration.
8. **Registry and catalog display together**: when searching providers, custom entries
   participate by display name and are mixed into the results (with a 「Custom」 marker);
   outside search, custom rows are always visible (they never enter the 「More providers」
   fold).
9. **Persistence after refresh**: refresh the page; the custom-provider list and selected-
   model bookmarks remain (`vivy.ui.customProviders` / `vivy.ui.savedModels` local storage).
10. **Read-only deployment**: with `read_only`, model/switch-like operations are locked,
    but adding/editing/deleting custom providers and removing bookmarks remain available
    (local preferences).

As before, keys are managed only by the runtime environment: neither the registry nor the
quick list stores any key fields.
