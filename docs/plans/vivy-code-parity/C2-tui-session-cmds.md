# C2 — TUI session commands (/tree /clone /import /export /copy /bug /debug)

**Goal:** pi-parity session command surface in the TUI.
**Epic:** C. **Requirements:** RQ-SESS. **Predecessor:** C1 (storage + RPC).
**Spec:** VCP-D1 §5.4.

## Scope

**Files:** `sdk/tui/command` Specs + handlers; `sdk/tui/view` tree navigator overlay (alt-screen, node graph, labels), transcript copy helper.

## Tasks

- [ ] `/tree`: navigable fork graph over `session/tree` — arrows select, Enter switches session (offer fork when switching branches mid-run), Esc closes. Renderer may start as indented list; upgrade to graph only if cheap.
- [ ] `/clone` → `session/clone` + switch; `/import <path>` → `session/import`; `/export` → `session/export` + show path; `/copy` → last assistant message to clipboard (OSC52 with fallback note).
- [ ] `/bug` → collect diagnostics bundle (redacted log tail + session id + version) into exports dir; `/debug` → tail of session debug log in an overlay.
- [ ] Tests: command specs parse; tree renders a fixture graph; export path shown.
- [ ] `go test ./sdk/tui/...`; `just ci`.
- [ ] Commit `feat(tui): session tree and portability commands`.

## Boundary

`/share` excluded (O4). Tree navigator consumes C1's read model only — no second graph builder in the face.

## Acceptance

Full loop in one TUI session: run → fork → /tree → switch → /export produces a readable HTML.
