# G3 — TUI extras: search, prompt-jump, copy, external editor, startup listing, OSC8, tool renderers

**Goal:** transcript search, prompt-jump nav, copy-last-message, `$EDITOR` external edit, startup resource listing, OSC8 links, per-tool renderer registry.
**Epic:** G. **Requirements:** RQ-TUI. **Partial predecessor:** C1 only for the tree-nav portion (tree nav itself lives in C2 — this story carries the rest).
**Spec:** VCP-D1 §5.8.

## Scope

**Files:** `sdk/tui/view` (search overlay, renderer registry), `sdk/tui/live` (editor integration, startup banner), `sdk/tui/command` (`/search`?— or direct key, per G2 action map).

## Tasks

- [ ] Transcript search: `/` or Ctrl+F opens search line; filters/highlights transcript rows, n/N jump, Esc restores.
- [ ] Prompt-jump: Ctrl+↑/↓ jumps between user messages.
- [ ] Copy-last-assistant-message action (OSC52 → clipboard, degrade gracefully).
- [ ] External editor: action opens `$EDITOR` (fallback vi/nano detect) on a temp file, loads result into composer on save-close.
- [ ] Startup listing: skills/tools/MCP servers/sessions count line under the header (pi parity).
- [ ] OSC8 hyperlinks for paths/URLs in assistant output (clickable in supporting terminals).
- [ ] Tool renderer registry: `view.RegisterToolRenderer(tool, fn)` + default renderer; tool calls render args/result through it (structured vs raw toggle).
- [ ] Tests: search filter, jump order, editor round-trip (fake $EDITOR script), OSC8 sequence presence, renderer dispatch.
- [ ] `go test ./sdk/tui/...`; `just ci`.
- [ ] Commit `feat(tui): transcript search, editor, renderers, startup listing`.

## Boundary

All face-internal. Renderer registry is NOT a plugin Port (documented decision — keeps 14-Port catalog intact).

## Acceptance

Every item demonstrable in one interactive session; each action appears in `/hotkeys` (G2 integration).
