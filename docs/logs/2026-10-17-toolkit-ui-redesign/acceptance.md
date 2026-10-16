# Acceptance: Toolkit UI redesign

What a human can check in the running split pair
(`just run` + `cd ui; pnpm dev`, then `http://127.0.0.1:3015` → **Toolkits**).

## 1. The page reads as a workspace, not a form

- The route header (title, subtitle) comes from the host; the Module page starts
  with the **Toolkit library** card.
- The library shows `Catalog epoch N · configuration revision M`, one **Refresh**
  control and one **Install Toolkit** button. There is exactly one install
  entry point on the page.
- With nothing installed the library states it and offers no second install
  button.
- Clicking **Refresh** keeps the list and the selected Toolkit in place; the
  button reports `aria-busy` instead of the page blanking out.

## 2. Install is its own decision

- **Install Toolkit** opens a dialog: local directory or Git source, optional
  ref, then **Preview source**.
- A bad path shows the failure inside the dialog and keeps everything typed.
- A good path previews the name, version, origin, `Release SHA-256:` digest and
  the component counts, then **Install disabled** commits it.
- Closing the dialog discards the preview and the error; nothing about the
  install attempt leaks into the details column.

## 3. The selected Toolkit owns its details

- The installed Toolkit appears in the library as `Disabled` and is selected;
  the details column shows its identity (state, source kind, version, bundle id,
  description, source, `Release SHA-256:`), and the lifecycle actions.
- The five component sections (Skills, Masks, Rules, Hooks, MCP) are reachable
  from the summary strip at the top of the inventory.
- Selecting another Toolkit in the library swaps the whole details column
  (including the source fields), and the previous Toolkit's typed source
  location does not follow.

## 4. A change is staged before it happens

- **Prepare enable** (or disable, uninstall, update, rebind, rollback) does not
  mutate anything: a bordered **Prepared change** card appears with the target,
  digest, epoch, operation id, the per-component `added / changed / removed`
  lines with before → after digests, `fresh consent required` where applicable,
  and any blocker text.
- While the card is open, other mutations are locked and the card says so; the
  library, the inventory and **Refresh** stay usable.
- **Publish change** applies it and the card disappears only after the library
  and the details agree with the backend. **Cancel** discards it.

## 5. Host environment bindings and consent are separate, per declaration

- Each Hook and MCP declaration has its own bindings form: it accepts host
  environment variable *names*, never secret values, and shows
  `Missing host environment bindings: …` when the runtime cannot see them.
- Consent is per declaration and bound to the declaration digest: the badge
  reads `not authorized`, `authorized for current declaration`, or
  `old consent is stale; confirm again`, and the button label follows
  (`Confirm authorization` / `Confirm current authorization` /
  `Revoke old authorization`).
- After a release update that changes a declaration, the Hook/MCP section shows
  the stale badge and asks for a fresh confirmation while the change card is
  still open.

## 6. Narrow viewport (390 × 844)

- The page is a single column; the document has no horizontal overflow.
- The library, the prepared change card and the inventory stack in reading
  order, and the details column is reachable by scrolling.
- **View content** opens a dialog that stays inside the viewport, the long body
  wraps instead of overflowing, and `Escape` returns focus to the button that
  opened it.

## 7. Keyboard

- The whole route is reachable with Tab/Enter from the sidebar; focus rings are
  visible on every control, and the dialogs (install, content) are the only
  modals on the page.
